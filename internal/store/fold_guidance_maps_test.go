package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestFoldTicketGuidanceMapsMigration simulates a pre-fold tickets table with
// dor_map/dod_map/ac_map columns populated, and verifies the upgrade embeds them
// into attrs as nested objects with no loss, drops the columns, and the maps remain
// readable via the typed Ticket fields.
func TestFoldTicketGuidanceMapsMigration(t *testing.T) {
	ctx := context.Background()
	db, path := attrsTestDB(t)
	tk, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: 1, Type: "task", Title: "g"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE tickets ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE tickets ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE tickets ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`,
	} {
		if _, e := raw.Exec(stmt); e != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, e)
		}
	}
	if _, e := raw.Exec(`UPDATE tickets SET dor_map=?, dod_map=?, ac_map=?, attrs='{}' WHERE ticket_id=?`,
		`{"default":"ready"}`, `{"develop":"done"}`, `{"default":"ac"}`, tk.ID); e != nil {
		_ = raw.Close()
		t.Fatalf("populate error = %v", e)
	}
	if _, e := raw.Exec(`UPDATE schema_meta SET value='11' WHERE key='schema_version'`); e != nil {
		_ = raw.Close()
		t.Fatalf("set version error = %v", e)
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
	got, err := GetTicket(ctx, db2, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if got.DORMap["default"] != "ready" || got.DODMap["develop"] != "done" || got.ACMap["default"] != "ac" {
		t.Fatalf("guidance maps not preserved: dor=%#v dod=%#v ac=%#v", got.DORMap, got.DODMap, got.ACMap)
	}
	for _, col := range []string{"dor_map", "dod_map", "ac_map"} {
		if columnExists(ctx, db2, "tickets", col) {
			t.Errorf("tickets.%s still exists after fold", col)
		}
	}
}

// TestFoldProjectGuidanceMapsMigration is the projects counterpart.
func TestFoldProjectGuidanceMapsMigration(t *testing.T) {
	ctx := context.Background()
	db, path := attrsTestDB(t)
	p, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{Title: "P", Prefix: "PFG"})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE projects ADD COLUMN dor_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE projects ADD COLUMN dod_map TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE projects ADD COLUMN ac_map TEXT NOT NULL DEFAULT '{}'`,
	} {
		if _, e := raw.Exec(stmt); e != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, e)
		}
	}
	if _, e := raw.Exec(`UPDATE projects SET dor_map=?, dod_map=?, ac_map=?, attrs='{}' WHERE project_id=?`,
		`{"default":"pready"}`, `{"default":"pdone"}`, `{"default":"pac"}`, p.ID); e != nil {
		_ = raw.Close()
		t.Fatalf("populate error = %v", e)
	}
	if _, e := raw.Exec(`UPDATE schema_meta SET value='11' WHERE key='schema_version'`); e != nil {
		_ = raw.Close()
		t.Fatalf("set version error = %v", e)
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
	got, err := GetProjectByID(ctx, db2, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if got.DORMap["default"] != "pready" || got.DODMap["default"] != "pdone" || got.ACMap["default"] != "pac" {
		t.Fatalf("project guidance maps not preserved: dor=%#v dod=%#v ac=%#v", got.DORMap, got.DODMap, got.ACMap)
	}
	for _, col := range []string{"dor_map", "dod_map", "ac_map"} {
		if columnExists(ctx, db2, "projects", col) {
			t.Errorf("projects.%s still exists after fold", col)
		}
	}
}

// TestFoldGuidanceMapsRoundTrip checks create/read of guidance maps via typed
// fields after the fold, with no attrs duplication.
func TestFoldGuidanceMapsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	tk, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: 1, Type: "task", Title: "rt",
		DORMap: GuidanceMap{"default": "x"}, ACMap: GuidanceMap{"default": "y"},
		Attrs: Attrs{"k": "v"},
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	got, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if got.DORMap["default"] != "x" || got.ACMap["default"] != "y" {
		t.Fatalf("guidance maps not round-tripped: %#v / %#v", got.DORMap, got.ACMap)
	}
	if got.Attrs.GetString("k") != "v" {
		t.Fatalf("extra attr lost: %#v", got.Attrs)
	}
	for _, k := range []string{"dor_map", "dod_map", "ac_map"} {
		if _, dup := got.Attrs[k]; dup {
			t.Fatalf("attrs duplicates guidance map %q: %#v", k, got.Attrs)
		}
	}
}
