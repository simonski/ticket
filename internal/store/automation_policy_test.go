package store

import (
	"context"
	"testing"
)

func TestAutomationPolicyRoundTripAndDiagnostics(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	project, err := CreateProject(ctx, db, "Policy Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Policy ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	policy, err := SetAutomationPolicy(ctx, db, AutomationPolicy{
		QueueStrategy:           "aging",
		ForecastLookbackHours:   12,
		InterventionEscalationH: 18,
		ForecastMinAccuracyPct:  75,
	})
	if err != nil {
		t.Fatalf("SetAutomationPolicy() error = %v", err)
	}
	if policy.QueueStrategy != "aging" || policy.ForecastMinAccuracyPct != 75 {
		t.Fatalf("policy = %#v", policy)
	}
	loaded, err := GetAutomationPolicy(ctx, db)
	if err != nil {
		t.Fatalf("GetAutomationPolicy() error = %v", err)
	}
	if loaded.QueueStrategy != "aging" || loaded.ForecastLookbackHours != 12 {
		t.Fatalf("loaded policy = %#v", loaded)
	}
	adminID := testAdminID(t, db)
	if _, err := SetTicketDraft(ctx, db, ticket.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft(false) error = %v", err)
	}
	diag, err := DiagnoseTicketPolicy(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("DiagnoseTicketPolicy() error = %v", err)
	}
	if !diag.Eligible {
		t.Fatalf("expected eligible diagnostics, got %#v", diag)
	}
	_, err = UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
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
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	diag, err = DiagnoseTicketPolicy(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("DiagnoseTicketPolicy(fail) error = %v", err)
	}
	if diag.Eligible {
		t.Fatalf("expected ineligible diagnostics, got %#v", diag)
	}
}
