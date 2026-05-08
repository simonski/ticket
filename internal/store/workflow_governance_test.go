package store

import (
	"context"
	"testing"
)

func TestWorkflowGovernanceVersionLifecycle(t *testing.T) {
	t.Parallel()
	db := setupWorkflowTestDB(t)
	ctx := context.Background()
	workflow, err := CreateWorkflow(ctx, db, "Governance", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := AddWorkflowStage(ctx, db, workflow.ID, "design", "", "", 0); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	v1, err := SaveWorkflowVersion(ctx, db, workflow.ID, "initial snapshot", "admin")
	if err != nil {
		t.Fatalf("SaveWorkflowVersion(v1) error = %v", err)
	}
	if !v1.Active || !v1.Approved {
		t.Fatalf("v1 expected active+approved, got %#v", v1)
	}
	if _, err := AddWorkflowStage(ctx, db, workflow.ID, "done", "", "", 1); err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	v2, err := SaveWorkflowVersion(ctx, db, workflow.ID, "added done", "admin")
	if err != nil {
		t.Fatalf("SaveWorkflowVersion(v2) error = %v", err)
	}
	if v2.VersionNumber <= v1.VersionNumber {
		t.Fatalf("expected incremented version number: v1=%#v v2=%#v", v1, v2)
	}
	versions, err := ListWorkflowVersions(ctx, db, workflow.ID, 10)
	if err != nil {
		t.Fatalf("ListWorkflowVersions() error = %v", err)
	}
	if len(versions) < 2 {
		t.Fatalf("expected at least 2 versions, got %#v", versions)
	}
	if _, err := ActivateWorkflowVersion(ctx, db, workflow.ID, v2.VersionID); err == nil {
		t.Fatalf("expected activate unapproved version to fail")
	}
	if _, err := ApproveWorkflowVersion(ctx, db, workflow.ID, v2.VersionID, "admin"); err != nil {
		t.Fatalf("ApproveWorkflowVersion() error = %v", err)
	}
	active, err := ActivateWorkflowVersion(ctx, db, workflow.ID, v2.VersionID)
	if err != nil {
		t.Fatalf("ActivateWorkflowVersion() error = %v", err)
	}
	if !active.Active {
		t.Fatalf("expected active version, got %#v", active)
	}
}
