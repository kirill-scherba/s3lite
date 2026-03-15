package serve

import (
	"encoding/json"
	"fmt"

	"github.com/kirill-scherba/s3lite"
)

// Users struct represents users database.
type Users struct {
	*s3lite.S3Lite
}

// User struct represents user.
type User struct {
	ID          string
	DisplayName string
	AccessKey   string
	SecretKey   string
}

// newUsers creates new Users object.
func newUsers(appShort string) (u *Users, err error) {

	// Create new Users object
	u = &Users{}

	// Get config path
	path, err := configPath(appShort)
	if err != nil {
		return
	}

	// Create new S3Lite object to store users
	u.S3Lite, err = s3lite.New(path, "$users")
	if err != nil {
		err = fmt.Errorf("can't create users object: %w", err)
	}

	return
}

// Set user store User object in s3lite.
func (u *Users) Set(user *User) (err error) {
	// Marshal user to json
	data, err := json.Marshal(user)
	if err != nil {
		err = fmt.Errorf("can't marshal user: %w", err)
		return
	}

	// Set user in s3lite
	_, err = u.S3Lite.Set(user.AccessKey, data)
	if err != nil {
		err = fmt.Errorf("can't set user: %w", err)
	}

	return
}

// GetByAccessKey gets user from s3lite by access key.
func (u *Users) GetByAccessKey(accessKey string) (user *User, err error) {

	// Get user by access key from s3lite
	data, err := u.S3Lite.Get(accessKey)
	if err != nil {
		err = fmt.Errorf("can't get user by access key: %w", err)
		return
	}

	// Unmarshal user data
	user = &User{}
	err = json.Unmarshal(data, user)
	if err != nil {
		err = fmt.Errorf("can't unmarshal user data: %w", err)
	}

	return
}
