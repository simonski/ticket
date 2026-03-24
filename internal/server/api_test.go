package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
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
	if got, ok := statusPayload["chat_max_connections"].(float64); !ok || int(got) != store.DefaultChatMaxConnections {
		t.Fatalf("status chat_max_connections = %#v", statusPayload["chat_max_connections"])
	}
	if got, ok := statusPayload["chat_enabled"].(bool); !ok || !got {
		t.Fatalf("status chat_enabled = %#v", statusPayload["chat_enabled"])
	}
	if got, ok := statusPayload["chat_max_duration_minutes"].(float64); !ok || int(got) != store.DefaultChatMaxDurationMinutes {
		t.Fatalf("status chat_max_duration_minutes = %#v", statusPayload["chat_max_duration_minutes"])
	}
	if got, ok := statusPayload["chat_running_processes"].(float64); !ok || int(got) != 0 {
		t.Fatalf("status chat_running_processes = %#v", statusPayload["chat_running_processes"])
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

func TestChatLimitsConfigAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &adminAuth)

	registerResp := doJSONRequest(t, handler, http.MethodPost, "/api/register", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResp.Code, http.StatusCreated)
	}
	nonAdminLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if nonAdminLoginResp.Code != http.StatusOK {
		t.Fatalf("alice login status = %d, want %d", nonAdminLoginResp.Code, http.StatusOK)
	}
	var userAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, nonAdminLoginResp, &userAuth)

	unauthorized := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_limits", map[string]int{
		"max_connections":      4,
		"max_duration_minutes": 9,
	}, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	forbidden := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_limits", map[string]int{
		"max_connections":      4,
		"max_duration_minutes": 9,
	}, userAuth.Token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	adminUpdate := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_limits", map[string]int{
		"max_connections":      4,
		"max_duration_minutes": 9,
	}, adminAuth.Token)
	if adminUpdate.Code != http.StatusOK {
		t.Fatalf("admin update status = %d, want %d body=%s", adminUpdate.Code, http.StatusOK, adminUpdate.Body.String())
	}
	var updated map[string]any
	decodeResponse(t, adminUpdate, &updated)
	if got := int(updated["chat_max_connections"].(float64)); got != 4 {
		t.Fatalf("chat_max_connections = %d, want 4", got)
	}
	if got := int(updated["chat_max_duration_minutes"].(float64)); got != 9 {
		t.Fatalf("chat_max_duration_minutes = %d, want 9", got)
	}

	statusResp := doJSONRequest(t, handler, http.MethodGet, "/api/status", nil, adminAuth.Token)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", statusResp.Code, http.StatusOK)
	}
	var status map[string]any
	decodeResponse(t, statusResp, &status)
	if got := int(status["chat_max_connections"].(float64)); got != 4 {
		t.Fatalf("status chat_max_connections = %d, want 4", got)
	}
	if got := int(status["chat_max_duration_minutes"].(float64)); got != 9 {
		t.Fatalf("status chat_max_duration_minutes = %d, want 9", got)
	}
	if got := int(status["chat_running_processes"].(float64)); got != 0 {
		t.Fatalf("status chat_running_processes = %d, want 0", got)
	}
}

func TestChatEnabledConfigAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	adminLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if adminLoginResp.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, want %d", adminLoginResp.Code, http.StatusOK)
	}
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, adminLoginResp, &adminAuth)

	registerResp := doJSONRequest(t, handler, http.MethodPost, "/api/register", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResp.Code, http.StatusCreated)
	}
	userLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if userLoginResp.Code != http.StatusOK {
		t.Fatalf("alice login status = %d, want %d", userLoginResp.Code, http.StatusOK)
	}
	var userAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, userLoginResp, &userAuth)

	unauthorized := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_enabled", map[string]bool{
		"enabled": false,
	}, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	forbidden := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_enabled", map[string]bool{
		"enabled": false,
	}, userAuth.Token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	adminUpdate := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_enabled", map[string]bool{
		"enabled": false,
	}, adminAuth.Token)
	if adminUpdate.Code != http.StatusOK {
		t.Fatalf("admin update status = %d, want %d body=%s", adminUpdate.Code, http.StatusOK, adminUpdate.Body.String())
	}
	var updated map[string]any
	decodeResponse(t, adminUpdate, &updated)
	if got, ok := updated["chat_enabled"].(bool); !ok || got {
		t.Fatalf("chat_enabled update = %#v, want false", updated["chat_enabled"])
	}

	statusResp := doJSONRequest(t, handler, http.MethodGet, "/api/status", nil, adminAuth.Token)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", statusResp.Code, http.StatusOK)
	}
	var status map[string]any
	decodeResponse(t, statusResp, &status)
	if got, ok := status["chat_enabled"].(bool); !ok || got {
		t.Fatalf("status chat_enabled = %#v, want false", status["chat_enabled"])
	}

	chatWSResp := doJSONRequest(t, handler, http.MethodGet, "/api/chat/ws", nil, adminAuth.Token)
	if chatWSResp.Code != http.StatusForbidden {
		t.Fatalf("chat ws status = %d, want %d body=%s", chatWSResp.Code, http.StatusForbidden, chatWSResp.Body.String())
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

func TestRoleAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/roles", map[string]string{
		"title":      "Release Manager",
		"motivation": "Ship reliable releases.",
		"goals":      "Coordinate release quality and timelines.",
	}, auth.Token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create role status = %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created store.Role
	decodeResponse(t, createResp, &created)
	if created.ID == 0 {
		t.Fatalf("created role id = 0")
	}

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/roles", nil, auth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list roles status = %d", listResp.Code)
	}
	var roles []store.Role
	decodeResponse(t, listResp, &roles)
	if len(roles) == 0 {
		t.Fatalf("expected seeded and/or created roles, got 0")
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/roles/"+strconv.FormatInt(created.ID, 10), map[string]string{
		"title":      "Release Captain",
		"motivation": "Ship cleanly.",
		"goals":      "Keep release velocity steady.",
	}, auth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update role status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/roles/"+strconv.FormatInt(created.ID, 10), nil, auth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete role status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestStoryAPIAndAnalyseFallback(t *testing.T) {
	t.Setenv("TICKET_ANALYSE_CMD", "false")

	handler, db := testHandler(t)
	defer db.Close()

	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &auth)

	createProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Story Test Project",
	}, auth.Token)
	if createProjectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", createProjectResp.Code, createProjectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, createProjectResp, &project)

	createStoryResp := doJSONRequest(t, handler, http.MethodPost, "/api/stories", map[string]any{
		"project_id":  project.ID,
		"title":       "Checkout improvement",
		"description": "Improve checkout conversion flow",
	}, auth.Token)
	if createStoryResp.Code != http.StatusCreated {
		t.Fatalf("create story status = %d body=%s", createStoryResp.Code, createStoryResp.Body.String())
	}
	var story store.Story
	decodeResponse(t, createStoryResp, &story)

	listStoriesResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/stories", nil, auth.Token)
	if listStoriesResp.Code != http.StatusOK {
		t.Fatalf("list stories status = %d body=%s", listStoriesResp.Code, listStoriesResp.Body.String())
	}
	var stories []store.Story
	decodeResponse(t, listStoriesResp, &stories)
	if len(stories) != 1 || stories[0].ID != story.ID {
		t.Fatalf("stories = %#v", stories)
	}

	analyseStoryResp := doJSONRequest(t, handler, http.MethodPost, "/api/stories/"+strconv.FormatInt(story.ID, 10)+"/analyse", nil, auth.Token)
	if analyseStoryResp.Code != http.StatusOK {
		t.Fatalf("analyse story status = %d body=%s", analyseStoryResp.Code, analyseStoryResp.Body.String())
	}
	var analyseStoryPayload map[string]any
	decodeResponse(t, analyseStoryResp, &analyseStoryPayload)
	if createdEpics, _ := analyseStoryPayload["created_epics"].(float64); createdEpics < 1 {
		t.Fatalf("analyse story created_epics = %v, want >= 1", analyseStoryPayload["created_epics"])
	}

	ticketsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets", nil, auth.Token)
	if ticketsResp.Code != http.StatusOK {
		t.Fatalf("list tickets status = %d body=%s", ticketsResp.Code, ticketsResp.Body.String())
	}
	var tickets []store.Ticket
	decodeResponse(t, ticketsResp, &tickets)
	var epicID int64
	for _, ticket := range tickets {
		if ticket.Type == "epic" {
			epicID = ticket.ID
			break
		}
	}
	if epicID == 0 {
		t.Fatalf("expected generated epic ticket, got %#v", tickets)
	}

	analyseEpicResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(epicID, 10)+"/analyse", nil, auth.Token)
	if analyseEpicResp.Code != http.StatusOK {
		t.Fatalf("analyse epic status = %d body=%s", analyseEpicResp.Code, analyseEpicResp.Body.String())
	}
	var analyseEpicPayload map[string]any
	decodeResponse(t, analyseEpicResp, &analyseEpicPayload)
	if createdTickets, _ := analyseEpicPayload["created_tickets"].(float64); createdTickets < 1 {
		t.Fatalf("analyse epic created_tickets = %v, want >= 1", analyseEpicPayload["created_tickets"])
	}
}

func TestAgentAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	adminLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if adminLogin.Code != http.StatusOK {
		t.Fatalf("admin login status = %d body=%s", adminLogin.Code, adminLogin.Body.String())
	}
	var adminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, adminLogin, &adminAuth)

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "nonadmin",
		"password": "password123",
	}, adminAuth.Token)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create user status = %d body=%s", createUserResp.Code, createUserResp.Body.String())
	}
	nonAdminLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "nonadmin",
		"password": "password123",
	}, "")
	if nonAdminLogin.Code != http.StatusOK {
		t.Fatalf("non-admin login status = %d body=%s", nonAdminLogin.Code, nonAdminLogin.Body.String())
	}
	var nonAdminAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, nonAdminLogin, &nonAdminAuth)

	createAgentResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents", map[string]string{
		"description": "Autonomous worker",
	}, adminAuth.Token)
	if createAgentResp.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d body=%s", createAgentResp.Code, createAgentResp.Body.String())
	}
	var createPayload struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	decodeResponse(t, createAgentResp, &createPayload)
	if createPayload.Agent.ID == 0 {
		t.Fatalf("created agent id = 0, want non-zero")
	}
	if createPayload.Password == "" {
		t.Fatalf("create password empty, want generated password")
	}
	agentUUID := createPayload.Agent.UUID

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/agents", nil, adminAuth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list agents status = %d body=%s", listResp.Code, listResp.Body.String())
	}
	var agents []store.Agent
	decodeResponse(t, listResp, &agents)
	if len(agents) != 1 {
		t.Fatalf("agents list length = %d, want 1", len(agents))
	}

	forbiddenList := doJSONRequest(t, handler, http.MethodGet, "/api/agents", nil, nonAdminAuth.Token)
	if forbiddenList.Code != http.StatusForbidden {
		t.Fatalf("non-admin list agents status = %d, want %d", forbiddenList.Code, http.StatusForbidden)
	}

	updatedResp := doJSONRequest(t, handler, http.MethodPut, "/api/agents/"+strconv.FormatInt(createPayload.Agent.ID, 10), map[string]string{
		"description": "Updated worker",
	}, adminAuth.Token)
	if updatedResp.Code != http.StatusOK {
		t.Fatalf("update agent status = %d body=%s", updatedResp.Code, updatedResp.Body.String())
	}

	registerResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/register", agentUUID, createPayload.Password, nil)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register agent status = %d body=%s", registerResp.Code, registerResp.Body.String())
	}

	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents/"+strconv.FormatInt(createPayload.Agent.ID, 10)+"/disable", nil, adminAuth.Token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable agent status = %d body=%s", disableResp.Code, disableResp.Body.String())
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/agents/"+strconv.FormatInt(createPayload.Agent.ID, 10), nil, adminAuth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete agent status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
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
	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add alice editor membership error = %v", err)
	}

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
	var ticket store.Ticket
	decodeResponse(t, createTaskResp, &ticket)

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets", nil, auth.Token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list tasks status = %d", listResp.Code)
	}
	var tickets []store.Ticket
	decodeResponse(t, listResp, &tickets)
	if len(tickets) != 2 {
		t.Fatalf("tasks len = %d, want 2", len(tickets))
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

	assignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      ticket.Status,
	}, auth.Token)
	if assignResp.Code != http.StatusOK {
		t.Fatalf("assign task status = %d body=%s", assignResp.Code, assignResp.Body.String())
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       "Add password reset flow",
		"description": "Email reset support",
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      "design/active",
	}, aliceAuth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update task status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), nil, auth.Token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get task status = %d body=%s", getResp.Code, getResp.Body.String())
	}
	var updated store.Ticket
	decodeResponse(t, getResp, &updated)
	if updated.Status != "design/active" {
		t.Fatalf("updated status = %q, want design/active", updated.Status)
	}

	filteredResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/tickets?type=task&status=design/active&q=password", nil, auth.Token)
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered list status = %d body=%s", filteredResp.Code, filteredResp.Body.String())
	}
	var filtered []store.Ticket
	decodeResponse(t, filteredResp, &filtered)
	if len(filtered) != 1 || filtered[0].ID != ticket.ID {
		t.Fatalf("filtered tickets = %#v", filtered)
	}

	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/history", nil, auth.Token)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}
	var events []store.HistoryEvent
	decodeResponse(t, historyResp, &events)
	if len(events) < 2 {
		t.Fatalf("history events = %#v", events)
	}

	commentResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/comments", map[string]string{
		"comment": "Waiting on API changes.",
	}, auth.Token)
	if commentResp.Code != http.StatusCreated {
		t.Fatalf("comment status = %d body=%s", commentResp.Code, commentResp.Body.String())
	}

	commentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/comments", nil, auth.Token)
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
		"ticket_id":  ticket.ID,
		"depends_on": epic.ID,
	}, auth.Token)
	if dependencyCreateResp.Code != http.StatusCreated {
		t.Fatalf("dependency create status = %d body=%s", dependencyCreateResp.Code, dependencyCreateResp.Body.String())
	}

	dependencyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/dependencies", nil, auth.Token)
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
	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add alice editor membership error = %v", err)
	}

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
		"priority":    ticket.Priority,
		"order":       ticket.Order,
	}, adminAuth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update ticket status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	// Mark ticket ready so it can be claimed.
	readyResp := doJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/tickets/%d/ready", ticket.ID), nil, adminAuth.Token)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready ticket status = %d body=%s", readyResp.Code, readyResp.Body.String())
	}

	claimResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
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
	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add alice editor membership error = %v", err)
	}

	createTaskResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Add reset flow",
	}, adminAuth.Token)
	if createTaskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d body=%s", createTaskResp.Code, http.StatusCreated, createTaskResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, createTaskResp, &ticket)

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

	claimResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "alice",
		"status":      ticket.Status,
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
	bobUser, err := store.GetUserByUsername(db, "bob")
	if err != nil {
		t.Fatalf("get bob user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, bobUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add bob editor membership error = %v", err)
	}

	claimConflictResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "bob",
		"status":      ticket.Status,
	}, bobAuth.Token)
	if claimConflictResp.Code != http.StatusBadRequest {
		t.Fatalf("claim conflict status = %d, want %d body=%s", claimConflictResp.Code, http.StatusBadRequest, claimConflictResp.Body.String())
	}
	var claimConflictPayload map[string]string
	decodeResponse(t, claimConflictResp, &claimConflictPayload)
	if claimConflictPayload["error"] != "ticket is already assigned to alice" {
		t.Fatalf("claim conflict payload = %#v", claimConflictPayload)
	}

	overrideResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "charlie",
		"status":      ticket.Status,
	}, bobAuth.Token)
	if overrideResp.Code != http.StatusForbidden {
		t.Fatalf("override status = %d, want %d body=%s", overrideResp.Code, http.StatusForbidden, overrideResp.Body.String())
	}
	var overridePayload map[string]string
	decodeResponse(t, overrideResp, &overridePayload)
	if overridePayload["error"] != "user is not an admin" {
		t.Fatalf("override payload = %#v", overridePayload)
	}

	missingAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "nobody",
		"status":      ticket.Status,
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
	disabledAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "alice",
		"status":      ticket.Status,
	}, adminAuth.Token)
	if disabledAssignResp.Code != http.StatusBadRequest {
		t.Fatalf("disabled assign status = %d, want %d body=%s", disabledAssignResp.Code, http.StatusBadRequest, disabledAssignResp.Body.String())
	}
	var disabledAssignPayload map[string]string
	decodeResponse(t, disabledAssignResp, &disabledAssignPayload)
	if disabledAssignPayload["error"] != "user is disabled" {
		t.Fatalf("disabled assign payload = %#v", disabledAssignPayload)
	}

	statusForbiddenResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
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
	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add alice editor membership error = %v", err)
	}

	openResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Open task",
	}, adminAuth.Token)
	var openTask store.Ticket
	decodeResponse(t, openResp, &openTask)

	otherResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Other task",
	}, adminAuth.Token)
	var otherTask store.Ticket
	decodeResponse(t, otherResp, &otherTask)

	// Mark both tickets as ready so they can be claimed.
	readyResp := doJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/tickets/%d/ready", openTask.ID), nil, adminAuth.Token)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready open task status = %d body=%s", readyResp.Code, readyResp.Body.String())
	}
	readyResp2 := doJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/tickets/%d/ready", otherTask.ID), nil, adminAuth.Token)
	if readyResp2.Code != http.StatusOK {
		t.Fatalf("ready other task status = %d body=%s", readyResp2.Code, readyResp2.Body.String())
	}

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
		"ticket_id":  otherTask.ID,
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
		"status":      "design/active",
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
	bobUser, err := store.GetUserByUsername(db, "bob")
	if err != nil {
		t.Fatalf("get bob user error = %v", err)
	}
	if _, err := store.AddProjectMember(db, project.ID, bobUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add bob editor membership error = %v", err)
	}

	rejectedResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
		"ticket_id":  otherTask.ID,
	}, bobAuth.Token)
	if rejectedResp.Code != http.StatusOK {
		t.Fatalf("request rejected status = %d body=%s", rejectedResp.Code, rejectedResp.Body.String())
	}
	var rejectedPayload map[string]string
	decodeResponse(t, rejectedResp, &rejectedPayload)
	if rejectedPayload["status"] != "REJECTED" {
		t.Fatalf("request rejected payload = %#v", rejectedPayload)
	}

	// Assign otherTask so no idle unassigned tickets remain for the NO-WORK test
	assignOtherResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+strconv.FormatInt(otherTask.ID, 10), map[string]any{
		"title":       otherTask.Title,
		"description": otherTask.Description,
		"assignee":    "alice",
		"state":       "active",
	}, adminAuth.Token)
	if assignOtherResp.Code != http.StatusOK {
		t.Fatalf("assign otherTask status = %d body=%s", assignOtherResp.Code, assignOtherResp.Body.String())
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
	var tickets []store.Ticket
	decodeResponse(t, listResp, &tickets)
	var clonedChild *store.Ticket
	for i := range tickets {
		if tickets[i].CloneOf != nil && *tickets[i].CloneOf == child.ID {
			clonedChild = &tickets[i]
			break
		}
	}
	if clonedChild == nil {
		t.Fatalf("cloned child not found in %#v", tickets)
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
	var ticket store.Ticket
	decodeResponse(t, taskResp, &ticket)

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), nil, auth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10), nil, auth.Token)
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

func TestAutoProgressTicketLifecycleDesignToDevelop(t *testing.T) {
	current := store.Ticket{
		Stage:       store.StageDesign,
		State:       store.StateIdle,
		Title:       "Old",
		Description: "Desc",
	}
	payload := ticketRequest{
		Title:       "New",
		Description: "Desc",
	}
	next := autoProgressTicketLifecycle(payload, current, "alice")
	if next.Stage != store.StageDevelop || next.State != store.StateActive {
		t.Fatalf("expected develop/active, got %s/%s", next.Stage, next.State)
	}
	if next.Assignee != "alice" {
		t.Fatalf("expected assignee alice, got %q", next.Assignee)
	}
}

func TestAutoProgressTicketLifecycleDevelopToTestOnEstimateComplete(t *testing.T) {
	current := store.Ticket{
		Stage:            store.StageDevelop,
		State:            store.StateActive,
		Title:            "Task",
		Description:      "Desc",
		EstimateComplete: "",
		Assignee:         "bob",
	}
	payload := ticketRequest{
		Title:            "Task updated",
		Description:      "Desc",
		EstimateComplete: "2026-03-10T10:00:00Z",
		Assignee:         "bob",
	}
	next := autoProgressTicketLifecycle(payload, current, "bob")
	if next.Stage != store.StageTest || next.State != store.StateActive {
		t.Fatalf("expected test/active, got %s/%s", next.Stage, next.State)
	}
}

func TestAutoProgressTicketLifecycleRespectsExplicitLifecycle(t *testing.T) {
	current := store.Ticket{
		Stage:       store.StageDesign,
		State:       store.StateIdle,
		Title:       "Old",
		Description: "Desc",
	}
	payload := ticketRequest{
		Title:       "New",
		Description: "Desc",
		Stage:       store.StageDone,
		State:       store.StateSuccess,
	}
	next := autoProgressTicketLifecycle(payload, current, "alice")
	if next.Stage != store.StageDone || next.State != store.StateSuccess {
		t.Fatalf("expected explicit done/success to be preserved, got %s/%s", next.Stage, next.State)
	}
}

func TestProjectVisibilityAndRolePermissions(t *testing.T) {
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

	aliceCreate := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminAuth.Token)
	if aliceCreate.Code != http.StatusCreated {
		t.Fatalf("create alice status=%d body=%s", aliceCreate.Code, aliceCreate.Body.String())
	}
	bobCreate := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "bob",
		"password": "password123",
	}, adminAuth.Token)
	if bobCreate.Code != http.StatusCreated {
		t.Fatalf("create bob status=%d body=%s", bobCreate.Code, bobCreate.Body.String())
	}

	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	bobLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "bob",
		"password": "password123",
	}, "")
	var bobAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, bobLogin, &bobAuth)

	privateProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Private Program",
		"visibility": "private",
	}, adminAuth.Token)
	if privateProjectResp.Code != http.StatusCreated {
		t.Fatalf("create private project status=%d body=%s", privateProjectResp.Code, privateProjectResp.Body.String())
	}
	var privateProject store.Project
	decodeResponse(t, privateProjectResp, &privateProject)
	if privateProject.Visibility != store.ProjectVisibilityPrivate {
		t.Fatalf("private project visibility=%q", privateProject.Visibility)
	}

	aliceProjectsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects", nil, aliceAuth.Token)
	if aliceProjectsResp.Code != http.StatusOK {
		t.Fatalf("alice list projects status=%d body=%s", aliceProjectsResp.Code, aliceProjectsResp.Body.String())
	}
	var aliceProjects []store.Project
	decodeResponse(t, aliceProjectsResp, &aliceProjects)
	for _, p := range aliceProjects {
		if p.ID == privateProject.ID {
			t.Fatalf("private project should not be visible to non-member")
		}
	}

	aliceGetPrivate := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10), nil, aliceAuth.Token)
	if aliceGetPrivate.Code != http.StatusForbidden {
		t.Fatalf("alice get private project status=%d want=%d body=%s", aliceGetPrivate.Code, http.StatusForbidden, aliceGetPrivate.Body.String())
	}

	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername(alice) error=%v", err)
	}
	setViewerResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10)+"/users", map[string]any{
		"user_id": aliceUser.ID,
		"role":    store.ProjectRoleViewer,
	}, adminAuth.Token)
	if setViewerResp.Code != http.StatusOK {
		t.Fatalf("set viewer status=%d body=%s", setViewerResp.Code, setViewerResp.Body.String())
	}

	aliceGetPrivate = doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10), nil, aliceAuth.Token)
	if aliceGetPrivate.Code != http.StatusOK {
		t.Fatalf("alice get private as viewer status=%d body=%s", aliceGetPrivate.Code, aliceGetPrivate.Body.String())
	}

	aliceWriteAsViewer := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": privateProject.ID,
		"type":       "task",
		"title":      "viewer should not write",
	}, aliceAuth.Token)
	if aliceWriteAsViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer create ticket status=%d want=%d body=%s", aliceWriteAsViewer.Code, http.StatusForbidden, aliceWriteAsViewer.Body.String())
	}
	aliceProjectUpdateAsViewer := doJSONRequest(t, handler, http.MethodPut, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10), map[string]any{
		"title": "Viewer update blocked",
	}, aliceAuth.Token)
	if aliceProjectUpdateAsViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer update project status=%d want=%d body=%s", aliceProjectUpdateAsViewer.Code, http.StatusForbidden, aliceProjectUpdateAsViewer.Body.String())
	}

	setEditorResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10)+"/users", map[string]any{
		"user_id": aliceUser.ID,
		"role":    store.ProjectRoleEditor,
	}, adminAuth.Token)
	if setEditorResp.Code != http.StatusOK {
		t.Fatalf("set editor status=%d body=%s", setEditorResp.Code, setEditorResp.Body.String())
	}

	aliceWriteAsEditor := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": privateProject.ID,
		"type":       "task",
		"title":      "editor can write",
	}, aliceAuth.Token)
	if aliceWriteAsEditor.Code != http.StatusCreated {
		t.Fatalf("editor create ticket status=%d body=%s", aliceWriteAsEditor.Code, aliceWriteAsEditor.Body.String())
	}
	aliceProjectUpdateAsEditor := doJSONRequest(t, handler, http.MethodPut, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10), map[string]any{
		"title": "Private Program Updated",
	}, aliceAuth.Token)
	if aliceProjectUpdateAsEditor.Code != http.StatusOK {
		t.Fatalf("editor update project status=%d body=%s", aliceProjectUpdateAsEditor.Code, aliceProjectUpdateAsEditor.Body.String())
	}

	bobGetPrivate := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(privateProject.ID, 10), nil, bobAuth.Token)
	if bobGetPrivate.Code != http.StatusForbidden {
		t.Fatalf("bob get private project status=%d want=%d body=%s", bobGetPrivate.Code, http.StatusForbidden, bobGetPrivate.Body.String())
	}

	publicProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Public Program",
		"visibility": "public",
	}, adminAuth.Token)
	if publicProjectResp.Code != http.StatusCreated {
		t.Fatalf("create public project status=%d body=%s", publicProjectResp.Code, publicProjectResp.Body.String())
	}
	var publicProject store.Project
	decodeResponse(t, publicProjectResp, &publicProject)

	bobProjectsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects", nil, bobAuth.Token)
	if bobProjectsResp.Code != http.StatusOK {
		t.Fatalf("bob list projects status=%d body=%s", bobProjectsResp.Code, bobProjectsResp.Body.String())
	}
	var bobProjects []store.Project
	decodeResponse(t, bobProjectsResp, &bobProjects)
	foundPublic := false
	for _, p := range bobProjects {
		if p.ID == publicProject.ID {
			foundPublic = true
			break
		}
	}
	if !foundPublic {
		t.Fatalf("public project should be visible to non-member")
	}

	bobReadPublic := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(publicProject.ID, 10)+"/tickets", nil, bobAuth.Token)
	if bobReadPublic.Code != http.StatusOK {
		t.Fatalf("bob read public project tickets status=%d body=%s", bobReadPublic.Code, bobReadPublic.Body.String())
	}
	bobWritePublic := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": publicProject.ID,
		"type":       "task",
		"title":      "public non-member write blocked",
	}, bobAuth.Token)
	if bobWritePublic.Code != http.StatusForbidden {
		t.Fatalf("bob write public project status=%d want=%d body=%s", bobWritePublic.Code, http.StatusForbidden, bobWritePublic.Body.String())
	}
}

func TestTeamAPIsAndProjectAccessViaTeam(t *testing.T) {
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

	createAlice := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminAuth.Token)
	if createAlice.Code != http.StatusCreated {
		t.Fatalf("create alice status=%d body=%s", createAlice.Code, createAlice.Body.String())
	}
	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	createTeam := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]any{
		"name": "Platform",
	}, adminAuth.Token)
	if createTeam.Code != http.StatusCreated {
		t.Fatalf("create team status=%d body=%s", createTeam.Code, createTeam.Body.String())
	}
	var team store.Team
	decodeResponse(t, createTeam, &team)

	teamsResp := doJSONRequest(t, handler, http.MethodGet, "/api/teams", nil, adminAuth.Token)
	if teamsResp.Code != http.StatusOK {
		t.Fatalf("list teams status=%d body=%s", teamsResp.Code, teamsResp.Body.String())
	}

	aliceUser, err := store.GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername(alice) error=%v", err)
	}
	addAliceTeam := doJSONRequest(t, handler, http.MethodPost, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/users", map[string]any{
		"user_id":   aliceUser.ID,
		"role":      store.TeamRoleMember,
		"job_title": "Engineer",
	}, adminAuth.Token)
	if addAliceTeam.Code != http.StatusOK {
		t.Fatalf("add team member status=%d body=%s", addAliceTeam.Code, addAliceTeam.Body.String())
	}

	createProject := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Private Team Project",
		"visibility": "private",
	}, adminAuth.Token)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project store.Project
	decodeResponse(t, createProject, &project)

	addTeamToProject := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/teams", map[string]any{
		"team_id": team.ID,
		"role":    store.ProjectRoleEditor,
	}, adminAuth.Token)
	if addTeamToProject.Code != http.StatusOK {
		t.Fatalf("add team to project status=%d body=%s", addTeamToProject.Code, addTeamToProject.Body.String())
	}

	aliceGetProject := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10), nil, aliceAuth.Token)
	if aliceGetProject.Code != http.StatusOK {
		t.Fatalf("alice get project via team status=%d body=%s", aliceGetProject.Code, aliceGetProject.Body.String())
	}

	aliceCreateTicket := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Team-backed write",
	}, aliceAuth.Token)
	if aliceCreateTicket.Code != http.StatusCreated {
		t.Fatalf("alice create ticket via team role status=%d body=%s", aliceCreateTicket.Code, aliceCreateTicket.Body.String())
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

	srv, err := New(":0", db, "1.2.3", false, nil, "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return srv.httpServer.Handler, db
}

func doBasicAuthRequest(t *testing.T, handler http.Handler, method, path, username, password string, payload any) *httptest.ResponseRecorder {
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
	request.SetBasicAuth(username, password)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
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

func loginAdmin(t *testing.T, handler http.Handler) string {
	t.Helper()
	resp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, want %d, body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var auth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, resp, &auth)
	return auth.Token
}

func TestHealthzAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()

	resp := doJSONRequest(t, handler, http.MethodGet, "/api/healthz", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", resp.Code, http.StatusOK)
	}
	var payload map[string]string
	decodeResponse(t, resp, &payload)
	if payload["status"] != "ok" {
		t.Fatalf("healthz status = %q, want ok", payload["status"])
	}
}

func TestWorkflowAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// List workflows (should include default)
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list workflows status = %d", listResp.Code)
	}
	var workflows []store.Workflow
	decodeResponse(t, listResp, &workflows)
	if len(workflows) == 0 {
		t.Fatal("expected at least one default workflow")
	}

	// Create workflow
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows", map[string]string{
		"name":        "CI Pipeline",
		"description": "build, test, deploy",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create workflow status = %d, body=%s", createResp.Code, createResp.Body.String())
	}
	var created store.Workflow
	decodeResponse(t, createResp, &created)
	if created.Name != "CI Pipeline" {
		t.Fatalf("created workflow name = %q", created.Name)
	}

	// Get workflow with stages
	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows/"+strconv.FormatInt(created.ID, 10), nil, token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get workflow status = %d", getResp.Code)
	}

	// Add stage
	stageResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows/"+strconv.FormatInt(created.ID, 10)+"/stages", map[string]any{
		"stage_name":  "build",
		"description": "compile",
		"sort_order":  0,
	}, token)
	if stageResp.Code != http.StatusCreated {
		t.Fatalf("add stage status = %d, body=%s", stageResp.Code, stageResp.Body.String())
	}
	var stage store.WorkflowStage
	decodeResponse(t, stageResp, &stage)
	if stage.StageName != "build" {
		t.Fatalf("stage name = %q", stage.StageName)
	}

	// Delete stage
	delStageResp := doJSONRequest(t, handler, http.MethodDelete, "/api/workflows/stages/"+strconv.FormatInt(stage.ID, 10), nil, token)
	if delStageResp.Code != http.StatusOK {
		t.Fatalf("delete stage status = %d", delStageResp.Code)
	}

	// Export workflow
	exportResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows/"+strconv.FormatInt(created.ID, 10)+"/export", nil, token)
	if exportResp.Code != http.StatusOK {
		t.Fatalf("export workflow status = %d", exportResp.Code)
	}

	// Delete workflow
	delResp := doJSONRequest(t, handler, http.MethodDelete, "/api/workflows/"+strconv.FormatInt(created.ID, 10), nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete workflow status = %d", delResp.Code)
	}
}

func TestLabelAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a label for project 1
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/labels", map[string]string{
		"name":  "urgent",
		"color": "red",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create label status = %d, body=%s", createResp.Code, createResp.Body.String())
	}
	var label store.Label
	decodeResponse(t, createResp, &label)
	if label.Name != "urgent" {
		t.Fatalf("label name = %q", label.Name)
	}

	// List labels
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/labels", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list labels status = %d", listResp.Code)
	}
	var labels []store.Label
	decodeResponse(t, listResp, &labels)
	if len(labels) < 1 {
		t.Fatal("expected at least one label")
	}

	// Create a ticket to attach the label to
	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "Label test ticket",
		"priority":   1,
	}, token)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d, body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	// Add label to ticket
	addLabelResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/labels", map[string]any{
		"label_id": label.ID,
	}, token)
	if addLabelResp.Code != http.StatusOK && addLabelResp.Code != http.StatusCreated {
		t.Fatalf("add label status = %d, body=%s", addLabelResp.Code, addLabelResp.Body.String())
	}

	// List ticket labels
	ticketLabelsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/labels", nil, token)
	if ticketLabelsResp.Code != http.StatusOK {
		t.Fatalf("list ticket labels status = %d", ticketLabelsResp.Code)
	}

	// Remove label from ticket
	removeLabelResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if removeLabelResp.Code != http.StatusOK {
		t.Fatalf("remove ticket label status = %d", removeLabelResp.Code)
	}

	// Delete label
	delResp := doJSONRequest(t, handler, http.MethodDelete, "/api/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete label status = %d", delResp.Code)
	}
}

func TestTimeEntryAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a ticket
	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "Time tracking ticket",
		"priority":   1,
	}, token)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d", ticketResp.Code)
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	// Log time
	logResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/time", map[string]any{
		"minutes": 45,
		"note":    "Initial work",
	}, token)
	if logResp.Code != http.StatusCreated {
		t.Fatalf("log time status = %d, body=%s", logResp.Code, logResp.Body.String())
	}
	var entry store.TimeEntry
	decodeResponse(t, logResp, &entry)
	if entry.Minutes != 45 {
		t.Fatalf("entry minutes = %d, want 45", entry.Minutes)
	}

	// List time entries
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/time", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list time status = %d", listResp.Code)
	}
	var entries []store.TimeEntry
	decodeResponse(t, listResp, &entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 time entry, got %d", len(entries))
	}

	// Get total
	totalResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticket.ID, 10)+"/time/total", nil, token)
	if totalResp.Code != http.StatusOK {
		t.Fatalf("total time status = %d", totalResp.Code)
	}
	var totalPayload map[string]any
	decodeResponse(t, totalResp, &totalPayload)
	if total, ok := totalPayload["total"].(float64); !ok || int(total) != 45 {
		t.Fatalf("total = %v, want 45", totalPayload["total"])
	}

	// Delete time entry
	delResp := doJSONRequest(t, handler, http.MethodDelete, "/api/time/"+strconv.FormatInt(entry.ID, 10), nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete time entry status = %d", delResp.Code)
	}
}

func TestDependencyAPI(t *testing.T) {
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create two tickets
	t1Resp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1, "type": "task", "title": "Ticket A", "priority": 1,
	}, token)
	if t1Resp.Code != http.StatusCreated {
		t.Fatalf("create ticket A status = %d", t1Resp.Code)
	}
	var ticketA store.Ticket
	decodeResponse(t, t1Resp, &ticketA)

	t2Resp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1, "type": "task", "title": "Ticket B", "priority": 1,
	}, token)
	if t2Resp.Code != http.StatusCreated {
		t.Fatalf("create ticket B status = %d", t2Resp.Code)
	}
	var ticketB store.Ticket
	decodeResponse(t, t2Resp, &ticketB)

	// Add dependency: A depends on B
	addResp := doJSONRequest(t, handler, http.MethodPost, "/api/dependencies", map[string]any{
		"project_id": 1,
		"ticket_id":  ticketA.ID,
		"depends_on": ticketB.ID,
	}, token)
	if addResp.Code != http.StatusCreated {
		t.Fatalf("add dependency status = %d, body=%s", addResp.Code, addResp.Body.String())
	}

	// List dependencies for ticket A
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+strconv.FormatInt(ticketA.ID, 10)+"/dependencies", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list dependencies status = %d", listResp.Code)
	}
	var deps []store.Dependency
	decodeResponse(t, listResp, &deps)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].DependsOn != ticketB.ID {
		t.Fatalf("depends_on = %d, want %d", deps[0].DependsOn, ticketB.ID)
	}

	// Remove dependency (DELETE uses query params)
	delPath := "/api/dependencies?project_id=1&ticket_id=" + strconv.FormatInt(ticketA.ID, 10) + "&depends_on=" + strconv.FormatInt(ticketB.ID, 10)
	delResp := doJSONRequest(t, handler, http.MethodDelete, delPath, nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("remove dependency status = %d, body=%s", delResp.Code, delResp.Body.String())
	}
}
