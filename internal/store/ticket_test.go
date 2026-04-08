package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

func TestCreateUpdateAndListTickets(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Authentication",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}

	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID:        project.ID,
		ParentID:         &epic.ID,
		Type:             "task",
		Title:            "Add password reset",
		EstimateEffort:   5,
		EstimateComplete: "2026-04-01T12:00:00Z",
		CreatedBy:        "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	if ticket.ParentID == nil || *ticket.ParentID != epic.ID {
		t.Fatalf("CreateTicket().ParentID = %#v, want %s", ticket.ParentID, epic.ID)
	}
	if ticket.Stage != StageDesign || ticket.State != StateIdle {
		t.Fatalf("CreateTicket().Lifecycle = %s/%s, want design/idle", ticket.Stage, ticket.State)
	}
	if ticket.EstimateEffort != 5 || ticket.EstimateComplete != "2026-04-01T12:00:00Z" {
		t.Fatalf("CreateTicket() estimates = %#v", ticket)
	}

	tickets, err := ListTicketsByProject(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListTicketsByProject() error = %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("ListTicketsByProject() len = %d, want 2", len(tickets))
	}

	updated, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
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

	history, err := ListHistoryEvents(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	for _, event := range history {
		if event.EventType == "ticket_lifecycle_changed" {
			t.Fatalf("ticket_lifecycle_changed unexpectedly present for title-only update: %+v", event)
		}
	}

	statusUpdated, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:            updated.Title,
		Description:      updated.Description,
		ParentID:         updated.ParentID,
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
	if statusUpdated.Status != "design/active" {
		t.Fatalf("UpdateTicket().Status = %q, want design/active", statusUpdated.Status)
	}
	if statusUpdated.Stage != StageDesign || statusUpdated.State != StateActive {
		t.Fatalf("UpdateTicket().Lifecycle = %s/%s, want design/active", statusUpdated.Stage, statusUpdated.State)
	}

	history, err = ListHistoryEvents(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	var transitions [][2]string
	var reasons []string
	for _, event := range history {
		if event.EventType != "ticket_lifecycle_changed" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", event.Payload, err)
		}
		fromStatus, ok := payload["from_status"].(string)
		if !ok {
			t.Fatalf("history event missing from_status: %#v", payload)
		}
		toStatus, ok := payload["to_status"].(string)
		if !ok {
			t.Fatalf("history event missing to_status: %#v", payload)
		}
		transitions = append(transitions, [2]string{fromStatus, toStatus})
		reason, ok := payload["reason"].(string)
		if !ok {
			t.Fatalf("history event missing reason: %#v", payload)
		}
		reasons = append(reasons, reason)
	}
	if len(transitions) != 1 {
		t.Fatalf("ticket lifecycle transitions = %#v, want [[\"design/idle\", \"design/active\"]]", transitions)
	}
	if transitions[0] != ([2]string{"design/idle", "design/active"}) {
		t.Fatalf("ticket lifecycle transition = %#v, want [\"design/idle\" \"design/active\"]", transitions[0])
	}
	if len(reasons) != 1 || reasons[0] != "manual update" {
		t.Fatalf("ticket lifecycle reason = %#v, want [\"manual update\"]", reasons)
	}

	filtered, err := ListTickets(context.Background(), db, TicketListParams{
		ProjectID: project.ID,
		Type:      "task",
		Status:    "design/active",
		Search:    "password",
	})
	if err != nil {
		t.Fatalf("ListTickets(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != ticket.ID {
		t.Fatalf("ListTickets(filtered) = %#v", filtered)
	}

	limited, err := ListTickets(context.Background(), db, TicketListParams{
		ProjectID: project.ID,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ListTickets(limited) error = %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("ListTickets(limited) len = %d, want 1", len(limited))
	}

	got, err := GetTicketByProject(context.Background(), db, project.ID, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByProject() error = %v", err)
	}
	if got.ID != ticket.ID {
		t.Fatalf("GetTicketByProject().ID = %s, want %s", got.ID, ticket.ID)
	}
}

func TestSetTicketHealth(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Health check",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	updated, err := SetTicketHealth(context.Background(), db, ticket.ID, 3)
	if err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
	if updated.HealthScore != 3 {
		t.Fatalf("SetTicketHealth() score = %d, want 3", updated.HealthScore)
	}
	reloaded, err := GetTicket(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if reloaded.HealthScore != 3 {
		t.Fatalf("GetTicket().HealthScore = %d, want 3", reloaded.HealthScore)
	}
	if _, err := SetTicketHealth(context.Background(), db, ticket.ID, 6); err == nil {
		t.Fatalf("SetTicketHealth(out of range) = nil, want error")
	}
	if _, err := SetTicketHealth(context.Background(), db, "9999", 1); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("SetTicketHealth(unknown task) error = %v, want %v", err, ErrTicketNotFound)
	}
}

func TestCreateOrUpdateTicketEnforcesEpicParentRules(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	taskParent, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Regular task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &taskParent.ID,
		Type:      "epic",
		Title:     "Invalid epic",
		CreatedBy: "",
	}); err == nil || err.Error() != "task cannot parent epic" {
		t.Fatalf("CreateTicket(epic with non-epic parent) error = %v, want task cannot parent epic", err)
	}

	epicParent, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Valid epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}

	taskChild, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epicParent.ID,
		Type:      "task",
		Title:     "Task child",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task child) error = %v", err)
	}

	_, err = UpdateTicket(context.Background(), db, epicParent.ID, TicketUpdateParams{
		Title:    "Valid epic",
		ParentID: &taskChild.ID,
	})
	if err == nil || err.Error() != "task cannot parent epic" {
		t.Fatalf("UpdateTicket(epic parented by task) error = %v, want task cannot parent epic", err)
	}
}

func TestRequestTicket(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	notReady, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Blocked setup",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(design/idle) error = %v", err)
	}
	// Mark first ticket as ready so it can be claimed.
	if _, err := SetTicketReady(context.Background(), db, notReady.ID, true, "admin", ""); err != nil {
		t.Fatalf("SetTicketReady() error = %v", err)
	}
	secondTicket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Open task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(develop/idle) error = %v", err)
	}
	// Mark second ticket as ready too.
	if _, err := SetTicketReady(context.Background(), db, secondTicket.ID, true, "admin", ""); err != nil {
		t.Fatalf("SetTicketReady() error = %v", err)
	}

	assigned, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "alice",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("RequestTicket(any) error = %v", err)
	}
	if status != "ASSIGNED" || assigned.ID != notReady.ID {
		t.Fatalf("RequestTicket(any) = %#v, %q", assigned, status)
	}

	assignedAgain, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "alice",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("RequestTicket(existing open) error = %v", err)
	}
	if status != "ASSIGNED" || assignedAgain.ID != notReady.ID {
		t.Fatalf("RequestTicket(existing open) = %#v, %q", assignedAgain, status)
	}

	inProgress, err := UpdateTicket(context.Background(), db, notReady.ID, TicketUpdateParams{
		Title:         assigned.Title,
		Description:   assigned.Description,
		ParentID:      assigned.ParentID,
		Assignee:      "alice",
		State:         StateActive,
		UpdatedBy:     "",
		ActorUsername: "alice",
		ActorRole:     "user",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(develop/active) error = %v", err)
	}

	requested, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		TicketID:  &notReady.ID,
		Username:  "alice",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("RequestTicket(existing inprogress) error = %v", err)
	}
	if status != "ASSIGNED" || requested.ID != inProgress.ID {
		t.Fatalf("RequestTicket(existing inprogress) = %#v, %q", requested, status)
	}

	if _, err := CreateUser(context.Background(), db, "bob", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	rejected, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		TicketID:  &notReady.ID,
		Username:  "bob",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("RequestTicket(rejected) error = %v", err)
	}
	if status != "REJECTED" || rejected.ID != "" {
		t.Fatalf("RequestTicket(rejected) = %#v, %q", rejected, status)
	}

	// Bob gets the remaining idle ticket (openTask, ID 2)
	bobAssigned, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "bob",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("RequestTicket(bob) error = %v", err)
	}
	if status != "ASSIGNED" {
		t.Fatalf("RequestTicket(bob) status = %q, want ASSIGNED", status)
	}
	// Now no idle tickets left — should get NO-WORK
	noWork, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "charlie",
		UserID:    "",
	})
	_ = bobAssigned
	if err != nil {
		t.Fatalf("RequestTicket(no-work) error = %v", err)
	}
	if status != "NO-WORK" || noWork.ID != "" {
		t.Fatalf("RequestTicket(no-work) = %#v, %q", noWork, status)
	}
}

func TestRequestTicketDryRun(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "DryRun Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "DryRun task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := SetTicketReady(context.Background(), db, ticket.ID, true, "admin", ""); err != nil {
		t.Fatalf("SetTicketReady() error = %v", err)
	}

	// DryRun should return AVAILABLE without actually claiming
	preview, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		Username:  "alice",
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("RequestTicket(DryRun) error = %v", err)
	}
	if status != "AVAILABLE" {
		t.Fatalf("RequestTicket(DryRun) status = %q, want AVAILABLE", status)
	}
	if preview.Assignee != "alice" {
		t.Fatalf("RequestTicket(DryRun).Assignee = %q, want alice", preview.Assignee)
	}
	if preview.State != StateActive {
		t.Fatalf("RequestTicket(DryRun).State = %q, want active", preview.State)
	}

}

func TestRequestTicketByRef(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Ref Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Ref task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Request by TicketRef (resolved via GetTicketByRef -> GetTicket)
	// The ticket is not claimable (wrong stage), so this will be REJECTED
	_, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		TicketRef: ticket.ID,
		Username:  "alice",
	})
	if err != nil {
		t.Fatalf("RequestTicket(TicketRef) error = %v", err)
	}
	if status != "REJECTED" {
		t.Fatalf("RequestTicket(TicketRef non-claimable) status = %q, want REJECTED", status)
	}
}

func TestUpdateTicketAssignmentRulesForNonAdmin(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add password reset",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "bob", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}

	claimed, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
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

	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         claimed.Title,
		Description:   claimed.Description,
		ParentID:      claimed.ParentID,
		Assignee:      "bob",
		ActorUsername: "bob",
		ActorRole:     "user",
	}); err == nil || err.Error() != "ticket is already assigned to alice" {
		t.Fatalf("UpdateTicket(claim assigned) error = %v, want ticket is already assigned to alice", err)
	}

	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         claimed.Title,
		Description:   claimed.Description,
		ParentID:      claimed.ParentID,
		Assignee:      "",
		ActorUsername: "bob",
		ActorRole:     "user",
	}); err == nil || err.Error() != "ticket is assigned to alice" {
		t.Fatalf("UpdateTicket(unclaim other) error = %v, want ticket is assigned to alice", err)
	}
}

func TestUpdateTicketAssignRequiresExistingEnabledUser(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Add password reset",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      "nobody",
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err == nil || err.Error() != "user not found" {
		t.Fatalf("UpdateTicket(assign missing user) error = %v, want user not found", err)
	}
	if err := SetUserEnabled(context.Background(), db, "alice", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      "alice",
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err == nil || err.Error() != "user is disabled" {
		t.Fatalf("UpdateTicket(assign disabled user) error = %v, want user is disabled", err)
	}
}

func TestUpdateTicketStatusRequiresAssignee(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Status-owned task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      "",
		State:         StateActive,
		UpdatedBy:     "",
		ActorUsername: "alice",
		ActorRole:     "user",
	}); err == nil || err.Error() != "active ticket requires assignee" {
		t.Fatalf("UpdateTicket(status unassigned) error = %v, want active ticket requires assignee", err)
	}
}

func TestUpdateTicketStatusAllowsAdminBypass(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Admin-bypass task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	updated, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      "alice",
		State:         StateActive,
		UpdatedBy:     "",
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(admin lifecycle bypass) error = %v", err)
	}
	if updated.Status != "design/active" {
		t.Fatalf("UpdateTicket(admin lifecycle bypass).Status = %q, want design/active", updated.Status)
	}
}

func TestClosedTaskCannotBeReopened(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Closed task",
		Assignee:  "alice",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	// Advance through all workflow stages by setting state=success repeatedly.
	// Each success auto-advances to the next stage with state=idle.
	// When we reach the final stage, success stays.
	current := ticket
	for {
		updated, err := UpdateTicket(context.Background(), db, current.ID, TicketUpdateParams{
			Title:         current.Title,
			Description:   current.Description,
			ParentID:      current.ParentID,
			Assignee:      "alice",
			State:         StateSuccess,
			UpdatedBy:     "",
			ActorUsername: "alice",
			ActorRole:     "admin",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(advance) error = %v", err)
		}
		if updated.State == StateSuccess {
			// Reached final stage
			current = updated
			break
		}
		current = updated
	}
	// Now try to reopen — should fail
	if _, err := UpdateTicket(context.Background(), db, current.ID, TicketUpdateParams{
		Title:         current.Title,
		Description:   current.Description,
		ParentID:      current.ParentID,
		Assignee:      "alice",
		State:         StateIdle,
		UpdatedBy:     "",
		ActorUsername: "alice",
		ActorRole:     "user",
	}); err == nil || err.Error() != "done ticket cannot be reopened" {
		t.Fatalf("UpdateTicket(reopen) error = %v", err)
	}
}

func TestCloneTicketClonesSingleTask(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID:          project.ID,
		Type:               "task",
		Title:              "Original task",
		Description:        "desc",
		AcceptanceCriteria: "ac",
		Assignee:           "alice",
		State:              StateActive,
		Priority:           3,
		CreatedBy:          "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	cloned, err := CloneTicket(context.Background(), db, ticket.ID, "", "")
	if err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if cloned.ID == ticket.ID || cloned.Status != "design/idle" || cloned.Assignee != "" {
		t.Fatalf("CloneTicket() = %#v", cloned)
	}
	if cloned.CloneOf == nil || *cloned.CloneOf != ticket.ID {
		t.Fatalf("CloneTicket().CloneOf = %#v, want %s", cloned.CloneOf, ticket.ID)
	}
}

func TestDeleteTicketDeletesTaskAndRelatedRows(t *testing.T) {
	db := testDB(t)
	adminID := testAdminID(t, db)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Delete me",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	clone, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		CloneOf:   &ticket.ID,
		Type:      "task",
		Title:     "Clone stays",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(clone) error = %v", err)
	}
	if _, err := AddComment(context.Background(), db, ticket.ID, adminID, "hello"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if err := AddHistoryEvent(context.Background(), db, project.ID, ticket.ID, "task_updated", map[string]any{"title": ticket.Title}, ""); err != nil {
		t.Fatalf("AddHistoryEvent() error = %v", err)
	}
	dependency, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Dependency",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(dependency) error = %v", err)
	}
	if _, err := AddDependency(context.Background(), db, project.ID, ticket.ID, dependency.ID, ""); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	if err := DeleteTicket(context.Background(), db, ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := GetTicket(context.Background(), db, ticket.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}

	clonedTask, err := GetTicket(context.Background(), db, clone.ID)
	if err != nil {
		t.Fatalf("GetTicket(clone) error = %v", err)
	}
	if clonedTask.CloneOf != nil {
		t.Fatalf("CloneOf = %#v, want nil after source delete", clonedTask.CloneOf)
	}
	if comments, err := ListComments(context.Background(), db, ticket.ID); err != nil || len(comments) != 0 {
		t.Fatalf("ListComments(deleted) = %#v, %v", comments, err)
	}
	if history, err := ListHistoryEvents(context.Background(), db, ticket.ID); err != nil || len(history) != 0 {
		t.Fatalf("ListHistoryEvents(deleted) = %#v, %v", history, err)
	}
	if deps, err := ListDependencies(context.Background(), db, ticket.ID); err != nil || len(deps) != 0 {
		t.Fatalf("ListDependencies(deleted) = %#v, %v", deps, err)
	}
}

func TestDeleteTicketFailsWhenTaskHasChildren(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	parent, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Parent",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parent.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	if err := DeleteTicket(context.Background(), db, parent.ID); !errors.Is(err, ErrTicketHasChildren) {
		t.Fatalf("DeleteTicket(parent) error = %v, want ErrTicketHasChildren", err)
	}
}

func TestCloneEpicClonesChildren(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	child, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}
	clonedEpic, err := CloneTicket(context.Background(), db, epic.ID, "", "")
	if err != nil {
		t.Fatalf("CloneTicket(epic) error = %v", err)
	}
	tickets, err := ListTicketsByProject(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListTicketsByProject() error = %v", err)
	}
	var clonedChild Ticket
	var found bool
	for _, ticket := range tickets {
		if ticket.CloneOf != nil && *ticket.CloneOf == child.ID {
			clonedChild = ticket
			found = true
		}
	}
	if !found {
		t.Fatalf("cloned child not found in %#v", tickets)
	}
	if clonedChild.ParentID == nil || *clonedChild.ParentID != clonedEpic.ID {
		t.Fatalf("cloned child parent = %#v, want %s", clonedChild.ParentID, clonedEpic.ID)
	}
}

func TestParentLifecycleRecalculatesRecursivelyAndWritesDerivedHistory(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	parentTask, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Parent Task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parentTask) error = %v", err)
	}
	leafBug, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parentTask.ID,
		Type:      "bug",
		Title:     "Leaf Bug",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(leafBug) error = %v", err)
	}

	updatedLeaf, err := UpdateTicket(context.Background(), db, leafBug.ID, TicketUpdateParams{
		Title:         leafBug.Title,
		Description:   leafBug.Description,
		ParentID:      leafBug.ParentID,
		Assignee:      "alice",
		State:         StateActive,
		UpdatedBy:     "",
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(leaf to develop/active) error = %v", err)
	}
	if updatedLeaf.Status != "design/active" {
		t.Fatalf("leaf status = %q, want design/active", updatedLeaf.Status)
	}

	reloadedParent, err := GetTicket(context.Background(), db, parentTask.ID)
	if err != nil {
		t.Fatalf("GetTicket(parentTask) error = %v", err)
	}
	if reloadedParent.Status != "design/active" {
		t.Fatalf("parent task status = %q, want design/active", reloadedParent.Status)
	}
	reloadedEpic, err := GetTicket(context.Background(), db, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(epic) error = %v", err)
	}
	if reloadedEpic.Status != "design/active" {
		t.Fatalf("epic status = %q, want design/active", reloadedEpic.Status)
	}

	// Advance leaf through all remaining stages by setting success repeatedly
	currentLeaf, _ := GetTicket(context.Background(), db, leafBug.ID)
	for currentLeaf.State != StateSuccess {
		currentLeaf, err = UpdateTicket(context.Background(), db, currentLeaf.ID, TicketUpdateParams{
			Title:         currentLeaf.Title,
			Description:   currentLeaf.Description,
			ParentID:      currentLeaf.ParentID,
			Assignee:      "alice",
			State:         StateSuccess,
			UpdatedBy:     "",
			ActorUsername: "admin",
			ActorRole:     "admin",
		})
		if err != nil {
			t.Fatalf("UpdateTicket(leaf advance) error = %v", err)
		}
	}

	reloadedParent, err = GetTicket(context.Background(), db, parentTask.ID)
	if err != nil {
		t.Fatalf("GetTicket(parentTask after complete) error = %v", err)
	}
	if reloadedParent.Status != "done/success" {
		t.Fatalf("parent task status after complete = %q, want done/success", reloadedParent.Status)
	}
	reloadedEpic, err = GetTicket(context.Background(), db, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(epic after complete) error = %v", err)
	}
	if reloadedEpic.Status != "done/success" {
		t.Fatalf("epic status after complete = %q, want done/success", reloadedEpic.Status)
	}
}

func TestSetTicketOpenAndArchived(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Open/Archive Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Toggle ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Close ticket
	closed, err := SetTicketOpen(context.Background(), db, ticket.ID, false, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketOpen(false) error = %v", err)
	}
	if closed.Open {
		t.Fatal("SetTicketOpen(false).Open = true, want false")
	}

	// Reopen ticket
	reopened, err := SetTicketOpen(context.Background(), db, ticket.ID, true, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketOpen(true) error = %v", err)
	}
	if !reopened.Open {
		t.Fatal("SetTicketOpen(true).Open = false, want true")
	}

	// Idempotent: already open
	same, err := SetTicketOpen(context.Background(), db, ticket.ID, true, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketOpen(noop) error = %v", err)
	}
	if same.ID != ticket.ID {
		t.Fatalf("SetTicketOpen(noop) ID mismatch")
	}

	// Archive ticket
	archived, err := SetTicketArchived(context.Background(), db, ticket.ID, true, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketArchived(true) error = %v", err)
	}
	if !archived.Archived {
		t.Fatal("SetTicketArchived(true).Archived = false, want true")
	}

	// Unarchive ticket
	unarchived, err := SetTicketArchived(context.Background(), db, ticket.ID, false, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketArchived(false) error = %v", err)
	}
	if unarchived.Archived {
		t.Fatal("SetTicketArchived(false).Archived = true, want false")
	}
}

func TestGetTicketByRefAndSearchTickets(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Search Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Find me please",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// GetTicketByRef
	found, err := GetTicketByRef(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicketByRef() error = %v", err)
	}
	if found.ID != ticket.ID {
		t.Fatalf("GetTicketByRef().ID = %q, want %q", found.ID, ticket.ID)
	}

	// GetTicketByRef with empty string
	if _, err := GetTicketByRef(context.Background(), db, ""); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("GetTicketByRef(empty) error = %v, want ErrTicketNotFound", err)
	}

	// SearchTickets
	results, err := SearchTickets(context.Background(), db, project.ID, "Find me")
	if err != nil {
		t.Fatalf("SearchTickets() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != ticket.ID {
		t.Fatalf("SearchTickets() = %#v, want 1 result with ID %q", results, ticket.ID)
	}
}

func TestListTicketParents(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Parents Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Grand Epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}
	task, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Child Task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	bug, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &task.ID,
		Type:      "bug",
		Title:     "Leaf Bug",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(bug) error = %v", err)
	}

	parents, err := ListTicketParents(context.Background(), db, bug.ID)
	if err != nil {
		t.Fatalf("ListTicketParents() error = %v", err)
	}
	if len(parents) != 2 {
		t.Fatalf("ListTicketParents() len = %d, want 2", len(parents))
	}
	if parents[0].ID != task.ID || parents[1].ID != epic.ID {
		t.Fatalf("ListTicketParents() = [%s, %s], want [%s, %s]", parents[0].ID, parents[1].ID, task.ID, epic.ID)
	}
}

func TestCurrentAssignedTicketForUser(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Assign Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}

	// No assigned ticket yet
	_, found, err := CurrentAssignedTicketForUser(context.Background(), db, project.ID, "alice")
	if err != nil {
		t.Fatalf("CurrentAssignedTicketForUser() error = %v", err)
	}
	if found {
		t.Fatal("CurrentAssignedTicketForUser() found = true, want false")
	}

	// Create and assign a ticket
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Assigned task",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      "alice",
		State:         StateActive,
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err != nil {
		t.Fatalf("UpdateTicket(assign) error = %v", err)
	}

	assigned, found, err := CurrentAssignedTicketForUser(context.Background(), db, project.ID, "alice")
	if err != nil {
		t.Fatalf("CurrentAssignedTicketForUser() error = %v", err)
	}
	if !found {
		t.Fatal("CurrentAssignedTicketForUser() found = false, want true")
	}
	if assigned.ID != ticket.ID {
		t.Fatalf("CurrentAssignedTicketForUser().ID = %q, want %q", assigned.ID, ticket.ID)
	}
}

func TestExplainNoWork(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "NoWork Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create various ticket states to get better coverage of all code paths
	if _, err := CreateUser(context.Background(), db, "someone", "password123", "user"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	// Idle unassigned not-ready ticket
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Not ready",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket(not ready) error = %v", err)
	}
	// Idle assigned ticket
	assigned, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Assigned idle",
		Assignee:  "someone",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(assigned) error = %v", err)
	}
	_ = assigned
	// Closed ticket
	closed, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Closed one",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(closed) error = %v", err)
	}
	if _, err := SetTicketOpen(context.Background(), db, closed.ID, false, "admin", ""); err != nil {
		t.Fatalf("SetTicketOpen(false) error = %v", err)
	}
	// Parent ticket with children (non-leaf)
	parent, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Parent",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parent.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	reasons, err := ExplainNoWork(context.Background(), db, project.ID, "alice")
	if err != nil {
		t.Fatalf("ExplainNoWork() error = %v", err)
	}
	if len(reasons) < 3 {
		t.Fatalf("ExplainNoWork() returned %d reasons, want >= 3", len(reasons))
	}
}

func TestSetAndUnsetTicketWorkflow(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Workflow Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	wfBase, err := CreateWorkflow(context.Background(), db, "Custom Flow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := AddWorkflowStage(context.Background(), db, wfBase.ID, "Review", "", nil, 1); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	wf, err := GetWorkflow(context.Background(), db, wfBase.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}

	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Workflow ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Set workflow
	updated, err := SetTicketWorkflow(context.Background(), db, ticket.ID, wf.ID)
	if err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if updated.WorkflowID == nil || *updated.WorkflowID != wf.ID {
		t.Fatalf("SetTicketWorkflow().WorkflowID = %v, want %d", updated.WorkflowID, wf.ID)
	}

	// Unset workflow
	unset, err := UnsetTicketWorkflow(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}
	if unset.WorkflowID != nil {
		t.Fatalf("UnsetTicketWorkflow().WorkflowID = %v, want nil", unset.WorkflowID)
	}
}

func TestResolveWorkflowIDAndEnrichTicketContext(t *testing.T) {
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Context Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Create a workflow and set it on a parent ticket
	wfBase, err := CreateWorkflow(context.Background(), db, "Context WF", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := AddWorkflowStage(context.Background(), db, wfBase.ID, "step1", "", nil, 0); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}

	epic, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Context epic",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(epic) error = %v", err)
	}

	// Set workflow on the epic
	epic, err = SetTicketWorkflow(context.Background(), db, epic.ID, wfBase.ID)
	if err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}

	// Create child ticket under the epic (no workflow set directly)
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &epic.ID,
		Type:      "task",
		Title:     "Context ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// ResolveWorkflowID should inherit from parent
	wfID := ResolveWorkflowID(context.Background(), db, ticket)
	if wfID == nil {
		t.Fatal("ResolveWorkflowID() = nil, want inherited from parent")
	}
	if *wfID != wfBase.ID {
		t.Fatalf("ResolveWorkflowID() = %d, want %d", *wfID, wfBase.ID)
	}

	// EnrichTicketContext
	ctx := EnrichTicketContext(context.Background(), db, ticket)
	if ctx.Project == nil {
		t.Fatal("EnrichTicketContext().Project = nil, want non-nil")
	}
	if ctx.Project.ID != project.ID {
		t.Fatalf("EnrichTicketContext().Project.ID = %d, want %d", ctx.Project.ID, project.ID)
	}
	if ctx.Workflow == nil {
		t.Fatal("EnrichTicketContext().Workflow = nil, want non-nil")
	}
	if len(ctx.Parents) == 0 {
		t.Fatal("EnrichTicketContext().Parents = empty, want non-empty")
	}
}

func assertDerivedLifecycleHistory(t *testing.T, db *sql.DB, taskID string, wantTransitions [][2]string) {
	t.Helper()

	events, err := ListHistoryEvents(context.Background(), db, taskID)
	if err != nil {
		t.Fatalf("ListHistoryEvents(%s) error = %v", taskID, err)
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
		t.Fatalf("derived transitions for %s = %#v, want %#v", taskID, derivedTransitions, wantTransitions)
	}
	for i := range wantTransitions {
		if derivedTransitions[i] != wantTransitions[i] {
			t.Fatalf("derived transitions for %s = %#v, want %#v", taskID, derivedTransitions, wantTransitions)
		}
	}
}
