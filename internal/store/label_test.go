package store

import "testing"

func TestLabelCRUD(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Label Test", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create
	label, err := CreateLabel(db, project.ID, "bug", "#ff0000")
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if label.Name != "bug" || label.Color != "#ff0000" {
		t.Fatalf("CreateLabel() = %+v", label)
	}

	// Get
	got, err := GetLabel(db, label.ID)
	if err != nil {
		t.Fatalf("GetLabel() error = %v", err)
	}
	if got.Name != "bug" {
		t.Fatalf("GetLabel().Name = %q", got.Name)
	}

	// List
	_, _ = CreateLabel(db, project.ID, "feature", "#00ff00")
	labels, err := ListLabels(db, project.ID)
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ListLabels() len = %d, want 2", len(labels))
	}
	if labels[0].Name != "bug" || labels[1].Name != "feature" {
		t.Fatalf("ListLabels() order = %q, %q", labels[0].Name, labels[1].Name)
	}

	// Empty name
	if _, err := CreateLabel(db, project.ID, "", ""); err == nil {
		t.Fatal("CreateLabel() with empty name should fail")
	}

	// Duplicate name
	if _, err := CreateLabel(db, project.ID, "bug", ""); err == nil {
		t.Fatal("CreateLabel() duplicate name should fail")
	}

	// Delete
	if err := DeleteLabel(db, label.ID); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
	if _, err := GetLabel(db, label.ID); err != ErrLabelNotFound {
		t.Fatalf("GetLabel() after delete = %v, want ErrLabelNotFound", err)
	}
}

func TestTicketLabels(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Ticket Labels", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Labeled Task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	label1, _ := CreateLabel(db, project.ID, "priority", "#ff0000")
	label2, _ := CreateLabel(db, project.ID, "ui", "#0000ff")

	// Add labels
	if err := AddTicketLabel(db, ticket.ID, label1.ID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	if err := AddTicketLabel(db, ticket.ID, label2.ID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}

	// Idempotent add
	if err := AddTicketLabel(db, ticket.ID, label1.ID); err != nil {
		t.Fatalf("AddTicketLabel() duplicate error = %v", err)
	}

	// List ticket labels
	labels, err := ListTicketLabels(db, ticket.ID)
	if err != nil {
		t.Fatalf("ListTicketLabels() error = %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ListTicketLabels() len = %d, want 2", len(labels))
	}

	// List tickets by label
	ids, err := ListTicketsByLabel(db, label1.ID)
	if err != nil {
		t.Fatalf("ListTicketsByLabel() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != ticket.ID {
		t.Fatalf("ListTicketsByLabel() = %v, want [%s]", ids, ticket.ID)
	}

	// Remove label
	if err := RemoveTicketLabel(db, ticket.ID, label1.ID); err != nil {
		t.Fatalf("RemoveTicketLabel() error = %v", err)
	}
	labels, _ = ListTicketLabels(db, ticket.ID)
	if len(labels) != 1 {
		t.Fatalf("ListTicketLabels() after remove len = %d, want 1", len(labels))
	}

	// Delete label cascades from ticket_labels
	if err := DeleteLabel(db, label2.ID); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
	labels, _ = ListTicketLabels(db, ticket.ID)
	if len(labels) != 0 {
		t.Fatalf("ListTicketLabels() after label delete len = %d, want 0", len(labels))
	}
}
