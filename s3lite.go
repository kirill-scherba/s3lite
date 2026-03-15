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

// ErrKeyNotFound is returned when key isn't found on storage.
var ErrKeyNotFound = badger.ErrKeyNotFound

// S3Lite is a wrapper for Badger database.
type S3Lite struct {
	db     *badger.DB
	dbInfo *badger.DB
}

// New creates a new S3 object.
//
// The dbPath argument is the path to the directory where the Badger database
// files will be stored.
//
// If the database connection can't be opened, an error is returned.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func New(path, bucket string) (s *S3Lite, err error) {
	s = &S3Lite{}

	// Open connection to bucket database
	s.db, err = s.open(path, bucket)
	if err != nil {
		err = fmt.Errorf("can't open database connection: %w", err)
		return
	}

	// Open connection to bucket-info database
	s.dbInfo, err = s.open(path, bucket+"-info")
	if err != nil {
		err = fmt.Errorf("can't open database info connection: %w", err)
		return
	}

	return
}

// Close closes the database connection.
func (s *S3Lite) Close() {
	s.dbInfo.Close()
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
//
//	s, err := s3.New("/path/to/db", "bucket")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	err = s.Set("key", []byte("value"))
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *S3Lite) Set(key string, value []byte, info ...*ObjectInfo) (
	objectInfo *ObjectInfo, err error) {

	// Set object data
	if err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), value)
		return err
	}); err != nil {
		return
	}

	// Set content type and metadata from input object info
	// var contentType string
	objectInfo = &ObjectInfo{}
	if len(info) > 0 && info[0] != nil {
		objectInfo = info[0]
	}

	// Set ommited required object info parameters
	objectInfo.set(s, key, value)

	// Set object info
	objectInfo, err = s.SetInfo(key, objectInfo)
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

// GetInfo retrieves object info by its key from the database.
//
// Parameters:
//   - key: the key to get object info from the database.
//
// Returns:
//   - objectInfo: the object info retrieved from the database.
//   - error: an error if the GetInfo operation fails.
//
// Example:
// s, err := s3.New("./data", "my-bucket")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	objectInfo, err := s.GetInfo("key")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func (s *S3Lite) GetInfo(key string) (objectInfo *ObjectInfo, err error) {

	// Init object info
	objectInfo = &ObjectInfo{}

	// Check if key object is folder
	if s.IsFolder(key) {
		objectInfo.IsFolder = true
		return
	}

	// Get object info
	err = s.dbInfo.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		objectInfo = &ObjectInfo{}
		err = objectInfo.UnmarshalBinary(value)

		return err
	})
	return
}

// SetInfo sets object info.
//
// Parameters:
//   - key: the key to set in the database.
//   - value: the value to set in the database.
//   - contentType: the content type of the value, default "application/octet-stream".
//   - metadata: the additional metadata in key-value pairs.
//
// Returns:
//   - error: an error if the SetInfo operation fails.
func (s *S3Lite) SetInfo(key string, objectInfo *ObjectInfo) (
	outObjectInfo *ObjectInfo, err error) {

	// Create object info
	// objectInfo = new(ObjectInfo).set(s, key, value, contentType, metadata)
	outObjectInfo = objectInfo

	// Set object info
	err = s.dbInfo.Update(func(txn *badger.Txn) error {
		// Marshal objectInfo info dataBytes binary marshal
		dataBytes, err := objectInfo.MarshalBinary()
		if err != nil {
			return err
		}

		// Set object info
		err = txn.Set([]byte(key), dataBytes)
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

	fmt.Printf("Del %s\n", key)

	// Delete object
	err = s.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(key))
		return err
	})

	// Delete object info
	err = s.dbInfo.Update(func(txn *badger.Txn) error {
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
func (s *S3Lite) open(path, bucket string) (db *badger.DB, err error) {

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

	// Open database
	db, err = badger.Open(badger.DefaultOptions(path).WithInMemory(inMemory).
		WithLogger(nil))

	return
}
