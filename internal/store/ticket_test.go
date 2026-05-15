package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCreateUpdateAndListTickets(t *testing.T) {
	t.Parallel()
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
	if ticket.Stage != StageDevelop || ticket.State != StateIdle {
		t.Fatalf("CreateTicket().Lifecycle = %s/%s, want develop/idle", ticket.Stage, ticket.State)
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

	history, err := ListHistoryEvents(context.Background(), db, ticket.ID, 0, 0)
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
	if statusUpdated.Status != "develop/active" {
		t.Fatalf("UpdateTicket().Status = %q, want design/active", statusUpdated.Status)
	}
	if statusUpdated.Stage != StageDevelop || statusUpdated.State != StateActive {
		t.Fatalf("UpdateTicket().Lifecycle = %s/%s, want develop/active", statusUpdated.Stage, statusUpdated.State)
	}
	workItems, err := ListWorkItemsByTicket(context.Background(), db, ticket.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListWorkItemsByTicket(active) error = %v", err)
	}
	if len(workItems) != 1 {
		t.Fatalf("ListWorkItemsByTicket(active) len = %d, want 1", len(workItems))
	}
	if workItems[0].Status != WorkItemStatusActive {
		t.Fatalf("active work item status = %q, want %q", workItems[0].Status, WorkItemStatusActive)
	}

	history, err = ListHistoryEvents(context.Background(), db, ticket.ID, 0, 0)
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
	if transitions[0] != ([2]string{"develop/idle", "develop/active"}) {
		t.Fatalf("ticket lifecycle transition = %#v, want [\"design/idle\" \"design/active\"]", transitions[0])
	}
	if len(reasons) != 1 || reasons[0] != "manual update" {
		t.Fatalf("ticket lifecycle reason = %#v, want [\"manual update\"]", reasons)
	}

	filtered, err := ListTickets(context.Background(), db, TicketListParams{
		ProjectID: project.ID,
		Type:      "task",
		Status:    "develop/active",
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
	offset, err := ListTickets(context.Background(), db, TicketListParams{
		ProjectID: project.ID,
		Limit:     1,
		Offset:    1,
	})
	if err != nil {
		t.Fatalf("ListTickets(offset) error = %v", err)
	}
	if len(offset) != 1 {
		t.Fatalf("ListTickets(offset) len = %d, want 1", len(offset))
	}
	if offset[0].ID == limited[0].ID {
		t.Fatalf("ListTickets(offset) returned %#v, want a different ticket than %#v", offset[0], limited[0])
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
	t.Parallel()
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
	if _, err := SetTicketHealth(context.Background(), db, ticket.ID, 11); err == nil {
		t.Fatalf("SetTicketHealth(out of range) = nil, want error")
	}
	if _, err := SetTicketHealth(context.Background(), db, "9999", 1); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("SetTicketHealth(unknown task) error = %v, want %v", err, ErrTicketNotFound)
	}
}

func TestListWorkItemsByTicketWithParamsFilters(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Work Item Filters", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Filter work items",
		Assignee:  "alice",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	active, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		Assignee:      ticket.Assignee,
		Priority:      ticket.Priority,
		Order:         ticket.Order,
		State:         StateActive,
		UpdatedBy:     alice.ID,
		ActorUsername: alice.Username,
		ActorRole:     alice.Role,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(active) error = %v", err)
	}
	if active.State != StateActive {
		t.Fatalf("active ticket state = %q, want %q", active.State, StateActive)
	}
	success, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         active.Title,
		Description:   active.Description,
		Assignee:      active.Assignee,
		Priority:      active.Priority,
		Order:         active.Order,
		State:         StateSuccess,
		UpdatedBy:     alice.ID,
		ActorUsername: alice.Username,
		ActorRole:     alice.Role,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(success) error = %v", err)
	}
	if success.State != StateIdle {
		t.Fatalf("success transition state = %q, want %q", success.State, StateIdle)
	}

	filteredSuccess, err := ListWorkItemsByTicketWithParams(context.Background(), db, ticket.ID, WorkItemListParams{
		Status: WorkItemStatusSuccess,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListWorkItemsByTicketWithParams(success) error = %v", err)
	}
	if len(filteredSuccess) != 1 || filteredSuccess[0].Status != WorkItemStatusSuccess {
		t.Fatalf("filtered success = %#v", filteredSuccess)
	}

	filteredHuman, err := ListWorkItemsByTicketWithParams(context.Background(), db, ticket.ID, WorkItemListParams{
		AssigneeType: "human",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("ListWorkItemsByTicketWithParams(human) error = %v", err)
	}
	if len(filteredHuman) != 1 {
		t.Fatalf("filtered human len = %d, want 1", len(filteredHuman))
	}

	filteredActive, err := ListWorkItemsByTicketWithParams(context.Background(), db, ticket.ID, WorkItemListParams{
		Status: WorkItemStatusActive,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListWorkItemsByTicketWithParams(active) error = %v", err)
	}
	if len(filteredActive) != 0 {
		t.Fatalf("filtered active len = %d, want 0", len(filteredActive))
	}

	if _, err := ListWorkItemsByTicketWithParams(context.Background(), db, ticket.ID, WorkItemListParams{Status: "bogus"}); err == nil {
		t.Fatalf("ListWorkItemsByTicketWithParams(invalid status) = nil, want error")
	}
	if _, err := ListWorkItemsByTicketWithParams(context.Background(), db, ticket.ID, WorkItemListParams{AssigneeType: "robot"}); err == nil {
		t.Fatalf("ListWorkItemsByTicketWithParams(invalid assignee type) = nil, want error")
	}
}

func TestWorkItemLifecycleActions(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Work Item Actions", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	bob, err := CreateUser(context.Background(), db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Exercise work-item actions",
		Assignee:  alice.Username,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		Assignee:      ticket.Assignee,
		Priority:      ticket.Priority,
		Order:         ticket.Order,
		State:         StateActive,
		UpdatedBy:     alice.ID,
		ActorUsername: alice.Username,
		ActorRole:     alice.Role,
	}); err != nil {
		t.Fatalf("UpdateTicket(active) error = %v", err)
	}
	items, err := ListWorkItemsByTicket(context.Background(), db, ticket.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListWorkItemsByTicket(active) error = %v", err)
	}
	if len(items) != 1 || items[0].Status != WorkItemStatusActive {
		t.Fatalf("active work items = %#v", items)
	}
	active := items[0]

	reassigned, err := ReassignWorkItem(context.Background(), db, ticket.ID, active.ID, bob.Username, "admin", "admin")
	if err != nil {
		t.Fatalf("ReassignWorkItem() error = %v", err)
	}
	if reassigned.AssigneeID != bob.ID {
		t.Fatalf("ReassignWorkItem().AssigneeID = %q, want %q", reassigned.AssigneeID, bob.ID)
	}
	feedbackUpdated, err := AddWorkItemFeedback(context.Background(), db, ticket.ID, active.ID, "investigated root cause", "abc123", "admin", "admin")
	if err != nil {
		t.Fatalf("AddWorkItemFeedback() error = %v", err)
	}
	if feedbackUpdated.CommitRef != "abc123" {
		t.Fatalf("AddWorkItemFeedback().CommitRef = %q, want %q", feedbackUpdated.CommitRef, "abc123")
	}
	if !strings.Contains(feedbackUpdated.Feedback, "investigated root cause") {
		t.Fatalf("AddWorkItemFeedback().Feedback = %q", feedbackUpdated.Feedback)
	}
	if _, err := AddWorkItemFeedback(context.Background(), db, ticket.ID, active.ID, "", "", "admin", "admin"); err == nil {
		t.Fatalf("AddWorkItemFeedback(empty payload) = nil, want error")
	}
	if _, err := ReassignWorkItem(context.Background(), db, ticket.ID, active.ID, "not-a-user", "admin", "admin"); err == nil {
		t.Fatalf("ReassignWorkItem(unknown assignee) = nil, want error")
	}
	if err := SetUserEnabled(context.Background(), db, "bob", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	if _, err := ReassignWorkItem(context.Background(), db, ticket.ID, active.ID, bob.Username, "admin", "admin"); err == nil {
		t.Fatalf("ReassignWorkItem(disabled assignee) = nil, want error")
	}
	if err := SetUserEnabled(context.Background(), db, "bob", true); err != nil {
		t.Fatalf("SetUserEnabled(true) error = %v", err)
	}
	cancelled, err := CancelWorkItem(context.Background(), db, ticket.ID, active.ID, "stopping work", "admin", "admin")
	if err != nil {
		t.Fatalf("CancelWorkItem() error = %v", err)
	}
	if cancelled.Status != WorkItemStatusStopped {
		t.Fatalf("CancelWorkItem().Status = %q, want %q", cancelled.Status, WorkItemStatusStopped)
	}
	retried, err := RetryWorkItem(context.Background(), db, ticket.ID, active.ID, bob.Username, "admin", "admin")
	if err != nil {
		t.Fatalf("RetryWorkItem() error = %v", err)
	}
	if retried.Status != WorkItemStatusActive || retried.ID == active.ID {
		t.Fatalf("RetryWorkItem() = %#v, want new active work item", retried)
	}
	if retried.AssigneeID != bob.ID {
		t.Fatalf("RetryWorkItem().AssigneeID = %q, want %q", retried.AssigneeID, bob.ID)
	}
	if _, err := RetryWorkItem(context.Background(), db, ticket.ID, active.ID, "", "admin", "admin"); err == nil {
		t.Fatalf("RetryWorkItem(with existing active) = nil, want error")
	}
}

func TestTicketGuidanceMapsPersistAndResolve(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID:          project.ID,
		Type:               "task",
		Title:              "Guidance ticket",
		AcceptanceCriteria: "legacy ticket ac",
		DORMap:             GuidanceMap{"default": "ticket default dor", "develop": "ticket develop dor"},
		DODMap:             GuidanceMap{"default": "ticket default dod"},
		ACMap:              GuidanceMap{"qa": "ticket qa ac"},
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if !reflect.DeepEqual(ticket.DORMap, GuidanceMap{"default": "ticket default dor", "develop": "ticket develop dor"}) {
		t.Fatalf("CreateTicket().DORMap = %#v", ticket.DORMap)
	}
	if !reflect.DeepEqual(ticket.ACMap, GuidanceMap{"default": "legacy ticket ac", "qa": "ticket qa ac"}) {
		t.Fatalf("CreateTicket().ACMap = %#v", ticket.ACMap)
	}

	reloaded, err := GetTicket(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	resolved := reloaded.ResolveGuidance("develop")
	if !resolved.HasDOR || resolved.DOR != "ticket develop dor" {
		t.Fatalf("ResolveGuidance(develop).DOR = %#v", resolved)
	}
	if !resolved.HasDOD || resolved.DOD != "ticket default dod" {
		t.Fatalf("ResolveGuidance(develop).DOD = %#v", resolved)
	}
	if !resolved.HasAC || resolved.AC != "legacy ticket ac" {
		t.Fatalf("ResolveGuidance(develop).AC = %#v", resolved)
	}

	updated, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:  ticket.Title,
		DORMap: GuidanceMap{"qa": "updated ticket dor"},
		DODMap: GuidanceMap{"qa": "updated ticket dod"},
		ACMap:  GuidanceMap{"qa": "updated ticket ac"},
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if !reflect.DeepEqual(updated.DODMap, GuidanceMap{"qa": "updated ticket dod"}) {
		t.Fatalf("UpdateTicket().DODMap = %#v", updated.DODMap)
	}
	if !reflect.DeepEqual(updated.ACMap, GuidanceMap{"default": "legacy ticket ac", "qa": "updated ticket ac"}) {
		t.Fatalf("UpdateTicket().ACMap = %#v", updated.ACMap)
	}
}

func TestCreateAndUpdateTicketAllowLineageIndependentOfType(t *testing.T) {
	t.Parallel()
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
	}); err != nil {
		t.Fatalf("CreateTicket(epic with task parent) error = %v, want nil", err)
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

	standaloneFeature, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "feature",
		Title:     "Standalone feature",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(standalone feature) error = %v", err)
	}

	updated, err := UpdateTicket(context.Background(), db, standaloneFeature.ID, TicketUpdateParams{
		Title:    standaloneFeature.Title,
		ParentID: &taskChild.ID,
	})
	if err != nil {
		t.Fatalf("UpdateTicket(feature parented by task) error = %v, want nil", err)
	}
	if updated.ParentID == nil || *updated.ParentID != taskChild.ID {
		t.Fatalf("UpdateTicket(feature parented by task).ParentID = %#v, want %q", updated.ParentID, taskChild.ID)
	}
}

func TestCreateTicketAcceptsCanonicalTypes(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Customer Portal", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	types := []string{
		"epic",
		"story",
		"task",
		"bug",
		"feature",
		"idea",
		"spike",
		"chore",
		"note",
		"question",
		"requirement",
		"decision",
		"action",
	}
	for _, ticketType := range types {
		ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
			ProjectID: project.ID,
			Type:      ticketType,
			Title:     "Ticket type " + ticketType,
			CreatedBy: "",
		})
		if err != nil {
			t.Fatalf("CreateTicket(%q) error = %v", ticketType, err)
		}
		if ticket.Type != ticketType {
			t.Fatalf("CreateTicket(%q).Type = %q, want %q", ticketType, ticket.Type, ticketType)
		}
	}
}

func TestRequestTicket(t *testing.T) {
	t.Parallel()
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
		t.Fatalf("CreateTicket() error = %v", err)
	}
	_, err = CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Open task",
		State:     StateIdle,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
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
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "DryRun Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "DryRun task",
		State:     StateIdle,
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
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
	t.Parallel()
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
	// The ticket is at develop/idle which is claimable.
	assigned, status, err := RequestTicket(context.Background(), db, TicketRequestParams{
		ProjectID: project.ID,
		TicketRef: ticket.ID,
		Username:  "alice",
	})
	if err != nil {
		t.Fatalf("RequestTicket(TicketRef) error = %v", err)
	}
	if status != "ASSIGNED" {
		t.Fatalf("RequestTicket(TicketRef) status = %q, want ASSIGNED", status)
	}
	if assigned.ID != ticket.ID {
		t.Fatalf("RequestTicket(TicketRef).ID = %q, want %q", assigned.ID, ticket.ID)
	}
}

func TestUpdateTicketAssignmentRulesForNonAdmin(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if updated.Status != "develop/active" {
		t.Fatalf("UpdateTicket(admin lifecycle bypass).Status = %q, want design/active", updated.Status)
	}
}

func TestClosedTaskCannotBeReopened(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	if cloned.ID == ticket.ID || cloned.Status != "develop/idle" || cloned.Assignee != "" {
		t.Fatalf("CloneTicket() = %#v", cloned)
	}
	if cloned.CloneOf == nil || *cloned.CloneOf != ticket.ID {
		t.Fatalf("CloneTicket().CloneOf = %#v, want %s", cloned.CloneOf, ticket.ID)
	}
}

func TestDeleteTicketDeletesTaskAndRelatedRows(t *testing.T) {
	t.Parallel()
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
	label, err := CreateLabel(context.Background(), db, project.ID, "priority", "#ff0000")
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if err := AddTicketLabel(context.Background(), db, ticket.ID, label.ID); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	if _, err := LogTime(context.Background(), db, ticket.ID, adminID, 30, "cleanup"); err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	story, err := CreateStory(context.Background(), db, project.ID, "Delete Story", "", "")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if err := LinkStoryToTicket(context.Background(), db, story.ID, ticket.ID); err != nil {
		t.Fatalf("LinkStoryToTicket() error = %v", err)
	}

	if err := DeleteTicket(context.Background(), db, ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := GetTicket(context.Background(), db, ticket.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
	if _, err := GetTicketByRef(context.Background(), db, ticket.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("GetTicketByRef(deleted) error = %v, want ErrTicketNotFound", err)
	}
	listed, err := ListTicketsByProject(context.Background(), db, project.ID)
	if err != nil {
		t.Fatalf("ListTicketsByProject() error = %v", err)
	}
	for _, listedTicket := range listed {
		if listedTicket.ID == ticket.ID {
			t.Fatalf("deleted ticket %s should not appear in project listing", ticket.ID)
		}
	}

	clonedTask, err := GetTicket(context.Background(), db, clone.ID)
	if err != nil {
		t.Fatalf("GetTicket(clone) error = %v", err)
	}
	if clonedTask.CloneOf == nil || *clonedTask.CloneOf != ticket.ID {
		t.Fatalf("CloneOf = %#v, want %q preserved after soft delete", clonedTask.CloneOf, ticket.ID)
	}
	var deleted int
	if err := db.QueryRowContext(context.Background(), `SELECT deleted FROM tickets WHERE ticket_id = ?`, ticket.ID).Scan(&deleted); err != nil {
		t.Fatalf("deleted flag query error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted flag = %d, want 1", deleted)
	}
	if comments, err := ListComments(context.Background(), db, ticket.ID); err != nil || len(comments) != 1 {
		t.Fatalf("ListComments(soft-deleted) = %#v, %v", comments, err)
	}
	if history, err := ListHistoryEvents(context.Background(), db, ticket.ID, 0, 0); err != nil || len(history) < 2 {
		t.Fatalf("ListHistoryEvents(soft-deleted) = %#v, %v", history, err)
	}
	if deps, err := ListDependencies(context.Background(), db, ticket.ID); err != nil || len(deps) != 1 {
		t.Fatalf("ListDependencies(soft-deleted) = %#v, %v", deps, err)
	}
	if labels, err := ListTicketLabels(context.Background(), db, ticket.ID); err != nil || len(labels) != 1 {
		t.Fatalf("ListTicketLabels(soft-deleted) = %#v, %v", labels, err)
	}
	if entries, err := ListTimeEntries(context.Background(), db, ticket.ID); err != nil || len(entries) != 1 {
		t.Fatalf("ListTimeEntries(soft-deleted) = %#v, %v", entries, err)
	}
	var storyLinks int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM story_ticket_links WHERE story_id = ?`, story.ID).Scan(&storyLinks); err != nil {
		t.Fatalf("story_ticket_links count query error = %v", err)
	}
	if storyLinks != 1 {
		t.Fatalf("story_ticket_links count = %d, want 1 after soft delete", storyLinks)
	}
}

func TestDeleteTicketFailsWhenTaskHasChildren(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if updatedLeaf.Status != "develop/active" {
		t.Fatalf("leaf status = %q, want design/active", updatedLeaf.Status)
	}

	reloadedParent, err := GetTicket(context.Background(), db, parentTask.ID)
	if err != nil {
		t.Fatalf("GetTicket(parentTask) error = %v", err)
	}
	if reloadedParent.Status != "develop/active" {
		t.Fatalf("parent task status = %q, want design/active", reloadedParent.Status)
	}
	reloadedEpic, err := GetTicket(context.Background(), db, epic.ID)
	if err != nil {
		t.Fatalf("GetTicket(epic) error = %v", err)
	}
	if reloadedEpic.Status != "develop/active" {
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

func TestSetTicketCompleteAndArchived(t *testing.T) {
	t.Parallel()
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

	// Complete ticket
	closed, err := SetTicketComplete(context.Background(), db, ticket.ID, true, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketComplete(true) error = %v", err)
	}
	if !closed.Complete {
		t.Fatal("SetTicketComplete(true) should set complete=true")
	}

	// Reopen ticket
	reopened, err := SetTicketComplete(context.Background(), db, ticket.ID, false, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketComplete(false) error = %v", err)
	}
	if reopened.Complete {
		t.Fatal("SetTicketComplete(false) should set complete=false")
	}

	// Idempotent: already not complete
	same, err := SetTicketComplete(context.Background(), db, ticket.ID, false, "admin", "")
	if err != nil {
		t.Fatalf("SetTicketComplete(noop) error = %v", err)
	}
	if same.ID != ticket.ID {
		t.Fatalf("SetTicketComplete(noop) ID mismatch")
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if _, err := SetTicketComplete(context.Background(), db, closed.ID, false, "admin", ""); err != nil {
		t.Fatalf("SetTicketComplete(false) error = %v", err)
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
	t.Parallel()
	db := testDB(t)
	project, err := CreateProject(context.Background(), db, "Workflow Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	wfBase, err := CreateWorkflow(context.Background(), db, "Custom Flow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := AddWorkflowStage(context.Background(), db, wfBase.ID, "Review", "", "", 1); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	reviewer, err := CreateRoleWithParams(context.Background(), db, RoleCreateParams{
		WorkflowID:  &wfBase.ID,
		Title:       "reviewer",
		Description: "reviews work",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	wf, err := GetWorkflow(context.Background(), db, wfBase.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(wf.Stages) == 0 {
		t.Fatal("expected workflow to have stages")
	}
	if err := AddWorkflowStageRole(context.Background(), db, wf.ID, wf.Stages[0].ID, reviewer.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
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
	if updated.RoleID == nil || *updated.RoleID != reviewer.ID {
		t.Fatalf("SetTicketWorkflow().RoleID = %#v, want %d", updated.RoleID, reviewer.ID)
	}

	// Unset workflow
	unset, err := UnsetTicketWorkflow(context.Background(), db, ticket.ID)
	if err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}
	if unset.WorkflowID != nil {
		t.Fatalf("UnsetTicketWorkflow().WorkflowID = %v, want nil", unset.WorkflowID)
	}
	if unset.RoleID != nil {
		t.Fatalf("UnsetTicketWorkflow().RoleID = %#v, want nil", unset.RoleID)
	}
}

func TestCreateTicketUsesFirstRoleFromProjectWorkflow(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	project, err := CreateProject(context.Background(), db, "Workflow Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	wf, err := CreateWorkflow(context.Background(), db, "Workflow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := AddWorkflowStage(context.Background(), db, wf.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	role, err := CreateRoleWithParams(context.Background(), db, RoleCreateParams{
		WorkflowID:  &wf.ID,
		Title:       "designer",
		Description: "designs work",
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := AddWorkflowStageRole(context.Background(), db, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}
	if _, err := UpdateProjectWithParams(context.Background(), db, project.ID, ProjectUpdateParams{
		Title:              project.Title,
		Description:        project.Description,
		AcceptanceCriteria: project.AcceptanceCriteria,
		WorkflowID:         &wf.ID,
	}); err != nil {
		t.Fatalf("UpdateProjectWithParams() error = %v", err)
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
	if ticket.WorkflowStageID == nil || *ticket.WorkflowStageID != stage.ID {
		t.Fatalf("ticket.WorkflowStageID = %#v, want %d", ticket.WorkflowStageID, stage.ID)
	}
	if ticket.RoleID == nil || *ticket.RoleID != role.ID {
		t.Fatalf("ticket.RoleID = %#v, want %d", ticket.RoleID, role.ID)
	}
}

func TestUpdateTicketSuccessAutoAdvancesToNextStageFirstRole(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Role Workflow Project", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	wf, err := CreateWorkflow(ctx, db, "Role Workflow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	designStage, err := AddWorkflowStage(ctx, db, wf.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := AddWorkflowStage(ctx, db, wf.ID, "test", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	designer, err := CreateRoleWithParams(ctx, db, RoleCreateParams{
		WorkflowID:  &wf.ID,
		Title:       "designer",
		Description: "designs work",
	})
	if err != nil {
		t.Fatalf("CreateRole(designer) error = %v", err)
	}
	tester, err := CreateRoleWithParams(ctx, db, RoleCreateParams{
		WorkflowID:  &wf.ID,
		Title:       "tester",
		Description: "tests work",
	})
	if err != nil {
		t.Fatalf("CreateRole(tester) error = %v", err)
	}
	if err := AddWorkflowStageRole(ctx, db, wf.ID, designStage.ID, designer.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(design) error = %v", err)
	}
	if err := AddWorkflowStageRole(ctx, db, wf.ID, testStage.ID, tester.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(test) error = %v", err)
	}
	if _, err := UpdateProjectWithParams(ctx, db, project.ID, ProjectUpdateParams{
		Title:              project.Title,
		Description:        project.Description,
		AcceptanceCriteria: project.AcceptanceCriteria,
		WorkflowID:         &wf.ID,
	}); err != nil {
		t.Fatalf("UpdateProjectWithParams() error = %v", err)
	}

	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Role workflow ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	updated, err := UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		State:         StateSuccess,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(success) error = %v", err)
	}
	if updated.Stage != "test" || updated.State != StateIdle || updated.RoleID == nil || *updated.RoleID != tester.ID || updated.WorkflowStageID == nil || *updated.WorkflowStageID != testStage.ID {
		t.Fatalf("updated ticket = %#v, want test/idle on tester role", updated)
	}
}

func TestResolveWorkflowIDAndEnrichTicketContext(t *testing.T) {
	t.Parallel()
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
	if _, err := AddWorkflowStage(context.Background(), db, wfBase.ID, "step1", "", "", 0); err != nil {
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

func TestSetTicketDraftAndWorkflowProgression(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	wf, err := CreateWorkflow(ctx, db, "Ticket Flow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	design, err := AddWorkflowStage(ctx, db, wf.ID, "design", "design work", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := AddWorkflowStage(ctx, db, wf.ID, "test", "test work", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	doneStage, err := AddWorkflowStage(ctx, db, wf.ID, "done", "done work", "", 2)
	if err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	designer, err := CreateRole(ctx, db, &wf.ID, "designer", "designs", "ready for test")
	if err != nil {
		t.Fatalf("CreateRole(designer) error = %v", err)
	}
	tester, err := CreateRole(ctx, db, &wf.ID, "tester", "tests", "ready for done")
	if err != nil {
		t.Fatalf("CreateRole(tester) error = %v", err)
	}
	if err := AddWorkflowStageRole(ctx, db, wf.ID, design.ID, designer.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(design) error = %v", err)
	}
	if err := AddWorkflowStageRole(ctx, db, wf.ID, testStage.ID, tester.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole(test) error = %v", err)
	}
	project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:     "WF",
		Title:      "Workflow Project",
		WorkflowID: &wf.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Workflow Ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	adminID := testAdminID(t, db)

	drafted, err := SetTicketDraft(ctx, db, ticket.ID, true, "admin", adminID)
	if err != nil {
		t.Fatalf("SetTicketDraft(true) error = %v", err)
	}
	if !drafted.Draft {
		t.Fatalf("Draft = %v, want true", drafted.Draft)
	}
	ready, err := SetTicketDraft(ctx, db, ticket.ID, false, "admin", adminID)
	if err != nil {
		t.Fatalf("SetTicketDraft(false) error = %v", err)
	}
	if ready.Draft {
		t.Fatalf("Draft = %v, want false", ready.Draft)
	}

	events, err := ListHistoryEvents(ctx, db, ticket.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListHistoryEvents() error = %v", err)
	}
	actions := map[string]bool{}
	for _, event := range events {
		actions[event.EventType] = true
	}
	if !actions["marked_draft"] || !actions["marked_ready"] {
		t.Fatalf("history actions = %#v, want marked_draft and marked_ready", actions)
	}

	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateSuccess, RenderLifecycleStatus(ticket.Stage, StateSuccess), ticket.ID); err != nil {
		t.Fatalf("set state success error = %v", err)
	}
	advanced, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket() error = %v", err)
	}
	if advanced.Stage != "test" || advanced.RoleID == nil || *advanced.RoleID != tester.ID || advanced.State != StateIdle {
		t.Fatalf("NextTicket() = %#v, want test stage with tester role idle", advanced)
	}

	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateFail, RenderLifecycleStatus(advanced.Stage, StateFail), ticket.ID); err != nil {
		t.Fatalf("set state fail error = %v", err)
	}
	regressed, err := PreviousTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("PreviousTicket() error = %v", err)
	}
	if regressed.Stage != "design" || regressed.RoleID == nil || *regressed.RoleID != designer.ID || regressed.State != StateIdle {
		t.Fatalf("PreviousTicket() = %#v, want design stage with designer role idle", regressed)
	}

	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateSuccess, RenderLifecycleStatus(regressed.Stage, StateSuccess), ticket.ID); err != nil {
		t.Fatalf("set design success error = %v", err)
	}
	if _, err := NextTicket(ctx, db, ticket.ID, "admin", adminID); err != nil {
		t.Fatalf("NextTicket(design->test) error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateSuccess, RenderLifecycleStatus(testStage.StageName, StateSuccess), ticket.ID); err != nil {
		t.Fatalf("set test success error = %v", err)
	}
	completed, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(test->done) error = %v", err)
	}
	if !completed.Complete || completed.Stage != "done" || completed.State != StateIdle || completed.WorkflowStageID == nil || *completed.WorkflowStageID != doneStage.ID {
		t.Fatalf("completed ticket = %#v, want complete done/idle on done stage", completed)
	}
}

func TestWorkflowProgressionModeAffectsSuccessAdvance(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	createTicketWithPolicy := func(prefix, title, progressionMode string) (Ticket, int64) {
		wf, err := CreateWorkflowWithOptions(ctx, db, nil, prefix+" flow", "", WorkflowApprovalPolicyAllRoles, progressionMode)
		if err != nil {
			t.Fatalf("CreateWorkflowWithOptions(%s) error = %v", progressionMode, err)
		}
		reviewStage, err := AddWorkflowStage(ctx, db, wf.ID, "review", "", "", 0)
		if err != nil {
			t.Fatalf("AddWorkflowStage(review) error = %v", err)
		}
		doneStage, err := AddWorkflowStage(ctx, db, wf.ID, "done", "", "", 1)
		if err != nil {
			t.Fatalf("AddWorkflowStage(done) error = %v", err)
		}
		roleA, err := CreateRole(ctx, db, &wf.ID, prefix+"-a", "", "")
		if err != nil {
			t.Fatalf("CreateRole(a) error = %v", err)
		}
		roleB, err := CreateRole(ctx, db, &wf.ID, prefix+"-b", "", "")
		if err != nil {
			t.Fatalf("CreateRole(b) error = %v", err)
		}
		if err := AddWorkflowStageRole(ctx, db, wf.ID, reviewStage.ID, roleA.ID); err != nil {
			t.Fatalf("AddWorkflowStageRole(a) error = %v", err)
		}
		if err := AddWorkflowStageRole(ctx, db, wf.ID, reviewStage.ID, roleB.ID); err != nil {
			t.Fatalf("AddWorkflowStageRole(b) error = %v", err)
		}
		project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
			Prefix:     prefix,
			Title:      prefix + " project",
			WorkflowID: &wf.ID,
		})
		if err != nil {
			t.Fatalf("CreateProjectWithParams() error = %v", err)
		}
		ticket, err := CreateTicket(ctx, db, TicketCreateParams{
			ProjectID: project.ID,
			Type:      "task",
			Title:     title,
			CreatedBy: "",
		})
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
		return ticket, doneStage.ID
	}

	linearTicket, _ := createTicketWithPolicy("LIN", "linear all roles ticket", WorkflowProgressionModeLinear)
	linearAdvanced, err := UpdateTicket(ctx, db, linearTicket.ID, TicketUpdateParams{
		Title:         linearTicket.Title,
		Description:   linearTicket.Description,
		ParentID:      linearTicket.ParentID,
		State:         StateSuccess,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(linear success) error = %v", err)
	}
	if linearAdvanced.Stage != "review" || linearAdvanced.State != StateIdle || linearAdvanced.Complete {
		t.Fatalf("linear success advance = %#v, want review/idle and incomplete", linearAdvanced)
	}

	stageOnlyTicket, doneStageID := createTicketWithPolicy("STG", "stage-only all roles ticket", WorkflowProgressionModeStageOnly)
	stageOnlyAdvanced, err := UpdateTicket(ctx, db, stageOnlyTicket.ID, TicketUpdateParams{
		Title:         stageOnlyTicket.Title,
		Description:   stageOnlyTicket.Description,
		ParentID:      stageOnlyTicket.ParentID,
		State:         StateSuccess,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(stage_only success) error = %v", err)
	}
	if stageOnlyAdvanced.Stage != StageDone || stageOnlyAdvanced.State != StateIdle {
		t.Fatalf("stage_only success advance = %#v, want done/idle", stageOnlyAdvanced)
	}
	if stageOnlyAdvanced.WorkflowStageID == nil || *stageOnlyAdvanced.WorkflowStageID != doneStageID {
		t.Fatalf("stage_only workflow_stage_id = %#v, want %d", stageOnlyAdvanced.WorkflowStageID, doneStageID)
	}
	if stageOnlyAdvanced.Complete {
		t.Fatalf("stage_only complete = %v, want false; completion is set by NextTicket at terminal step", stageOnlyAdvanced.Complete)
	}

	stageOnlySuccess, err := UpdateTicket(ctx, db, stageOnlyTicket.ID, TicketUpdateParams{
		Title:         stageOnlyAdvanced.Title,
		Description:   stageOnlyAdvanced.Description,
		ParentID:      stageOnlyAdvanced.ParentID,
		State:         StateSuccess,
		ActorUsername: "admin",
		ActorRole:     "admin",
	})
	if err != nil {
		t.Fatalf("UpdateTicket(done success) error = %v", err)
	}
	if stageOnlySuccess.State != StateSuccess {
		t.Fatalf("state after done success = %q, want %q", stageOnlySuccess.State, StateSuccess)
	}
	completed, err := NextTicket(ctx, db, stageOnlyTicket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket(done success) error = %v", err)
	}
	if !completed.Complete || completed.Stage != StageDone || completed.State != StateIdle {
		t.Fatalf("NextTicket(done success) = %#v, want done/idle complete", completed)
	}
}

func TestWorkflowStageTransitionsDriveNextAndPrevious(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	adminID := testAdminID(t, db)

	wf, err := CreateWorkflow(ctx, db, "DAG Stage Flow", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	design, err := AddWorkflowStage(ctx, db, wf.ID, "design", "", "", 0)
	if err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	testStage, err := AddWorkflowStage(ctx, db, wf.ID, "test", "", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	done, err := AddWorkflowStage(ctx, db, wf.ID, "done", "", "", 2)
	if err != nil {
		t.Fatalf("AddWorkflowStage(done) error = %v", err)
	}
	if err := SetWorkflowStageTransitions(ctx, db, wf.ID, design.ID, []int64{done.ID}); err != nil {
		t.Fatalf("SetWorkflowStageTransitions(design->done) error = %v", err)
	}
	if err := SetWorkflowStageTransitions(ctx, db, wf.ID, done.ID, []int64{testStage.ID}); err != nil {
		t.Fatalf("SetWorkflowStageTransitions(done->test) error = %v", err)
	}
	project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:     "DAG",
		Title:      "DAG Project",
		WorkflowID: &wf.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Transition ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateSuccess, RenderLifecycleStatus(ticket.Stage, StateSuccess), ticket.ID); err != nil {
		t.Fatalf("set state success error = %v", err)
	}
	advanced, err := NextTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("NextTicket() error = %v", err)
	}
	if advanced.WorkflowStageID == nil || *advanced.WorkflowStageID != done.ID {
		t.Fatalf("advanced.WorkflowStageID = %#v, want %d", advanced.WorkflowStageID, done.ID)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET state = ?, status = ? WHERE ticket_id = ?`, StateFail, RenderLifecycleStatus(advanced.Stage, StateFail), ticket.ID); err != nil {
		t.Fatalf("set state fail error = %v", err)
	}
	regressed, err := PreviousTicket(ctx, db, ticket.ID, "admin", adminID)
	if err != nil {
		t.Fatalf("PreviousTicket() error = %v", err)
	}
	if regressed.WorkflowStageID == nil || *regressed.WorkflowStageID != design.ID {
		t.Fatalf("regressed.WorkflowStageID = %#v, want %d", regressed.WorkflowStageID, design.ID)
	}
}

func TestGetTicketByRefAndValidateTicketStage(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	wf, err := CreateWorkflow(ctx, db, "Stage Validation", "")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := AddWorkflowStage(ctx, db, wf.ID, "Design", "", "", 0); err != nil {
		t.Fatalf("AddWorkflowStage(design) error = %v", err)
	}
	if _, err := AddWorkflowStage(ctx, db, wf.ID, " test ", "", "", 1); err != nil {
		t.Fatalf("AddWorkflowStage(test) error = %v", err)
	}
	project, err := CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:     "REF",
		Title:      "Ref Project",
		WorkflowID: &wf.ID,
	})
	if err != nil {
		t.Fatalf("CreateProjectWithParams() error = %v", err)
	}
	ticket, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Reference Ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	byExact, err := GetTicketByRef(ctx, db, strings.ToLower(ticket.ID))
	if err != nil {
		t.Fatalf("GetTicketByRef(exact) error = %v", err)
	}
	if byExact.ID != ticket.ID {
		t.Fatalf("GetTicketByRef(exact).ID = %q, want %q", byExact.ID, ticket.ID)
	}

	bySequence, err := GetTicketByRef(ctx, db, "1")
	if err != nil {
		t.Fatalf("GetTicketByRef(sequence) error = %v", err)
	}
	if bySequence.ID != ticket.ID {
		t.Fatalf("GetTicketByRef(sequence).ID = %q, want %q", bySequence.ID, ticket.ID)
	}

	validStage, err := validateTicketStage(ctx, db, ticket, " Test ")
	if err != nil {
		t.Fatalf("validateTicketStage(valid) error = %v", err)
	}
	if validStage != "test" {
		t.Fatalf("validateTicketStage(valid) = %q, want %q", validStage, "test")
	}

	if _, err := validateTicketStage(ctx, db, ticket, "ship"); err == nil || !strings.Contains(err.Error(), `valid stages: design, test`) {
		t.Fatalf("validateTicketStage(invalid) error = %v", err)
	}

	names := normalizeStageNames([]WorkflowStage{
		{StageName: "Design"},
		{StageName: " test "},
		{StageName: "TEST"},
		{StageName: ""},
	})
	if got, want := names, []string{"design", "test"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeStageNames() = %v, want %v", got, want)
	}
}

func TestTicketHasChildrenUsesExistenceCheck(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()

	project, err := CreateProject(ctx, db, "Child Check", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	parent, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "epic",
		Title:     "Parent",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	if hasChildren, err := ticketHasChildren(ctx, db, parent.ID); err != nil || hasChildren {
		t.Fatalf("ticketHasChildren(parent before child) = %v, %v, want false, nil", hasChildren, err)
	}
	if _, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID: project.ID,
		ParentID:  &parent.ID,
		Type:      "task",
		Title:     "Child",
		CreatedBy: "",
	}); err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}
	hasChildren, err := ticketHasChildren(ctx, db, parent.ID)
	if err != nil {
		t.Fatalf("ticketHasChildren(parent after child) error = %v", err)
	}
	if !hasChildren {
		t.Fatal("ticketHasChildren(parent after child) = false, want true")
	}
}
