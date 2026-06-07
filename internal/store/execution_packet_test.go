package store

import (
	"context"
	"testing"
)

func TestBuildExecutionPacketMergesLayersByPrecedence(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	workflow, err := CreateWorkflow(ctx, db, "Packet Workflow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStageWithDefinitions(ctx, db, workflow.ID, "develop", "phase-wow", "phase-dor", "phase-dod", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStageWithDefinitions() error = %v", err)
	}
	role, err := CreateRoleWithParams(ctx, db, RoleCreateParams{
		WorkflowID: &workflow.ID,
		Title:      "Engineer",
		DORMap:     GuidanceMap{"develop": "role-dor"},
		DODMap:     GuidanceMap{"develop": "role-dod"},
		ACMap:      GuidanceMap{"develop": "role-ac"},
	})
	if err != nil {
		t.Fatalf("CreateRoleWithParams() error = %v", err)
	}
	if err := AddWorkflowStageRole(ctx, db, workflow.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}

	project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:     "PKT",
		Title:      "Packet Project",
		WorkflowID: &workflow.ID,
		DORMap:     GuidanceMap{"develop": "project-dor"},
		DODMap:     GuidanceMap{"develop": "project-dod"},
		ACMap:      GuidanceMap{"develop": "project-ac"},
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}

	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &workflow.ID,
		Type:       "task",
		Title:      "Implement packet",
		DORMap:     GuidanceMap{"develop": "ticket-dor"},
		ACMap:      GuidanceMap{"develop": "ticket-ac"},
		CreatedBy:  "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	packet, err := BuildExecutionPacket(ctx, db, ticket.ID)
	if err != nil {
		t.Fatalf("BuildExecutionPacket() error = %v", err)
	}
	if packet.Layers.Project == nil || packet.Layers.Phase == nil || packet.Layers.Role == nil || packet.Layers.Ticket == nil {
		t.Fatalf("expected all layers present, got %#v", packet.Layers)
	}
	if packet.RoleTitle != "Engineer" {
		t.Fatalf("packet.RoleTitle = %q, want Engineer", packet.RoleTitle)
	}
	if packet.Effective.DOR != "ticket-dor" {
		t.Fatalf("effective DOR = %q, want ticket-dor", packet.Effective.DOR)
	}
	if packet.Effective.DOD != "role-dod" {
		t.Fatalf("effective DOD = %q, want role-dod", packet.Effective.DOD)
	}
	if packet.Effective.AC != "ticket-ac" {
		t.Fatalf("effective AC = %q, want ticket-ac", packet.Effective.AC)
	}
}
