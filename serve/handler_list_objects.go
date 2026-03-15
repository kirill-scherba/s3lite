package serve

import (
	"encoding/xml"
	"net/http"
	"sort"
	"strings"

	"github.com/kirill-scherba/log"
)

type ListBucketResultV2 struct {
	XMLName               xml.Name       `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name                  string         `xml:"Name"`                     // Bucket name
	Prefix                string         `xml:"Prefix"`                   // Filter (prefix)
	Delimiter             string         `xml:"Delimiter"`                // Filter (delimiter)
	KeyCount              int            `xml:"KeyCount"`                 // Number of objects in response
	MaxKeys               int            `xml:"MaxKeys"`                  // Limit
	IsTruncated           bool           `xml:"IsTruncated"`              // Is there another page?
	Contents              []Object       `xml:"Contents"`                 // List of files
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"` // Folders
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
}

type Object struct {
	Key          string `xml:"Key"`          // File path
	LastModified S3Time `xml:"LastModified"` // Last modified date
	ETag         string `xml:"ETag"`         // MD5 hash in quotes: "abc123..."
	Size         int64  `xml:"Size"`         // Size in bytes
	StorageClass string `xml:"StorageClass"` // Default is "STANDARD"
}

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

func (s *Server) listObjectsHandler(w http.ResponseWriter, r *http.Request) {

	// Log request
	log.Infof("ListObjectsHandler %s %s", r.Method, r.URL)

	// Extract the bucket name from the URL path (e.g. from /bucket1)
	bucketName := strings.Trim(r.URL.Path, "/")

	// Get the filter parameters
	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	pretty := query.Get("pretty") == "true"
	recursive := delimiter == ""

	// listType := query.Get("list-type") // Should be "2"

	// Get objects from S3Lite
	var objects []Object
	var commonPrefixes []CommonPrefix
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		log.Error(err)
		return
	}
	list := func(prefix string) {
		for key := range s3Lite.List(prefix) {

			// If this is a folder and we are in "ls" mode (there is a delimiter)
			if s.buckets.buckets.IsFolder(key) {
				// key = strings.TrimSuffix(key, "/")
				if key != "/" {
					commonPrefixes = append(commonPrefixes, CommonPrefix{Prefix: key})
				}
				continue
			}

			// Get object info
			info, err := s3Lite.GetInfo(key)
			if err != nil {
				log.Errorf("key: '%s', err: %v", key, err)
				continue
			}

			// Check if this is a multipart upload part
			if _, ok := info.Metadata["uploadId"]; ok {
				if _, ok := info.Metadata["partNumber"]; ok {
					continue
				}
			}

			// Append object to list
			objects = append(objects, Object{
				Key:          key,
				LastModified: S3Time(info.ModifiedAt),
				ETag:         "\"" + info.Checksum + "\"",
				Size:         info.ContentLength,
				StorageClass: "STANDARD",
			})
		}
	}
	list(prefix)

	// Process common prefixes for recursive mode
	var sortObjects bool
	for recursive && len(commonPrefixes) > 0 {
		list(commonPrefixes[0].Prefix)
		commonPrefixes = commonPrefixes[1:]
	}
	// Sort objects slice
	if sortObjects {
		sort.Slice(objects, func(i, j int) bool {
			return objects[i].Key < objects[j].Key
		})
	}

	// Prepare the XML response
	resp := ListBucketResultV2{
		Name:           bucketName,
		Prefix:         prefix,
		Delimiter:      delimiter,
		KeyCount:       len(objects),
		MaxKeys:        1000,
		IsTruncated:    false,
		Contents:       objects,
		CommonPrefixes: commonPrefixes,
	}

	// Write xml response
	xmlEncode(w, pretty, resp)
}

func xmlEncode(w http.ResponseWriter, pretty bool, resp any) error {

	// Write the XML header manually to avoid potential problems
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))

	// Create a new XML encoder with optional indentation
	enc := xml.NewEncoder(w)
	if pretty {
		enc.Indent("", "  ")
	}

	return enc.Encode(resp)
}
