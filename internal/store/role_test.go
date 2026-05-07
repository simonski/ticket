package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
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
	t.Parallel()

	t.Skip("no default roles — roles are now per-Workflow")
}

func TestRoleCRUD(t *testing.T) {
	t.Parallel()
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

func TestRoleGuidanceMapsPersistAndResolve(t *testing.T) {
	t.Parallel()
	db := openRoleTestDB(t)
	defer db.Close()

	role, err := CreateRoleWithParams(context.Background(), db, RoleCreateParams{
		Title:              "QA",
		Description:        "Own quality",
		AcceptanceCriteria: "legacy role ac",
		DORMap:             GuidanceMap{"default": "role default dor", "qa": "role qa dor"},
		DODMap:             GuidanceMap{"default": "role default dod"},
		ACMap:              GuidanceMap{"develop": "role develop ac"},
	})
	if err != nil {
		t.Fatalf("CreateRoleWithParams() error = %v", err)
	}
	if !reflect.DeepEqual(role.DORMap, GuidanceMap{"default": "role default dor", "qa": "role qa dor"}) {
		t.Fatalf("CreateRoleWithParams().DORMap = %#v", role.DORMap)
	}
	if !reflect.DeepEqual(role.ACMap, GuidanceMap{"default": "legacy role ac", "develop": "role develop ac"}) {
		t.Fatalf("CreateRoleWithParams().ACMap = %#v", role.ACMap)
	}

	reloaded, err := GetRoleByID(context.Background(), db, role.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	resolved := reloaded.ResolveGuidance("qa")
	if !resolved.HasDOR || resolved.DOR != "role qa dor" {
		t.Fatalf("ResolveGuidance(qa).DOR = %#v", resolved)
	}
	if !resolved.HasDOD || resolved.DOD != "role default dod" {
		t.Fatalf("ResolveGuidance(qa).DOD = %#v", resolved)
	}
	if !resolved.HasAC || resolved.AC != "legacy role ac" {
		t.Fatalf("ResolveGuidance(qa).AC = %#v", resolved)
	}

	updated, err := UpdateRoleWithParams(context.Background(), db, role.ID, RoleUpdateParams{
		Title:  "QA",
		DORMap: GuidanceMap{"develop": "updated role dor"},
		DODMap: GuidanceMap{"develop": "updated role dod"},
		ACMap:  GuidanceMap{"develop": "updated role ac"},
	})
	if err != nil {
		t.Fatalf("UpdateRoleWithParams() error = %v", err)
	}
	if !reflect.DeepEqual(updated.DODMap, GuidanceMap{"develop": "updated role dod"}) {
		t.Fatalf("UpdateRoleWithParams().DODMap = %#v", updated.DODMap)
	}
	if !reflect.DeepEqual(updated.ACMap, GuidanceMap{"default": "legacy role ac", "develop": "updated role ac"}) {
		t.Fatalf("UpdateRoleWithParams().ACMap = %#v", updated.ACMap)
	}
}

func TestListRolesByWorkflowAndGetRoleByTitle(t *testing.T) {
	t.Parallel()
	db := openRoleTestDB(t)
	defer db.Close()

	wf, err := CreateWorkflow(context.Background(), db, "role-scope", "scoped roles")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	globalRole, err := CreateRole(context.Background(), db, nil, "Global QA", "global", "covers all workflows")
	if err != nil {
		t.Fatalf("CreateRole(global) error = %v", err)
	}
	scopedRole, err := CreateRole(context.Background(), db, &wf.ID, "Workflow QA", "scoped", "covers one workflow")
	if err != nil {
		t.Fatalf("CreateRole(scoped) error = %v", err)
	}

	allRoles, err := ListRoles(context.Background(), db, 10)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(allRoles) < 2 {
		t.Fatalf("ListRoles() len = %d, want at least 2", len(allRoles))
	}

	scopedRoles, err := ListRolesByWorkflow(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("ListRolesByWorkflow() error = %v", err)
	}
	if len(scopedRoles) != 1 || scopedRoles[0].ID != scopedRole.ID {
		t.Fatalf("ListRolesByWorkflow() = %#v, want only scoped role %d", scopedRoles, scopedRole.ID)
	}

	byTitle, err := GetRoleByTitle(context.Background(), db, " Workflow QA ")
	if err != nil {
		t.Fatalf("GetRoleByTitle() error = %v", err)
	}
	if byTitle.ID != scopedRole.ID {
		t.Fatalf("GetRoleByTitle().ID = %d, want %d", byTitle.ID, scopedRole.ID)
	}
	if globalRole.ID == 0 {
		t.Fatalf("expected global role to be created")
	}
}

func TestDefaultRoleContentIsDetailed(t *testing.T) {
	t.Parallel()
	t.Skip("no default roles — roles are now per-Workflow")
	db := openRoleTestDB(t)
	defer db.Close()

	roles, err := ListRoles(context.Background(), db, 0)
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

// Legacy seedDefaultRoles test removed — function was deleted.
// Roles are now seeded from embedded static files by tk init.
