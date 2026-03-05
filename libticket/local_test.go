package libticket_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"github.com/simonski/ticket/libtickettest"
)

func TestLocalServiceContract(t *testing.T) {
	libtickettest.RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		tempDir := t.TempDir()
		t.Setenv("TICKET_MODE", "local")
		t.Setenv("TICKET_HOME", tempDir)
		dbPath := filepath.Join(tempDir, "ticket.db")
		if err := store.Init(dbPath, "admin", "secret"); err != nil {
			t.Fatalf("store.Init() error = %v", err)
		}
		return libticket.NewLocal(config.Config{})
	}, libtickettest.ContractOptions{RequireStatusOwnership: false})
}

func TestLocalServiceStatusDefaultsToAdmin(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
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
	svc := libticket.NewLocal(config.Config{})

	if _, err := svc.Register("alice", "secret"); err == nil {
		t.Fatal("Register() error = nil, want remote-mode error")
	}
	if _, _, err := svc.Login("alice", "secret"); err == nil {
		t.Fatal("Login() error = nil, want remote-mode error")
	}
	if err := svc.Logout(); err == nil {
		t.Fatal("Logout() error = nil, want remote-mode error")
	}
}

func TestLocalServiceStatusFailsWhenDatabaseMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
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
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
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
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
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

	updated, err := svc.SetTicketParent(child.ID, parent.ID)
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", updated)
	}

	detached, err := svc.UnsetTicketParent(child.ID)
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}
}

func TestLocalServiceUpdateTicketSupportsExpandedFields(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	parent, err := svc.CreateTicket(libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	task, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:          1,
		Type:               "task",
		Title:              "Child",
		Description:        "old description",
		AcceptanceCriteria: "old ac",
		Priority:           1,
		EstimateEffort:     2,
		EstimateComplete:   "2026-04-01T09:00:00Z",
		Stage:              "develop",
		State:              "idle",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	requested, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: 1, TicketID: &task.ID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}

	updated, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
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
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	task, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	updated, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
		Title:       task.Title,
		Description: task.Description,
		ParentID:    task.ParentID,
		Assignee:    task.Assignee,
		Status:      "done/complete",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Status != "done/complete" {
		t.Fatalf("UpdateTicket().Status = %q, want done/complete", updated.Status)
	}
}

func TestLocalServiceDeleteTicket(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)
	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	svc := libticket.NewLocal(config.Config{})
	task, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := svc.DeleteTicket(task.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := svc.GetTicketByID(task.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}
