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
	mux.Handle("POST /api/v1/login", handleLogin(secret, log, userStore))
	//mux.Handle("POST /api/v1/refresh-token", handleSessionTokenRefresh(secret, log, userStore))
	mux.Handle("GET /api/v1/whoami", requireLogin(secret, log, handleWhoami(secret, log)))

	mux.Handle("GET /api/v1/fs/ls/{uuid}", requireLogin(secret, log, handleLs(fileStore, log)))
	mux.Handle("GET /api/v1/fs/cat/{uuid}/{section}", requireLogin(secret, log, handleCat(fileStore, log)))
	mux.Handle("POST /api/v1/fs/upload/{uuid}/{section}", requireLogin(secret, log, handleUpload(log, fileStore)))
	mux.Handle("POST /api/v1/fs/touch/{uuid}/{name}", requireLogin(secret, log, handleTouch(fileStore, log)))
	mux.Handle("POST /api/v1/fs/mkdir/{uuid}/{name}", requireLogin(secret, log, handleMkdir(fileStore, log)))
	mux.Handle("POST /api/v1/fs/mount/{parentUUID}/{childUUID}", requireLogin(secret, log, handleMount(fileStore, log)))
	mux.Handle("POST /api/v1/fs/unmount/{parentUUID}/{childUUID}", requireLogin(secret, log, handleUnmount(fileStore, log)))

	mux.Handle("/", http.NotFoundHandler())
}
