// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S3 like native Golang storage.
//
// This package provides a simple storage system, similar to Amazon S3.
// It allows to store and retrieve data by key.
// Data is stored in a directory, with each key being a file.
// The package provides functions to set, get, and delete data.
// It also provides an iterator to traverse over all data in the storage.
package s3lite

import (
	"fmt"
	"iter"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v3"
)

// S3Lite is a wrapper for Badger database.
type S3Lite struct{ db *badger.DB }

// New creates a new S3 object.
//
// The dbPath argument is the path to the directory where the Badger database
// files will be stored.
//
// If the database connection can't be opened, an error is returned.
//
// Example:
// s, err := s3.New("/path/to/db")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func New(path, bucket string) (s *S3Lite, err error) {
	s = &S3Lite{}

	// Open connection to database
	err = s.open(path, bucket)
	if err != nil {
		err = fmt.Errorf("can't open database connection: %w", err)
		return
	}

	return
}

// Close closes the database connection.
func (s *S3Lite) Close() {
	s.db.Close()
}

// Set sets a key-value pair in the database.
//
// Parameters:
//   - key: the key to set in the database.
//   - value: the value to set in the database.
//
// Returns:
//   - error: an error if the Set operation fails.
//
// Example:
//	s, err := s3.New("/path/to/db")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	err = s.Set("key", []byte("value"))
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *S3Lite) Set(key string, value []byte) (err error) {
	err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), value)
		return err
	})
	return
}

// Get retrieves a value by its key from the database.
//
// Parameters:
//   - key: the key to get from the database.
//
// Returns:
//   - value: the value retrieved from the database.
//   - error: an error if the Get operation fails.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
// value, err := s.Get("key")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *S3Lite) Get(key string) (value []byte, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return
}

// Del deletes a key-value pair from the database.
//
// Parameters:
//   - key: the key to delete from the database.
//
// Returns:
//   - error: an error if the Del operation fails.
//
// Example:
// s, err := s3.New("/path/to/db")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
// err = s.Del("key")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *S3Lite) Del(key string) (err error) {
	err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(key))
		return err
	})
	return
}

// List returns an iterator for all keys with given prefix.
//
// The iterator yields each key that starts with the given prefix.
// The keys are yielded in lexicographical order.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
// keysIter, err := s.List("prefix")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for keysIter.Next() {
//		key := keysIter.Value()
//		fmt.Println(key)
//	}
func (s *S3Lite) List(prefix string) iter.Seq[string] {

	// Return iterator
	return func(yield func(key string) bool) {

		s.db.View(func(txn *badger.Txn) (err error) {

			// Create new iterator
			iterator := txn.NewIterator(badger.DefaultIteratorOptions)
			defer iterator.Close()

			// Calculate number of folders in prefix
			numFoldersInPrefix := strings.Count(prefix, "/")
			if len(prefix) > 0 && prefix[len(prefix)-1] != '/' {
				numFoldersInPrefix++
			}

			// Create map to store subfolders
			var subfolders = make(map[string]struct{})

			// Iterate over keys with prefix
			for iterator.Seek([]byte(prefix)); iterator.ValidForPrefix([]byte(prefix)); {

				// Get key
				key := string(iterator.Item().KeyCopy(nil))

				// Check if key has correct number of folders
				var skip bool
				if strings.Count(key, "/") > numFoldersInPrefix {
					// Split by folders
					folders := strings.Split(key, "/")

					// Make folder key
					key = strings.Join(folders[:numFoldersInPrefix+1], "/") + "/"

					// Check if key is in subfolder map and skip if it is
					if _, ok := subfolders[key]; !ok {
						subfolders[key] = struct{}{}
					} else {
						skip = true
					}
				}

				// Valid key found if not skipped, sending to yield function
				if !skip && !yield(key) {
					break
				}

				// Get next key
				iterator.Next()
				if !iterator.Valid() {
					break
				}
			}

			return
		})
	}
}

// IsFolder returns true if key is a folder.
func (s *S3Lite) IsFolder(key string) bool {
	l := len(key)
	return l > 0 && key[l-1] == '/'
}

// Dir returns the directory of the given key.
func (s *S3Lite) Dir(key string) (dir string) {
	dir = filepath.Dir(key)
	if dir == "." {
		dir = ""
	}
	return
}

// open create new badger database connection.
func (s *S3Lite) open(path, bucket string) (err error) {

	// Remove trailing slash from path
	if _, ok := strings.CutSuffix(path, "/"); !ok {
		path += "/"
	}

	// Check if path is empty then open in memory
	var inMemory bool
	if path == "/" {
		inMemory = true
		bucket = ""
		path = ""
	}

	// Add bucket and extension to path
	if !inMemory {
		path += bucket + ".s3lite"
	}

	s.db, err = badger.Open(badger.DefaultOptions(path).WithInMemory(inMemory))
	return
}
