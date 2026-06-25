package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestParseAgentTaskInstruction(t *testing.T) {
	cases := []struct {
		body, agent  string
		wantInstr    string
		wantIsTask   bool
		caseExplains string
	}{
		{"@bot do fix the build", "bot", "fix the build", true, "do verb"},
		{"@bot queue run the tests", "bot", "run the tests", true, "queue verb"},
		{"@BOT DO shout", "bot", "shout", true, "case-insensitive"},
		{"@bot hello there", "bot", "", false, "plain mention"},
		{"@bot do", "bot", "", false, "verb without instruction"},
		{"@bot doom the world", "bot", "", false, "verb must be a whole word"},
		{"no mention here", "bot", "", false, "no mention"},
	}
	for _, c := range cases {
		instr, ok := parseAgentTaskInstruction(c.body, c.agent)
		if ok != c.wantIsTask || instr != c.wantInstr {
			t.Errorf("%s: parseAgentTaskInstruction(%q) = (%q, %v), want (%q, %v)",
				c.caseExplains, c.body, instr, ok, c.wantInstr, c.wantIsTask)
		}
	}
}

func TestRoomAgentTaskQueueLifecycle(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	agent, _, err := store.CreateAgent(ctx, db, "")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	room, err := store.CreateRoom(ctx, db, store.Room{Name: "Work", CreatedBy: "admin"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	// Enqueue two tasks for the same (room, agent).
	t1, err := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: agent.ID, AgentName: agent.Username, Instruction: "first"})
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if t1.State != store.RoomAgentTaskQueued || !t1.Ephemeral {
		t.Fatalf("enqueued task = %+v, want queued+ephemeral", t1)
	}
	if _, err := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: agent.ID, AgentName: agent.Username, Instruction: "second"}); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}

	// First claim returns task 1 and marks it running.
	claimed, ok, err := store.ClaimNextRoomAgentTask(ctx, db)
	if err != nil || !ok {
		t.Fatalf("claim 1: ok=%v err=%v", ok, err)
	}
	if claimed.ID != t1.ID || claimed.State != store.RoomAgentTaskRunning {
		t.Fatalf("claimed = %+v, want task %d running", claimed, t1.ID)
	}

	// Second claim must NOT return task 2: serial concurrency-1 per (room, agent)
	// while task 1 is still running.
	if _, ok2, err := store.ClaimNextRoomAgentTask(ctx, db); err != nil || ok2 {
		t.Fatalf("claim 2 returned ok=%v (want no eligible task while one runs)", ok2)
	}

	// Finishing task 1 frees the (room, agent) so task 2 becomes claimable.
	if _, err := store.FinishRoomAgentTask(ctx, db, t1.ID, store.RoomAgentTaskDone, "done one"); err != nil {
		t.Fatalf("finish 1: %v", err)
	}
	claimed2, ok, err := store.ClaimNextRoomAgentTask(ctx, db)
	if err != nil || !ok {
		t.Fatalf("claim after finish: ok=%v err=%v", ok, err)
	}
	if claimed2.Instruction != "second" {
		t.Fatalf("claimed2 = %+v, want the second task", claimed2)
	}

	// Listing shows both tasks for the room.
	tasks, err := store.ListRoomAgentTasks(ctx, db, room.ID)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("list tasks = %d (%v)", len(tasks), err)
	}
}

func TestRoomAgentTaskClaimSeparateAgentsConcurrent(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	a1, _, _ := store.CreateAgent(ctx, db, "")
	a2, _, _ := store.CreateAgent(ctx, db, "")
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Multi", CreatedBy: "admin"})
	if _, err := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: a1.ID, AgentName: a1.Username, Instruction: "x"}); err != nil {
		t.Fatalf("enqueue a1: %v", err)
	}
	if _, err := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: a2.ID, AgentName: a2.Username, Instruction: "y"}); err != nil {
		t.Fatalf("enqueue a2: %v", err)
	}

	// Both tasks belong to different agents, so both are simultaneously claimable.
	c1, ok1, _ := store.ClaimNextRoomAgentTask(ctx, db)
	c2, ok2, _ := store.ClaimNextRoomAgentTask(ctx, db)
	if !ok1 || !ok2 {
		t.Fatalf("expected two concurrent claims for distinct agents, got ok1=%v ok2=%v", ok1, ok2)
	}
	if c1.AgentID == c2.AgentID {
		t.Fatalf("claims went to the same agent: %s twice", c1.AgentID)
	}
}

func TestExecuteRoomAgentTaskPostsResultAndCompletes(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	agent, _, _ := store.CreateAgent(ctx, db, "")
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Run", CreatedBy: "admin"})

	orig := roomAgentReply
	roomAgentReply = func(_ context.Context, _ store.AgentModelConfig, _ []string, agentName, instruction string, _ []store.RoomMessage) (string, error) {
		return "result for: " + instruction, nil
	}
	defer func() { roomAgentReply = orig }()

	task, err := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: agent.ID, AgentName: agent.Username, Instruction: "summarise"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claimed, ok, err := store.ClaimNextRoomAgentTask(ctx, db)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}

	s := &Server{}
	s.executeRoomAgentTask(db, nil, claimed)

	// Task is now done with the result recorded.
	got, err := store.GetRoomAgentTask(ctx, db, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.State != store.RoomAgentTaskDone || !strings.Contains(got.Result, "summarise") {
		t.Fatalf("task after run = %+v, want done with result", got)
	}

	// The result is posted into the room as an agent message.
	msgs, _ := store.ListRoomMessages(ctx, db, room.ID, 50, 0)
	found := false
	for _, m := range msgs {
		if m.SenderID == agent.ID && strings.Contains(m.Body, "result for: summarise") {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent result not posted into the room; messages=%+v", msgs)
	}
}

func TestPurgeFinishedRoomAgentTasks(t *testing.T) {
	_, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()

	agent, _, _ := store.CreateAgent(ctx, db, "")
	room, _ := store.CreateRoom(ctx, db, store.Room{Name: "Purge", CreatedBy: "admin"})
	task, _ := store.EnqueueRoomAgentTask(ctx, db, store.RoomAgentTask{RoomID: room.ID, AgentID: agent.ID, AgentName: agent.Username, Instruction: "old"})
	if _, err := store.FinishRoomAgentTask(ctx, db, task.ID, store.RoomAgentTaskDone, "ok"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	// A finished task is not purged while still within its TTL.
	if removed, err := store.PurgeFinishedRoomAgentTasks(ctx, db, 3600); err != nil || removed != 0 {
		t.Fatalf("purge within TTL removed=%d err=%v, want 0", removed, err)
	}
	// With a zero age, finished tasks are eligible and removed.
	if removed, err := store.PurgeFinishedRoomAgentTasks(ctx, db, 0); err != nil || removed != 1 {
		t.Fatalf("purge age=0 removed=%d err=%v, want 1", removed, err)
	}
}

func TestRoomAgentQueueEnqueueListAndPromote(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	ctx := context.Background()
	admin := roomLoginToken(t, handler, "admin", "password")

	// A project so the room is project-scoped (promotion creates a ticket there).
	projResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{"title": "Queue Project"}, admin)
	if projResp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", projResp.Code, projResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projResp, &project)

	// An agent named "buildbot", joined to a project room.
	agent, _, err := store.CreateAgent(ctx, db, "")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	uname := "buildbot"
	if _, uerr := store.UpdateAgent(ctx, db, agent.ID, store.AgentUpdateParams{Username: &uname}); uerr != nil {
		t.Fatalf("rename agent: %v", uerr)
	}
	pid := project.ID
	room, err := store.CreateRoom(ctx, db, store.Room{Name: "BuildRoom", ProjectID: &pid, CreatedBy: "admin"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if jerr := store.JoinRoom(ctx, db, room.ID, agent.ID, "member"); jerr != nil {
		t.Fatalf("join agent: %v", jerr)
	}

	roomPath := "/api/rooms/" + itoa(room.ID)

	// "@buildbot do …" enqueues an ephemeral task rather than replying.
	postResp := doJSONRequest(t, handler, http.MethodPost, roomPath+"/messages", map[string]string{"body": "@buildbot do run the linter"}, admin)
	if postResp.Code != http.StatusCreated {
		t.Fatalf("post task message: %d %s", postResp.Code, postResp.Body.String())
	}

	// The queue endpoint lists the queued task.
	queueResp := doJSONRequest(t, handler, http.MethodGet, roomPath+"/agent-queue", nil, admin)
	if queueResp.Code != http.StatusOK {
		t.Fatalf("agent-queue: %d %s", queueResp.Code, queueResp.Body.String())
	}
	var tasks []store.RoomAgentTask
	decodeResponse(t, queueResp, &tasks)
	if len(tasks) != 1 || tasks[0].Instruction != "run the linter" || tasks[0].State != store.RoomAgentTaskQueued {
		t.Fatalf("queue = %+v, want one queued 'run the linter'", tasks)
	}

	// /promote turns the queued task into a real ticket and clears it from the queue.
	promoteResp := doJSONRequest(t, handler, http.MethodPost, roomPath+"/messages", map[string]string{"body": "/promote " + itoa(tasks[0].ID)}, admin)
	if promoteResp.Code != http.StatusCreated {
		t.Fatalf("promote: %d %s", promoteResp.Code, promoteResp.Body.String())
	}
	var promoted store.RoomMessage
	decodeResponse(t, promoteResp, &promoted)
	if promoted.Kind != "task" || promoted.Attrs["task_id"] == nil {
		t.Fatalf("promote message = %+v, want a task message with task_id", promoted)
	}

	afterTasks, _ := store.ListRoomAgentTasks(ctx, db, room.ID)
	if len(afterTasks) != 0 {
		t.Fatalf("queue after promote = %+v, want empty", afterTasks)
	}
}
