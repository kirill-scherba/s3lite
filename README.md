# S3Lite

[![Go Report Card](https://goreportcard.com/badge/github.com/kirill-scherba/s3lite)](https://goreportcard.com/report/github.com/kirill-scherba/s3lite)
[![GoDoc](https://godoc.org/github.com/kirill-scherba/s3lite?status.svg)](https://godoc.org/github.com/kirill-scherba/s3lite/)

`S3Lite` is a lightweight Go key-value storage system, similar to Amazon S3. It uses [BadgerDB](https://github.com/dgraph-io/badger) as the embedded database backend.

## Features

- **Single-process mode** — direct BadgerDB access via `s3lite.New()`
- **Multi-process mode** — share the same bucket between multiple processes on one machine via Unix sockets (`multy` package)
- **Auto-takeover** — if the server process dies, a client automatically acquires the Badger lock and becomes the new server
- **Object metadata** — each key can store `ObjectInfo` (content-type, checksum, creation/modification timestamps)
- **Hierarchical listing** — `List(prefix)` shows one directory level at a time (grouped by `/`)
- **In-memory mode** — pass `/` as path to use in-memory Badger
- **HTTP server** — optional S3-compatible HTTP server (`serve` package)

## Installation

```bash
go get github.com/kirill-scherba/s3lite
```

## Quick Start (Single-process)

```go
package main

import (
    "fmt"
    "github.com/kirill-scherba/s3lite"
)

func main() {
    // Create a new S3Lite instance
    store, err := s3lite.New("./data", "my-bucket")
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // Set a value
    if err := store.Set("greeting", []byte("Hello, S3Lite!")); err != nil {
        panic(err)
    }

    // Get the value
    value, err := store.Get("greeting")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(value))

    // Iterate over keys with a prefix
    for key := range store.List("") {
        fmt.Println("key:", key)
    }

    // Delete the key
    if err := store.Del("greeting"); err != nil {
        panic(err)
    }
}
```

## Object Metadata

Each key can store metadata alongside its value:

```go
info := &s3lite.ObjectInfo{
    ContentType: "application/json",
    Checksum:    []byte("..."),
}
if err := store.SetInfo("config.json", info); err != nil {
    panic(err)
}

retrieved, err := store.GetInfo("config.json")
if err != nil {
    panic(err)
}
fmt.Println("Content-Type:", retrieved.ContentType)
fmt.Printf("Created: %s\n", retrieved.CreatedAt)
fmt.Printf("Modified: %s\n", retrieved.ModifiedAt)
```

## Multi-process Mode (multy)

Share the same bucket between multiple processes on the same machine. The first process opens Badger and becomes the Unix socket server. Other processes connect as clients. If the server dies, a client automatically takes over.

```go
package main

import (
    "fmt"
    "github.com/kirill-scherba/s3lite"
    "github.com/kirill-scherba/s3lite/multy"
)

func main() {
    // Use multy.New() instead of s3lite.New()
    // First caller becomes the server, others become clients
    store, err := multy.New("./data", "shared-bucket")
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // Same interface as s3lite.New()
    store.Set("key", []byte("shared value"))
    val, _ := store.Get("key")
    fmt.Println(string(val))
}
```

**How it works:**
1. `multy.New()` tries to open Badger directly (first-wins)
2. If Badger is already locked, it connects to the existing server via a Unix socket at `/tmp/s3lite-<bucket>.sock`
3. A health-check goroutine monitors the connection every second
4. If the server dies, the client re-opens Badger and starts its own server

## Using the KeyValueStore Interface

Define functions that accept `s3lite.KeyValueStore` to work with any backend:

```go
func printKeys(store s3lite.KeyValueStore, prefix string) {
    for key := range store.List(prefix) {
        fmt.Println(key)
    }
}

// Works with both:
printKeys(singleStore, "users/")
printKeys(multiStore, "users/")
```

## In-Memory Mode

Pass `/` as the database path to use an in-memory Badger backend (data is lost on close):

```go
store, _ := s3lite.New("/", "ephemeral")
defer store.Close()
```

## HTTP S3 Server

The `serve` package provides an S3-compatible HTTP server. Build and run the standalone server:

```bash
go run github.com/kirill-scherba/s3lite/cmd/server
```

## API Overview

### s3lite.KeyValueStore Interface

| Method | Description |
|--------|-------------|
| `Set(key string, value []byte) error` | Store a value |
| `Get(key string) ([]byte, error)` | Retrieve a value |
| `Del(keys ...string) error` | Delete one or more keys |
| `List(prefix string) iter.Seq[string]` | Iterate keys with prefix (hierarchical) |
| `Count(prefix string) int` | Count keys with prefix |
| `GetInfo(key string) (*ObjectInfo, error)` | Get metadata |
| `SetInfo(key string, info *ObjectInfo) error` | Set metadata |
| `Close() error` | Close the storage |

### ObjectInfo

| Field | Type | Description |
|-------|------|-------------|
| `ContentType` | `string` | MIME type |
| `Checksum` | `[]byte` | MD5 or other checksum |
| `CreatedAt` | `time.Time` | Creation timestamp |
| `ModifiedAt` | `time.Time` | Last modification timestamp |

## Package Structure

| Package | Path | Role |
|---------|------|------|
| `s3lite` | `./` | Core BadgerDB wrapper, `KeyValueStore` interface, `ObjectInfo` |
| `multy` | `./multy/` | Multi-process mode via Unix sockets |
| `serve` | `./serve/` | HTTP S3-compatible server |
| `cmd/server` | `./cmd/server/` | Standalone S3 HTTP server binary |

## Requirements

- Go 1.23+ (uses `iter.Seq`)
- BadgerDB v3

## License

BSD
