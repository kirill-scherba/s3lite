package serve

import (
	"fmt"
	"net/http"

	"github.com/kirill-scherba/log"
)

// getObjectHandler is a handler for GET /bucket/key requests. It processes
// multipart upload initialization and completion, and handles normal GET
// requests. It returns an error if the specified bucket does not exist,
// or if the specified key does not exist.
//
// The handler logs the request, parses the path, gets the S3Lite object for
// the bucket, gets the object info, sets the necessary headers,
// and serves the content. It also sets the metadata to headers.
//
// If the request is a HEAD request, the handler just writes the headers
// and does not serve the content.
func (s *Server) getObjectHandler(w http.ResponseWriter, r *http.Request) {

	// Process error at return
	var bucketName, key string
	var err error
	defer func() {
		if err != nil {
			s.WriteError(w, r, err)
			log.Debugf("Get key %s error: %s", key, err)
		}
	}()

	// Check multipart upload
	if r.URL.Query().Has("uploads") && r.Header.Get("X-Amz-Metadata-Directive") == "" {
		s.initiateMultipartHandler(w, r)
		return
	}

	// Log request
	log.Infof("GetObjectHandler %s %s", r.Method, r.URL)

	// Get the range header
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		log.Debugf("Client requests a specific range: %s, for key: %s\n",
			rangeHeader, r.URL.Path)
	} else {
		// log.Info("Client requests the entire file")
	}

	// Parse path: /bucket1/key1
	bucketName, key, err = parsePath(r)
	if err != nil {
		return
	}

	// Get S3Lite object for bucket
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		err = ErrNoSuchBucket
		return
	}
	info, err := s3Lite.GetInfo(key)
	if err != nil {
		err = ErrNoSuchKey
		return
	}
	contentType := info.ContentType
	contentLength := info.ContentLength

	// Set necessary headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("ETag", "\""+info.Checksum+"\"")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.ModifiedAt.UTC().Format(http.TimeFormat))

	// Set metadata to headers
	for h, v := range info.Metadata {
		w.Header().Set(h, v)
	}

	// Skip writing content if this is a HEAD request
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get parts reader and serve content
	reader, err := newPartsReader(s3Lite, key)
	if err != nil {
		return
	}
	http.ServeContent(w, r, key, info.ModifiedAt, reader)
	reader.Close()
}
