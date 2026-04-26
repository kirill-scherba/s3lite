// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package s3lite

import "iter"

// KeyValueStore is the interface for S3-like key-value storage.
//
// Both [S3Lite] (direct single-process Badger access) and
// multy.S3LiteMulty (multi-process Unix socket access) implement
// this interface, allowing transparent substitution.
//
// Implementations must handle all methods defined below.
type KeyValueStore interface {
	// Get retrieves a value by its key.
	Get(key string) (value []byte, err error)

	// Set sets a key-value pair. Optionally accepts ObjectInfo for
	// setting content type, metadata, etc.
	Set(key string, value []byte, info ...*ObjectInfo) (objectInfo *ObjectInfo, err error)

	// Del deletes one or more keys.
	Del(keys ...string) (err error)

	// List returns an iterator over all keys with the given prefix.
	List(prefix string) iter.Seq[string]

	// GetInfo retrieves object info by key.
	GetInfo(key string) (objectInfo *ObjectInfo, err error)

	// SetInfo sets object info for a key.
	SetInfo(key string, objectInfo *ObjectInfo) (outObjectInfo *ObjectInfo, err error)

	// Count returns the number of keys with the given prefix.
	Count(prefix string) (count int)

	// IsFolder returns true if key is a folder (ends with '/').
	IsFolder(key string) bool

	// IsFolderWithFiles returns true if key is a folder and contains files.
	IsFolderWithFiles(key string) bool

	// Dir returns the directory part of the key.
	Dir(key string) (dir string)

	// Close closes the storage and releases resources.
	Close()
}