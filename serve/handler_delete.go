package serve

import (
	"net/http"

	"github.com/kirill-scherba/log"
)

// deleteObjectHandler delete object from S3Lite storage.
// It will parse the path, get S3Lite object from the bucket,
// check if the object with the same key already exists,
// and delete it from S3Lite.
// If the object is a folder with files, it will skip it.
// If the object is a multipart upload, it will delete all parts.
// If the object does not exist, it will return ErrNoSuchKey.
// If the object is a folder, it will return ErrKeyIsFolder.
// If the object is a multipart upload and one of the parts does not exist,
// it will return ErrInvalidQuery.
// At the end, it will write status 204 No Content to the http client.
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

	// Check if this key is folder with files then skip it
	if info.IsFolder && s3Lite.IsFolderWithFiles(key) {
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
