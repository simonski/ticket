package store

import (
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

func TestDefaultRoleContentIsDetailed(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	roles, err := ListRoles(db)
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
	if !strings.Contains(productOwner.Motivation, "\n\n") {
		t.Fatalf("Product Owner motivation should contain multiple paragraphs")
	}
	if !strings.Contains(productOwner.Goals, "\n\n") {
		t.Fatalf("Product Owner goals should contain multiple paragraphs")
	}
}

func TestSeedDefaultRolesBackfillsLegacyRoleText(t *testing.T) {
	db := openRoleTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`
		UPDATE roles
		SET motivation = 'Maintain coherent system design.',
		    goals = 'Define architecture guardrails and reduce complexity.'
		WHERE title = 'Architect'
	`); err != nil {
		t.Fatalf("seed setup update error = %v", err)
	}

	if err := seedDefaultRoles(db); err != nil {
		t.Fatalf("seedDefaultRoles() error = %v", err)
	}

	role, err := getRoleByTitle(db, "Architect")
	if err != nil {
		t.Fatalf("getRoleByTitle() error = %v", err)
	}
	if !strings.Contains(role.Motivation, "\n\n") {
		t.Fatalf("Architect motivation should be backfilled to detailed content")
	}
	if !strings.Contains(role.Goals, "\n\n") {
		t.Fatalf("Architect goals should be backfilled to detailed content")
	}
}

func getRoleByTitle(db *sql.DB, title string) (Role, error) {
	row := db.QueryRow(`
		SELECT role_id, title, motivation, goals, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, title)
	var role Role
	if err := row.Scan(&role.ID, &role.Title, &role.Motivation, &role.Goals, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	return role, nil
}
