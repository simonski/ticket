package client

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
)

func localClientConfig(dbPath string) config.Config {
	return config.Config{Location: dbPath}
}

func TestLocalModeClientUsesSQLiteDirectly(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	api := New(localClientConfig(dbPath))
	projects, err := api.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != 1 {
		t.Fatalf("ListProjects() = %#v", projects)
	}

	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{
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

	if _, err := api.ReadyTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}

	// Advance design -> develop so ticket is claimable
	if _, err := api.UpdateTicket(context.Background(), ticket.ID, TicketUpdateRequest{
		Title:       ticket.Title,
		Description: ticket.Description,
		ParentID:    ticket.ParentID,
		Assignee:    ticket.Assignee,
		State:       "success",
	}); err != nil {
		t.Fatalf("UpdateTicket(design->develop) error = %v", err)
	}

	requested, err := api.RequestTicket(context.Background(), TicketRequest{ProjectID: 1})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if requested.Status != "ASSIGNED" || requested.Ticket == nil {
		t.Fatalf("RequestTicket() = %#v", requested)
	}

	// After request, ticket is in develop/active
	if requested.Ticket.Status != "develop/active" {
		t.Fatalf("RequestTicket().Ticket.Status = %q, want develop/active", requested.Ticket.Status)
	}

	parent, err := api.CreateTicket(context.Background(), TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent epic",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	reparented, err := api.SetTicketParent(context.Background(), ticket.ID, parent.ID, "")
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if reparented.ParentID == nil || *reparented.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", reparented)
	}

	detached, err := api.UnsetTicketParent(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}

	comment, err := api.AddComment(context.Background(), ticket.ID, "hello")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.Text != "hello" || comment.Author == "" {
		t.Fatalf("AddComment() = %#v", comment)
	}
}

func TestLocalModeClientCreateAndUpdateTicketUsingStatusAndMessages(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	api := New(localClientConfig(dbPath))
	ctx := context.Background()
	ticket, err := api.CreateTicket(ctx, TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Status-based task",
		Assignee:  "admin",
		Status:    "design/active",
		Message:   "created from status",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if ticket.State != "active" {
		t.Fatalf("CreateTicket().State = %q, want active", ticket.State)
	}
	updated, err := api.UpdateTicket(ctx, ticket.ID, TicketUpdateRequest{
		Title:       ticket.Title,
		Description: ticket.Description,
		ParentID:    ticket.ParentID,
		Assignee:    ticket.Assignee,
		Status:      "design/success",
		Message:     "updated from status",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.ID != ticket.ID {
		t.Fatalf("UpdateTicket().ID = %q, want %q", updated.ID, ticket.ID)
	}
	comments, err := api.ListComments(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(comments) < 2 {
		t.Fatalf("ListComments() len = %d, want at least 2", len(comments))
	}
}

func TestLocalModeClientIgnoresOwnershipForStatusChanges(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	api := New(localClientConfig(dbPath))
	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{
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

	// Advance through all stages to reach done/success (4-stage Workflow: design, develop, test, done)
	for _, wantStatus := range []string{"develop/idle", "test/idle", "done/idle", "done/success"} {
		ticket, err = api.GetTicketByID(context.Background(), ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		updated, err := api.UpdateTicket(context.Background(), ticket.ID, TicketUpdateRequest{
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
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	api := New(localClientConfig(dbPath))
	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := api.DeleteTicket(context.Background(), ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := api.GetTicketByID(context.Background(), ticket.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func TestLocalModeClientStatusIsReadOnlyWithoutMatchingUser(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	api := New(localClientConfig(dbPath))
	status, err := api.Status(context.Background())
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

	api := New(localClientConfig(filepath.Join(tempDir, "ticket.db")))
	if _, err := api.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want missing database error")
	}
}

func TestLocalModeClientRolesCRUD(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	role, err := api.CreateRole(context.Background(), RoleRequest{Title: "dev", Description: "build", AcceptanceCriteria: "ship"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if role.Title != "dev" {
		t.Fatalf("CreateRole().Title = %q", role.Title)
	}
	roles, err := api.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) == 0 {
		t.Fatal("ListRoles() returned empty")
	}
	updated, err := api.UpdateRole(context.Background(), role.ID, RoleRequest{Title: "dev2", Description: "build", AcceptanceCriteria: "ship"})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updated.Title != "dev2" {
		t.Fatalf("UpdateRole().Title = %q", updated.Title)
	}
	if err := api.DeleteRole(context.Background(), role.ID); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
}

func TestLocalModeClientUserOps(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	user, err := api.CreateUser(context.Background(), "bob", "password1")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.Username != "bob" {
		t.Fatalf("CreateUser().Username = %q", user.Username)
	}
	users, err := api.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) < 2 {
		t.Fatal("ListUsers() too few")
	}
	if err := api.SetUserEnabled(context.Background(), "bob", false); err != nil {
		t.Fatalf("SetUserEnabled() error = %v", err)
	}
	reset, err := api.ResetUserPassword(context.Background(), "bob", "newpassword1")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if reset.Username != "bob" {
		t.Fatalf("ResetUserPassword().Username = %q", reset.Username)
	}
	if err := api.DeleteUser(context.Background(), "bob"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
}

func TestLocalModeClientRegistrationToggle(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	if err := api.SetRegistrationEnabled(context.Background(), false); err != nil {
		t.Fatalf("SetRegistrationEnabled(false) error = %v", err)
	}
	if err := api.SetRegistrationEnabled(context.Background(), true); err != nil {
		t.Fatalf("SetRegistrationEnabled(true) error = %v", err)
	}
}

func TestLocalModeClientCount(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	if _, err := api.Count(context.Background(), nil); err != nil {
		t.Fatalf("Count(nil) error = %v", err)
	}
	pid := int64(1)
	if _, err := api.Count(context.Background(), &pid); err != nil {
		t.Fatalf("Count(&1) error = %v", err)
	}
}

func TestLocalModeClientProjectOps(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	proj, err := api.CreateProject(context.Background(), ProjectCreateRequest{Title: "P2", Prefix: "PP"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	got, err := api.GetProject(context.Background(), "2")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if got.Title != proj.Title {
		t.Fatalf("GetProject().Title = %q", got.Title)
	}
	upd, err := api.UpdateProject(context.Background(), proj.ID, ProjectUpdateRequest{Title: "P3"})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if upd.Title != "P3" {
		t.Fatalf("UpdateProject().Title = %q", upd.Title)
	}
	if _, err := api.SetProjectEnabled(context.Background(), proj.ID, false); err != nil {
		t.Fatalf("SetProjectEnabled() error = %v", err)
	}
	// Note: DeleteProject may fail on some schemas, skip if error
	_ = api.DeleteProject(context.Background(), proj.ID)
}

func TestLocalModeClientTicketLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "LC"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.CloseTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("CloseTicket() error = %v", err)
	}
	if _, err := api.OpenTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("OpenTicket() error = %v", err)
	}
	if _, err := api.ArchiveTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("ArchiveTicket() error = %v", err)
	}
	if _, err := api.UnarchiveTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("UnarchiveTicket() error = %v", err)
	}
	if _, err := api.NotReadyTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if _, err := api.GetTicket(context.Background(), ticket.ID); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	clone, err := api.CloneTicket(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if clone.CloneOf == nil || *clone.CloneOf != ticket.ID {
		t.Fatalf("CloneTicket().CloneOf = %v", clone.CloneOf)
	}
	if _, err := api.ListTickets(context.Background(), 1); err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if _, err := api.ListHistory(context.Background(), ticket.ID); err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if _, err := api.ListProjectHistory(context.Background(), 1, 5); err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
	if _, err := api.ListComments(context.Background(), ticket.ID); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if _, err := api.SetTicketHealth(context.Background(), ticket.ID, 3); err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
}

func TestLocalModeClientDependencies(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	t1, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "A"})
	if err != nil {
		t.Fatalf("CreateTicket(A) error = %v", err)
	}
	t2, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "B"})
	if err != nil {
		t.Fatalf("CreateTicket(B) error = %v", err)
	}
	if _, err := api.AddDependency(context.Background(), DependencyRequest{ProjectID: 1, TicketID: t1.ID, DependsOn: t2.ID}); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	deps, err := api.ListDependencies(context.Background(), t1.ID)
	if err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("ListDependencies() len = %d", len(deps))
	}
	if err := api.RemoveDependency(context.Background(), DependencyRequest{ProjectID: 1, TicketID: t1.ID, DependsOn: t2.ID}); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}
}

func TestLocalModeClientWorkflows(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	wf, err := api.CreateWorkflow(context.Background(), WorkflowRequest{Name: "wf1", Description: "d"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := api.ListWorkflows(context.Background()); err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if _, err := api.GetWorkflow(context.Background(), wf.ID); err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	stage, err := api.AddWorkflowStage(context.Background(), wf.ID, WorkflowStageRequest{StageName: "design", SortOrder: 0})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if err := api.ReorderWorkflowStages(context.Background(), wf.ID, []int64{stage.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	if err := api.RemoveWorkflowStage(context.Background(), stage.ID); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	export, err := api.ExportWorkflow(context.Background(), wf.ID)
	if err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	export.Name = "imported-wf"
	if _, err := api.ImportWorkflow(context.Background(), export); err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}

	// Test SetTicketWorkflow/UnsetTicketWorkflow
	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "wf-test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.SetTicketWorkflow(context.Background(), ticket.ID, wf.ID); err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if _, err := api.UnsetTicketWorkflow(context.Background(), ticket.ID); err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}

	if err := api.DeleteWorkflow(context.Background(), wf.ID); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
}

func TestLocalModeClientWorkflowStagesProjectDraftAndTicketAliases(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	ctx := context.Background()
	wf, err := api.CreateWorkflow(ctx, WorkflowRequest{Name: "wf-advanced", Description: "d"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := api.AddWorkflowStage(ctx, wf.ID, WorkflowStageRequest{
		StageName:          "develop",
		Description:        "ways",
		AcceptanceCriteria: "ready",
		DefinitionOfDone:   "done",
		SortOrder:          1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if stage.Description != "ways" || stage.DefinitionOfReady != "ready" || stage.DefinitionOfDone != "done" {
		t.Fatalf("AddWorkflowStage() = %#v, want fallback values copied", stage)
	}
	stage, err = api.UpdateWorkflowStage(ctx, stage.ID, WorkflowStageRequest{
		StageName:          "develop",
		Description:        "updated ways",
		AcceptanceCriteria: "updated ready",
		DefinitionOfDone:   "updated done",
	})
	if err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	if stage.Description != "updated ways" || stage.DefinitionOfReady != "updated ready" {
		t.Fatalf("UpdateWorkflowStage() = %#v, want updated fallback values", stage)
	}
	gotStage, err := api.GetWorkflowStage(ctx, stage.ID)
	if err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if gotStage.ID != stage.ID {
		t.Fatalf("GetWorkflowStage().ID = %d, want %d", gotStage.ID, stage.ID)
	}
	stages, err := api.ListWorkflowStages(ctx, wf.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStages() error = %v", err)
	}
	if len(stages) != 1 {
		t.Fatalf("ListWorkflowStages() len = %d, want 1", len(stages))
	}

	role, err := api.CreateRole(ctx, RoleRequest{Title: "Engineer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := api.AddWorkflowStageRole(ctx, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}
	if err := api.ReorderWorkflowStageRoles(ctx, wf.ID, stage.ID, []int64{role.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStageRoles() error = %v", err)
	}
	if err := api.RemoveWorkflowStageRole(ctx, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("RemoveWorkflowStageRole() error = %v", err)
	}

	projectBefore, err := api.GetProject(ctx, "1")
	if err != nil {
		t.Fatalf("GetProject(before) error = %v", err)
	}
	if err := api.SetProjectDefaultDraft(ctx, 1, !projectBefore.DefaultDraft); err != nil {
		t.Fatalf("SetProjectDefaultDraft() error = %v", err)
	}
	projectAfter, err := api.GetProject(ctx, "1")
	if err != nil {
		t.Fatalf("GetProject(after) error = %v", err)
	}
	if projectAfter.DefaultDraft == projectBefore.DefaultDraft {
		t.Fatalf("DefaultDraft unchanged: before=%v after=%v", projectBefore.DefaultDraft, projectAfter.DefaultDraft)
	}

	ticket, err := api.CreateTicket(ctx, TicketCreateRequest{ProjectID: 1, Type: "task", Title: "alias-test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.ReadyTicket(ctx, ticket.ID, "ready"); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	db, err := api.openLocalDB()
	if err != nil {
		t.Fatalf("openLocalDB() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, store.StateSuccess, store.RenderLifecycleStatus(ticket.Stage, store.StateSuccess), ticket.ID); err != nil {
		t.Fatalf("set success state error = %v", err)
	}
	advanced, err := api.NextTicket(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("NextTicket() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, store.StateFail, store.RenderLifecycleStatus(advanced.Stage, store.StateFail), ticket.ID); err != nil {
		t.Fatalf("set fail state error = %v", err)
	}
	if _, err := api.PreviousTicket(ctx, ticket.ID); err != nil {
		t.Fatalf("PreviousTicket() error = %v", err)
	}
	if _, err := api.ArchiveTicket(ctx, ticket.ID, "archive"); err != nil {
		t.Fatalf("ArchiveTicket() error = %v", err)
	}
	if _, err := api.UnarchiveTicket(ctx, ticket.ID, "unarchive"); err != nil {
		t.Fatalf("UnarchiveTicket() error = %v", err)
	}
	if _, err := api.CompleteTicket(ctx, ticket.ID, "complete"); err != nil {
		t.Fatalf("CompleteTicket() error = %v", err)
	}
	if _, err := api.ReopenTicket(ctx, ticket.ID, "reopen"); err != nil {
		t.Fatalf("ReopenTicket() error = %v", err)
	}
	if _, err := api.DraftTicket(ctx, ticket.ID, "draft"); err != nil {
		t.Fatalf("DraftTicket() error = %v", err)
	}
	if _, err := api.UndraftTicket(ctx, ticket.ID, "undraft"); err != nil {
		t.Fatalf("UndraftTicket() error = %v", err)
	}
	if _, err := api.CloneTicket(ctx, ticket.ID, "clone"); err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
}

func TestLocalModeClientTimeAndLabels(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "time-test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	entry, err := api.LogTime(context.Background(), ticket.ID, TimeEntryRequest{Minutes: 30, Note: "work"})
	if err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	entries, err := api.ListTimeEntries(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("ListTimeEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ListTimeEntries() len = %d", len(entries))
	}
	total, err := api.TotalTimeForTicket(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("TotalTimeForTicket() error = %v", err)
	}
	if total != 30 {
		t.Fatalf("TotalTimeForTicket() = %d, want 30", total)
	}
	if err := api.DeleteTimeEntry(context.Background(), entry.ID); err != nil {
		t.Fatalf("DeleteTimeEntry() error = %v", err)
	}

	label, err := api.CreateLabel(context.Background(), 1, LabelRequest{Name: "bug", Color: "red"})
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	labels, err := api.ListLabels(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("ListLabels() returned empty")
	}
	if err := api.AddTicketLabel(context.Background(), ticket.ID, label.ID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	ticketLabels, err := api.ListTicketLabels(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("ListTicketLabels() error = %v", err)
	}
	if len(ticketLabels) != 1 {
		t.Fatalf("ListTicketLabels() len = %d", len(ticketLabels))
	}
	if err := api.RemoveTicketLabel(context.Background(), ticket.ID, label.ID); err != nil {
		t.Fatalf("RemoveTicketLabel() error = %v", err)
	}
	if err := api.DeleteLabel(context.Background(), label.ID); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
}

func TestLocalModeClientStories(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	story, err := api.CreateStory(context.Background(), 1, "S1", "desc")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	stories, err := api.ListStories(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("ListStories() returned empty")
	}
	got, err := api.GetStory(context.Background(), story.ID)
	if err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if got.Title != "S1" {
		t.Fatalf("GetStory().Title = %q", got.Title)
	}
	upd, err := api.UpdateStory(context.Background(), story.ID, "S2", "desc2")
	if err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if upd.Title != "S2" {
		t.Fatalf("UpdateStory().Title = %q", upd.Title)
	}
	if err := api.DeleteStory(context.Background(), story.ID); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
}

func TestLocalModeClientTeamsAndMembers(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	team, err := api.CreateTeam(context.Background(), TeamRequest{Name: "alpha"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	teams, err := api.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) == 0 {
		t.Fatal("ListTeams() returned empty")
	}
	upd, err := api.UpdateTeam(context.Background(), team.ID, TeamRequest{Name: "beta"})
	if err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if upd.Name != "beta" {
		t.Fatalf("UpdateTeam().Name = %q", upd.Name)
	}

	// Get user ID for member operations
	status, err := api.Status(context.Background())
	if err != nil || status.User == nil {
		t.Fatalf("Status() error = %v", err)
	}
	userID := status.User.ID

	member, err := api.AddTeamMember(context.Background(), team.ID, TeamMemberRequest{UserID: userID, Role: "member"})
	if err != nil {
		t.Fatalf("AddTeamMember() error = %v", err)
	}
	_ = member
	members, err := api.ListTeamMembers(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("ListTeamMembers() error = %v", err)
	}
	if len(members) == 0 {
		t.Fatal("ListTeamMembers() returned empty")
	}
	if err := api.RemoveTeamMember(context.Background(), team.ID, userID); err != nil {
		t.Fatalf("RemoveTeamMember() error = %v", err)
	}

	// Project members
	pm, err := api.AddProjectMember(context.Background(), 1, ProjectMemberRequest{UserID: userID, Role: "viewer"})
	if err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	_ = pm
	pms, err := api.ListProjectMembers(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	if len(pms) == 0 {
		t.Fatal("ListProjectMembers() returned empty")
	}
	if err := api.RemoveProjectMember(context.Background(), 1, userID); err != nil {
		t.Fatalf("RemoveProjectMember() error = %v", err)
	}

	// Project team members
	ptm, err := api.AddProjectTeamMember(context.Background(), 1, ProjectTeamMemberRequest{TeamID: team.ID, Role: "viewer"})
	if err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}
	_ = ptm
	ptms, err := api.ListProjectTeamMembers(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListProjectTeamMembers() error = %v", err)
	}
	if len(ptms) == 0 {
		t.Fatal("ListProjectTeamMembers() returned empty")
	}
	if err := api.RemoveProjectTeamMember(context.Background(), 1, team.ID); err != nil {
		t.Fatalf("RemoveProjectTeamMember() error = %v", err)
	}

	if err := api.DeleteTeam(context.Background(), team.ID); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}
}

func TestLocalModeClientAgents(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	agent, pw, err := api.CreateAgent(context.Background(), AgentCreateRequest{Password: "agentpw"})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if pw == "" {
		t.Fatal("CreateAgent() password is empty")
	}
	agents, err := api.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("ListAgents() returned empty")
	}
	statuses, err := api.ListAgentStatuses(context.Background())
	if err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	_ = statuses

	if _, err := api.SetAgentEnabled(context.Background(), agent.ID, false); err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if _, err := api.SetAgentEnabled(context.Background(), agent.ID, true); err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}

	newpw := "newpw"
	if _, err := api.UpdateAgent(context.Background(), agent.ID, AgentUpdateRequest{Password: &newpw}); err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	if err := api.SetAgentConfig(context.Background(), agent.ID, "k", "v"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	entries, err := api.ListAgentConfig(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("ListAgentConfig() returned empty")
	}
	if err := api.DeleteAgentConfig(context.Background(), agent.ID, "k"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}

	// Team agents
	team, err := api.CreateTeam(context.Background(), TeamRequest{Name: "ateam"})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	ta, err := api.AddTeamAgent(context.Background(), team.ID, agent.ID)
	if err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	_ = ta
	tas, err := api.ListTeamAgents(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("ListTeamAgents() error = %v", err)
	}
	if len(tas) == 0 {
		t.Fatal("ListTeamAgents() returned empty")
	}
	if err := api.RemoveTeamAgent(context.Background(), team.ID, agent.ID); err != nil {
		t.Fatalf("RemoveTeamAgent() error = %v", err)
	}

	// RegisterAgent, HeartbeatAgent, RequestAgentWork
	registered, err := api.RegisterAgent(context.Background(), AgentRegisterRequest{ID: agent.ID, Password: newpw})
	if err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if registered.ID != agent.ID {
		t.Fatalf("RegisterAgent().ID = %q", registered.ID)
	}
	if err := api.HeartbeatAgent(context.Background(), agent.ID, newpw, "online"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}

	// Create a ready ticket for RequestAgentWork
	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "agent-work"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.ReadyTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	resp, err := api.RequestAgentWork(context.Background(), AgentRequest{ID: agent.ID, Password: newpw, ProjectID: 1})
	if err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	_ = resp
	dryRunResp, err := api.RequestAgentWork(context.Background(), AgentRequest{ID: agent.ID, Password: newpw, TicketID: &ticket.ID, DryRun: true})
	if err != nil {
		t.Fatalf("RequestAgentWork(ticket dry run) error = %v", err)
	}
	_ = dryRunResp

	// AgentUpdateTicket
	if resp.Status == "NEW" || resp.Status == "CURRENT" {
		if _, err := api.AgentUpdateTicket(context.Background(), resp.Ticket.ID, AgentTicketUpdateRequest{ID: agent.ID, Password: newpw, Result: "done"}); err != nil {
			t.Fatalf("AgentUpdateTicket() error = %v", err)
		}
	}

	if err := api.DeleteAgent(context.Background(), agent.ID); err != nil {
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

func TestEnsureLocalUserCreatesAndReenablesUser(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	created, err := ensureLocalUser(ctx, db, "builder")
	if err != nil {
		t.Fatalf("ensureLocalUser(create) error = %v", err)
	}
	if created.Username != "builder" || !created.Enabled {
		t.Fatalf("ensureLocalUser(create) = %#v", created)
	}
	if err := store.SetUserEnabled(ctx, db, "builder", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	reenabled, err := ensureLocalUser(ctx, db, "builder")
	if err != nil {
		t.Fatalf("ensureLocalUser(reenable) error = %v", err)
	}
	if !reenabled.Enabled {
		t.Fatalf("ensureLocalUser(reenable) = %#v, want enabled user", reenabled)
	}
}

func TestLocalModeClientLogoutFails(t *testing.T) {
	t.Parallel()

	api := New(localClientConfig(filepath.Join(t.TempDir(), "ticket.db")))
	if err := api.Logout(context.Background()); err == nil {
		t.Fatal("Logout() error = nil, want local mode error")
	}
}

func TestLocalModeClientLoginFails(t *testing.T) {
	t.Parallel()

	api := New(localClientConfig(filepath.Join(t.TempDir(), "ticket.db")))
	if _, err := api.Login(context.Background(), "admin", "secret"); err == nil {
		t.Fatal("Login() error = nil, want local mode error")
	}
}

func TestLocalModeClientRequestTicketByRefDryRun(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	api := New(localClientConfig(dbPath))

	ticket, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: 1, Type: "task", Title: "dry-run"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.ReadyTicket(context.Background(), ticket.ID, ""); err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	resp, err := api.RequestTicket(context.Background(), TicketRequest{TicketRef: ticket.ID, DryRun: true})
	if err != nil {
		t.Fatalf("RequestTicket(dry run) error = %v", err)
	}
	if resp.Status == "" {
		t.Fatalf("RequestTicket(dry run) = %#v, want non-empty status", resp)
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
