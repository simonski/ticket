package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestPersonalAgentEndpointAndDMReply(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()
	adminToken := loginToken(t, handler, "admin", "password")

	// GET /api/me/agent provisions the personal agent + DM room.
	resp := doJSONRequest(t, handler, http.MethodGet, "/api/me/agent", nil, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/me/agent = %d, body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		RoomID int64 `json:"room_id"`
		Agent  struct {
			Username string `json:"username"`
		} `json:"agent"`
	}
	decodeResponse(t, resp, &payload)
	if payload.RoomID == 0 || !strings.Contains(payload.Agent.Username, "assistant") {
		t.Fatalf("unexpected personal-agent payload: %+v", payload)
	}

	// Idempotent: a second call returns the same DM room.
	resp2 := doJSONRequest(t, handler, http.MethodGet, "/api/me/agent", nil, adminToken)
	var payload2 struct {
		RoomID int64 `json:"room_id"`
	}
	decodeResponse(t, resp2, &payload2)
	if payload2.RoomID != payload.RoomID {
		t.Fatalf("second call returned a different DM room (%d != %d)", payload2.RoomID, payload.RoomID)
	}

	// In the DM, the agent replies to a message with NO @mention (stubbed responder).
	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, agentName, _ string, _ []store.RoomMessage) (string, error) {
		return "hello from " + agentName, nil
	}
	defer func() { roomAgentReply = orig }()

	dm, _ := store.GetRoom(ctx, db, payload.RoomID)
	admin, _ := store.GetUserByUsername(ctx, db, "admin")
	msg, _ := store.PostRoomMessage(ctx, db, store.RoomMessage{RoomID: dm.ID, SenderID: admin.ID, Body: "are you there?"})
	if n := replyAsAgents(ctx, db, dm, msg, nil); n != 1 {
		t.Fatalf("DM auto-reply posted %d, want 1", n)
	}
	msgs, _ := store.ListRoomMessages(ctx, db, dm.ID, 50, 0)
	found := false
	for _, m := range msgs {
		if strings.Contains(m.Body, "hello from") {
			found = true
		}
	}
	if !found {
		t.Fatal("agent did not reply in the personal-agent DM")
	}
}

func TestUserAgentModelEndpoint(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	tok := loginToken(t, handler, "admin", "password")

	if r := doJSONRequest(t, handler, http.MethodPut, "/api/me/agent-model", map[string]any{"provider": "anthropic", "model": "claude-x"}, tok); r.Code != http.StatusOK {
		t.Fatalf("PUT /api/me/agent-model = %d, body=%s", r.Code, r.Body.String())
	}
	g := doJSONRequest(t, handler, http.MethodGet, "/api/me/agent-model", nil, tok)
	var cfg store.AgentModelConfig
	decodeResponse(t, g, &cfg)
	if cfg.Provider != "anthropic" || cfg.Model != "claude-x" {
		t.Fatalf("per-user config not persisted: %+v", cfg)
	}
}
