package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	StageDesign  = "design"
	StageDevelop = "develop"
	StageTest    = "test"
	StageDone    = "done"
)

const (
	StateIdle     = "idle"
	StateActive   = "active"
	StateSuccess  = "success"
	StateFail     = "fail"
	StateComplete = "complete"
)

var validStages = map[string]bool{
	StageDesign:  true,
	StageDevelop: true,
	StageTest:    true,
	StageDone:    true,
}

var validStates = map[string]bool{
	StateIdle:    true,
	StateActive:  true,
	StateSuccess: true,
	StateFail:    true,
}

var legacyStateAliases = map[string]string{
	StateComplete: StateSuccess,
}

func ValidStage(stage string) bool {
	return validStages[stage]
}

func ValidState(state string) bool {
	state = normalizeState(state)
	return validStates[state]
}

func normalizeState(state string) string {
	state = strings.TrimSpace(strings.ToLower(state))
	if normalized, ok := legacyStateAliases[state]; ok {
		return normalized
	}
	return state
}

func ValidLifecycle(stage, state string) bool {
	state = normalizeState(state)
	if !ValidStage(stage) || !ValidState(state) {
		return false
	}
	if stage == StageDone {
		return state == StateSuccess || state == StateFail
	}
	return true
}

func RenderLifecycleStatus(stage, state string) string {
	return stage + "/" + state
}

func ParseLifecycleStatus(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" || !strings.Contains(trimmed, "/") {
		return "", "", fmt.Errorf("invalid status %q", raw)
	}
	parts := strings.SplitN(trimmed, "/", 2)
	state := normalizeState(parts[1])
	if len(parts) == 2 && ValidLifecycle(parts[0], state) {
		return parts[0], state, nil
	}
	return "", "", fmt.Errorf("invalid status %q", raw)
}

// getNextWorkflowStage returns the next workflow stage after the given stage ID.
// Returns (nil, "", nil) if the given stage is the final stage.
func getNextWorkflowStage(ctx context.Context, db *sql.DB, currentStageID int64) (*int64, string, error) {
	var workflowID int64
	var currentOrder int
	if err := db.QueryRowContext(ctx, `SELECT workflow_id, sort_order FROM workflow_stages WHERE workflow_stage_id = ?`, currentStageID).Scan(&workflowID, &currentOrder); err != nil {
		return nil, "", err
	}
	var nextID int64
	var nextName string
	err := db.QueryRowContext(ctx, `SELECT workflow_stage_id, stage_name FROM workflow_stages WHERE workflow_id = ? AND sort_order > ? ORDER BY sort_order LIMIT 1`, workflowID, currentOrder).Scan(&nextID, &nextName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil // final stage
	}
	if err != nil {
		return nil, "", err
	}
	return &nextID, nextName, nil
}

// GetWorkflowStageOrder returns the sort_order for a workflow stage by ID.
func GetWorkflowStageOrder(ctx context.Context, db *sql.DB, stageID int64) (int, error) {
	var order int
	err := db.QueryRowContext(ctx, `SELECT sort_order FROM workflow_stages WHERE workflow_stage_id = ?`, stageID).Scan(&order)
	return order, err
}

// batchGetWorkflowStageOrders collects all unique non-nil WorkflowStageID values from
// the given tickets and fetches their sort_order in a single query. Returns a
// map from workflow_stage_id to sort_order.
func batchGetWorkflowStageOrders(ctx context.Context, db *sql.DB, tickets []Ticket) (map[int64]int, error) {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, t := range tickets {
		if t.WorkflowStageID != nil && !seen[*t.WorkflowStageID] {
			seen[*t.WorkflowStageID] = true
			ids = append(ids, *t.WorkflowStageID)
		}
	}
	if len(ids) == 0 {
		return make(map[int64]int), nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT workflow_stage_id, sort_order FROM workflow_stages WHERE workflow_stage_id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]int, len(ids))
	for rows.Next() {
		var stageID int64
		var order int
		if err := rows.Scan(&stageID, &order); err != nil {
			return nil, err
		}
		result[stageID] = order
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
