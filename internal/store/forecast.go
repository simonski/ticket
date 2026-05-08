package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type ProjectForecastEntry struct {
	TicketID          string `json:"ticket_id"`
	Key               string `json:"key"`
	Detail            string `json:"detail"`
	ConfidencePercent int    `json:"confidence_percent"`
}

type ForecastCalibrationBucket struct {
	Range        string  `json:"range"`
	Total        int     `json:"total"`
	Hits         int     `json:"hits"`
	AccuracyRate float64 `json:"accuracy_rate"`
}

type ProjectForecastCalibration struct {
	ProjectID    int                         `json:"project_id"`
	SampleCount  int                         `json:"sample_count"`
	HitCount     int                         `json:"hit_count"`
	AccuracyRate float64                     `json:"accuracy_rate"`
	Buckets      []ForecastCalibrationBucket `json:"buckets"`
}

type WorkItemQueueCandidate struct {
	TicketID  string `json:"ticket_id"`
	Title     string `json:"title"`
	Priority  int    `json:"priority"`
	Order     int    `json:"order"`
	Stage     string `json:"stage"`
	State     string `json:"state"`
	Assignee  string `json:"assignee"`
	Blocked   bool   `json:"blocked"`
	BlockedBy string `json:"blocked_by,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

func ensureForecastSnapshotTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS forecast_snapshots (
			snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			ticket_id TEXT NOT NULL,
			detail TEXT NOT NULL,
			confidence_percent INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_forecast_snapshots_project_ticket ON forecast_snapshots(project_id, ticket_id, created_at);
	`)
	return err
}

func BuildProjectForecast(ctx context.Context, db *sql.DB, projectID int64, limit int) ([]ProjectForecastEntry, error) {
	if err := ensureForecastSnapshotTable(ctx, db); err != nil {
		return nil, err
	}
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
	for _, entry := range forecast {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO forecast_snapshots (project_id, ticket_id, detail, confidence_percent)
			VALUES (?, ?, ?, ?)
		`, projectID, entry.TicketID, entry.Detail, entry.ConfidencePercent); err != nil {
			return nil, err
		}
	}
	return forecast, nil
}

func ListProjectWorkItemQueue(ctx context.Context, db *sql.DB, projectID int64, strategy string, limit int) ([]WorkItemQueueCandidate, error) {
	if limit <= 0 {
		limit = 25
	}
	tickets, err := ListTickets(ctx, db, TicketListParams{
		ProjectID: projectID,
		Limit:     0,
	})
	if err != nil {
		return nil, err
	}
	queue := make([]WorkItemQueueCandidate, 0, len(tickets))
	for _, ticket := range tickets {
		if ticket.Archived || ticket.Complete || ticket.Draft || ticket.Deleted {
			continue
		}
		if strings.TrimSpace(ticket.Assignee) != "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(ticket.State), StateFail) {
			continue
		}
		deps, depErr := ListDependencies(ctx, db, ticket.ID)
		if depErr != nil {
			return nil, depErr
		}
		blocked := false
		blockedBy := ""
		for _, dep := range deps {
			depTicket, depTicketErr := GetTicket(ctx, db, dep.DependsOn)
			if depTicketErr != nil {
				return nil, depTicketErr
			}
			if !depTicket.Complete && !depTicket.Archived {
				blocked = true
				blockedBy = dep.DependsOn
				break
			}
		}
		queue = append(queue, WorkItemQueueCandidate{
			TicketID:  ticket.ID,
			Title:     ticket.Title,
			Priority:  ticket.Priority,
			Order:     ticket.Order,
			Stage:     ticket.Stage,
			State:     ticket.State,
			Assignee:  ticket.Assignee,
			Blocked:   blocked,
			BlockedBy: blockedBy,
			UpdatedAt: ticket.UpdatedAt,
		})
	}
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "order":
		sort.SliceStable(queue, func(i, j int) bool {
			if queue[i].Blocked != queue[j].Blocked {
				return !queue[i].Blocked
			}
			return queue[i].Order < queue[j].Order
		})
	case "aging":
		sort.SliceStable(queue, func(i, j int) bool {
			if queue[i].Blocked != queue[j].Blocked {
				return !queue[i].Blocked
			}
			return queue[i].UpdatedAt < queue[j].UpdatedAt
		})
	default:
		sort.SliceStable(queue, func(i, j int) bool {
			if queue[i].Blocked != queue[j].Blocked {
				return !queue[i].Blocked
			}
			if queue[i].Priority != queue[j].Priority {
				return queue[i].Priority < queue[j].Priority
			}
			return queue[i].Order < queue[j].Order
		})
	}
	if len(queue) > limit {
		queue = queue[:limit]
	}
	return queue, nil
}

func BuildProjectForecastCalibration(ctx context.Context, db *sql.DB, projectID int64, lookbackHours int) (ProjectForecastCalibration, error) {
	if err := ensureForecastSnapshotTable(ctx, db); err != nil {
		return ProjectForecastCalibration{}, err
	}
	if lookbackHours <= 0 {
		lookbackHours = 1
	}
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_id, detail, confidence_percent
		FROM forecast_snapshots
		WHERE project_id = ? AND created_at <= datetime('now', ?)
		AND snapshot_id IN (
			SELECT MAX(snapshot_id) FROM forecast_snapshots
			WHERE project_id = ? AND created_at <= datetime('now', ?)
			GROUP BY ticket_id
		)
	`, projectID, fmt.Sprintf("-%d hours", lookbackHours), projectID, fmt.Sprintf("-%d hours", lookbackHours))
	if err != nil {
		return ProjectForecastCalibration{}, err
	}
	defer rows.Close()
	report := ProjectForecastCalibration{
		ProjectID: int(projectID),
		Buckets: []ForecastCalibrationBucket{
			{Range: "0-49"},
			{Range: "50-69"},
			{Range: "70-100"},
		},
	}
	type snapshotSample struct {
		TicketID   string
		Detail     string
		Confidence int
	}
	samples := make([]snapshotSample, 0)
	for rows.Next() {
		var sample snapshotSample
		if scanErr := rows.Scan(&sample.TicketID, &sample.Detail, &sample.Confidence); scanErr != nil {
			return ProjectForecastCalibration{}, scanErr
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return ProjectForecastCalibration{}, err
	}
	for _, sample := range samples {
		ticket, ticketErr := GetTicket(ctx, db, sample.TicketID)
		if ticketErr != nil {
			return ProjectForecastCalibration{}, ticketErr
		}
		hit := evaluateForecastHit(sample.Detail, ticket)
		bucketIdx := 2
		if sample.Confidence < 50 {
			bucketIdx = 0
		} else if sample.Confidence < 70 {
			bucketIdx = 1
		}
		report.Buckets[bucketIdx].Total++
		report.SampleCount++
		if hit {
			report.Buckets[bucketIdx].Hits++
			report.HitCount++
		}
	}
	for idx := range report.Buckets {
		b := &report.Buckets[idx]
		if b.Total > 0 {
			b.AccuracyRate = float64(b.Hits) / float64(b.Total)
		}
	}
	if report.SampleCount > 0 {
		report.AccuracyRate = float64(report.HitCount) / float64(report.SampleCount)
	}
	return report, nil
}

func evaluateForecastHit(detail string, ticket Ticket) bool {
	label := strings.ToLower(strings.TrimSpace(detail))
	switch {
	case strings.Contains(label, "blocked by:"):
		return !ticket.Complete && !ticket.Archived
	case strings.Contains(label, "requires intervention"):
		return strings.EqualFold(strings.TrimSpace(ticket.State), StateFail)
	case strings.Contains(label, "completion/close decision"):
		return ticket.Complete || strings.EqualFold(strings.TrimSpace(ticket.Stage), StageDone)
	case strings.Contains(label, "in progress:"):
		return strings.EqualFold(strings.TrimSpace(ticket.State), StateActive) || strings.EqualFold(strings.TrimSpace(ticket.State), StateIdle)
	case strings.Contains(label, "next:"):
		return strings.EqualFold(strings.TrimSpace(ticket.State), StateSuccess) || strings.EqualFold(strings.TrimSpace(ticket.State), StateIdle)
	default:
		return false
	}
}
