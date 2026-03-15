// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package serve contains HTTP Server module of S3 like native Golang storage.
package serve

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/kirill-scherba/log"
	"golang.org/x/crypto/acme/autocert"
)

const (
	serverVersion = "0.0.1"

	patternVersion     = "/version"
	patternListBuckets = "/"
)

type Server struct {
	appShort string   // Application short name
	buckets  *Buckets // Buckets s3Lite database objects
	Users    *Users   // Users s3Lite database object

	*http.Server             // HTTP server
	ready        atomic.Bool // Wait HTTP server started or error flag.
}

func init() {}

func New(appShort, address, domain string) (s *Server, err error) {

	mux := http.NewServeMux()

	// Create new Buckets object
	buckets, err := newBackets(appShort)
	if err != nil {
		return
	}

	// Create new Users object
	users, err := newUsers(appShort)
	if err != nil {
		return
	}

	// Create a new HTTP server object
	s = &Server{
		appShort: appShort,
		Server:   &http.Server{Addr: address, Handler: mux},
		buckets:  buckets,
		Users:    users,
	}

	// Register HTTP handlers
	mux.HandleFunc(patternVersion, handleVersion)
	mux.Handle(patternListBuckets, s.authMiddleware(s.listBucketsHandler()))

	// Start HTTP server
	go func() {
		log.Infof("starting http server on %s", address)

		// Wait for HTTP server to start
		go s.waitStarted(address, domain)

		// Start HTTP server
		if domain == "" {
			// Start HTTP server if domain is empty
			err = s.ListenAndServe()
		} else {
			// Start HTTPS server on 443 port if domain is not empty
			err = s.Serve(autocert.NewListener(domain))
		}
		if err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf("can't start http server: %v", err)
			s.ready.Store(true)
			return
		}

		log.Infof("stop http server on %s", address)
	}()

	// Wait for HTTP server to start or error
	for !s.ready.Load() {
		// Empty loop body
	}
	if err != nil {
		return
	}

	log.Infof("http server started on %s", address)

	return
}

func (s *Server) Close() {
	s.Server.Close()
	s.Users.Close()
	s.buckets.buckets.Close()
}

// Wait for HTTP server to start
func (s *Server) waitStarted(address, domain string) {
	for !s.ready.Load() && !func() bool {
		client := http.Client{}

		// Make url to get server version
		var url string
		if domain == "" {
			url = "http://" + address + patternVersion
		} else {
			url = "https://" + domain + patternVersion
		}

		// Get version of server
		resp, err := client.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode == http.StatusOK {
			s.ready.Store(true)
			return true
		}

		return false
	}() {
		// Empty loop body
	}
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(serverVersion))
}
