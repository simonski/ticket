package libticket_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func localServiceConfig(dbPath string) config.Config {
	return config.Config{Location: dbPath}
}

func TestLocalServiceContract(t *testing.T) {

	RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		tempDir := t.TempDir()
		t.Setenv("TICKET_HOME", tempDir)
		dbPath := filepath.Join(tempDir, "ticket.db")
		if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
			t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
		}
		return libticket.NewLocal(localServiceConfig(dbPath))
	}, ContractOptions{RequireStatusOwnership: false})
}

func TestLocalServiceStatusDefaultsToAdmin(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil {
		t.Fatalf("Status() = %#v, want authenticated admin", status)
	}
	if status.User.Username != "admin" {
		t.Fatalf("Status().User.Username = %q, want admin", status.User.Username)
	}
}

func TestLocalServiceRemoteAuthCommandsFail(t *testing.T) {
	t.Parallel()
	svc := libticket.NewLocal(config.Config{})

	if _, err := svc.Register(context.Background(), "alice", "secret12"); err == nil {
		t.Fatal("Register() error = nil, want remote-mode error")
	}
	if _, _, err := svc.Login(context.Background(), "alice", "secret12"); err == nil {
		t.Fatal("Login() error = nil, want remote-mode error")
	}
	if err := svc.Logout(context.Background()); err == nil {
		t.Fatal("Logout() error = nil, want remote-mode error")
	}
}

func TestLocalServiceStatusFailsWhenDatabaseMissing(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	svc := libticket.NewLocal(localServiceConfig(filepath.Join(tempDir, "ticket.db")))
	if _, err := svc.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want missing database error")
	}
}

func TestLocalUsernameUsesEnvironmentFallbacks(t *testing.T) {

	t.Setenv("USER", "env-user")
	t.Setenv("USERNAME", "env-username")

	got := libticket.LocalUsername()
	if got != "admin" {
		t.Fatalf("LocalUsername() = %q, want admin", got)
	}
}

func TestLocalServiceUsesTicketHomeDatabasePath(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("ListProjects() returned no projects")
	}
}

func TestLocalServiceSetTicketParent(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	child, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Child"})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	updated, err := svc.SetTicketParent(context.Background(), child.ID, parent.ID, "")
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", updated)
	}

	detached, err := svc.UnsetTicketParent(context.Background(), child.ID, "")
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}
}

func TestLocalServiceUpdateTicketSupportsExpandedFields(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID:          1,
		Type:               "task",
		Title:              "Child",
		Description:        "old description",
		AcceptanceCriteria: "old ac",
		Priority:           1,
		EstimateEffort:     2,
		EstimateComplete:   "2026-04-01T09:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	// Assign the ticket directly and set it active.
	updated, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{
		Title:              "Updated Child",
		Description:        "new description",
		AcceptanceCriteria: "new ac",
		ParentID:           &parent.ID,
		Assignee:           "admin",
		Status:             "design/active",
		Priority:           3,
		Order:              7,
		EstimateEffort:     5,
		EstimateComplete:   "2026-04-15T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Title != "Updated Child" || updated.Description != "new description" || updated.AcceptanceCriteria != "new ac" || updated.Status != "design/active" || updated.Priority != 3 || updated.Order != 7 || updated.EstimateEffort != 5 || updated.EstimateComplete != "2026-04-15T12:00:00Z" {
		t.Fatalf("UpdateTicket() = %#v", updated)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("UpdateTicket() parent = %#v", updated)
	}
}

func TestLocalServiceIgnoresOwnershipForStatusChanges(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Advance through all stages: design -> develop -> test -> done (4-stage SDLC)
	// Each state=success on a non-final stage auto-advances to next stage with state=idle
	for _, wantStatus := range []string{"develop/idle", "test/idle", "done/idle", "done/success"} {
		ticket, err = svc.GetTicketByID(context.Background(), ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		updated, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{
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

func TestLocalServiceDeleteTicket(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := svc.DeleteTicket(context.Background(), ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := svc.GetTicketByID(context.Background(), ticket.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func newLocalSvc(t *testing.T) libticket.Service {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12", static.SeedDatabase); err != nil {
		t.Fatalf("store.Init(, static.SeedDatabase) error = %v", err)
	}
	return libticket.NewLocal(localServiceConfig(dbPath))
}
func TestLocalServiceDeleteProject(t *testing.T) {
	svc := newLocalSvc(t)
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("no projects to delete")
	}
	if err := svc.DeleteProject(context.Background(), projects[0].ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
}

func TestLocalServiceResetUserPassword(t *testing.T) {
	svc := newLocalSvc(t)
	user, err := svc.ResetUserPassword(context.Background(), "admin", "newsecret")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("ResetUserPassword().Username = %q, want admin", user.Username)
	}
}

func TestLocalServiceNotReadyTicket(t *testing.T) {
	svc := newLocalSvc(t)
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Ready test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	_, err = svc.ReadyTicket(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	updated, err := svc.NotReadyTicket(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if !updated.Draft {
		t.Fatal("NotReadyTicket() did not set draft flag")
	}
}

func TestLocalServiceSetUnsetTicketSdlc(t *testing.T) {
	svc := newLocalSvc(t)
	wf, err := svc.CreateSdlc(context.Background(), libticket.SdlcRequest{Name: "Test WF"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "WF test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	updated, err := svc.SetTicketSdlc(context.Background(), ticket.ID, wf.ID)
	if err != nil {
		t.Fatalf("SetTicketSdlc() error = %v", err)
	}
	if updated.SdlcID == nil || *updated.SdlcID != wf.ID {
		t.Fatalf("SetTicketSdlc() sdlc_id = %v, want %d", updated.SdlcID, wf.ID)
	}
	unset, err := svc.UnsetTicketSdlc(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("UnsetTicketSdlc() error = %v", err)
	}
	if unset.SdlcID != nil {
		t.Fatalf("UnsetTicketSdlc() sdlc_id = %v, want nil", unset.SdlcID)
	}
}

func TestLocalServiceListAgentStatuses(t *testing.T) {
	svc := newLocalSvc(t)
	statuses, err := svc.ListAgentStatuses(context.Background())
	if err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	// No agents yet, should return empty.
	if statuses == nil {
		t.Fatal("ListAgentStatuses() returned nil, want empty slice")
	}
}

func TestLocalServiceAgentConfig(t *testing.T) {
	svc := newLocalSvc(t)
	agent, _, err := svc.CreateAgent(context.Background(), libticket.AgentCreateRequest{})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if err := svc.SetAgentConfig(context.Background(), agent.ID, "poll_interval", "5"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	entries, err := svc.ListAgentConfig(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "poll_interval" || entries[0].Value != "5" {
		t.Fatalf("ListAgentConfig() = %v, want [{poll_interval 5}]", entries)
	}
	if err := svc.DeleteAgentConfig(context.Background(), agent.ID, "poll_interval"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	entries, err = svc.ListAgentConfig(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() after delete error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ListAgentConfig() after delete = %v, want empty", entries)
	}
}

func TestLocalServiceStoryCRUD(t *testing.T) {
	svc := newLocalSvc(t)
	story, err := svc.CreateStory(context.Background(), 1, "Test Story", "Story description")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if story.Title != "Test Story" {
		t.Fatalf("CreateStory().Title = %q, want %q", story.Title, "Test Story")
	}

	stories, err := svc.ListStories(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("ListStories() len = %d, want 1", len(stories))
	}

	got, err := svc.GetStory(context.Background(), story.ID)
	if err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if got.Title != "Test Story" {
		t.Fatalf("GetStory().Title = %q, want %q", got.Title, "Test Story")
	}

	updated, err := svc.UpdateStory(context.Background(), story.ID, "Updated Story", "New desc")
	if err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if updated.Title != "Updated Story" {
		t.Fatalf("UpdateStory().Title = %q, want %q", updated.Title, "Updated Story")
	}

	if err := svc.DeleteStory(context.Background(), story.ID); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
	stories, err = svc.ListStories(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListStories() after delete error = %v", err)
	}
	if len(stories) != 0 {
		t.Fatalf("ListStories() after delete len = %d, want 0", len(stories))
	}
}

func TestLocalServiceListProjectHistory(t *testing.T) {
	svc := newLocalSvc(t)
	_, err := svc.ListProjectHistory(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
}
