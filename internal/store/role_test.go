package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
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

	roles, err := ListRoles(context.Background(), db)
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

	created, err := CreateRole(context.Background(), db, nil, "Principal QA", "Own quality", "Automate and validate")
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("CreateRole().ID = 0")
	}

	updated, err := UpdateRole(context.Background(), db, created.ID, "Principal QA+", "Improve quality", "Expand test strategy")
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if updated.Title != "Principal QA+" {
		t.Fatalf("UpdateRole().Title = %q, want %q", updated.Title, "Principal QA+")
	}

	if err := DeleteRole(context.Background(), db, created.ID); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
	if _, err := GetRoleByID(context.Background(), db, created.ID); err == nil {
		t.Fatalf("GetRoleByID(deleted) error = nil, want error")
	}
}

func TestDefaultRoleContentIsDetailed(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	roles, err := ListRoles(context.Background(), db)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	var productOwner Role
	found := false
	for _, role := range roles {
		if role.Title == "Product Owner" {
			productOwner = role
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Product Owner role not seeded")
	}
	if !strings.Contains(productOwner.Description, "\n\n") {
		t.Fatalf("Product Owner motivation should contain multiple paragraphs")
	}
	if !strings.Contains(productOwner.AcceptanceCriteria, "\n\n") {
		t.Fatalf("Product Owner goals should contain multiple paragraphs")
	}
}

// TestSeedDefaultRolesIsNoOp verifies that seedDefaultRoles is a no-op (roles are now per-SDLC).
func TestSeedDefaultRolesIsNoOp(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	roles, _ := ListRoles(context.Background(), db)
	before := len(roles)

	if err := seedDefaultRoles(context.Background(), db); err != nil {
		t.Fatalf("seedDefaultRoles() error = %v", err)
	}

	roles, _ = ListRoles(context.Background(), db)
	if len(roles) != before {
		t.Fatalf("seedDefaultRoles() should be a no-op, but role count changed: %d -> %d", before, len(roles))
	}
}

func getRoleByTitle(db *sql.DB, title string) (Role, error) {
	row := db.QueryRow(`
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, title)
	var role Role
	var sdlcID sql.NullInt64
	if err := row.Scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	return role, nil
}
