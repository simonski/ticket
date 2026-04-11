package libticket_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func TestLocalServiceContract(t *testing.T) {

	RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		tempDir := t.TempDir()
		t.Setenv("TICKET_HOME", tempDir)
		dbPath := filepath.Join(tempDir, "ticket.db")
		if err := store.Init(dbPath, "admin", "secret12"); err != nil {
			t.Fatalf("store.Init() error = %v", err)
		}
		return libticket.NewLocal(config.Config{})
	}, ContractOptions{RequireStatusOwnership: false})
}

func TestLocalServiceStatusDefaultsToAdmin(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	status, err := svc.Status()
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

	if _, err := svc.Register("alice", "secret12"); err == nil {
		t.Fatal("Register() error = nil, want remote-mode error")
	}
	if _, _, err := svc.Login("alice", "secret12"); err == nil {
		t.Fatal("Login() error = nil, want remote-mode error")
	}
	if err := svc.Logout(); err == nil {
		t.Fatal("Logout() error = nil, want remote-mode error")
	}
}

func TestLocalServiceStatusFailsWhenDatabaseMissing(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	svc := libticket.NewLocal(config.Config{})
	if _, err := svc.Status(); err == nil {
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
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	projects, err := svc.ListProjects()
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
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	parent, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	child, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Child"})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	updated, err := svc.SetTicketParent(child.ID, parent.ID, "")
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", updated)
	}

	detached, err := svc.UnsetTicketParent(child.ID, "")
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
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	parent, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
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
	// Ticket starts at develop/idle (2-stage SDLC), request it directly
	requested, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: 1, TicketID: &ticket.ID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}

	updated, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
		Title:              "Updated Child",
		Description:        "new description",
		AcceptanceCriteria: "new ac",
		ParentID:           &parent.ID,
		Assignee:           requested.Ticket.Assignee,
		Status:             "develop/active",
		Priority:           3,
		Order:              7,
		EstimateEffort:     5,
		EstimateComplete:   "2026-04-15T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Title != "Updated Child" || updated.Description != "new description" || updated.AcceptanceCriteria != "new ac" || updated.Status != "develop/active" || updated.Priority != 3 || updated.Order != 7 || updated.EstimateEffort != 5 || updated.EstimateComplete != "2026-04-15T12:00:00Z" {
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
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Advance through all stages: develop -> done (2-stage SDLC)
	// Each state=success on a non-final stage auto-advances to next stage with state=idle
	for _, wantStatus := range []string{"done/idle", "done/success"} {
		ticket, err = svc.GetTicketByID(ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		updated, err := svc.UpdateTicket(ticket.ID, libticket.TicketUpdateRequest{
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
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := svc.DeleteTicket(ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := svc.GetTicketByID(ticket.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func newLocalSvc(t *testing.T) libticket.Service {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret12"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	return libticket.NewLocal(config.Config{})
}
func TestLocalServiceDeleteProject(t *testing.T) {
	svc := newLocalSvc(t)
	projects, err := svc.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("no projects to delete")
	}
	if err := svc.DeleteProject(projects[0].ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
}

func TestLocalServiceResetUserPassword(t *testing.T) {
	svc := newLocalSvc(t)
	user, err := svc.ResetUserPassword("admin", "newsecret")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("ResetUserPassword().Username = %q, want admin", user.Username)
	}
}


func TestLocalServiceNotReadyTicket(t *testing.T) {
	svc := newLocalSvc(t)
	ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Ready test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	_, err = svc.ReadyTicket(ticket.ID, "")
	if err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	updated, err := svc.NotReadyTicket(ticket.ID, "")
	if err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if !updated.Draft {
		t.Fatal("NotReadyTicket() did not set draft flag")
	}
}

func TestLocalServiceSetUnsetTicketSdlc(t *testing.T) {
	svc := newLocalSvc(t)
	wf, err := svc.CreateSdlc(libticket.SdlcRequest{Name: "Test WF"})
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	ticket, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "WF test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	updated, err := svc.SetTicketSdlc(ticket.ID, wf.ID)
	if err != nil {
		t.Fatalf("SetTicketSdlc() error = %v", err)
	}
	if updated.SdlcID == nil || *updated.SdlcID != wf.ID {
		t.Fatalf("SetTicketSdlc() sdlc_id = %v, want %d", updated.SdlcID, wf.ID)
	}
	unset, err := svc.UnsetTicketSdlc(ticket.ID)
	if err != nil {
		t.Fatalf("UnsetTicketSdlc() error = %v", err)
	}
	if unset.SdlcID != nil {
		t.Fatalf("UnsetTicketSdlc() sdlc_id = %v, want nil", unset.SdlcID)
	}
}

func TestLocalServiceListAgentStatuses(t *testing.T) {
	svc := newLocalSvc(t)
	statuses, err := svc.ListAgentStatuses()
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
	agent, _, err := svc.CreateAgent(libticket.AgentCreateRequest{})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if err := svc.SetAgentConfig(agent.ID, "poll_interval", "5"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	entries, err := svc.ListAgentConfig(agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "poll_interval" || entries[0].Value != "5" {
		t.Fatalf("ListAgentConfig() = %v, want [{poll_interval 5}]", entries)
	}
	if err := svc.DeleteAgentConfig(agent.ID, "poll_interval"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	entries, err = svc.ListAgentConfig(agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() after delete error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ListAgentConfig() after delete = %v, want empty", entries)
	}
}

func TestLocalServiceStoryCRUD(t *testing.T) {
	svc := newLocalSvc(t)
	story, err := svc.CreateStory(1, "Test Story", "Story description")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if story.Title != "Test Story" {
		t.Fatalf("CreateStory().Title = %q, want %q", story.Title, "Test Story")
	}

	stories, err := svc.ListStories(1)
	if err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("ListStories() len = %d, want 1", len(stories))
	}

	got, err := svc.GetStory(story.ID)
	if err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if got.Title != "Test Story" {
		t.Fatalf("GetStory().Title = %q, want %q", got.Title, "Test Story")
	}

	updated, err := svc.UpdateStory(story.ID, "Updated Story", "New desc")
	if err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if updated.Title != "Updated Story" {
		t.Fatalf("UpdateStory().Title = %q, want %q", updated.Title, "Updated Story")
	}

	if err := svc.DeleteStory(story.ID); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
	stories, err = svc.ListStories(1)
	if err != nil {
		t.Fatalf("ListStories() after delete error = %v", err)
	}
	if len(stories) != 0 {
		t.Fatalf("ListStories() after delete len = %d, want 0", len(stories))
	}
}

func TestLocalServiceListProjectHistory(t *testing.T) {
	svc := newLocalSvc(t)
	_, err := svc.ListProjectHistory(1, 100)
	if err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
}
