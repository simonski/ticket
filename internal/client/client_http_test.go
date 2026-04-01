package client

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func TestRemoteClientSendsAuthHeaderAndParsesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("path = %q, want /api/status", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token-123" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"authenticated":  true,
			"server_version": "1.2.3",
			"user": map[string]any{
				"username": "alice",
				"role":     "user",
			},
		})
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL, Token: "token-123"})
	status, err := api.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" || status.ServerVersion != "1.2.3" {
		t.Fatalf("Status() = %#v", status)
	}
}

func TestRemoteClientListTicketsFilteredBuildsQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/7/tickets" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		if values.Get("type") != "bug" || values.Get("status") != "develop/idle" || values.Get("q") != "needle" || values.Get("assignee") != "alice" || values.Get("limit") != "25" {
			t.Fatalf("query = %#v", values)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})
	if _, err := api.ListTicketsFiltered(7, "bug", "", "", "develop/idle", "needle", "alice", 25, false); err != nil {
		t.Fatalf("ListTicketsFiltered() error = %v", err)
	}
}

func TestRemoteClientListTicketsFilteredIncludesArchived(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/7/tickets" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		if values.Get("include_archived") != "1" {
			t.Fatalf("query = %#v", values)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})
	if _, err := api.ListTicketsFiltered(7, "", "", "", "", "", "", 0, true); err != nil {
		t.Fatalf("ListTicketsFiltered() error = %v", err)
	}
}

func TestRemoteClientRequestTicketPostsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tickets/claim" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token-123" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var payload TicketRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.ProjectID != 3 || payload.TicketID == nil || *payload.TicketID != "9" {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"REJECTED"}`))
	}))
	defer server.Close()
	

	taskID := "9"
	api := New(config.Config{Location: server.URL, Token: "token-123"})
	resp, err := api.RequestTicket(TicketRequest{ProjectID: 3, TicketID: &taskID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if resp.Status != "REJECTED" {
		t.Fatalf("RequestTicket() = %#v", resp)
	}
}

func TestRemoteClientReturnsAPIErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"denied"}`))
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})
	if _, err := api.Count(nil); err == nil || err.Error() != "denied" {
		t.Fatalf("Count() error = %v, want denied", err)
	}
}

func TestRemoteClientReturnsStatusErrorForNonJSONFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain failure", http.StatusBadGateway)
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})
	if _, err := api.Count(nil); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("Count() error = %v, want status error containing 502", err)
	}
}

func TestRemoteClientReturnsDecodeErrorOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":`))
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want decode error")
	}
}

func TestRemoteClientHandlesNetworkFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	api := New(config.Config{Location: "http://" + addr})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want network error")
	}
}

func TestLocalModeClientRejectsRemoteOnlyAuthCalls(t *testing.T) {

	api := New(config.Config{})
	if _, err := api.Register("alice", "secret"); err == nil {
		t.Fatal("Register() error = nil")
	}
	if _, err := api.Login("alice", "secret"); err == nil {
		t.Fatal("Login() error = nil")
	}
	if err := api.Logout(); err == nil {
		t.Fatal("Logout() error = nil")
	}
}

func TestRemoteClientCRUDRoutes(t *testing.T) {
	projectID := int64(7)
	taskID := "11"
	dependsOn := "12"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/users":
			_, _ = w.Write([]byte(`{"user_id":"test-uuid-2","username":"alice","role":"user","enabled":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/users":
			_, _ = w.Write([]byte(`[{"user_id":"test-uuid-2","username":"alice","role":"user","enabled":true}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/users/alice/enable":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/users/alice":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects":
			_, _ = w.Write([]byte(`{"project_id":7,"title":"P","status":"active"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_, _ = w.Write([]byte(`[{"project_id":7,"title":"P","status":"active"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7":
			_, _ = w.Write([]byte(`{"project_id":7,"title":"P","status":"active"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/projects/7":
			_, _ = w.Write([]byte(`{"project_id":7,"title":"P2","status":"active"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/7/disable":
			_, _ = w.Write([]byte(`{"project_id":7,"title":"P2","status":"disabled"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets":
			_, _ = w.Write([]byte(`{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11":
			_, _ = w.Write([]byte(`{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/tickets/11":
			_, _ = w.Write([]byte(`{"status":"deleted"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/tickets/11":
			var payload TicketUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(task update) error = %v", err)
			}
			parentJSON := ""
			if payload.ParentID != nil {
				parentJSON = `,"parent_id":"` + *payload.ParentID + `"`
			}
			_, _ = w.Write([]byte(`{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"develop","state":"active","status":"develop/active"` + parentJSON + `}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/clone":
			_, _ = w.Write([]byte(`{"ticket_id":"21","project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle","clone_of":"11"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/history":
			_, _ = w.Write([]byte(`[{"id":1,"ticket_id":"11","event_type":"ticket_updated"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/comments":
			_, _ = w.Write([]byte(`{"id":1,"item_id":11,"comment":"hello"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/comments":
			_, _ = w.Write([]byte(`[{"id":1,"item_id":11,"comment":"hello"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/dependencies":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"ticket_id":"11","depends_on":"12"}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/dependencies"):
			if r.URL.Query().Get("project_id") != "7" || r.URL.Query().Get("ticket_id") != "11" || r.URL.Query().Get("depends_on") != "12" {
				t.Fatalf("dependency delete query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/dependencies":
			_, _ = w.Write([]byte(`[{"id":1,"project_id":7,"ticket_id":"11","depends_on":"12"}]`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.CreateUser("alice", "secret"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := api.ListUsers(); err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if err := api.SetUserEnabled("alice", true); err != nil {
		t.Fatalf("SetUserEnabled() error = %v", err)
	}
	if err := api.DeleteUser("alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := api.CreateProject(ProjectCreateRequest{Title: "P", Prefix: "PPP"}); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := api.ListProjects(); err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if _, err := api.GetProject(strconv.FormatInt(projectID, 10)); err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if _, err := api.UpdateProject(projectID, ProjectUpdateRequest{Title: "P2"}); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if _, err := api.SetProjectEnabled(projectID, false); err != nil {
		t.Fatalf("SetProjectEnabled() error = %v", err)
	}
	if _, err := api.CreateTicket(TicketCreateRequest{ProjectID: projectID, Type: "task", Title: "T"}); err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.GetTicketByID(taskID); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if err := api.DeleteTicket(taskID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := api.UpdateTicket(taskID, TicketUpdateRequest{Title: "T", Stage: "develop", State: "active"}); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if _, err := api.SetTicketParent(taskID, dependsOn, ""); err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if _, err := api.UnsetTicketParent(taskID, ""); err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if _, err := api.CloneTicket(taskID, ""); err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if _, err := api.ListHistory(taskID); err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if _, err := api.AddComment(taskID, "hello"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if _, err := api.ListComments(taskID); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if _, err := api.AddDependency(DependencyRequest{ProjectID: projectID, TicketID: taskID, DependsOn: dependsOn}); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	if err := api.RemoveDependency(DependencyRequest{ProjectID: projectID, TicketID: taskID, DependsOn: dependsOn}); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}
	if _, err := api.ListDependencies(taskID); err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
}

func TestRemoteClientRolesCRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/roles":
			_, _ = w.Write([]byte(`{"id":1,"title":"dev","motivation":"build","goals":"ship"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/roles":
			_, _ = w.Write([]byte(`[{"id":1,"title":"dev","motivation":"build","goals":"ship"}]`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/roles/1":
			_, _ = w.Write([]byte(`{"id":1,"title":"dev2","motivation":"build","goals":"ship"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/roles/1":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.CreateRole(RoleRequest{Title: "dev", Motivation: "build", Goals: "ship"}); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if _, err := api.ListRoles(); err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if _, err := api.UpdateRole(1, RoleRequest{Title: "dev2", Motivation: "build", Goals: "ship"}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if err := api.DeleteRole(1); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
}

func TestRemoteClientAgentsCRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents":
			_, _ = w.Write([]byte(`{"agent":{"id":"a1","username":"agent-a1","enabled":true},"password":"secret"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/a1/enable":
			_, _ = w.Write([]byte(`{"id":"a1","username":"agent-a1","enabled":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/a1/disable":
			_, _ = w.Write([]byte(`{"id":"a1","username":"agent-a1","enabled":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents":
			_, _ = w.Write([]byte(`[{"id":"a1","username":"agent-a1","enabled":true}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents/statuses":
			_, _ = w.Write([]byte(`[{"agent_id":"a1","status":"online"}]`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/agents/a1":
			_, _ = w.Write([]byte(`{"id":"a1","username":"agent-a1","enabled":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/agents/a1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/a1/config":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents/a1/config":
			_, _ = w.Write([]byte(`[{"agent_id":"a1","key":"k","value":"v"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/agents/a1/config/k":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/register":
			_, _ = w.Write([]byte(`{"agent":{"id":"a1","username":"agent-a1","enabled":true}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/heartbeat":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/request":
			_, _ = w.Write([]byte(`{"status":"NONE"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/tickets/11/update":
			_, _ = w.Write([]byte(`{"ticket_id":"11","project_id":7,"title":"T","type":"task"}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, _, err := api.CreateAgent(AgentCreateRequest{Password: "secret"}); err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if _, err := api.SetAgentEnabled("a1", true); err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}
	if _, err := api.SetAgentEnabled("a1", false); err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if _, err := api.ListAgents(); err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if _, err := api.ListAgentStatuses(); err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	pw := "newpw"
	if _, err := api.UpdateAgent("a1", AgentUpdateRequest{Password: &pw}); err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}
	if err := api.DeleteAgent("a1"); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	if err := api.SetAgentConfig("a1", "k", "v"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	if _, err := api.ListAgentConfig("a1"); err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if err := api.DeleteAgentConfig("a1", "k"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	if _, err := api.RegisterAgent(AgentRegisterRequest{ID: "a1", Password: "secret"}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if err := api.HeartbeatAgent("a1", "secret", "online"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}
	if _, err := api.RequestAgentWork(AgentRequest{ID: "a1", Password: "secret", ProjectID: 7}); err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	if _, err := api.AgentUpdateTicket("11", AgentTicketUpdateRequest{ID: "a1", Password: "secret", Result: "done"}); err != nil {
		t.Fatalf("AgentUpdateTicket() error = %v", err)
	}
}

func TestRemoteClientProjectMembersAndTeams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/projects/7":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/7/users":
			_, _ = w.Write([]byte(`{"project_id":7,"user_id":"u1","role":"member"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/projects/7/users/u1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/users":
			_, _ = w.Write([]byte(`[{"project_id":7,"user_id":"u1","role":"member"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/7/teams":
			_, _ = w.Write([]byte(`{"project_id":7,"team_id":1,"role":"member"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/projects/7/teams/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/teams":
			_, _ = w.Write([]byte(`[{"project_id":7,"team_id":1,"role":"member"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/teams":
			_, _ = w.Write([]byte(`{"id":1,"name":"alpha"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/teams":
			_, _ = w.Write([]byte(`[{"id":1,"name":"alpha"}]`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/teams/1":
			_, _ = w.Write([]byte(`{"id":1,"name":"beta"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/teams/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/teams/1/users":
			_, _ = w.Write([]byte(`{"team_id":1,"user_id":"u1","role":"member"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/teams/1/users/u1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/teams/1/users":
			_, _ = w.Write([]byte(`[{"team_id":1,"user_id":"u1","role":"member"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/teams/1/agents":
			_, _ = w.Write([]byte(`{"team_id":1,"agent_id":"a1"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/teams/1/agents/a1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/teams/1/agents":
			_, _ = w.Write([]byte(`[{"team_id":1,"agent_id":"a1"}]`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if err := api.DeleteProject(7); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := api.AddProjectMember(7, ProjectMemberRequest{UserID: "u1", Role: "member"}); err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	if err := api.RemoveProjectMember(7, "u1"); err != nil {
		t.Fatalf("RemoveProjectMember() error = %v", err)
	}
	if _, err := api.ListProjectMembers(7); err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	if _, err := api.AddProjectTeamMember(7, ProjectTeamMemberRequest{TeamID: 1, Role: "member"}); err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}
	if err := api.RemoveProjectTeamMember(7, 1); err != nil {
		t.Fatalf("RemoveProjectTeamMember() error = %v", err)
	}
	if _, err := api.ListProjectTeamMembers(7); err != nil {
		t.Fatalf("ListProjectTeamMembers() error = %v", err)
	}
	if _, err := api.CreateTeam(TeamRequest{Name: "alpha"}); err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if _, err := api.ListTeams(); err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if _, err := api.UpdateTeam(1, TeamRequest{Name: "beta"}); err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if err := api.DeleteTeam(1); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}
	if _, err := api.AddTeamMember(1, TeamMemberRequest{UserID: "u1", Role: "member"}); err != nil {
		t.Fatalf("AddTeamMember() error = %v", err)
	}
	if err := api.RemoveTeamMember(1, "u1"); err != nil {
		t.Fatalf("RemoveTeamMember() error = %v", err)
	}
	if _, err := api.ListTeamMembers(1); err != nil {
		t.Fatalf("ListTeamMembers() error = %v", err)
	}
	if _, err := api.AddTeamAgent(1, "a1"); err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	if err := api.RemoveTeamAgent(1, "a1"); err != nil {
		t.Fatalf("RemoveTeamAgent() error = %v", err)
	}
	if _, err := api.ListTeamAgents(1); err != nil {
		t.Fatalf("ListTeamAgents() error = %v", err)
	}
}

func TestRemoteClientTicketLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ticketJSON := `{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle"}`
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/tickets":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/close":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/open":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/archive":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/unarchive":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/notready":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/workflow":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/tickets/11/workflow":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/health":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/history":
			_, _ = w.Write([]byte(`[{"id":1,"ticket_id":"11","event_type":"ticket_updated"}]`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.ListTickets(7); err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if _, err := api.CloseTicket("11", ""); err != nil {
		t.Fatalf("CloseTicket() error = %v", err)
	}
	if _, err := api.OpenTicket("11", ""); err != nil {
		t.Fatalf("OpenTicket() error = %v", err)
	}
	if _, err := api.ArchiveTicket("11", ""); err != nil {
		t.Fatalf("ArchiveTicket() error = %v", err)
	}
	if _, err := api.UnarchiveTicket("11", ""); err != nil {
		t.Fatalf("UnarchiveTicket() error = %v", err)
	}
	if _, err := api.NotReadyTicket("11", ""); err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if _, err := api.SetTicketWorkflow("11", 1); err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if _, err := api.UnsetTicketWorkflow("11"); err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}
	if _, err := api.GetTicket("11"); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if _, err := api.SetTicketHealth("11", 80); err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
	if _, err := api.ListProjectHistory(7, 5); err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
}

func TestRemoteClientWorkflowsCRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/workflows":
			_, _ = w.Write([]byte(`{"id":1,"name":"wf1","description":"d"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/workflows":
			_, _ = w.Write([]byte(`[{"id":1,"name":"wf1","description":"d"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/workflows/1":
			_, _ = w.Write([]byte(`{"workflow":{"id":1,"name":"wf1"},"stages":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/workflows/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/1/stages":
			_, _ = w.Write([]byte(`{"id":1,"workflow_id":1,"stage_name":"design","sort_order":0}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/workflows/stages/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/workflows/1/reorder":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/workflows/1/export":
			_, _ = w.Write([]byte(`{"name":"wf1","description":"d","stages":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/import":
			_, _ = w.Write([]byte(`{"id":2,"name":"wf1","description":"d"}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.CreateWorkflow(WorkflowRequest{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := api.ListWorkflows(); err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if _, err := api.GetWorkflow(1); err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if err := api.DeleteWorkflow(1); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
	if _, err := api.AddWorkflowStage(1, WorkflowStageRequest{StageName: "design", SortOrder: 0}); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if err := api.RemoveWorkflowStage(1); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	if err := api.ReorderWorkflowStages(1, []int64{1, 2}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	if _, err := api.ExportWorkflow(1); err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	if _, err := api.ImportWorkflow(store.WorkflowExport{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}
}

func TestRemoteClientTimeTrackingAndLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/time":
			_, _ = w.Write([]byte(`{"id":1,"ticket_id":"11","user_id":"u1","minutes":30,"note":"work"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/time":
			_, _ = w.Write([]byte(`[{"id":1,"ticket_id":"11","user_id":"u1","minutes":30,"note":"work"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/time/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/time/total":
			_, _ = w.Write([]byte(`{"total":30}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/7/labels":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"name":"bug","color":"red"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/labels":
			_, _ = w.Write([]byte(`[{"id":1,"project_id":7,"name":"bug","color":"red"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/labels/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/labels":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/tickets/11/labels/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/labels":
			_, _ = w.Write([]byte(`[{"id":1,"project_id":7,"name":"bug","color":"red"}]`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.LogTime("11", libticket.TimeEntryRequest{Minutes: 30, Note: "work"}); err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	if _, err := api.ListTimeEntries("11"); err != nil {
		t.Fatalf("ListTimeEntries() error = %v", err)
	}
	if err := api.DeleteTimeEntry(1); err != nil {
		t.Fatalf("DeleteTimeEntry() error = %v", err)
	}
	if total, err := api.TotalTimeForTicket("11"); err != nil {
		t.Fatalf("TotalTimeForTicket() error = %v", err)
	} else if total != 30 {
		t.Fatalf("TotalTimeForTicket() = %d, want 30", total)
	}
	if _, err := api.CreateLabel(7, libticket.LabelRequest{Name: "bug", Color: "red"}); err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if _, err := api.ListLabels(7); err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if err := api.DeleteLabel(1); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
	if err := api.AddTicketLabel("11", 1); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	if err := api.RemoveTicketLabel("11", 1); err != nil {
		t.Fatalf("RemoveTicketLabel() error = %v", err)
	}
	if _, err := api.ListTicketLabels("11"); err != nil {
		t.Fatalf("ListTicketLabels() error = %v", err)
	}
}

func TestRemoteClientStoriesCRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/stories":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"title":"S","description":"desc"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/7/stories":
			_, _ = w.Write([]byte(`[{"id":1,"project_id":7,"title":"S","description":"desc"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/stories/1":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"title":"S","description":"desc"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/stories/1":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"title":"S2","description":"desc2"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/stories/1":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if _, err := api.CreateStory(7, "S", "desc"); err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if _, err := api.ListStories(7); err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if _, err := api.GetStory(1); err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if _, err := api.UpdateStory(1, "S2", "desc2"); err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if err := api.DeleteStory(1); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
}

func TestRemoteClientRegistrationAndPasswordReset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/config/registration":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/users/alice/reset-password":
			_, _ = w.Write([]byte(`{"user_id":"u1","username":"alice","role":"user","enabled":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/register":
			_, _ = w.Write([]byte(`{"user_id":"u1","username":"alice","role":"user","enabled":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/login":
			_, _ = w.Write([]byte(`{"token":"tok","user":{"user_id":"u1","username":"alice","role":"user","enabled":true}}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	

	api := New(config.Config{Location: server.URL})

	if err := api.SetRegistrationEnabled(true); err != nil {
		t.Fatalf("SetRegistrationEnabled() error = %v", err)
	}
	if _, err := api.ResetUserPassword("alice", "newpw"); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if _, err := api.Register("alice", "secret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp, err := api.Login("alice", "secret"); err != nil {
		t.Fatalf("Login() error = %v", err)
	} else if resp.Token != "tok" {
		t.Fatalf("Login() token = %q, want tok", resp.Token)
	}
}
