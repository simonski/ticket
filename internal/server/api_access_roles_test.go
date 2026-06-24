package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func loginToken(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()
	resp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("login %s status = %d, body = %s", username, resp.Code, resp.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	decodeResponse(t, resp, &payload)
	if payload.Token == "" {
		t.Fatalf("login %s returned empty token", username)
	}
	return payload.Token
}

func TestAccessRoleAPIAndEnforcement(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	if _, err := store.CreateUser(context.Background(), db, "bob", "password123", "user"); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	adminToken := loginToken(t, handler, "admin", "password")
	bobToken := loginToken(t, handler, "bob", "password123")

	// Non-admin cannot list access roles.
	if resp := doJSONRequest(t, handler, http.MethodGet, "/api/access-roles", nil, bobToken); resp.Code != http.StatusForbidden {
		t.Fatalf("bob GET /api/access-roles = %d, want 403", resp.Code)
	}

	// Admin lists roles (builtin Member is seeded).
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/access-roles", nil, adminToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("admin list = %d", listResp.Code)
	}
	var roles []store.AccessRole
	decodeResponse(t, listResp, &roles)
	if len(roles) == 0 || roles[0].Name != "Member" {
		t.Fatalf("expected builtin Member role, got %+v", roles)
	}

	// Admin creates a restrictive role (tickets only).
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/access-roles", map[string]any{
		"name":   "Limited",
		"panels": []string{store.PanelTickets},
	}, adminToken)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create role = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var limited store.AccessRole
	decodeResponse(t, createResp, &limited)

	// Granting an admin panel is rejected.
	if resp := doJSONRequest(t, handler, http.MethodPost, "/api/access-roles", map[string]any{
		"name":   "Sneaky",
		"panels": []string{store.PanelUsers},
	}, adminToken); resp.Code != http.StatusBadRequest {
		t.Fatalf("granting admin panel = %d, want 400", resp.Code)
	}

	// Before assignment bob is grandfathered to all grantable panels and can hit /api/workflows.
	if resp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows", nil, bobToken); resp.Code == http.StatusForbidden {
		t.Fatalf("ungated bob should reach /api/workflows, got 403")
	}

	// Assign bob the restrictive role.
	bob, err := store.GetUserByUsername(context.Background(), db, "bob")
	if err != nil {
		t.Fatalf("get bob: %v", err)
	}
	if resp := doJSONRequest(t, handler, http.MethodPut, "/api/access-roles/memberships", map[string]any{
		"user_id":  bob.ID,
		"role_ids": []int64{limited.ID},
	}, adminToken); resp.Code != http.StatusOK {
		t.Fatalf("assign membership = %d, body = %s", resp.Code, resp.Body.String())
	}

	// bob's effective panels are now tickets only.
	meResp := doJSONRequest(t, handler, http.MethodGet, "/api/me/panels", nil, bobToken)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me/panels = %d", meResp.Code)
	}
	var me struct {
		Panels []string `json:"panels"`
	}
	decodeResponse(t, meResp, &me)
	if len(me.Panels) != 1 || me.Panels[0] != store.PanelTickets {
		t.Fatalf("bob panels = %v, want [tickets]", me.Panels)
	}

	// Enforcement: bob can no longer reach the workflows panel API; admin still can.
	if resp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows", nil, bobToken); resp.Code != http.StatusForbidden {
		t.Fatalf("restricted bob GET /api/workflows = %d, want 403", resp.Code)
	}
	if resp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows", nil, adminToken); resp.Code == http.StatusForbidden {
		t.Fatalf("admin must bypass panel enforcement, got 403")
	}
}
