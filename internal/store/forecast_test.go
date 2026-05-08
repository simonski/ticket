package store

import (
	"context"
	"testing"
)

func TestBuildProjectForecast(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Forecast Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	workflow, err := CreateWorkflow(ctx, db, "forecast workflow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	design, err := AddWorkflowStage(ctx, db, workflow.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := AddWorkflowStage(ctx, db, workflow.ID, "test", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	done, err := AddWorkflowStage(ctx, db, workflow.ID, "done", "", "", 2)
	if err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	if err := SetWorkflowStageTransitions(ctx, db, workflow.ID, design.ID, []int64{testStage.ID}); err != nil {
		t.Fatalf("SetWorkflowStageTransitions() error = %v", err)
	}
	if err := SetWorkflowStageTransitions(ctx, db, workflow.ID, testStage.ID, []int64{done.ID}); err != nil {
		t.Fatalf("SetWorkflowStageTransitions(test->done) error = %v", err)
	}

	blocker, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &workflow.ID,
		Type:       "task",
		Title:      "Blocker",
		State:      StateIdle,
		CreatedBy:  "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(blocker) error = %v", err)
	}
	blocked, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &workflow.ID,
		Type:       "task",
		Title:      "Blocked",
		State:      StateIdle,
		CreatedBy:  "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(blocked) error = %v", err)
	}
	if _, err := AddDependency(ctx, db, project.ID, blocked.ID, blocker.ID, ""); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	failing, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &workflow.ID,
		Type:       "task",
		Title:      "Failing",
		State:      StateFail,
		CreatedBy:  "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(failing) error = %v", err)
	}
	succeeding, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:  project.ID,
		WorkflowID: &workflow.ID,
		Type:       "task",
		Title:      "Succeeding",
		State:      StateSuccess,
		CreatedBy:  "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(succeeding) error = %v", err)
	}
	succeeding, err = UpdateTicket(ctx, db, succeeding.ID, TicketUpdateParams{
		Title:         succeeding.Title,
		Description:   succeeding.Description,
		Assignee:      succeeding.Assignee,
		Priority:      succeeding.Priority,
		Order:         succeeding.Order,
		Stage:         design.StageName,
		State:         StateSuccess,
		ActorRole:     "admin",
		ActorUsername: "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(succeeding) error = %v", err)
	}
	forecasts, err := BuildProjectForecast(ctx, db, project.ID, 25)
	if err != nil {
		t.Fatalf("BuildProjectForecast() error = %v", err)
	}
	if len(forecasts) < 3 {
		t.Fatalf("expected at least three forecasts, got %d %#v", len(forecasts), forecasts)
	}
	hasBlocked := false
	hasFail := false
	hasNext := false
	for _, entry := range forecasts {
		if entry.TicketID == blocked.ID && entry.ConfidencePercent >= 75 {
			hasBlocked = true
		}
		if entry.TicketID == failing.ID && entry.ConfidencePercent <= 40 {
			hasFail = true
		}
		if entry.TicketID == succeeding.ID && entry.Detail != "" {
			hasNext = true
		}
	}
	if !hasBlocked || !hasFail || !hasNext {
		t.Fatalf("unexpected forecast entries: %#v", forecasts)
	}
}
