package store

import (
	"context"
	"testing"
)

// TestBacklogStageConstants verifies the three backlog stage constants exist and are valid.
func TestBacklogStageConstants(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{StageIdea, StageRefine, StageReady} {
		if !ValidStage(stage) {
			t.Fatalf("ValidStage(%q) = false, want true", stage)
		}
	}
}

// TestIsBacklogStage checks that idea/refine/ready are backlog stages and others are not.
func TestIsBacklogStage(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{StageIdea, StageRefine, StageReady} {
		if !IsBacklogStage(stage) {
			t.Fatalf("IsBacklogStage(%q) = false, want true", stage)
		}
	}
	for _, stage := range []string{StageDevelop, StageTest, StageDone, StageDesign, StageComplete, StageReject, "open", ""} {
		if IsBacklogStage(stage) {
			t.Fatalf("IsBacklogStage(%q) = true, want false", stage)
		}
	}
}

// TestNonBacklogStageConstants verifies that complete and reject are valid non-backlog stages.
func TestNonBacklogStageConstants(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{StageComplete, StageReject} {
		if !ValidStage(stage) {
			t.Fatalf("ValidStage(%q) = false, want true", stage)
		}
		if IsBacklogStage(stage) {
			t.Fatalf("IsBacklogStage(%q) = true, want false (non-backlog stage)", stage)
		}
	}
}

// TestNewTicketStartsAtIdea verifies tickets default to the idea stage.
func TestNewTicketStartsAtIdea(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	project, err := CreateProject(ctx, db, "Backlog Test", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "New ticket",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if ticket.Stage != StageIdea {
		t.Fatalf("CreateTicket().Stage = %q, want %q", ticket.Stage, StageIdea)
	}
}

// TestNextTicketCannotAdvancePastReadyWithoutSprint verifies that a backlog ticket
// at "ready" stage cannot progress to "develop" unless assigned to a sprint.
func TestNextTicketCannotAdvancePastReadyWithoutSprint(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	project, err := CreateProject(ctx, db, "Backlog Guard", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "needs a sprint",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if ticket.Stage != StageIdea {
		t.Fatalf("initial stage = %q, want idea", ticket.Stage)
	}

	// Advance idea→refine, refine→ready via NextTicket.
	setSuccess := func(id string) {
		t.Helper()
		cur, err := GetTicket(ctx, db, id)
		if err != nil {
			t.Fatalf("GetTicket() error = %v", err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`,
			StateSuccess, RenderLifecycleStatus(cur.Stage, StateSuccess), id); err != nil {
			t.Fatalf("set success error = %v", err)
		}
	}

	setSuccess(ticket.ID)
	refined, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(idea->refine) error = %v", err)
	}
	if refined.Stage != StageRefine {
		t.Fatalf("after first advance stage = %q, want refine", refined.Stage)
	}

	setSuccess(ticket.ID)
	ready, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(refine->ready) error = %v", err)
	}
	if ready.Stage != StageReady {
		t.Fatalf("after second advance stage = %q, want ready", ready.Stage)
	}

	// Attempt to advance past "ready" without a sprint — must fail.
	setSuccess(ticket.ID)
	_, err = NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err == nil {
		t.Fatal("NextTicket(ready->develop) without sprint: want error, got nil")
	}
}

// TestMarkTicketReadySetsReadyStage verifies MarkTicketReady moves to "ready" stage.
func TestMarkTicketReadySetsReadyStage(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	project, err := CreateProject(ctx, db, "Mark Ready", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "mark me ready",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	ready, err := MarkTicketReady(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("MarkTicketReady() error = %v", err)
	}
	if ready.Stage != StageReady {
		t.Fatalf("MarkTicketReady().Stage = %q, want %q", ready.Stage, StageReady)
	}
	if ready.Draft {
		t.Fatalf("MarkTicketReady().Draft = true, want false")
	}
}

// TestMarkTicketReadyMovesToDevelopWhenNoReadyStage verifies that, in a workflow
// without a "ready" holding stage (e.g. Agile design→develop→test→done), marking a
// refined story ready moves it straight into the develop stage.
func TestMarkTicketReadyMovesToDevelopWhenNoReadyStage(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	wf, err := CreateWorkflow(ctx, db, "agile", "design develop test done")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	design, err := AddWorkflowStage(ctx, db, wf.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	if err := SetWorkflowStageBacklog(ctx, db, design.ID, true); err != nil {
		t.Fatalf("SetWorkflowStageBacklog() error = %v", err)
	}
	for i, name := range []string{"develop", "test", "done"} {
		if _, err := AddWorkflowStage(ctx, db, wf.ID, name, "", "", i+1); err != nil {
			t.Fatalf("AddWorkflowStage(%s) error = %v", name, err)
		}
	}

	project, err := CreateProject(ctx, db, "Agile", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	wfID := wf.ID
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &wfID,
		Type:       "story",
		Title:      "refine me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	ready, err := MarkTicketReady(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("MarkTicketReady() error = %v", err)
	}
	if ready.Stage != StageDevelop {
		t.Fatalf("MarkTicketReady().Stage = %q, want %q", ready.Stage, StageDevelop)
	}
	if ready.Draft {
		t.Fatalf("MarkTicketReady().Draft = true, want false")
	}
}
