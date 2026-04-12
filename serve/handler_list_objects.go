package serve

import (
	"encoding/xml"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kirill-scherba/log"
	"github.com/kirill-scherba/s3lite"
)

// ListBucketResultV2 — the root element for the ListObjectsV2 response
type ListBucketResultV2 struct {
	XMLName               xml.Name       `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name                  string         `xml:"Name"`                            // Bucket name
	Prefix                string         `xml:"Prefix"`                          // Filter (prefix)
	Delimiter             string         `xml:"Delimiter"`                       // Filter (delimiter)
	KeyCount              int            `xml:"KeyCount"`                        // Number of objects in response
	MaxKeys               int            `xml:"MaxKeys"`                         // Limit
	IsTruncated           bool           `xml:"IsTruncated"`                     // Is there another page?
	Contents              []Object       `xml:"Contents"`                        // List of files
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`        // Folders
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`     // Continuation token
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"` // Continuation token
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

// listObjectsHandler is a handler for GET /bucket requests. It returns a list of all objects in the specified bucket.
// The handler takes several parameters: "prefix", "delimiter", "max-keys", and "pretty".
// "prefix" is a filter parameter that limits the list of objects to those with keys
// starting with the specified prefix. "delimiter" is a filter parameter that limits the
// list of objects to those with keys ending with the specified delimiter.
// If the delimiter is an empty string, the handler will list all objects in the bucket,
// without filtering by prefix.
// "max-keys" is a parameter that limits the number of objects returned in the response.
// If the parameter is set to 0, the handler will return all objects in the bucket.
// "pretty" is a parameter that formats the XML response with indentation and line breaks.
// If the parameter is set to "true", the handler will format the XML response with indentation and line breaks.
// Otherwise, the handler will format the XML response with no indentation or line breaks.
// The handler returns an error if the specified bucket does not exist, or if the specified parameters are invalid.
// The handler logs the request, gets the list of objects from S3Lite, and writes the list to the HTTP response writer in XML format.
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
	maxKeys, _ := strconv.Atoi(query.Get("max-keys"))
	// marker := query.Get("marker")
	// listType := query.Get("list-type") // Should be "2"

	// Get objects from S3Lite
	var objects []Object
	var objectsMut sync.Mutex
	var commonPrefixes []CommonPrefix
	var keysToRemove = make(map[string]struct{})
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		log.Error(err)
		return
	}
	list := func(prefix string, maxKeys int) {
		var i int

		var wg sync.WaitGroup
		for key := range s3Lite.List(prefix) {

			// If this is a folder and we are in "ls" mode (there is a delimiter)
			if s3Lite.IsFolder(key) {
				if key != "/" {
					commonPrefixes = append(commonPrefixes, CommonPrefix{Prefix: key})
				}
				continue
			}

			// wg.Go(
			func() {
				// Get object info
				info, err := s3Lite.GetInfo(key)
				if err != nil {
					log.Errorf("key: '%s', err: %v", key, err)
					return // continue
				}

				// Check if this is a multipart upload part
				uploadId := info.Metadata["uploadId"]
				partNumber := info.Metadata["partNumber"]
				if uploadId != "" && partNumber != "" {

					// Get parent key for multipart upload part and check parent info
					parentKey := strings.TrimSuffix(key, "."+uploadId+"."+partNumber)
					info, err = s3Lite.GetInfo(parentKey)
					if !(err == nil && info.Metadata["uploadId"] != "") {
						log.Debugf("\033[31mLost multipart upload part: %s\033[0m", key)
						keysToRemove[key] = struct{}{}
					}

					return // continue
				}

				// Append object to list
				objectsMut.Lock()
				defer objectsMut.Unlock()
				objects = append(objects, Object{
					Key:          key,
					LastModified: S3Time(info.ModifiedAt),
					ETag:         "\"" + info.Checksum + "\"",
					Size:         info.ContentLength,
					StorageClass: "STANDARD",
				})
			}()
			// )

			// Check max keys
			i++
			if maxKeys > 0 && i >= maxKeys {
				break
			}
		}
		wg.Wait()
	}
	list(prefix, maxKeys)

	// Process common prefixes for recursive mode
	var sortObjects bool
	for recursive && len(commonPrefixes) > 0 {
		list(commonPrefixes[0].Prefix, maxKeys)
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

	// Remove lost multipart keys
	removeKeysMap(s3Lite, keysToRemove)

	// Write xml response
	xmlEncode(w, pretty, resp)
}

// xmlEncode writes the given response to the HTTP response writer in XML format.
// If pretty is true, the XML response will be formatted with indentation and line breaks.
// Otherwise, the XML response will be formatted with no indentation or line breaks.
// The function returns an error if the XML encoding fails.
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

// removeKeysMap removes all keys from the given map from the S3Lite storage.
// The function takes a pointer to the S3Lite object and a map of keys to delete.
// It returns an error if any of the keys do not exist in the S3Lite storage.
func removeKeysMap(s3Lite *s3lite.S3Lite, keysMap map[string]struct{}) (err error) {
	var keys []string
	for key := range keysMap {
		keys = append(keys, key)
	}
	s3Lite.Del(keys...)
	return
}
