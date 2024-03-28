package main

import (
	"archiiv/user"
	"net/http"
	"time"
)

func getSessionToken(r *http.Request) string {
	return r.Header.Get("Authorization")
}

func getUsername(r *http.Request, secret string) string {
	// This function is only called in endpoints wrapped around
	// `requireLogin` middleware so this function can assume that some user
	// is logged in
	token := getSessionToken(r)
	username, err := verifySignature(token, secret, 7*24*time.Hour)
	if err != nil {
		panic(err)
	}
	return username
}

func validateToken(secret, token string) bool {
	_, err := verifySignature(token, secret, 7*24*time.Hour)
	return err == nil
}

func login(name string, pwd [64]byte, secret string, userStore user.UserStore) (ok bool, token string) {
	correctPwd, err := userStore.Get(name)
	if err != nil {
		ok = false
		return
	}

	if correctPwd != pwd {
		ok = false
		return
	}

	token, err = sign(name, secret)
	if err != nil {
		ok = false
		return
	}

	ok = true
	return
}
