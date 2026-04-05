package serve

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kirill-scherba/log"
	"github.com/kirill-scherba/s3lite"
)

// PUT /bucket/key
func (s *Server) putObjectHandler(w http.ResponseWriter, r *http.Request) {

	// Process error at return
	var err error
	defer func() {
		if err != nil {
			s.WriteError(w, r, err)
			log.Debug(err)
		}
	}()

	// Check multipart upload init
	uploads := r.URL.Query().Has("uploads")
	if uploads && r.Header.Get("X-Amz-Metadata-Directive") == "" {
		s.initiateMultipartHandler(w, r)
		return
	}

	// Check multipart upload complete
	uploadId := r.URL.Query().Get("uploadId")
	if uploadId != "" && r.Method == http.MethodPost {
		s.completeMultipartHandler(w, r)
		return
	}

	// Log request
	log.Infof("PutObjectHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket/key
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		err = S3Error{"InvalidURI", "Could not parse the specified URI.",
			http.StatusBadRequest}
		return
	}
	bucketName, key := parts[0], parts[1]

	// Get part number for multipart upload
	partNumber := r.URL.Query().Get("partNumber")
	if uploadId != "" && partNumber != "" {
		key += "." + uploadId + "." + partNumber
	}

	// Get body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err = S3Error{"ReadBodyError", "Could not read the specified body: " +
			err.Error(), http.StatusInternalServerError}
		return
	}

	// Get buckets s3Lite object
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		err = S3Error{"NoSuchBucket", "The specified bucket does not exist: " +
			err.Error(), http.StatusNotFound}
		return
	}

	// Get metadata from headers
	metadata := getMetadata(r.Header, nil)

	// If an empty content is received
	if len(body) == 0 {
		log.Debugf("%s empty body, url: %s\n", r.Method, r.URL)

		// Get info
		info, errInfo := s3Lite.GetInfo(key)
		if errInfo != nil {
			info = &s3lite.ObjectInfo{}
		}

		// Set metadata if empty
		if info.Metadata == nil {
			info.Metadata = map[string]string{}
		}

		// Send response for MultipartUpload part and empty body
		if uploadId != "" {
			etag := info.Checksum // etag from empty body: "d41d8cd98f00b204e9800998ecf8427e"
			log.Debugf("Send MultipartUpload part result, url: %s, etag: %s, empty body request", r.URL, etag)
			copyPartResult(w, etag)
			return
		}

		// Set metadata to info and update it
		if len(metadata) > 0 {

			// Copy metadata
			maps.Copy(info.Metadata, metadata)

			// Set content type
			contentType := r.Header.Get("Content-Type")
			if contentType != "" {
				info.ContentType = contentType
			}

			// Set info to S3Lite
			_, err = s3Lite.SetInfo(key, info)
			if err != nil {
				err = S3Error{"UpdateMetadataError", "Can't update metadata: " +
					err.Error(), http.StatusInternalServerError}
				return
			}

			log.Debugf("Set info '%s': %v", key, info)
		}

		// Send response for request MultipartUpload with ?uploads= and empty body
		if uploads {
			log.Debugf("Send MultipartUpload result, empty body request, url: %s", r.URL)
			initiateMultipartUploadResult(w, bucketName, key, info.Metadata["uploadId"])
			return
		}

		// Check if it's not a special s3fs request
		// It's better to simply return 200 OK without updating the old file
		// if it's just an attempt to "update the time"
		w.WriteHeader(http.StatusOK)
		return
	}

	// Prepare objectInfo and copy metodata
	var info = &s3lite.ObjectInfo{Metadata: map[string]string{}}
	if uploadId != "" && partNumber != "" {
		info.Metadata["uploadId"] = uploadId
		info.Metadata["partNumber"] = partNumber
	}
	info.ContentType = r.Header.Get("Content-Type")
	maps.Copy(info.Metadata, metadata)

	// Set to S3Lite
	objectInfo, err := s3Lite.Set(key, body, info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	// Response with ETag and status 200
	w.Header().Set("ETag", "\""+objectInfo.Checksum+"\"")
	w.WriteHeader(http.StatusOK)
}

func copyPartResult(w http.ResponseWriter, etag string) {

	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
    <CopyPartResult>
      <LastModified>%s</LastModified>
      <ETag>"%s"</ETag>
    </CopyPartResult>`, time.Now().Format(time.RFC3339), etag)

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

// initiateMultipartHandler initiate multipart upload at POST /bucket/key?uploads
func (s *Server) initiateMultipartHandler(w http.ResponseWriter, r *http.Request) {

	// Log request
	log.Infof("InitiateMultipartHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket/key
	bucket, key, err := parsePath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get buckets s3Lite object
	s3Lite, err := s.buckets.get(bucket)
	if err != nil {
		return
	}

	// Get input metadata
	metodata := getMetadata(r.Header, nil)

	// Generate random upload UUID
	uploadID := uuid.New().String()

	// Get object info if exists
	objectInfo, err := s3Lite.GetInfo(key)
	if err != nil {
		objectInfo = &s3lite.ObjectInfo{Metadata: map[string]string{}}
	}
	if id := objectInfo.Metadata["uploadId"]; id != "" {
		uploadID = id
	}

	contentType := r.Header.Get("Content-Type")
	objectInfo.ContentType = contentType

	// Set metadata
	objectInfo.Metadata["uploadId"] = uploadID
	maps.Copy(objectInfo.Metadata, metodata)
	_, err = s3Lite.SetInfo(key, objectInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	initiateMultipartUploadResult(w, bucket, key, uploadID)
}

func initiateMultipartUploadResult(w http.ResponseWriter, bucket, key,
	uploadID string) {

	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
    <InitiateMultipartUploadResult>
      <Bucket>%s</Bucket>
      <Key>%s</Key>
      <UploadId>%s</UploadId>
    </InitiateMultipartUploadResult>`, bucket, key, uploadID)

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

type CompleteMultipartUpload struct {
	Parts []Part `xml:"Part"`
}
type Part struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// POST /bucket/key?uploadId=UUID
func (s *Server) completeMultipartHandler(w http.ResponseWriter, r *http.Request) {

	// Log request
	log.Infof("CompleteMultipartHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket/key
	bucket, key, err := parsePath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get upload ID
	uploadId := r.URL.Query().Get("uploadId")

	// Decode request body
	var completeReq CompleteMultipartUpload
	if err := xml.NewDecoder(r.Body).Decode(&completeReq); err != nil {
		http.Error(w, "Bad XML", http.StatusBadRequest)
		return
	}

	log.Debugf("CompleteMultipartUpload: %v", completeReq)

	// Merge parts to single file
	var objectInfo = &s3lite.ObjectInfo{}
	if len(completeReq.Parts) > 1 {
		objectInfo, err = s.mergeParts(uploadId, completeReq.Parts, bucket, key)
		if err != nil {
			http.Error(w, "Merge failed", 500)
			return
		}
	}

	// Make response
	// ETag of the final file in S3 for Multipart is usually "hexdigest-checksum-N",
	// where N is the number of parts.
	finalETag := fmt.Sprintf(`"%s"`, objectInfo.Checksum)
	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
    <CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
      <Location>http://%s%s</Location>
      <Bucket>%s</Bucket>
      <Key>%s</Key>
      <ETag>%s</ETag>
    </CompleteMultipartUploadResult>`, r.Host, r.URL.Path, bucket, key, finalETag)

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(resp))
}

// mergeParts merge parts to single file.
func (s *Server) mergeParts(uploadId string, parts []Part, bucket, key string) (
	info *s3lite.ObjectInfo, err error) {

	// Get buckets s3Lite object
	s3Lite, err := s.buckets.get(bucket)
	if err != nil {
		return
	}

	// Prepare data
	var сontentLength int64

	// Range parts and combine to data
	for _, p := range parts {
		partKey := partKey(key, uploadId, p.PartNumber)

		// Get part info from S3Lite
		info, err := s3Lite.GetInfo(partKey)
		if err != nil {
			return nil, err
		}
		сontentLength += info.ContentLength
	}

	// Get object info
	info, err = s3Lite.GetInfo(key)
	if err != nil {
		info = &s3lite.ObjectInfo{Metadata: make(map[string]string)}
	}
	// fmt.Println(info)

	// Save data to S3Lite
	info.ContentLength = сontentLength
	info.Checksum = calculateMultipartETag(parts)
	info.Metadata["uploadId"] = uploadId
	info.Metadata["numParts"] = fmt.Sprintf("%d", len(parts))

	info, err = s3Lite.Set(key, []byte(""), info)
	if err != nil {
		log.Errorf("Error saving %s(%d bytes): %v", key, сontentLength, err)
		return
	}

	// Set content length
	info.Checksum = calculateMultipartETag(parts)
	info.ContentLength = сontentLength
	s3Lite.SetInfo(key, info)

	return
}

// calculateMultipartETag calculates Multipart ETag.
func calculateMultipartETag(parts []Part) string {
	var combinedBinaryMD5 []byte

	for _, p := range parts {
		// Remove quotes if they exist
		cleanETag := strings.ReplaceAll(p.ETag, `"`, "")

		// Convert HEX string (hash part) to binary
		binaryTag, _ := hex.DecodeString(cleanETag)
		combinedBinaryMD5 = append(combinedBinaryMD5, binaryTag...)
	}

	// Calculate MD5 of combined bytes
	finalHash := md5.Sum(combinedBinaryMD5)

	// Form final string: hex(hash)-N
	return fmt.Sprintf(`"%x-%d"`, finalHash, len(parts))
}

// parsePath parse path: /bucket/key from request.
func parsePath(r *http.Request) (bucketName, key string, err error) {
	// Parse path: /bucket/key
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		err = ErrInvalidURI
		return
	}
	bucketName, key = parts[0], parts[1]
	return
}

func getMetadata(header http.Header, metadata map[string]string) map[string]string {

	if metadata == nil {
		metadata = make(map[string]string)
	}

	for h := range header {
		if strings.HasPrefix(h, "X-Amz-Meta-") {
			key := strings.ToLower(h)
			metadata[key] = header.Get(h)
		}
	}

	return metadata
}

func printMetadata(key string, metodata map[string]string) {

	// Show all metodata
	var str strings.Builder
	fmt.Fprintf(&str, "Metadata %s:\n", key)
	for h := range metodata {
		fmt.Fprintf(&str, "- %s: %s\n", h, metodata[h])
	}
	str.WriteString("\n")

	fmt.Print(str.String())
}
