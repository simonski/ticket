package store

import (
	"context"
	"database/sql"
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

// getNextSdlcStage returns the next sdlc stage after the given stage ID.
// Returns (nil, "", nil) if the given stage is the final stage.
func getNextSdlcStage(ctx context.Context, db *sql.DB, currentStageID int64) (*int64, string, error) {
	var sdlcID int64
	var currentOrder int
	if err := db.QueryRowContext(ctx, `SELECT sdlc_id, sort_order FROM sdlc_stages WHERE sdlc_stage_id = ?`, currentStageID).Scan(&sdlcID, &currentOrder); err != nil {
		return nil, "", err
	}
	var nextID int64
	var nextName string
	err := db.QueryRowContext(ctx, `SELECT sdlc_stage_id, stage_name FROM sdlc_stages WHERE sdlc_id = ? AND sort_order > ? ORDER BY sort_order LIMIT 1`, sdlcID, currentOrder).Scan(&nextID, &nextName)
	if err == sql.ErrNoRows {
		return nil, "", nil // final stage
	}
	if err != nil {
		return nil, "", err
	}
	return &nextID, nextName, nil
}

// GetSdlcStageOrder returns the sort_order for a sdlc stage by ID.
func GetSdlcStageOrder(ctx context.Context, db *sql.DB, stageID int64) (int, error) {
	var order int
	err := db.QueryRowContext(ctx, `SELECT sort_order FROM sdlc_stages WHERE sdlc_stage_id = ?`, stageID).Scan(&order)
	return order, err
}
