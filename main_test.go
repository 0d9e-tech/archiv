package main

import (
	"archiiv/fs"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestServer(t *testing.T) http.Handler {
	return newTestServerWithUsers(t, map[string][64]byte{})
}

func newTestServerWithUsers(t *testing.T, users map[string][64]byte) http.Handler {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	dir := t.TempDir()
	rootID, err := fs.InitFsDir(dir, users)
	if err != nil {
		t.Error(err)
	}

	secret := generateSecret()

	srv, _, err := createServer(log, []string{
		"--data_dir", dir,
		"--root_id", rootID.String(),
	}, func(s string) string {
		if s == "ARCHIIV_SECRET" {
			return secret
		}
		return ""
	})

	if err != nil {
		t.Fatalf("newTestServer: %v", err)
	}

	return srv
}

func decodeResponse[T any](t *testing.T, r *http.Response) (v T) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&v)

	assert.NoError(t, err)

	_, err = dec.Token()
	if assert.Error(t, err) {
		assert.Equal(t, err, io.EOF)
	}

	return
}

func getBody(t *testing.T, res *http.Response) string {
	b, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	return string(b)
}

func expectFail(t *testing.T, res *http.Response, statusCode int, errorMessage string) {
	assert.Equal(t, statusCode, res.StatusCode)
	b := decodeResponse[responseError](t, res)
	assert.Equal(t, false, b.Ok, false)
	assert.Equal(t, errorMessage, b.Error)
}

func expectStringLooksLikeToken(t *testing.T, token string) {
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("string does not look like token: base64 decode: %v", err)
	}

	ft, err := gobDecode[fullToken](data)
	if err != nil {
		t.Errorf("string does not look like token: gob decode %v", err)
	}

	_, err = payloadToBytes(ft.Data)
	if err != nil {
		t.Errorf("string does not look like token: payload to bytes: %v", err)
	}
}

func hit(srv http.Handler, method, target string, body io.Reader) *http.Response {
	req := httptest.NewRequest(method, target, body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w.Result()
}

func hitPost(t *testing.T, srv http.Handler, target string, body any) *http.Response {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		t.Errorf("failed to encode post body: %v", err)
	}

	return hit(srv, http.MethodPost, "/api/v1/login", &buf)
}

func hitGet(srv http.Handler, target string) *http.Response {
	req := httptest.NewRequest(http.MethodGet, target, strings.NewReader(""))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w.Result()
}

type loginRequest struct {
	Username string   `json:"username"`
	Password [64]byte `json:"password"`
}

func loginHelper(t *testing.T, srv http.Handler, username, pwd string) string {
	res := hitPost(t, srv, "/api/v1/login", loginRequest{Username: username, Password: hashPassword(pwd)})

	type LoginResponse struct {
		Ok   bool `json:"ok"`
		Data struct {
			Token      string    `json:"token"`
			ExpireDate time.Time `json:"expireDate"`
		} `json:"data"`
	}

	lr := decodeResponse[LoginResponse](t, res)

	return lr.Data.Token
}

func TestWhoamiNeedsLogin(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	res := hitGet(srv, "/api/v1/whoami")
	expectFail(t, res, http.StatusUnauthorized, "401 unauthorized")
}

func TestRootReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	res := hitGet(srv, "/")
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Equal(t, "404 page not found\n", getBody(t, res))
}

func TestLogin(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithUsers(t, map[string][64]byte{
		"prokop": hashPassword("catboy123"),
	})

	expectFail(t, hitPost(t, srv, "/api/v1/login", loginRequest{Username: "prokop", Password: hashPassword("eek")}), http.StatusForbidden, "wrong name or password")
	expectFail(t, hitPost(t, srv, "/api/v1/login", loginRequest{Username: "prokop", Password: hashPassword("uuhk")}), http.StatusForbidden, "wrong name or password")
	expectFail(t, hitPost(t, srv, "/api/v1/login", loginRequest{Username: "marek", Password: hashPassword("catboy123")}), http.StatusForbidden, "wrong name or password")
	res := hitPost(t, srv, "/api/v1/login", loginRequest{Username: "prokop", Password: hashPassword("catboy123")})
	assert.Equal(t, http.StatusOK, res.StatusCode)
	response := decodeResponse[struct {
		Ok   bool `json:"ok"`
		Data struct {
			Token      string    `json:"token"`
			ExpireDate time.Time `json:"expireDate"`
		} `json:"data"`
	}](t, res)
	assert.Equal(t, true, response.Ok)
	expectStringLooksLikeToken(t, response.Data.Token)
}

func TestWhoami(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithUsers(t, map[string][64]byte{"matuush": hashPassword("kadit")})

	token := loginHelper(t, srv, "matuush", "kadit")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/whoami", strings.NewReader(""))
	req.Header.Add("Authorization", token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	res := w.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "{\"ok\":true,\"data\":{\"name\":\"matuush\"}}\n", getBody(t, res))
}

func TestDelete(t *testing.T) {
	t.Parallel()
	srv := newTestServerWithUsers(t, map[string][64]byte{
		"matuush": hashPassword("kadit"),
		"admin":   hashPassword("heslo123")})

	token := loginHelper(t, srv, "matuush", "kadit")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/whoami", strings.NewReader(""))
	req.Header.Add("Authorization", token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "{\"ok\":true,\"data\":{\"name\":\"matuush\"}}\n", getBody(t, res))

	adminToken := loginHelper(t, srv, "admin", "heslo123")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/whoami", strings.NewReader(""))
	req.Header.Add("Authorization", adminToken)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "{\"ok\":true,\"data\":{\"name\":\"admin\"}}\n", getBody(t, res))

	req = httptest.NewRequest(http.MethodPost, "/api/v1/delete/matuush", strings.NewReader(""))
	req.Header.Add("Authorization", token)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	assert.Equal(t, "{\"ok\":false,\"error\":\"401 unauthorized\"}\n", getBody(t, res))

	req = httptest.NewRequest(http.MethodPost, "/api/v1/delete/matuush", strings.NewReader(""))
	req.Header.Add("Authorization", adminToken)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "{\"ok\":true}\n", getBody(t, res))
}
