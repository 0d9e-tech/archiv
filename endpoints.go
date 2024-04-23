package main

import (
	"archiiv/fs"
	"archiiv/id"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
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

func adminOnly(secret string, log *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if validateToken(secret, getSessionToken(r)) {
			if getUsername(r, secret) == "admin" {
				h.ServeHTTP(w, r)
				return
			}
		}
		sendError(log, w, http.StatusUnauthorized, "401 unauthorized")
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
	type loginRequest struct {
		Username string   `json:"username"`
		Password [64]byte `json:"password"`
	}

	type loginResponse struct {
		Token      string    `json:"token"`
		ExpireDate time.Time `json:"expireDate"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lr, err := decode[loginRequest](r)
		if err != nil {
			sendError(log, w, http.StatusBadRequest, "wrong name or password")
			return
		}

		ok, token := login(lr.Username, lr.Password, secret, userStore)

		if !ok {
			log.Info("Failed login", "user", lr.Username)
			sendError(log, w, http.StatusForbidden, "wrong name or password")
			return
		}

		log.Info("New login", "user", lr.Username)
		sendOK(log, w, loginResponse{Token: token})
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
		idArg := r.PathValue("id")

		id, e := id.Parse(idArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
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
		idArg := r.PathValue("id")
		sectionArg := r.PathValue("section")

		id, e := id.Parse(idArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
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
		idArg := r.PathValue("id")
		sectionArg := r.PathValue("section")

		id, e := id.Parse(idArg)
		if e != nil {
			log.Error("handleUpload", "error", e)
			sendError(log, w, http.StatusBadRequest, "invalid id")
			return
		}

		// TODO(matěj) check permission

		sectionWriter, e := fs.CreateSection(id, sectionArg)
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
		NewFileid id.ID `json:"new_file_id"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idArg := r.PathValue("id")
		name := r.PathValue("name")

		parentID, e := id.Parse(idArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		// TODO(matěj) check permission

		fileID, e := fs.Touch(parentID, name)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("touch: %v", e))
			return
		}

		sendOK(log, w, OkResponse{NewFileid: fileID})
	})
}

func handleMkdir(fs *fs.Fs, log *slog.Logger) http.Handler {
	type OkResponse struct {
		NewDirID id.ID `json:"new_dir_id"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idArg := r.PathValue("id")
		name := r.PathValue("name")

		id, e := id.Parse(idArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		// TODO(matěj) check permission

		fileID, e := fs.Mkdir(id, name)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("mkdir: %v", e))
			return
		}

		sendOK(log, w, OkResponse{NewDirID: fileID})
	})
}

func handleMount(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentArg := r.PathValue("parentID")
		childArg := r.PathValue("childID")

		parentID, e := id.Parse(parentArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		childID, e := id.Parse(childArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		// TODO(matěj) check permission

		e = fs.Mount(parentID, childID)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("parse id: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}

func handleUnmount(fs *fs.Fs, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentArg := r.PathValue("parentID")
		childArg := r.PathValue("childID")

		parentID, e := id.Parse(parentArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		childID, e := id.Parse(childArg)
		if e != nil {
			sendError(log, w, http.StatusBadRequest, fmt.Sprintf("parse id: %v", e))
			return
		}

		// TODO(matěj) check permission

		e = fs.Unmount(parentID, childID)
		if e != nil {
			sendError(log, w, http.StatusInternalServerError, fmt.Sprintf("parse id: %v", e))
			return
		}

		sendOK(log, w, nil)
	})
}

func handleDeleteUser(secret string, log *slog.Logger, userStore userStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetUser := r.PathValue("username")
		err := userStore.deleteUser(targetUser)

		if err != nil {
			sendError(log, w, http.StatusNotFound, "username not found")
			return
		}

		sendOK(log, w, nil)
	})
}
