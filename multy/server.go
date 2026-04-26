// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multy

import (
	"log"
	"net"
	"os"
)

const socketDir = "/tmp"

// socketPath returns the path to the Unix socket for the given bucket.
func socketPath(bucket string) string {
	return socketDir + "/s3lite-" + bucket + ".sock"
}

// startServer starts a Unix socket listener and handles incoming connections.
// It runs in a separate goroutine. The listener is stored in S3LiteMulty.listener.
func (s *S3LiteMulty) startServer() error {
	path := socketPath(s.bucket)

	// Remove stale socket file if it exists
	os.Remove(path)

	// Create Unix socket listener
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	s.listener = listener

	// Accept connections in a goroutine
	go s.acceptLoop()
	return nil
}

// stopServer stops the Unix socket listener.
func (s *S3LiteMulty) stopServer() {
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
}

// acceptLoop accepts incoming connections and handles them in separate goroutines.
func (s *S3LiteMulty) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener was closed, exit the loop
			return
		}
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection.
// It reads requests, executes them, and sends responses back.
func (s *S3LiteMulty) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		req, err := readRequest(conn)
		if err != nil {
			return
		}

		resp := s.executeRequest(req)

		if err := writeResponse(conn, resp); err != nil {
			log.Printf("multy: failed to write response: %v", err)
			return
		}
	}
}

// executeRequest executes the request on the local S3Lite and returns a response.
func (s *S3LiteMulty) executeRequest(req *Request) *Response {
	resp := &Response{ID: req.ID}

	var err error

	switch req.Method {
	case "Get":
		resp.Value, err = s.store.Get(req.Key)
	case "Set":
		resp.Info, err = s.store.Set(req.Key, req.Value, req.Info)
	case "Del":
		err = s.store.Del(req.Keys...)
	case "GetInfo":
		resp.Info, err = s.store.GetInfo(req.Key)
	case "SetInfo":
		resp.Info, err = s.store.SetInfo(req.Key, req.Info)
	case "Count":
		resp.Count = s.store.Count(req.Prefix)
	case "List":
		// For List we collect all keys into a slice
		for k := range s.store.List(req.Prefix) {
			resp.Keys = append(resp.Keys, k)
		}
	default:
		resp.Err = "unknown method: " + req.Method
	}

	if err != nil {
		resp.Err = err.Error()
	}

	return resp
}