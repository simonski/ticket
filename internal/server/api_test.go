package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/simonski/ticket/internal/store"
)

func TestAuthAndAdminAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	registerResp := doJSONRequest(t, handler, http.MethodPost, "/api/register", map[string]string{
		"username": "carol",
		"password": "password123",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResp.Code, http.StatusCreated)
	}
	var registerPayload store.User
	decodeResponse(t, registerResp, &registerPayload)
	if registerPayload.Username != "carol" {
		t.Fatalf("register payload = %#v", registerPayload)
	}

	statusResp := doJSONRequest(t, handler, http.MethodGet, "/api/status", nil, "")
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", statusResp.Code, http.StatusOK)
	}
	var statusPayload map[string]any
	decodeResponse(t, statusResp, &statusPayload)
	if statusPayload["server_version"] != "1.2.3" {
		t.Fatalf("status payload server_version = %#v", statusPayload)
	}
	if authenticated, _ := statusPayload["authenticated"].(bool); authenticated {
		t.Fatalf("register should not authenticate the user, payload = %#v", statusPayload)
	}

	loginCarolResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "carol",
		"password": "password123",
	}, "")
	if loginCarolResp.Code != http.StatusOK {
		t.Fatalf("carol login status = %d, want %d", loginCarolResp.Code, http.StatusOK)
	}
	var carolLoginPayload struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginCarolResp, &carolLoginPayload)

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var loginPayload struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &loginPayload)

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "bob",
		"password": "password123",
	}, loginPayload.Token)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d", createUserResp.Code, http.StatusCreated)
	}

	listUsersResp := doJSONRequest(t, handler, http.MethodGet, "/api/users", nil, loginPayload.Token)
	if listUsersResp.Code != http.StatusOK {
		t.Fatalf("list users status = %d, want %d", listUsersResp.Code, http.StatusOK)
	}

	nonAdminForbidden := doJSONRequest(t, handler, http.MethodGet, "/api/users", nil, carolLoginPayload.Token)
	if nonAdminForbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin list users status = %d, want %d body=%s", nonAdminForbidden.Code, http.StatusForbidden, nonAdminForbidden.Body.String())
	}
	var forbiddenPayload map[string]string
	decodeResponse(t, nonAdminForbidden, &forbiddenPayload)
	if forbiddenPayload["error"] != "user is not an admin" {
		t.Fatalf("non-admin forbidden payload = %#v", forbiddenPayload)
	}

	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/users/bob/disable", nil, loginPayload.Token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable user status = %d, want %d", disableResp.Code, http.StatusOK)
	}

	failedLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "bob",
		"password": "password123",
	}, "")
	if failedLogin.Code != http.StatusForbidden {
		t.Fatalf("disabled user login status = %d, want %d", failedLogin.Code, http.StatusForbidden)
	}

	enableResp := doJSONRequest(t, handler, http.MethodPost, "/api/users/bob/enable", nil, loginPayload.Token)
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable user status = %d, want %d", enableResp.Code, http.StatusOK)
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/users/bob", nil, loginPayload.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete user status = %d, want %d body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	logoutResp := doJSONRequest(t, handler, http.MethodPost, "/api/logout", nil, carolLoginPayload.Token)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", logoutResp.Code, http.StatusOK)
	}
}

func TestProjectAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title":       "Customer Portal",
		"description": "Portal backlog",
	}, auth.Token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d body=%s", createResp.Code, http.StatusCreated, createResp.Body.String())
	}

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects", nil, auth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list projects status = %d, want %d", listResp.Code, http.StatusOK)
	}
	var projects []store.Project
	decodeResponse(t, listResp, &projects)
	if len(projects) != 2 || projects[1].Title != "Customer Portal" {
		t.Fatalf("projects = %#v", projects)
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(projects[1].ID, 10), nil, auth.Token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get project status = %d, want %d", getResp.Code, http.StatusOK)
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/projects/"+strconv.FormatInt(projects[1].ID, 10), map[string]string{
		"title":               "Customer Portal 2",
		"description":         "Updated",
		"acceptance_criteria": "AC",
	}, auth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update project status = %d, want %d body=%s", updateResp.Code, http.StatusOK, updateResp.Body.String())
	}

	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(projects[1].ID, 10)+"/disable", nil, auth.Token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable project status = %d, want %d body=%s", disableResp.Code, http.StatusOK, disableResp.Body.String())
	}
}

func TestTaskAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, auth.Token)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create alice status = %d body=%s", createUserResp.Code, createUserResp.Body.String())
	}
	aliceLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLoginResp, &aliceAuth)

	createProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Customer Portal",
	}, auth.Token)
	var project store.Project
	decodeResponse(t, createProjectResp, &project)

	createEpicResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "epic",
		"title":      "Authentication",
	}, auth.Token)
	if createEpicResp.Code != http.StatusCreated {
		t.Fatalf("create epic status = %d body=%s", createEpicResp.Code, createEpicResp.Body.String())
	}
	var epic store.Ticket
	decodeResponse(t, createEpicResp, &epic)

	createTaskResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"parent_id":  epic.ID,
		"type":       "task",
		"title":      "Add reset flow",
	}, auth.Token)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d body=%s", createTaskResp.Code, createTaskResp.Body.String())
	}
	var task store.Ticket
	decodeResponse(t, createTaskResp, &task)

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets", nil, auth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list tasks status = %d", listResp.Code)
	}
	var tasks []store.Ticket
	decodeResponse(t, listResp, &tasks)
	if len(tasks) != 2 {
		t.Fatalf("tasks len = %d, want 2", len(tasks))
	}

	limitedResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets?limit=1", nil, auth.Token)
	if limitedResp.Code != http.StatusOK {
		t.Fatalf("limited list tasks status = %d body=%s", limitedResp.Code, limitedResp.Body.String())
	}
	var limitedTasks []store.Ticket
	decodeResponse(t, limitedResp, &limitedTasks)
	if len(limitedTasks) != 1 {
		t.Fatalf("limited tasks len = %d, want 1", len(limitedTasks))
	}

	assignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      task.Status,
	}, auth.Token)
	if assignResp.Code != http.StatusOK {
		t.Fatalf("assign task status = %d body=%s", assignResp.Code, assignResp.Body.String())
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       "Add password reset flow",
		"description": "Email reset support",
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      "develop/active",
	}, aliceAuth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update task status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(task.ID, 10), nil, auth.Token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get task status = %d body=%s", getResp.Code, getResp.Body.String())
	}
	var updated store.Ticket
	decodeResponse(t, getResp, &updated)
	if updated.Status != "develop/active" {
		t.Fatalf("updated status = %q, want develop/active", updated.Status)
	}

	filteredResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets?type=task&status=develop/active&q=password", nil, auth.Token)
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered list status = %d body=%s", filteredResp.Code, filteredResp.Body.String())
	}
	var filtered []store.Ticket
	decodeResponse(t, filteredResp, &filtered)
	if len(filtered) != 1 || filtered[0].ID != task.ID {
		t.Fatalf("filtered tasks = %#v", filtered)
	}

	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(task.ID, 10)+"/history", nil, auth.Token)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}
	var events []store.HistoryEvent
	decodeResponse(t, historyResp, &events)
	if len(events) < 2 {
		t.Fatalf("history events = %#v", events)
	}

	commentResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(task.ID, 10)+"/comments", map[string]string{
		"comment": "Waiting on API changes.",
	}, auth.Token)
	if commentResp.Code != http.StatusCreated {
		t.Fatalf("comment status = %d body=%s", commentResp.Code, commentResp.Body.String())
	}

	commentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(task.ID, 10)+"/comments", nil, auth.Token)
	if commentsResp.Code != http.StatusOK {
		t.Fatalf("comments status = %d body=%s", commentsResp.Code, commentsResp.Body.String())
	}
	var comments []store.Comment
	decodeResponse(t, commentsResp, &comments)
	if len(comments) != 1 || comments[0].Text != "Waiting on API changes." || comments[0].Author != "admin" {
		t.Fatalf("comments = %#v", comments)
	}

	dependencyCreateResp := doJSONRequest(t, handler, http.MethodPost, "/api/dependencies", map[string]any{
		"project_id": project.ID,
		"ticket_id":  task.ID,
		"depends_on": epic.ID,
	}, auth.Token)
	if dependencyCreateResp.Code != http.StatusCreated {
		t.Fatalf("dependency create status = %d body=%s", dependencyCreateResp.Code, dependencyCreateResp.Body.String())
	}

	dependencyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(task.ID, 10)+"/dependencies", nil, auth.Token)
	if dependencyResp.Code != http.StatusOK {
		t.Fatalf("dependency status = %d body=%s", dependencyResp.Code, dependencyResp.Body.String())
	}
	var dependencies []store.Dependency
	decodeResponse(t, dependencyResp, &dependencies)
	if len(dependencies) == 0 {
		t.Fatalf("dependencies empty")
	}

	bugResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "bug",
		"title":      "Password reset email is not sent",
		"status":     "develop/idle",
	}, auth.Token)
	if bugResp.Code != http.StatusCreated {
		t.Fatalf("bug create status = %d body=%s", bugResp.Code, bugResp.Body.String())
	}
}

func TestTicketRouteAliasesAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	adminLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, adminLogin, &adminAuth)

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminAuth.Token)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create alice status = %d body=%s", createUserResp.Code, createUserResp.Body.String())
	}

	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	createProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Customer Portal",
	}, adminAuth.Token)
	if createProjectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", createProjectResp.Code, createProjectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, createProjectResp, &project)

	createTicketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Add reset flow",
	}, adminAuth.Token)
	if createTicketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", createTicketResp.Code, createTicketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, createTicketResp, &ticket)

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets", nil, adminAuth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list tickets status = %d body=%s", listResp.Code, listResp.Body.String())
	}
	var tickets []store.Ticket
	decodeResponse(t, listResp, &tickets)
	if len(tickets) != 1 || tickets[0].Key != ticket.Key {
		t.Fatalf("tickets = %#v", tickets)
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.Key, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "",
		"stage":       "develop",
		"state":       "idle",
		"priority":    ticket.Priority,
		"order":       ticket.Order,
	}, adminAuth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update ticket status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	claimResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
		"task_ref":   ticket.Key,
	}, aliceAuth.Token)
	if claimResp.Code != http.StatusOK {
		t.Fatalf("claim ticket status = %d body=%s", claimResp.Code, claimResp.Body.String())
	}
	var claimPayload struct {
		Status string       `json:"status"`
		Ticket store.Ticket `json:"ticket"`
	}
	decodeResponse(t, claimResp, &claimPayload)
	if claimPayload.Status != "ASSIGNED" || claimPayload.Ticket.Key != ticket.Key || claimPayload.Ticket.Assignee != "alice" {
		t.Fatalf("claim payload = %#v", claimPayload)
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.Key, nil, adminAuth.Token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get ticket status = %d body=%s", getResp.Code, getResp.Body.String())
	}
}

func TestCountAPIAndAssignmentRules(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	adminLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, adminLogin, &adminAuth)

	userResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminAuth.Token)
	if userResp.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d body=%s", userResp.Code, http.StatusCreated, userResp.Body.String())
	}

	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	createProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Customer Portal",
	}, adminAuth.Token)
	var project store.Project
	decodeResponse(t, createProjectResp, &project)

	createTaskResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Add reset flow",
	}, adminAuth.Token)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d body=%s", createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
	}
	var task store.Ticket
	decodeResponse(t, createTaskResp, &task)

	countResp := doJSONRequest(t, handler, http.MethodGet, "/api/count?project_id="+strconv.FormatInt(project.ID, 10), nil, adminAuth.Token)
	if countResp.Code != http.StatusOK {
		t.Fatalf("count status = %d, want %d body=%s", countResp.Code, http.StatusOK, countResp.Body.String())
	}
	var countPayload store.CountSummary
	decodeResponse(t, countResp, &countPayload)
	if countPayload.Users != 2 {
		t.Fatalf("count users = %d, want 2", countPayload.Users)
	}
	if len(countPayload.Types) != 1 || countPayload.Types[0].Type != "task" || countPayload.Types[0].Total != 1 {
		t.Fatalf("count payload types = %#v", countPayload.Types)
	}

	claimResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "alice",
		"status":      task.Status,
	}, aliceAuth.Token)
	if claimResp.Code != http.StatusOK {
		t.Fatalf("claim status = %d, want %d body=%s", claimResp.Code, http.StatusOK, claimResp.Body.String())
	}

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "bob",
		"password": "password123",
	}, adminAuth.Token)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create bob status = %d, want %d body=%s", createUserResp.Code, http.StatusCreated, createUserResp.Body.String())
	}
	bobLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "bob",
		"password": "password123",
	}, "")
	var bobAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, bobLogin, &bobAuth)

	claimConflictResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "bob",
		"status":      task.Status,
	}, bobAuth.Token)
	if claimConflictResp.Code != http.StatusBadRequest {
		t.Fatalf("claim conflict status = %d, want %d body=%s", claimConflictResp.Code, http.StatusBadRequest, claimConflictResp.Body.String())
	}
	var claimConflictPayload map[string]string
	decodeResponse(t, claimConflictResp, &claimConflictPayload)
	if claimConflictPayload["error"] != "task is already assigned to alice" {
		t.Fatalf("claim conflict payload = %#v", claimConflictPayload)
	}

	overrideResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "charlie",
		"status":      task.Status,
	}, bobAuth.Token)
	if overrideResp.Code != http.StatusForbidden {
		t.Fatalf("override status = %d, want %d body=%s", overrideResp.Code, http.StatusForbidden, overrideResp.Body.String())
	}
	var overridePayload map[string]string
	decodeResponse(t, overrideResp, &overridePayload)
	if overridePayload["error"] != "user is not an admin" {
		t.Fatalf("override payload = %#v", overridePayload)
	}

	missingAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "nobody",
		"status":      task.Status,
	}, adminAuth.Token)
	if missingAssignResp.Code != http.StatusBadRequest {
		t.Fatalf("missing assign status = %d, want %d body=%s", missingAssignResp.Code, http.StatusBadRequest, missingAssignResp.Body.String())
	}
	var missingAssignPayload map[string]string
	decodeResponse(t, missingAssignResp, &missingAssignPayload)
	if missingAssignPayload["error"] != "user not found" {
		t.Fatalf("missing assign payload = %#v", missingAssignPayload)
	}

	disableAliceResp := doJSONRequest(t, handler, http.MethodPost, "/api/users/alice/disable", nil, adminAuth.Token)
	if disableAliceResp.Code != http.StatusOK {
		t.Fatalf("disable alice status = %d, want %d body=%s", disableAliceResp.Code, http.StatusOK, disableAliceResp.Body.String())
	}
	disabledAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "alice",
		"status":      task.Status,
	}, adminAuth.Token)
	if disabledAssignResp.Code != http.StatusBadRequest {
		t.Fatalf("disabled assign status = %d, want %d body=%s", disabledAssignResp.Code, http.StatusBadRequest, disabledAssignResp.Body.String())
	}
	var disabledAssignPayload map[string]string
	decodeResponse(t, disabledAssignResp, &disabledAssignPayload)
	if disabledAssignPayload["error"] != "user is disabled" {
		t.Fatalf("disabled assign payload = %#v", disabledAssignPayload)
	}

	statusForbiddenResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(task.ID, 10), map[string]any{
		"title":       task.Title,
		"description": task.Description,
		"assignee":    "",
		"status":      "inprogress",
	}, aliceAuth.Token)
	if statusForbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("status change without assignment status = %d, want %d body=%s", statusForbiddenResp.Code, http.StatusForbidden, statusForbiddenResp.Body.String())
	}
}

func TestTicketRequestAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	adminLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, adminLogin, &adminAuth)

	userResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminAuth.Token)
	if userResp.Code != http.StatusCreated {
		t.Fatalf("create alice status = %d body=%s", userResp.Code, userResp.Body.String())
	}
	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Customer Portal",
	}, adminAuth.Token)
	var project store.Project
	decodeResponse(t, projectResp, &project)

	notReadyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Not ready task",
		"status":     "design/idle",
	}, adminAuth.Token)
	var notReady store.Ticket
	decodeResponse(t, notReadyResp, &notReady)

	openResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Open task",
		"status":     "develop/idle",
	}, adminAuth.Token)
	var openTask store.Ticket
	decodeResponse(t, openResp, &openTask)

	requestAnyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
	}, aliceAuth.Token)
	if requestAnyResp.Code != http.StatusOK {
		t.Fatalf("request any status = %d body=%s", requestAnyResp.Code, requestAnyResp.Body.String())
	}
	var requestAnyPayload struct {
		Status string       `json:"status"`
		Ticket store.Ticket `json:"ticket"`
	}
	decodeResponse(t, requestAnyResp, &requestAnyPayload)
	if requestAnyPayload.Status != "ASSIGNED" || requestAnyPayload.Ticket.ID != openTask.ID || requestAnyPayload.Ticket.Assignee != "alice" {
		t.Fatalf("request any payload = %#v", requestAnyPayload)
	}

	requestSpecificResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
		"ticket_id":  notReady.ID,
	}, aliceAuth.Token)
	if requestSpecificResp.Code != http.StatusOK {
		t.Fatalf("request specific status = %d body=%s", requestSpecificResp.Code, requestSpecificResp.Body.String())
	}
	var requestSpecificPayload struct {
		Status string       `json:"status"`
		Ticket store.Ticket `json:"ticket"`
	}
	decodeResponse(t, requestSpecificResp, &requestSpecificPayload)
	if requestSpecificPayload.Status != "ASSIGNED" || requestSpecificPayload.Ticket.ID != openTask.ID {
		t.Fatalf("request specific payload = %#v", requestSpecificPayload)
	}

	inProgressResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(openTask.ID, 10), map[string]any{
		"title":       openTask.Title,
		"description": openTask.Description,
		"assignee":    "alice",
		"status":      "develop/active",
	}, aliceAuth.Token)
	if inProgressResp.Code != http.StatusOK {
		t.Fatalf("set inprogress status = %d body=%s", inProgressResp.Code, inProgressResp.Body.String())
	}

	userResp = doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "bob",
		"password": "password123",
	}, adminAuth.Token)
	if userResp.Code != http.StatusCreated {
		t.Fatalf("create bob status = %d body=%s", userResp.Code, userResp.Body.String())
	}
	bobLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "bob",
		"password": "password123",
	}, "")
	var bobAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, bobLogin, &bobAuth)

	rejectedResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
		"ticket_id":  notReady.ID,
	}, bobAuth.Token)
	if rejectedResp.Code != http.StatusOK {
		t.Fatalf("request rejected status = %d body=%s", rejectedResp.Code, rejectedResp.Body.String())
	}
	var rejectedPayload map[string]string
	decodeResponse(t, rejectedResp, &rejectedPayload)
	if rejectedPayload["status"] != "REJECTED" {
		t.Fatalf("request rejected payload = %#v", rejectedPayload)
	}

	noWorkResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
	}, bobAuth.Token)
	if noWorkResp.Code != http.StatusOK {
		t.Fatalf("request no-work status = %d body=%s", noWorkResp.Code, noWorkResp.Body.String())
	}
	var noWorkPayload map[string]string
	decodeResponse(t, noWorkResp, &noWorkPayload)
	if noWorkPayload["status"] != "NO-WORK" {
		t.Fatalf("request no-work payload = %#v", noWorkPayload)
	}
}

func TestCloneTicketAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Customer Portal",
	}, auth.Token)
	var project store.Project
	decodeResponse(t, projectResp, &project)

	epicResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "epic",
		"title":      "Epic",
	}, auth.Token)
	var epic store.Ticket
	decodeResponse(t, epicResp, &epic)

	childResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"parent_id":  epic.ID,
		"type":       "task",
		"title":      "Child",
		"status":     "develop/idle",
		"assignee":   "admin",
	}, auth.Token)
	var child store.Ticket
	decodeResponse(t, childResp, &child)

	cloneResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(epic.ID, 10)+"/clone", nil, auth.Token)
	if cloneResp.Code != http.StatusCreated {
		t.Fatalf("clone status = %d, want %d body=%s", cloneResp.Code, http.StatusCreated, cloneResp.Body.String())
	}
	var clonedEpic store.Ticket
	decodeResponse(t, cloneResp, &clonedEpic)
	if clonedEpic.CloneOf == nil || *clonedEpic.CloneOf != epic.ID || clonedEpic.Status != "design/idle" || clonedEpic.Assignee != "" {
		t.Fatalf("cloned epic = %#v", clonedEpic)
	}

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets", nil, auth.Token)
	var tasks []store.Ticket
	decodeResponse(t, listResp, &tasks)
	var clonedChild *store.Ticket
	for i := range tasks {
		if tasks[i].CloneOf != nil && *tasks[i].CloneOf == child.ID {
			clonedChild = &tasks[i]
			break
		}
	}
	if clonedChild == nil {
		t.Fatalf("cloned child not found in %#v", tasks)
	}
	if clonedChild.ParentID == nil || *clonedChild.ParentID != clonedEpic.ID || clonedChild.Status != "design/idle" || clonedChild.Assignee != "" {
		t.Fatalf("cloned child = %#v", clonedChild)
	}
}

func TestDeleteTicketAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	taskResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "Delete me",
	}, auth.Token)
	var task store.Ticket
	decodeResponse(t, taskResp, &task)

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+strconv.FormatInt(task.ID, 10), nil, auth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(task.ID, 10), nil, auth.Token)
	if getResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want %d body=%s", getResp.Code, http.StatusNotFound, getResp.Body.String())
	}
}

func TestDeleteTicketAPIFailsWhenTaskHasChildren(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	parentResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "epic",
		"title":      "Parent",
	}, auth.Token)
	var parent store.Ticket
	decodeResponse(t, parentResp, &parent)

	childResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"parent_id":  parent.ID,
		"type":       "task",
		"title":      "Child",
	}, auth.Token)
	if childResp.Code != http.StatusCreated {
		t.Fatalf("child create status = %d body=%s", childResp.Code, childResp.Body.String())
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+strconv.FormatInt(parent.ID, 10), nil, auth.Token)
	if deleteResp.Code != http.StatusBadRequest {
		t.Fatalf("delete parent status = %d, want %d body=%s", deleteResp.Code, http.StatusBadRequest, deleteResp.Body.String())
	}
}

func TestAPIMethodValidation(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	resp := doJSONRequest(t, handler, http.MethodPost, "/api/status", nil, "")
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status method code = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}

	resp = doJSONRequest(t, handler, http.MethodGet, "/api/login", nil, "")
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("login method code = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}
}

func TestAPIInvalidJSONAndBadParams(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d", loginResp.Code)
	}
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	invalidJSON := doRawRequest(t, handler, http.MethodPost, "/api/register", []byte("{bad"), "")
	if invalidJSON.Code != http.StatusBadRequest {
		t.Fatalf("register invalid json code = %d, want %d", invalidJSON.Code, http.StatusBadRequest)
	}

	badCount := doJSONRequest(t, handler, http.MethodGet, "/api/count?project_id=abc", nil, auth.Token)
	if badCount.Code != http.StatusBadRequest {
		t.Fatalf("count bad project_id code = %d, want %d", badCount.Code, http.StatusBadRequest)
	}

	badTask := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/not-a-number", nil, auth.Token)
	if badTask.Code != http.StatusNotFound {
		t.Fatalf("task bad id code = %d, want %d", badTask.Code, http.StatusNotFound)
	}
}

func TestAPIMissingAuth(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	resp := doJSONRequest(t, handler, http.MethodGet, "/api/users", nil, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth code = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func testHandler(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	srv, err := New(":0", db, "1.2.3", false, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return srv.httpServer.Handler, db
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func doRawRequest(t *testing.T, handler http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, recorder.Body.String())
	}
}
