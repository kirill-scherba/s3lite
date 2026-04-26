// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package multy provides a multi-process S3Lite access via Unix domain socket.
//
// The first process to open a Badger database becomes the server and listens
// on a Unix socket. Subsequent processes become clients and forward all
// operations to the server. If the server process dies, a client
// automatically takes over and becomes the new server.
package multy

import (
	"encoding/gob"
	"io"

	"github.com/kirill-scherba/s3lite"
)

// Request types sent from client to server.
type Request struct {
	ID     uint64
	Method string // "Get", "Set", "Del", "List", "GetInfo", "SetInfo", "Count", "Close"

	Key    string
	Value  []byte
	Info   *s3lite.ObjectInfo
	Keys   []string
	Prefix string
}

// Response types sent from server to client.
type Response struct {
	ID      uint64
	Value   []byte
	Info    *s3lite.ObjectInfo
	Keys    []string
	Count   int
	Err     string // empty if no error, error message otherwise
}

// init registers types with gob encoder.
func init() {
	gob.Register(&Request{})
	gob.Register(&Response{})
	gob.Register(&s3lite.ObjectInfo{})
}

// writeRequest writes a Request to the writer using gob encoding.
func writeRequest(w io.Writer, req *Request) error {
	return gob.NewEncoder(w).Encode(req)
}

// readRequest reads a Request from the reader using gob decoding.
func readRequest(r io.Reader) (*Request, error) {
	var req Request
	err := gob.NewDecoder(r).Decode(&req)
	return &req, err
}

// writeResponse writes a Response to the writer using gob encoding.
func writeResponse(w io.Writer, resp *Response) error {
	return gob.NewEncoder(w).Encode(resp)
}

// readResponse reads a Response from the reader using gob decoding.
func readResponse(r io.Reader) (*Response, error) {
	var resp Response
	err := gob.NewDecoder(r).Decode(&resp)
	return &resp, err
}