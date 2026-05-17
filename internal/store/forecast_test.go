package store

import (
	"context"
	"testing"
)

func TestBuildProjectForecast(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

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
	if _, err := SetTicketDraft(ctx, db, blocker.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft(blocker=false) error = %v", err)
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
	if _, err := SetTicketDraft(ctx, db, blocked.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft(blocked=false) error = %v", err)
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
	if _, err := SetTicketDraft(ctx, db, failing.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft(failing=false) error = %v", err)
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
	if _, err := SetTicketDraft(ctx, db, succeeding.ID, false, "admin", adminID); err != nil {
		t.Fatalf("SetTicketDraft(succeeding=false) error = %v", err)
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
	queue, err := ListProjectWorkItemQueue(ctx, db, project.ID, "priority", 20)
	if err != nil {
		t.Fatalf("ListProjectWorkItemQueue() error = %v", err)
	}
	if len(queue) == 0 {
		t.Fatalf("expected queue candidates, got %#v", queue)
	}
	if _, err := db.ExecContext(ctx, `UPDATE forecast_snapshots SET created_at = datetime('now', '-2 hours') WHERE project_id = ?`, project.ID); err != nil {
		t.Fatalf("aging forecast snapshots error = %v", err)
	}
	calibration, err := BuildProjectForecastCalibration(ctx, db, project.ID, 1)
	if err != nil {
		t.Fatalf("BuildProjectForecastCalibration() error = %v", err)
	}
	if len(calibration.Buckets) != 3 {
		t.Fatalf("unexpected calibration payload: %#v", calibration)
	}
	backtest, err := BuildProjectForecastBacktest(ctx, db, project.ID, 24)
	if err != nil {
		t.Fatalf("BuildProjectForecastBacktest() error = %v", err)
	}
	if backtest.ProjectID != int(project.ID) {
		t.Fatalf("backtest project id = %d, want %d", backtest.ProjectID, project.ID)
	}
	if backtest.SampleCount == 0 || len(backtest.Points) == 0 {
		t.Fatalf("expected non-empty backtest payload, got %#v", backtest)
	}
}
