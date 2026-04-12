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

// S3Time is a time.Time that marshals to ISO8601.
type S3Time time.Time

func (t S3Time) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	s := time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z")
	return e.EncodeElement(s, start)
}

// listBucketsHandler is a handler for GET / requests. It returns a list of all
// buckets, including the name and creation date of each bucket.
//
// The handler takes one parameter: "pretty". If the parameter is set to "true", the
// handler formats the XML response with indentation and line breaks. Otherwise, the
// handler formats the XML response with no indentation or line breaks.
//
// The handler logs the request, gets the list of buckets, creates a ListAllMyBucketsResult
// struct, and writes the struct to the HTTP response writer in XML format.
//
// Example:
// curl -X GET 'http://localhost:8080?pretty=true'
//
// Response:
// <?xml version="1.0" encoding="UTF-8"?>
// <ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
//     <Owner>
//         <ID>scherba-001</ID>
//         <DisplayName>Scherba</DisplayName>
//     </Owner>
//     <Buckets>
//         <Bucket>
//             <Name>bucket1</Name>
//             <CreationDate>2022-04-11T12:46:55.000Z</CreationDate>
//         </Bucket>
//         <Bucket>
//             <Name>bucket2</Name>
//             <CreationDate>2022-04-11T12:46:55.000Z</CreationDate>
//         </Bucket>
//     </Buckets>
// </ListAllMyBucketsResult>
func (s *Server) listBucketsHandler(w http.ResponseWriter, r *http.Request) {

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
}

// addBucketHandler is a handler for PUT /bucket requests. It adds a new
// bucket with the given name. If the bucket already exists, it will
// return an error with a status code of 409. It will set the Date and
// Location headers, and write a status code of 200.
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

// deleteBucketHandler deletes a bucket with the given name.
// If the bucket does not exist, it will return an error with a status code of 404.
// At the end, it will write a status code of 204 No Content to the http client.
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

// checkBucketHandler is a handler for HEAD /bucket requests. It checks
// if the specified bucket exists. If the bucket does not exist, it will
// return an error with a status code of 404. At the end, it will write
// a status code of 204 No Content to the http client.
func (s *Server) checkBucketHandler(w http.ResponseWriter, r *http.Request) {
	// Log request
	log.Infof("CheckBucketHandler %s %s", r.Method, r.URL)

	// Trim leading and trailing slashes
	bucket := strings.Trim(r.URL.Path, "/")

	// Get bucket and return Status on error
	if _, err := s.buckets.get(bucket); err != nil {
		s.WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
