package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/kirill-scherba/s3lite/serve"
)

const appShort = "s3lite-server-test"

func main() {

	// Start pprof server.
	// Run in browser: 
	//   http://localhost:6060/debug/pprof
	//   http://localhost:8080/proxy/6060/debug/pprof/goroutine?debug=1
	//
	//   curl http://localhost:6060/debug/pprof/trace?seconds=20 > trace.out
	//   go tool trace trace.out
	go func() {
		err := http.ListenAndServe("localhost:6060", nil)
		if err != nil {
			panic(err)
		}
	}()

	// Create new S3Lite HTTP server
	s, err := serve.New(appShort, "localhost:7080", "")
	if err != nil {
		panic(err)
	}

	// Set test user
	s.Users.Set(&serve.User{
		AccessKey: "ACCESS_KEY",
		SecretKey: "SECRET_ACCESS_KEY",
	})

	select {}
}
