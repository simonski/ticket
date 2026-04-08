package store

import (
	"context"
	"testing"
)

func TestCreateListAndGetProject(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Customer Portal", "Portal work", "Ship the portal safely.", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.AcceptanceCriteria != "Ship the portal safely." {
		t.Fatalf("CreateProject().AcceptanceCriteria = %q", project.AcceptanceCriteria)
	}

	projects, err := ListProjects(context.Background(), db)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("ListProjects() len = %d, want 2", len(projects))
	}

	if projects[0].Title != "Default Project" {
		t.Fatalf("ListProjects()[0].Title = %q, want Default Project", projects[0].Title)
	}

	byID, err := GetProject(context.Background(), db, "2")
	if err != nil {
		t.Fatalf("GetProject(id) error = %v", err)
	}
	if byID.ID != project.ID {
		t.Fatalf("GetProject(id).ID = %d, want %d", byID.ID, project.ID)
	}
}

func TestUpdateAndEnableDisableProject(t *testing.T) {
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Customer Portal", "Portal work", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	updated, err := UpdateProject(context.Background(), db, project.ID, "New Title", "New Description", "New AC")
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if updated.Title != "New Title" || updated.Description != "New Description" || updated.AcceptanceCriteria != "New AC" {
		t.Fatalf("UpdateProject() = %#v", updated)
	}

	disabled, err := SetProjectStatus(context.Background(), db, project.ID, false)
	if err != nil {
		t.Fatalf("SetProjectStatus(disable) error = %v", err)
	}
	if disabled.Status != "closed" {
		t.Fatalf("SetProjectStatus(disable).Status = %q", disabled.Status)
	}

	enabled, err := SetProjectStatus(context.Background(), db, project.ID, true)
	if err != nil {
		t.Fatalf("SetProjectStatus(enable) error = %v", err)
	}
	if enabled.Status != "open" {
		t.Fatalf("SetProjectStatus(enable).Status = %q", enabled.Status)
	}
}

func TestGetProjectByPrefix(t *testing.T) {
	db := testDB(t)

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix: "ABC",
		Title:  "Prefix Project",
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}

	// Get by prefix
	byPrefix, err := GetProject(context.Background(), db, "ABC")
	if err != nil {
		t.Fatalf("GetProject(prefix) error = %v", err)
	}
	if byPrefix.ID != project.ID {
		t.Fatalf("GetProject(prefix).ID = %d, want %d", byPrefix.ID, project.ID)
	}

	// Get by prefix lowercase
	byLower, err := GetProject(context.Background(), db, "abc")
	if err != nil {
		t.Fatalf("GetProject(lower prefix) error = %v", err)
	}
	if byLower.ID != project.ID {
		t.Fatalf("GetProject(lower prefix).ID = %d, want %d", byLower.ID, project.ID)
	}

	// Get with empty string
	if _, err := GetProject(context.Background(), db, ""); err == nil {
		t.Fatal("GetProject(empty) error = nil, want error")
	}

	// Get nonexistent prefix
	if _, err := GetProject(context.Background(), db, "ZZZ"); err == nil {
		t.Fatal("GetProject(nonexistent) error = nil, want error")
	}
}

func TestProjectVisibilityAndVisibleListing(t *testing.T) {
	db := testDB(t)

	admin, err := GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	privateProject, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix:     "PRV",
		Title:      "Private Project",
		Visibility: ProjectVisibilityPrivate,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams(private) error = %v", err)
	}
	publicProject, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
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

	visible, err := ListProjectsVisibleToUser(context.Background(), db, alice)
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

	if _, err := AddProjectMember(context.Background(), db, privateProject.ID, alice.ID, ProjectRoleViewer); err != nil {
		t.Fatalf("AddProjectMember(viewer) error = %v", err)
	}
	visible, err = ListProjectsVisibleToUser(context.Background(), db, alice)
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

func TestRenameProjectPrefix(t *testing.T) {
	db := testDB(t)

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix: "OLD",
		Title:  "Rename Me",
	})
	if err != nil {
		t.Fatalf("CreateProject error = %v", err)
	}

	// Create tickets of different types.
	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{ProjectID: project.ID, Type: "epic", Title: "Epic"})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	parentID := epic.ID
	task, err := CreateTicket(context.Background(), db, TicketCreateParams{ProjectID: project.ID, Type: "task", Title: "Task", ParentID: &parentID})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	bug, err := CreateTicket(context.Background(), db, TicketCreateParams{ProjectID: project.ID, Type: "bug", Title: "Bug"})
	if err != nil {
		t.Fatalf("CreateTicket(bug) error = %v", err)
	}

	// Add a dependency, comment, and time entry.
	adminID := testAdminID(t, db)
	if _, err := AddDependency(context.Background(), db, project.ID, task.ID, bug.ID, adminID); err != nil {
		t.Fatalf("AddDependency error = %v", err)
	}
	if _, err := AddComment(context.Background(), db, task.ID, adminID, "a comment"); err != nil {
		t.Fatalf("AddComment error = %v", err)
	}
	if _, err := LogTime(context.Background(), db, task.ID, adminID, 30, "work"); err != nil {
		t.Fatalf("LogTime error = %v", err)
	}

	// Rename.
	count, err := RenameProjectPrefix(context.Background(), db, project.ID, "NEW")
	if err != nil {
		t.Fatalf("RenameProjectPrefix error = %v", err)
	}
	if count != 3 {
		t.Fatalf("RenameProjectPrefix count = %d, want 3", count)
	}

	// Verify project prefix updated.
	updated, err := GetProjectByID(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID error = %v", err)
	}
	if updated.Prefix != "NEW" {
		t.Fatalf("project prefix = %q, want NEW", updated.Prefix)
	}

	// Verify tickets have new keys.
	newEpic, err := GetTicket(context.Background(), db, "NEW-E-1")
	if err != nil {
		t.Fatalf("GetTicket(NEW-E-1) error = %v", err)
	}
	if newEpic.Title != "Epic" {
		t.Fatalf("epic title = %q", newEpic.Title)
	}

	newTask, err := GetTicket(context.Background(), db, "NEW-T-2")
	if err != nil {
		t.Fatalf("GetTicket(NEW-T-2) error = %v", err)
	}
	if newTask.ParentID == nil || *newTask.ParentID != "NEW-E-1" {
		t.Fatalf("task parent = %v, want NEW-E-1", newTask.ParentID)
	}

	// Verify dependency updated.
	deps, err := ListDependencies(context.Background(), db, "NEW-T-2")
	if err != nil {
		t.Fatalf("ListDependencies error = %v", err)
	}
	if len(deps) != 1 || deps[0].DependsOn != "NEW-B-3" {
		t.Fatalf("dependency = %v, want [NEW-B-3]", deps)
	}

	// Verify old keys are gone.
	if _, err := GetTicket(context.Background(), db, "OLD-E-1"); err == nil {
		t.Fatal("old key OLD-E-1 should not exist")
	}

	// Renaming to same prefix is a no-op.
	count, err = RenameProjectPrefix(context.Background(), db, project.ID, "NEW")
	if err != nil {
		t.Fatalf("RenameProjectPrefix(same) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("RenameProjectPrefix(same) count = %d, want 0", count)
	}

	// Renaming to an invalid prefix fails.
	if _, err := RenameProjectPrefix(context.Background(), db, project.ID, "x"); err == nil {
		t.Fatal("RenameProjectPrefix(invalid) should fail")
	}
}

func TestDeleteProject(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Delete Me", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create a ticket with comments, time entries, labels, dependencies
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID, Type: "task", Title: "Task to delete",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	adminID := testAdminID(t, db)
	if _, err := AddComment(context.Background(), db, ticket.ID, adminID, "test comment"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if _, err := LogTime(context.Background(), db, ticket.ID, adminID, 30, "work"); err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}

	if err := DeleteProject(context.Background(), db, project.ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := GetProjectByID(context.Background(), db, project.ID); err == nil {
		t.Fatal("GetProjectByID after delete should fail")
	}
}
