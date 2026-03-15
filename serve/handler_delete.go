package serve

import (
	"net/http"

	"github.com/kirill-scherba/log"
)

func (s *Server) deleteObjectHandler(w http.ResponseWriter, r *http.Request) {

	// Log request
	log.Infof("DeleteObjectHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket1/key1
	bucketName, key, err := parsePath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Delete from S3Lite
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Get key info
	info, err := s3Lite.GetInfo(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if this key is folder
	if info.IsFolder {
		http.Error(w, "key is folder", http.StatusBadRequest)
		return
	}

	// Check if this key is multipart upload and delete all parts
	if uploadId, numParts, exists, err := getMultipadUploadInfo(info); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else if exists {
		for i := range numParts {
			// Delete part from S3Lite
			partKey := partKey(key, uploadId, i+1)
			err = s3Lite.Del(partKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	// Delete key from S3Lite
	err = s3Lite.Del(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
