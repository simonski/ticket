package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

func TestCreateUpdateAndListTickets(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	epic, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Authentication",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}

	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID:        project.ID,
		ParentID:         &epic.ID,
		Type:             "task",
		Title:            "Add password reset",
		EstimateEffort:   5,
		EstimateComplete: "2026-04-01T12:00:00Z",
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Fatalf("CreateTicket().ParentID = %#v, want %d", task.ParentID, epic.ID)
	}
	if task.Stage != StageDesign || task.State != StateIdle {
		t.Fatalf("CreateTicket().Lifecycle = %s/%s, want design/idle", task.Stage, task.State)
	}
	if task.EstimateEffort != 5 || task.EstimateComplete != "2026-04-01T12:00:00Z" {
		t.Fatalf("CreateTicket() estimates = %#v", task)
	}

	tasks, err := ListTicketsByProject(db, project.ID)
	if err != nil {
		t.Fatalf("ListTicketsByProject() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("ListTicketsByProject() len = %d, want 2", len(tasks))
	}

	updated, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:            "Add password reset workflow",
		Description:      "Support email-based reset",
		ParentID:         &epic.ID,
		EstimateEffort:   8,
		EstimateComplete: "2026-04-15T09:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Title != "Add password reset workflow" {
		t.Fatalf("UpdateTicket().Title = %q", updated.Title)
	}
	if updated.EstimateEffort != 8 || updated.EstimateComplete != "2026-04-15T09:00:00Z" {
		t.Fatalf("UpdateTicket() estimates = %#v", updated)
	}

	statusUpdated, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:            updated.Title,
		Description:      updated.Description,
		ParentID:         updated.ParentID,
		Stage:            StageDevelop,
		State:            StateActive,
		Assignee:         "alice",
		ActorUsername:    "admin",
		ActorRole:        "admin",
		EstimateEffort:   updated.EstimateEffort,
		EstimateComplete: updated.EstimateComplete,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(stage/state) error = %v", err)
	}
	if statusUpdated.Status != "develop/active" {
		t.Fatalf("UpdateTicket().Status = %q, want develop/active", statusUpdated.Status)
	}
	if statusUpdated.Stage != StageDevelop || statusUpdated.State != StateActive {
		t.Fatalf("UpdateTicket().Lifecycle = %s/%s, want develop/active", statusUpdated.Stage, statusUpdated.State)
	}

	filtered, err := ListTickets(db, TicketListParams{
		ProjectID: project.ID,
		Type:      "task",
		Status:    "develop/active",
		Search:    "password",
	})
	if err != nil {
		t.Fatalf("ListTickets(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != task.ID {
		t.Fatalf("ListTickets(filtered) = %#v", filtered)
	}

	limited, err := ListTickets(db, TicketListParams{
		ProjectID: project.ID,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ListTickets(limited) error = %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("ListTickets(limited) len = %d, want 1", len(limited))
	}

	got, err := GetTicketByProject(db, project.ID, task.ID)
	if err != nil {
		t.Fatalf("GetTicketByProject() error = %v", err)
	}
	if got.ID != task.ID {
		t.Fatalf("GetTicketByProject().ID = %d, want %d", got.ID, task.ID)
	}
}

func TestSetTicketHealth(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Health check",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	updated, err := SetTicketHealth(db, task.ID, 3)
	if err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
	if updated.HealthScore != 3 {
		t.Fatalf("SetTicketHealth() score = %d, want 3", updated.HealthScore)
	}
	reloaded, err := GetTicket(db, task.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if reloaded.HealthScore != 3 {
		t.Fatalf("GetTicket().HealthScore = %d, want 3", reloaded.HealthScore)
	}
	if _, err := SetTicketHealth(db, task.ID, 6); err == nil {
		t.Fatalf("SetTicketHealth(out of range) = nil, want error")
	}
	if _, err := SetTicketHealth(db, 9999, 1); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("SetTicketHealth(unknown task) error = %v, want %v", err, ErrTicketNotFound)
	}
}

func TestCreateOrUpdateTicketEnforcesEpicParentRules(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	taskParent, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Regular task",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &taskParent.ID,
		Type:      "epic",
		Title:     "Invalid epic",
		CreatedBy: 1,
	}); err == nil || err.Error() != "task cannot parent epic" {
		t.Fatalf("CreateTicket(epic with non-epic parent) error = %v, want task cannot parent epic", err)
	}

	epicParent, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Valid epic",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}

	taskChild, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epicParent.ID,
		Type:      "task",
		Title:     "Task child",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(task child) error = %v", err)
	}

	_, err = UpdateTicket(db, epicParent.ID, TicketUpdateParams{
		Title:    "Valid epic",
		ParentID: &taskChild.ID,
	})
	if err == nil || err.Error() != "task cannot parent epic" {
		t.Fatalf("UpdateTicket(epic parented by task) error = %v, want task cannot parent epic", err)
	}
}

func TestRequestTicket(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	notReady, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Blocked setup",
		Stage:     StageDesign,
		State:     StateIdle,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(design/idle) error = %v", err)
	}
	openTask, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Open task",
		Stage:     StageDevelop,
		State:     StateIdle,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(develop/idle) error = %v", err)
	}

	assigned, status, err := RequestTicket(db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "alice",
		UserID:    2,
	})
	if err != nil {
		t.Fatalf("RequestTicket(any) error = %v", err)
	}
	if status != "ASSIGNED" || assigned.ID != openTask.ID {
		t.Fatalf("RequestTicket(any) = %#v, %q", assigned, status)
	}

	assignedAgain, status, err := RequestTicket(db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "alice",
		UserID:    2,
	})
	if err != nil {
		t.Fatalf("RequestTicket(existing open) error = %v", err)
	}
	if status != "ASSIGNED" || assignedAgain.ID != openTask.ID {
		t.Fatalf("RequestTicket(existing open) = %#v, %q", assignedAgain, status)
	}

	inProgress, err := UpdateTicket(db, openTask.ID, TicketUpdateParams{
		Title:         assigned.Title,
		Description:   assigned.Description,
		ParentID:      assigned.ParentID,
		Assignee:      "alice",
		Stage:         StageDevelop,
		State:         StateActive,
		UpdatedBy:     2,
		ActorUsername: "alice",
		ActorRole:     "user",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(develop/active) error = %v", err)
	}

	requested, status, err := RequestTicket(db, TicketRequestParams{
		ProjectID: project.ID,
		TicketID:  &notReady.ID,
		Username:  "alice",
		UserID:    2,
	})
	if err != nil {
		t.Fatalf("RequestTicket(existing inprogress) error = %v", err)
	}
	if status != "ASSIGNED" || requested.ID != inProgress.ID {
		t.Fatalf("RequestTicket(existing inprogress) = %#v, %q", requested, status)
	}

	if _, err := CreateUser(db, "bob", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	rejected, status, err := RequestTicket(db, TicketRequestParams{
		ProjectID: project.ID,
		TicketID:  &notReady.ID,
		Username:  "bob",
		UserID:    3,
	})
	if err != nil {
		t.Fatalf("RequestTicket(rejected) error = %v", err)
	}
	if status != "REJECTED" || rejected.ID != 0 {
		t.Fatalf("RequestTicket(rejected) = %#v, %q", rejected, status)
	}

	noWork, status, err := RequestTicket(db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "bob",
		UserID:    3,
	})
	if err != nil {
		t.Fatalf("RequestTicket(no-work) error = %v", err)
	}
	if status != "NO-WORK" || noWork.ID != 0 {
		t.Fatalf("RequestTicket(no-work) = %#v, %q", noWork, status)
	}
}

func TestUpdateTicketAssignmentRulesForNonAdmin(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add password reset",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	if _, err := CreateUser(db, "bob", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}

	claimed, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "alice",
		ActorUsername: "alice",
		ActorRole:     "user",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(claim self) error = %v", err)
	}
	if claimed.Assignee != "alice" {
		t.Fatalf("UpdateTicket(claim self).Assignee = %q, want alice", claimed.Assignee)
	}

	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         claimed.Title,
		Description:   claimed.Description,
		ParentID:      claimed.ParentID,
		Assignee:      "bob",
		ActorUsername: "bob",
		ActorRole:     "user",
	}); err == nil || err.Error() != "task is already assigned to alice" {
		t.Fatalf("UpdateTicket(claim assigned) error = %v, want task is already assigned to alice", err)
	}

	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         claimed.Title,
		Description:   claimed.Description,
		ParentID:      claimed.ParentID,
		Assignee:      "",
		ActorUsername: "bob",
		ActorRole:     "user",
	}); err == nil || err.Error() != "task is assigned to alice" {
		t.Fatalf("UpdateTicket(unclaim other) error = %v, want task is assigned to alice", err)
	}
}

func TestUpdateTicketAssignRequiresExistingEnabledUser(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add password reset",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "nobody",
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err == nil || err.Error() != "user not found" {
		t.Fatalf("UpdateTicket(assign missing user) error = %v, want user not found", err)
	}
	if err := SetUserEnabled(db, "alice", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "alice",
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err == nil || err.Error() != "user is disabled" {
		t.Fatalf("UpdateTicket(assign disabled user) error = %v, want user is disabled", err)
	}
}

func TestUpdateTicketStatusRequiresAssignee(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Status-owned task",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "",
		Stage:         StageDevelop,
		State:         StateActive,
		UpdatedBy:     2,
		ActorUsername: "alice",
		ActorRole:     "user",
	}); err == nil || err.Error() != "active ticket requires assignee" {
		t.Fatalf("UpdateTicket(status unassigned) error = %v, want active ticket requires assignee", err)
	}
}

func TestUpdateTicketStatusAllowsAdminBypass(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Admin-bypass task",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	updated, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "alice",
		Stage:         StageDevelop,
		State:         StateActive,
		UpdatedBy:     1,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(admin lifecycle bypass) error = %v", err)
	}
	if updated.Status != "develop/active" {
		t.Fatalf("UpdateTicket(admin lifecycle bypass).Status = %q, want develop/active", updated.Status)
	}
}

func TestClosedTaskCannotBeReopened(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Closed task",
		Assignee:  "alice",
		Stage:     StageDone,
		State:     StateComplete,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:         task.Title,
		Description:   task.Description,
		ParentID:      task.ParentID,
		Assignee:      "alice",
		Stage:         StageDevelop,
		State:         StateIdle,
		UpdatedBy:     2,
		ActorUsername: "alice",
		ActorRole:     "user",
	}); err == nil || err.Error() != "done ticket cannot be reopened" {
		t.Fatalf("UpdateTicket(reopen) error = %v", err)
	}
}

func TestCloneTicketClonesSingleTask(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID:          project.ID,
		Type:               "task",
		Title:              "Original task",
		Description:        "desc",
		AcceptanceCriteria: "ac",
		Assignee:           "alice",
		Stage:              StageDevelop,
		State:              StateActive,
		Priority:           3,
		CreatedBy:          1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	cloned, err := CloneTicket(db, task.ID, 1)
	if err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if cloned.ID == task.ID || cloned.Status != "design/idle" || cloned.Assignee != "" {
		t.Fatalf("CloneTicket() = %#v", cloned)
	}
	if cloned.CloneOf == nil || *cloned.CloneOf != task.ID {
		t.Fatalf("CloneTicket().CloneOf = %#v, want %d", cloned.CloneOf, task.ID)
	}
}

func TestDeleteTicketDeletesTaskAndRelatedRows(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	task, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Delete me",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	clone, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		CloneOf:   &task.ID,
		Type:      "task",
		Title:     "Clone stays",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(clone) error = %v", err)
	}
	if _, err := AddComment(db, task.ID, 1, "hello"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if err := AddHistoryEvent(db, project.ID, task.ID, "task_updated", map[string]any{"title": task.Title}, 1); err != nil {
		t.Fatalf("AddHistoryEvent() error = %v", err)
	}
	dependency, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Dependency",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(dependency) error = %v", err)
	}
	if _, err := AddDependency(db, project.ID, task.ID, dependency.ID, 1); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	if err := DeleteTicket(db, task.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := GetTicket(db, task.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}

	clonedTask, err := GetTicket(db, clone.ID)
	if err != nil {
		t.Fatalf("GetTicket(clone) error = %v", err)
	}
	if clonedTask.CloneOf != nil {
		t.Fatalf("CloneOf = %#v, want nil after source delete", clonedTask.CloneOf)
	}
	if comments, err := ListComments(db, task.ID); err != nil || len(comments) != 0 {
		t.Fatalf("ListComments(deleted) = %#v, %v", comments, err)
	}
	if history, err := ListHistoryEvents(db, task.ID); err != nil || len(history) != 0 {
		t.Fatalf("ListHistoryEvents(deleted) = %#v, %v", history, err)
	}
	if deps, err := ListDependencies(db, task.ID); err != nil || len(deps) != 0 {
		t.Fatalf("ListDependencies(deleted) = %#v, %v", deps, err)
	}
}

func TestDeleteTicketFailsWhenTaskHasChildren(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	parent, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Parent",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	if _, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parent.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: 1,
	}); err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	if err := DeleteTicket(db, parent.ID); !errors.Is(err, ErrTicketHasChildren) {
		t.Fatalf("DeleteTicket(parent) error = %v, want ErrTicketHasChildren", err)
	}
}

func TestCloneEpicClonesChildren(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	epic, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Epic",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	child, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}
	clonedEpic, err := CloneTicket(db, epic.ID, 1)
	if err != nil {
		t.Fatalf("CloneTicket(epic) error = %v", err)
	}
	tasks, err := ListTicketsByProject(db, project.ID)
	if err != nil {
		t.Fatalf("ListTicketsByProject() error = %v", err)
	}
	var clonedChild Ticket
	var found bool
	for _, task := range tasks {
		if task.CloneOf != nil && *task.CloneOf == child.ID {
			clonedChild = task
			found = true
		}
	}
	if !found {
		t.Fatalf("cloned child not found in %#v", tasks)
	}
	if clonedChild.ParentID == nil || *clonedChild.ParentID != clonedEpic.ID {
		t.Fatalf("cloned child parent = %#v, want %d", clonedChild.ParentID, clonedEpic.ID)
	}
}

func TestParentLifecycleRecalculatesRecursivelyAndWritesDerivedHistory(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(db, "Customer Portal", "", "", 1)
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	epic, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Epic",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	parentTask, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Parent Task",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(parentTask) error = %v", err)
	}
	leafBug, err := CreateTicket(db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parentTask.ID,
		Type:      "bug",
		Title:     "Leaf Bug",
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("CreateTicket(leafBug) error = %v", err)
	}

	updatedLeaf, err := UpdateTicket(db, leafBug.ID, TicketUpdateParams{
		Title:         leafBug.Title,
		Description:   leafBug.Description,
		ParentID:      leafBug.ParentID,
		Assignee:      "alice",
		Stage:         StageDevelop,
		State:         StateActive,
		UpdatedBy:     1,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(leaf to develop/active) error = %v", err)
	}
	if updatedLeaf.Status != "develop/active" {
		t.Fatalf("leaf status = %q, want develop/active", updatedLeaf.Status)
	}

	reloadedParent, err := GetTicket(db, parentTask.ID)
	if err != nil {
		t.Fatalf("GetTicket(parentTask) error = %v", err)
	}
	if reloadedParent.Status != "develop/active" {
		t.Fatalf("parent task status = %q, want develop/active", reloadedParent.Status)
	}
	reloadedEpic, err := GetTicket(db, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(epic) error = %v", err)
	}
	if reloadedEpic.Status != "develop/active" {
		t.Fatalf("epic status = %q, want develop/active", reloadedEpic.Status)
	}

	if _, err := UpdateTicket(db, leafBug.ID, TicketUpdateParams{
		Title:         leafBug.Title,
		Description:   leafBug.Description,
		ParentID:      leafBug.ParentID,
		Assignee:      "alice",
		Stage:         StageDone,
		State:         StateComplete,
		UpdatedBy:     1,
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err != nil {
		t.Fatalf("UpdateTicket(leaf to done/complete) error = %v", err)
	}

	reloadedParent, err = GetTicket(db, parentTask.ID)
	if err != nil {
		t.Fatalf("GetTicket(parentTask after complete) error = %v", err)
	}
	if reloadedParent.Status != "done/complete" {
		t.Fatalf("parent task status after complete = %q, want done/complete", reloadedParent.Status)
	}
	reloadedEpic, err = GetTicket(db, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(epic after complete) error = %v", err)
	}
	if reloadedEpic.Status != "done/complete" {
		t.Fatalf("epic status after complete = %q, want done/complete", reloadedEpic.Status)
	}

	assertDerivedLifecycleHistory(t, db, parentTask.ID, [][2]string{
		{"design/idle", "develop/active"},
		{"develop/active", "done/complete"},
	})
	assertDerivedLifecycleHistory(t, db, epic.ID, [][2]string{
		{"design/idle", "develop/active"},
		{"develop/active", "done/complete"},
	})
}

func assertDerivedLifecycleHistory(t *testing.T, db *sql.DB, taskID int64, wantTransitions [][2]string) {
	t.Helper()

	events, err := ListHistoryEvents(db, taskID)
	if err != nil {
		t.Fatalf("ListHistoryEvents(%d) error = %v", taskID, err)
	}

	var derivedTransitions [][2]string
	for _, event := range events {
		if event.EventType != "ticket_parent_lifecycle_changed" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", event.Payload, err)
		}
		derivedTransitions = append(derivedTransitions, [2]string{
			payload["from_status"].(string),
			payload["to_status"].(string),
		})
	}
	if len(derivedTransitions) != len(wantTransitions) {
		t.Fatalf("derived transitions for %d = %#v, want %#v", taskID, derivedTransitions, wantTransitions)
	}
	for i := range wantTransitions {
		if derivedTransitions[i] != wantTransitions[i] {
			t.Fatalf("derived transitions for %d = %#v, want %#v", taskID, derivedTransitions, wantTransitions)
		}
	}
}
