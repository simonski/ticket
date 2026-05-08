package store

import (
	"context"
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
	t.Parallel()
	db := setupWorkflowTestDB(t)
	workflows, err := ListWorkflows(context.Background(), db, 0, 0)
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

func TestDefaultWorkflowHasTwoStages(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)
	workflows, _ := ListWorkflows(context.Background(), db, 0, 0)
	var workflowID int64
	for _, w := range workflows {
		if w.Name == "default" {
			workflowID = w.ID
		}
	}
	wf, err := GetWorkflow(context.Background(), db, workflowID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(wf.Stages) != 2 {
		t.Fatalf("default workflow stages = %d, want 2", len(wf.Stages))
	}
	if wf.Stages[0].StageName != "develop" {
		t.Errorf("stage[0] = %q, want develop", wf.Stages[0].StageName)
	}
	if wf.Stages[1].StageName != "done" {
		t.Errorf("stage[1] = %q, want done", wf.Stages[1].StageName)
	}
}

func TestWorkflowCRUD(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	wf, err := CreateWorkflow(context.Background(), db, "custom", "A custom workflow")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if wf.Name != "custom" {
		t.Fatalf("Name = %q, want %q", wf.Name, "custom")
	}

	// Add stages (no role_id — roles are via junction table now)
	s1, err := AddWorkflowStage(context.Background(), db, wf.ID, "analysis", "Analyse requirements", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if s1.StageName != "analysis" {
		t.Fatalf("StageName = %q, want %q", s1.StageName, "analysis")
	}

	s2, err := AddWorkflowStage(context.Background(), db, wf.ID, "build", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(build) error = %v", err)
	}

	// Get workflow with stages
	got, err := GetWorkflow(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(got.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(got.Stages))
	}

	// Remove stage
	if err := RemoveWorkflowStage(context.Background(), db, s1.ID); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	got, _ = GetWorkflow(context.Background(), db, wf.ID)
	if len(got.Stages) != 1 {
		t.Fatalf("stages after remove = %d, want 1", len(got.Stages))
	}
	if got.Stages[0].ID != s2.ID {
		t.Fatalf("remaining stage ID = %d, want %d", got.Stages[0].ID, s2.ID)
	}

	// Delete workflow
	if err := DeleteWorkflow(context.Background(), db, wf.ID); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
	_, err = GetWorkflow(context.Background(), db, wf.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestWorkflowPolicyAndProgressionCRUD(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)
	ctx := context.Background()

	wf, err := CreateWorkflowWithOptions(ctx, db, nil, "policy-flow", "policy coverage", WorkflowApprovalPolicyAllRoles, WorkflowProgressionModeStageOnly)
	if err != nil {
		t.Fatalf("CreateWorkflowWithOptions() error = %v", err)
	}
	if wf.ApprovalPolicy != WorkflowApprovalPolicyAllRoles {
		t.Fatalf("ApprovalPolicy = %q, want %q", wf.ApprovalPolicy, WorkflowApprovalPolicyAllRoles)
	}
	if wf.ProgressionMode != WorkflowProgressionModeStageOnly {
		t.Fatalf("ProgressionMode = %q, want %q", wf.ProgressionMode, WorkflowProgressionModeStageOnly)
	}

	updated, err := UpdateWorkflow(ctx, db, wf.ID, "policy-flow-updated", "updated", WorkflowApprovalPolicySingleRole, WorkflowProgressionModeLinear)
	if err != nil {
		t.Fatalf("UpdateWorkflow() error = %v", err)
	}
	if updated.ApprovalPolicy != WorkflowApprovalPolicySingleRole {
		t.Fatalf("updated ApprovalPolicy = %q, want %q", updated.ApprovalPolicy, WorkflowApprovalPolicySingleRole)
	}
	if updated.ProgressionMode != WorkflowProgressionModeLinear {
		t.Fatalf("updated ProgressionMode = %q, want %q", updated.ProgressionMode, WorkflowProgressionModeLinear)
	}

	if _, err := CreateWorkflowWithOptions(ctx, db, nil, "bad-policy", "", "invalid", WorkflowProgressionModeLinear); err == nil {
		t.Fatal("CreateWorkflowWithOptions(invalid policy) error = nil, want error")
	}
	if _, err := UpdateWorkflow(ctx, db, wf.ID, "bad-progression", "", WorkflowApprovalPolicySingleRole, "invalid"); err == nil {
		t.Fatal("UpdateWorkflow(invalid progression) error = nil, want error")
	}
}

func TestReorderWorkflowStages(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)
	wf, _ := CreateWorkflow(context.Background(), db, "reorder-test", "")
	s1, _ := AddWorkflowStage(context.Background(), db, wf.ID, "first", "", "", 0)
	s2, _ := AddWorkflowStage(context.Background(), db, wf.ID, "second", "", "", 1)
	s3, _ := AddWorkflowStage(context.Background(), db, wf.ID, "third", "", "", 2)

	// Reverse order
	if err := ReorderWorkflowStages(context.Background(), db, wf.ID, []int64{s3.ID, s2.ID, s1.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	got, _ := GetWorkflow(context.Background(), db, wf.ID)
	if got.Stages[0].StageName != "third" {
		t.Fatalf("first stage = %q, want %q", got.Stages[0].StageName, "third")
	}
	if got.Stages[2].StageName != "first" {
		t.Fatalf("last stage = %q, want %q", got.Stages[2].StageName, "first")
	}
}

func TestUpdateGetAndListWorkflowStage(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	wf, err := CreateWorkflow(context.Background(), db, "stage-details", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStage(context.Background(), db, wf.ID, "triage", "initial review", "", 3)
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}

	stage, err = UpdateWorkflowStage(context.Background(), db, stage.ID, "triage", "updated review", "Clarify the issue")
	if err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	if stage.Description != "updated review" || stage.AcceptanceCriteria != "Clarify the issue" {
		t.Fatalf("UpdateWorkflowStage() = %#v", stage)
	}

	got, err := GetWorkflowStage(context.Background(), db, stage.ID)
	if err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if got.ID != stage.ID || got.Description != "updated review" {
		t.Fatalf("GetWorkflowStage() = %#v, want updated stage", got)
	}

	stages, err := ListWorkflowStages(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStages() error = %v", err)
	}
	if len(stages) != 1 || stages[0].ID != stage.ID {
		t.Fatalf("ListWorkflowStages() = %#v, want only stage %d", stages, stage.ID)
	}

	order, err := GetWorkflowStageOrder(context.Background(), db, stage.ID)
	if err != nil {
		t.Fatalf("GetWorkflowStageOrder() error = %v", err)
	}
	if order != 3 {
		t.Fatalf("GetWorkflowStageOrder() = %d, want 3", order)
	}
}

func TestWorkflowStageDefinitionsCRUD(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	wf, err := CreateWorkflow(context.Background(), db, "stage-defs", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	stage, err := AddWorkflowStageWithDefinitions(context.Background(), db, wf.ID, "develop", "ways", "ready", "done", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStageWithDefinitions() error = %v", err)
	}
	if stage.Description != "ways" || stage.AcceptanceCriteria != "ready" || stage.DefinitionOfReady != "ready" || stage.DefinitionOfDone != "done" {
		t.Fatalf("added stage = %#v", stage)
	}

	stage, err = UpdateWorkflowStageWithDefinitions(context.Background(), db, stage.ID, "develop", "ways-2", "ready-2", "done-2")
	if err != nil {
		t.Fatalf("UpdateWorkflowStageWithDefinitions() error = %v", err)
	}
	if stage.Description != "ways-2" || stage.AcceptanceCriteria != "ready-2" || stage.DefinitionOfReady != "ready-2" || stage.DefinitionOfDone != "done-2" {
		t.Fatalf("updated stage = %#v", stage)
	}
}

func TestWorkflowStageTransitionsCRUD(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)
	ctx := context.Background()

	wf, err := CreateWorkflow(ctx, db, "dag", "dag workflow")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	design, err := AddWorkflowStage(ctx, db, wf.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := AddWorkflowStage(ctx, db, wf.ID, "test", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	done, err := AddWorkflowStage(ctx, db, wf.ID, "done", "", "", 2)
	if err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	if err := SetWorkflowStageTransitions(ctx, db, wf.ID, design.ID, []int64{done.ID, testStage.ID}); err != nil {
		t.Fatalf("SetWorkflowStageTransitions() error = %v", err)
	}
	transitions, err := ListWorkflowStageTransitions(ctx, db, wf.ID, &design.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStageTransitions() error = %v", err)
	}
	if len(transitions) != 2 || transitions[0].ToStageID != done.ID || transitions[1].ToStageID != testStage.ID {
		t.Fatalf("transitions = %#v, want [done,test]", transitions)
	}
	stage, err := GetWorkflowStage(ctx, db, design.ID)
	if err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if len(stage.NextStageIDs) != 2 || stage.NextStageIDs[0] != done.ID {
		t.Fatalf("stage.NextStageIDs = %#v, want [%d,%d]", stage.NextStageIDs, done.ID, testStage.ID)
	}
}

func TestWorkflowExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	// Find the default workflow
	workflows, _ := ListWorkflows(context.Background(), db, 0, 0)
	var defaultID int64
	for _, w := range workflows {
		if w.Name == "default" {
			defaultID = w.ID
		}
	}

	exported, err := ExportWorkflow(context.Background(), db, defaultID)
	if err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	if exported.Name != "default" {
		t.Fatalf("exported.Name = %q, want %q", exported.Name, "default")
	}
	if len(exported.Stages) != 2 {
		t.Fatalf("exported stages = %d, want 2", len(exported.Stages))
	}

	// Import as a new workflow with different name
	exported.Name = "imported-copy"
	imported, err := ImportWorkflow(context.Background(), db, exported)
	if err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}
	if imported.Name != "imported-copy" {
		t.Fatalf("imported.Name = %q, want %q", imported.Name, "imported-copy")
	}

	// Verify stages match
	got, _ := GetWorkflow(context.Background(), db, imported.ID)
	if len(got.Stages) != 2 {
		t.Fatalf("imported stages = %d, want 2", len(got.Stages))
	}
	for i, s := range got.Stages {
		if s.StageName != exported.Stages[i].StageName {
			t.Errorf("stage[%d] = %q, want %q", i, s.StageName, exported.Stages[i].StageName)
		}
	}
}

func TestWorkflowStageRoles(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	workflow, _ := CreateWorkflow(context.Background(), db, "role-test", "")
	stage, _ := AddWorkflowStage(context.Background(), db, workflow.ID, "design", "", "", 0)

	r1, _ := CreateRole(context.Background(), db, &workflow.ID, "Architect", "design role", "review architecture")
	r2, _ := CreateRole(context.Background(), db, &workflow.ID, "BA", "analysis role", "gather requirements")

	// Add roles to stage
	if err := AddWorkflowStageRole(context.Background(), db, workflow.ID, stage.ID, r1.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(r1) error = %v", err)
	}
	if err := AddWorkflowStageRole(context.Background(), db, workflow.ID, stage.ID, r2.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(r2) error = %v", err)
	}

	// List roles
	roles, err := ListWorkflowStageRoles(context.Background(), db, workflow.ID, stage.ID)
	if err != nil {
		t.Fatalf("ListWorkflowStageRoles() error = %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2", len(roles))
	}
	if roles[0].Title != "Architect" {
		t.Errorf("roles[0] = %q, want Architect", roles[0].Title)
	}

	// Reorder
	if err := ReorderWorkflowStageRoles(context.Background(), db, workflow.ID, stage.ID, []int64{r2.ID, r1.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStageRoles() error = %v", err)
	}
	roles, _ = ListWorkflowStageRoles(context.Background(), db, workflow.ID, stage.ID)
	if roles[0].Title != "BA" {
		t.Errorf("after reorder roles[0] = %q, want BA", roles[0].Title)
	}

	// Remove
	if err := RemoveWorkflowStageRole(context.Background(), db, workflow.ID, stage.ID, r1.ID); err != nil {
		t.Fatalf("RemoveWorkflowStageRole() error = %v", err)
	}
	roles, _ = ListWorkflowStageRoles(context.Background(), db, workflow.ID, stage.ID)
	if len(roles) != 1 {
		t.Fatalf("roles after remove = %d, want 1", len(roles))
	}

	// Verify stage loads roles via GetWorkflow
	got, _ := GetWorkflow(context.Background(), db, workflow.ID)
	if len(got.Stages[0].Roles) != 1 {
		t.Fatalf("stage.Roles = %d, want 1", len(got.Stages[0].Roles))
	}
	if got.Stages[0].Roles[0].Title != "BA" {
		t.Errorf("stage.Roles[0] = %q, want BA", got.Stages[0].Roles[0].Title)
	}
}

func TestListWorkflowsPagination(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)

	for _, name := range []string{"alpha flow", "beta flow", "gamma flow"} {
		if _, err := CreateWorkflow(context.Background(), db, name, ""); err != nil {
			t.Fatalf("CreateWorkflow(%q) error = %v", name, err)
		}
	}

	workflows, err := ListWorkflows(context.Background(), db, 2, 1)
	if err != nil {
		t.Fatalf("ListWorkflows(limit, offset) error = %v", err)
	}
	if len(workflows) != 2 {
		t.Fatalf("ListWorkflows(limit, offset) len = %d, want 2", len(workflows))
	}

	if _, err := ListWorkflows(context.Background(), db, -1, 0); err == nil {
		t.Fatal("ListWorkflows(negative limit) error = nil, want error")
	}
	if _, err := ListWorkflows(context.Background(), db, 1, -1); err == nil {
		t.Fatal("ListWorkflows(negative offset) error = nil, want error")
	}
}
