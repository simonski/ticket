package server

import (
	"context"
	"errors"
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
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, agentName, prompt string, _ []store.RoomMessage) (string, error) {
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

func TestReplyAsAgentsPostsNoticeOnFailure(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	agent, _, err := store.CreateAgent(ctx, db, "")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	uname := "helper"
	if _, uerr := store.UpdateAgent(ctx, db, agent.ID, store.AgentUpdateParams{Username: &uname}); uerr != nil {
		t.Fatalf("rename agent: %v", uerr)
	}
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Help", CreatedBy: "admin"})
	_ = store.JoinRoom(ctx, db, room.ID, agent.ID, "member")

	// The agent command fails.
	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, _, _ string, _ []store.RoomMessage) (string, error) {
		return "", errors.New("exit status 1: model not supported")
	}
	defer func() { roomAgentReply = orig }()

	msg, _ := store.PostRoomMessage(ctx, db, store.RoomMessage{RoomID: room.ID, SenderID: "u1", Body: "hey @helper?"})
	replyAsAgents(ctx, db, room, msg, nil)

	msgs, _ := store.ListRoomMessages(ctx, db, room.ID, 50, 0)
	notice := false
	for _, m := range msgs {
		if m.SenderID == agent.ID && m.Kind == "agent_event" && strings.Contains(m.Body, "couldn't reply") {
			notice = true
		}
	}
	if !notice {
		t.Fatalf("a failed agent reply should post an in-room notice; messages=%+v", msgs)
	}
}

func TestReplyAsAgentsPostsNoticeOnEmptyReply(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	agent, _, _ := store.CreateAgent(ctx, db, "")
	uname := "helper"
	_, _ = store.UpdateAgent(ctx, db, agent.ID, store.AgentUpdateParams{Username: &uname})
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Help", CreatedBy: "admin"})
	_ = store.JoinRoom(ctx, db, room.ID, agent.ID, "member")

	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, _, _ string, _ []store.RoomMessage) (string, error) {
		return "   ", nil // empty after trim, no error
	}
	defer func() { roomAgentReply = orig }()

	msg, _ := store.PostRoomMessage(ctx, db, store.RoomMessage{RoomID: room.ID, SenderID: "u1", Body: "hey @helper?"})
	replyAsAgents(ctx, db, room, msg, nil)

	msgs, _ := store.ListRoomMessages(ctx, db, room.ID, 50, 0)
	notice := false
	for _, m := range msgs {
		if m.SenderID == agent.ID && m.Kind == "agent_event" && strings.Contains(m.Body, "empty reply") {
			notice = true
		}
	}
	if !notice {
		t.Fatalf("an empty agent reply should post a notice; messages=%+v", msgs)
	}
}

func TestReplyAsAgentsIgnoresNonAgentAndNonMember(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Room", CreatedBy: "admin"})

	called := false
	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, _, _ string, _ []store.RoomMessage) (string, error) {
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

func TestDefaultRoomAgentReplyPromptPlaceholder(t *testing.T) {
	ctx := context.Background()
	// {prompt} placeholder: substituted as an argument (echo prints it back).
	out, err := defaultRoomAgentReply(ctx, store.AgentModelConfig{}, []string{"echo", "{prompt}"}, "helper", "ping me", nil)
	if err != nil {
		t.Fatalf("echo placeholder: %v", err)
	}
	if !strings.Contains(out, "helper") || !strings.Contains(out, "ping me") {
		t.Fatalf("placeholder output missing prompt: %q", out)
	}
	// No placeholder: prompt is piped to stdin (cat echoes it).
	out2, err := defaultRoomAgentReply(ctx, store.AgentModelConfig{}, []string{"cat"}, "helper", "ping me", nil)
	if err != nil {
		t.Fatalf("cat stdin: %v", err)
	}
	if !strings.Contains(out2, "ping me") {
		t.Fatalf("stdin output missing prompt: %q", out2)
	}
}

func TestExtractAgentReplyStreamJSON(t *testing.T) {
	// Claude stream-json: NDJSON events; prefer the final result.
	nd := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hello "}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"there"}]}}
{"type":"result","subtype":"success","result":"Hello there"}`
	if got := extractAgentReply([]string{"claude", "-p", "{prompt}", "--output-format", "stream-json", "--verbose"}, nd); got != "Hello there" {
		t.Fatalf("stream-json = %q, want 'Hello there'", got)
	}
	// stream-json with no result event falls back to concatenated assistant text.
	nd2 := `{"type":"assistant","message":{"content":[{"type":"text","text":"part1 "}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"part2"}]}}`
	if got := extractAgentReply([]string{"claude", "--output-format", "stream-json"}, nd2); got != "part1 part2" {
		t.Fatalf("stream-json accum = %q, want 'part1 part2'", got)
	}
	// --output-format json: single result object.
	single := `{"type":"result","subtype":"success","result":"single answer"}`
	if got := extractAgentReply([]string{"claude", "-p", "{prompt}", "--output-format", "json"}, single); got != "single answer" {
		t.Fatalf("json = %q, want 'single answer'", got)
	}
	// Plain command: raw stdout passthrough.
	if got := extractAgentReply([]string{"codex", "exec"}, "raw reply text"); got != "raw reply text" {
		t.Fatalf("raw = %q", got)
	}
}
