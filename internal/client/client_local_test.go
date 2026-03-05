package client

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func TestLocalModeClientUsesSQLiteDirectly(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
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

	task, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Local task",
		Stage:     "develop",
		State:     "idle",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if strings.TrimSpace(task.Assignee) != "" || task.Status != "develop/idle" {
		t.Fatalf("CreateTicket() = %#v", task)
	}

	requested, err := api.RequestTicket(TicketRequest{ProjectID: 1})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if requested.Status != "ASSIGNED" || requested.Ticket == nil {
		t.Fatalf("RequestTicket() = %#v", requested)
	}

	updated, err := api.UpdateTicket(task.ID, TicketUpdateRequest{
		Title:       task.Title,
		Description: task.Description,
		ParentID:    task.ParentID,
		Assignee:    requested.Ticket.Assignee,
		Stage:       "develop",
		State:       "active",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Status != "develop/active" {
		t.Fatalf("UpdateTicket().Status = %q, want develop/active", updated.Status)
	}

	parent, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent epic",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	reparented, err := api.SetTicketParent(task.ID, parent.ID)
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if reparented.ParentID == nil || *reparented.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", reparented)
	}

	detached, err := api.UnsetTicketParent(task.ID)
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}

	comment, err := api.AddComment(task.ID, "hello")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.Text != "hello" || comment.Author == "" {
		t.Fatalf("AddComment() = %#v", comment)
	}
}

func TestLocalModeClientIgnoresOwnershipForStatusChanges(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	task, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if strings.TrimSpace(task.Assignee) != "" {
		t.Fatalf("CreateTicket().Assignee = %q, want unassigned", task.Assignee)
	}

	updated, err := api.UpdateTicket(task.ID, TicketUpdateRequest{
		Title:       task.Title,
		Description: task.Description,
		ParentID:    task.ParentID,
		Assignee:    task.Assignee,
		Stage:       "done",
		State:       "complete",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Status != "done/complete" {
		t.Fatalf("UpdateTicket().Status = %q, want done/complete", updated.Status)
	}
}

func TestLocalModeClientDeleteTicket(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	api := New(config.Config{})
	task, err := api.CreateTicket(TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := api.DeleteTicket(task.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := api.GetTicketByID(task.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func TestLocalModeClientStatusIsReadOnlyWithoutMatchingUser(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
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
	if _, err := store.GetUserByUsername(mustOpenDB(t, dbPath), localUsername()); err != nil {
		t.Fatalf("Status() should use existing admin user, err = %v", err)
	}
}

func TestLocalModeClientStatusFailsWhenDatabaseMissing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_MODE", "local")
	t.Setenv("TICKET_HOME", tempDir)

	api := New(config.Config{})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want missing database error")
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
