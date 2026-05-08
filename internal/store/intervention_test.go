package store

import (
	"context"
	"testing"
)

func TestInterventionStateLifecycle(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	project, err := CreateProject(ctx, db, "Interventions", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Fails in prod",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	opened, err := GetInterventionState(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("GetInterventionState() error = %v", err)
	}
	if opened.State != InterventionStateOpen {
		t.Fatalf("default state = %q, want %q", opened.State, InterventionStateOpen)
	}
	triaged, err := SetInterventionState(ctx, db, ticket.ID, InterventionStateTriaged, "", "")
	if err != nil {
		t.Fatalf("SetInterventionState(triaged) error = %v", err)
	}
	if triaged.State != InterventionStateTriaged {
		t.Fatalf("triaged state = %q, want %q", triaged.State, InterventionStateTriaged)
	}
	resolved, err := ClaimIntervention(ctx, db, ticket.ID, "", "")
	if err != nil {
		t.Fatalf("ClaimIntervention() error = %v", err)
	}
	if resolved.State != InterventionStateInProgress {
		t.Fatalf("claimed state = %q, want %q", resolved.State, InterventionStateInProgress)
	}
}
