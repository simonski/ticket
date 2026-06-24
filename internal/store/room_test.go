package store

import (
	"context"
	"testing"
)

func TestRoomScopeAndCreate(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	pid := int64(7)

	global, err := CreateRoom(ctx, db, Room{Name: "General", CreatedBy: "alice"})
	if err != nil {
		t.Fatalf("create global: %v", err)
	}
	if global.Scope() != "global" {
		t.Fatalf("global scope = %q", global.Scope())
	}
	if global.Slug != "general" {
		t.Fatalf("slug = %q, want general", global.Slug)
	}

	proj, err := CreateRoom(ctx, db, Room{Name: "Ops Channel", CreatedBy: "alice", ProjectID: &pid})
	if err != nil {
		t.Fatalf("create project room: %v", err)
	}
	if proj.Scope() != "project" {
		t.Fatalf("project scope = %q", proj.Scope())
	}

	breakout, err := CreateRoom(ctx, db, Room{Name: "Epic TK-118", CreatedBy: "alice", ProjectID: &pid, TicketID: "TK-118"})
	if err != nil {
		t.Fatalf("create breakout: %v", err)
	}
	if breakout.Scope() != "breakout" {
		t.Fatalf("breakout scope = %q", breakout.Scope())
	}

	// The creator is auto-joined as owner.
	members, err := ListRoomMembers(ctx, db, global.ID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || members[0].MemberID != "alice" || members[0].Role != "owner" {
		t.Fatalf("creator membership = %+v", members)
	}
}

func TestRoomJoinLeaveAndMembership(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	// The first public room in a scope is the permanent (non-leavable) room, so
	// seed a primary "general" first; "team" is then an ordinary leavable room.
	if _, err := CreateRoom(ctx, db, Room{Name: "general", CreatedBy: "alice"}); err != nil {
		t.Fatalf("seed general: %v", err)
	}
	room, err := CreateRoom(ctx, db, Room{Name: "team", CreatedBy: "alice"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := JoinRoom(ctx, db, room.ID, "bob", "member"); err != nil {
		t.Fatalf("join: %v", err)
	}
	// Idempotent join.
	if err := JoinRoom(ctx, db, room.ID, "bob", "member"); err != nil {
		t.Fatalf("re-join: %v", err)
	}
	if ok, _ := IsRoomMember(ctx, db, room.ID, "bob"); !ok {
		t.Fatalf("bob should be a member")
	}
	members, _ := ListRoomMembers(ctx, db, room.ID)
	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}
	if err := LeaveRoom(ctx, db, room.ID, "bob"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if ok, _ := IsRoomMember(ctx, db, room.ID, "bob"); ok {
		t.Fatalf("bob should have left")
	}
}

func TestRoomPermanenceAndUnreadCounts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// First public global room is permanent; a second public room is leavable.
	general, _ := CreateRoom(ctx, db, Room{Name: "general", CreatedBy: "admin"})
	team, _ := CreateRoom(ctx, db, Room{Name: "team", CreatedBy: "admin"})
	priv, _ := CreateRoom(ctx, db, Room{Name: "secret", Visibility: "private", CreatedBy: "admin"})

	if p, _ := RoomIsPermanent(ctx, db, general); !p {
		t.Fatal("the primary public room should be permanent")
	}
	if p, _ := RoomIsPermanent(ctx, db, team); p {
		t.Fatal("a secondary public room should be leavable")
	}
	if p, _ := RoomIsPermanent(ctx, db, priv); p {
		t.Fatal("a private room should be leavable")
	}

	// Leaving the permanent room is rejected.
	_ = JoinRoom(ctx, db, general.ID, "bob", "member")
	if err := LeaveRoom(ctx, db, general.ID, "bob"); err != ErrRoomPermanent {
		t.Fatalf("leave permanent = %v, want ErrRoomPermanent", err)
	}

	// Unread counts: bob is a member of team; two messages arrive unread.
	_ = JoinRoom(ctx, db, team.ID, "bob", "member")
	_, _ = PostRoomMessage(ctx, db, RoomMessage{RoomID: team.ID, SenderID: "admin", Body: "hi"})
	_, _ = PostRoomMessage(ctx, db, RoomMessage{RoomID: team.ID, SenderID: "admin", Body: "again"})
	counts, err := RoomUnreadCounts(ctx, db, "bob")
	if err != nil {
		t.Fatalf("unread counts: %v", err)
	}
	if counts[team.ID] != 2 {
		t.Fatalf("team unread = %d, want 2", counts[team.ID])
	}
	// After marking read, the count clears.
	_ = MarkRoomRead(ctx, db, team.ID, "bob")
	counts, _ = RoomUnreadCounts(ctx, db, "bob")
	if counts[team.ID] != 0 {
		t.Fatalf("team unread after read = %d, want 0", counts[team.ID])
	}

	// FindRoomByName resolves case-insensitively.
	found, err := FindRoomByName(ctx, db, "TEAM", "bob", nil)
	if err != nil || found.ID != team.ID {
		t.Fatalf("find team: %v (id=%d)", err, found.ID)
	}
	if _, err := FindRoomByName(ctx, db, "nope", "bob", nil); err == nil {
		t.Fatal("unknown room should error")
	}
}

func TestRoomListVisibilityAndScope(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	pid := int64(3)

	pub, _ := CreateRoom(ctx, db, Room{Name: "public room", CreatedBy: "alice"})
	priv, _ := CreateRoom(ctx, db, Room{Name: "secret", CreatedBy: "alice", Visibility: "private"})
	_, _ = CreateRoom(ctx, db, Room{Name: "proj room", CreatedBy: "alice", ProjectID: &pid})

	// A non-member sees only public rooms.
	rooms, err := ListRooms(ctx, db, RoomFilter{MemberID: "bob"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if containsRoom(rooms, priv.ID) {
		t.Fatalf("bob should not see the private room")
	}
	if !containsRoom(rooms, pub.ID) {
		t.Fatalf("bob should see the public room")
	}

	// alice (a member/owner of the private room) sees it.
	rooms, _ = ListRooms(ctx, db, RoomFilter{MemberID: "alice"})
	if !containsRoom(rooms, priv.ID) {
		t.Fatalf("alice should see her private room")
	}

	// Global-only filter excludes the project room.
	globals, _ := ListRooms(ctx, db, RoomFilter{GlobalOnly: true, MemberID: "alice"})
	for _, r := range globals {
		if r.ProjectID != nil {
			t.Fatalf("global filter returned a project room: %+v", r)
		}
	}

	// Project filter returns only that project's rooms.
	projRooms, _ := ListRooms(ctx, db, RoomFilter{ProjectID: &pid, MemberID: "alice"})
	for _, r := range projRooms {
		if r.ProjectID == nil || *r.ProjectID != pid {
			t.Fatalf("project filter returned wrong room: %+v", r)
		}
	}
}

func TestRoomMessagesAndPagination(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	room, _ := CreateRoom(ctx, db, Room{Name: "chat", CreatedBy: "alice"})

	for i := range 5 {
		body := []string{"one", "two", "three", "four", "five"}[i]
		if _, err := PostRoomMessage(ctx, db, RoomMessage{RoomID: room.ID, SenderID: "alice", Body: body}); err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
	}

	all, err := ListRoomMessages(ctx, db, room.ID, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("messages = %d, want 5", len(all))
	}
	if all[0].Body != "one" || all[4].Body != "five" {
		t.Fatalf("chronological order wrong: %q .. %q", all[0].Body, all[4].Body)
	}

	// Latest page of 2.
	page, _ := ListRoomMessages(ctx, db, room.ID, 2, 0)
	if len(page) != 2 || page[0].Body != "four" || page[1].Body != "five" {
		t.Fatalf("latest page wrong: %+v", page)
	}
	// Older page before the first of that page.
	older, _ := ListRoomMessages(ctx, db, room.ID, 2, page[0].ID)
	if len(older) != 2 || older[1].Body != "three" {
		t.Fatalf("older page wrong: %+v", older)
	}

	// Attrs round-trip on a task message.
	msg, err := PostRoomMessage(ctx, db, RoomMessage{RoomID: room.ID, SenderID: "alice", Kind: "task", Body: "tasked", Attrs: Attrs{"task_id": "TK-200"}})
	if err != nil {
		t.Fatalf("task msg: %v", err)
	}
	got, _ := GetRoomMessage(ctx, db, msg.ID)
	if got.Kind != "task" || got.Attrs.GetString("task_id") != "TK-200" {
		t.Fatalf("task message round-trip wrong: kind=%q attrs=%+v", got.Kind, got.Attrs)
	}
}

func TestRoomArchiveHidesFromList(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	room, _ := CreateRoom(ctx, db, Room{Name: "temp", CreatedBy: "alice"})
	if err := ArchiveRoom(ctx, db, room.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	rooms, _ := ListRooms(ctx, db, RoomFilter{MemberID: "alice"})
	if containsRoom(rooms, room.ID) {
		t.Fatalf("archived room should not be listed")
	}
}

func containsRoom(rooms []Room, id int64) bool {
	for _, r := range rooms {
		if r.ID == id {
			return true
		}
	}
	return false
}
