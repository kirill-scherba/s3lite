package serve

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/kirill-scherba/log"
)

// ListAllMyBucketsResult — the root element for the ListBuckets response
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListAllMyBucketsResult"`
	Owner   Owner    `xml:"Owner"`
	Buckets []Bucket `xml:"Buckets>Bucket"`
}

type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type Bucket struct {
	Name         string `xml:"Name"`
	CreationDate S3Time `xml:"CreationDate"` // Automatically formatted in ISO8601
}

type S3Time time.Time

func (t S3Time) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	s := time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z")
	return e.EncodeElement(s, start)
}

func (s *Server) listBucketsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Trim leading and trailing slashes
		path := strings.Trim(r.URL.Path, "/")

		// Split the path into parts
		if path != "" {
			parts := strings.Split(path, "/")
			if len(parts) == 1 {
				// Request of the form /bucket1/ — return a list of files
				if r.Method == http.MethodPut {
					s.addBucketHandler(w, r)
					return
				}
				if r.Method == http.MethodDelete {
					s.deleteBucketHandler(w, r)
					return
				}
				s.listObjectsHandler(w, r)
				return
			}

			// Request of the form /bucket1/sub1/file.txt — work with the object
			s.getObjectHandler(w, r)
			return
		}

		// Log request
		log.Infof("ListBucketsHandler %s %s", r.Method, r.URL)

		// Get parameters
		query := r.URL.Query()
		pretty := query.Get("pretty") == "true"

		// Get buckets list
		b, _ := s.buckets.list()

		// Create response
		resp := ListAllMyBucketsResult{
			Owner:   Owner{ID: "scherba-001", DisplayName: "Scherba"},
			Buckets: b,
		}

		// Write xml response
		xmlEncode(w, pretty, resp)
	})
}

func (s *Server) addBucketHandler(w http.ResponseWriter, r *http.Request) {
	// Log request
	log.Infof("AddBucketsHandler %s %s", r.Method, r.URL)

	// Trim leading and trailing slashes
	bucket := strings.Trim(r.URL.Path, "/")

	// Add bucket
	if err := s.buckets.add(bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write headers
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Location", "/"+bucket)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteBucketHandler(w http.ResponseWriter, r *http.Request) {
	// Log request
	log.Infof("DeleteBucketsHandler %s %s", r.Method, r.URL)

	// Trim leading and trailing slashes
	bucket := strings.Trim(r.URL.Path, "/")

	// Delete bucket
	if err := s.buckets.delete(bucket); err != nil {
		sendError(w, "NoSuchBucket",
			"The specified bucket does not exist",
			http.StatusNotFound,
		)
		return
	}

	// Write headers
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusNoContent)
}
