package main

import (
	"archiiv/fs"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// Tries to send a error response but if that also fails just logs the error
func sendError(log *slog.Logger, w http.ResponseWriter, errorCode int, errorText string) {
	err := encodeError(w, errorCode, errorText)
	if err != nil {
		log.Error("failed to send error response", "error", err)
	}
}

func sendOK(log *slog.Logger, w http.ResponseWriter, v any) {
	err := encodeOK(w, v)
	if err != nil {
		log.Error("failed to send ok reponse", "error", err)
	}
}

func logAccesses(log *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("request", "url", r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

func requireLogin(secret string, log *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if validateToken(secret, token) {
			h.ServeHTTP(w, r)
		} else {
			sendError(log, w, http.StatusUnauthorized, "401 unauthorized")
		}
	})
}

func handleLogin(secret string, log *slog.Logger, userStore userStore) http.Handler {
	type LoginRequest struct {
		Username string   `json:"username"`
		Password [64]byte `json:"password"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lr, err := decode[LoginRequest](r)
		if err != nil {
			sendError(log, w, http.StatusBadRequest, "wrong name or password")
			return
		}

		ok, token := login(lr.Username, lr.Password, secret, userStore)

		if ok {
			log.Info("New login", "user", lr.Username)
			sendOK(log, w, struct {
				Token string `json:"token"`
			}{Token: token})
			return
		} else {
			log.Info("Failed login", "user", lr.Username)
			sendError(log, w, http.StatusForbidden, "wrong name or password")
			return
		}
	})
}

func handleWhoami(secret string, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := getUsername(r, secret)
		sendOK(log, w, struct {
			Name string `json:"name"`
		}{Name: name})
	})
}

func handleLs(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuidArg := r.PathValue("uuid")

		id, e := uuid.Parse(uuidArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		ch, e := fs.GetChildren(id)
		if e != nil {
			sendError(log, w, http.StatusNotFound, fmt.Sprintf("file not found: %v", e))
			return
		}

		// TODO(matěj) check permission

		sendOK(log, w, ch)
	})
}

func handleCat(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuidArg := r.PathValue("uuid")
		sectionArg := r.PathValue("section")

		id, e := uuid.Parse(uuidArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		// TODO(matěj) check permission

		sectionReader, e := fs.OpenSection(id, sectionArg)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("open section: %v", e))
			return
		}

		if _, e = io.Copy(w, sectionReader); e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("io copy: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}

func handleUpload(log *slog.Logger, fs *fs.Fs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuidArg := r.PathValue("uuid")
		sectionArg := r.PathValue("section")

		uuid, e := uuid.Parse(uuidArg)
		if e != nil {
			log.Error("handleUpload", "error", e)
			sendError(log, w, http.StatusBadRequest, "invalid uuid")
			return
		}

		// TODO(matěj) check permission

		sectionWriter, e := fs.CreateSection(uuid, sectionArg)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("create section: %v", e))
			return
		}

		if _, e = io.Copy(sectionWriter, r.Body); e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("io copy: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}

func handleTouch(fs *fs.Fs, log *slog.Logger) http.Handler {
	type OkResponse struct {
		NewFileUUID uuid.UUID `json:"new_file_uuid"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuidArg := r.PathValue("uuid")
		name := r.PathValue("name")

		parentID, e := uuid.Parse(uuidArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		// TODO(matěj) check permission

		fileID, e := fs.Touch(parentID, name)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("touch: %v", e))
			return
		}

		sendOK(log, w, OkResponse{NewFileUUID: fileID})
	})
}

func handleMkdir(fs *fs.Fs, log *slog.Logger) http.Handler {
	type OkResponse struct {
		NewDirUUID uuid.UUID `json:"new_dir_uuid"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuidArg := r.PathValue("uuid")
		name := r.PathValue("name")

		id, e := uuid.Parse(uuidArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		// TODO(matěj) check permission

		fileID, e := fs.Mkdir(id, name)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("mkdir: %v", e))
			return
		}

		sendOK(log, w, OkResponse{NewDirUUID: fileID})
	})
}

func handleMount(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentArg := r.PathValue("parentUUID")
		childArg := r.PathValue("childUUID")

		parentUUID, e := uuid.Parse(parentArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		childUUID, e := uuid.Parse(childArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		// TODO(matěj) check permission

		e = fs.Mount(parentUUID, childUUID)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}

func handleUnmount(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentArg := r.PathValue("parentUUID")
		childArg := r.PathValue("childUUID")

		parentUUID, e := uuid.Parse(parentArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		childUUID, e := uuid.Parse(childArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		// TODO(matěj) check permission

		e = fs.Unmount(parentUUID, childUUID)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("parse uuid: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}
