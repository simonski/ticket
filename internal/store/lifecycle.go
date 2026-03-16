package store

import (
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

var stageOrder = map[string]int{
	StageDesign:  0,
	StageDevelop: 1,
	StageTest:    2,
	StageDone:    3,
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

func CompareStageOrder(left, right string) int {
	leftOrder, leftOK := stageOrder[left]
	rightOrder, rightOK := stageOrder[right]
	if !leftOK || !rightOK {
		return 0
	}
	switch {
	case leftOrder < rightOrder:
		return -1
	case leftOrder > rightOrder:
		return 1
	default:
		return 0
	}
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
