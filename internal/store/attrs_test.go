package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func attrsTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "attrs.db")
	if err := Init(path, "admin", "secret12"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func TestTicketAttrsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	// Empty bag: created without attrs reads back empty.
	plain, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: 1, Type: "task", Title: "plain"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if len(plain.Attrs) != 0 {
		t.Fatalf("new ticket attrs = %v, want empty", plain.Attrs)
	}

	// Populated + nested bag round-trips through create.
	nested := Attrs{
		"repro_steps": "open app; click; crash",
		"severity":    "high",
		"meta":        map[string]any{"count": float64(3), "tags": []any{"a", "b"}},
	}
	tk, err := CreateTicket(ctx, db, TicketCreateParams{ProjectID: 1, Type: "bug", Title: "bug", Attrs: nested})
	if err != nil {
		t.Fatalf("CreateTicket(with attrs) error = %v", err)
	}
	got, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if !reflect.DeepEqual(got.Attrs, nested) {
		t.Fatalf("round-tripped attrs = %#v, want %#v", got.Attrs, nested)
	}

	// Update with nil Attrs preserves the existing bag.
	if _, err := UpdateTicket(ctx, db, tk.ID, TicketUpdateParams{Title: "bug2", ActorUsername: "admin", ActorRole: "admin"}); err != nil {
		t.Fatalf("UpdateTicket(preserve) error = %v", err)
	}
	preserved, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if !reflect.DeepEqual(preserved.Attrs, nested) {
		t.Fatalf("after preserve update attrs = %#v, want %#v", preserved.Attrs, nested)
	}

	// Update with a new bag replaces it.
	replacement := Attrs{"severity": "low"}
	if _, err := UpdateTicket(ctx, db, tk.ID, TicketUpdateParams{Title: "bug2", Attrs: replacement, ActorUsername: "admin", ActorRole: "admin"}); err != nil {
		t.Fatalf("UpdateTicket(replace) error = %v", err)
	}
	replaced, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if !reflect.DeepEqual(replaced.Attrs, replacement) {
		t.Fatalf("after replace attrs = %#v, want %#v", replaced.Attrs, replacement)
	}
}

func TestProjectAttrsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	bag := Attrs{"jira_key": "ABC-1", "config": map[string]any{"x": float64(1)}}
	p, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{Title: "P", Prefix: "PX", Attrs: bag})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	got, err := GetProjectByID(ctx, db, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if !reflect.DeepEqual(got.Attrs, bag) {
		t.Fatalf("project attrs = %#v, want %#v", got.Attrs, bag)
	}
	// Preserve on update without Attrs.
	if _, err := UpdateProjectWithParams(ctx, db, p.ID, ProjectUpdateParams{Title: "P2"}); err != nil {
		t.Fatalf("UpdateProjectWithParams() error = %v", err)
	}
	pres, err := GetProjectByID(ctx, db, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID() error = %v", err)
	}
	if !reflect.DeepEqual(pres.Attrs, bag) {
		t.Fatalf("project attrs after preserve = %#v, want %#v", pres.Attrs, bag)
	}
}

func TestRoleAttrsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	bag := Attrs{"color": "blue"}
	r, err := CreateRoleWithParams(ctx, db, RoleCreateParams{Title: "Architect", Attrs: bag})
	if err != nil {
		t.Fatalf("CreateRoleWithParams() error = %v", err)
	}
	got, err := GetRoleByID(ctx, db, r.ID)
	if err != nil {
		t.Fatalf("GetRoleByID() error = %v", err)
	}
	if !reflect.DeepEqual(got.Attrs, bag) {
		t.Fatalf("role attrs = %#v, want %#v", got.Attrs, bag)
	}
}

func TestWorkflowStageAttrsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	wf, err := CreateWorkflow(ctx, db, "wf-attrs", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStageWithDefinitions(ctx, db, wf.ID, "design", "", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStageWithDefinitions() error = %v", err)
	}
	if len(stage.Attrs) != 0 {
		t.Fatalf("new stage attrs = %v, want empty", stage.Attrs)
	}
	bag := Attrs{"board_color": "green", "wip_limit": float64(5)}
	updated, err := SetWorkflowStageAttrs(ctx, db, stage.ID, bag)
	if err != nil {
		t.Fatalf("SetWorkflowStageAttrs() error = %v", err)
	}
	if !reflect.DeepEqual(updated.Attrs, bag) {
		t.Fatalf("stage attrs = %#v, want %#v", updated.Attrs, bag)
	}
	// A normal stage update must preserve attrs (attrs is not in its SET clause).
	if _, err := UpdateWorkflowStage(ctx, db, stage.ID, "design", "desc", "ac"); err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	wfs, err := GetWorkflow(ctx, db, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(wfs.Stages) == 0 || !reflect.DeepEqual(wfs.Stages[0].Attrs, bag) {
		t.Fatalf("stage attrs after normal update not preserved: %+v", wfs.Stages)
	}
}

// TestAttrsZeroDDLExtensibility demonstrates that a brand-new optional field can be
// stored and read back with NO schema migration and NO schema-version bump — the
// whole point of the bag. The schema version is identical before and after.
func TestAttrsZeroDDLExtensibility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, path := attrsTestDB(t)

	before, err := DetectSchemaVersion(path)
	if err != nil {
		t.Fatalf("DetectSchemaVersion() error = %v", err)
	}
	// A field that does not exist anywhere in the schema today.
	tk, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: 1, Type: "spike", Title: "timeboxed",
		Attrs: Attrs{"timebox_hours": float64(8), "spike_outcome": "spike-doc"},
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	got, err := GetTicket(ctx, db, tk.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if got.Attrs.GetInt("timebox_hours") != 8 || got.Attrs.GetString("spike_outcome") != "spike-doc" {
		t.Fatalf("new field not stored/read: %#v", got.Attrs)
	}
	after, err := DetectSchemaVersion(path)
	if err != nil {
		t.Fatalf("DetectSchemaVersion() error = %v", err)
	}
	if before != after {
		t.Fatalf("schema version changed (%d -> %d) for a new Tier-2 field; expected no migration", before, after)
	}
}

// TestAttrsMigrationReaddsColumns exercises the additive attrs migration: dropping
// the attrs columns and re-running the in-place upgrade restores them.
func TestAttrsMigrationReaddsColumns(t *testing.T) {
	// Not parallel: opens the DB with a second raw connection.
	ctx := context.Background()
	db, path := attrsTestDB(t)
	_ = db.Close()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, table := range []string{"tickets", "projects", "roles", "workflow_stages"} {
		if _, err := raw.Exec(`ALTER TABLE ` + table + ` DROP COLUMN attrs`); err != nil {
			_ = raw.Close()
			t.Fatalf("drop attrs from %s error = %v", table, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close() error = %v", err)
	}

	if _, err := UpgradeInPlace(ctx, path); err != nil {
		t.Fatalf("UpgradeInPlace() error = %v", err)
	}

	check, err := Open(path)
	if err != nil {
		t.Fatalf("Open() after upgrade error = %v", err)
	}
	defer check.Close()
	for _, table := range []string{"tickets", "projects", "roles", "workflow_stages"} {
		if !columnExists(ctx, check, table, "attrs") {
			t.Fatalf("attrs column not restored on %s", table)
		}
	}
}
