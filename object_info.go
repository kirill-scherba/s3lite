// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The ObjectInfo of S3 like native Golang storage.

package s3lite

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"
)

// ObjectInfo of S3 like native Golang storage.
type ObjectInfo struct {
	IsFolder      bool              // True if key is a folder
	ContentLength int64             // Content length in bytes
	ContentType   string            // Content type, default "application/octet-stream"
	CreatedAt     time.Time         // Created at
	ModifiedAt    time.Time         // Modified at
	Checksum      string            // MD5 checksum
	Metadata      map[string]string // Some additional Metadata in key-value pairs
}

// SetContentType returns object info with content type set.
func SetContentType(contentType string) *ObjectInfo {
	return &ObjectInfo{ContentType: contentType}
}

// SetMetadata returns object info with metadata set.
func SetMetadata(metadata map[string]string) *ObjectInfo {
	return &ObjectInfo{Metadata: metadata}
}

// SetContentType sets content type of object info and returns object info.
func (o *ObjectInfo) SetContentType(contentType string) *ObjectInfo {
	o.ContentType = contentType
	return o
}

// SetMetadata sets metadata of object info and returns object info.
func (o *ObjectInfo) SetMetadata(metadata map[string]string) *ObjectInfo {
	o.Metadata = metadata
	return o
}

// MarshalBinary marshal objectInfo info json data to bytes array.
// The method returns json marshaled object info data and error if json
// marshaling failed. If json marshaling failed, the method returns error with
// message "can't marshal object info: %w".
func (o *ObjectInfo) MarshalBinary() (data []byte, err error) {
	// Marshal objectInfo info json
	data, err = json.Marshal(o)
	if err != nil {
		err = fmt.Errorf("can't marshal object info: %w", err)
		return
	}

	return
}

// UnmarshalBinary unmarshals object info json data from bytes array.
// The method returns error if json unmarshaling failed. If json unmarshaling
// failed, the method returns error with message "can't unmarshal object info: %w".
func (o *ObjectInfo) UnmarshalBinary(data []byte) (err error) {
	// Unmarshal objectInfo info json
	err = json.Unmarshal(data, o)
	if err != nil {
		err = fmt.Errorf("can't unmarshal object info: %w", err)
		return
	}

	return
}

// Set sets object info.
//
// Parameters:
//   - s: the S3Lite storage.
//   - key: the key to set in the S3Lite storage.
//   - value: the value to set in the S3Lite storage.
//   - contentType: the content type of the value, default "application/octet-stream".
//   - m: the additional metadata in key-value pairs.
//
// Returns:
//   - *ObjectInfo: the object info.
//
// Example:
//
//	s, err := s3.New("./data", "my-bucket")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	err = s.Set("key", []byte("value"), "text/plain", map[string]string{
//		"Metadata": "value",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
func (o *ObjectInfo) set(s *S3Lite, key string, value []byte) *ObjectInfo {

	// Set content type
	if o.ContentType == "" {
		o.ContentType = "application/octet-stream"
	}

	// Create md5 checksum
	o.Checksum = fmt.Sprintf("%x", md5.Sum(value))

	// Check if key exists and get CreatedAt
	o.CreatedAt = time.Now()
	o.ModifiedAt = o.CreatedAt
	if info, err := s.GetInfo(key); err == nil {
		o.CreatedAt = info.CreatedAt
	}

	// Set object info
	o.ContentLength = int64(len(value))

	return o
}
