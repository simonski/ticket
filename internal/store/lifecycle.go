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
	StateComplete = "complete"
)

var validStages = map[string]bool{
	StageDesign:  true,
	StageDevelop: true,
	StageTest:    true,
	StageDone:    true,
}

var validStates = map[string]bool{
	StateIdle:     true,
	StateActive:   true,
	StateComplete: true,
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
	return validStates[state]
}

func ValidLifecycle(stage, state string) bool {
	if !ValidStage(stage) || !ValidState(state) {
		return false
	}
	if stage == StageDone {
		return state == StateComplete
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
	if len(parts) == 2 && ValidLifecycle(parts[0], parts[1]) {
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("invalid status %q", raw)
}
