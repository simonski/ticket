package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestServerServesHealthAndFrontend(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	srv, err := New(":0", db, "1.2.3", false, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/healthz")
	if err != nil {
		t.Fatalf("GET /api/healthz error = %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("health payload = %#v, want status ok", payload)
	}
	if payload["version"] != "1.2.3" {
		t.Fatalf("health payload version = %#v, want 1.2.3", payload)
	}

	rootResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	defer rootResp.Body.Close()

	body, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if !strings.Contains(string(body), "<title>task</title>") {
		t.Fatalf("root response missing embedded frontend")
	}
}

func TestServerVerboseLogging(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var logs strings.Builder
	srv, err := New(":0", db, "1.2.3", true, &logs)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/healthz")
	if err != nil {
		t.Fatalf("GET /api/healthz error = %v", err)
	}
	defer resp.Body.Close()

	if !strings.Contains(logs.String(), "REQUEST GET /api/healthz") {
		t.Fatalf("verbose logs missing request:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "RESPONSE 200") {
		t.Fatalf("verbose logs missing response:\n%s", logs.String())
	}
}
