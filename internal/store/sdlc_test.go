package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func setupSdlcTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInitSeedsDefaultSdlc(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)
	sdlcs, err := ListSdlcs(context.Background(), db, 0, 0)
	if err != nil {
		t.Fatalf("ListSdlcs() error = %v", err)
	}
	if len(sdlcs) == 0 {
		t.Fatal("expected default sdlc to be seeded")
	}
	found := false
	for _, w := range sdlcs {
		if w.Name == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("default sdlc not found")
	}
}

func TestDefaultSdlcHasTwoStages(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)
	sdlcs, _ := ListSdlcs(context.Background(), db, 0, 0)
	var sdlcID int64
	for _, w := range sdlcs {
		if w.Name == "default" {
			sdlcID = w.ID
		}
	}
	wf, err := GetSdlc(context.Background(), db, sdlcID)
	if err != nil {
		t.Fatalf("GetSdlc() error = %v", err)
	}
	if len(wf.Stages) != 2 {
		t.Fatalf("default sdlc stages = %d, want 2", len(wf.Stages))
	}
	if wf.Stages[0].StageName != "develop" {
		t.Errorf("stage[0] = %q, want develop", wf.Stages[0].StageName)
	}
	if wf.Stages[1].StageName != "done" {
		t.Errorf("stage[1] = %q, want done", wf.Stages[1].StageName)
	}
}

func TestSdlcCRUD(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	wf, err := CreateSdlc(context.Background(), db, "custom", "A custom sdlc")
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	if wf.Name != "custom" {
		t.Fatalf("Name = %q, want %q", wf.Name, "custom")
	}

	// Add stages (no role_id — roles are via junction table now)
	s1, err := AddSdlcStage(context.Background(), db, wf.ID, "analysis", "Analyse requirements", "", 0)
	if err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}
	if s1.StageName != "analysis" {
		t.Fatalf("StageName = %q, want %q", s1.StageName, "analysis")
	}

	s2, err := AddSdlcStage(context.Background(), db, wf.ID, "build", "", "", 1)
	if err != nil {
		t.Fatalf("AddSdlcStage(build) error = %v", err)
	}

	// Get sdlc with stages
	got, err := GetSdlc(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("GetSdlc() error = %v", err)
	}
	if len(got.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(got.Stages))
	}

	// Remove stage
	if err := RemoveSdlcStage(context.Background(), db, s1.ID); err != nil {
		t.Fatalf("RemoveSdlcStage() error = %v", err)
	}
	got, _ = GetSdlc(context.Background(), db, wf.ID)
	if len(got.Stages) != 1 {
		t.Fatalf("stages after remove = %d, want 1", len(got.Stages))
	}
	if got.Stages[0].ID != s2.ID {
		t.Fatalf("remaining stage ID = %d, want %d", got.Stages[0].ID, s2.ID)
	}

	// Delete sdlc
	if err := DeleteSdlc(context.Background(), db, wf.ID); err != nil {
		t.Fatalf("DeleteSdlc() error = %v", err)
	}
	_, err = GetSdlc(context.Background(), db, wf.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestReorderSdlcStages(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)
	wf, _ := CreateSdlc(context.Background(), db, "reorder-test", "")
	s1, _ := AddSdlcStage(context.Background(), db, wf.ID, "first", "", "", 0)
	s2, _ := AddSdlcStage(context.Background(), db, wf.ID, "second", "", "", 1)
	s3, _ := AddSdlcStage(context.Background(), db, wf.ID, "third", "", "", 2)

	// Reverse order
	if err := ReorderSdlcStages(context.Background(), db, wf.ID, []int64{s3.ID, s2.ID, s1.ID}); err != nil {
		t.Fatalf("ReorderSdlcStages() error = %v", err)
	}
	got, _ := GetSdlc(context.Background(), db, wf.ID)
	if got.Stages[0].StageName != "third" {
		t.Fatalf("first stage = %q, want %q", got.Stages[0].StageName, "third")
	}
	if got.Stages[2].StageName != "first" {
		t.Fatalf("last stage = %q, want %q", got.Stages[2].StageName, "first")
	}
}

func TestUpdateGetAndListSdlcStage(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	wf, err := CreateSdlc(context.Background(), db, "stage-details", "")
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	stage, err := AddSdlcStage(context.Background(), db, wf.ID, "triage", "initial review", "", 3)
	if err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}

	stage, err = UpdateSdlcStage(context.Background(), db, stage.ID, "triage", "updated review", "Clarify the issue")
	if err != nil {
		t.Fatalf("UpdateSdlcStage() error = %v", err)
	}
	if stage.Description != "updated review" || stage.AcceptanceCriteria != "Clarify the issue" {
		t.Fatalf("UpdateSdlcStage() = %#v", stage)
	}

	got, err := GetSdlcStage(context.Background(), db, stage.ID)
	if err != nil {
		t.Fatalf("GetSdlcStage() error = %v", err)
	}
	if got.ID != stage.ID || got.Description != "updated review" {
		t.Fatalf("GetSdlcStage() = %#v, want updated stage", got)
	}

	stages, err := ListSdlcStages(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("ListSdlcStages() error = %v", err)
	}
	if len(stages) != 1 || stages[0].ID != stage.ID {
		t.Fatalf("ListSdlcStages() = %#v, want only stage %d", stages, stage.ID)
	}

	order, err := GetSdlcStageOrder(context.Background(), db, stage.ID)
	if err != nil {
		t.Fatalf("GetSdlcStageOrder() error = %v", err)
	}
	if order != 3 {
		t.Fatalf("GetSdlcStageOrder() = %d, want 3", order)
	}
}

func TestSdlcStageDefinitionsCRUD(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	wf, err := CreateSdlc(context.Background(), db, "stage-defs", "")
	if err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}

	stage, err := AddSdlcStageWithDefinitions(context.Background(), db, wf.ID, "develop", "ways", "ready", "done", 0)
	if err != nil {
		t.Fatalf("AddSdlcStageWithDefinitions() error = %v", err)
	}
	if stage.Description != "ways" || stage.AcceptanceCriteria != "ready" || stage.DefinitionOfReady != "ready" || stage.DefinitionOfDone != "done" {
		t.Fatalf("added stage = %#v", stage)
	}

	stage, err = UpdateSdlcStageWithDefinitions(context.Background(), db, stage.ID, "develop", "ways-2", "ready-2", "done-2")
	if err != nil {
		t.Fatalf("UpdateSdlcStageWithDefinitions() error = %v", err)
	}
	if stage.Description != "ways-2" || stage.AcceptanceCriteria != "ready-2" || stage.DefinitionOfReady != "ready-2" || stage.DefinitionOfDone != "done-2" {
		t.Fatalf("updated stage = %#v", stage)
	}
}

func TestSdlcExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	// Find the default sdlc
	sdlcs, _ := ListSdlcs(context.Background(), db, 0, 0)
	var defaultID int64
	for _, w := range sdlcs {
		if w.Name == "default" {
			defaultID = w.ID
		}
	}

	exported, err := ExportSdlc(context.Background(), db, defaultID)
	if err != nil {
		t.Fatalf("ExportSdlc() error = %v", err)
	}
	if exported.Name != "default" {
		t.Fatalf("exported.Name = %q, want %q", exported.Name, "default")
	}
	if len(exported.Stages) != 2 {
		t.Fatalf("exported stages = %d, want 2", len(exported.Stages))
	}

	// Import as a new sdlc with different name
	exported.Name = "imported-copy"
	imported, err := ImportSdlc(context.Background(), db, exported)
	if err != nil {
		t.Fatalf("ImportSdlc() error = %v", err)
	}
	if imported.Name != "imported-copy" {
		t.Fatalf("imported.Name = %q, want %q", imported.Name, "imported-copy")
	}

	// Verify stages match
	got, _ := GetSdlc(context.Background(), db, imported.ID)
	if len(got.Stages) != 2 {
		t.Fatalf("imported stages = %d, want 2", len(got.Stages))
	}
	for i, s := range got.Stages {
		if s.StageName != exported.Stages[i].StageName {
			t.Errorf("stage[%d] = %q, want %q", i, s.StageName, exported.Stages[i].StageName)
		}
	}
}

func TestSdlcStageRoles(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	sdlc, _ := CreateSdlc(context.Background(), db, "role-test", "")
	stage, _ := AddSdlcStage(context.Background(), db, sdlc.ID, "design", "", "", 0)

	r1, _ := CreateRole(context.Background(), db, &sdlc.ID, "Architect", "design role", "review architecture")
	r2, _ := CreateRole(context.Background(), db, &sdlc.ID, "BA", "analysis role", "gather requirements")

	// Add roles to stage
	if err := AddSdlcStageRole(context.Background(), db, sdlc.ID, stage.ID, r1.ID); err != nil {
		t.Fatalf("AddSdlcStageRole(r1) error = %v", err)
	}
	if err := AddSdlcStageRole(context.Background(), db, sdlc.ID, stage.ID, r2.ID); err != nil {
		t.Fatalf("AddSdlcStageRole(r2) error = %v", err)
	}

	// List roles
	roles, err := ListSdlcStageRoles(context.Background(), db, sdlc.ID, stage.ID)
	if err != nil {
		t.Fatalf("ListSdlcStageRoles() error = %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2", len(roles))
	}
	if roles[0].Title != "Architect" {
		t.Errorf("roles[0] = %q, want Architect", roles[0].Title)
	}

	// Reorder
	if err := ReorderSdlcStageRoles(context.Background(), db, sdlc.ID, stage.ID, []int64{r2.ID, r1.ID}); err != nil {
		t.Fatalf("ReorderSdlcStageRoles() error = %v", err)
	}
	roles, _ = ListSdlcStageRoles(context.Background(), db, sdlc.ID, stage.ID)
	if roles[0].Title != "BA" {
		t.Errorf("after reorder roles[0] = %q, want BA", roles[0].Title)
	}

	// Remove
	if err := RemoveSdlcStageRole(context.Background(), db, sdlc.ID, stage.ID, r1.ID); err != nil {
		t.Fatalf("RemoveSdlcStageRole() error = %v", err)
	}
	roles, _ = ListSdlcStageRoles(context.Background(), db, sdlc.ID, stage.ID)
	if len(roles) != 1 {
		t.Fatalf("roles after remove = %d, want 1", len(roles))
	}

	// Verify stage loads roles via GetSdlc
	got, _ := GetSdlc(context.Background(), db, sdlc.ID)
	if len(got.Stages[0].Roles) != 1 {
		t.Fatalf("stage.Roles = %d, want 1", len(got.Stages[0].Roles))
	}
	if got.Stages[0].Roles[0].Title != "BA" {
		t.Errorf("stage.Roles[0] = %q, want BA", got.Stages[0].Roles[0].Title)
	}
}

func TestListSdlcsPagination(t *testing.T) {
	t.Parallel()
	db := setupSdlcTestDB(t)

	for _, name := range []string{"alpha flow", "beta flow", "gamma flow"} {
		if _, err := CreateSdlc(context.Background(), db, name, ""); err != nil {
			t.Fatalf("CreateSdlc(%q) error = %v", name, err)
		}
	}

	sdlcs, err := ListSdlcs(context.Background(), db, 2, 1)
	if err != nil {
		t.Fatalf("ListSdlcs(limit, offset) error = %v", err)
	}
	if len(sdlcs) != 2 {
		t.Fatalf("ListSdlcs(limit, offset) len = %d, want 2", len(sdlcs))
	}

	if _, err := ListSdlcs(context.Background(), db, -1, 0); err == nil {
		t.Fatal("ListSdlcs(negative limit) error = nil, want error")
	}
	if _, err := ListSdlcs(context.Background(), db, 1, -1); err == nil {
		t.Fatal("ListSdlcs(negative offset) error = nil, want error")
	}
}
