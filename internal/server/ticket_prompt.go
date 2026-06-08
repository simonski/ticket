package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

// buildTicketWorkPrompt assembles the prompt an agent would receive to work a
// ticket — role, project, workflow, the ticket itself, and its parent chain.
// It mirrors the CLI agent's buildAgentPrompt so the web UI can preview exactly
// what an agent sees. Best-effort: missing related entities are simply omitted.
func buildTicketWorkPrompt(ctx context.Context, db *sql.DB, ticket store.Ticket) string {
	var b strings.Builder

	if ticket.RoleID != nil {
		if role, err := store.GetRoleByID(ctx, db, *ticket.RoleID); err == nil {
			b.WriteString(fmt.Sprintf("Role: %s\n", role.Title))
			if strings.TrimSpace(role.Description) != "" {
				b.WriteString(fmt.Sprintf("Description: %s\n", strings.TrimSpace(role.Description)))
			}
			if strings.TrimSpace(role.AcceptanceCriteria) != "" {
				b.WriteString(fmt.Sprintf("AcceptanceCriteria: %s\n", strings.TrimSpace(role.AcceptanceCriteria)))
			}
			b.WriteString("\n")
		}
	}

	if project, err := store.GetProjectByID(ctx, db, ticket.ProjectID); err == nil {
		b.WriteString(fmt.Sprintf("Project: %s — %s\n", project.Prefix, project.Title))
		if strings.TrimSpace(project.GitRepository) != "" {
			b.WriteString(fmt.Sprintf("Repository: %s\n", strings.TrimSpace(project.GitRepository)))
		}
		b.WriteString("\n")
	}

	if wfID := store.ResolveWorkflowID(ctx, db, ticket); wfID != nil {
		if wf, err := store.GetWorkflow(ctx, db, *wfID); err == nil {
			b.WriteString(fmt.Sprintf("Workflow: %s\n", wf.Name))
			b.WriteString("Stages:")
			for _, stage := range wf.Stages {
				marker := " "
				if ticket.WorkflowStageID != nil && stage.ID == *ticket.WorkflowStageID {
					marker = ">"
				}
				b.WriteString(fmt.Sprintf(" %s %s", marker, stage.StageName))
			}
			b.WriteString("\n\n")
		}
	}

	b.WriteString(fmt.Sprintf("Ticket: %s [%s]\n", ticket.ID, ticket.Type))
	b.WriteString(fmt.Sprintf("Title: %s\n", strings.TrimSpace(ticket.Title)))
	if strings.TrimSpace(ticket.Description) != "" {
		b.WriteString("Description:\n")
		b.WriteString(strings.TrimSpace(ticket.Description))
		b.WriteString("\n")
	}
	if strings.TrimSpace(ticket.AcceptanceCriteria) != "" {
		b.WriteString("Acceptance Criteria:\n")
		b.WriteString(strings.TrimSpace(ticket.AcceptanceCriteria))
		b.WriteString("\n")
	}

	// Walk the parent chain so the agent has hierarchical context.
	parents := make([]string, 0)
	parentID := ticket.ParentID
	for parentID != nil {
		parent, err := store.GetTicket(ctx, db, *parentID)
		if err != nil {
			break
		}
		parents = append(parents, fmt.Sprintf("  %s [%s] %s", parent.ID, parent.Type, strings.TrimSpace(parent.Title)))
		parentID = parent.ParentID
	}
	if len(parents) > 0 {
		b.WriteString("\nParents:\n")
		b.WriteString(strings.Join(parents, "\n"))
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}
