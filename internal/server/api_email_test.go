package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestEmailSettingsAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	if _, err := store.CreateUser(context.Background(), db, "bob", "password123", "user"); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	adminToken := loginToken(t, handler, "admin", "password")
	bobToken := loginToken(t, handler, "bob", "password123")

	// Non-admin is denied.
	if resp := doJSONRequest(t, handler, http.MethodGet, "/api/email/settings", nil, bobToken); resp.Code != http.StatusForbidden {
		t.Fatalf("bob GET email settings = %d, want 403", resp.Code)
	}

	// Admin sees defaults; no password configured.
	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/email/settings", nil, adminToken)
	if getResp.Code != http.StatusOK {
		t.Fatalf("admin GET = %d", getResp.Code)
	}
	var cfg map[string]any
	decodeResponse(t, getResp, &cfg)
	if cfg["has_password"] != false || cfg["enabled"] != false {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	// Save a full config.
	putResp := doJSONRequest(t, handler, http.MethodPut, "/api/email/settings", map[string]any{
		"enabled": true, "host": "smtp.example.com", "port": 465, "username": "mailer",
		"password": "secret", "from_address": "noreply@example.com", "security": "tls",
	}, adminToken)
	if putResp.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, body = %s", putResp.Code, putResp.Body.String())
	}
	decodeResponse(t, putResp, &cfg)
	if cfg["has_password"] != true {
		t.Fatalf("has_password should be true after save: %+v", cfg)
	}
	if _, leaked := cfg["password"]; leaked {
		t.Fatalf("password must never be returned to clients: %+v", cfg)
	}

	// Saving again without a password preserves the stored secret.
	if resp := doJSONRequest(t, handler, http.MethodPut, "/api/email/settings", map[string]any{
		"enabled": true, "host": "smtp.example.com", "port": 587, "username": "mailer",
		"from_address": "noreply@example.com", "security": "starttls",
	}, adminToken); resp.Code != http.StatusOK {
		t.Fatalf("PUT no-password = %d", resp.Code)
	}
	saved, _ := store.GetEmailConfig(context.Background(), db)
	if saved.Password != "secret" || saved.Port != 587 {
		t.Fatalf("expected preserved password + updated port, got %+v", saved)
	}

	// Toggle enabled off.
	if resp := doJSONRequest(t, handler, http.MethodPut, "/api/email/enabled", map[string]any{"enabled": false}, adminToken); resp.Code != http.StatusOK {
		t.Fatalf("PUT enabled = %d", resp.Code)
	}
	saved, _ = store.GetEmailConfig(context.Background(), db)
	if saved.Enabled {
		t.Fatal("email should be disabled after toggle")
	}
}
