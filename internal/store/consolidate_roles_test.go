package store

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

// TestConsolidateRolesMigrationNoDataLoss simulates the pre-consolidation roles
// shape (guidance columns present and populated) and verifies the upgrade folds the
// values into attrs with no loss, drops the columns, and the values remain readable
// via the typed Role fields.
func TestConsolidateRolesMigrationNoDataLoss(t *testing.T) {
	// Not parallel: manipulates schema_version and raw columns.
	ctx := context.Background()
	db, path := attrsTestDB(t)
	r, err := CreateRoleWithParams(ctx, db, RoleCreateParams{Title: "Engineer"})
	if err != nil {
		t.Fatalf("CreateRoleWithParams() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE roles ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roles ADD COLUMN acceptance_criteria TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roles ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE roles ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE roles ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`,
	} {
		if _, execErr := raw.Exec(stmt); execErr != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, execErr)
		}
	}
	if _, execErr := raw.Exec(
		`UPDATE roles SET description=?, acceptance_criteria=?, dor_map=?, dod_map=?, ac_map=?, attrs='{}' WHERE role_id=?`,
		"builds the thing", "compiles", `{"default":"branch exists"}`, `{"develop":"tests pass"}`, `{"default":"meets AC"}`, r.ID,
	); execErr != nil {
		_ = raw.Close()
		t.Fatalf("populate error = %v", execErr)
	}
	if _, execErr := raw.Exec(`UPDATE schema_meta SET value='11' WHERE key='schema_version'`); execErr != nil {
		_ = raw.Close()
		t.Fatalf("set version error = %v", execErr)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close() error = %v", err)
	}

	if _, err := UpgradeInPlace(ctx, path); err != nil {
		t.Fatalf("UpgradeInPlace() error = %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after upgrade error = %v", err)
	}
	defer db2.Close()

	got, err := GetRoleByID(ctx, db2, r.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	if got.Description != "builds the thing" {
		t.Errorf("Description = %q, want preserved", got.Description)
	}
	if got.AcceptanceCriteria != "compiles" {
		t.Errorf("AcceptanceCriteria = %q, want preserved", got.AcceptanceCriteria)
	}
	if !reflect.DeepEqual(got.DORMap, GuidanceMap{"default": "branch exists"}) {
		t.Errorf("DORMap = %#v, want preserved", got.DORMap)
	}
	if got.DODMap["develop"] != "tests pass" {
		t.Errorf("DODMap = %#v, want preserved", got.DODMap)
	}
	for _, col := range []string{"description", "acceptance_criteria", "dor_map", "dod_map", "ac_map"} {
		if columnExists(ctx, db2, "roles", col) {
			t.Errorf("roles.%s still exists after consolidation", col)
		}
	}
}

// TestConsolidatedRoleRoundTrip verifies role guidance still round-trips through
// the typed fields after consolidation, with the extra bag preserved.
func TestConsolidatedRoleRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	r, err := CreateRoleWithParams(ctx, db, RoleCreateParams{
		Title: "QA", Description: "tests things", AcceptanceCriteria: "all green",
		DORMap: GuidanceMap{"default": "ready"},
		Attrs:  Attrs{"color": "purple"},
	})
	if err != nil {
		t.Fatalf("CreateRoleWithParams() error = %v", err)
	}
	got, err := GetRoleByID(ctx, db, r.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	if got.Description != "tests things" || got.AcceptanceCriteria != "all green" {
		t.Fatalf("role text fields not round-tripped: %+v", got)
	}
	if got.DORMap["default"] != "ready" {
		t.Fatalf("DORMap not round-tripped: %#v", got.DORMap)
	}
	if got.Attrs.GetString("color") != "purple" {
		t.Fatalf("extra attr lost: %#v", got.Attrs)
	}
	for _, k := range []string{"description", "acceptance_criteria", "dor_map", "dod_map", "ac_map"} {
		if _, dup := got.Attrs[k]; dup {
			t.Fatalf("attrs duplicates typed field %q: %#v", k, got.Attrs)
		}
	}
}
