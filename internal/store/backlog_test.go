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

// TestSprintStageConstants verifies that complete and reject are valid non-backlog stages.
func TestSprintStageConstants(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{StageComplete, StageReject} {
		if !ValidStage(stage) {
			t.Fatalf("ValidStage(%q) = false, want true", stage)
		}
		if IsBacklogStage(stage) {
			t.Fatalf("IsBacklogStage(%q) = true, want false (sprint stage, not backlog)", stage)
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

// TestNextTicketCanAdvancePastReadyWhenInSprint verifies that a ticket in "ready"
// stage CAN advance to "develop" once it is assigned to a sprint.
func TestNextTicketCanAdvancePastReadyWhenInSprint(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	project, err := CreateProject(ctx, db, "Sprint Advance", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "sprint ticket",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Advance through the backlog stages via the state machine: idea→refine→ready.
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
	if _, err := NextTicket(ctx, db, ticket.ID, "admin", adminID); err != nil {
		t.Fatalf("NextTicket(idea->refine) error = %v", err)
	}
	setSuccess(ticket.ID)
	readyTicket, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(refine->ready) error = %v", err)
	}
	if readyTicket.Stage != StageReady {
		t.Fatalf("after backlog advance stage = %q, want ready", readyTicket.Stage)
	}
	// Set to success so the next advance works.
	setSuccess(ticket.ID)

	// Adding to sprint requires ready stage — should succeed.
	sprint, err := CreateSprint(ctx, db, int(project.ID), "Sprint 1")
	if err != nil {
		t.Fatalf("CreateSprint() error = %v", err)
	}
	if err := SetTicketSprint(ctx, db, ticket.ID, &sprint.ID); err != nil {
		t.Fatalf("SetTicketSprint() error = %v", err)
	}

	// Now NextTicket should advance to develop.
	advanced, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(ready->develop) with sprint: error = %v", err)
	}
	if advanced.Stage != StageDevelop {
		t.Fatalf("NextTicket(ready->develop) stage = %q, want develop", advanced.Stage)
	}
}

// TestSetTicketSprintRequiresReadyStage verifies that a ticket must be in "ready"
// stage before it can be assigned to a sprint.
func TestSetTicketSprintRequiresReadyStage(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Sprint Ready Guard", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	sprint, err := CreateSprint(ctx, db, int(project.ID), "Sprint 1")
	if err != nil {
		t.Fatalf("CreateSprint() error = %v", err)
	}

	// Ticket at "idea" stage — not yet ready.
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "not ready yet",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if ticket.Stage != StageIdea {
		t.Fatalf("initial stage = %q, want idea", ticket.Stage)
	}

	if err := SetTicketSprint(ctx, db, ticket.ID, &sprint.ID); err == nil {
		t.Fatal("SetTicketSprint(idea stage) want error, got nil")
	}

	// Move to refine — still not ready.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, state = ?, status = ? WHERE ticket_id = ?`,
		StageRefine, StateIdle, RenderLifecycleStatus(StageRefine, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set refine error = %v", err)
	}
	if err := SetTicketSprint(ctx, db, ticket.ID, &sprint.ID); err == nil {
		t.Fatal("SetTicketSprint(refine stage) want error, got nil")
	}

	// Move to ready — now it should succeed.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, state = ?, status = ? WHERE ticket_id = ?`,
		StageReady, StateIdle, RenderLifecycleStatus(StageReady, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set ready error = %v", err)
	}
	if err := SetTicketSprint(ctx, db, ticket.ID, &sprint.ID); err != nil {
		t.Fatalf("SetTicketSprint(ready stage) error = %v", err)
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

// TestSprintActivationBlockedByBacklogStages verifies that a sprint cannot be
// activated if it contains tickets that are still in idea or refine stage.
func TestSprintActivationBlockedByBacklogStages(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Sprint Activate", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	sprint, err := CreateSprint(ctx, db, int(project.ID), "Sprint 1")
	if err != nil {
		t.Fatalf("CreateSprint() error = %v", err)
	}

	// Create a ticket and force it to "ready" so it can be added to the sprint.
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "in sprint",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, state = ?, status = ? WHERE ticket_id = ?`,
		StageReady, StateIdle, RenderLifecycleStatus(StageReady, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set ready error = %v", err)
	}
	if err := SetTicketSprint(ctx, db, ticket.ID, &sprint.ID); err != nil {
		t.Fatalf("SetTicketSprint() error = %v", err)
	}

	// Now regress the ticket back to "idea" stage directly (simulates a ticket
	// that was moved back to backlog stages after sprint assignment).
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, status = ? WHERE ticket_id = ?`,
		StageIdea, RenderLifecycleStatus(StageIdea, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set idea error = %v", err)
	}

	// Activating the sprint must fail.
	if _, err := UpdateSprint(ctx, db, sprint.ID, sprint.Title, "active"); err == nil {
		t.Fatal("UpdateSprint(active) with idea-stage ticket: want error, got nil")
	}

	// Also test with refine stage.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, status = ? WHERE ticket_id = ?`,
		StageRefine, RenderLifecycleStatus(StageRefine, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set refine error = %v", err)
	}
	if _, err := UpdateSprint(ctx, db, sprint.ID, sprint.Title, "active"); err == nil {
		t.Fatal("UpdateSprint(active) with refine-stage ticket: want error, got nil")
	}

	// Restore to "ready" — activation should succeed.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET stage = ?, status = ? WHERE ticket_id = ?`,
		StageReady, RenderLifecycleStatus(StageReady, StateIdle), ticket.ID); err != nil {
		t.Fatalf("set ready error = %v", err)
	}
	if _, err := UpdateSprint(ctx, db, sprint.ID, sprint.Title, "active"); err != nil {
		t.Fatalf("UpdateSprint(active) with ready-stage ticket: error = %v", err)
	}
}
