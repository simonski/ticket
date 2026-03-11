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

func TestProjectVisibilityAndVisibleListing(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	alice, err := CreateUser(db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	privateProject, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:     "PRV",
		Title:      "Private Project",
		Visibility: ProjectVisibilityPrivate,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams(private) error = %v", err)
	}
	publicProject, err := CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:     "PUB",
		Title:      "Public Project",
		Visibility: ProjectVisibilityPublic,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams(public) error = %v", err)
	}
	if privateProject.Visibility != ProjectVisibilityPrivate {
		t.Fatalf("privateProject.Visibility = %q, want %q", privateProject.Visibility, ProjectVisibilityPrivate)
	}
	if publicProject.Visibility != ProjectVisibilityPublic {
		t.Fatalf("publicProject.Visibility = %q, want %q", publicProject.Visibility, ProjectVisibilityPublic)
	}

	visible, err := ListProjectsVisibleToUser(db, alice)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser(alice) error = %v", err)
	}
	if len(visible) != 2 {
		t.Fatalf("ListProjectsVisibleToUser(alice) len = %d, want 2 (default + public)", len(visible))
	}
	for _, project := range visible {
		if project.ID == privateProject.ID {
			t.Fatalf("private project should not be visible without membership")
		}
	}

	if _, err := AddProjectMember(db, privateProject.ID, alice.ID, ProjectRoleViewer); err != nil {
		t.Fatalf("AddProjectMember(viewer) error = %v", err)
	}
	visible, err = ListProjectsVisibleToUser(db, alice)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser(alice, member) error = %v", err)
	}
	foundPrivate := false
	for _, project := range visible {
		if project.ID == privateProject.ID {
			foundPrivate = true
			break
		}
	}
	if !foundPrivate {
		t.Fatalf("private project should be visible once membership exists")
	}
}
