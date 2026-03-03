# S3Lite

`S3Lite` is a native Golang storage system, similar to Amazon S3.

[![Go Report Card](https://goreportcard.com/badge/github.com/kirill-scherba/s3lite)](https://goreportcard.com/report/github.com/kirill-scherba/s3lite)
[![GoDoc](https://godoc.org/github.com/kirill-scherba/s3lite?status.svg)](https://godoc.org/github.com/kirill-scherba/s3lite/)

It allows to store and retrieve data by key.
Data is stored in a directory, with each key being a file.
The package provides functions to set, get, and delete data.
It also provides an iterator to traverse over all data in the storage.

## Installation

```bash
go get github.com/kirill-scherba/sqlh
```

## Usage

```go
package main

import (
    "github.com/kirill-scherba/s3lite"
)

func main() {
    // Create a new S3Lite instance
    s3lite, err := s3lite.New("./data", "my-bucket")
    if err != nil {
        panic(err)
    }

    // Add some data to the S3Lite instance with Set method
    err := s3lite.Set("key", []byte("value"))
    if err != nil {
        panic(err)
    }

    // Get the data from the S3Lite instance with Get method
    value, err := s3lite.Get("key")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(value))

    // Iterate over all keys with List method
    for key := range s3lite.List("") {
        fmt.Println(key)
    }

    // Delete the data from the S3Lite instance with Delete method
    err = s3lite.Delete("key")
    if err != nil {
        panic(err)
    }
}
```

## Licence

BSD
