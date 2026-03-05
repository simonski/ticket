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
	assertTableExists(t, db, "tasks")
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

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var found string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&found); err != nil {
		t.Fatalf("table %s not found: %v", name, err)
	}
}
