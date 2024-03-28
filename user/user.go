// Package user manipulates the users directory and provides a simple API for
// the endpoints
package user

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// All user data is stored in a directory. Each user has a file named after
// their username. Username has to be [A-Za-z0-9_-]+

type UserStore struct {
	// path of the users directory
	path string
}

func NewUserStore(path string) (us UserStore, err error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		err = fmt.Errorf("users directory has to be absolute path (is: %v)", path)
	} else {
		us = UserStore{path: path}
	}
	return
}

var usernameRegex = regexp.MustCompile("^[A-Za-z0-9_-]+$")

func usernameIsSane(username string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username does not match the usernameRegex (is %v)", username)
	}
	return nil
}

func (fm *UserStore) Get(username string) (pwd [64]byte, err error) {
	if err = usernameIsSane(username); err != nil {
		return
	}
	filename := filepath.Join(fm.path, username)
	content, err := os.ReadFile(filename) // #nosec G304: fm.path is trusted, username matches aggressive regex
	if len(content) != 64 {
		err = fmt.Errorf("corrupt user data (file %v)", filename)
		return
	}
	if err == nil {
		copy(pwd[:], content)
	}
	return
}

func (fm *UserStore) Set(username string, pwd [64]byte) error {
	if err := usernameIsSane(username); err != nil {
		return err
	}
	filename := filepath.Join(fm.path, username)
	return os.WriteFile(filename, pwd[:], 0600)
}

func (us UserStore) Delete(name string) error {
	// TODO: GC user files here?
	filename := filepath.Join(us.path, name)
	return os.Remove(filename)
}
