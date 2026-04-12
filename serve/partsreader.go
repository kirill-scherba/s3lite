package serve

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"

	"github.com/kirill-scherba/s3lite"
)

// PartsReader struct holds information about a multipart upload
type PartsReader struct {
	id            uint64
	key           string            // Key
	s3Lite        *s3lite.S3Lite    // S3Lite object
	parts         []PartMeta        // List of part metadata (key and size of each part)
	contentLength int64             // Count of bytes (size of file)
	offset        int64             // Current offset (cursor position in file)
	partsCached   map[string][]byte // Cache parts data
}

// PartMeta struct
type PartMeta struct {
	Key  string
	Size int64
}

// partsReaderId is used to generate unique IDs for DBPartsReader objects
var partsReaderId atomic.Uint64

// newPartsReader creates a new DBPartsReader object from the given S3Lite and key.
//
// Parameters:
//   - s3Lite: the S3Lite object to use for reading.
//   - key: the key of the object to read.
//
// Returns:
//   - r: the created DBPartsReader object.
//   - err: an error if the creation fails.
//
// Example:
//
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
// r, err := newPartsReader(s, "key")
//
//	if err != nil {
//		log.Fatal(err)
//	}
func newPartsReader(s3Lite *s3lite.S3Lite, key string) (
	r *PartsReader, err error) {

	id := partsReaderId.Add(1)

	// Create DBPartsReader object
	r = &PartsReader{
		id:          id,
		key:         key,
		offset:      0,
		partsCached: make(map[string][]byte),
	}

	// Set S3Lite object
	r.s3Lite = s3Lite

	// Get key info
	keyInfo, err := r.s3Lite.GetInfo(key)
	if err != nil {
		return
	}
	r.contentLength = keyInfo.ContentLength

	// Get multipart upload info
	uploadId, numParts, exist, err := getMultipadUploadInfo(keyInfo)
	if err != nil {
		return
	}
	if exist {
		// Multipart upload
		for i := range numParts {
			partKey := partKey(key, uploadId, i+1)
			partInfo, errPartInfo := r.s3Lite.GetInfo(partKey)
			if errPartInfo != nil {
				err = errPartInfo
				return
			}
			r.parts = append(r.parts, PartMeta{partKey, partInfo.ContentLength})
		}
		return
	}

	// Single part upload
	r.parts = append(r.parts, PartMeta{key, r.contentLength})

	return
}

// Read realizes io.Reader
func (r *PartsReader) Read(p []byte) (n int, err error) {
	if r.offset >= r.contentLength {
		return 0, io.EOF
	}

	var currentOffset int64
	for _, part := range r.parts {
		partSize := int64(part.Size)
		// Check if we need to read this part
		if r.offset < currentOffset+partSize {
			// Read part
			data, err := r.Get(part.Key)
			if err != nil {
				return 0, err
			}

			// Calculate the offset from which to start reading within this part
			relativeOffset := r.offset - currentOffset
			copied := copy(p, data[relativeOffset:])

			r.offset += int64(copied)
			return copied, nil
		}
		currentOffset += partSize
	}
	return 0, io.EOF
}

// Get retrieves the data for the given key from the cache or from S3Lite if it's not in the cache.
// If the key is not found in the cache, it is retrieved from S3Lite and added to the cache.
// If the key is not found in S3Lite, an error is returned.
// Parameters:
//   - key: the key of the object to retrieve.
//
// Returns:
//   - data: the retrieved object data.
//   - err: an error if the retrieval fails.
func (r *PartsReader) Get(key string) ([]byte, error) {

	// Get from cache
	if data, ok := r.partsCached[key]; ok {
		return data, nil
	}

	// Get from S3Lite
	data, err := r.s3Lite.Get(key)
	if err != nil {
		return nil, err
	}

	// Add to cache and return
	r.partsCached[key] = data
	return data, nil
}

// Seek realizes io.Seeker (allows to set cursor position in file)
func (r *PartsReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		newOffset = r.contentLength + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if newOffset < 0 || newOffset > r.contentLength {
		return 0, errors.New("offset out of range")
	}

	r.offset = newOffset
	return r.offset, nil
}

// Close the DBPartsReader
func (r *PartsReader) Close() error {
	return nil
}

// getMultipadUploadInfo retrieves multipart upload information from the given keyInfo.
//
// Parameters:
//   - keyInfo: the object info to retrieve multipart upload information from.
//
// Returns:
//   - uploadId: the UUID of the multipart upload, empty if it doesn't exist.
//   - numParts: the number of parts in the multipart upload, 0 if it doesn't exist.
//   - exists: true if the multipart upload exists for the given keyInfo, false otherwise.
//   - err: an error if the GetMultipadUploadInfo operation fails.
func getMultipadUploadInfo(keyInfo *s3lite.ObjectInfo) (uploadId string,
	numParts int, exists bool, err error) {

	if uploadId, exists = keyInfo.Metadata["uploadId"]; exists && uploadId != "" {
		if str, ok := keyInfo.Metadata["numParts"]; ok && str != "" {
			numParts, err = strconv.Atoi(str)
			if err != nil {
				return
			}

			// Multipart upload exists for this keyInfo.
			exists = true
			return
		}
	}

	// Multipart upload doesn't exist for this keyInfo
	return
}

// partKey returns the key for a given part of a multipart upload.
func partKey(key, uploadId string, partNumber int) string {
	return fmt.Sprintf("%s.%s.%d", key, uploadId, partNumber)
}
