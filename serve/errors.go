package serve

import (
	"fmt"
	"net/http"
)

var (
	ErrAccessDenied  = S3Error{"AccessDenied", "Access Denied", http.StatusForbidden}
	ErrInternalError = S3Error{"InternalError", "We encountered an internal error", http.StatusInternalServerError}

	ErrNoSuchBucket      = S3Error{"NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound}
	ErrBucketNotEmpty    = S3Error{"BucketNotEmpty", "The bucket you tried to delete is not empty", http.StatusConflict}
	ErrInvalidBucketName = S3Error{"InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest}

	ErrNoSuchKey    = S3Error{"NoSuchKey", "The specified key does not exist.", http.StatusNotFound}
	ErrInvalidURI   = S3Error{"InvalidURI", "Could not parse the specified URI.", http.StatusBadRequest}
	ErrInvalidQuery = S3Error{"InvalidQuery", "The specified query is not valid.", http.StatusBadRequest}
	ErrKeyIsFolder  = S3Error{"NoSuchKey", "The specified key is a directory-like prefix, not an object.", http.StatusNotFound}
)

// S3Error represents an error returned by S3.
type S3Error struct {
	Code       string // XML code (e.g. NoSuchBucket)
	Message    string // Human-readable description
	HTTPStatus int    // HTTP response code (404, 403, etc.)
}

// Error returns a string representation of the S3Error.
func (e S3Error) Error() string {
	return fmt.Sprintf("%d %s: %s", e.HTTPStatus, e.Code, e.Message)
}

// WriteError writes an error response to the client in XML format.
func (s *Server) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	// Convert error to S3Error
	s3Err, ok := err.(S3Error)
	if !ok {
		// Unknown error type - default to internal error
		s3Err = ErrInternalError
		s3Err.Message = err.Error()
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(s3Err.HTTPStatus)

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
    <Code>%s</Code>
    <Message>%s</Message>
    <Resource>%s</Resource>
    <RequestId>1337</RequestId>
</Error>`, s3Err.Code, s3Err.Message, r.URL.Path)

	w.Write([]byte(xmlBody))
}
