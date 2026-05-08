package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	WorkItemStatusActive  = "active"
	WorkItemStatusSuccess = "success"
	WorkItemStatusFail    = "fail"
	WorkItemStatusStopped = "stopped"
)

type WorkItem struct {
	ID                string `json:"work_item_id"`
	TicketID          string `json:"ticket_id"`
	ProjectID         int64  `json:"project_id"`
	WorkflowID        *int64 `json:"workflow_id,omitempty"`
	WorkflowStageID   *int64 `json:"workflow_stage_id,omitempty"`
	RoleID            *int64 `json:"role_id,omitempty"`
	Status            string `json:"status"`
	AssigneeType      string `json:"assignee_type"`
	AssigneeID        string `json:"assignee_id"`
	ObjectiveSnapshot string `json:"objective_snapshot"`
	PromptSnapshot    string `json:"prompt_snapshot"`
	Feedback          string `json:"feedback"`
	StartedAt         string `json:"started_at,omitempty"`
	CompletedAt       string `json:"completed_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func ListWorkItemsByTicket(ctx context.Context, db *sql.DB, ticketID string, limit, offset int) ([]WorkItem, error) {
	limit, offset, err := normalizePage(limit, offset, DefaultHistoryLimit)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
		       assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback,
		       started_at, completed_at, created_at, updated_at
		FROM work_items
		WHERE ticket_id = ?
		ORDER BY created_at DESC, work_item_id DESC
		LIMIT ? OFFSET ?
	`, ticketID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WorkItem, 0)
	for rows.Next() {
		item, scanErr := scanWorkItem(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func syncTicketWorkItems(ctx context.Context, db *sql.DB, before, after Ticket, requestedState, actorUsername, actorID string) error {
	wasActive := normalizeState(before.State) == StateActive
	nowActive := normalizeState(after.State) == StateActive

	if !wasActive && nowActive {
		return ensureActiveWorkItem(ctx, db, after, actorUsername)
	}
	if wasActive && !nowActive {
		finalStatus := WorkItemStatusStopped
		switch normalizeState(requestedState) {
		case StateSuccess:
			finalStatus = WorkItemStatusSuccess
		case StateFail:
			finalStatus = WorkItemStatusFail
		default:
			switch normalizeState(after.State) {
			case StateSuccess:
				finalStatus = WorkItemStatusSuccess
			case StateFail:
				finalStatus = WorkItemStatusFail
			}
		}
		feedback := fmt.Sprintf("state transition %s/%s -> %s/%s by %s (%s)", before.Stage, before.State, after.Stage, after.State, strings.TrimSpace(actorUsername), strings.TrimSpace(actorID))
		return closeActiveWorkItems(ctx, db, after.ID, finalStatus, feedback)
	}
	return nil
}

func ensureActiveWorkItem(ctx context.Context, db *sql.DB, ticket Ticket, assigneeUsername string) error {
	assigneeUsername = strings.TrimSpace(assigneeUsername)
	if assigneeUsername == "" {
		assigneeUsername = strings.TrimSpace(ticket.Assignee)
	}
	if assigneeUsername == "" {
		return nil
	}
	var existing string
	err := db.QueryRowContext(ctx, `
		SELECT work_item_id
		FROM work_items
		WHERE ticket_id = ? AND status = ?
		ORDER BY created_at DESC, work_item_id DESC
		LIMIT 1
	`, ticket.ID, WorkItemStatusActive).Scan(&existing)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}

	assigneeID := assigneeUsername
	assigneeType := "human"
	if user, userErr := GetUserByUsername(ctx, db, assigneeUsername); userErr == nil {
		assigneeID = user.ID
		if strings.EqualFold(strings.TrimSpace(user.UserType), "agent") {
			assigneeType = "agent"
		}
	}

	workflowID := ResolveWorkflowID(ctx, db, ticket)
	objectiveSnapshot, promptSnapshot := buildWorkItemSnapshots(ctx, db, ticket, workflowID)
	_, err = db.ExecContext(ctx, `
		INSERT INTO work_items (
			work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
			assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback, started_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP)
	`, uuid.NewString(), ticket.ID, ticket.ProjectID, nullableInt64(workflowID), nullableInt64(ticket.WorkflowStageID), nullableInt64(ticket.RoleID),
		WorkItemStatusActive, assigneeType, assigneeID, objectiveSnapshot, promptSnapshot)
	return err
}

func closeActiveWorkItems(ctx context.Context, db *sql.DB, ticketID, finalStatus, feedback string) error {
	finalStatus = strings.TrimSpace(strings.ToLower(finalStatus))
	if finalStatus == "" {
		finalStatus = WorkItemStatusStopped
	}
	switch finalStatus {
	case WorkItemStatusSuccess, WorkItemStatusFail, WorkItemStatusStopped:
	default:
		finalStatus = WorkItemStatusStopped
	}
	_, err := db.ExecContext(ctx, `
		UPDATE work_items
		SET status = ?, feedback = ?, completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ? AND status = ?
	`, finalStatus, strings.TrimSpace(feedback), ticketID, WorkItemStatusActive)
	return err
}

func scanWorkItem(scan func(dest ...any) error) (WorkItem, error) {
	var item WorkItem
	var workflowID sql.NullInt64
	var workflowStageID sql.NullInt64
	var roleID sql.NullInt64
	var startedAt sql.NullString
	var completedAt sql.NullString
	if err := scan(
		&item.ID, &item.TicketID, &item.ProjectID, &workflowID, &workflowStageID, &roleID, &item.Status,
		&item.AssigneeType, &item.AssigneeID, &item.ObjectiveSnapshot, &item.PromptSnapshot, &item.Feedback,
		&startedAt, &completedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return WorkItem{}, err
	}
	item.WorkflowID = nullInt64ToPtr(workflowID)
	item.WorkflowStageID = nullInt64ToPtr(workflowStageID)
	item.RoleID = nullInt64ToPtr(roleID)
	if startedAt.Valid {
		item.StartedAt = startedAt.String
	}
	if completedAt.Valid {
		item.CompletedAt = completedAt.String
	}
	return item, nil
}

func nullInt64ToPtr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func buildWorkItemSnapshots(ctx context.Context, db *sql.DB, ticket Ticket, workflowID *int64) (objective, prompt string) {
	stageName := strings.TrimSpace(ticket.Stage)
	if stageName == "" {
		stageName = StageDevelop
	}
	roleName := ""
	if ticket.RoleID != nil {
		_ = db.QueryRowContext(ctx, `SELECT title FROM roles WHERE role_id = ?`, *ticket.RoleID).Scan(&roleName)
	}
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		roleName = "engineer"
	}
	objective = strings.TrimSpace(ticket.AcceptanceCriteria)
	if objective == "" {
		objective = strings.TrimSpace(ticket.Title)
	}
	if objective == "" {
		objective = "progress ticket to next workflow step"
	}
	workflowLabel := "workflow"
	if workflowID != nil {
		var name string
		if err := db.QueryRowContext(ctx, `SELECT name FROM workflows WHERE workflow_id = ?`, *workflowID).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
			workflowLabel = strings.TrimSpace(name)
		}
	}
	prompt = fmt.Sprintf(
		"During %s (%s), perform the role %s. Objective: %s. Ticket: %s (%s). Acceptance criteria: %s.",
		stageName,
		workflowLabel,
		roleName,
		objective,
		strings.TrimSpace(ticket.ID),
		strings.TrimSpace(ticket.Title),
		objective,
	)
	return objective, prompt
}
