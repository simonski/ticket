package store

import "testing"

func TestHistoryAndComments(t *testing.T) {
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(db, "Customer Portal", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add login",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	events, err := ListHistoryEvents(db, ticket.ID)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	if len(events) == 0 || events[0].EventType != "ticket_created" {
		t.Fatalf("history after create = %#v", events)
	}

	_, err = UpdateTicket(db, ticket.ID, TicketUpdateParams{
		Title:       ticket.Title,
		Description: "Updated description",
		ParentID:    ticket.ParentID,
		UpdatedBy:   "",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	events, err = ListHistoryEvents(db, ticket.ID)
	if err != nil {
		t.Fatalf("ListHistoryEvents(after update) error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("history length = %d, want at least 2", len(events))
	}

	comment, err := AddComment(db, ticket.ID, adminID, "Waiting on API changes.")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.Comment != "Waiting on API changes." {
		t.Fatalf("AddComment().Comment = %q", comment.Comment)
	}

	comments, err := ListComments(db, ticket.ID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("ListComments() len = %d, want 1", len(comments))
	}
	if comments[0].Author != "admin" || comments[0].Text != "Waiting on API changes." {
		t.Fatalf("ListComments() = %#v", comments)
	}

	taskWithComments, err := GetTicket(db, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if len(taskWithComments.Comments) != 1 || taskWithComments.Comments[0].Author != "admin" {
		t.Fatalf("GetTicket().Comments = %#v", taskWithComments.Comments)
	}
}

func TestListProjectHistory(t *testing.T) {
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(db, "History Project", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "History task",
		CreatedBy: adminID,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Update ticket to create more history
	if _, err := UpdateTicket(db, ticket.ID, TicketUpdateParams{
		Title:       "Updated history task",
		Description: "desc",
		ParentID:    ticket.ParentID,
		UpdatedBy:   adminID,
	}); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	events, err := ListProjectHistory(db, project.ID, 10)
	if err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("ListProjectHistory() len = %d, want >= 1", len(events))
	}

	// Test with default limit (0)
	events, err = ListProjectHistory(db, project.ID, 0)
	if err != nil {
		t.Fatalf("ListProjectHistory(limit=0) error = %v", err)
	}
	if len(events) < 1 {
		t.Fatalf("ListProjectHistory(limit=0) len = %d, want >= 1", len(events))
	}
}
