package store

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func TestCreateListAndGetProject(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Customer Portal", "Portal work", "Ship the portal safely.", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.AcceptanceCriteria != "Ship the portal safely." {
		t.Fatalf("CreateProject().AcceptanceCriteria = %q", project.AcceptanceCriteria)
	}

	projects, err := ListProjects(context.Background(), db, 0)
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 4 {
		t.Fatalf("ListProjects() len = %d, want 4", len(projects))
	}

	foundPrivate := false
	foundTicket := false
	for _, listed := range projects {
		if listed.Title == "Private" && listed.Prefix == "PRIV" {
			foundPrivate = true
		}
		if listed.Title == ticketProjectTitle && listed.Prefix == ticketProjectPrefix && listed.GitRepository == ticketProjectRepo {
			foundTicket = true
		}
	}
	if !foundPrivate {
		t.Fatalf("ListProjects() missing seeded Private project: %#v", projects)
	}
	if !foundTicket {
		t.Fatalf("ListProjects() missing seeded ticket project: %#v", projects)
	}

	byID, err := GetProject(context.Background(), db, strconv.FormatInt(project.ID, 10))
	if err != nil {
		t.Fatalf("GetProject(id) error = %v", err)
	}
	if byID.ID != project.ID {
		t.Fatalf("GetProject(id).ID = %d, want %d", byID.ID, project.ID)
	}
}

func TestUpdateAndEnableDisableProject(t *testing.T) {
	t.Parallel()
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

func TestProjectGitRepositories(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Title:         "Repo Project",
		Prefix:        "REP",
		GitRepository: "github.com/acme/one.git",
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}

	repositories, err := ListProjectGitRepositories(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListProjectGitRepositories() error = %v", err)
	}
	if !reflect.DeepEqual(repositories, []string{"github.com/acme/one.git"}) {
		t.Fatalf("ListProjectGitRepositories() = %#v", repositories)
	}

	if err := AddProjectGitRepository(context.Background(), db, project.ID, "github.com/acme/two.git"); err != nil {
		t.Fatalf("AddProjectGitRepository() error = %v", err)
	}
	repositories, err = ListProjectGitRepositories(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListProjectGitRepositories(second) error = %v", err)
	}
	if !reflect.DeepEqual(repositories, []string{"github.com/acme/one.git", "github.com/acme/two.git"}) {
		t.Fatalf("ListProjectGitRepositories(second) = %#v", repositories)
	}

	byRepo, err := GetProjectByGitRepository(context.Background(), db, "github.com/acme/two.git")
	if err != nil {
		t.Fatalf("GetProjectByGitRepository() error = %v", err)
	}
	if byRepo.ID != project.ID {
		t.Fatalf("GetProjectByGitRepository().ID = %d, want %d", byRepo.ID, project.ID)
	}

	if err := RemoveProjectGitRepository(context.Background(), db, project.ID, "github.com/acme/one.git"); err != nil {
		t.Fatalf("RemoveProjectGitRepository() error = %v", err)
	}
	updated, err := GetProjectByID(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if updated.GitRepository != "github.com/acme/two.git" {
		t.Fatalf("GetProjectByID().GitRepository = %q", updated.GitRepository)
	}
}

func TestSetProjectDefaultDraft(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Draft Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if err := SetProjectDefaultDraft(context.Background(), db, project.ID, true); err != nil {
		t.Fatalf("SetProjectDefaultDraft(true) error = %v", err)
	}
	updated, err := GetProjectByID(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if !updated.DefaultDraft {
		t.Fatalf("DefaultDraft = %v, want true", updated.DefaultDraft)
	}

	if err := SetProjectDefaultDraft(context.Background(), db, project.ID, false); err != nil {
		t.Fatalf("SetProjectDefaultDraft(false) error = %v", err)
	}
	updated, err = GetProjectByID(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() after false error = %v", err)
	}
	if updated.DefaultDraft {
		t.Fatalf("DefaultDraft = %v, want false", updated.DefaultDraft)
	}

	if err := SetProjectDefaultDraft(context.Background(), db, 9999, true); err != ErrProjectNotFound {
		t.Fatalf("SetProjectDefaultDraft(missing) error = %v, want %v", err, ErrProjectNotFound)
	}
}

func TestProjectGuidanceMapsPersistAndResolve(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix:             "MAP",
		Title:              "Guidance Project",
		AcceptanceCriteria: "legacy acceptance",
		DORMap:             GuidanceMap{"default": "project default dor", "develop": "project develop dor"},
		DODMap:             GuidanceMap{"default": "project default dod"},
		ACMap:              GuidanceMap{"qa": "qa acceptance"},
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if !reflect.DeepEqual(project.DORMap, GuidanceMap{"default": "project default dor", "develop": "project develop dor"}) {
		t.Fatalf("CreateProjectWithParams().DORMap = %#v", project.DORMap)
	}
	if !reflect.DeepEqual(project.ACMap, GuidanceMap{"default": "legacy acceptance", "qa": "qa acceptance"}) {
		t.Fatalf("CreateProjectWithParams().ACMap = %#v", project.ACMap)
	}

	reloaded, err := GetProjectByID(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.DODMap, GuidanceMap{"default": "project default dod"}) {
		t.Fatalf("GetProjectByID().DODMap = %#v", reloaded.DODMap)
	}
	resolved := reloaded.ResolveGuidance("develop")
	if !resolved.HasDOR || resolved.DOR != "project develop dor" {
		t.Fatalf("ResolveGuidance(develop).DOR = %#v", resolved)
	}
	if !resolved.HasDOD || resolved.DOD != "project default dod" {
		t.Fatalf("ResolveGuidance(develop).DOD = %#v", resolved)
	}
	if !resolved.HasAC || resolved.AC != "legacy acceptance" {
		t.Fatalf("ResolveGuidance(develop).AC = %#v", resolved)
	}

	updated, err := UpdateProjectWithParams(context.Background(), db, project.ID, ProjectUpdateParams{
		Title:  project.Title,
		DORMap: GuidanceMap{"qa": "project qa dor"},
		DODMap: GuidanceMap{"qa": "project qa dod"},
		ACMap:  GuidanceMap{"qa": "project qa ac"},
	})
	if err != nil {
		t.Fatalf("UpdateProjectWithParams() error = %v", err)
	}
	if !reflect.DeepEqual(updated.DORMap, GuidanceMap{"qa": "project qa dor"}) {
		t.Fatalf("UpdateProjectWithParams().DORMap = %#v", updated.DORMap)
	}
	if !reflect.DeepEqual(updated.ACMap, GuidanceMap{"default": "legacy acceptance", "qa": "project qa ac"}) {
		t.Fatalf("UpdateProjectWithParams().ACMap = %#v", updated.ACMap)
	}
}

func TestGetProjectByPrefix(t *testing.T) {
	t.Parallel()
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

func TestGetProjectByTitle(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix: "TTL",
		Title:  "Title Lookup Project",
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}

	byTitle, err := GetProject(context.Background(), db, "Title Lookup Project")
	if err != nil {
		t.Fatalf("GetProject(title) error = %v", err)
	}
	if byTitle.ID != project.ID {
		t.Fatalf("GetProject(title).ID = %d, want %d", byTitle.ID, project.ID)
	}

	byTitleLower, err := GetProject(context.Background(), db, "title lookup project")
	if err != nil {
		t.Fatalf("GetProject(lower title) error = %v", err)
	}
	if byTitleLower.ID != project.ID {
		t.Fatalf("GetProject(lower title).ID = %d, want %d", byTitleLower.ID, project.ID)
	}
}

func TestGetProjectByTitleReturnsAmbiguousError(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	if _, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix: "AMB",
		Title:  "Duplicated Title",
	}); err != nil {
		t.Fatalf("CreateProjectWithParams(first) error = %v", err)
	}
	if _, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix: "AM2",
		Title:  "Duplicated Title",
	}); err != nil {
		t.Fatalf("CreateProjectWithParams(second) error = %v", err)
	}

	_, err := GetProject(context.Background(), db, "Duplicated Title")
	if !errors.Is(err, ErrProjectAmbiguous) {
		t.Fatalf("GetProject(ambiguous title) error = %v, want %v", err, ErrProjectAmbiguous)
	}
}

func TestProjectVisibilityAndVisibleListing(t *testing.T) {
	t.Parallel()
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
	teamProject, err := CreateProjectWithParams(context.Background(), db, ProjectCreateParams{
		Prefix:     "TEM",
		Title:      "Team Project",
		Visibility: ProjectVisibilityTeam,
		CreatedBy:  admin.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams(team) error = %v", err)
	}
	if privateProject.Visibility != ProjectVisibilityPrivate {
		t.Fatalf("privateProject.Visibility = %q, want %q", privateProject.Visibility, ProjectVisibilityPrivate)
	}
	if publicProject.Visibility != ProjectVisibilityPublic {
		t.Fatalf("publicProject.Visibility = %q, want %q", publicProject.Visibility, ProjectVisibilityPublic)
	}
	if teamProject.Visibility != ProjectVisibilityTeam {
		t.Fatalf("teamProject.Visibility = %q, want %q", teamProject.Visibility, ProjectVisibilityTeam)
	}

	visible, err := ListProjectsVisibleToUser(context.Background(), db, alice)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser(alice) error = %v", err)
	}
	if len(visible) != 3 {
		t.Fatalf("ListProjectsVisibleToUser(alice) len = %d, want 3 (seeded public + alice private alias + public project)", len(visible))
	}
	for _, project := range visible {
		if project.ID == privateProject.ID {
			t.Fatalf("private project should not be visible without membership")
		}
		if project.ID == teamProject.ID {
			t.Fatalf("team project should not be visible without membership")
		}
	}

	if _, err := AddProjectMember(context.Background(), db, privateProject.ID, alice.ID, ProjectRoleViewer); err != nil {
		t.Fatalf("AddProjectMember(viewer) error = %v", err)
	}
	visible, err = ListProjectsVisibleToUser(context.Background(), db, alice)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser(alice, member) error = %v", err)
	}
	if len(visible) != 4 {
		t.Fatalf("ListProjectsVisibleToUser(alice, member) len = %d, want 4", len(visible))
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

	team, err := CreateTeam(context.Background(), db, "Customer Team", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if _, err := AddTeamMember(context.Background(), db, team.ID, alice.ID, TeamRoleMember, "Engineer"); err != nil {
		t.Fatalf("AddTeamMember() error = %v", err)
	}
	if _, err := AddProjectTeamMember(context.Background(), db, teamProject.ID, team.ID, ProjectRoleObserver); err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}

	visible, err = ListProjectsVisibleToUser(context.Background(), db, alice)
	if err != nil {
		t.Fatalf("ListProjectsVisibleToUser(alice, team member) error = %v", err)
	}
	foundTeam := false
	for _, project := range visible {
		if project.ID == teamProject.ID {
			foundTeam = true
			break
		}
	}
	if !foundTeam {
		t.Fatalf("team project should be visible once team membership exists")
	}
}

func TestRenameProjectPrefix(t *testing.T) {
	t.Parallel()
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

	// Verify tickets have new keys (PREFIX-N format, no type code).
	newEpic, err := GetTicket(context.Background(), db, "NEW-1")
	if err != nil {
		t.Fatalf("GetTicket(NEW-1) error = %v", err)
	}
	if newEpic.Title != "Epic" {
		t.Fatalf("epic title = %q", newEpic.Title)
	}

	newTask, err := GetTicket(context.Background(), db, "NEW-2")
	if err != nil {
		t.Fatalf("GetTicket(NEW-2) error = %v", err)
	}
	if newTask.ParentID == nil || *newTask.ParentID != "NEW-1" {
		t.Fatalf("task parent = %v, want NEW-1", newTask.ParentID)
	}

	// Verify dependency updated.
	deps, err := ListDependencies(context.Background(), db, "NEW-2")
	if err != nil {
		t.Fatalf("ListDependencies error = %v", err)
	}
	if len(deps) != 1 || deps[0].DependsOn != "NEW-3" {
		t.Fatalf("dependency = %v, want [NEW-3]", deps)
	}

	// Verify old keys are gone.
	if _, err := GetTicket(context.Background(), db, "OLD-1"); err == nil {
		t.Fatal("old key OLD-1 should not exist")
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
	if _, err := RenameProjectPrefix(context.Background(), db, project.ID, "1!"); err == nil {
		t.Fatal("RenameProjectPrefix(invalid) should fail")
	}
}

func TestDeleteProject(t *testing.T) {
	t.Parallel()
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
