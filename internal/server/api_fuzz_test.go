package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func FuzzLoginRequestBody(f *testing.F) {
	f.Add([]byte(`{"username":"admin","password":"password"}`))
	f.Add([]byte(`{"username":"admin"}`))
	f.Add([]byte(`{"password":"password"}`))
	f.Add([]byte(`{"username":123}`))
	f.Add([]byte(`not-json`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, body []byte) {
		handler, db := testHandler(t)
		defer db.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)

		switch resp.Code {
		case http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		default:
			t.Fatalf("unexpected status %d for body %q", resp.Code, string(body))
		}
	})
}
