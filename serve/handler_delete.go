package serve

import (
	"net/http"

	"github.com/kirill-scherba/log"
)

func (s *Server) deleteObjectHandler(w http.ResponseWriter, r *http.Request) {

	// Process error at return
	var err error
	defer func() {
		if err != nil {
			s.WriteError(w, r, err)
			log.Debug(err)
		}
	}()

	// Log request
	log.Infof("DeleteObjectHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket1/key1
	bucketName, key, err := parsePath(r)
	if err != nil {
		return
	}

	// Delete from S3Lite
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		return
	}

	// Get key info
	info, err := s3Lite.GetInfo(key)
	if err != nil {
		err = ErrNoSuchKey
		return
	}

	// Check if this key is folder
	if info.IsFolder {
		// err = ErrKeyIsFolder
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Check if this key is multipart upload and delete all parts
	if uploadId, numParts, exists, err := getMultipadUploadInfo(info); err != nil {
		err = ErrInvalidQuery
		return
	} else if exists {
		for i := range numParts {
			// Delete part from S3Lite
			partKey := partKey(key, uploadId, i+1)
			err = s3Lite.Del(partKey)
			if err != nil {
				err = ErrInvalidQuery
				return
			}
		}
	}

	// Delete key from S3Lite
	err = s3Lite.Del(key)
	if err != nil {
		err = ErrInvalidQuery
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
