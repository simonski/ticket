package store

import (
	"context"
	"database/sql"
	"strings"
)

// ExecutionPacket captures the resolved runtime context that drives agent work
// for a ticket: ticket + role + phase + project rules, and the effective merge.
type ExecutionPacket struct {
	TicketID        string                `json:"ticket_id"`
	ProjectID       int64                 `json:"project_id"`
	Phase           string                `json:"phase"`
	WorkflowID      *int64                `json:"workflow_id,omitempty"`
	WorkflowStageID *int64                `json:"workflow_stage_id,omitempty"`
	RoleID          *int64                `json:"role_id,omitempty"`
	RoleTitle       string                `json:"role_title,omitempty"`
	Layers          ExecutionPacketLayers `json:"layers"`
	Effective       ResolvedGuidance      `json:"effective"`
}

type ExecutionPacketLayers struct {
	Project *ResolvedGuidance `json:"project,omitempty"`
	Phase   *ResolvedGuidance `json:"phase,omitempty"`
	Role    *ResolvedGuidance `json:"role,omitempty"`
	Ticket  *ResolvedGuidance `json:"ticket,omitempty"`
}

// BuildExecutionPacket resolves and merges guidance with precedence:
// project < phase < role < ticket.
func BuildExecutionPacket(ctx context.Context, db *sql.DB, ticketID string) (ExecutionPacket, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return ExecutionPacket{}, err
	}
	enriched := EnrichTicketContext(ctx, db, ticket)
	phase := strings.TrimSpace(ticket.Stage)

	projectLayer := guidancePtr(layerFromProject(enriched.Project, phase))
	phaseLayer := guidancePtr(layerFromWorkflowStage(ticket, enriched.Workflow))
	roleLayer := guidancePtr(layerFromRole(enriched.Role, phase))
	ticketLayer := guidancePtr(ticket.ResolveGuidance(phase))
	effective := mergeGuidanceLayers(
		projectLayer,
		phaseLayer,
		roleLayer,
		ticketLayer,
	)

	packet := ExecutionPacket{
		TicketID:        ticket.ID,
		ProjectID:       ticket.ProjectID,
		Phase:           phase,
		WorkflowID:      ResolveWorkflowID(ctx, db, ticket),
		WorkflowStageID: ticket.WorkflowStageID,
		RoleID:          ticket.RoleID,
		Layers: ExecutionPacketLayers{
			Project: projectLayer,
			Phase:   phaseLayer,
			Role:    roleLayer,
			Ticket:  ticketLayer,
		},
		Effective: effective,
	}
	if enriched.Role != nil {
		packet.RoleTitle = enriched.Role.Title
	}
	return packet, nil
}

func layerFromProject(project *Project, phase string) ResolvedGuidance {
	if project == nil {
		return ResolvedGuidance{}
	}
	return project.ResolveGuidance(phase)
}

func layerFromRole(role *Role, phase string) ResolvedGuidance {
	if role == nil {
		return ResolvedGuidance{}
	}
	return role.ResolveGuidance(phase)
}

func layerFromWorkflowStage(ticket Ticket, workflow *WorkflowWithStages) ResolvedGuidance {
	if workflow == nil {
		return ResolvedGuidance{}
	}
	stage := currentWorkflowStage(ticket, workflow)
	if stage == nil {
		return ResolvedGuidance{}
	}
	resolved := ResolvedGuidance{}
	if dor := strings.TrimSpace(stage.DefinitionOfReady); dor != "" {
		resolved.DOR = dor
		resolved.HasDOR = true
	}
	if dod := strings.TrimSpace(stage.DefinitionOfDone); dod != "" {
		resolved.DOD = dod
		resolved.HasDOD = true
	}
	if ac := strings.TrimSpace(stage.AcceptanceCriteria); ac != "" {
		resolved.AC = ac
		resolved.HasAC = true
	}
	return resolved
}

func currentWorkflowStage(ticket Ticket, workflow *WorkflowWithStages) *WorkflowStage {
	if workflow == nil {
		return nil
	}
	if ticket.WorkflowStageID != nil {
		for i := range workflow.Stages {
			if workflow.Stages[i].ID == *ticket.WorkflowStageID {
				return &workflow.Stages[i]
			}
		}
	}
	needle := strings.TrimSpace(strings.ToLower(ticket.Stage))
	if needle == "" {
		return nil
	}
	for i := range workflow.Stages {
		if strings.TrimSpace(strings.ToLower(workflow.Stages[i].StageName)) == needle {
			return &workflow.Stages[i]
		}
	}
	return nil
}

func guidancePtr(r ResolvedGuidance) *ResolvedGuidance {
	if !r.HasDOR && !r.HasDOD && !r.HasAC {
		return nil
	}
	guidance := ResolvedGuidance{
		DOR: r.DOR,
		DOD: r.DOD,
		AC:  r.AC,
	}
	return &guidance
}

func mergeGuidanceLayers(layers ...*ResolvedGuidance) ResolvedGuidance {
	merged := ResolvedGuidance{}
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		if dor := strings.TrimSpace(layer.DOR); dor != "" {
			merged.DOR = dor
			merged.HasDOR = true
		}
		if dod := strings.TrimSpace(layer.DOD); dod != "" {
			merged.DOD = dod
			merged.HasDOD = true
		}
		if ac := strings.TrimSpace(layer.AC); ac != "" {
			merged.AC = ac
			merged.HasAC = true
		}
	}
	return merged
}
