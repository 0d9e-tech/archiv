package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type responseError struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

func encodeError(w http.ResponseWriter, status int, e string) error {
	return encode(w, status, responseError{Ok: false, Error: e})
}

func encodeOK[T any](w http.ResponseWriter, v T) error {
	return encode(w, http.StatusOK, struct {
		Ok   bool `json:"ok"`
		Data T    `json:"data,omitempty"`
	}{
		Ok:   true,
		Data: v,
	})
}

func encode(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}

// decodes the given struct as JSON from the request body
func decode[T any](r *http.Request) (T, error) {
	var v T

	err := json.NewDecoder(r.Body).Decode(&v)
	if err != nil {
		return v, fmt.Errorf("decode json: %w", err)
	}

	return v, nil
}
