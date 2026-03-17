package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func setupWorkflowTestDB(t *testing.T) *sql.DB {
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

func TestInitSeedsDefaultWorkflow(t *testing.T) {
	db := setupWorkflowTestDB(t)
	workflows, err := ListWorkflows(db)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) == 0 {
		t.Fatal("expected default workflow to be seeded")
	}
	found := false
	for _, w := range workflows {
		if w.Name == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("default workflow not found")
	}
}

func TestDefaultWorkflowHasFourStages(t *testing.T) {
	db := setupWorkflowTestDB(t)
	workflows, _ := ListWorkflows(db)
	var wfID int64
	for _, w := range workflows {
		if w.Name == "default" {
			wfID = w.ID
		}
	}
	wf, err := GetWorkflow(db, wfID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(wf.Stages) != 4 {
		t.Fatalf("default workflow stages = %d, want 4", len(wf.Stages))
	}
	expected := []struct {
		name string
		role string
	}{
		{"design", "BA"},
		{"develop", "Lead Engineer"},
		{"test", "QA/Tester"},
		{"done", "Product Owner"},
	}
	for i, want := range expected {
		got := wf.Stages[i]
		if got.StageName != want.name {
			t.Errorf("stage[%d].StageName = %q, want %q", i, got.StageName, want.name)
		}
		if got.RoleTitle != want.role {
			t.Errorf("stage[%d].RoleTitle = %q, want %q", i, got.RoleTitle, want.role)
		}
		if got.SortOrder != i {
			t.Errorf("stage[%d].SortOrder = %d, want %d", i, got.SortOrder, i)
		}
	}
}

func TestWorkflowCRUD(t *testing.T) {
	db := setupWorkflowTestDB(t)

	wf, err := CreateWorkflow(db, "custom", "A custom workflow")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if wf.Name != "custom" {
		t.Fatalf("Name = %q, want %q", wf.Name, "custom")
	}

	// Add stages
	role, _ := GetRoleByTitle(db, "BA")
	s1, err := AddWorkflowStage(db, wf.ID, "analysis", "Analyse requirements", &role.ID, 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if s1.StageName != "analysis" {
		t.Fatalf("StageName = %q, want %q", s1.StageName, "analysis")
	}
	if s1.RoleTitle != "BA" {
		t.Fatalf("RoleTitle = %q, want %q", s1.RoleTitle, "BA")
	}

	s2, err := AddWorkflowStage(db, wf.ID, "build", "", nil, 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(build) error = %v", err)
	}

	// Get workflow with stages
	got, err := GetWorkflow(db, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(got.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(got.Stages))
	}

	// Remove stage
	if err := RemoveWorkflowStage(db, s1.ID); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	got, _ = GetWorkflow(db, wf.ID)
	if len(got.Stages) != 1 {
		t.Fatalf("stages after remove = %d, want 1", len(got.Stages))
	}
	if got.Stages[0].ID != s2.ID {
		t.Fatalf("remaining stage ID = %d, want %d", got.Stages[0].ID, s2.ID)
	}

	// Delete workflow
	if err := DeleteWorkflow(db, wf.ID); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
	_, err = GetWorkflow(db, wf.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestReorderWorkflowStages(t *testing.T) {
	db := setupWorkflowTestDB(t)
	wf, _ := CreateWorkflow(db, "reorder-test", "")
	s1, _ := AddWorkflowStage(db, wf.ID, "first", "", nil, 0)
	s2, _ := AddWorkflowStage(db, wf.ID, "second", "", nil, 1)
	s3, _ := AddWorkflowStage(db, wf.ID, "third", "", nil, 2)

	// Reverse order
	if err := ReorderWorkflowStages(db, wf.ID, []int64{s3.ID, s2.ID, s1.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	got, _ := GetWorkflow(db, wf.ID)
	if got.Stages[0].StageName != "third" {
		t.Fatalf("first stage = %q, want %q", got.Stages[0].StageName, "third")
	}
	if got.Stages[2].StageName != "first" {
		t.Fatalf("last stage = %q, want %q", got.Stages[2].StageName, "first")
	}
}

func TestWorkflowExportImportRoundTrip(t *testing.T) {
	db := setupWorkflowTestDB(t)

	// Find the default workflow
	workflows, _ := ListWorkflows(db)
	var defaultID int64
	for _, w := range workflows {
		if w.Name == "default" {
			defaultID = w.ID
		}
	}

	exported, err := ExportWorkflow(db, defaultID)
	if err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	if exported.Name != "default" {
		t.Fatalf("exported.Name = %q, want %q", exported.Name, "default")
	}
	if len(exported.Stages) != 4 {
		t.Fatalf("exported stages = %d, want 4", len(exported.Stages))
	}

	// Import as a new workflow with different name
	exported.Name = "imported-copy"
	imported, err := ImportWorkflow(db, exported)
	if err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}
	if imported.Name != "imported-copy" {
		t.Fatalf("imported.Name = %q, want %q", imported.Name, "imported-copy")
	}

	// Verify stages match
	got, _ := GetWorkflow(db, imported.ID)
	if len(got.Stages) != 4 {
		t.Fatalf("imported stages = %d, want 4", len(got.Stages))
	}
	for i, s := range got.Stages {
		if s.StageName != exported.Stages[i].StageName {
			t.Errorf("stage[%d] = %q, want %q", i, s.StageName, exported.Stages[i].StageName)
		}
		if s.RoleTitle != exported.Stages[i].Role {
			t.Errorf("stage[%d] role = %q, want %q", i, s.RoleTitle, exported.Stages[i].Role)
		}
	}
}
