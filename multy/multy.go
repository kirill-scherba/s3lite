// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multy

import (
	"fmt"
	"iter"
	"net"
	"strings"
	"sync/atomic"

	"github.com/kirill-scherba/s3lite"
)

// S3LiteMulty provides multi-process access to a Badger-backed S3Lite storage.
//
// The first process to open the database becomes the server and listens on a
// Unix socket. Subsequent processes become clients and forward all operations
// to the server. If the server process dies, a client automatically takes over
// and becomes the new server.
type S3LiteMulty struct {
	// Configuration
	dbPath string
	bucket string

	// Server mode fields
	store    s3lite.KeyValueStore // local S3Lite instance (server mode)
	isServer bool
	listener net.Listener

	// Client mode fields
	client    *clientConn
	takingOver atomic.Bool // prevents concurrent takeover attempts
}

// compile-time check that S3LiteMulty implements KeyValueStore.
var _ s3lite.KeyValueStore = (*S3LiteMulty)(nil)

// New creates a new S3LiteMulty instance.
//
// It first tries to open Badger directly (becoming the server). If that fails
// due to a lock conflict (another process already has it open), it connects
// as a client via Unix socket.
//
// Parameters:
//   - dbPath: path to the directory where Badger database files are stored.
//   - bucket: bucket name used as the database subdirectory and socket name.
//
// Returns a KeyValueStore that transparently handles server/client mode.
//
// Example:
//
//	store, err := multy.New("/path/to/db", "key-value-store")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
func New(dbPath, bucket string) (s3lite.KeyValueStore, error) {
	s := &S3LiteMulty{
		dbPath: dbPath,
		bucket: bucket,
	}

	// Try to open Badger directly (become server)
	store, err := s3lite.New(dbPath, bucket)
	if err != nil {
		// Check if this is a lock conflict
		errMsg := err.Error()
		if strings.Contains(errMsg, "resource temporarily unavailable") ||
			strings.Contains(errMsg, "Lock") ||
			strings.Contains(errMsg, "locked") {

			// Try to connect as client
			if connectErr := s.connectClient(); connectErr != nil {
				return nil, fmt.Errorf("multy: can't open as server (%w) and can't connect as client (%v)", err, connectErr)
			}

			return s, nil
		}

		// Other error - return it
		return nil, fmt.Errorf("multy: %w", err)
	}

	// Successfully opened Badger - become server
	s.store = store
	s.isServer = true

	// Start Unix socket server
	if startErr := s.startServer(); startErr != nil {
		store.Close()
		return nil, fmt.Errorf("multy: can't start server: %w", startErr)
	}

	return s, nil
}

// Close closes the storage. If the instance is a server, it also stops the
// Unix socket listener. If it is a client, it closes the client connection.
func (s *S3LiteMulty) Close() {
	if s.isServer {
		s.stopServer()
		if s.store != nil {
			s.store.Close()
		}
	} else {
		s.closeClient()
	}
}

// Get retrieves a value by its key.
func (s *S3LiteMulty) Get(key string) ([]byte, error) {
	if s.isServer {
		return s.store.Get(key)
	}
	return s.clientGet(key)
}

// Set sets a key-value pair. Optionally accepts ObjectInfo for metadata.
func (s *S3LiteMulty) Set(key string, value []byte, info ...*s3lite.ObjectInfo) (*s3lite.ObjectInfo, error) {
	if s.isServer {
		return s.store.Set(key, value, info...)
	}
	return s.clientSet(key, value, info...)
}

// Del deletes one or more keys.
func (s *S3LiteMulty) Del(keys ...string) error {
	if s.isServer {
		return s.store.Del(keys...)
	}
	return s.clientDel(keys...)
}

// List returns an iterator over all keys with the given prefix.
func (s *S3LiteMulty) List(prefix string) iter.Seq[string] {
	if s.isServer {
		return s.store.List(prefix)
	}
	return s.clientList(prefix)
}

// GetInfo retrieves object info by key.
func (s *S3LiteMulty) GetInfo(key string) (*s3lite.ObjectInfo, error) {
	if s.isServer {
		return s.store.GetInfo(key)
	}
	return s.clientGetInfo(key)
}

// SetInfo sets object info for a key.
func (s *S3LiteMulty) SetInfo(key string, objectInfo *s3lite.ObjectInfo) (*s3lite.ObjectInfo, error) {
	if s.isServer {
		return s.store.SetInfo(key, objectInfo)
	}
	return s.clientSetInfo(key, objectInfo)
}

// Count returns the number of keys with the given prefix.
func (s *S3LiteMulty) Count(prefix string) int {
	if s.isServer {
		return s.store.Count(prefix)
	}
	return s.clientCount(prefix)
}

// IsFolder returns true if key is a folder (ends with '/').
func (s *S3LiteMulty) IsFolder(key string) bool {
	l := len(key)
	return l > 0 && key[l-1] == '/'
}

// IsFolderWithFiles returns true if key is a folder and contains files.
func (s *S3LiteMulty) IsFolderWithFiles(key string) bool {
	if s.IsFolder(key) {
		for range s.List(key) {
			return true
		}
	}
	return false
}

// Dir returns the directory part of the key.
func (s *S3LiteMulty) Dir(key string) string {
	idx := strings.LastIndex(key, "/")
	if idx < 0 {
		return ""
	}
	return key[:idx]
}