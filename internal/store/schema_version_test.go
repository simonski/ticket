package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func createLegacyDatabaseForTest(t *testing.T) (string, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: 1,
		Type:      "task",
		Title:     "Legacy ticket",
		CreatedBy: "",
	})
	if closeErr := db.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := rawDB.Exec(`DROP TABLE schema_meta`); err != nil {
		if closeErr := rawDB.Close(); closeErr != nil {
			t.Fatalf("rawDB.Close() error after drop failure = %v", closeErr)
		}
		t.Fatalf("DROP TABLE schema_meta error = %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("rawDB.Close() error = %v", err)
	}
	return dbPath, ticket.ID
}

func TestOpenRejectsLegacyDatabaseWithoutSchemaVersion(t *testing.T) {
	t.Parallel()

	dbPath, _ := createLegacyDatabaseForTest(t)
	_, err := Open(dbPath)
	if err == nil {
		t.Fatal("Open() error = nil, want schema version error")
	}
	var versionErr *SchemaVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("Open() error = %v, want SchemaVersionError", err)
	}
	if versionErr.Found != LegacySchemaVersion {
		t.Fatalf("SchemaVersionError.Found = %d, want %d", versionErr.Found, LegacySchemaVersion)
	}
	if !versionErr.UpgradeNeeded {
		t.Fatal("SchemaVersionError.UpgradeNeeded = false, want true")
	}
}

func TestUpgradeDatabasePortsLegacyDatabaseWithoutMutatingSource(t *testing.T) {
	t.Parallel()

	sourcePath, ticketID := createLegacyDatabaseForTest(t)
	targetPath := filepath.Join(t.TempDir(), "new_database", "ticket.db")

	if err := UpgradeDatabase(context.Background(), sourcePath, targetPath); err != nil {
		t.Fatalf("UpgradeDatabase() error = %v", err)
	}
	if got, err := DetectSchemaVersion(sourcePath); err != nil {
		t.Fatalf("DetectSchemaVersion(source) error = %v", err)
	} else if got != LegacySchemaVersion {
		t.Fatalf("DetectSchemaVersion(source) = %d, want %d", got, LegacySchemaVersion)
	}
	if got, err := DetectSchemaVersion(targetPath); err != nil {
		t.Fatalf("DetectSchemaVersion(target) error = %v", err)
	} else if got != CurrentSchemaVersion {
		t.Fatalf("DetectSchemaVersion(target) = %d, want %d", got, CurrentSchemaVersion)
	}

	targetDB, err := Open(targetPath)
	if err != nil {
		t.Fatalf("Open(target) error = %v", err)
	}
	defer targetDB.Close()
	ticket, err := GetTicket(context.Background(), targetDB, ticketID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.Title != "Legacy ticket" {
		t.Fatalf("ticket.Title = %q, want %q", ticket.Title, "Legacy ticket")
	}
}

func TestUpgradeDatabaseSupportsLegacyTicketsWithoutDraftColumn(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(t.TempDir(), "legacy-no-draft.db")
	rawDB, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = rawDB.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (
			user_id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			display_name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO users (user_id, username, password_hash, role, display_name) VALUES ('u1', 'admin', 'hash', 'admin', 'admin');
		CREATE TABLE projects (
			project_id INTEGER PRIMARY KEY AUTOINCREMENT,
			prefix TEXT NOT NULL DEFAULT 'TK',
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			acceptance_criteria TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open',
			created_by TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ticket_sequence INTEGER NOT NULL DEFAULT 1
		);
		INSERT INTO projects (project_id, prefix, title, created_by, ticket_sequence) VALUES (1, 'TK', 'Legacy', 'u1', 1);
		CREATE TABLE tickets (
			ticket_id TEXT PRIMARY KEY,
			project_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			acceptance_criteria TEXT NOT NULL DEFAULT '',
			stage TEXT NOT NULL DEFAULT 'develop',
			state TEXT NOT NULL DEFAULT 'idle',
			status TEXT NOT NULL DEFAULT 'develop/idle',
			priority INTEGER NOT NULL DEFAULT 3,
			assignee TEXT NOT NULL DEFAULT '',
			archived INTEGER NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO tickets (ticket_id, project_id, type, title, created_by) VALUES ('TK-1', 1, 'task', 'No draft column', 'u1');
	`)
	if err != nil {
		if closeErr := rawDB.Close(); closeErr != nil {
			t.Fatalf("rawDB.Close() error after exec failure = %v", closeErr)
		}
		t.Fatalf("rawDB.Exec() error = %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("rawDB.Close() error = %v", err)
	}

	targetPath := filepath.Join(t.TempDir(), "new_database", "ticket.db")
	if err := UpgradeDatabase(context.Background(), sourcePath, targetPath); err != nil {
		t.Fatalf("UpgradeDatabase() error = %v", err)
	}
	targetDB, err := Open(targetPath)
	if err != nil {
		t.Fatalf("Open(target) error = %v", err)
	}
	defer targetDB.Close()
	ticket, err := GetTicket(context.Background(), targetDB, "TK-1")
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if ticket.Title != "No draft column" {
		t.Fatalf("ticket.Title = %q, want %q", ticket.Title, "No draft column")
	}
}
