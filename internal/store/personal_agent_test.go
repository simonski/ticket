package store

import (
	"context"
	"strings"
	"testing"
)

func TestEnsurePersonalAgent(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)

	owner, err := CreateUser(ctx, db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	agent, dm, err := EnsurePersonalAgent(ctx, db, owner)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if agent.UserType != "agent" {
		t.Fatalf("personal agent should be an agent, got %q", agent.UserType)
	}
	if agent.Username != "alice-assistant" {
		t.Fatalf("agent username = %q, want alice-assistant", agent.Username)
	}
	if !strings.HasPrefix(dm.Slug, "dm-") {
		t.Fatalf("DM slug = %q, want a dm- room", dm.Slug)
	}

	// Idempotent: a second call returns the same agent + DM.
	agent2, dm2, err := EnsurePersonalAgent(ctx, db, owner)
	if err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if agent2.ID != agent.ID {
		t.Fatalf("second ensure made a new agent (%s != %s)", agent2.ID, agent.ID)
	}
	if dm2.ID != dm.ID {
		t.Fatalf("second ensure made a new DM (%d != %d)", dm2.ID, dm.ID)
	}

	// A different user gets their own distinct agent.
	bob, _ := CreateUser(ctx, db, "bob", "password123", "user")
	bobAgent, _, err := EnsurePersonalAgent(ctx, db, bob)
	if err != nil {
		t.Fatalf("ensure bob: %v", err)
	}
	if bobAgent.ID == agent.ID {
		t.Fatal("bob should get his own agent, not alice's")
	}
}
