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
	CommitRef         string `json:"commit_ref"`
	StartedAt         string `json:"started_at,omitempty"`
	CompletedAt       string `json:"completed_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type WorkItemListParams struct {
	Status       string
	AssigneeType string
	Limit        int
	Offset       int
}

var ErrWorkItemNotFound = errors.New("work item not found")

func ListWorkItemsByTicket(ctx context.Context, db *sql.DB, ticketID string, limit, offset int) ([]WorkItem, error) {
	return ListWorkItemsByTicketWithParams(ctx, db, ticketID, WorkItemListParams{
		Limit:  limit,
		Offset: offset,
	})
}

func ListWorkItemsByTicketWithParams(ctx context.Context, db *sql.DB, ticketID string, params WorkItemListParams) ([]WorkItem, error) {
	limit, offset, err := normalizePage(params.Limit, params.Offset, DefaultHistoryLimit)
	if err != nil {
		return nil, err
	}
	status, err := normalizeWorkItemStatus(params.Status)
	if err != nil {
		return nil, err
	}
	assigneeType, err := normalizeWorkItemAssigneeType(params.AssigneeType)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
		       assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback, commit_ref,
		       started_at, completed_at, created_at, updated_at
		FROM work_items
		WHERE ticket_id = ?
	`
	args := []any{ticketID}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if assigneeType != "" {
		query += " AND assignee_type = ?"
		args = append(args, assigneeType)
	}
	query += `
		ORDER BY created_at DESC, work_item_id DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)
	rows, err := db.QueryContext(ctx, query, args...)
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

func normalizeWorkItemStatus(raw string) (string, error) {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		return "", nil
	}
	switch status {
	case WorkItemStatusActive, WorkItemStatusSuccess, WorkItemStatusFail, WorkItemStatusStopped:
		return status, nil
	default:
		return "", fmt.Errorf("invalid work item status %q", raw)
	}
}

func normalizeWorkItemAssigneeType(raw string) (string, error) {
	assigneeType := strings.TrimSpace(strings.ToLower(raw))
	if assigneeType == "" {
		return "", nil
	}
	switch assigneeType {
	case "human", "agent":
		return assigneeType, nil
	default:
		return "", fmt.Errorf("invalid work item assignee_type %q", raw)
	}
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

	assigneeType, assigneeID := resolveWorkItemAssignee(ctx, db, assigneeUsername)

	workflowID := ResolveWorkflowID(ctx, db, ticket)
	objectiveSnapshot, promptSnapshot := buildWorkItemSnapshots(ctx, db, ticket, workflowID)
	_, err = db.ExecContext(ctx, `
		INSERT INTO work_items (
			work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
			assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback, commit_ref, started_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', CURRENT_TIMESTAMP)
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

func GetWorkItemByTicket(ctx context.Context, db *sql.DB, ticketID, workItemID string) (WorkItem, error) {
	row := db.QueryRowContext(ctx, `
		SELECT work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
		       assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback, commit_ref,
		       started_at, completed_at, created_at, updated_at
		FROM work_items
		WHERE ticket_id = ? AND work_item_id = ?
	`, ticketID, workItemID)
	item, err := scanWorkItem(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkItem{}, ErrWorkItemNotFound
		}
		return WorkItem{}, err
	}
	return item, nil
}

func ReassignWorkItem(ctx context.Context, db *sql.DB, ticketID, workItemID, assigneeUsername, actorUsername, actorID string) (WorkItem, error) {
	item, err := GetWorkItemByTicket(ctx, db, ticketID, workItemID)
	if err != nil {
		return WorkItem{}, err
	}
	if item.Status != WorkItemStatusActive {
		return WorkItem{}, errors.New("only active work items can be reassigned")
	}
	assigneeUsername = strings.TrimSpace(assigneeUsername)
	if assigneeUsername == "" {
		return WorkItem{}, errors.New("assignee is required")
	}
	assigneeType, assigneeID, resolveErr := resolveExistingWorkItemAssignee(ctx, db, assigneeUsername)
	if resolveErr != nil {
		return WorkItem{}, resolveErr
	}
	note := fmt.Sprintf("reassigned by %s (%s) to %s", strings.TrimSpace(actorUsername), strings.TrimSpace(actorID), assigneeUsername)
	_, err = db.ExecContext(ctx, `
		UPDATE work_items
		SET assignee_type = ?, assignee_id = ?, feedback = ?, updated_at = CURRENT_TIMESTAMP
		WHERE work_item_id = ? AND ticket_id = ?
	`, assigneeType, assigneeID, appendWorkItemFeedback(item.Feedback, note), workItemID, ticketID)
	if err != nil {
		return WorkItem{}, err
	}
	return GetWorkItemByTicket(ctx, db, ticketID, workItemID)
}

func AddWorkItemFeedback(ctx context.Context, db *sql.DB, ticketID, workItemID, message, commitRef, actorUsername, actorID string) (WorkItem, error) {
	item, err := GetWorkItemByTicket(ctx, db, ticketID, workItemID)
	if err != nil {
		return WorkItem{}, err
	}
	message = strings.TrimSpace(message)
	commitRef = strings.TrimSpace(commitRef)
	if message == "" && commitRef == "" {
		return WorkItem{}, errors.New("feedback message or commit_ref is required")
	}
	note := message
	if note == "" {
		note = "feedback updated"
	}
	audit := fmt.Sprintf("feedback by %s (%s): %s", strings.TrimSpace(actorUsername), strings.TrimSpace(actorID), note)
	nextCommitRef := item.CommitRef
	if commitRef != "" {
		nextCommitRef = commitRef
	}
	_, err = db.ExecContext(ctx, `
		UPDATE work_items
		SET feedback = ?, commit_ref = ?, updated_at = CURRENT_TIMESTAMP
		WHERE work_item_id = ? AND ticket_id = ?
	`, appendWorkItemFeedback(item.Feedback, audit), nextCommitRef, workItemID, ticketID)
	if err != nil {
		return WorkItem{}, err
	}
	return GetWorkItemByTicket(ctx, db, ticketID, workItemID)
}

func CancelWorkItem(ctx context.Context, db *sql.DB, ticketID, workItemID, reason, actorUsername, actorID string) (WorkItem, error) {
	item, err := GetWorkItemByTicket(ctx, db, ticketID, workItemID)
	if err != nil {
		return WorkItem{}, err
	}
	if item.Status != WorkItemStatusActive {
		return WorkItem{}, errors.New("only active work items can be cancelled")
	}
	note := fmt.Sprintf("cancelled by %s (%s)", strings.TrimSpace(actorUsername), strings.TrimSpace(actorID))
	if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
		note += ": " + trimmedReason
	}
	_, err = db.ExecContext(ctx, `
		UPDATE work_items
		SET status = ?, completed_at = CURRENT_TIMESTAMP, feedback = ?, updated_at = CURRENT_TIMESTAMP
		WHERE work_item_id = ? AND ticket_id = ?
	`, WorkItemStatusStopped, appendWorkItemFeedback(item.Feedback, note), workItemID, ticketID)
	if err != nil {
		return WorkItem{}, err
	}
	return GetWorkItemByTicket(ctx, db, ticketID, workItemID)
}

func RetryWorkItem(ctx context.Context, db *sql.DB, ticketID, workItemID, assigneeUsername, actorUsername, actorID string) (WorkItem, error) {
	item, err := GetWorkItemByTicket(ctx, db, ticketID, workItemID)
	if err != nil {
		return WorkItem{}, err
	}
	if item.Status == WorkItemStatusActive {
		return WorkItem{}, errors.New("active work item cannot be retried")
	}
	var activeID string
	activeErr := db.QueryRowContext(ctx, `
		SELECT work_item_id FROM work_items
		WHERE ticket_id = ? AND status = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, ticketID, WorkItemStatusActive).Scan(&activeID)
	if activeErr == nil {
		return WorkItem{}, errors.New("ticket already has an active work item")
	}
	if activeErr != nil && !errors.Is(activeErr, sql.ErrNoRows) {
		return WorkItem{}, activeErr
	}

	nextAssigneeType := item.AssigneeType
	nextAssigneeID := item.AssigneeID
	if assignee := strings.TrimSpace(assigneeUsername); assignee != "" {
		assigneeType, assigneeID, resolveErr := resolveExistingWorkItemAssignee(ctx, db, assignee)
		if resolveErr != nil {
			return WorkItem{}, resolveErr
		}
		nextAssigneeType, nextAssigneeID = assigneeType, assigneeID
	}
	nextID := uuid.NewString()
	note := fmt.Sprintf("retry of %s by %s (%s)", item.ID, strings.TrimSpace(actorUsername), strings.TrimSpace(actorID))
	_, err = db.ExecContext(ctx, `
		INSERT INTO work_items (
			work_item_id, ticket_id, project_id, workflow_id, workflow_stage_id, role_id, status,
			assignee_type, assignee_id, objective_snapshot, prompt_snapshot, feedback, commit_ref, started_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP)
	`, nextID, item.TicketID, item.ProjectID, nullableInt64(item.WorkflowID), nullableInt64(item.WorkflowStageID), nullableInt64(item.RoleID),
		WorkItemStatusActive, nextAssigneeType, nextAssigneeID, item.ObjectiveSnapshot, item.PromptSnapshot, note)
	if err != nil {
		return WorkItem{}, err
	}
	return GetWorkItemByTicket(ctx, db, ticketID, nextID)
}

func resolveWorkItemAssignee(ctx context.Context, db *sql.DB, assigneeUsername string) (assigneeType, assigneeID string) {
	assigneeUsername = strings.TrimSpace(assigneeUsername)
	if assigneeUsername == "" {
		return "human", ""
	}
	assigneeID = assigneeUsername
	assigneeType = "human"
	if user, userErr := GetUserByUsername(ctx, db, assigneeUsername); userErr == nil {
		assigneeID = user.ID
		if strings.EqualFold(strings.TrimSpace(user.UserType), "agent") {
			assigneeType = "agent"
		}
	}
	return assigneeType, assigneeID
}

func resolveExistingWorkItemAssignee(ctx context.Context, db *sql.DB, assigneeUsername string) (assigneeType, assigneeID string, err error) {
	assigneeUsername = strings.TrimSpace(assigneeUsername)
	if assigneeUsername == "" {
		return "", "", errors.New("assignee is required")
	}
	user, userErr := GetUserByUsername(ctx, db, assigneeUsername)
	if userErr != nil {
		if errors.Is(userErr, sql.ErrNoRows) {
			return "", "", errors.New("assignee user not found")
		}
		return "", "", userErr
	}
	if !user.Enabled {
		return "", "", errors.New("assignee user is disabled")
	}
	assigneeType = "human"
	if strings.EqualFold(strings.TrimSpace(user.UserType), "agent") {
		assigneeType = "agent"
	}
	return assigneeType, user.ID, nil
}

func appendWorkItemFeedback(existing, note string) string {
	existing = strings.TrimSpace(existing)
	note = strings.TrimSpace(note)
	if note == "" {
		return existing
	}
	if existing == "" {
		return note
	}
	return existing + "\n" + note
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
		&item.AssigneeType, &item.AssigneeID, &item.ObjectiveSnapshot, &item.PromptSnapshot, &item.Feedback, &item.CommitRef,
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
