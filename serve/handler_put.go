package serve

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/kirill-scherba/log"
	"github.com/kirill-scherba/s3lite"
)

// PUT /bucket/key
func (s *Server) putObjectHandler(w http.ResponseWriter, r *http.Request) {

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
		http.Error(w, "Invalid path", http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	// Set to S3Lite
	s3Lite, err := s.buckets.get(bucketName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	var inObjectInfo *s3lite.ObjectInfo
	if uploadId != "" && partNumber != "" {
		inObjectInfo = &s3lite.ObjectInfo{Metadata: map[string]string{
			"uploadId": uploadId, "partNumber": partNumber,
		}}
	}
	// fmt.Printf("metadata 0: %s => %v\n", key, inObjectInfo.Metadata)
	objectInfo, err := s3Lite.Set(key, body, inObjectInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	// fmt.Printf("Set %s => %v\n", key, objectInfo.Metadata)

	// Response with ETag and status 200
	w.Header().Set("ETag", "\""+objectInfo.Checksum+"\"")
	w.WriteHeader(http.StatusOK)
}

// POST /bucket/key?uploads
func (s *Server) initiateMultipartHandler(w http.ResponseWriter, r *http.Request) {

	// Log request
	log.Infof("InitiateMultipartHandler %s %s", r.Method, r.URL)

	// Parse path: /bucket/key
	bucket, key, err := parsePath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate random upload UUID
	uploadID := uuid.New().String()

	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
    <InitiateMultipartUploadResult>
      <Bucket>%s</Bucket>
      <Key>%s</Key>
      <UploadId>%s</UploadId>
    </InitiateMultipartUploadResult>`, bucket, key, uploadID)

	w.Header().Set("Content-Type", "application/xml")
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

	// Merge parts to single file
	ObjectInfo, err := s.mergeParts(uploadId, completeReq.Parts, bucket, key)
	if err != nil {
		http.Error(w, "Merge failed", 500)
		return
	}

	// Make response
	// ETag of the final file in S3 for Multipart is usually "hexdigest-checksum-N",
	// where N is the number of parts.
	finalETag := fmt.Sprintf(`"%s"`, ObjectInfo.Checksum)
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

func (s *Server) mergeParts(uploadId string, parts []Part, bucket, key string) (
	objectInfo *s3lite.ObjectInfo, err error) {

	// Get buckets s3Lite object
	s3Lite, err := s.buckets.get(bucket)
	if err != nil {
		return
	}

	// Prepare data
	var сontentLength int64
	outPath := "/" + bucket + "/" + key

	fmt.Printf("Merge %d parts to %s\n", len(parts), outPath)

	// Range parts and combine to data
	for _, p := range parts {
		partKey := partKey(key, uploadId, p.PartNumber)
		fmt.Printf("Part %d: %s => %s\n", p.PartNumber, p.ETag, partKey)

		// Get part info from S3Lite
		info, err := s3Lite.GetInfo(partKey)
		if err != nil {
			return nil, err
		}
		сontentLength += info.ContentLength
	}

	// Save data to S3Lite
	inObjectInfo := &s3lite.ObjectInfo{
		ContentLength: сontentLength,
		Checksum:      сalculateMultipartETag(parts),
		Metadata: map[string]string{
			"uploadId": uploadId, "numParts": fmt.Sprintf("%d", len(parts)),
		},
	}
	objectInfo, err = s3Lite.Set(key, []byte(""), inObjectInfo)
	if err != nil {
		log.Errorf("Error saving %s(%d bytes): %v", key, сontentLength, err)
		return
	}

	// Set content length
	objectInfo.ContentLength = сontentLength
	s3Lite.SetInfo(key, objectInfo)

	return
}

func сalculateMultipartETag(parts []Part) string {
	var combinedBinaryMD5 []byte

	for _, p := range parts {
		// Удаляем кавычки, если они есть
		cleanETag := strings.ReplaceAll(p.ETag, `"`, "")

		// Переводим HEX-строку (хеш части) в бинарный вид
		binaryTag, _ := hex.DecodeString(cleanETag)
		combinedBinaryMD5 = append(combinedBinaryMD5, binaryTag...)
	}

	// Считаем MD5 от склеенных байтов
	finalHash := md5.Sum(combinedBinaryMD5)

	// Формируем финальную строку: hex(hash)-N
	return fmt.Sprintf(`"%x-%d"`, finalHash, len(parts))
}

func parsePath(r *http.Request) (bucketName, key string, err error) {
	// Parse path: /bucket/key
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		err = fmt.Errorf("invalid path")
		return
	}
	bucketName, key = parts[0], parts[1]
	return
}
