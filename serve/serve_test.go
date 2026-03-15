// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package serve

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestServe(t *testing.T) {

	const appShort = "s3lite-server-test"

	t.Run("StartStop S3Lite HTTP server", func(t *testing.T) {
		s, err := New(appShort, "localhost:7080", "")
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		s.Close()
	})

	addr := "localhost:7080"
	endpoint := "http://" + addr

	// Start S3Lite HTTP server
	s, err := New(appShort, addr, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Add buckets
	t.Run("Add buckets to Server", func(t *testing.T) {

		bucketName := "bucket1"

		// Add bucket
		err = s.buckets.add(bucketName)
		if err != nil {
			// return
		}

		// Add objects to bucket
		//
		// Get bucket
		bucket, errGet := s.buckets.get(bucketName)
		if errGet != nil {
			err = errGet
			return
		}
		// Add object
		objectName := "key1"
		_, err = bucket.Set(objectName, []byte("value1"))
		if err != nil {
			err = fmt.Errorf("can't set object: %w", err)
			return
		}
		t.Logf("set object: '%s' to bucket: '%s'", objectName, bucketName)

		// Add object
		_, err = bucket.Set("key2", []byte("value2"))
		if err != nil {
			err = fmt.Errorf("can't set object: %w", err)
			return
		}

		err = s.buckets.add("bucket2")
		if err != nil {
			// return
		}

		err = nil
	})
	required(t, err)

	t.Run("List of buckets", func(t *testing.T) {

		// Get buckets list
		resp, err := http.Get(endpoint + patternListBuckets)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Fatal(resp.StatusCode)
		}

		// Check response body
		if resp.Header.Get("Content-Type") != "application/xml" {
			t.Fatal(resp.Header.Get("Content-Type"))
		}

		// Check response body
		// if resp.Header.Get("Content-Length") != "0" {
		// 	t.Fatal(resp.Header.Get("Content-Length"))
		// }

		// Check response body
		if resp.Header.Get("Date") == "" {
			t.Fatal(resp.Header.Get("Date"))
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(string(body))
		}
		t.Log(string(body))

	})

	// Sleep before Stop S3Lite HTTP server
	time.Sleep(200 * time.Millisecond)
}

func required(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
