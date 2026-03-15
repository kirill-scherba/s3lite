package main

import "github.com/kirill-scherba/s3lite/serve"

const appShort = "s3lite-server-test"

func main() {

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
