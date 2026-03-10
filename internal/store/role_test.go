package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func openRoleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return db
}

func TestDefaultRolesSeeded(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	roles, err := ListRoles(db)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) < 7 {
		t.Fatalf("default roles len = %d, want >= 7", len(roles))
	}
}

func TestRoleCRUD(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	created, err := CreateRole(db, "Principal QA", "Own quality", "Automate and validate")
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("CreateRole().ID = 0")
	}

	updated, err := UpdateRole(db, created.ID, "Principal QA+", "Improve quality", "Expand test strategy")
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updated.Title != "Principal QA+" {
		t.Fatalf("UpdateRole().Title = %q, want %q", updated.Title, "Principal QA+")
	}

	if err := DeleteRole(db, created.ID); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
	if _, err := GetRoleByID(db, created.ID); err == nil {
		t.Fatalf("GetRoleByID(deleted) error = nil, want error")
	}
}
