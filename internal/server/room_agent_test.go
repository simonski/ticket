package server

import (
	"context"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestReplyAsAgentsPostsAgentReply(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	// An agent named "helper".
	agent, _, err := store.CreateAgent(ctx, db, "")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	uname := "helper"
	if _, uerr := store.UpdateAgent(ctx, db, agent.ID, store.AgentUpdateParams{Username: &uname}); uerr != nil {
		t.Fatalf("rename agent: %v", uerr)
	}

	room, err := store.CreateRoom(ctx, db, store.Room{Name: "Help", CreatedBy: "admin"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if jerr := store.JoinRoom(ctx, db, room.ID, agent.ID, "member"); jerr != nil {
		t.Fatalf("join agent: %v", jerr)
	}

	// Stub the responder so the test is deterministic and offline.
	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, agentName, prompt string, _ []store.RoomMessage) (string, error) {
		return "On it — " + agentName + " here", nil
	}
	defer func() { roomAgentReply = orig }()

	msg, err := store.PostRoomMessage(ctx, db, store.RoomMessage{RoomID: room.ID, SenderID: "u1", Body: "hey @helper can you look at the build?"})
	if err != nil {
		t.Fatalf("post message: %v", err)
	}

	if n := replyAsAgents(ctx, db, room, msg, nil); n != 1 {
		t.Fatalf("replyAsAgents posted %d replies, want 1", n)
	}

	msgs, _ := store.ListRoomMessages(ctx, db, room.ID, 50, 0)
	found := false
	for _, m := range msgs {
		if m.SenderID == agent.ID && strings.Contains(m.Body, "On it") {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent reply not posted into the room; messages=%+v", msgs)
	}
}

func TestReplyAsAgentsIgnoresNonAgentAndNonMember(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Room", CreatedBy: "admin"})

	called := false
	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _, _ string, _ []store.RoomMessage) (string, error) {
		called = true
		return "should not happen", nil
	}
	defer func() { roomAgentReply = orig }()

	// "admin" is a human, not an agent → no reply. "ghost" doesn't exist → no reply.
	msg, _ := store.PostRoomMessage(ctx, db, store.RoomMessage{RoomID: room.ID, SenderID: "u1", Body: "ping @admin and @ghost"})
	if n := replyAsAgents(ctx, db, room, msg, nil); n != 0 || called {
		t.Fatalf("replyAsAgents should not respond for non-agents/non-members (n=%d called=%v)", n, called)
	}
}
