package serve

import (
	"net/http"
	"strings"

	"github.com/kirill-scherba/log"
)

// routeHandler returns a http.Handler that routes incoming requests to the
// appropriate handler.
//
// The handler checks the path and method of the incoming request and calls the
// appropriate handler.
// The handler checks for the following:
//   - If the path is empty, call listBucketsHandler.
//   - If the path has one part, call listObjectsHandler, checkBucketHandler,
//     addBucketHandler, or deleteBucketHandler depending on the method.
//   - If the path has more than one part, call getObjectHandler, putObjectHandler,
//     or deleteObjectHandler depending on the method.
//   - If the request is invalid, write an error response with a status code of 400.
func (s *Server) routeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Trim leading and trailing slashes
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")

		var (
			err               error
			errInvalidRequest S3Error = S3Error{
				Code:       "InvalidRequest",
				Message:    "invalid request",
				HTTPStatus: http.StatusBadRequest,
			}
		)

		switch {
		case path == "":
			s.listBucketsHandler(w, r)
		case len(parts) == 1:
			switch r.Method {
			case http.MethodGet:
				s.listObjectsHandler(w, r)
			case http.MethodHead:
				s.checkBucketHandler(w, r)
			case http.MethodPut:
				s.addBucketHandler(w, r)
			case http.MethodDelete:
				s.deleteBucketHandler(w, r)
			default:
				err = errInvalidRequest
			}
		case len(parts) > 1:
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				s.getObjectHandler(w, r)
			case http.MethodPut, http.MethodPost:
				s.putObjectHandler(w, r)
			case http.MethodDelete:
				s.deleteObjectHandler(w, r)
			default:
				err = errInvalidRequest
			}
		default:
			err = errInvalidRequest
		}

		// Check error
		if err != nil {
			s.WriteError(w, r, err)
			log.Errorf("\033[91m%s\033[0m\n", err)
		}
	})
}
