package serve

import (
	"fmt"
	"net/http"

	"github.com/kirill-scherba/log"
)

func (s *Server) getObjectHandler(w http.ResponseWriter, r *http.Request) {

	// Process error at return
	var err error
	defer func() {
		if err != nil {
			s.WriteError(w, r, err)
			log.Debug(err)
		}
	}()

	// Check multipart upload
	if r.URL.Query().Has("uploads") {
		s.initiateMultipartHandler(w, r)
		return
	}

	// Check method is "PUT" or "POST" and call PutObjectHandler instead
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		s.putObjectHandler(w, r)
		return
	}

	// Check method is "DELETE" and call DeleteObjectHandler instead
	if r.Method == http.MethodDelete {
		s.deleteObjectHandler(w, r)
		return
	}

	// Log request
	log.Infof("GetObjectHandler %s %s", r.Method, r.URL)

	// Get the range header
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		log.Infof("Client requests a specific range: %s\n", rangeHeader)
	} else {
		// log.Info("Client requests the entire file")
	}

	// Parse path: /bucket1/key1
	bucketName, key, err := parsePath(r)
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
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("ETag", "\""+info.Checksum+"\"")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.ModifiedAt.UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Type", contentType)

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
