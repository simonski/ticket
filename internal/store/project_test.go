package store

import "testing"

func TestCreateListAndGetProject(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Customer Portal", "Portal work", "Ship the portal safely.", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.AcceptanceCriteria != "Ship the portal safely." {
		t.Fatalf("CreateProject().AcceptanceCriteria = %q", project.AcceptanceCriteria)
	}

	projects, err := ListProjects(db)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("ListProjects() len = %d, want 2", len(projects))
	}

	if projects[0].Title != "Default Project" {
		t.Fatalf("ListProjects()[0].Title = %q, want Default Project", projects[0].Title)
	}

	byID, err := GetProject(db, "2")
	if err != nil {
		t.Fatalf("GetProject(id) error = %v", err)
	}
	if byID.ID != project.ID {
		t.Fatalf("GetProject(id).ID = %d, want %d", byID.ID, project.ID)
	}
}

func TestUpdateAndEnableDisableProject(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(db, "Customer Portal", "Portal work", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	updated, err := UpdateProject(db, project.ID, "New Title", "New Description", "New AC")
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if updated.Title != "New Title" || updated.Description != "New Description" || updated.AcceptanceCriteria != "New AC" {
		t.Fatalf("UpdateProject() = %#v", updated)
	}

	disabled, err := SetProjectStatus(db, project.ID, false)
	if err != nil {
		t.Fatalf("SetProjectStatus(disable) error = %v", err)
	}
	if disabled.Status != "closed" {
		t.Fatalf("SetProjectStatus(disable).Status = %q", disabled.Status)
	}

	enabled, err := SetProjectStatus(db, project.ID, true)
	if err != nil {
		t.Fatalf("SetProjectStatus(enable) error = %v", err)
	}
	if enabled.Status != "open" {
		t.Fatalf("SetProjectStatus(enable).Status = %q", enabled.Status)
	}
}
