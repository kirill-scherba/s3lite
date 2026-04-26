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

// tryTakeover attempts to become the server. Returns true if takeover was
// performed, false if another goroutine is already handling it.
func (s *S3LiteMulty) tryTakeover() bool {
	if s.isServer {
		return false
	}
	if !s.takingOver.CompareAndSwap(false, true) {
		// Another goroutine is already taking over
		return false
	}

	// Clean up the client connection
	s.client.mu.Lock()
	if s.client.conn != nil {
		s.client.conn.Close()
		s.client.conn = nil
	}
	s.client.mu.Unlock()

	// Try to become the new server
	if err := s.becomeServer(); err != nil {
		log.Printf("multy: failed to become server: %v", err)
		s.takingOver.Store(false)
		return false
	}

	return true
}

// healthCheckLoop periodically checks if the server connection is alive.
// If the connection is lost, it tries to become the new server.
func (s *S3LiteMulty) healthCheckLoop() {
	ticker := time.NewTicker(serverCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		if s.isServer {
			return
		}
		s.client.mu.Lock()
		conn := s.client.conn
		s.client.mu.Unlock()

		if conn == nil {
			return
		}

		// Try to detect if connection is dead (read with zero timeout)
		var zero [1]byte
		conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		_, err := conn.Read(zero[:])
		if err != nil {
			// Timeout means connection is alive (no data available), skip
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				conn.SetReadDeadline(time.Time{}) // Reset deadline
				continue
			}
			log.Printf("multy: server connection lost, trying to become server...")
			s.tryTakeover()
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
// If the server connection is lost, it synchronously attempts to take over
// and become the new server, then executes the request locally.
func (s *S3LiteMulty) sendRequest(req *Request) (*Response, error) {
	s.client.mu.Lock()
	conn := s.client.conn
	s.client.mu.Unlock()

	if conn == nil {
		// Connection already nil — try to become server synchronously
		return s.tryTakeoverAndExecute(req)
	}

	// Send request
	if err := writeRequest(conn, req); err != nil {
		// Write failed — connection lost, try takeover
		return s.tryTakeoverOnError(req, conn)
	}

	// Read response
	resp, err := readResponse(conn)
	if err != nil {
		// Read failed — connection lost, try takeover
		return s.tryTakeoverOnError(req, conn)
	}

	if resp.Err != "" {
		return resp, fmt.Errorf("%s", resp.Err)
	}

	return resp, nil
}

// tryTakeoverOnError cleans up the dead connection, attempts to become
// the server, and if successful executes the request locally.
func (s *S3LiteMulty) tryTakeoverOnError(req *Request, deadConn net.Conn) (*Response, error) {
	// Clean up dead connection
	s.client.mu.Lock()
	if s.client.conn == deadConn {
		deadConn.Close()
		s.client.conn = nil
	}
	s.client.mu.Unlock()

	return s.tryTakeoverAndExecute(req)
}

// tryTakeoverAndExecute tries to become the server and executes the request
// locally if takeover succeeds.
func (s *S3LiteMulty) tryTakeoverAndExecute(req *Request) (*Response, error) {
	// Try to become the new server
	if !s.tryTakeover() {
		return nil, fmt.Errorf("multy: not connected to server")
	}

	// Execute the request locally
	resp := s.executeRequest(req)
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