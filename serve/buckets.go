package serve

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kirill-scherba/s3lite"
)

type Buckets struct {
	appShort string                    // Application short name
	buckets  *s3lite.S3Lite            // Buckets S3Lite instance with all buckets info
	m        map[string]*s3lite.S3Lite // Buckets map
	mut      sync.RWMutex
}

// newBackets creates a new Buckets object.
//
// Parameters:
//   - appShort: Application short name.
//
// Returns:
//   - b: new Buckets object.
//   - error: an error if there was an error creating the Buckets object.
//
// The function creates a new Buckets object with a map of buckets and a S3Lite
// object to store buckets info. The S3Lite object is created with a path to
// the config directory and the "$buckets" bucket name. If there was an error
// creating the S3Lite object, the error is returned.
func newBackets(appShort string) (b *Buckets, err error) {

	// Create new Buckets object
	b = &Buckets{appShort: appShort, m: make(map[string]*s3lite.S3Lite)}

	// Get config path
	path, err := configPath(appShort)
	if err != nil {
		return
	}

	// Create new S3Lite object to store buckets
	b.buckets, err = s3lite.New(path, "$buckets")
	if err != nil {
		err = fmt.Errorf("can't create buckets object: %w", err)
	}

	return
}

// add creates a new bucket.
//
// Parameters:
//   - name: the name of the bucket to create.
//
// Returns:
//   - error: an error if the bucket already exists or if there was an error creating the bucket.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
//
// err = s.AddBucket("my-bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
func (s *Buckets) add(name string) (err error) {

	// Check if bucket exists
	if _, err = s.buckets.GetInfo(name); err == nil {
		err = fmt.Errorf("bucket '%s' already exists", name)
		return
	}
	if err != s3lite.ErrKeyNotFound {
		err = fmt.Errorf("can't check bucket %s: %w", name, err)
		return
	}

	// Create new bucket
	_, err = s.buckets.Set(name, []byte{})

	return
}

// Get retrieves a S3Lite object by its bucket name.
//
// Parameters:
//   - bucketName: the name of the bucket to retrieve.
//
// Returns:
//   - s3Lite: the S3Lite object associated with the bucket.
//   - error: an error if the bucket doesn't exist or if there was an error retrieving the bucket.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
//
// s3Lite, err := s.Get("my-bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
func (s *Buckets) get(bucketName string) (s3Lite *s3lite.S3Lite, err error) {

	// Get bucket info to check if it exists
	if _, err = s.buckets.Get(bucketName); err != nil {
		err = fmt.Errorf("bucket %s doesn't exists", bucketName)
		return
	}

	// Get config path
	path, err := configPath(s.appShort)
	if err != nil {
		return
	}

	// Get bucket from map
	s.mut.Lock()
	defer s.mut.Unlock()
	s3Lite, ok := s.m[bucketName]
	if !ok {
		// Open new s3lite object
		s3Lite, err = s3lite.New(path, bucketName)
		if err != nil {
			err = fmt.Errorf("can't create bucket %s: %w", bucketName, err)
			return
		}
		s.m[bucketName] = s3Lite
	}

	return
}

// Delete removes a bucket from the database.
//
// Parameters:
//   - name: the name of the bucket to remove.
//
// Returns:
//   - error: an error if the bucket doesn't exist or if there was an error removing the bucket.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
//
// err = s.Delete("my-bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
func (s *Buckets) delete(name string) (err error) {

	// Check if bucket exists
	if _, err = s.buckets.GetInfo(name); err != nil {
		err = fmt.Errorf("can't remove bucket %s: %w", name, err)
		return
	}

	// Remove bucket
	err = s.buckets.Del(name)

	return
}

// List returns a list of all buckets.
//
// The list includes the name and creation date of each bucket.
//
// The returned list is sorted alphabetically by bucket name.
//
// If there is an error while retrieving the list, an error is returned.
//
// Example:
// s, err := s3.New("/path/to/db", "bucket")
//
//	if err != nil {
//			log.Fatal(err)
//		}
//
// buckets, err := s.List()
//
//	if err != nil {
//			log.Fatal(err)
//		}
func (s *Buckets) list() (buckets []Bucket, err error) {
	for key := range s.buckets.List("") {

		// Set bucket name
		bucket := Bucket{
			Name:         key,
			CreationDate: S3Time(time.Now()),
		}

		// Set bucket creation date
		if info, err := s.buckets.GetInfo(key); err == nil {
			bucket.CreationDate = S3Time(info.CreatedAt)
		}

		// Append bucket
		buckets = append(buckets, bucket)
	}
	return
}

// configPath returns the path to the configuration directory for the given application.
//
// The path is constructed by appending the application short name to the user configuration directory.
//
// If there was an error while retrieving the user configuration directory, an error is returned.
//
// Example:
// path, err := configPath("my-app")
//
//	if err != nil {
//			log.Fatal(err)
//		}
//
// fmt.Printf("Configuration directory path: %s\n", path)
func configPath(appShort string) (path string, err error) {
	// Get os config path
	path, err = os.UserConfigDir()
	if err != nil {
		err = fmt.Errorf("can't get user config dir: %w", err)
		return
	}
	path += "/" + appShort

	return
}
