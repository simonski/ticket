package store

import "testing"

func TestTimeEntryCRUD(t *testing.T) {
	db := testDB(t)
	adminID := testAdminID(t, db)

	project, err := CreateProject(db, "Time Test", "", "", adminID)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Timed Task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Log time
	entry, err := LogTime(db, ticket.ID, adminID, 30, "initial work")
	if err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	if entry.Minutes != 30 || entry.Note != "initial work" {
		t.Fatalf("LogTime() = %+v", entry)
	}

	// Log more time
	_, err = LogTime(db, ticket.ID, adminID, 45, "follow-up")
	if err != nil {
		t.Fatalf("LogTime() second error = %v", err)
	}

	// Invalid minutes
	if _, err := LogTime(db, ticket.ID, adminID, 0, ""); err == nil {
		t.Fatal("LogTime() with 0 minutes should fail")
	}

	// List
	entries, err := ListTimeEntries(db, ticket.ID)
	if err != nil {
		t.Fatalf("ListTimeEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListTimeEntries() len = %d, want 2", len(entries))
	}

	// Total
	total, err := TotalTimeForTicket(db, ticket.ID)
	if err != nil {
		t.Fatalf("TotalTimeForTicket() error = %v", err)
	}
	if total != 75 {
		t.Fatalf("TotalTimeForTicket() = %d, want 75", total)
	}

	// Delete
	if err := DeleteTimeEntry(db, entry.ID); err != nil {
		t.Fatalf("DeleteTimeEntry() error = %v", err)
	}
	entries, _ = ListTimeEntries(db, ticket.ID)
	if len(entries) != 1 {
		t.Fatalf("ListTimeEntries() after delete len = %d, want 1", len(entries))
	}

	total, _ = TotalTimeForTicket(db, ticket.ID)
	if total != 45 {
		t.Fatalf("TotalTimeForTicket() after delete = %d, want 45", total)
	}
}
