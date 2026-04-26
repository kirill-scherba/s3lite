// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multy

import (
	"fmt"
	"iter"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kirill-scherba/s3lite"
)

const (
	// maxRetries is the maximum number of reconnection attempts.
	maxRetries = 3
	// retryDelay is the delay between reconnection attempts.
	retryDelay = 100 * time.Millisecond
	// serverCheckInterval is how often a client checks if the server is alive.
	serverCheckInterval = 1 * time.Second
)

// clientConn manages a connection to the Unix socket server.
type clientConn struct {
	conn   net.Conn
	mu     sync.Mutex
	nextID atomic.Uint64
}

// connectClient connects to the Unix socket server.
func (s *S3LiteMulty) connectClient() error {
	path := socketPath(s.bucket)

	// Try to connect with retries
	var conn net.Conn
	var err error
	for i := 0; i < maxRetries; i++ {
		conn, err = net.DialTimeout("unix", path, retryDelay)
		if err == nil {
			break
		}
		time.Sleep(retryDelay)
	}
	if err != nil {
		return fmt.Errorf("multy: can't connect to server at %s: %w", path, err)
	}

	s.client = &clientConn{conn: conn}

	// Start background health check
	go s.healthCheckLoop()

	return nil
}

// healthCheckLoop periodically checks if the server connection is alive.
// If the connection is lost, it tries to become the new server.
func (s *S3LiteMulty) healthCheckLoop() {
	ticker := time.NewTicker(serverCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.client.mu.Lock()
		conn := s.client.conn
		s.client.mu.Unlock()

		if conn == nil {
			return
		}

		// Try to send a dummy message (a zero-byte write isn't reliable,
		// but a read with zero timeout can detect closed connections)
		var zero [1]byte
		conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		if _, err := conn.Read(zero[:]); err != nil {
			// Connection is dead, try to become server
			log.Printf("multy: server connection lost, trying to become server...")

			s.client.mu.Lock()
			s.client.conn.Close()
			s.client.conn = nil
			s.client.mu.Unlock()

			// Try to become the new server
			if err := s.becomeServer(); err != nil {
				log.Printf("multy: failed to become server: %v", err)
				return
			}
			return
		}
		conn.SetReadDeadline(time.Time{}) // Reset deadline
	}
}

// becomeServer closes the client connection, opens Badger directly,
// and starts a Unix socket server.
func (s *S3LiteMulty) becomeServer() error {
	// Try to open Badger directly
	store, err := s3lite.New(s.dbPath, s.bucket)
	if err != nil {
		return fmt.Errorf("multy: can't open Badger to become server: %w", err)
	}
	s.store = store
	s.isServer = true

	// Start Unix socket server
	if err := s.startServer(); err != nil {
		s.store.Close()
		return fmt.Errorf("multy: can't start server after takeover: %w", err)
	}

	log.Printf("multy: successfully became server for bucket %s", s.bucket)
	return nil
}

// closeClient closes the client connection.
func (s *S3LiteMulty) closeClient() {
	if s.client != nil {
		s.client.mu.Lock()
		defer s.client.mu.Unlock()
		if s.client.conn != nil {
			s.client.conn.Close()
			s.client.conn = nil
		}
	}
}

// sendRequest sends a request to the server and returns the response.
func (s *S3LiteMulty) sendRequest(req *Request) (*Response, error) {
	s.client.mu.Lock()
	conn := s.client.conn
	s.client.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("multy: not connected to server")
	}

	// Send request
	if err := writeRequest(conn, req); err != nil {
		return nil, fmt.Errorf("multy: write request failed: %w", err)
	}

	// Read response
	resp, err := readResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("multy: read response failed: %w", err)
	}

	if resp.Err != "" {
		return resp, fmt.Errorf("%s", resp.Err)
	}

	return resp, nil
}

// clientGet implements Get via client.
func (s *S3LiteMulty) clientGet(key string) ([]byte, error) {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "Get",
		Key:    key,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// clientSet implements Set via client.
func (s *S3LiteMulty) clientSet(key string, value []byte, info ...*s3lite.ObjectInfo) (*s3lite.ObjectInfo, error) {
	var objInfo *s3lite.ObjectInfo
	if len(info) > 0 {
		objInfo = info[0]
	}
	req := &Request{
		ID:    s.client.nextID.Add(1),
		Method: "Set",
		Key:   key,
		Value: value,
		Info:  objInfo,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Info, nil
}

// clientDel implements Del via client.
func (s *S3LiteMulty) clientDel(keys ...string) error {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "Del",
		Keys:   keys,
	}
	_, err := s.sendRequest(req)
	return err
}

// clientGetInfo implements GetInfo via client.
func (s *S3LiteMulty) clientGetInfo(key string) (*s3lite.ObjectInfo, error) {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "GetInfo",
		Key:    key,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Info, nil
}

// clientSetInfo implements SetInfo via client.
func (s *S3LiteMulty) clientSetInfo(key string, objectInfo *s3lite.ObjectInfo) (*s3lite.ObjectInfo, error) {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "SetInfo",
		Key:    key,
		Info:   objectInfo,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Info, nil
}

// clientCount implements Count via client.
func (s *S3LiteMulty) clientCount(prefix string) int {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "Count",
		Prefix: prefix,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return 0
	}
	return resp.Count
}

// clientList implements List via client.
func (s *S3LiteMulty) clientList(prefix string) iter.Seq[string] {
	req := &Request{
		ID:     s.client.nextID.Add(1),
		Method: "List",
		Prefix: prefix,
	}
	resp, err := s.sendRequest(req)
	if err != nil {
		return func(yield func(string) bool) {}
	}

	return func(yield func(string) bool) {
		for _, k := range resp.Keys {
			if !yield(k) {
				return
			}
		}
	}
}

// socketExists checks if the Unix socket file exists.
func socketExists(bucket string) bool {
	_, err := os.Stat(socketPath(bucket))
	return err == nil
}