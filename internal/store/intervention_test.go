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

func TestBuildInterventionReport(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	project, err := CreateProject(ctx, db, "Interventions Report", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Escalate me",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	ticket, err = UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		Assignee:      ticket.Assignee,
		Priority:      ticket.Priority,
		Order:         ticket.Order,
		State:         StateFail,
		ActorRole:     "admin",
		ActorUsername: "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() fail error = %v", err)
	}
	if _, err := SetInterventionState(ctx, db, ticket.ID, InterventionStateOpen, "", ""); err != nil {
		t.Fatalf("SetInterventionState(open) error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE intervention_states SET updated_at = datetime('now', '-48 hours') WHERE ticket_id = ?`, ticket.ID); err != nil {
		t.Fatalf("aging intervention state error = %v", err)
	}

	report, err := BuildInterventionReport(ctx, db, project.ID, 24)
	if err != nil {
		t.Fatalf("BuildInterventionReport() error = %v", err)
	}
	if report.ProjectID != project.ID {
		t.Fatalf("report project id = %d, want %d", report.ProjectID, project.ID)
	}
	if report.OpenCount != 1 || len(report.Items) != 1 {
		t.Fatalf("unexpected report counts: %#v", report)
	}
	if !report.Items[0].Escalated {
		t.Fatalf("expected escalated intervention item, got %#v", report.Items[0])
	}
	if report.OldestOpenAgeH < 24 {
		t.Fatalf("oldest age = %d, want >= 24", report.OldestOpenAgeH)
	}
	if len(report.Trends) == 0 {
		t.Fatalf("expected trends in report, got %#v", report)
	}
	trends, err := BuildInterventionTrends(ctx, db, project.ID, 7)
	if err != nil {
		t.Fatalf("BuildInterventionTrends() error = %v", err)
	}
	if len(trends) != 7 {
		t.Fatalf("trends len = %d, want 7", len(trends))
	}
}
