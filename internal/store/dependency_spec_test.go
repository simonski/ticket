package store

import "testing"

func TestDependencies(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	source, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Prepare password reset flow",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(source) error = %v", err)
	}
	blocker, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Password Reset",
		Stage:     StageDesign,
		State:     StateIdle,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(blocker) error = %v", err)
	}
	dependent, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "bug",
		Title:     "Reset link expires immediately.",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(dependent) error = %v", err)
	}
	if _, err := AddDependency(db, project.ID, dependent.ID, blocker.ID, 1); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	dependencies, err := ListDependencies(db, source.ID)
	if err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
	if len(dependencies) != 0 {
		t.Fatalf("ListDependencies(source) len = %d, want 0", len(dependencies))
	}

	dependencies, err = ListDependencies(db, dependent.ID)
	if err != nil {
		t.Fatalf("ListDependencies(dependent) error = %v", err)
	}
	if len(dependencies) != 1 {
		t.Fatalf("ListDependencies(dependent) len = %d, want 1", len(dependencies))
	}
	if err := DeleteDependency(db, project.ID, dependent.ID, blocker.ID); err != nil {
		t.Fatalf("DeleteDependency() error = %v", err)
	}
	dependencies, err = ListDependencies(db, dependent.ID)
	if err != nil {
		t.Fatalf("ListDependencies(dependent after delete) error = %v", err)
	}
	if len(dependencies) != 0 {
		t.Fatalf("ListDependencies(dependent after delete) len = %d, want 0", len(dependencies))
	}

}
