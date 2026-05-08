package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type ProjectForecastEntry struct {
	TicketID          string `json:"ticket_id"`
	Key               string `json:"key"`
	Detail            string `json:"detail"`
	ConfidencePercent int    `json:"confidence_percent"`
}

func BuildProjectForecast(ctx context.Context, db *sql.DB, projectID int64, limit int) ([]ProjectForecastEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	tickets, err := ListTickets(ctx, db, TicketListParams{
		ProjectID: projectID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	workflowByID := map[int64]WorkflowWithStages{}
	forecast := make([]ProjectForecastEntry, 0, len(tickets))
	for _, ticket := range tickets {
		if ticket.Archived || ticket.Complete || ticket.Deleted {
			continue
		}
		key := strings.TrimSpace(ticket.ID)
		if key == "" {
			key = fmt.Sprintf("%d", ticket.ProjectID)
		}
		if strings.EqualFold(strings.TrimSpace(ticket.State), StateFail) {
			forecast = append(forecast, ProjectForecastEntry{
				TicketID:          ticket.ID,
				Key:               key,
				Detail:            "requires intervention triage",
				ConfidencePercent: 35,
			})
			continue
		}
		deps, depErr := ListDependencies(ctx, db, ticket.ID)
		if depErr != nil {
			return nil, depErr
		}
		blockedBy := make([]string, 0)
		for _, dep := range deps {
			depTicket, depTicketErr := GetTicket(ctx, db, dep.DependsOn)
			if depTicketErr != nil {
				return nil, depTicketErr
			}
			if !depTicket.Complete {
				blockedBy = append(blockedBy, dep.DependsOn)
			}
		}
		if len(blockedBy) > 0 {
			forecast = append(forecast, ProjectForecastEntry{
				TicketID:          ticket.ID,
				Key:               key,
				Detail:            "blocked by: " + strings.Join(blockedBy, ", "),
				ConfidencePercent: 80,
			})
			continue
		}
		if ticket.WorkflowID == nil || *ticket.WorkflowID == 0 {
			forecast = append(forecast, ProjectForecastEntry{
				TicketID:          ticket.ID,
				Key:               key,
				Detail:            "next: completion/close decision",
				ConfidencePercent: 55,
			})
			continue
		}
		workflow, ok := workflowByID[*ticket.WorkflowID]
		if !ok {
			wf, getErr := GetWorkflow(ctx, db, *ticket.WorkflowID)
			if getErr != nil {
				return nil, getErr
			}
			workflow = wf
			workflowByID[*ticket.WorkflowID] = workflow
		}
		stageIndex := -1
		for idx, stage := range workflow.Stages {
			if ticket.WorkflowStageID != nil && stage.ID == *ticket.WorkflowStageID {
				stageIndex = idx
				break
			}
			if stageIndex == -1 && strings.EqualFold(stage.StageName, strings.TrimSpace(ticket.Stage)) {
				stageIndex = idx
			}
		}
		if stageIndex == -1 {
			forecast = append(forecast, ProjectForecastEntry{
				TicketID:          ticket.ID,
				Key:               key,
				Detail:            "next: completion/close decision",
				ConfidencePercent: 50,
			})
			continue
		}
		currentStage := workflow.Stages[stageIndex]
		roleIndex := -1
		for idx, role := range currentStage.Roles {
			if ticket.RoleID != nil && role.ID == *ticket.RoleID {
				roleIndex = idx
				break
			}
		}
		if strings.EqualFold(strings.TrimSpace(ticket.State), StateSuccess) {
			if roleIndex >= 0 && roleIndex+1 < len(currentStage.Roles) {
				nextRole := currentStage.Roles[roleIndex+1]
				forecast = append(forecast, ProjectForecastEntry{
					TicketID:          ticket.ID,
					Key:               key,
					Detail:            "next: " + currentStage.StageName + " / " + nextRole.Title,
					ConfidencePercent: 75,
				})
				continue
			}
			nextStageID := int64(0)
			if len(currentStage.NextStageIDs) > 0 {
				nextStageID = currentStage.NextStageIDs[0]
			} else if stageIndex+1 < len(workflow.Stages) {
				nextStageID = workflow.Stages[stageIndex+1].ID
			}
			if nextStageID == 0 {
				forecast = append(forecast, ProjectForecastEntry{
					TicketID:          ticket.ID,
					Key:               key,
					Detail:            "next: completion/close decision",
					ConfidencePercent: 90,
				})
				continue
			}
			var nextStage WorkflowStage
			for _, candidate := range workflow.Stages {
				if candidate.ID == nextStageID {
					nextStage = candidate
					break
				}
			}
			nextRole := ""
			if len(nextStage.Roles) > 0 {
				nextRole = nextStage.Roles[0].Title
			}
			detail := "next: " + nextStage.StageName
			if nextRole != "" {
				detail += " / " + nextRole
			}
			forecast = append(forecast, ProjectForecastEntry{
				TicketID:          ticket.ID,
				Key:               key,
				Detail:            detail,
				ConfidencePercent: 70,
			})
			continue
		}
		currentRole := ""
		if roleIndex >= 0 {
			currentRole = currentStage.Roles[roleIndex].Title
		}
		detail := "in progress: " + currentStage.StageName
		if currentRole != "" {
			detail += " / " + currentRole
		}
		forecast = append(forecast, ProjectForecastEntry{
			TicketID:          ticket.ID,
			Key:               key,
			Detail:            detail,
			ConfidencePercent: 65,
		})
	}
	return forecast, nil
}
