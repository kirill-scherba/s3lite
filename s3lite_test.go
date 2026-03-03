// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package s3lite

import (
	"testing"
)

func init() {

}

func TestS3(t *testing.T) {

	var s3 *S3Lite
	defer func() { s3.Close() }()

	type keyValue struct {
		key   string
		value []byte
	}

	t.Run("New", func(t *testing.T) {

		var err error
		s3, err = New("", "")
		if err != nil {
			t.Fatal(err)
		}

		t.Log("connect to s3 storage,", s3)

	})

	t.Run("Set and Get", func(t *testing.T) {

		var keys = []keyValue{
			{"key1", []byte("value1")},
			{"key2", []byte("value2")},
			{"key3", []byte("value3")},
		}

		for _, k := range keys {
			// Set to s3
			err := s3.Set(k.key, k.value)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("set to s3 storage,", k.key)
		}

		for _, k := range keys {
			// Get from s3
			data, err := s3.Get(k.key)
			if err != nil {
				t.Fatal(err)
			}
			require(t, string(k.value), string(data))
			t.Log("get from s3 storage,", k.key, string(data))
		}

	})

	t.Run("List", func(t *testing.T) {
		for key := range s3.List("") {
			t.Logf("get key: %s", key)
		}
	})

	t.Run("List folder", func(t *testing.T) {

		folder := "test/"
		subfolder := folder + "subfolder/"

		// Create folders objects
		var keys = []keyValue{
			{folder + "key1", []byte("value1")},
			{folder + "key2", []byte("value2")},
			{folder + "key3", []byte("value3")},
		}

		// Create subfolders objects
		var keys2 = []keyValue{
			{subfolder + "key1", []byte("value1")},
			{subfolder + "key2", []byte("value2")},
			{subfolder + "key3", []byte("value3")},
		}

		keys = append(keys, keys2...)

		// Add some new objects to root folder
		keys = append(keys,
			keyValue{"new key4", []byte("value4")},
			keyValue{"a new key5", []byte("value5")},
			keyValue{"afolder/key6", []byte("value6")},
			keyValue{"afolder/key7", []byte("value7")},
		)
		for _, k := range keys {
			// Set to s3
			err := s3.Set(k.key, k.value)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("set to s3 storage,", k.key)
		}

		const color = "\033[1;32m%s '%s'\033[0m"

		// List root
		t.Logf(color, "List root:", "")
		for key := range s3.List("") {
			t.Logf("root key: %s", key)
		}

		// List folder
		t.Logf(color, "List folder:", folder)
		for key := range s3.List(folder) {
			t.Logf("folder key: %s", key)
		}

		// List subfolder
		t.Logf(color, "List subfolder:", subfolder)
		for key := range s3.List(subfolder) {
			t.Logf("subfolder key: %s", key)
		}
	})

	t.Run("Dir", func(t *testing.T) {
		dir := s3.Dir("test/key1")
		t.Logf("dir: %s", dir)
		require(t, "test", dir)

		dir = s3.Dir("test/folder/key1")
		t.Logf("dir: %s", dir)
		require(t, "test/folder", dir)

		dir = s3.Dir("key1")
		t.Logf("dir: %s", dir)
		require(t, "", dir)
	})
}

func require[T comparable](t *testing.T, expected, actual T) {
	if expected != actual {
		t.Fatal("expected:", expected, "actual:", actual)
	}
}
