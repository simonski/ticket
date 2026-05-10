package store

import (
	"context"
	"testing"
)

func TestTicketPhaseSignoffSetAndList(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Phase Signoff", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(ctx, db, "approver", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(approver) error = %v", err)
	}
	approver, err := GetUserByUsername(ctx, db, "approver")
	if err != nil {
		t.Fatalf("GetUserByUsername(approver) error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Needs signoff",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	initial, err := ListTicketPhaseSignoffs(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("ListTicketPhaseSignoffs(initial) error = %v", err)
	}
	if len(initial) != 3 {
		t.Fatalf("initial signoff len=%d want 3", len(initial))
	}
	for _, phase := range initial {
		if phase.Approved {
			t.Fatalf("expected unapproved initial phase, got %#v", phase)
		}
	}

	updated, err := SetTicketPhaseSignoff(ctx, db, ticket.ID, PhasePlanning, true, approver.ID, "ready for development")
	if err != nil {
		t.Fatalf("SetTicketPhaseSignoff() error = %v", err)
	}
	if !updated.Approved || updated.ApprovedBy != "approver" {
		t.Fatalf("updated signoff=%#v", updated)
	}

	reloaded, err := GetTicketPhaseSignoff(ctx, db, ticket.ID, PhasePlanning)
	if err != nil {
		t.Fatalf("GetTicketPhaseSignoff() error = %v", err)
	}
	if !reloaded.Approved || reloaded.Note != "ready for development" {
		t.Fatalf("reloaded signoff=%#v", reloaded)
	}
}
