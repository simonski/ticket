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
	t.Setenv("TICKET_URL", "")
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
	reparented, err := api.SetTicketParent(ticket.ID, parent.ID)
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if reparented.ParentID == nil || *reparented.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", reparented)
	}

	detached, err := api.UnsetTicketParent(ticket.ID)
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
	t.Setenv("TICKET_URL", "")
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
	t.Setenv("TICKET_URL", "")
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
	t.Setenv("TICKET_URL", "")
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
	t.Setenv("TICKET_URL", "")
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
