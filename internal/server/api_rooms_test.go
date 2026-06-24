package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func roomLoginToken(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()
	resp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("login %s: status = %d", username, resp.Code)
	}
	var payload struct {
		Token string `json:"token"`
	}
	decodeResponse(t, resp, &payload)
	if payload.Token == "" {
		t.Fatalf("login %s: empty token", username)
	}
	return payload.Token
}

func registerUser(t *testing.T, handler http.Handler, username, password string) {
	t.Helper()
	resp := doJSONRequest(t, handler, http.MethodPost, "/api/register", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("register %s: status = %d", username, resp.Code)
	}
}

func TestRoomAPICRUDAndMessages(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	admin := roomLoginToken(t, handler, "admin", "password")

	// Create a room.
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{
		"name":  "General",
		"topic": "Everything",
	}, admin)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create room status = %d, body=%s", createResp.Code, createResp.Body.String())
	}
	var room store.Room
	decodeResponse(t, createResp, &room)
	if room.ID == 0 || room.Slug != "general" {
		t.Fatalf("created room = %+v", room)
	}

	// List shows it.
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/rooms", nil, admin)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d", listResp.Code)
	}
	var rooms []store.Room
	decodeResponse(t, listResp, &rooms)
	if len(rooms) != 1 || rooms[0].ID != room.ID {
		t.Fatalf("list = %+v", rooms)
	}

	// Post + list messages.
	roomPath := "/api/rooms/" + itoa(room.ID)
	postResp := doJSONRequest(t, handler, http.MethodPost, roomPath+"/messages", map[string]string{"body": "hello room"}, admin)
	if postResp.Code != http.StatusCreated {
		t.Fatalf("post message status = %d, body=%s", postResp.Code, postResp.Body.String())
	}
	msgResp := doJSONRequest(t, handler, http.MethodGet, roomPath+"/messages", nil, admin)
	var msgs []store.RoomMessage
	decodeResponse(t, msgResp, &msgs)
	if len(msgs) != 1 || msgs[0].Body != "hello room" {
		t.Fatalf("messages = %+v", msgs)
	}

	// Empty body rejected.
	badResp := doJSONRequest(t, handler, http.MethodPost, roomPath+"/messages", map[string]string{"body": ""}, admin)
	if badResp.Code != http.StatusBadRequest {
		t.Fatalf("empty message status = %d, want 400", badResp.Code)
	}

	// Archive.
	delResp := doJSONRequest(t, handler, http.MethodDelete, roomPath, nil, admin)
	if delResp.Code != http.StatusOK {
		t.Fatalf("archive status = %d", delResp.Code)
	}
	listResp = doJSONRequest(t, handler, http.MethodGet, "/api/rooms", nil, admin)
	decodeResponse(t, listResp, &rooms)
	if len(rooms) != 0 {
		t.Fatalf("archived room still listed: %+v", rooms)
	}
}

func TestRoomAPIJoinLeaveAndPrivateVisibility(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	admin := roomLoginToken(t, handler, "admin", "password")
	registerUser(t, handler, "bob", "password123")
	bob := roomLoginToken(t, handler, "bob", "password123")

	// The first public room in a scope is permanent (non-leavable); seed it so the
	// "Public" room below is an ordinary, leavable room.
	doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "general"}, admin)
	// Public room: bob can see and join.
	pubResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "Public"}, admin)
	var pub store.Room
	decodeResponse(t, pubResp, &pub)
	pubPath := "/api/rooms/" + itoa(pub.ID)

	var bobRooms []store.Room
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, "/api/rooms", nil, bob), &bobRooms)
	if !roomListed(bobRooms, pub.ID) {
		t.Fatalf("bob should see the public room")
	}
	if joinResp := doJSONRequest(t, handler, http.MethodPost, pubPath+"/join", nil, bob); joinResp.Code != http.StatusOK {
		t.Fatalf("bob join status = %d", joinResp.Code)
	}
	var members []store.RoomMember
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, pubPath+"/members", nil, admin), &members)
	if len(members) != 2 {
		t.Fatalf("members after join = %d, want 2", len(members))
	}
	if leaveResp := doJSONRequest(t, handler, http.MethodPost, pubPath+"/leave", nil, bob); leaveResp.Code != http.StatusOK {
		t.Fatalf("bob leave status = %d", leaveResp.Code)
	}

	// Private room: bob cannot see or fetch it.
	privResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "Secret", "visibility": "private"}, admin)
	var priv store.Room
	decodeResponse(t, privResp, &priv)
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, "/api/rooms", nil, bob), &bobRooms)
	if roomListed(bobRooms, priv.ID) {
		t.Fatalf("bob should not see the private room")
	}
	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/rooms/"+itoa(priv.ID), nil, bob)
	if getResp.Code != http.StatusForbidden {
		t.Fatalf("bob fetch private room status = %d, want 403", getResp.Code)
	}
}

func roomListed(rooms []store.Room, id int64) bool {
	for _, r := range rooms {
		if r.ID == id {
			return true
		}
	}
	return false
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestParseTaskCommand(t *testing.T) {
	cases := []struct {
		body     string
		assignee string
		desc     string
		ok       bool
	}{
		{"/task fix the login bug", "", "fix the login bug", true},
		{"/task @agent-1 ship the feature", "agent-1", "ship the feature", true},
		{"  /task   @bob   do it  ", "bob", "do it", true},
		{"/task", "", "", true},
		{"hello world", "", "", false},
		{"/taskfoo", "", "", false},
	}
	for _, tc := range cases {
		a, d, ok := parseTaskCommand(tc.body)
		if ok != tc.ok || a != tc.assignee || d != tc.desc {
			t.Errorf("parseTaskCommand(%q) = (%q,%q,%v), want (%q,%q,%v)", tc.body, a, d, ok, tc.assignee, tc.desc, tc.ok)
		}
	}
}

func TestRoomTaskCommandCreatesTicket(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	admin := roomLoginToken(t, handler, "admin", "password")

	// A project to scope the room/ticket to.
	projResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"prefix": "ROOM",
		"title":  "Room Project",
	}, admin)
	if projResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, body=%s", projResp.Code, projResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projResp, &project)

	// A project-scoped room.
	roomResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{
		"name":       "Build",
		"project_id": project.ID,
	}, admin)
	var room store.Room
	decodeResponse(t, roomResp, &room)
	if room.ProjectID == nil {
		t.Fatalf("project room missing project_id: %+v", room)
	}

	// /task creates a ticket and posts a task message linking it.
	taskResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms/"+itoa(room.ID)+"/messages",
		map[string]string{"body": "/task wire up the deploy script"}, admin)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("/task status = %d, body=%s", taskResp.Code, taskResp.Body.String())
	}
	var msg store.RoomMessage
	decodeResponse(t, taskResp, &msg)
	if msg.Kind != "task" {
		t.Fatalf("task message kind = %q, want task", msg.Kind)
	}
	ticketKey := msg.Attrs.GetString("task_id")
	if ticketKey == "" {
		t.Fatalf("task message has no task_id: %+v", msg.Attrs)
	}

	// The ticket exists.
	getTicket := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticketKey, nil, admin)
	if getTicket.Code != http.StatusOK {
		t.Fatalf("created ticket %s not found, status = %d", ticketKey, getTicket.Code)
	}

	// Tasking a global room is rejected.
	globalResp := doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "Global"}, admin)
	var global store.Room
	decodeResponse(t, globalResp, &global)
	badTask := doJSONRequest(t, handler, http.MethodPost, "/api/rooms/"+itoa(global.ID)+"/messages",
		map[string]string{"body": "/task do something"}, admin)
	if badTask.Code != http.StatusBadRequest {
		t.Fatalf("/task in global room status = %d, want 400", badTask.Code)
	}
}

func TestParseMentions(t *testing.T) {
	got := parseMentions("hey @alice and @bob-1, see #backend and @alice again")
	want := []string{"alice", "bob-1"}
	if len(got) != len(want) || got[0] != "alice" || got[1] != "bob-1" {
		t.Fatalf("parseMentions = %v, want %v", got, want)
	}
	if len(parseMentions("no mentions here")) != 0 {
		t.Fatalf("expected no mentions")
	}
}

func TestRoomMessageStoresMentions(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	admin := roomLoginToken(t, handler, "admin", "password")
	var room store.Room
	decodeResponse(t, doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "Standup"}, admin), &room)

	var msg store.RoomMessage
	decodeResponse(t, doJSONRequest(t, handler, http.MethodPost, "/api/rooms/"+itoa(room.ID)+"/messages",
		map[string]string{"body": "ping @agent-1 about #deploy"}, admin), &msg)
	mentions, ok := msg.Attrs["mentions"].([]any)
	if !ok || len(mentions) != 1 || mentions[0] != "agent-1" {
		t.Fatalf("message mentions = %v (ok=%v)", msg.Attrs["mentions"], ok)
	}
}

func TestRoomChatCommands(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	admin := roomLoginToken(t, handler, "admin", "password")
	registerUser(t, handler, "bob", "password123")

	var room store.Room
	decodeResponse(t, doJSONRequest(t, handler, http.MethodPost, "/api/rooms", map[string]any{"name": "Ops"}, admin), &room)
	msgPath := "/api/rooms/" + itoa(room.ID) + "/messages"
	send := func(body string) *httptest.ResponseRecorder {
		return doJSONRequest(t, handler, http.MethodPost, msgPath, map[string]string{"body": body}, admin)
	}

	// /new creates a room.
	if r := send("/new Engineering"); r.Code != http.StatusCreated {
		t.Fatalf("/new status=%d body=%s", r.Code, r.Body.String())
	}
	var rooms []store.Room
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, "/api/rooms", nil, admin), &rooms)
	if !roomNamed(rooms, "Engineering") {
		t.Fatalf("/new did not create Engineering: %+v", rooms)
	}

	// /invite then /kick adjust membership.
	send("/invite bob")
	var members []store.RoomMember
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, "/api/rooms/"+itoa(room.ID)+"/members", nil, admin), &members)
	if len(members) != 2 {
		t.Fatalf("after /invite members=%d, want 2", len(members))
	}
	send("/kick bob")
	decodeResponse(t, doJSONRequest(t, handler, http.MethodGet, "/api/rooms/"+itoa(room.ID)+"/members", nil, admin), &members)
	if len(members) != 1 {
		t.Fatalf("after /kick members=%d, want 1", len(members))
	}

	// /list returns a system message.
	var listMsg store.RoomMessage
	decodeResponse(t, send("/list"), &listMsg)
	if listMsg.Kind != "system" || !strings.Contains(listMsg.Body, "Rooms:") {
		t.Fatalf("/list message = %+v", listMsg)
	}

	// /msg routes to a DM room (different from the current room).
	var dm store.RoomMessage
	decodeResponse(t, send("/msg @bob hello in private"), &dm)
	if dm.RoomID == room.ID || dm.Body != "hello in private" {
		t.Fatalf("/msg routed wrong: %+v", dm)
	}

	// Bare "@bob ..." is also a DM shortcut to the same room.
	var dm2 store.RoomMessage
	decodeResponse(t, send("@bob and again"), &dm2)
	if dm2.RoomID != dm.RoomID {
		t.Fatalf("bare @ shortcut room=%d, want DM room %d", dm2.RoomID, dm.RoomID)
	}
}

func roomNamed(rooms []store.Room, name string) bool {
	for _, r := range rooms {
		if r.Name == name {
			return true
		}
	}
	return false
}
