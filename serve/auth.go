package serve

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/minio/minio-go/v7/pkg/signer"
)

// authRegex is a regular expression to extract Access Key from Authorization header
var authRegex = regexp.MustCompile(`Credential=([^/]+)/([^/]+)/([^/]+)/s3/aws4_request`)

// extractAccessKey gets Access Key from Authorization header.
func extractAccessKey(authHeader string) string {
	// Example: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260309/us-east-1/s3/aws4_request...
	matches := authRegex.FindStringSubmatch(authHeader)
	if len(matches) > 1 {
		return matches[1] // Return "AKIAIOSFODNN7EXAMPLE"
	}
	return ""
}

// authMiddleware provides authentication for all incoming requests.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendError(w, "AccessDenied", "Access Denied", http.StatusForbidden)
			return
		}

		// Extract Access Key from header (use extractAccessKey function we wrote earlier)
		accessKey := extractAccessKey(authHeader)

		// Get Secret Key from users database
		user, err := s.Users.GetByAccessKey(accessKey)
		if err != nil {
			sendError(w, "InvalidAccessKeyId",
				"The AWS Access Key Id you provided does not exist in our records.",
				http.StatusForbidden)
			return
		}

		// Very important part: Verify signature using MinIO package
		// We compare signature in request with that we generate ourselves
		if verify(r, accessKey, user.SecretKey) {
			sendError(w, "SignatureDoesNotMatch",
				"The request signature we calculated does not match the signature you provided.",
				http.StatusForbidden)
			return
		}

		// If everything matches, let request proceed
		next.ServeHTTP(w, r)
	})
}

// verify checks if the signature sent in the Authorization header matches the
// one we generate ourselves by re-signing the entire request with the Access
// Key and Secret Key. It returns true if the signatures match, and false
// otherwise.
func verify(r *http.Request, accessKey, secretKey string) bool {
	// Save the signature sent by the client (from the Authorization header)
	receivedAuth := r.Header.Get("Authorization")
	if receivedAuth == "" {
		return false
	}

	// Clear the Authorization header in the request before re-signing
	// (the SignV4 function itself will add new headers)
	r.Header.Del("Authorization")
	r.Header.Del("X-Amz-Signature")

	// 3. Use the SignV4 function from the signer package to create the "target" signature
	// We sign the entire request with the Access Key and Secret Key
	signedReq := signer.SignV4(*r, accessKey, secretKey, "", "us-east-1")

	// 4. Compare what we got with what the client sent
	expectedAuth := signedReq.Header.Get("Authorization")

	// Return the result of the comparison
	return receivedAuth == expectedAuth
}

// sendError writes an error response to the client in XML format.
// It takes four parameters: the http.ResponseWriter to write to, the error code,
// the error message, and the HTTP status code. It sets the Content-Type header
// to "application/xml", writes the status code to the client, and writes the
// error XML to the client.
func sendError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
    <Code>%s</Code>
    <Message>%s</Message>
    <Resource>%s</Resource>
</Error>`, code, message, "your-resource")
}
