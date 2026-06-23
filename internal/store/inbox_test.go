package store

import (
	"context"
	"testing"
)

func TestFailureEscalationInboxLifecycle(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Inbox", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(ctx, db, "owner", "password123", "admin"); err != nil {
		t.Fatalf("CreateUser(owner) error = %v", err)
	}
	owner, err := GetUserByUsername(ctx, db, "owner")
	if err != nil {
		t.Fatalf("GetUserByUsername(owner) error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Escalate me",
		CreatedBy: owner.ID,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	ticket, err = UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
		Title:              ticket.Title,
		Description:        ticket.Description,
		AcceptanceCriteria: ticket.AcceptanceCriteria,
		GitRepository:      ticket.GitRepository,
		GitBranch:          ticket.GitBranch,
		ParentID:           ticket.ParentID,
		Assignee:           ticket.Assignee,
		Stage:              ticket.Stage,
		State:              StateFail,
		Priority:           ticket.Priority,
		Order:              ticket.Order,
		EstimateEffort:     ticket.EstimateEffort,
		EstimateComplete:   ticket.EstimateComplete,
		Type:               ticket.Type,
		UpdatedBy:          owner.ID,
		ActorUsername:      owner.Username,
		ActorRole:          owner.Role,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(fail) error = %v", err)
	}

	entry, err := EnsureFailureEscalationInboxEntry(ctx, db, ticket, "failed checks", owner.ID)
	if err != nil {
		t.Fatalf("EnsureFailureEscalationInboxEntry() error = %v", err)
	}
	if entry.Status != InboxStatusOpen || entry.Kind != InboxKindFailureEscalation {
		t.Fatalf("entry=%#v", entry)
	}
	// Two recommendations remain after the Goals feature (clarify_goal) was removed.
	if len(entry.Recommendations) != 2 {
		t.Fatalf("entry recommendations=%#v", entry.Recommendations)
	}

	entries, err := ListInboxEntriesByTicket(ctx, db, ticket.ID, "")
	if err != nil {
		t.Fatalf("ListInboxEntriesByTicket() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len=%d want=1", len(entries))
	}

	decided, err := DecideInboxEntry(ctx, db, entry.ID, InboxDecisionRefineRequirements, "tighten acceptance criteria", owner.ID)
	if err != nil {
		t.Fatalf("DecideInboxEntry() error = %v", err)
	}
	if decided.Status != InboxStatusResolved || decided.Decision != InboxDecisionRefineRequirements {
		t.Fatalf("decided entry=%#v", decided)
	}
}
