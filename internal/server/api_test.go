package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
)

func TestAuthAndAdminAPI(t *testing.T) {
	t.Parallel()
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

func TestOpenAPIVersionMatchesBinaryVersion(t *testing.T) {
	t.Parallel()

	versionBytes, err := os.ReadFile(filepath.Join("..", "..", "cmd", "tk", "VERSION"))
	if err != nil {
		t.Fatalf("ReadFile(VERSION) error = %v", err)
	}
	want := strings.TrimSpace(string(versionBytes))

	specBytes, err := os.ReadFile(filepath.Join("..", "..", "openapi.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(openapi.yaml) error = %v", err)
	}
	lines := strings.Split(string(specBytes), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "  version: ") {
			got := strings.TrimSpace(strings.TrimPrefix(line, "  version: "))
			if got != want {
				t.Fatalf("openapi.yaml info.version = %q, want VERSION %q", got, want)
			}
			return
		}
		if i > 20 {
			break
		}
	}
	t.Fatal("openapi.yaml missing info.version")
}

func TestPublicAPIContractValidationAndAuthPaths(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	userResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "alice",
		"password": "password123",
	}, adminToken)
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

	teamResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]string{"name": "Contract Team"}, adminToken)
	if teamResp.Code != http.StatusCreated {
		t.Fatalf("create team status = %d body=%s", teamResp.Code, teamResp.Body.String())
	}
	var team store.Team
	decodeResponse(t, teamResp, &team)

	agentResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents", map[string]string{"password": "agent-secret"}, adminToken)
	if agentResp.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d body=%s", agentResp.Code, agentResp.Body.String())
	}
	var agentPayload struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	decodeResponse(t, agentResp, &agentPayload)

	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "Contract validation ticket",
	}, adminToken)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	cases := []struct {
		name    string
		method  string
		path    string
		payload any
		token   string
		want    int
	}{
		{"ws post rejects method", http.MethodPost, "/api/ws", nil, adminToken, http.StatusMethodNotAllowed},
		{"ws get requires auth", http.MethodGet, "/api/ws", nil, "", http.StatusUnauthorized},
		{"chat ws post rejects method", http.MethodPost, "/api/chat/ws", nil, adminToken, http.StatusMethodNotAllowed},
		{"chat ws get requires auth", http.MethodGet, "/api/chat/ws", nil, "", http.StatusUnauthorized},
		{"users put rejects method", http.MethodPut, "/api/users", nil, adminToken, http.StatusMethodNotAllowed},
		{"users path get rejects method", http.MethodGet, "/api/users/alice", nil, adminToken, http.StatusMethodNotAllowed},
		{"users unknown action not found", http.MethodPost, "/api/users/alice/promote", nil, adminToken, http.StatusNotFound},
		{"missing user delete not found", http.MethodDelete, "/api/users/missing", nil, adminToken, http.StatusNotFound},
		{"agents list requires admin", http.MethodGet, "/api/agents", nil, aliceAuth.Token, http.StatusForbidden},
		{"agents bad limit", http.MethodGet, "/api/agents?limit=0", nil, adminToken, http.StatusBadRequest},
		{"agents bad offset", http.MethodGet, "/api/agents?offset=-1", nil, adminToken, http.StatusBadRequest},
		{"agents statuses reject method", http.MethodPost, "/api/agents/statuses", nil, adminToken, http.StatusMethodNotAllowed},
		{"agents register requires basic auth", http.MethodPost, "/api/agents/register", nil, "", http.StatusUnauthorized},
		{"agents heartbeat requires basic auth", http.MethodPost, "/api/agents/heartbeat", nil, "", http.StatusUnauthorized},
		{"agents request requires basic auth", http.MethodPost, "/api/agents/request", nil, "", http.StatusUnauthorized},
		{"agent update missing id", http.MethodPut, "/api/agents/missing", map[string]string{"password": "x"}, adminToken, http.StatusNotFound},
		{"agent unknown action not found", http.MethodPost, "/api/agents/" + agentPayload.Agent.ID + "/unknown", nil, adminToken, http.StatusNotFound},
		{"agent config put rejects method", http.MethodPut, "/api/agents/" + agentPayload.Agent.ID + "/config", nil, adminToken, http.StatusMethodNotAllowed},
		{"teams list requires auth", http.MethodGet, "/api/teams", nil, "", http.StatusUnauthorized},
		{"teams create requires admin", http.MethodPost, "/api/teams", map[string]string{"name": "User Team"}, aliceAuth.Token, http.StatusForbidden},
		{"teams bad id", http.MethodGet, "/api/teams/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"teams missing id", http.MethodGet, "/api/teams/", nil, adminToken, http.StatusNotFound},
		{"team users post forbidden to memberless user", http.MethodPost, "/api/teams/" + strconv.FormatInt(team.ID, 10) + "/users", map[string]string{"user_id": "x"}, aliceAuth.Token, http.StatusForbidden},
		{"team agents post forbidden to memberless user", http.MethodPost, "/api/teams/" + strconv.FormatInt(team.ID, 10) + "/agents", map[string]string{"agent_id": agentPayload.Agent.ID}, aliceAuth.Token, http.StatusForbidden},
		{"team unknown route not found", http.MethodGet, "/api/teams/" + strconv.FormatInt(team.ID, 10) + "/unknown", nil, adminToken, http.StatusNotFound},
		{"projects list requires auth", http.MethodGet, "/api/projects", nil, "", http.StatusUnauthorized},
		{"projects post requires auth", http.MethodPost, "/api/projects", map[string]string{"title": "No Auth"}, "", http.StatusUnauthorized},
		{"project tickets missing project", http.MethodGet, "/api/projects/999999/tickets", nil, adminToken, http.StatusNotFound},
		{"project tickets bad limit", http.MethodGet, "/api/projects/1/tickets?limit=abc", nil, adminToken, http.StatusBadRequest},
		{"project tickets bad offset", http.MethodGet, "/api/projects/1/tickets?offset=abc", nil, adminToken, http.StatusBadRequest},
		{"project interventions missing project", http.MethodGet, "/api/projects/999999/interventions", nil, adminToken, http.StatusNotFound},
		{"project interventions bad limit", http.MethodGet, "/api/projects/1/interventions?limit=abc", nil, adminToken, http.StatusBadRequest},
		{"project interventions bad offset", http.MethodGet, "/api/projects/1/interventions?offset=abc", nil, adminToken, http.StatusBadRequest},
		{"project history bad limit", http.MethodGet, "/api/projects/1/history?limit=abc", nil, adminToken, http.StatusBadRequest},
		{"project history bad team", http.MethodGet, "/api/projects/1/history?team_id=abc", nil, adminToken, http.StatusBadRequest},
		{"project stories missing project", http.MethodGet, "/api/projects/999999/stories", nil, adminToken, http.StatusNotFound},
		{"project users delete missing user id", http.MethodDelete, "/api/projects/1/users", nil, adminToken, http.StatusBadRequest},
		{"project teams delete bad team id", http.MethodDelete, "/api/projects/1/teams/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"project labels bad project id", http.MethodGet, "/api/projects/not-a-number/labels", nil, adminToken, http.StatusBadRequest},
		{"project label bad label id", http.MethodDelete, "/api/projects/1/labels/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"project label put rejects method", http.MethodPut, "/api/projects/1/labels", nil, adminToken, http.StatusMethodNotAllowed},
		{"project set draft requires put route", http.MethodPost, "/api/projects/1/set-draft", map[string]bool{"draft": true}, adminToken, http.StatusNotFound},
		{"project unknown post action not found", http.MethodPost, "/api/projects/1/unknown", nil, adminToken, http.StatusNotFound},
		{"project nested unknown method not allowed", http.MethodGet, "/api/projects/1/unknown/path", nil, adminToken, http.StatusMethodNotAllowed},
		{"missing project get not found", http.MethodGet, "/api/projects/999999", nil, adminToken, http.StatusNotFound},
		{"project delete requires admin", http.MethodDelete, "/api/projects/1", nil, aliceAuth.Token, http.StatusForbidden},
		{"workflows import requires admin", http.MethodPost, "/api/workflows/import", map[string]string{"name": "x"}, aliceAuth.Token, http.StatusForbidden},
		{"workflows import rejects method", http.MethodGet, "/api/workflows/import", nil, adminToken, http.StatusMethodNotAllowed},
		{"workflow stage bad id", http.MethodGet, "/api/workflows/stages/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"workflow stage missing", http.MethodGet, "/api/workflows/stages/999999", nil, adminToken, http.StatusNotFound},
		{"workflow stage role bad path", http.MethodPost, "/api/workflows/stages/roles/1", nil, adminToken, http.StatusBadRequest},
		{"workflow stage role bad workflow id", http.MethodPost, "/api/workflows/stages/roles/x/1", nil, adminToken, http.StatusBadRequest},
		{"workflow stage role bad stage id", http.MethodPost, "/api/workflows/stages/roles/1/x", nil, adminToken, http.StatusBadRequest},
		{"workflow stage role delete requires role", http.MethodDelete, "/api/workflows/stages/roles/1/1", nil, adminToken, http.StatusBadRequest},
		{"workflow bad id", http.MethodGet, "/api/workflows/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"workflow missing", http.MethodGet, "/api/workflows/999999", nil, adminToken, http.StatusNotFound},
		{"workflow direct rejects patch", http.MethodPatch, "/api/workflows/1", nil, adminToken, http.StatusMethodNotAllowed},
		{"story post rejects method", http.MethodGet, "/api/stories", nil, adminToken, http.StatusMethodNotAllowed},
		{"story bad id", http.MethodGet, "/api/stories/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"story missing", http.MethodGet, "/api/stories/999999", nil, adminToken, http.StatusNotFound},
		{"tickets create rejects method", http.MethodGet, "/api/tickets", nil, adminToken, http.StatusMethodNotAllowed},
		{"tickets claim missing ticket", http.MethodPost, "/api/tickets/claim", map[string]any{"ticket_id": "TST-999999"}, adminToken, http.StatusNotFound},
		{"ticket labels bad label id", http.MethodDelete, "/api/tickets/" + ticket.ID + "/labels/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"ticket labels put rejects method", http.MethodPut, "/api/tickets/" + ticket.ID + "/labels", nil, adminToken, http.StatusMethodNotAllowed},
		{"ticket time total accepts any method as read", http.MethodPost, "/api/tickets/" + ticket.ID + "/time/total", nil, adminToken, http.StatusOK},
		{"ticket unknown method not allowed", http.MethodPatch, "/api/tickets/" + ticket.ID, nil, adminToken, http.StatusMethodNotAllowed},
		{"labels delete bad id", http.MethodDelete, "/api/labels/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"time delete bad id", http.MethodDelete, "/api/time/not-a-number", nil, adminToken, http.StatusBadRequest},
		{"dependencies bad delete project id", http.MethodDelete, "/api/dependencies?project_id=abc", nil, adminToken, http.StatusBadRequest},
		{"dependencies missing ticket id", http.MethodDelete, "/api/dependencies?project_id=1", nil, adminToken, http.StatusBadRequest},
		{"dependencies missing depends on", http.MethodDelete, "/api/dependencies?project_id=1&ticket_id=" + ticket.ID, nil, adminToken, http.StatusBadRequest},
		{"dependencies rejects put", http.MethodPut, "/api/dependencies", nil, adminToken, http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSONRequest(t, handler, tc.method, tc.path, tc.payload, tc.token)
			if resp.Code != tc.want {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, resp.Code, tc.want, resp.Body.String())
			}
		})
	}

	rawCases := []struct {
		name   string
		method string
		path   string
		token  string
		want   int
	}{
		{"register invalid json", http.MethodPost, "/api/register", "", http.StatusBadRequest},
		{"login invalid json", http.MethodPost, "/api/login", "", http.StatusBadRequest},
		{"user create invalid json", http.MethodPost, "/api/users", adminToken, http.StatusBadRequest},
		{"agent create invalid json", http.MethodPost, "/api/agents", adminToken, http.StatusBadRequest},
		{"team create invalid json", http.MethodPost, "/api/teams", adminToken, http.StatusBadRequest},
		{"project create invalid json", http.MethodPost, "/api/projects", adminToken, http.StatusBadRequest},
		{"project update invalid json", http.MethodPut, "/api/projects/1", adminToken, http.StatusBadRequest},
		{"project set draft invalid json", http.MethodPut, "/api/projects/1/set-draft", adminToken, http.StatusBadRequest},
		{"workflow create invalid json", http.MethodPost, "/api/workflows", adminToken, http.StatusBadRequest},
		{"workflow stage create invalid json", http.MethodPost, "/api/workflows/1/stages", adminToken, http.StatusBadRequest},
		{"ticket create invalid json", http.MethodPost, "/api/tickets", adminToken, http.StatusBadRequest},
		{"ticket update invalid json", http.MethodPut, "/api/tickets/" + ticket.ID, adminToken, http.StatusBadRequest},
		{"story create invalid json", http.MethodPost, "/api/stories", adminToken, http.StatusBadRequest},
		{"ticket comment invalid json", http.MethodPost, "/api/tickets/" + ticket.ID + "/comments", adminToken, http.StatusBadRequest},
	}
	for _, tc := range rawCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRawRequest(t, handler, tc.method, tc.path, []byte("{bad"), tc.token)
			if resp.Code != tc.want {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, resp.Code, tc.want, resp.Body.String())
			}
		})
	}
}

func TestCreateEndpointsSupportForcedIDs(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"id":     201,
		"prefix": "EXP",
		"title":  "Explicit Project",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("project create status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)
	if project.ID != 201 {
		t.Fatalf("project id = %d, want 201", project.ID)
	}

	teamResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]any{
		"id":   202,
		"name": "Explicit Team",
	}, adminToken)
	if teamResp.Code != http.StatusCreated {
		t.Fatalf("team create status = %d body=%s", teamResp.Code, teamResp.Body.String())
	}
	var team store.Team
	decodeResponse(t, teamResp, &team)
	if team.ID != 202 {
		t.Fatalf("team id = %d, want 202", team.ID)
	}

	roleResp := doJSONRequest(t, handler, http.MethodPost, "/api/roles", map[string]any{
		"id":    203,
		"title": "Explicit Role",
	}, adminToken)
	if roleResp.Code != http.StatusCreated {
		t.Fatalf("role create status = %d body=%s", roleResp.Code, roleResp.Body.String())
	}
	var role store.Role
	decodeResponse(t, roleResp, &role)
	if role.ID != 203 {
		t.Fatalf("role id = %d, want 203", role.ID)
	}

	workflowResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows", map[string]any{
		"id":   204,
		"name": "Explicit Workflow",
	}, adminToken)
	if workflowResp.Code != http.StatusCreated {
		t.Fatalf("workflow create status = %d body=%s", workflowResp.Code, workflowResp.Body.String())
	}
	var wf store.Workflow
	decodeResponse(t, workflowResp, &wf)
	if wf.ID != 204 {
		t.Fatalf("workflow id = %d, want 204", wf.ID)
	}

	labelResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/labels", map[string]any{
		"id":   205,
		"name": "explicit-label",
	}, adminToken)
	if labelResp.Code != http.StatusCreated {
		t.Fatalf("label create status = %d body=%s", labelResp.Code, labelResp.Body.String())
	}
	var label store.Label
	decodeResponse(t, labelResp, &label)
	if label.ID != 205 {
		t.Fatalf("label id = %d, want 205", label.ID)
	}

	storyResp := doJSONRequest(t, handler, http.MethodPost, "/api/stories", map[string]any{
		"id":          206,
		"project_id":  1,
		"title":       "Explicit Story",
		"description": "forced story id",
	}, adminToken)
	if storyResp.Code != http.StatusCreated {
		t.Fatalf("story create status = %d body=%s", storyResp.Code, storyResp.Body.String())
	}
	var story store.Story
	decodeResponse(t, storyResp, &story)
	if story.ID != 206 {
		t.Fatalf("story id = %d, want 206", story.ID)
	}
}

func TestChatLimitsConfigAPI(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestResetPasswordProjectDraftAndAgentConfigAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	if _, err := store.CreateUser(context.Background(), db, "bob", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	if _, err := store.CreateUser(context.Background(), db, "carol", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(carol) error = %v", err)
	}
	carolLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "carol",
		"password": "password123",
	}, "")
	if carolLoginResp.Code != http.StatusOK {
		t.Fatalf("carol login status = %d, want %d", carolLoginResp.Code, http.StatusOK)
	}
	var carolAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, carolLoginResp, &carolAuth)

	unauthorizedReset := doJSONRequest(t, handler, http.MethodPost, "/api/users/bob/reset-password", map[string]string{
		"password": "new-password-123",
	}, "")
	if unauthorizedReset.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized reset status = %d, want %d", unauthorizedReset.Code, http.StatusUnauthorized)
	}

	resetResp := doJSONRequest(t, handler, http.MethodPost, "/api/users/bob/reset-password", map[string]string{
		"password": "new-password-123",
	}, adminToken)
	if resetResp.Code != http.StatusOK {
		t.Fatalf("reset password status = %d, want %d body=%s", resetResp.Code, http.StatusOK, resetResp.Body.String())
	}

	reloginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "bob",
		"password": "new-password-123",
	}, "")
	if reloginResp.Code != http.StatusOK {
		t.Fatalf("bob relogin status = %d, want %d", reloginResp.Code, http.StatusOK)
	}

	missingReset := doJSONRequest(t, handler, http.MethodPost, "/api/users/missing/reset-password", map[string]string{
		"password": "new-password-123",
	}, adminToken)
	if missingReset.Code != http.StatusNotFound {
		t.Fatalf("missing reset status = %d, want %d", missingReset.Code, http.StatusNotFound)
	}

	unauthorizedDraft := doJSONRequest(t, handler, http.MethodPut, "/api/projects/1/set-draft", map[string]bool{
		"draft": true,
	}, "")
	if unauthorizedDraft.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized set-draft status = %d, want %d", unauthorizedDraft.Code, http.StatusUnauthorized)
	}

	forbiddenDraft := doJSONRequest(t, handler, http.MethodPut, "/api/projects/1/set-draft", map[string]bool{
		"draft": true,
	}, carolAuth.Token)
	if forbiddenDraft.Code != http.StatusForbidden {
		t.Fatalf("forbidden set-draft status = %d, want %d", forbiddenDraft.Code, http.StatusForbidden)
	}

	draftResp := doJSONRequest(t, handler, http.MethodPut, "/api/projects/1/set-draft", map[string]bool{
		"draft": true,
	}, adminToken)
	if draftResp.Code != http.StatusOK {
		t.Fatalf("set-draft status = %d, want %d body=%s", draftResp.Code, http.StatusOK, draftResp.Body.String())
	}
	project, err := store.GetProject(context.Background(), db, "1")
	if err != nil {
		t.Fatalf("GetProject(1) error = %v", err)
	}
	if !project.DefaultDraft {
		t.Fatalf("project.DefaultDraft = %v, want true", project.DefaultDraft)
	}

	agent, _, err := store.CreateAgent(context.Background(), db, "secret-agent")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if _, _, err := store.CreateAgent(context.Background(), db, "secret-agent-2"); err != nil {
		t.Fatalf("CreateAgent(second) error = %v", err)
	}

	listAgentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/agents?limit=1&offset=1", nil, adminToken)
	if listAgentsResp.Code != http.StatusOK {
		t.Fatalf("list agents status = %d, want %d", listAgentsResp.Code, http.StatusOK)
	}
	var agents []store.User
	decodeResponse(t, listAgentsResp, &agents)
	if len(agents) != 1 {
		t.Fatalf("paged agents len = %d, want 1", len(agents))
	}

	listConfigResp := doJSONRequest(t, handler, http.MethodGet, "/api/agents/"+agent.ID+"/config", nil, adminToken)
	if listConfigResp.Code != http.StatusOK {
		t.Fatalf("list agent config status = %d, want %d", listConfigResp.Code, http.StatusOK)
	}
	var emptyConfig []store.AgentConfigEntry
	decodeResponse(t, listConfigResp, &emptyConfig)
	if len(emptyConfig) != 0 {
		t.Fatalf("initial agent config len = %d, want 0", len(emptyConfig))
	}

	setConfigResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents/"+agent.ID+"/config", map[string]string{
		"key":   "llm",
		"value": "gpt-5",
	}, adminToken)
	if setConfigResp.Code != http.StatusOK {
		t.Fatalf("set agent config status = %d, want %d body=%s", setConfigResp.Code, http.StatusOK, setConfigResp.Body.String())
	}

	listConfigResp = doJSONRequest(t, handler, http.MethodGet, "/api/agents/"+agent.ID+"/config", nil, adminToken)
	if listConfigResp.Code != http.StatusOK {
		t.Fatalf("list agent config after set status = %d, want %d", listConfigResp.Code, http.StatusOK)
	}
	var configEntries []store.AgentConfigEntry
	decodeResponse(t, listConfigResp, &configEntries)
	if len(configEntries) != 1 || configEntries[0].Key != "llm" || configEntries[0].Value != "gpt-5" {
		t.Fatalf("agent config entries = %#v", configEntries)
	}

	badDeleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/agents/"+agent.ID+"/config", nil, adminToken)
	if badDeleteResp.Code != http.StatusBadRequest {
		t.Fatalf("bad delete config status = %d, want %d", badDeleteResp.Code, http.StatusBadRequest)
	}

	deleteConfigResp := doJSONRequest(t, handler, http.MethodDelete, "/api/agents/"+agent.ID+"/config/llm", nil, adminToken)
	if deleteConfigResp.Code != http.StatusOK {
		t.Fatalf("delete agent config status = %d, want %d", deleteConfigResp.Code, http.StatusOK)
	}
}

func TestSystemMetricsHealthAndCountAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	healthResp := doJSONRequest(t, handler, http.MethodGet, "/api/healthz", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", healthResp.Code, http.StatusOK)
	}
	var health map[string]string
	decodeResponse(t, healthResp, &health)
	if health["status"] != "ok" || health["version"] != "1.2.3" {
		t.Fatalf("health payload = %#v", health)
	}

	adminToken := loginAdmin(t, handler)

	metricsUnauthorized := doJSONRequest(t, handler, http.MethodGet, "/metrics", nil, "")
	if metricsUnauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("metrics unauthorized status = %d, want %d", metricsUnauthorized.Code, http.StatusUnauthorized)
	}

	metricsResp := doJSONRequest(t, handler, http.MethodGet, "/metrics", nil, adminToken)
	if metricsResp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d body=%s", metricsResp.Code, http.StatusOK, metricsResp.Body.String())
	}
	if got := metricsResp.Header().Get("Content-Type"); got != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics content-type = %q", got)
	}
	for _, want := range []string{"ticket_up 1", "ticket_projects_total", "ticket_users_total", "go_goroutines"} {
		if !bytes.Contains(metricsResp.Body.Bytes(), []byte(want)) {
			t.Fatalf("metrics body missing %q:\n%s", want, metricsResp.Body.String())
		}
	}

	countResp := doJSONRequest(t, handler, http.MethodGet, "/api/count", nil, adminToken)
	if countResp.Code != http.StatusOK {
		t.Fatalf("count status = %d, want %d", countResp.Code, http.StatusOK)
	}

	badCountResp := doJSONRequest(t, handler, http.MethodGet, "/api/count?project_id=abc", nil, adminToken)
	if badCountResp.Code != http.StatusBadRequest {
		t.Fatalf("bad count status = %d, want %d", badCountResp.Code, http.StatusBadRequest)
	}
}

func TestRoleAndTeamManagementAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	alice, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	aliceLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if aliceLoginResp.Code != http.StatusOK {
		t.Fatalf("alice login status = %d, want %d", aliceLoginResp.Code, http.StatusOK)
	}
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLoginResp, &aliceAuth)

	createRoleResp := doJSONRequest(t, handler, http.MethodPost, "/api/roles", map[string]string{
		"title":               "reviewer",
		"description":         "reviews work",
		"acceptance_criteria": "approves changes",
	}, adminToken)
	if createRoleResp.Code != http.StatusCreated {
		t.Fatalf("create role status = %d, want %d body=%s", createRoleResp.Code, http.StatusCreated, createRoleResp.Body.String())
	}
	var role store.Role
	decodeResponse(t, createRoleResp, &role)

	listRolesResp := doJSONRequest(t, handler, http.MethodGet, "/api/roles", nil, adminToken)
	if listRolesResp.Code != http.StatusOK {
		t.Fatalf("list roles status = %d, want %d", listRolesResp.Code, http.StatusOK)
	}

	updateRoleResp := doJSONRequest(t, handler, http.MethodPut, "/api/roles/"+strconv.FormatInt(role.ID, 10), map[string]string{
		"title":               "qa-reviewer",
		"description":         "reviews and validates",
		"acceptance_criteria": "approves releases",
	}, adminToken)
	if updateRoleResp.Code != http.StatusOK {
		t.Fatalf("update role status = %d, want %d body=%s", updateRoleResp.Code, http.StatusOK, updateRoleResp.Body.String())
	}

	createTeamResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]string{
		"name": "Platform",
	}, adminToken)
	if createTeamResp.Code != http.StatusCreated {
		t.Fatalf("create team status = %d, want %d body=%s", createTeamResp.Code, http.StatusCreated, createTeamResp.Body.String())
	}
	var team store.Team
	decodeResponse(t, createTeamResp, &team)

	addMemberResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/users", map[string]string{
		"user_id":   alice.ID,
		"role":      store.TeamRoleMember,
		"job_title": "Engineer",
	}, adminToken)
	if addMemberResp.Code != http.StatusOK {
		t.Fatalf("add team member status = %d, want %d body=%s", addMemberResp.Code, http.StatusOK, addMemberResp.Body.String())
	}

	listMembersResp := doJSONRequest(t, handler, http.MethodGet, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/users", nil, aliceAuth.Token)
	if listMembersResp.Code != http.StatusOK {
		t.Fatalf("list team members status = %d, want %d", listMembersResp.Code, http.StatusOK)
	}
	var members []store.TeamMember
	decodeResponse(t, listMembersResp, &members)
	if len(members) < 2 {
		t.Fatalf("team members len = %d, want at least 2", len(members))
	}

	agent, _, err := store.CreateAgent(context.Background(), db, "team-secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	addAgentResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/agents", map[string]string{
		"agent_id": agent.ID,
	}, adminToken)
	if addAgentResp.Code != http.StatusOK {
		t.Fatalf("add team agent status = %d, want %d body=%s", addAgentResp.Code, http.StatusOK, addAgentResp.Body.String())
	}

	listAgentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/agents", nil, aliceAuth.Token)
	if listAgentsResp.Code != http.StatusOK {
		t.Fatalf("list team agents status = %d, want %d", listAgentsResp.Code, http.StatusOK)
	}
	var teamAgents []store.TeamAgent
	decodeResponse(t, listAgentsResp, &teamAgents)
	if len(teamAgents) != 1 || teamAgents[0].AgentID != agent.ID {
		t.Fatalf("team agents = %#v", teamAgents)
	}

	updateTeamResp := doJSONRequest(t, handler, http.MethodPut, "/api/teams/"+strconv.FormatInt(team.ID, 10), map[string]string{
		"name": "Platform Core",
	}, adminToken)
	if updateTeamResp.Code != http.StatusOK {
		t.Fatalf("update team status = %d, want %d body=%s", updateTeamResp.Code, http.StatusOK, updateTeamResp.Body.String())
	}

	removeAgentResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/agents/"+agent.ID, nil, adminToken)
	if removeAgentResp.Code != http.StatusOK {
		t.Fatalf("remove team agent status = %d, want %d", removeAgentResp.Code, http.StatusOK)
	}

	removeMemberResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/users/"+alice.ID, nil, adminToken)
	if removeMemberResp.Code != http.StatusOK {
		t.Fatalf("remove team member status = %d, want %d", removeMemberResp.Code, http.StatusOK)
	}
	removeAdminMemberResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+strconv.FormatInt(team.ID, 10)+"/users/"+adminUser.ID, nil, adminToken)
	if removeAdminMemberResp.Code != http.StatusOK {
		t.Fatalf("remove admin team member status = %d, want %d body=%s", removeAdminMemberResp.Code, http.StatusOK, removeAdminMemberResp.Body.String())
	}

	deleteTeamResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+strconv.FormatInt(team.ID, 10), nil, adminToken)
	if deleteTeamResp.Code != http.StatusOK {
		t.Fatalf("delete team status = %d, want %d", deleteTeamResp.Code, http.StatusOK)
	}

	deleteRoleResp := doJSONRequest(t, handler, http.MethodDelete, "/api/roles/"+strconv.FormatInt(role.ID, 10), nil, adminToken)
	if deleteRoleResp.Code != http.StatusOK {
		t.Fatalf("delete role status = %d, want %d", deleteRoleResp.Code, http.StatusOK)
	}
}

func TestProjectTicketsHistoryAndStoriesAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	alice, err := store.CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	team, err := store.CreateTeam(context.Background(), db, "Platform", nil)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	ticket, err := store.CreateTicket(context.Background(), db, store.TicketCreateParams{
		ProjectID: 1,
		Type:      "task",
		Title:     "API project ticket",
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	badTicketsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/tickets?limit=abc", nil, adminToken)
	if badTicketsResp.Code != http.StatusBadRequest {
		t.Fatalf("bad tickets status = %d, want %d", badTicketsResp.Code, http.StatusBadRequest)
	}

	ticketsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/tickets?limit=5&offset=0&type=task", nil, adminToken)
	if ticketsResp.Code != http.StatusOK {
		t.Fatalf("tickets status = %d, want %d body=%s", ticketsResp.Code, http.StatusOK, ticketsResp.Body.String())
	}
	var tickets []store.Ticket
	decodeResponse(t, ticketsResp, &tickets)
	found := false
	for _, item := range tickets {
		if item.ID == ticket.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tickets response missing %q", ticket.ID)
	}

	badHistoryResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/history?limit=abc", nil, adminToken)
	if badHistoryResp.Code != http.StatusBadRequest {
		t.Fatalf("bad history status = %d, want %d", badHistoryResp.Code, http.StatusBadRequest)
	}

	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/history?limit=5", nil, adminToken)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d, want %d body=%s", historyResp.Code, http.StatusOK, historyResp.Body.String())
	}
	var events []store.HistoryEvent
	decodeResponse(t, historyResp, &events)
	if len(events) == 0 {
		t.Fatal("expected project history events")
	}

	addProjectUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/users", map[string]string{
		"user_id": alice.ID,
		"role":    store.ProjectRoleViewer,
	}, adminToken)
	if addProjectUserResp.Code != http.StatusOK {
		t.Fatalf("add project user status = %d, want %d body=%s", addProjectUserResp.Code, http.StatusOK, addProjectUserResp.Body.String())
	}

	projectUsersResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/users", nil, adminToken)
	if projectUsersResp.Code != http.StatusOK {
		t.Fatalf("project users status = %d, want %d", projectUsersResp.Code, http.StatusOK)
	}

	addProjectTeamResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/teams", map[string]any{
		"team_id": team.ID,
		"role":    store.ProjectRoleEditor,
	}, adminToken)
	if addProjectTeamResp.Code != http.StatusOK {
		t.Fatalf("add project team status = %d, want %d body=%s", addProjectTeamResp.Code, http.StatusOK, addProjectTeamResp.Body.String())
	}

	projectTeamsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/teams", nil, adminToken)
	if projectTeamsResp.Code != http.StatusOK {
		t.Fatalf("project teams status = %d, want %d", projectTeamsResp.Code, http.StatusOK)
	}

	storiesResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/stories", nil, adminToken)
	if storiesResp.Code != http.StatusOK {
		t.Fatalf("stories status = %d, want %d body=%s", storiesResp.Code, http.StatusOK, storiesResp.Body.String())
	}
	var stories []store.Story
	decodeResponse(t, storiesResp, &stories)
	if len(stories) != 0 {
		t.Fatalf("stories len = %d, want 0 for empty project stories", len(stories))
	}

	removeProjectTeamResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/1/teams/"+strconv.FormatInt(team.ID, 10), nil, adminToken)
	if removeProjectTeamResp.Code != http.StatusOK {
		t.Fatalf("remove project team status = %d, want %d", removeProjectTeamResp.Code, http.StatusOK)
	}

	removeProjectUserResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/1/users/"+alice.ID, nil, adminToken)
	if removeProjectUserResp.Code != http.StatusOK {
		t.Fatalf("remove project user status = %d, want %d", removeProjectUserResp.Code, http.StatusOK)
	}
}

func TestWorkflowStageRoleAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
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

	wf, err := store.CreateWorkflow(context.Background(), db, "API Workflow", "stage role api test")
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := store.AddWorkflowStage(context.Background(), db, wf.ID, "triage", "triage", "", 1)
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	roleA, err := store.CreateRole(context.Background(), db, &wf.ID, "reviewer", "reviews work", "")
	if err != nil {
		t.Fatalf("CreateRole(roleA) error = %v", err)
	}
	roleB, err := store.CreateRole(context.Background(), db, &wf.ID, "qa", "verifies work", "")
	if err != nil {
		t.Fatalf("CreateRole(roleB) error = %v", err)
	}
	stagePath := fmt.Sprintf("/api/workflows/stages/roles/%d/%d", wf.ID, stage.ID)

	unauthorized := doJSONRequest(t, handler, http.MethodPost, stagePath, map[string]int64{"role_id": roleA.ID}, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized add status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	forbidden := doJSONRequest(t, handler, http.MethodPost, stagePath, map[string]int64{"role_id": roleA.ID}, userAuth.Token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden add status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	invalid := doJSONRequest(t, handler, http.MethodPost, stagePath, map[string]int64{}, adminToken)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid add status = %d, want %d body=%s", invalid.Code, http.StatusBadRequest, invalid.Body.String())
	}

	added := doJSONRequest(t, handler, http.MethodPost, stagePath, map[string]int64{"role_id": roleA.ID}, adminToken)
	if added.Code != http.StatusCreated {
		t.Fatalf("add roleA status = %d, want %d body=%s", added.Code, http.StatusCreated, added.Body.String())
	}
	added = doJSONRequest(t, handler, http.MethodPost, stagePath, map[string]int64{"role_id": roleB.ID}, adminToken)
	if added.Code != http.StatusCreated {
		t.Fatalf("add roleB status = %d, want %d body=%s", added.Code, http.StatusCreated, added.Body.String())
	}

	reordered := doJSONRequest(t, handler, http.MethodPut, stagePath, map[string][]int64{"role_ids": {roleB.ID, roleA.ID}}, adminToken)
	if reordered.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want %d body=%s", reordered.Code, http.StatusOK, reordered.Body.String())
	}

	got, err := store.GetWorkflow(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(got.Stages) != 1 || len(got.Stages[0].Roles) != 2 {
		t.Fatalf("GetWorkflow() stages = %#v", got.Stages)
	}
	if got.Stages[0].Roles[0].ID != roleB.ID || got.Stages[0].Roles[1].ID != roleA.ID {
		t.Fatalf("role order = %#v, want [%d %d]", got.Stages[0].Roles, roleB.ID, roleA.ID)
	}

	removed := doJSONRequest(t, handler, http.MethodDelete, fmt.Sprintf("%s/%d", stagePath, roleA.ID), nil, adminToken)
	if removed.Code != http.StatusOK {
		t.Fatalf("remove status = %d, want %d body=%s", removed.Code, http.StatusOK, removed.Body.String())
	}

	got, err = store.GetWorkflow(context.Background(), db, wf.ID)
	if err != nil {
		t.Fatalf("GetWorkflow(after remove) error = %v", err)
	}
	if remaining := got.Stages[0].Roles; len(remaining) != 1 || remaining[0].ID != roleB.ID {
		t.Fatalf("remaining roles = %#v, want only roleB", remaining)
	}
}

func TestProjectAPI(t *testing.T) {
	t.Parallel()
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

	enableResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(projects[1].ID, 10)+"/enable", nil, auth.Token)
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable project status = %d, want %d body=%s", enableResp.Code, http.StatusOK, enableResp.Body.String())
	}

	deleteProjectResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/"+strconv.FormatInt(projects[1].ID, 10), nil, auth.Token)
	if deleteProjectResp.Code != http.StatusOK {
		t.Fatalf("delete project status = %d, want %d body=%s", deleteProjectResp.Code, http.StatusOK, deleteProjectResp.Body.String())
	}
}

func TestRoleAPI(t *testing.T) {
	t.Parallel()
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

func TestRoleAPIValidationAndAuthPaths(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	if _, err := store.CreateUser(context.Background(), db, "alice", "password123", "user"); err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	aliceLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	var aliceAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, aliceLogin, &aliceAuth)

	cases := []struct {
		name    string
		method  string
		path    string
		payload any
		token   string
		want    int
	}{
		{"list requires admin", http.MethodGet, "/api/roles", nil, aliceAuth.Token, http.StatusForbidden},
		{"create requires admin", http.MethodPost, "/api/roles", map[string]string{"title": "x"}, aliceAuth.Token, http.StatusForbidden},
		{"collection rejects patch", http.MethodPatch, "/api/roles", nil, adminToken, http.StatusMethodNotAllowed},
		{"bad id", http.MethodPut, "/api/roles/not-a-number", map[string]string{"title": "x"}, adminToken, http.StatusBadRequest},
		{"missing update", http.MethodPut, "/api/roles/999999", map[string]string{"title": "x"}, adminToken, http.StatusNotFound},
		{"missing delete", http.MethodDelete, "/api/roles/999999", nil, adminToken, http.StatusNotFound},
		{"item rejects get", http.MethodGet, "/api/roles/1", nil, adminToken, http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSONRequest(t, handler, tc.method, tc.path, tc.payload, tc.token)
			if resp.Code != tc.want {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, resp.Code, tc.want, resp.Body.String())
			}
		})
	}

	rawCreate := doRawRequest(t, handler, http.MethodPost, "/api/roles", []byte("{bad"), adminToken)
	if rawCreate.Code != http.StatusBadRequest {
		t.Fatalf("raw role create status = %d, want %d body=%s", rawCreate.Code, http.StatusBadRequest, rawCreate.Body.String())
	}
	rawUpdate := doRawRequest(t, handler, http.MethodPut, "/api/roles/1", []byte("{bad"), adminToken)
	if rawUpdate.Code != http.StatusBadRequest {
		t.Fatalf("raw role update status = %d, want %d body=%s", rawUpdate.Code, http.StatusBadRequest, rawUpdate.Body.String())
	}
}

func TestGuidanceMapsAPI(t *testing.T) {
	t.Parallel()
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

	createProjectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"prefix":              "MAP",
		"title":               "Guidance Project",
		"acceptance_criteria": "legacy project ac",
		"dor_map":             map[string]string{"develop": "project develop dor"},
		"dod_map":             map[string]string{"develop": "project develop dod"},
		"ac_map":              map[string]string{"qa": "project qa ac"},
	}, auth.Token)
	if createProjectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d body=%s", createProjectResp.Code, http.StatusCreated, createProjectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, createProjectResp, &project)
	if got := project.DORMap["develop"]; got != "project develop dor" {
		t.Fatalf("project.DORMap[develop] = %q", got)
	}
	if got := project.ACMap["default"]; got != "legacy project ac" {
		t.Fatalf("project.ACMap[default] = %q", got)
	}

	createRoleResp := doJSONRequest(t, handler, http.MethodPost, "/api/roles", map[string]any{
		"title":               "reviewer",
		"description":         "reviews work",
		"acceptance_criteria": "legacy role ac",
		"dor_map":             map[string]string{"develop": "role develop dor"},
		"dod_map":             map[string]string{"develop": "role develop dod"},
		"ac_map":              map[string]string{"qa": "role qa ac"},
	}, auth.Token)
	if createRoleResp.Code != http.StatusCreated {
		t.Fatalf("create role status = %d, want %d body=%s", createRoleResp.Code, http.StatusCreated, createRoleResp.Body.String())
	}
	var role store.Role
	decodeResponse(t, createRoleResp, &role)
	if got := role.DODMap["develop"]; got != "role develop dod" {
		t.Fatalf("role.DODMap[develop] = %q", got)
	}
	if got := role.ACMap["default"]; got != "legacy role ac" {
		t.Fatalf("role.ACMap[default] = %q", got)
	}

	createTicketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id":          project.ID,
		"type":                "task",
		"title":               "Mapped ticket",
		"acceptance_criteria": "legacy ticket ac",
		"dor_map":             map[string]string{"develop": "ticket develop dor"},
		"dod_map":             map[string]string{"develop": "ticket develop dod"},
		"ac_map":              map[string]string{"qa": "ticket qa ac"},
	}, auth.Token)
	if createTicketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d, want %d body=%s", createTicketResp.Code, http.StatusCreated, createTicketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, createTicketResp, &ticket)
	if got := ticket.DORMap["develop"]; got != "ticket develop dor" {
		t.Fatalf("ticket.DORMap[develop] = %q", got)
	}
	if got := ticket.ACMap["default"]; got != "legacy ticket ac" {
		t.Fatalf("ticket.ACMap[default] = %q", got)
	}
	if ticket.Deleted {
		t.Fatalf("ticket.Deleted = true, want false")
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

	badStoriesResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/stories?offset=abc", nil, auth.Token)
	if badStoriesResp.Code != http.StatusBadRequest {
		t.Fatalf("bad stories status = %d body=%s", badStoriesResp.Code, badStoriesResp.Body.String())
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
	var epicID string
	for _, ticket := range tickets {
		if ticket.Type == "epic" {
			epicID = ticket.ID
			break
		}
	}
	if epicID == "" {
		t.Fatalf("expected generated epic ticket, got %#v", tickets)
	}

	analyseEpicResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+epicID+"/analyse", nil, auth.Token)
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
	t.Parallel()
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
	if createPayload.Agent.ID == "" {
		t.Fatalf("created agent id empty, want non-empty")
	}
	if createPayload.Password == "" {
		t.Fatalf("create password empty, want generated password")
	}
	agentUUID := createPayload.Agent.ID

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

	updatedResp := doJSONRequest(t, handler, http.MethodPut, "/api/agents/"+createPayload.Agent.ID, map[string]string{
		"password": "new-password",
	}, adminAuth.Token)
	if updatedResp.Code != http.StatusOK {
		t.Fatalf("update agent status = %d body=%s", updatedResp.Code, updatedResp.Body.String())
	}

	registerResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/register", agentUUID, "new-password", nil)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register agent status = %d body=%s", registerResp.Code, registerResp.Body.String())
	}

	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents/"+createPayload.Agent.ID+"/disable", nil, adminAuth.Token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable agent status = %d body=%s", disableResp.Code, disableResp.Body.String())
	}

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/agents/"+createPayload.Agent.ID, nil, adminAuth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete agent status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestTaskAPI(t *testing.T) {
	t.Parallel()
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
	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
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

	assignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      ticket.Status,
	}, auth.Token)
	if assignResp.Code != http.StatusOK {
		t.Fatalf("assign task status = %d body=%s", assignResp.Code, assignResp.Body.String())
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       "Add password reset flow",
		"description": "Email reset support",
		"parent_id":   epic.ID,
		"assignee":    "alice",
		"status":      "develop/active",
	}, aliceAuth.Token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update task status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID, nil, auth.Token)
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
	if len(filtered) != 1 || filtered[0].ID != ticket.ID {
		t.Fatalf("filtered tickets = %#v", filtered)
	}

	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/history", nil, auth.Token)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}
	var events []store.HistoryEvent
	decodeResponse(t, historyResp, &events)
	if len(events) < 2 {
		t.Fatalf("history events = %#v", events)
	}

	pagedHistoryResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/history?limit=1&offset=1", nil, auth.Token)
	if pagedHistoryResp.Code != http.StatusOK {
		t.Fatalf("paged history status = %d body=%s", pagedHistoryResp.Code, pagedHistoryResp.Body.String())
	}
	var pagedEvents []store.HistoryEvent
	decodeResponse(t, pagedHistoryResp, &pagedEvents)
	if len(pagedEvents) != 1 {
		t.Fatalf("paged history events = %#v", pagedEvents)
	}

	badHistoryResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/history?limit=abc", nil, auth.Token)
	if badHistoryResp.Code != http.StatusBadRequest {
		t.Fatalf("bad history status = %d body=%s", badHistoryResp.Code, badHistoryResp.Body.String())
	}

	commentResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/comments", map[string]string{
		"comment": "Waiting on API changes.",
	}, auth.Token)
	if commentResp.Code != http.StatusCreated {
		t.Fatalf("comment status = %d body=%s", commentResp.Code, commentResp.Body.String())
	}

	commentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/comments", nil, auth.Token)
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

	dependencyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/dependencies", nil, auth.Token)
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
	t.Parallel()
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
	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
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
	if len(tickets) != 1 || tickets[0].ID != ticket.ID {
		t.Fatalf("tickets = %#v", tickets)
	}

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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
	readyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/ready", nil, adminAuth.Token)
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
	if claimPayload.Status != "ASSIGNED" || claimPayload.Ticket.ID != ticket.ID || claimPayload.Ticket.Assignee != "alice" {
		t.Fatalf("claim payload = %#v", claimPayload)
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID, nil, adminAuth.Token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get ticket status = %d body=%s", getResp.Code, getResp.Body.String())
	}
}

func TestProjectInterventionsAPIListsFailedTickets(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Intervention Project",
		"visibility": "private",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	okResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Healthy ticket",
	}, adminToken)
	if okResp.Code != http.StatusCreated {
		t.Fatalf("create healthy ticket status = %d body=%s", okResp.Code, okResp.Body.String())
	}

	failedResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Needs intervention",
	}, adminToken)
	if failedResp.Code != http.StatusCreated {
		t.Fatalf("create failed ticket status = %d body=%s", failedResp.Code, failedResp.Body.String())
	}
	var failedTicket store.Ticket
	decodeResponse(t, failedResp, &failedTicket)

	setFailResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+failedTicket.ID, map[string]any{
		"title":       failedTicket.Title,
		"description": failedTicket.Description,
		"assignee":    failedTicket.Assignee,
		"priority":    failedTicket.Priority,
		"order":       failedTicket.Order,
		"state":       "fail",
	}, adminToken)
	if setFailResp.Code != http.StatusOK {
		t.Fatalf("set fail status = %d body=%s", setFailResp.Code, setFailResp.Body.String())
	}

	interventionsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/interventions", nil, adminToken)
	if interventionsResp.Code != http.StatusOK {
		t.Fatalf("list interventions status = %d body=%s", interventionsResp.Code, interventionsResp.Body.String())
	}
	var interventions []store.Ticket
	decodeResponse(t, interventionsResp, &interventions)
	if len(interventions) != 1 {
		t.Fatalf("expected 1 intervention ticket, got %d: %#v", len(interventions), interventions)
	}
	if interventions[0].ID != failedTicket.ID || interventions[0].State != "fail" {
		t.Fatalf("unexpected intervention ticket = %#v", interventions[0])
	}

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "outsider",
		"password": "password123",
	}, adminToken)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create outsider status = %d body=%s", createUserResp.Code, createUserResp.Body.String())
	}
	outsiderLoginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "outsider",
		"password": "password123",
	}, "")
	var outsiderAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, outsiderLoginResp, &outsiderAuth)

	outsiderResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/interventions", nil, outsiderAuth.Token)
	if outsiderResp.Code != http.StatusForbidden {
		t.Fatalf("outsider interventions status = %d body=%s", outsiderResp.Code, outsiderResp.Body.String())
	}
}

func TestTicketInterventionDecisionAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Intervention Decisions",
		"visibility": "private",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Flaky deploy check",
	}, adminToken)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", createResp.Code, createResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, createResp, &ticket)

	setFailResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    ticket.Assignee,
		"priority":    ticket.Priority,
		"order":       ticket.Order,
		"state":       "fail",
	}, adminToken)
	if setFailResp.Code != http.StatusOK {
		t.Fatalf("set fail status = %d body=%s", setFailResp.Code, setFailResp.Body.String())
	}

	splitResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/intervene", map[string]any{
		"outcome": "split-work",
		"message": "Break this into smaller follow-up work.",
	}, adminToken)
	if splitResp.Code != http.StatusOK {
		t.Fatalf("split-work intervention status = %d body=%s", splitResp.Code, splitResp.Body.String())
	}
	var splitPayload struct {
		Ticket   store.Ticket  `json:"ticket"`
		FollowUp *store.Ticket `json:"follow_up"`
		Decision string        `json:"decision"`
	}
	decodeResponse(t, splitResp, &splitPayload)
	if splitPayload.Decision != "split-work" {
		t.Fatalf("decision = %q, want split-work", splitPayload.Decision)
	}
	if splitPayload.Ticket.State != "idle" {
		t.Fatalf("ticket state = %q, want idle", splitPayload.Ticket.State)
	}
	if splitPayload.FollowUp == nil || splitPayload.FollowUp.ID == "" {
		t.Fatalf("expected follow-up ticket in response, got %#v", splitPayload.FollowUp)
	}

	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/history", nil, adminToken)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("ticket history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}
	var history []store.HistoryEvent
	decodeResponse(t, historyResp, &history)
	foundDecision := false
	for _, item := range history {
		if item.EventType == "ticket_intervention_decided" && strings.Contains(item.Payload, "split-work") {
			foundDecision = true
			break
		}
	}
	if !foundDecision {
		t.Fatalf("missing ticket_intervention_decided history event: %#v", history)
	}

	resetFailResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       splitPayload.Ticket.Title,
		"description": splitPayload.Ticket.Description,
		"assignee":    splitPayload.Ticket.Assignee,
		"priority":    splitPayload.Ticket.Priority,
		"order":       splitPayload.Ticket.Order,
		"state":       "fail",
	}, adminToken)
	if resetFailResp.Code != http.StatusOK {
		t.Fatalf("reset fail status = %d body=%s", resetFailResp.Code, resetFailResp.Body.String())
	}

	invalidResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/intervene", map[string]any{
		"outcome": "unknown",
	}, adminToken)
	if invalidResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid intervention outcome status = %d body=%s", invalidResp.Code, invalidResp.Body.String())
	}
}

func TestInterventionAndWorkItemAccessRequiresEditorOrOwner(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Public Access Controls",
		"visibility": "public",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Requires intervention visibility controls",
	}, adminToken)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	setFailResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    ticket.Assignee,
		"priority":    ticket.Priority,
		"order":       ticket.Order,
		"state":       "fail",
	}, adminToken)
	if setFailResp.Code != http.StatusOK {
		t.Fatalf("set fail status = %d body=%s", setFailResp.Code, setFailResp.Body.String())
	}

	createUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "vieweruser",
		"password": "password123",
	}, adminToken)
	if createUserResp.Code != http.StatusCreated {
		t.Fatalf("create viewer status = %d body=%s", createUserResp.Code, createUserResp.Body.String())
	}
	var viewer store.User
	decodeResponse(t, createUserResp, &viewer)

	viewerLogin := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "vieweruser",
		"password": "password123",
	}, "")
	if viewerLogin.Code != http.StatusOK {
		t.Fatalf("viewer login status = %d body=%s", viewerLogin.Code, viewerLogin.Body.String())
	}
	var viewerAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, viewerLogin, &viewerAuth)

	interventionsAsViewer := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/interventions", nil, viewerAuth.Token)
	if interventionsAsViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer interventions status = %d body=%s", interventionsAsViewer.Code, interventionsAsViewer.Body.String())
	}

	workItemsAsViewer := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items", nil, viewerAuth.Token)
	if workItemsAsViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer work-items status = %d body=%s", workItemsAsViewer.Code, workItemsAsViewer.Body.String())
	}

	addEditorResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/users", map[string]any{
		"user_id": viewer.ID,
		"role":    store.ProjectRoleEditor,
	}, adminToken)
	if addEditorResp.Code != http.StatusOK {
		t.Fatalf("add editor membership status = %d body=%s", addEditorResp.Code, addEditorResp.Body.String())
	}

	interventionsAsEditor := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+strconv.FormatInt(project.ID, 10)+"/interventions", nil, viewerAuth.Token)
	if interventionsAsEditor.Code != http.StatusOK {
		t.Fatalf("editor interventions status = %d body=%s", interventionsAsEditor.Code, interventionsAsEditor.Body.String())
	}

	workItemsAsEditor := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items", nil, viewerAuth.Token)
	if workItemsAsEditor.Code != http.StatusOK {
		t.Fatalf("editor work-items status = %d body=%s", workItemsAsEditor.Code, workItemsAsEditor.Body.String())
	}
}

func TestTicketWorkItemsAPIQueryFilters(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Work Item Filters API",
		"visibility": "private",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Work item filtering",
		"assignee":   "admin",
	}, adminToken)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	setActiveResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    "admin",
		"priority":    ticket.Priority,
		"order":       ticket.Order,
		"state":       "active",
	}, adminToken)
	if setActiveResp.Code != http.StatusOK {
		t.Fatalf("set active status = %d body=%s", setActiveResp.Code, setActiveResp.Body.String())
	}
	var activeTicket store.Ticket
	decodeResponse(t, setActiveResp, &activeTicket)

	setSuccessResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       activeTicket.Title,
		"description": activeTicket.Description,
		"assignee":    activeTicket.Assignee,
		"priority":    activeTicket.Priority,
		"order":       activeTicket.Order,
		"state":       "success",
	}, adminToken)
	if setSuccessResp.Code != http.StatusOK {
		t.Fatalf("set success status = %d body=%s", setSuccessResp.Code, setSuccessResp.Body.String())
	}

	successItemsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items?status=success", nil, adminToken)
	if successItemsResp.Code != http.StatusOK {
		t.Fatalf("list success work-items status = %d body=%s", successItemsResp.Code, successItemsResp.Body.String())
	}
	var successItems []store.WorkItem
	decodeResponse(t, successItemsResp, &successItems)
	if len(successItems) != 1 || successItems[0].Status != store.WorkItemStatusSuccess {
		t.Fatalf("success work-items = %#v", successItems)
	}

	humanItemsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items?assignee_type=human", nil, adminToken)
	if humanItemsResp.Code != http.StatusOK {
		t.Fatalf("list human work-items status = %d body=%s", humanItemsResp.Code, humanItemsResp.Body.String())
	}
	var humanItems []store.WorkItem
	decodeResponse(t, humanItemsResp, &humanItems)
	if len(humanItems) != 1 {
		t.Fatalf("human work-items len = %d, want 1", len(humanItems))
	}

	invalidStatusResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items?status=unknown", nil, adminToken)
	if invalidStatusResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid status filter code = %d body=%s", invalidStatusResp.Code, invalidStatusResp.Body.String())
	}

	invalidAssigneeResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items?assignee_type=robot", nil, adminToken)
	if invalidAssigneeResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid assignee filter code = %d body=%s", invalidAssigneeResp.Code, invalidAssigneeResp.Body.String())
	}
}

func TestTicketWorkItemActionAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)

	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]any{
		"title":      "Work Item Actions API",
		"visibility": "private",
	}, adminToken)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)

	createBobResp := doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "bob-workitem",
		"password": "password123",
	}, adminToken)
	if createBobResp.Code != http.StatusCreated {
		t.Fatalf("create user bob status = %d body=%s", createBobResp.Code, createBobResp.Body.String())
	}
	var bob store.User
	decodeResponse(t, createBobResp, &bob)

	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": project.ID,
		"type":       "task",
		"title":      "Work-item lifecycle actions",
		"assignee":   "admin",
	}, adminToken)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	setActiveResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"title":       ticket.Title,
		"description": ticket.Description,
		"assignee":    ticket.Assignee,
		"priority":    ticket.Priority,
		"order":       ticket.Order,
		"state":       "active",
	}, adminToken)
	if setActiveResp.Code != http.StatusOK {
		t.Fatalf("set active status = %d body=%s", setActiveResp.Code, setActiveResp.Body.String())
	}

	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/work-items?status=active", nil, adminToken)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list active work-items status = %d body=%s", listResp.Code, listResp.Body.String())
	}
	var activeItems []store.WorkItem
	decodeResponse(t, listResp, &activeItems)
	if len(activeItems) != 1 {
		t.Fatalf("active items len = %d, want 1", len(activeItems))
	}
	activeID := activeItems[0].ID

	reassignResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/work-items/"+activeID+"/reassign", map[string]any{
		"assignee": "bob-workitem",
		"message":  "handoff",
	}, adminToken)
	if reassignResp.Code != http.StatusOK {
		t.Fatalf("reassign work-item status = %d body=%s", reassignResp.Code, reassignResp.Body.String())
	}
	var reassigned store.WorkItem
	decodeResponse(t, reassignResp, &reassigned)
	if reassigned.AssigneeID != bob.ID {
		t.Fatalf("reassigned assignee_id = %q, want %q", reassigned.AssigneeID, bob.ID)
	}

	cancelResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/work-items/"+activeID+"/cancel", map[string]any{
		"message": "stop this attempt",
	}, adminToken)
	if cancelResp.Code != http.StatusOK {
		t.Fatalf("cancel work-item status = %d body=%s", cancelResp.Code, cancelResp.Body.String())
	}
	var cancelled store.WorkItem
	decodeResponse(t, cancelResp, &cancelled)
	if cancelled.Status != store.WorkItemStatusStopped {
		t.Fatalf("cancelled status = %q, want %q", cancelled.Status, store.WorkItemStatusStopped)
	}

	retryResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/work-items/"+activeID+"/retry", map[string]any{
		"assignee": "bob-workitem",
	}, adminToken)
	if retryResp.Code != http.StatusOK {
		t.Fatalf("retry work-item status = %d body=%s", retryResp.Code, retryResp.Body.String())
	}
	var retried store.WorkItem
	decodeResponse(t, retryResp, &retried)
	if retried.Status != store.WorkItemStatusActive || retried.ID == activeID {
		t.Fatalf("retried work-item = %#v, want new active item", retried)
	}

	invalidActionResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/work-items/"+retried.ID+"/unknown", map[string]any{}, adminToken)
	if invalidActionResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status = %d body=%s", invalidActionResp.Code, invalidActionResp.Body.String())
	}
}

func TestCountAPIAndAssignmentRules(t *testing.T) {
	t.Parallel()
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
	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
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

	claimResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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
	bobUser, err := store.GetUserByUsername(context.Background(), db, "bob")
	if err != nil {
		t.Fatalf("get bob user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, bobUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add bob editor membership error = %v", err)
	}

	claimConflictResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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

	overrideResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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

	missingAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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
	disabledAssignResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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

	statusForbiddenResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
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
	t.Parallel()
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
	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
	if err != nil {
		t.Fatalf("get alice user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, aliceUser.ID, store.ProjectRoleEditor); err != nil {
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
	readyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+openTask.ID+"/ready", nil, adminAuth.Token)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready open task status = %d body=%s", readyResp.Code, readyResp.Body.String())
	}
	readyResp2 := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+otherTask.ID+"/ready", nil, adminAuth.Token)
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

	inProgressResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+openTask.ID, map[string]any{
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
	bobUser, err := store.GetUserByUsername(context.Background(), db, "bob")
	if err != nil {
		t.Fatalf("get bob user error = %v", err)
	}
	if _, err := store.AddProjectMember(context.Background(), db, project.ID, bobUser.ID, store.ProjectRoleEditor); err != nil {
		t.Fatalf("add bob editor membership error = %v", err)
	}

	// Assign otherTask to alice so bob's claim is rejected
	assignOtherResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+otherTask.ID, map[string]any{
		"title":       otherTask.Title,
		"description": otherTask.Description,
		"assignee":    "alice",
		"state":       "active",
	}, adminAuth.Token)
	if assignOtherResp.Code != http.StatusOK {
		t.Fatalf("assign otherTask status = %d body=%s", assignOtherResp.Code, assignOtherResp.Body.String())
	}

	rejectedResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/claim", map[string]any{
		"project_id": project.ID,
		"ticket_id":  otherTask.ID,
	}, bobAuth.Token)
	if rejectedResp.Code != http.StatusOK {
		t.Fatalf("request rejected status = %d body=%s", rejectedResp.Code, rejectedResp.Body.String())
	}
	var rejectedPayload map[string]any
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
	t.Parallel()
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

	cloneResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+epic.ID+"/clone", nil, auth.Token)
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
	t.Parallel()
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

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+ticket.ID, nil, auth.Token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID, nil, auth.Token)
	if getResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want %d body=%s", getResp.Code, http.StatusNotFound, getResp.Body.String())
	}
}

func TestDeleteTicketAPIFailsWhenTaskHasChildren(t *testing.T) {
	t.Parallel()
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

	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+parent.ID, nil, auth.Token)
	if deleteResp.Code != http.StatusBadRequest {
		t.Fatalf("delete parent status = %d, want %d body=%s", deleteResp.Code, http.StatusBadRequest, deleteResp.Body.String())
	}
}

func TestAPIMethodValidation(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	resp := doJSONRequest(t, handler, http.MethodGet, "/api/users", nil, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth code = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func TestAutoProgressTicketLifecycleDesignToDevelop(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
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
	t.Parallel()
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

	aliceUser, err := store.GetUserByUsername(context.Background(), db, "alice")
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

func TestTicketStateOpsAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a ticket to exercise state operations on.
	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "State ops ticket",
	}, token)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	// POST /api/tickets/{ref}/close
	closeResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/close", nil, token)
	if closeResp.Code != http.StatusOK {
		t.Fatalf("close ticket status = %d body=%s", closeResp.Code, closeResp.Body.String())
	}

	// POST /api/tickets/{ref}/open
	openResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/open", nil, token)
	if openResp.Code != http.StatusOK {
		t.Fatalf("open ticket status = %d body=%s", openResp.Code, openResp.Body.String())
	}

	// POST /api/tickets/{ref}/archive
	archiveResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/archive", nil, token)
	if archiveResp.Code != http.StatusOK {
		t.Fatalf("archive ticket status = %d body=%s", archiveResp.Code, archiveResp.Body.String())
	}

	// POST /api/tickets/{ref}/unarchive
	unarchiveResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/unarchive", nil, token)
	if unarchiveResp.Code != http.StatusOK {
		t.Fatalf("unarchive ticket status = %d body=%s", unarchiveResp.Code, unarchiveResp.Body.String())
	}

	// POST /api/tickets/{ref}/notready
	notreadyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/notready", nil, token)
	if notreadyResp.Code != http.StatusOK {
		t.Fatalf("notready ticket status = %d body=%s", notreadyResp.Code, notreadyResp.Body.String())
	}

	// POST /api/tickets/{ref}/health
	healthResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/health", map[string]any{
		"score": 3,
	}, token)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health ticket status = %d body=%s", healthResp.Code, healthResp.Body.String())
	}
	var healthTicket store.Ticket
	decodeResponse(t, healthResp, &healthTicket)
	if healthTicket.HealthScore != 3 {
		t.Fatalf("health score = %d, want 3", healthTicket.HealthScore)
	}

	// Create a workflow to assign to the ticket.
	wfResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows", map[string]string{
		"name":        "Test WF",
		"description": "for ticket workflow test",
	}, token)
	if wfResp.Code != http.StatusCreated {
		t.Fatalf("create workflow status = %d body=%s", wfResp.Code, wfResp.Body.String())
	}
	var wf store.Workflow
	decodeResponse(t, wfResp, &wf)

	// POST /api/tickets/{ref}/workflow
	setWfResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/workflow", map[string]any{
		"workflow_id": wf.ID,
	}, token)
	if setWfResp.Code != http.StatusOK {
		t.Fatalf("set ticket workflow status = %d body=%s", setWfResp.Code, setWfResp.Body.String())
	}

	// DELETE /api/tickets/{ref}/workflow
	unsetWfResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+ticket.ID+"/workflow", nil, token)
	if unsetWfResp.Code != http.StatusOK {
		t.Fatalf("unset ticket workflow status = %d body=%s", unsetWfResp.Code, unsetWfResp.Body.String())
	}

	for _, op := range []string{"complete", "reopen", "draft", "undraft"} {
		resp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/"+op, map[string]string{
			"message": op + " through public API",
		}, token)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s ticket status = %d body=%s", op, resp.Code, resp.Body.String())
		}
	}
	nextResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/next", nil, token)
	if nextResp.Code != http.StatusBadRequest {
		t.Fatalf("next ticket status = %d, want %d body=%s", nextResp.Code, http.StatusBadRequest, nextResp.Body.String())
	}
	previousResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/previous", nil, token)
	if previousResp.Code != http.StatusBadRequest {
		t.Fatalf("previous ticket status = %d, want %d body=%s", previousResp.Code, http.StatusBadRequest, previousResp.Body.String())
	}
}

func TestRegistrationConfigAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Disable registration
	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/config/registration", map[string]bool{
		"enabled": false,
	}, token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable registration status = %d body=%s", disableResp.Code, disableResp.Body.String())
	}
	var disablePayload map[string]any
	decodeResponse(t, disableResp, &disablePayload)
	if got, ok := disablePayload["registration_enabled"].(bool); !ok || got {
		t.Fatalf("registration_enabled = %v, want false", disablePayload["registration_enabled"])
	}

	// Enable registration
	enableResp := doJSONRequest(t, handler, http.MethodPost, "/api/config/registration", map[string]bool{
		"enabled": true,
	}, token)
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable registration status = %d body=%s", enableResp.Code, enableResp.Body.String())
	}
	var enablePayload map[string]any
	decodeResponse(t, enableResp, &enablePayload)
	if got, ok := enablePayload["registration_enabled"].(bool); !ok || !got {
		t.Fatalf("registration_enabled = %v, want true", enablePayload["registration_enabled"])
	}
}

func TestRegistrationConfigAPIRejectsUnauthorizedForbiddenAndInvalidJSON(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	registerResp := doJSONRequest(t, handler, http.MethodPost, "/api/register", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResp.Code, http.StatusCreated)
	}
	loginResp := doJSONRequest(t, handler, http.MethodPost, "/api/login", map[string]string{
		"username": "alice",
		"password": "password123",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("alice login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	var userAuth struct {
		Token string `json:"token"`
	}
	decodeResponse(t, loginResp, &userAuth)
	adminToken := loginAdmin(t, handler)

	unauthorized := doJSONRequest(t, handler, http.MethodPost, "/api/config/registration", map[string]bool{
		"enabled": false,
	}, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	forbidden := doJSONRequest(t, handler, http.MethodPost, "/api/config/registration", map[string]bool{
		"enabled": false,
	}, userAuth.Token)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want %d body=%s", forbidden.Code, http.StatusForbidden, forbidden.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/config/registration", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestChatLimitsConfigAPIDefaultsNonPositiveValues(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	resp := doJSONRequest(t, handler, http.MethodPost, "/api/config/chat_limits", map[string]int{
		"max_connections":      0,
		"max_duration_minutes": -5,
	}, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("chat limits defaulting status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var payload map[string]any
	decodeResponse(t, resp, &payload)
	if got := int(payload["chat_max_connections"].(float64)); got != store.DefaultChatMaxConnections {
		t.Fatalf("chat_max_connections = %d, want %d", got, store.DefaultChatMaxConnections)
	}
	if got := int(payload["chat_max_duration_minutes"].(float64)); got != store.DefaultChatMaxDurationMinutes {
		t.Fatalf("chat_max_duration_minutes = %d, want %d", got, store.DefaultChatMaxDurationMinutes)
	}
}

func TestSystemMetricsRejectsWrongMethod(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()

	adminToken := loginAdmin(t, handler)
	resp := doJSONRequest(t, handler, http.MethodPost, "/metrics", map[string]string{"noop": "x"}, adminToken)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("metrics POST status = %d, want %d body=%s", resp.Code, http.StatusMethodNotAllowed, resp.Body.String())
	}
}

func TestAgentWorkflowAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create an agent and extract credentials.
	createAgentResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents", map[string]string{
		"description": "Test agent",
	}, token)
	if createAgentResp.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d body=%s", createAgentResp.Code, createAgentResp.Body.String())
	}
	var agentPayload struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	decodeResponse(t, createAgentResp, &agentPayload)
	agentID := agentPayload.Agent.ID
	agentPass := agentPayload.Password

	// GET /api/agents/statuses
	statusesResp := doJSONRequest(t, handler, http.MethodGet, "/api/agents/statuses", nil, token)
	if statusesResp.Code != http.StatusOK {
		t.Fatalf("agent statuses status = %d body=%s", statusesResp.Code, statusesResp.Body.String())
	}

	// POST /api/agents/register (basic auth)
	registerResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/register", agentID, agentPass, nil)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("agent register status = %d body=%s", registerResp.Code, registerResp.Body.String())
	}

	// POST /api/agents/heartbeat (basic auth)
	heartbeatResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/heartbeat", agentID, agentPass, map[string]string{
		"status": "online",
	})
	if heartbeatResp.Code != http.StatusOK {
		t.Fatalf("agent heartbeat status = %d body=%s", heartbeatResp.Code, heartbeatResp.Body.String())
	}

	// Create a ticket for the agent to request.
	ticketResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "Agent work item",
	}, token)
	if ticketResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d body=%s", ticketResp.Code, ticketResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, ticketResp, &ticket)

	// Mark ticket ready so it can be claimed.
	readyResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/ready", nil, token)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready ticket status = %d body=%s", readyResp.Code, readyResp.Body.String())
	}

	// POST /api/agents/request (basic auth)
	requestResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/request", agentID, agentPass, map[string]any{
		"project_id": 1,
	})
	if requestResp.Code != http.StatusOK {
		t.Fatalf("agent request status = %d body=%s", requestResp.Code, requestResp.Body.String())
	}
	var requestPayload map[string]any
	decodeResponse(t, requestResp, &requestPayload)
	if requestPayload["status"] != "NEW" {
		t.Fatalf("agent request status = %v, want NEW", requestPayload["status"])
	}

	// POST /api/agents/{id}/tickets/{ticket_id}/update (basic auth)
	updateResp := doBasicAuthRequest(t, handler, http.MethodPost, "/api/agents/tickets/"+ticket.ID+"/update", agentID, agentPass, map[string]string{
		"result": "done",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("agent ticket update status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}
}

func TestWorkflowImportExportReorderAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a workflow with two stages.
	wfResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows", map[string]string{
		"name":        "Import Export WF",
		"description": "workflow for import/export test",
	}, token)
	if wfResp.Code != http.StatusCreated {
		t.Fatalf("create workflow status = %d body=%s", wfResp.Code, wfResp.Body.String())
	}
	var wf store.Workflow
	decodeResponse(t, wfResp, &wf)

	stage1Resp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows/"+strconv.FormatInt(wf.ID, 10)+"/stages", map[string]any{
		"stage_name":  "build",
		"description": "compile step",
		"sort_order":  0,
	}, token)
	if stage1Resp.Code != http.StatusCreated {
		t.Fatalf("add stage1 status = %d body=%s", stage1Resp.Code, stage1Resp.Body.String())
	}
	var stage1 store.WorkflowStage
	decodeResponse(t, stage1Resp, &stage1)

	stage2Resp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows/"+strconv.FormatInt(wf.ID, 10)+"/stages", map[string]any{
		"stage_name":  "deploy",
		"description": "deploy step",
		"sort_order":  1,
	}, token)
	if stage2Resp.Code != http.StatusCreated {
		t.Fatalf("add stage2 status = %d body=%s", stage2Resp.Code, stage2Resp.Body.String())
	}
	var stage2 store.WorkflowStage
	decodeResponse(t, stage2Resp, &stage2)

	// GET /api/workflows/{id}/export
	exportResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows/"+strconv.FormatInt(wf.ID, 10)+"/export", nil, token)
	if exportResp.Code != http.StatusOK {
		t.Fatalf("export workflow status = %d body=%s", exportResp.Code, exportResp.Body.String())
	}
	var export store.WorkflowExport
	decodeResponse(t, exportResp, &export)
	if export.Name != "Import Export WF" {
		t.Fatalf("export name = %q, want 'Import Export WF'", export.Name)
	}

	// POST /api/workflows/import (change name to avoid unique constraint)
	export.Name = "Imported WF Copy"
	importResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows/import", export, token)
	if importResp.Code != http.StatusCreated {
		t.Fatalf("import workflow status = %d body=%s", importResp.Code, importResp.Body.String())
	}

	// PUT /api/workflows/{id}/reorder
	reorderResp := doJSONRequest(t, handler, http.MethodPut, "/api/workflows/"+strconv.FormatInt(wf.ID, 10)+"/reorder", map[string]any{
		"stage_ids": []int64{stage2.ID, stage1.ID},
	}, token)
	if reorderResp.Code != http.StatusOK {
		t.Fatalf("reorder workflow status = %d body=%s", reorderResp.Code, reorderResp.Body.String())
	}
}

func TestProjectMembershipAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a project.
	projectResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects", map[string]string{
		"title": "Membership Test Project",
	}, token)
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body=%s", projectResp.Code, projectResp.Body.String())
	}
	var project store.Project
	decodeResponse(t, projectResp, &project)
	pid := strconv.FormatInt(project.ID, 10)

	// POST /api/projects/{id}/enable
	enableResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+pid+"/enable", nil, token)
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable project status = %d body=%s", enableResp.Code, enableResp.Body.String())
	}

	// POST /api/projects/{id}/disable
	disableResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+pid+"/disable", nil, token)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable project status = %d body=%s", disableResp.Code, disableResp.Body.String())
	}

	// Re-enable for further testing.
	doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+pid+"/enable", nil, token)

	// Create a user to add as member.
	doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "memberuser",
		"password": "password123",
	}, token)
	memberUser, err := store.GetUserByUsername(context.Background(), db, "memberuser")
	if err != nil {
		t.Fatalf("get memberuser error = %v", err)
	}

	// GET /api/projects/{id}/users (empty initially)
	usersResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+pid+"/users", nil, token)
	if usersResp.Code != http.StatusOK {
		t.Fatalf("list project users status = %d body=%s", usersResp.Code, usersResp.Body.String())
	}

	// POST /api/projects/{id}/users
	addUserResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+pid+"/users", map[string]any{
		"user_id": memberUser.ID,
		"role":    "editor",
	}, token)
	if addUserResp.Code != http.StatusOK {
		t.Fatalf("add project user status = %d body=%s", addUserResp.Code, addUserResp.Body.String())
	}

	// DELETE /api/projects/{id}/users/{user_id}
	removeUserResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/"+pid+"/users/"+memberUser.ID, nil, token)
	if removeUserResp.Code != http.StatusOK {
		t.Fatalf("remove project user status = %d body=%s", removeUserResp.Code, removeUserResp.Body.String())
	}

	// Create a team.
	teamResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]any{
		"name": "Membership Team",
	}, token)
	if teamResp.Code != http.StatusCreated {
		t.Fatalf("create team status = %d body=%s", teamResp.Code, teamResp.Body.String())
	}
	var team store.Team
	decodeResponse(t, teamResp, &team)

	// GET /api/projects/{id}/teams
	teamsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+pid+"/teams", nil, token)
	if teamsResp.Code != http.StatusOK {
		t.Fatalf("list project teams status = %d body=%s", teamsResp.Code, teamsResp.Body.String())
	}

	// POST /api/projects/{id}/teams
	addTeamResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/"+pid+"/teams", map[string]any{
		"team_id": team.ID,
		"role":    "editor",
	}, token)
	if addTeamResp.Code != http.StatusOK {
		t.Fatalf("add project team status = %d body=%s", addTeamResp.Code, addTeamResp.Body.String())
	}

	// DELETE /api/projects/{id}/teams/{team_id}
	removeTeamResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/"+pid+"/teams/"+strconv.FormatInt(team.ID, 10), nil, token)
	if removeTeamResp.Code != http.StatusOK {
		t.Fatalf("remove project team status = %d body=%s", removeTeamResp.Code, removeTeamResp.Body.String())
	}

	// GET /api/projects/{id}/stories (empty list is fine)
	storiesResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+pid+"/stories", nil, token)
	if storiesResp.Code != http.StatusOK {
		t.Fatalf("list project stories status = %d body=%s", storiesResp.Code, storiesResp.Body.String())
	}

	// GET /api/projects/{id}/history
	historyResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/"+pid+"/history", nil, token)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("project history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}
}

func TestTeamCRUDAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a team.
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams", map[string]any{
		"name": "CRUD Team",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create team status = %d body=%s", createResp.Code, createResp.Body.String())
	}
	var team store.Team
	decodeResponse(t, createResp, &team)
	tid := strconv.FormatInt(team.ID, 10)

	// PUT /api/teams/{id}
	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/teams/"+tid, map[string]any{
		"name": "Updated CRUD Team",
	}, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update team status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}
	var updatedTeam store.Team
	decodeResponse(t, updateResp, &updatedTeam)
	if updatedTeam.Name != "Updated CRUD Team" {
		t.Fatalf("updated team name = %q, want 'Updated CRUD Team'", updatedTeam.Name)
	}

	// Create a user and add to team.
	doJSONRequest(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "teamuser",
		"password": "password123",
	}, token)
	teamUser, err := store.GetUserByUsername(context.Background(), db, "teamuser")
	if err != nil {
		t.Fatalf("get teamuser error = %v", err)
	}

	// POST /api/teams/{id}/users (add member) - admin is already owner from create
	addMemberResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams/"+tid+"/users", map[string]any{
		"user_id":   teamUser.ID,
		"role":      "member",
		"job_title": "Developer",
	}, token)
	if addMemberResp.Code != http.StatusOK {
		t.Fatalf("add team member status = %d body=%s", addMemberResp.Code, addMemberResp.Body.String())
	}

	// GET /api/teams/{id}/users
	membersResp := doJSONRequest(t, handler, http.MethodGet, "/api/teams/"+tid+"/users", nil, token)
	if membersResp.Code != http.StatusOK {
		t.Fatalf("list team members status = %d body=%s", membersResp.Code, membersResp.Body.String())
	}

	// DELETE /api/teams/{id}/users/{user_id}
	removeMemberResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+tid+"/users/"+teamUser.ID, nil, token)
	if removeMemberResp.Code != http.StatusOK {
		t.Fatalf("remove team member status = %d body=%s", removeMemberResp.Code, removeMemberResp.Body.String())
	}

	// Create an agent and add to team.
	agentResp := doJSONRequest(t, handler, http.MethodPost, "/api/agents", map[string]string{
		"description": "Team agent",
	}, token)
	if agentResp.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d body=%s", agentResp.Code, agentResp.Body.String())
	}
	var agentPayload struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	decodeResponse(t, agentResp, &agentPayload)

	// POST /api/teams/{id}/agents
	addAgentResp := doJSONRequest(t, handler, http.MethodPost, "/api/teams/"+tid+"/agents", map[string]any{
		"agent_id": agentPayload.Agent.ID,
	}, token)
	if addAgentResp.Code != http.StatusOK {
		t.Fatalf("add team agent status = %d body=%s", addAgentResp.Code, addAgentResp.Body.String())
	}

	// GET /api/teams/{id}/agents
	agentsResp := doJSONRequest(t, handler, http.MethodGet, "/api/teams/"+tid+"/agents", nil, token)
	if agentsResp.Code != http.StatusOK {
		t.Fatalf("list team agents status = %d body=%s", agentsResp.Code, agentsResp.Body.String())
	}

	// DELETE /api/teams/{id}/agents/{agent_id}
	removeAgentResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+tid+"/agents/"+agentPayload.Agent.ID, nil, token)
	if removeAgentResp.Code != http.StatusOK {
		t.Fatalf("remove team agent status = %d body=%s", removeAgentResp.Code, removeAgentResp.Body.String())
	}

	// Remove the admin owner membership before deleting the team.
	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("get admin user error = %v", err)
	}
	removeOwnerResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+tid+"/users/"+adminUser.ID, nil, token)
	if removeOwnerResp.Code != http.StatusOK {
		t.Fatalf("remove team owner status = %d body=%s", removeOwnerResp.Code, removeOwnerResp.Body.String())
	}

	// DELETE /api/teams/{id}
	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/teams/"+tid, nil, token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete team status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestStoryCRUDAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a story.
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/stories", map[string]any{
		"project_id":  1,
		"title":       "CRUD Story",
		"description": "A story for CRUD testing",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create story status = %d body=%s", createResp.Code, createResp.Body.String())
	}
	var story store.Story
	decodeResponse(t, createResp, &story)
	sid := strconv.FormatInt(story.ID, 10)

	// GET /api/stories/{id}
	getResp := doJSONRequest(t, handler, http.MethodGet, "/api/stories/"+sid, nil, token)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get story status = %d body=%s", getResp.Code, getResp.Body.String())
	}
	var fetched store.Story
	decodeResponse(t, getResp, &fetched)
	if fetched.Title != "CRUD Story" {
		t.Fatalf("story title = %q, want 'CRUD Story'", fetched.Title)
	}

	// PUT /api/stories/{id}
	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/stories/"+sid, map[string]any{
		"project_id":  1,
		"title":       "Updated Story",
		"description": "Updated description",
	}, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update story status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}
	var updated store.Story
	decodeResponse(t, updateResp, &updated)
	if updated.Title != "Updated Story" {
		t.Fatalf("updated story title = %q, want 'Updated Story'", updated.Title)
	}

	// DELETE /api/stories/{id}
	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/stories/"+sid, nil, token)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("delete story status = %d, want %d body=%s", deleteResp.Code, http.StatusNoContent, deleteResp.Body.String())
	}

	// Verify story is gone.
	getGoneResp := doJSONRequest(t, handler, http.MethodGet, "/api/stories/"+sid, nil, token)
	if getGoneResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted story status = %d, want %d", getGoneResp.Code, http.StatusNotFound)
	}
}

func TestDeleteLabelByIDAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	// Create a label on project 1.
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/labels", map[string]string{
		"name":  "deleteme",
		"color": "blue",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create label status = %d body=%s", createResp.Code, createResp.Body.String())
	}
	var label store.Label
	decodeResponse(t, createResp, &label)

	// DELETE /api/labels/{id}
	deleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete label status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestCreateTicketRejectsNonDesignStageAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	resp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "bad lifecycle",
		"stage":      "develop",
		"state":      "active",
	}, token)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("create ticket status = %d, want %d body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
	var payload map[string]string
	decodeResponse(t, resp, &payload)
	if payload["error"] != "new tickets must start in design stage" {
		t.Fatalf("create ticket error = %q", payload["error"])
	}
}

func TestUpdateTicketRejectsInvalidLifecycleCombinationAPI(t *testing.T) {
	t.Parallel()
	handler, db := testHandler(t)
	defer db.Close()
	token := loginAdmin(t, handler)

	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets", map[string]any{
		"project_id": 1,
		"type":       "task",
		"title":      "lifecycle update",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create ticket status = %d, want %d body=%s", createResp.Code, http.StatusCreated, createResp.Body.String())
	}
	var ticket store.Ticket
	decodeResponse(t, createResp, &ticket)

	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/tickets/"+ticket.ID, map[string]any{
		"stage": "done",
		"state": "idle",
	}, token)
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("update ticket status = %d, want %d body=%s", updateResp.Code, http.StatusBadRequest, updateResp.Body.String())
	}
	var payload map[string]string
	decodeResponse(t, updateResp, &payload)
	if payload["error"] != "invalid status \"done/idle\"" {
		t.Fatalf("update ticket error = %q", payload["error"])
	}
}

func testHandler(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := store.Init(dbPath, "admin", "password", static.SeedDatabase); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	srv, err := New(":0", db, "1.2.3", false, nil, "", "")
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
	t.Parallel()
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
	t.Parallel()
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

	badListResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows?offset=abc", nil, token)
	if badListResp.Code != http.StatusBadRequest {
		t.Fatalf("bad workflow list status = %d body=%s", badListResp.Code, badListResp.Body.String())
	}

	// Create workflow
	createResp := doJSONRequest(t, handler, http.MethodPost, "/api/workflows", map[string]any{
		"name":             "CI Pipeline",
		"description":      "build, test, deploy",
		"approval_policy":  "all_roles",
		"progression_mode": "stage_only",
	}, token)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create workflow status = %d, body=%s", createResp.Code, createResp.Body.String())
	}
	var created store.Workflow
	decodeResponse(t, createResp, &created)
	if created.Name != "CI Pipeline" {
		t.Fatalf("created workflow name = %q", created.Name)
	}
	if created.ApprovalPolicy != "all_roles" {
		t.Fatalf("created workflow approval_policy = %q, want all_roles", created.ApprovalPolicy)
	}
	if created.ProgressionMode != "stage_only" {
		t.Fatalf("created workflow progression_mode = %q, want stage_only", created.ProgressionMode)
	}

	// Update workflow policy fields.
	updateResp := doJSONRequest(t, handler, http.MethodPut, "/api/workflows/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"name":             "CI Pipeline Updated",
		"description":      "updated description",
		"approval_policy":  "single_role",
		"progression_mode": "linear",
	}, token)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update workflow status = %d body=%s", updateResp.Code, updateResp.Body.String())
	}
	var updated store.Workflow
	decodeResponse(t, updateResp, &updated)
	if updated.Name != "CI Pipeline Updated" {
		t.Fatalf("updated workflow name = %q", updated.Name)
	}
	if updated.ApprovalPolicy != "single_role" {
		t.Fatalf("updated workflow approval_policy = %q, want single_role", updated.ApprovalPolicy)
	}
	if updated.ProgressionMode != "linear" {
		t.Fatalf("updated workflow progression_mode = %q, want linear", updated.ProgressionMode)
	}

	// Reject invalid workflow progression mode.
	badUpdateResp := doJSONRequest(t, handler, http.MethodPut, "/api/workflows/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"name":             "CI Pipeline Updated",
		"description":      "updated description",
		"approval_policy":  "single_role",
		"progression_mode": "invalid",
	}, token)
	if badUpdateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid workflow update status = %d body=%s", badUpdateResp.Code, badUpdateResp.Body.String())
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
	getStageResp := doJSONRequest(t, handler, http.MethodGet, "/api/workflows/stages/"+strconv.FormatInt(stage.ID, 10), nil, token)
	if getStageResp.Code != http.StatusOK {
		t.Fatalf("get stage status = %d body=%s", getStageResp.Code, getStageResp.Body.String())
	}

	updateStageResp := doJSONRequest(t, handler, http.MethodPut, "/api/workflows/stages/"+strconv.FormatInt(stage.ID, 10), map[string]any{
		"stage_name":         "package",
		"ways_of_working":    "package artifacts",
		"definition_of_done": "artifact published",
	}, token)
	if updateStageResp.Code != http.StatusOK {
		t.Fatalf("update stage status = %d body=%s", updateStageResp.Code, updateStageResp.Body.String())
	}

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
	t.Parallel()
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

	badLabelsResp := doJSONRequest(t, handler, http.MethodGet, "/api/projects/1/labels?limit=abc", nil, token)
	if badLabelsResp.Code != http.StatusBadRequest {
		t.Fatalf("bad labels status = %d body=%s", badLabelsResp.Code, badLabelsResp.Body.String())
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
	addLabelResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/labels", map[string]any{
		"label_id": label.ID,
	}, token)
	if addLabelResp.Code != http.StatusOK && addLabelResp.Code != http.StatusCreated {
		t.Fatalf("add label status = %d, body=%s", addLabelResp.Code, addLabelResp.Body.String())
	}

	// List ticket labels
	ticketLabelsResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/labels", nil, token)
	if ticketLabelsResp.Code != http.StatusOK {
		t.Fatalf("list ticket labels status = %d", ticketLabelsResp.Code)
	}

	// Remove label from ticket
	removeLabelResp := doJSONRequest(t, handler, http.MethodDelete, "/api/tickets/"+ticket.ID+"/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if removeLabelResp.Code != http.StatusOK {
		t.Fatalf("remove ticket label status = %d", removeLabelResp.Code)
	}

	projectDeleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/1/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if projectDeleteResp.Code != http.StatusOK {
		t.Fatalf("project label delete status = %d body=%s", projectDeleteResp.Code, projectDeleteResp.Body.String())
	}

	missingProjectDeleteResp := doJSONRequest(t, handler, http.MethodDelete, "/api/projects/1/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if missingProjectDeleteResp.Code != http.StatusNotFound {
		t.Fatalf("missing project label delete status = %d body=%s", missingProjectDeleteResp.Code, missingProjectDeleteResp.Body.String())
	}

	recreateResp := doJSONRequest(t, handler, http.MethodPost, "/api/projects/1/labels", map[string]string{
		"name":  "urgent-again",
		"color": "red",
	}, token)
	if recreateResp.Code != http.StatusCreated {
		t.Fatalf("recreate label status = %d body=%s", recreateResp.Code, recreateResp.Body.String())
	}
	decodeResponse(t, recreateResp, &label)

	// Delete label through the global label route.
	delResp := doJSONRequest(t, handler, http.MethodDelete, "/api/labels/"+strconv.FormatInt(label.ID, 10), nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete label status = %d", delResp.Code)
	}
}

func TestTimeEntryAPI(t *testing.T) {
	t.Parallel()
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
	logResp := doJSONRequest(t, handler, http.MethodPost, "/api/tickets/"+ticket.ID+"/time", map[string]any{
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
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/time", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list time status = %d", listResp.Code)
	}
	var entries []store.TimeEntry
	decodeResponse(t, listResp, &entries)
	if len(entries) != 1 {
		t.Fatalf("expected 1 time entry, got %d", len(entries))
	}

	// Get total
	totalResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticket.ID+"/time/total", nil, token)
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
	t.Parallel()
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
	listResp := doJSONRequest(t, handler, http.MethodGet, "/api/tickets/"+ticketA.ID+"/dependencies", nil, token)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list dependencies status = %d", listResp.Code)
	}
	var deps []store.Dependency
	decodeResponse(t, listResp, &deps)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].DependsOn != ticketB.ID {
		t.Fatalf("depends_on = %s, want %s", deps[0].DependsOn, ticketB.ID)
	}

	// Remove dependency (DELETE uses query params)
	delPath := "/api/dependencies?project_id=1&ticket_id=" + ticketA.ID + "&depends_on=" + ticketB.ID
	delResp := doJSONRequest(t, handler, http.MethodDelete, delPath, nil, token)
	if delResp.Code != http.StatusOK {
		t.Fatalf("remove dependency status = %d, body=%s", delResp.Code, delResp.Body.String())
	}
}
