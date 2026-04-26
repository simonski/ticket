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

func TestPurgeExpiredSessions(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	adminID := testAdminID(t, db)

	expiredToken, err := CreateSession(context.Background(), db, adminID)
	if err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}
	activeToken, err := CreateSession(context.Background(), db, adminID)
	if err != nil {
		t.Fatalf("CreateSession(active) error = %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE sessions SET expires_at = datetime('now', '-2 days') WHERE token = ?`, expiredToken); err != nil {
		t.Fatalf("expire session update error = %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE sessions SET expires_at = datetime('now', '+2 days') WHERE token = ?`, activeToken); err != nil {
		t.Fatalf("active session update error = %v", err)
	}

	deleted, err := PurgeExpiredSessions(context.Background(), db)
	if err != nil {
		t.Fatalf("PurgeExpiredSessions() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("PurgeExpiredSessions() = %d, want 1", deleted)
	}

	if _, err := GetUserByToken(context.Background(), db, expiredToken); err == nil {
		t.Fatal("GetUserByToken(expired) error = nil, want deleted session")
	}
	if _, err := GetUserByToken(context.Background(), db, activeToken); err != nil {
		t.Fatalf("GetUserByToken(active) error = %v", err)
	}
}

func TestPurgeOldHistory(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(context.Background(), db, "Retention Project", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Retention task",
		CreatedBy: adminID,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := AddHistoryEvent(context.Background(), db, project.ID, ticket.ID, "old_event", map[string]any{"age": "old"}, adminID); err != nil {
		t.Fatalf("AddHistoryEvent(old) error = %v", err)
	}
	if err := AddHistoryEvent(context.Background(), db, project.ID, ticket.ID, "new_event", map[string]any{"age": "new"}, adminID); err != nil {
		t.Fatalf("AddHistoryEvent(new) error = %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE ticket_history SET created_at = datetime('now', '-10 days') WHERE event_type = 'old_event'`); err != nil {
		t.Fatalf("age old history error = %v", err)
	}

	deleted, err := PurgeOldHistory(context.Background(), db, 5)
	if err != nil {
		t.Fatalf("PurgeOldHistory() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("PurgeOldHistory() = %d, want 1", deleted)
	}
	events, err := ListHistoryEvents(context.Background(), db, ticket.ID, 20, 0)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	for _, event := range events {
		if event.EventType == "old_event" {
			t.Fatalf("old history event still present: %#v", events)
		}
	}

	deleted, err = PurgeOldHistory(context.Background(), db, 0)
	if err != nil {
		t.Fatalf("PurgeOldHistory(retention=0) error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("PurgeOldHistory(retention=0) = %d, want 0", deleted)
	}
}
