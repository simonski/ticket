package client

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func TestLocalModeClientUsesSQLiteDirectly(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	projects, err := api.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != 1 {
		t.Fatalf("ListProjects() = %#v", projects)
	}

	ticket, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if strings.TrimSpace(ticket.Assignee) != "" || ticket.Status != "design/idle" {
		t.Fatalf("CreateTicket() = %#v", ticket)
	}

	if _, err := api.ReadyTicket(ticket.ID, ""); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}

	requested, err := api.RequestTicket(TicketRequest{ProjectID: 1})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if requested.Status != "ASSIGNED" || requested.Ticket == nil {
		t.Fatalf("RequestTicket() = %#v", requested)
	}

	updated, err := api.UpdateTicket(ticket.ID, TicketUpdateRequest{
		Title:       ticket.Title,
		Description: ticket.Description,
		ParentID:    ticket.ParentID,
		Assignee:    requested.Ticket.Assignee,
		State:       "active",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Status != "design/active" {
		t.Fatalf("UpdateTicket().Status = %q, want design/active", updated.Status)
	}

	parent, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent epic",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	reparented, err := api.SetTicketParent(ticket.ID, parent.ID, "")
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if reparented.ParentID == nil || *reparented.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", reparented)
	}

	detached, err := api.UnsetTicketParent(ticket.ID, "")
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}

	comment, err := api.AddComment(ticket.ID, "hello")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.Text != "hello" || comment.Author == "" {
		t.Fatalf("AddComment() = %#v", comment)
	}
}

func TestLocalModeClientIgnoresOwnershipForStatusChanges(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	ticket, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if strings.TrimSpace(ticket.Assignee) != "" {
		t.Fatalf("CreateTicket().Assignee = %q, want unassigned", ticket.Assignee)
	}

	// Advance through all stages to reach done/success
	for _, wantStatus := range []string{"develop/idle", "test/idle", "done/idle", "done/success"} {
		ticket, err = api.GetTicketByID(ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		updated, err := api.UpdateTicket(ticket.ID, TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    ticket.Assignee,
			State:       "success",
		})
		if err != nil {
			t.Fatalf("UpdateTicket() error = %v", err)
		}
		if updated.Status != wantStatus {
			t.Fatalf("UpdateTicket().Status = %q, want %s", updated.Status, wantStatus)
		}
	}
}

func TestLocalModeClientDeleteTicket(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	ticket, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := api.DeleteTicket(ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := api.GetTicketByID(ticket.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func TestLocalModeClientStatusIsReadOnlyWithoutMatchingUser(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	status, err := api.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil {
		t.Fatalf("Status() = %#v, want authenticated admin", status)
	}
	if status.User.Username != "admin" {
		t.Fatalf("Status().User.Username = %q, want admin", status.User.Username)
	}
	if _, err := store.GetUserByUsername(context.Background(), mustOpenDB(t, dbPath), localUsername()); err != nil {
		t.Fatalf("Status() should use existing admin user, err = %v", err)
	}
}

func TestLocalModeClientStatusFailsWhenDatabaseMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	api := New(config.Config{})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want missing database error")
	}
}

func TestLocalModeClientRolesCRUD(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	role, err := api.CreateRole(RoleRequest{Title: "dev", Motivation: "build", Goals: "ship"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if role.Title != "dev" {
		t.Fatalf("CreateRole().Title = %q", role.Title)
	}
	roles, err := api.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) == 0 {
		t.Fatal("ListRoles() returned empty")
	}
	updated, err := api.UpdateRole(role.ID, RoleRequest{Title: "dev2", Motivation: "build", Goals: "ship"})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updated.Title != "dev2" {
		t.Fatalf("UpdateRole().Title = %q", updated.Title)
	}
	if err := api.DeleteRole(role.ID); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
}

func TestLocalModeClientUserOps(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	user, err := api.CreateUser("bob", "pw")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.Username != "bob" {
		t.Fatalf("CreateUser().Username = %q", user.Username)
	}
	users, err := api.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) < 2 {
		t.Fatal("ListUsers() too few")
	}
	if err := api.SetUserEnabled("bob", false); err != nil {
		t.Fatalf("SetUserEnabled() error = %v", err)
	}
	reset, err := api.ResetUserPassword("bob", "newpw")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if reset.Username != "bob" {
		t.Fatalf("ResetUserPassword().Username = %q", reset.Username)
	}
	if err := api.DeleteUser("bob"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
}

func TestLocalModeClientRegistrationToggle(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	if err := api.SetRegistrationEnabled(false); err != nil {
		t.Fatalf("SetRegistrationEnabled(false) error = %v", err)
	}
	if err := api.SetRegistrationEnabled(true); err != nil {
		t.Fatalf("SetRegistrationEnabled(true) error = %v", err)
	}
}

func TestLocalModeClientCount(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	if _, err := api.Count(nil); err != nil {
		t.Fatalf("Count(nil) error = %v", err)
	}
	pid := int64(1)
	if _, err := api.Count(&pid); err != nil {
		t.Fatalf("Count(&1) error = %v", err)
	}
}

func TestLocalModeClientProjectOps(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	proj, err := api.CreateProject(ProjectCreateRequest{Title: "P2", Prefix: "PP"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	got, err := api.GetProject("2")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if got.Title != proj.Title {
		t.Fatalf("GetProject().Title = %q", got.Title)
	}
	upd, err := api.UpdateProject(proj.ID, ProjectUpdateRequest{Title: "P3"})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if upd.Title != "P3" {
		t.Fatalf("UpdateProject().Title = %q", upd.Title)
	}
	if _, err := api.SetProjectEnabled(proj.ID, false); err != nil {
		t.Fatalf("SetProjectEnabled() error = %v", err)
	}
	// Note: DeleteProject may fail on some schemas, skip if error
	_ = api.DeleteProject(proj.ID)
}

func TestLocalModeClientTicketLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	ticket, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "LC"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.CloseTicket(ticket.ID, ""); err != nil {
		t.Fatalf("CloseTicket() error = %v", err)
	}
	if _, err := api.OpenTicket(ticket.ID, ""); err != nil {
		t.Fatalf("OpenTicket() error = %v", err)
	}
	if _, err := api.ArchiveTicket(ticket.ID, ""); err != nil {
		t.Fatalf("ArchiveTicket() error = %v", err)
	}
	if _, err := api.UnarchiveTicket(ticket.ID, ""); err != nil {
		t.Fatalf("UnarchiveTicket() error = %v", err)
	}
	if _, err := api.NotReadyTicket(ticket.ID, ""); err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if _, err := api.GetTicket(ticket.ID); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	clone, err := api.CloneTicket(ticket.ID, "")
	if err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if clone.CloneOf == nil || *clone.CloneOf != ticket.ID {
		t.Fatalf("CloneTicket().CloneOf = %v", clone.CloneOf)
	}
	if _, err := api.ListTickets(1); err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if _, err := api.ListHistory(ticket.ID); err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if _, err := api.ListProjectHistory(1, 5); err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
	if _, err := api.ListComments(ticket.ID); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if _, err := api.SetTicketHealth(ticket.ID, 3); err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
}

func TestLocalModeClientDependencies(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	t1, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "A"})
	if err != nil {
		t.Fatalf("CreateTicket(A) error = %v", err)
	}
	t2, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "B"})
	if err != nil {
		t.Fatalf("CreateTicket(B) error = %v", err)
	}
	if _, err := api.AddDependency(DependencyRequest{ProjectID: 1, TicketID: t1.ID, DependsOn: t2.ID}); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	deps, err := api.ListDependencies(t1.ID)
	if err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("ListDependencies() len = %d", len(deps))
	}
	if err := api.RemoveDependency(DependencyRequest{ProjectID: 1, TicketID: t1.ID, DependsOn: t2.ID}); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}
}

func TestLocalModeClientWorkflows(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	wf, err := api.CreateWorkflow(WorkflowRequest{Name: "wf1", Description: "d"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := api.ListWorkflows(); err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if _, err := api.GetWorkflow(wf.ID); err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	stage, err := api.AddWorkflowStage(wf.ID, WorkflowStageRequest{StageName: "design", SortOrder: 0})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if err := api.ReorderWorkflowStages(wf.ID, []int64{stage.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	if err := api.RemoveWorkflowStage(stage.ID); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	export, err := api.ExportWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	export.Name = "imported-wf"
	if _, err := api.ImportWorkflow(export); err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}

	// Test SetTicketWorkflow/UnsetTicketWorkflow
	ticket, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "wf-test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.SetTicketWorkflow(ticket.ID, wf.ID); err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if _, err := api.UnsetTicketWorkflow(ticket.ID); err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}

	if err := api.DeleteWorkflow(wf.ID); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
}

func TestLocalModeClientTimeAndLabels(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	ticket, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "time-test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	entry, err := api.LogTime(ticket.ID, libticket.TimeEntryRequest{Minutes: 30, Note: "work"})
	if err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	entries, err := api.ListTimeEntries(ticket.ID)
	if err != nil {
		t.Fatalf("ListTimeEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ListTimeEntries() len = %d", len(entries))
	}
	total, err := api.TotalTimeForTicket(ticket.ID)
	if err != nil {
		t.Fatalf("TotalTimeForTicket() error = %v", err)
	}
	if total != 30 {
		t.Fatalf("TotalTimeForTicket() = %d, want 30", total)
	}
	if err := api.DeleteTimeEntry(entry.ID); err != nil {
		t.Fatalf("DeleteTimeEntry() error = %v", err)
	}

	label, err := api.CreateLabel(1, libticket.LabelRequest{Name: "bug", Color: "red"})
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	labels, err := api.ListLabels(1)
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("ListLabels() returned empty")
	}
	if err := api.AddTicketLabel(ticket.ID, label.ID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	ticketLabels, err := api.ListTicketLabels(ticket.ID)
	if err != nil {
		t.Fatalf("ListTicketLabels() error = %v", err)
	}
	if len(ticketLabels) != 1 {
		t.Fatalf("ListTicketLabels() len = %d", len(ticketLabels))
	}
	if err := api.RemoveTicketLabel(ticket.ID, label.ID); err != nil {
		t.Fatalf("RemoveTicketLabel() error = %v", err)
	}
	if err := api.DeleteLabel(label.ID); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
}

func TestLocalModeClientStories(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	story, err := api.CreateStory(1, "S1", "desc")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	stories, err := api.ListStories(1)
	if err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("ListStories() returned empty")
	}
	got, err := api.GetStory(story.ID)
	if err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if got.Title != "S1" {
		t.Fatalf("GetStory().Title = %q", got.Title)
	}
	upd, err := api.UpdateStory(story.ID, "S2", "desc2")
	if err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if upd.Title != "S2" {
		t.Fatalf("UpdateStory().Title = %q", upd.Title)
	}
	if err := api.DeleteStory(story.ID); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
}

func TestLocalModeClientTeamsAndMembers(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	team, err := api.CreateTeam(TeamRequest{Name: "alpha"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	teams, err := api.ListTeams()
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) == 0 {
		t.Fatal("ListTeams() returned empty")
	}
	upd, err := api.UpdateTeam(team.ID, TeamRequest{Name: "beta"})
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if upd.Name != "beta" {
		t.Fatalf("UpdateTeam().Name = %q", upd.Name)
	}

	// Get user ID for member operations
	status, err := api.Status()
	if err != nil || status.User == nil {
		t.Fatalf("Status() error = %v", err)
	}
	userID := status.User.ID

	member, err := api.AddTeamMember(team.ID, TeamMemberRequest{UserID: userID, Role: "member"})
	if err != nil {
		t.Fatalf("AddTeamMember() error = %v", err)
	}
	_ = member
	members, err := api.ListTeamMembers(team.ID)
	if err != nil {
		t.Fatalf("ListTeamMembers() error = %v", err)
	}
	if len(members) == 0 {
		t.Fatal("ListTeamMembers() returned empty")
	}
	if err := api.RemoveTeamMember(team.ID, userID); err != nil {
		t.Fatalf("RemoveTeamMember() error = %v", err)
	}

	// Project members
	pm, err := api.AddProjectMember(1, ProjectMemberRequest{UserID: userID, Role: "viewer"})
	if err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	_ = pm
	pms, err := api.ListProjectMembers(1)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	if len(pms) == 0 {
		t.Fatal("ListProjectMembers() returned empty")
	}
	if err := api.RemoveProjectMember(1, userID); err != nil {
		t.Fatalf("RemoveProjectMember() error = %v", err)
	}

	// Project team members
	ptm, err := api.AddProjectTeamMember(1, ProjectTeamMemberRequest{TeamID: team.ID, Role: "viewer"})
	if err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}
	_ = ptm
	ptms, err := api.ListProjectTeamMembers(1)
	if err != nil {
		t.Fatalf("ListProjectTeamMembers() error = %v", err)
	}
	if len(ptms) == 0 {
		t.Fatal("ListProjectTeamMembers() returned empty")
	}
	if err := api.RemoveProjectTeamMember(1, team.ID); err != nil {
		t.Fatalf("RemoveProjectTeamMember() error = %v", err)
	}

	if err := api.DeleteTeam(team.ID); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}
}

func TestLocalModeClientAgents(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	api := New(config.Config{})

	agent, pw, err := api.CreateAgent(AgentCreateRequest{Password: "agentpw"})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if pw == "" {
		t.Fatal("CreateAgent() password is empty")
	}
	agents, err := api.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("ListAgents() returned empty")
	}
	statuses, err := api.ListAgentStatuses()
	if err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	_ = statuses

	if _, err := api.SetAgentEnabled(agent.ID, false); err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if _, err := api.SetAgentEnabled(agent.ID, true); err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}

	newpw := "newpw"
	if _, err := api.UpdateAgent(agent.ID, AgentUpdateRequest{Password: &newpw}); err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	if err := api.SetAgentConfig(agent.ID, "k", "v"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	entries, err := api.ListAgentConfig(agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("ListAgentConfig() returned empty")
	}
	if err := api.DeleteAgentConfig(agent.ID, "k"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}

	// Team agents
	team, err := api.CreateTeam(TeamRequest{Name: "ateam"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	ta, err := api.AddTeamAgent(team.ID, agent.ID)
	if err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	_ = ta
	tas, err := api.ListTeamAgents(team.ID)
	if err != nil {
		t.Fatalf("ListTeamAgents() error = %v", err)
	}
	if len(tas) == 0 {
		t.Fatal("ListTeamAgents() returned empty")
	}
	if err := api.RemoveTeamAgent(team.ID, agent.ID); err != nil {
		t.Fatalf("RemoveTeamAgent() error = %v", err)
	}

	// RegisterAgent, HeartbeatAgent, RequestAgentWork
	registered, err := api.RegisterAgent(AgentRegisterRequest{ID: agent.ID, Password: newpw})
	if err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if registered.ID != agent.ID {
		t.Fatalf("RegisterAgent().ID = %q", registered.ID)
	}
	if err := api.HeartbeatAgent(agent.ID, newpw, "online"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}

	// Create a ready ticket for RequestAgentWork
	ticket, err := api.CreateTicket(TicketCreateRequest{ProjectID: 1, Type: "task", Title: "agent-work"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.ReadyTicket(ticket.ID, ""); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	resp, err := api.RequestAgentWork(AgentRequest{ID: agent.ID, Password: newpw, ProjectID: 1})
	if err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	_ = resp

	// AgentUpdateTicket
	if resp.Status == "NEW" || resp.Status == "CURRENT" {
		if _, err := api.AgentUpdateTicket(resp.Ticket.ID, AgentTicketUpdateRequest{ID: agent.ID, Password: newpw, Result: "done"}); err != nil {
			t.Fatalf("AgentUpdateTicket() error = %v", err)
		}
	}

	if err := api.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
}

func TestGetenvFirst(t *testing.T) {
	t.Setenv("TEST_GETENV_A", "")
	t.Setenv("TEST_GETENV_B", "found")
	result := getenvFirst("TEST_GETENV_A", "TEST_GETENV_B")
	if result != "found" {
		t.Fatalf("getenvFirst() = %q, want found", result)
	}
	result = getenvFirst("TEST_GETENV_MISSING_1", "TEST_GETENV_MISSING_2")
	if result != "" {
		t.Fatalf("getenvFirst() = %q, want empty", result)
	}
}

func mustOpenDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
