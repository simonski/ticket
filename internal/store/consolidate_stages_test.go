package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestConsolidateStagesMigrationNoDataLoss simulates the pre-consolidation
// workflow_stages shape (guidance text columns present and populated) and verifies
// the upgrade folds the values into attrs with no loss, drops the columns, and the
// values remain readable via the typed WorkflowStage fields.
func TestConsolidateStagesMigrationNoDataLoss(t *testing.T) {
	// Not parallel: manipulates schema_version and raw columns.
	ctx := context.Background()
	db, path := attrsTestDB(t)
	wf, err := CreateWorkflow(ctx, db, "wf-mig", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStageWithDefinitions(ctx, db, wf.ID, "design", "", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStageWithDefinitions() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE workflow_stages ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE workflow_stages ADD COLUMN acceptance_criteria TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE workflow_stages ADD COLUMN definition_of_ready TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE workflow_stages ADD COLUMN definition_of_done TEXT NOT NULL DEFAULT ''`,
	} {
		if _, execErr := raw.Exec(stmt); execErr != nil {
			_ = raw.Close()
			t.Fatalf("setup %q error = %v", stmt, execErr)
		}
	}
	if _, execErr := raw.Exec(
		`UPDATE workflow_stages SET description=?, acceptance_criteria=?, definition_of_ready=?, definition_of_done=?, attrs='{}' WHERE workflow_stage_id=?`,
		"design phase", "approved design", "brief ready", "design signed off", stage.ID,
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

	got, err := GetWorkflowStage(ctx, db2, stage.ID)
	if err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if got.Description != "design phase" || got.AcceptanceCriteria != "approved design" ||
		got.DefinitionOfReady != "brief ready" || got.DefinitionOfDone != "design signed off" {
		t.Fatalf("stage guidance not preserved: %+v", got)
	}
	for _, col := range []string{"description", "acceptance_criteria", "definition_of_ready", "definition_of_done"} {
		if columnExists(ctx, db2, "workflow_stages", col) {
			t.Errorf("workflow_stages.%s still exists after consolidation", col)
		}
	}
}

// TestConsolidatedStageRoundTripAndExport verifies stage guidance round-trips via
// typed fields after consolidation, that SetWorkflowStageAttrs preserves guidance,
// and that workflow export/import still carries the stage description.
func TestConsolidatedStageRoundTripAndExport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, _ := attrsTestDB(t)

	wf, err := CreateWorkflow(ctx, db, "wf-rt", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStageWithDefinitions(ctx, db, wf.ID, "build", "build it", "branch", "merged", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStageWithDefinitions() error = %v", err)
	}
	if stage.Description != "build it" || stage.DefinitionOfDone != "merged" {
		t.Fatalf("stage guidance not round-tripped on create: %+v", stage)
	}

	// Setting extra attrs must preserve the guidance text.
	updated, err := SetWorkflowStageAttrs(ctx, db, stage.ID, Attrs{"board_color": "green"})
	if err != nil {
		t.Fatalf("SetWorkflowStageAttrs() error = %v", err)
	}
	if updated.Description != "build it" {
		t.Fatalf("SetWorkflowStageAttrs wiped guidance: %+v", updated)
	}
	if updated.Attrs.GetString("board_color") != "green" {
		t.Fatalf("extra attr lost: %#v", updated.Attrs)
	}
	if _, dup := updated.Attrs["description"]; dup {
		t.Fatalf("attrs duplicates guidance field: %#v", updated.Attrs)
	}

	// Workflow export/import must still carry the stage description.
	export, err := ExportWorkflow(ctx, db, wf.ID)
	if err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	export.Name = "wf-rt-copy"
	imported, err := ImportWorkflow(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}
	stages, err := ListWorkflowStages(ctx, db, imported.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStages() error = %v", err)
	}
	if len(stages) == 0 || stages[0].Description != "build it" {
		t.Fatalf("export/import did not preserve stage description: %+v", stages)
	}
}
