package main

import (
	"archiiv/fs"
	"log/slog"
	"net/http"
)

func addRoutes(
	mux *http.ServeMux,
	log *slog.Logger,
	secret string,
	userStore userStore,
	fileStore *fs.Fs,
) {
	mux.Handle("GET /api/v1/ls/{id}", requireLogin(secret, log, handleLs(fileStore, log)))
	mux.Handle("GET /api/v1/cat/{id}/{section}", requireLogin(secret, log, handleCat(fileStore, log)))
	mux.Handle("POST /api/v1/upload/{id}/{section}", requireLogin(secret, log, handleUpload(log, fileStore)))
	mux.Handle("POST /api/v1/touch/{id}/{name}", requireLogin(secret, log, handleTouch(fileStore, log)))
	mux.Handle("POST /api/v1/mkdir/{id}/{name}", requireLogin(secret, log, handleMkdir(fileStore, log)))
	mux.Handle("POST /api/v1/mount/{parentID}/{childID}", requireLogin(secret, log, handleMount(fileStore, log)))
	mux.Handle("POST /api/v1/unmount/{parentID}/{childID}", requireLogin(secret, log, handleUnmount(fileStore, log)))

	mux.Handle("POST /api/v1/login", handleLogin(secret, log, userStore))
	mux.Handle("POST /api/v1/relogin", http.NotFoundHandler()) // generates a new session token given old token
	mux.Handle("GET /api/v1/whoami", requireLogin(secret, log, handleWhoami(secret, log)))
	mux.Handle("POST /api/v1/delete/{username}", adminOnly(secret, log, handleDeleteUser(secret, log, userStore)))
	mux.Handle("POST /api/v1/create/{username}/{password}", http.NotFoundHandler())

	mux.Handle("/", http.NotFoundHandler())
}
