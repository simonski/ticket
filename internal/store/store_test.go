package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesDatabaseAndAdminUser(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")

	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'admin' AND role = 'admin'`).Scan(&count); err != nil {
		t.Fatalf("QueryRow(count) error = %v", err)
	}
	if count != 1 {
		t.Fatalf("admin row count = %d, want 1", count)
	}

	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE username = 'admin'`).Scan(&hash); err != nil {
		t.Fatalf("QueryRow(hash) error = %v", err)
	}
	if hash == "password" {
		t.Fatalf("password stored in plaintext")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("password hash = %q, want argon2id PHC string", hash)
	}

	assertTableExists(t, db, "projects")
	assertTableExists(t, db, "tickets")
	assertTableExists(t, db, "history_events")
	assertTableExists(t, db, "comments")
	assertTableExists(t, db, "dependencies")

	var users int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatalf("QueryRow(users) error = %v", err)
	}
	if users != 1 {
		t.Fatalf("user count = %d, want 1", users)
	}

	var projects int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects WHERE title = 'Default Project'`).Scan(&projects); err != nil {
		t.Fatalf("QueryRow(projects) error = %v", err)
	}
	if projects != 1 {
		t.Fatalf("default project count = %d, want 1", projects)
	}

	var prefix string
	if err := db.QueryRow(`SELECT prefix FROM projects WHERE title = 'Default Project'`).Scan(&prefix); err != nil {
		t.Fatalf("QueryRow(default project prefix) error = %v", err)
	}
	if prefix != "TK" {
		t.Fatalf("default project prefix = %q, want TK", prefix)
	}
}

func TestInitFailsIfDatabaseAlreadyExists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}

	if err := Init(dbPath, "admin", "password"); err == nil {
		t.Fatalf("second Init() error = nil, want error")
	}
}

func TestFixStaleForeignKeysMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ticket.db")

	// Create a DB with the old "tasks" table and stale FK references (pre-migration state).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rawDB.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (user_id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL, display_name TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO users (username, password_hash, role, display_name) VALUES ('admin', 'hash', 'admin', 'admin');
		CREATE TABLE projects (project_id INTEGER PRIMARY KEY AUTOINCREMENT, prefix TEXT NOT NULL DEFAULT 'TK', title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', acceptance_criteria TEXT NOT NULL DEFAULT '', git_repository TEXT NOT NULL DEFAULT '', git_branch TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'open', visibility TEXT NOT NULL DEFAULT 'public', created_by INTEGER, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, ticket_sequence INTEGER NOT NULL DEFAULT 0, FOREIGN KEY(created_by) REFERENCES users(user_id));
		INSERT INTO projects (title, created_by) VALUES ('Test', 1);
		CREATE TABLE tasks (task_id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT NOT NULL DEFAULT '', project_id INTEGER NOT NULL, parent_id INTEGER, clone_of INTEGER, type TEXT NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', acceptance_criteria TEXT NOT NULL DEFAULT '', git_repository TEXT NOT NULL DEFAULT '', git_branch TEXT NOT NULL DEFAULT '', stage TEXT NOT NULL DEFAULT 'design', state TEXT NOT NULL DEFAULT 'idle', status TEXT NOT NULL DEFAULT 'open', priority INTEGER NOT NULL DEFAULT 3, sort_order INTEGER NOT NULL DEFAULT 0, estimate_effort INTEGER NOT NULL DEFAULT 0, estimate_complete TEXT NOT NULL DEFAULT '', health_score INTEGER NOT NULL DEFAULT 0, assignee TEXT NOT NULL DEFAULT '', open INTEGER NOT NULL DEFAULT 1, archived INTEGER NOT NULL DEFAULT 0, created_by INTEGER, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(parent_id) REFERENCES tasks(task_id), FOREIGN KEY(clone_of) REFERENCES tasks(task_id), FOREIGN KEY(created_by) REFERENCES users(user_id));
		INSERT INTO tasks (key, project_id, type, title, created_by) VALUES ('TK-T-1', 1, 'task', 'Old task', 1);

		-- These tables have stale FKs referencing tasks instead of tickets
		CREATE TABLE history_events (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id INTEGER NOT NULL, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by INTEGER, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tasks(task_id), FOREIGN KEY(created_by) REFERENCES users(user_id));
		INSERT INTO history_events (project_id, ticket_id, event_type, created_by) VALUES (1, 1, 'created', 1);
		CREATE TABLE ticket_history (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id INTEGER NOT NULL, event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_by INTEGER, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tasks(task_id), FOREIGN KEY(created_by) REFERENCES users(user_id));
		CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, item_id INTEGER NOT NULL, user_id INTEGER NOT NULL, comment TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(item_id) REFERENCES tasks(task_id), FOREIGN KEY(user_id) REFERENCES users(user_id));
		CREATE TABLE dependencies (id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL, ticket_id INTEGER NOT NULL, depends_on INTEGER NOT NULL, created_by INTEGER, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(project_id) REFERENCES projects(project_id), FOREIGN KEY(ticket_id) REFERENCES tasks(task_id), FOREIGN KEY(depends_on) REFERENCES tasks(task_id), FOREIGN KEY(created_by) REFERENCES users(user_id));
	`)
	if err != nil {
		t.Fatal(err)
	}
	rawDB.Close()

	// Open via store.Open which runs migrations (including tasks→tickets rename and FK fix).
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() with stale FKs: %v", err)
	}
	defer db.Close()

	// Verify the tickets table exists (renamed from tasks) and has the old data.
	var title string
	if err := db.QueryRow(`SELECT title FROM tickets WHERE ticket_id = 1`).Scan(&title); err != nil {
		t.Fatalf("ticket not found after migration: %v", err)
	}
	if title != "Old task" {
		t.Fatalf("ticket title = %q, want %q", title, "Old task")
	}

	// Verify history_events data was preserved.
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_events`).Scan(&eventCount); err != nil {
		t.Fatalf("count history_events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("history_events count = %d, want 1", eventCount)
	}

	// The critical test: inserting into history_events with a ticket_id that exists
	// in the tickets table should now succeed (FK points to tickets, not tasks).
	ticket, err := CreateTicket(db, TicketCreateParams{
		ProjectID: 1,
		Type:      "task",
		Title:     "New ticket after migration",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket after FK migration: %v", err)
	}
	if ticket.ID == 0 {
		t.Fatal("created ticket has ID 0")
	}

	// Verify FK references are now correct (no stale references to tasks).
	for _, table := range []string{"history_events", "ticket_history", "comments", "dependencies"} {
		if tableHsFKTo(db, table, "tasks") {
			t.Errorf("table %s still has FK referencing tasks", table)
		}
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var found string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&found); err != nil {
		t.Fatalf("table %s not found: %v", name, err)
	}
}
