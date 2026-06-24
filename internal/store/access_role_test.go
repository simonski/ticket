package store

import (
	"context"
	"database/sql"
	"testing"
)

func openAccessRoleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := createSchema(context.Background(), db); err != nil {
		t.Fatalf("createSchema: %v", err)
	}
	return db
}

func TestEnsureDefaultAccessRolesIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)
	if err := EnsureDefaultAccessRoles(ctx, db); err != nil {
		t.Fatalf("ensure 1: %v", err)
	}
	if err := EnsureDefaultAccessRoles(ctx, db); err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	roles, err := ListAccessRoles(ctx, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(roles) != 1 || roles[0].Name != builtinMemberRoleName || !roles[0].Builtin {
		t.Fatalf("want one builtin Member role, got %+v", roles)
	}
	if len(roles[0].Panels) != len(grantablePanels) {
		t.Fatalf("Member role should grant all %d grantable panels, got %v", len(grantablePanels), roles[0].Panels)
	}
}

func TestAccessRoleCRUDAndPanelValidation(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)

	r, err := CreateAccessRole(ctx, db, "Viewer", "read-only", []string{PanelTickets, PanelProjects})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(r.Panels) != 2 {
		t.Fatalf("want 2 panels, got %v", r.Panels)
	}

	// Admin-only panels cannot be granted.
	if _, err := CreateAccessRole(ctx, db, "Sneaky", "", []string{PanelUsers}); err == nil {
		t.Fatal("granting an admin panel should fail")
	}
	// Unknown panels are rejected.
	if _, err := CreateAccessRole(ctx, db, "Bogus", "", []string{"nope"}); err == nil {
		t.Fatal("unknown panel should fail")
	}
	// Duplicate names are rejected.
	if _, err := CreateAccessRole(ctx, db, "Viewer", "", []string{PanelTickets}); err == nil {
		t.Fatal("duplicate name should fail")
	}

	updated, err := UpdateAccessRole(ctx, db, r.ID, "Viewer+", "", []string{PanelTickets, PanelChat, PanelDocuments})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Viewer+" || len(updated.Panels) != 3 {
		t.Fatalf("update mismatch: %+v", updated)
	}

	if err := DeleteAccessRole(ctx, db, r.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetAccessRole(ctx, db, r.ID); err != ErrAccessRoleNotFound {
		t.Fatalf("want ErrAccessRoleNotFound, got %v", err)
	}
}

func TestDeleteBuiltinAccessRoleRejected(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)
	roles, _ := ListAccessRoles(ctx, db)
	if err := DeleteAccessRole(ctx, db, roles[0].ID); err == nil {
		t.Fatal("deleting the builtin role should fail")
	}
}

func TestEffectivePanels(t *testing.T) {
	ctx := context.Background()
	db := openAccessRoleTestDB(t)

	// Admin sees every panel including admin-only ones.
	adminPanelsSet, err := EffectivePanels(ctx, db, "admin-user", true)
	if err != nil {
		t.Fatalf("admin panels: %v", err)
	}
	if len(adminPanelsSet) != len(AllPanels()) {
		t.Fatalf("admin should see all %d panels, got %d", len(AllPanels()), len(adminPanelsSet))
	}

	// A user with no assigned roles is grandfathered to the default member set.
	def, err := EffectivePanels(ctx, db, "u1", false)
	if err != nil {
		t.Fatalf("default panels: %v", err)
	}
	if len(def) != len(grantablePanels) {
		t.Fatalf("ungated user should see default %d panels, got %v", len(grantablePanels), def)
	}

	// Assigning a restrictive role narrows the set.
	role, err := CreateAccessRole(ctx, db, "TicketsOnly", "", []string{PanelTickets})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := SetUserAccessRoles(ctx, db, "u1", []int64{role.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	got, err := EffectivePanels(ctx, db, "u1", false)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}
	if len(got) != 1 || got[0] != PanelTickets {
		t.Fatalf("want [tickets], got %v", got)
	}

	ok, err := UserCanAccessPanel(ctx, db, "u1", false, PanelProjects)
	if err != nil || ok {
		t.Fatalf("u1 should NOT access projects (ok=%v err=%v)", ok, err)
	}
	ok, _ = UserCanAccessPanel(ctx, db, "u1", false, PanelTickets)
	if !ok {
		t.Fatal("u1 should access tickets")
	}
	// Admin-only panel denied to non-admin even if somehow requested.
	ok, _ = UserCanAccessPanel(ctx, db, "u1", false, PanelUsers)
	if ok {
		t.Fatal("non-admin must never access an admin panel")
	}
}
