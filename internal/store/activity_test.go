package store

import (
	"context"
	"testing"
)

func TestHistoryAndComments(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add login",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	events, err := ListHistoryEvents(context.Background(), db, ticket.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	if len(events) == 0 || events[0].EventType != "ticket_created" {
		t.Fatalf("history after create = %#v", events)
	}

	_, err = UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:       ticket.Title,
		Description: "Updated description",
		ParentID:    ticket.ParentID,
		UpdatedBy:   "",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	events, err = ListHistoryEvents(context.Background(), db, ticket.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListHistoryEvents(after update) error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("history length = %d, want at least 2", len(events))
	}

	comment, err := AddComment(context.Background(), db, ticket.ID, adminID, "Waiting on API changes.")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.Comment != "Waiting on API changes." {
		t.Fatalf("AddComment().Comment = %q", comment.Comment)
	}

	comments, err := ListComments(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("ListComments() len = %d, want 1", len(comments))
	}
	if comments[0].Author != "admin" || comments[0].Text != "Waiting on API changes." {
		t.Fatalf("ListComments() = %#v", comments)
	}

	taskWithComments, err := GetTicket(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if len(taskWithComments.Comments) != 1 || taskWithComments.Comments[0].Author != "admin" {
		t.Fatalf("GetTicket().Comments = %#v", taskWithComments.Comments)
	}
}

func TestListProjectHistory(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(context.Background(), db, "History Project", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "History task",
		CreatedBy: adminID,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Update ticket to create more history
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:       "Updated history task",
		Description: "desc",
		ParentID:    ticket.ParentID,
		UpdatedBy:   adminID,
	}); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	events, err := ListProjectHistory(context.Background(), db, project.ID, 10)
	if err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("ListProjectHistory() len = %d, want >= 1", len(events))
	}

	// Test with default limit (0)
	events, err = ListProjectHistory(context.Background(), db, project.ID, 0)
	if err != nil {
		t.Fatalf("ListProjectHistory(limit=0) error = %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("ListProjectHistory(limit=0) len = %d, want >= 1", len(events))
	}
}

func TestListHistoryEventsPagination(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(context.Background(), db, "Paged History", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "History pagination",
		CreatedBy: adminID,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
			Title:       ticket.Title,
			Description: "rev",
			ParentID:    ticket.ParentID,
			UpdatedBy:   adminID,
		}); err != nil {
			t.Fatalf("UpdateTicket() error = %v", err)
		}
	}

	events, err := ListHistoryEvents(context.Background(), db, ticket.ID, 2, 1)
	if err != nil {
		t.Fatalf("ListHistoryEvents(limit, offset) error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ListHistoryEvents(limit, offset) len = %d, want 2", len(events))
	}

	if _, err := ListHistoryEvents(context.Background(), db, ticket.ID, -1, 0); err == nil {
		t.Fatal("ListHistoryEvents(negative limit) error = nil, want error")
	}
	if _, err := ListHistoryEvents(context.Background(), db, ticket.ID, 1, -1); err == nil {
		t.Fatal("ListHistoryEvents(negative offset) error = nil, want error")
	}
}
