package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func TestRemoteClientSendsAuthHeaderAndParsesStatus(t *testing.T) {
	t.Parallel()
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
	status, err := api.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" || status.ServerVersion != "1.2.3" {
		t.Fatalf("Status() = %#v", status)
	}
}

func TestRemoteClientAutoAuthenticatesFromEnvTrio(t *testing.T) {
	t.Setenv("TICKET_URL", "http://example.test")
	t.Setenv("TICKET_USERNAME", "alice")
	t.Setenv("TICKET_PASSWORD", "secret12")

	loginCalls := 0
	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/login":
			loginCalls++
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(login) error = %v", err)
			}
			if payload["username"] != "alice" || payload["password"] != "secret12" {
				t.Fatalf("login payload = %#v", payload)
			}
			_, _ = w.Write([]byte(`{"token":"env-token","user":{"username":"alice","role":"admin"}}`))
		case "/api/status":
			statusCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer env-token" {
				t.Fatalf("Authorization = %q, want Bearer env-token", got)
			}
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if _, err := api.Status(context.Background()); err != nil {
		t.Fatalf("Status(first) error = %v", err)
	}
	if _, err := api.Status(context.Background()); err != nil {
		t.Fatalf("Status(second) error = %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("login calls = %d, want 1", loginCalls)
	}
	if statusCalls != 2 {
		t.Fatalf("status calls = %d, want 2", statusCalls)
	}
}

func TestRemoteClientEnvTrioReauthenticatesAfterUnauthorized(t *testing.T) {
	t.Setenv("TICKET_URL", "http://example.test")
	t.Setenv("TICKET_USERNAME", "alice")
	t.Setenv("TICKET_PASSWORD", "secret12")

	loginCalls := 0
	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/login":
			loginCalls++
			_, _ = w.Write([]byte(`{"token":"fresh-token","user":{"username":"alice","role":"admin"}}`))
		case "/api/status":
			statusCalls++
			if statusCalls == 1 {
				if got := r.Header.Get("Authorization"); got != "Bearer stale-token" {
					t.Fatalf("first Authorization = %q, want Bearer stale-token", got)
				}
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
				t.Fatalf("second Authorization = %q, want Bearer fresh-token", got)
			}
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":true}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL, Token: "stale-token"})
	if _, err := api.Status(context.Background()); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if loginCalls != 1 {
		t.Fatalf("login calls = %d, want 1", loginCalls)
	}
	if statusCalls != 2 {
		t.Fatalf("status calls = %d, want 2", statusCalls)
	}
}

func TestNewSetsHTTPTimeout(t *testing.T) {
	t.Parallel()
	api := New(config.Config{Location: "http://example.com"})
	if api.http == nil {
		t.Fatal("New().http = nil")
	}
	if api.http.Timeout != 30*time.Second {
		t.Fatalf("New().http.Timeout = %s, want %s", api.http.Timeout, 30*time.Second)
	}
}

func TestRemoteClientListTicketsFilteredBuildsQuery(t *testing.T) {
	t.Parallel()
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
	if _, err := api.ListTicketsFiltered(context.Background(), 7, "bug", "", "", "develop/idle", "needle", "alice", 25, false); err != nil {
		t.Fatalf("ListTicketsFiltered() error = %v", err)
	}
}

func TestRemoteClientListTicketsFilteredIncludesArchived(t *testing.T) {
	t.Parallel()
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
	if _, err := api.ListTicketsFiltered(context.Background(), 7, "", "", "", "", "", "", 0, true); err != nil {
		t.Fatalf("ListTicketsFiltered() error = %v", err)
	}
}

func TestRemoteClientListHistoryPagedBuildsQuery(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tickets/11/history" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		if values.Get("limit") != "7" || values.Get("offset") != "3" {
			t.Fatalf("query = %#v", values)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if _, err := api.ListHistoryPaged(context.Background(), "11", 7, 3); err != nil {
		t.Fatalf("ListHistoryPaged() error = %v", err)
	}
}

func TestRemoteClientRequestTicketPostsJSON(t *testing.T) {
	t.Parallel()
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
	resp, err := api.RequestTicket(context.Background(), TicketRequest{ProjectID: 3, TicketID: &taskID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if resp.Status != "REJECTED" {
		t.Fatalf("RequestTicket() = %#v", resp)
	}
}

func TestRemoteClientReturnsAPIErrorMessage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"denied"}`))
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if _, err := api.Count(context.Background(), nil); err == nil || err.Error() != "denied" {
		t.Fatalf("Count() error = %v, want denied", err)
	}
}

func TestRemoteClientReturnsStatusErrorForNonJSONFailures(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain failure", http.StatusBadGateway)
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if _, err := api.Count(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "502") {
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
	if _, err := api.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want decode error")
	}
}

func TestRemoteClientHandlesNetworkFailure(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	api := New(config.Config{Location: "http://" + addr})
	if _, err := api.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want network error")
	}
}

func TestLocalModeClientRejectsRemoteOnlyAuthCalls(t *testing.T) {
	t.Parallel()

	api := New(config.Config{})
	if _, err := api.Register(context.Background(), "alice", "secret"); err == nil {
		t.Fatal("Register() error = nil")
	}
	if _, err := api.Login(context.Background(), "alice", "secret"); err == nil {
		t.Fatal("Login() error = nil")
	}
	if err := api.Logout(context.Background()); err == nil {
		t.Fatal("Logout() error = nil")
	}
}

func TestRemoteClientCRUDRoutes(t *testing.T) {
	t.Parallel()
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

	if _, err := api.CreateUser(context.Background(), "alice", "secret"); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := api.ListUsers(context.Background()); err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if err := api.SetUserEnabled(context.Background(), "alice", true); err != nil {
		t.Fatalf("SetUserEnabled() error = %v", err)
	}
	if err := api.DeleteUser(context.Background(), "alice"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := api.CreateProject(context.Background(), ProjectCreateRequest{Title: "P", Prefix: "PPP"}); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := api.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if _, err := api.GetProject(context.Background(), strconv.FormatInt(projectID, 10)); err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if _, err := api.UpdateProject(context.Background(), projectID, ProjectUpdateRequest{Title: "P2"}); err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if _, err := api.SetProjectEnabled(context.Background(), projectID, false); err != nil {
		t.Fatalf("SetProjectEnabled() error = %v", err)
	}
	if _, err := api.CreateTicket(context.Background(), TicketCreateRequest{ProjectID: projectID, Type: "task", Title: "T"}); err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := api.GetTicketByID(context.Background(), taskID); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if err := api.DeleteTicket(context.Background(), taskID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := api.UpdateTicket(context.Background(), taskID, TicketUpdateRequest{Title: "T", Stage: "develop", State: "active"}); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if _, err := api.SetTicketParent(context.Background(), taskID, dependsOn, ""); err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if _, err := api.UnsetTicketParent(context.Background(), taskID, ""); err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if _, err := api.CloneTicket(context.Background(), taskID, ""); err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
	if _, err := api.ListHistory(context.Background(), taskID); err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if _, err := api.AddComment(context.Background(), taskID, "hello"); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if _, err := api.ListComments(context.Background(), taskID); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if _, err := api.AddDependency(context.Background(), DependencyRequest{ProjectID: projectID, TicketID: taskID, DependsOn: dependsOn}); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	if err := api.RemoveDependency(context.Background(), DependencyRequest{ProjectID: projectID, TicketID: taskID, DependsOn: dependsOn}); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}
	if _, err := api.ListDependencies(context.Background(), taskID); err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
}

func TestRemoteClientRolesCRUD(t *testing.T) {
	t.Parallel()
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

	if _, err := api.CreateRole(context.Background(), RoleRequest{Title: "dev", Description: "build", AcceptanceCriteria: "ship"}); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if _, err := api.ListRoles(context.Background()); err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if _, err := api.UpdateRole(context.Background(), 1, RoleRequest{Title: "dev2", Description: "build", AcceptanceCriteria: "ship"}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if err := api.DeleteRole(context.Background(), 1); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
}

func TestRemoteClientAgentsCRUD(t *testing.T) {
	t.Parallel()
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

	if _, _, err := api.CreateAgent(context.Background(), AgentCreateRequest{Password: "secret"}); err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if _, err := api.SetAgentEnabled(context.Background(), "a1", true); err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}
	if _, err := api.SetAgentEnabled(context.Background(), "a1", false); err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if _, err := api.ListAgents(context.Background()); err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if _, err := api.ListAgentStatuses(context.Background()); err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	pw := "newpw"
	if _, err := api.UpdateAgent(context.Background(), "a1", AgentUpdateRequest{Password: &pw}); err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}
	if err := api.DeleteAgent(context.Background(), "a1"); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	if err := api.SetAgentConfig(context.Background(), "a1", "k", "v"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	if _, err := api.ListAgentConfig(context.Background(), "a1"); err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if err := api.DeleteAgentConfig(context.Background(), "a1", "k"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	if _, err := api.RegisterAgent(context.Background(), AgentRegisterRequest{ID: "a1", Password: "secret"}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if err := api.HeartbeatAgent(context.Background(), "a1", "secret", "online"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}
	if _, err := api.RequestAgentWork(context.Background(), AgentRequest{ID: "a1", Password: "secret", ProjectID: 7}); err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	if _, err := api.AgentUpdateTicket(context.Background(), "11", AgentTicketUpdateRequest{ID: "a1", Password: "secret", Result: "done"}); err != nil {
		t.Fatalf("AgentUpdateTicket() error = %v", err)
	}
}

func TestRemoteClientProjectMembersAndTeams(t *testing.T) {
	t.Parallel()
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

	if err := api.DeleteProject(context.Background(), 7); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := api.AddProjectMember(context.Background(), 7, ProjectMemberRequest{UserID: "u1", Role: "member"}); err != nil {
		t.Fatalf("AddProjectMember() error = %v", err)
	}
	if err := api.RemoveProjectMember(context.Background(), 7, "u1"); err != nil {
		t.Fatalf("RemoveProjectMember() error = %v", err)
	}
	if _, err := api.ListProjectMembers(context.Background(), 7); err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	if _, err := api.AddProjectTeamMember(context.Background(), 7, ProjectTeamMemberRequest{TeamID: 1, Role: "member"}); err != nil {
		t.Fatalf("AddProjectTeamMember() error = %v", err)
	}
	if err := api.RemoveProjectTeamMember(context.Background(), 7, 1); err != nil {
		t.Fatalf("RemoveProjectTeamMember() error = %v", err)
	}
	if _, err := api.ListProjectTeamMembers(context.Background(), 7); err != nil {
		t.Fatalf("ListProjectTeamMembers() error = %v", err)
	}
	if _, err := api.CreateTeam(context.Background(), TeamRequest{Name: "alpha"}); err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if _, err := api.ListTeams(context.Background()); err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if _, err := api.UpdateTeam(context.Background(), 1, TeamRequest{Name: "beta"}); err != nil {
		t.Fatalf("UpdateTeam() error = %v", err)
	}
	if err := api.DeleteTeam(context.Background(), 1); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}
	if _, err := api.AddTeamMember(context.Background(), 1, TeamMemberRequest{UserID: "u1", Role: "member"}); err != nil {
		t.Fatalf("AddTeamMember() error = %v", err)
	}
	if err := api.RemoveTeamMember(context.Background(), 1, "u1"); err != nil {
		t.Fatalf("RemoveTeamMember() error = %v", err)
	}
	if _, err := api.ListTeamMembers(context.Background(), 1); err != nil {
		t.Fatalf("ListTeamMembers() error = %v", err)
	}
	if _, err := api.AddTeamAgent(context.Background(), 1, "a1"); err != nil {
		t.Fatalf("AddTeamAgent() error = %v", err)
	}
	if err := api.RemoveTeamAgent(context.Background(), 1, "a1"); err != nil {
		t.Fatalf("RemoveTeamAgent() error = %v", err)
	}
	if _, err := api.ListTeamAgents(context.Background(), 1); err != nil {
		t.Fatalf("ListTeamAgents() error = %v", err)
	}
}

func TestRemoteClientTicketLifecycle(t *testing.T) {
	t.Parallel()
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
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/sdlc":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/tickets/11/sdlc":
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

	if _, err := api.ListTickets(context.Background(), 7); err != nil {
		t.Fatalf("ListTickets() error = %v", err)
	}
	if _, err := api.CloseTicket(context.Background(), "11", ""); err != nil {
		t.Fatalf("CloseTicket() error = %v", err)
	}
	if _, err := api.OpenTicket(context.Background(), "11", ""); err != nil {
		t.Fatalf("OpenTicket() error = %v", err)
	}
	if _, err := api.ArchiveTicket(context.Background(), "11", ""); err != nil {
		t.Fatalf("ArchiveTicket() error = %v", err)
	}
	if _, err := api.UnarchiveTicket(context.Background(), "11", ""); err != nil {
		t.Fatalf("UnarchiveTicket() error = %v", err)
	}
	if _, err := api.NotReadyTicket(context.Background(), "11", ""); err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if _, err := api.SetTicketSdlc(context.Background(), "11", 1); err != nil {
		t.Fatalf("SetTicketSdlc() error = %v", err)
	}
	if _, err := api.UnsetTicketSdlc(context.Background(), "11"); err != nil {
		t.Fatalf("UnsetTicketSdlc() error = %v", err)
	}
	if _, err := api.GetTicket(context.Background(), "11"); err != nil {
		t.Fatalf("GetTicket() error = %v", err)
	}
	if _, err := api.SetTicketHealth(context.Background(), "11", 80); err != nil {
		t.Fatalf("SetTicketHealth() error = %v", err)
	}
	if _, err := api.ListProjectHistory(context.Background(), 7, 5); err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
}

func TestRemoteClientSdlcsCRUD(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/sdlcs":
			_, _ = w.Write([]byte(`{"id":1,"name":"wf1","description":"d"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/sdlcs":
			_, _ = w.Write([]byte(`[{"id":1,"name":"wf1","description":"d"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/sdlcs/1":
			_, _ = w.Write([]byte(`{"sdlc":{"id":1,"name":"wf1"},"stages":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/sdlcs/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/sdlcs/1/stages":
			_, _ = w.Write([]byte(`{"id":1,"sdlc_id":1,"stage_name":"design","sort_order":0}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/sdlcs/stages/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/sdlcs/1/reorder":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/sdlcs/1/export":
			_, _ = w.Write([]byte(`{"name":"wf1","description":"d","stages":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/sdlcs/import":
			_, _ = w.Write([]byte(`{"id":2,"name":"wf1","description":"d"}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})

	if _, err := api.CreateSdlc(context.Background(), SdlcRequest{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("CreateSdlc() error = %v", err)
	}
	if _, err := api.ListSdlcs(context.Background()); err != nil {
		t.Fatalf("ListSdlcs() error = %v", err)
	}
	if _, err := api.GetSdlc(context.Background(), 1); err != nil {
		t.Fatalf("GetSdlc() error = %v", err)
	}
	if err := api.DeleteSdlc(context.Background(), 1); err != nil {
		t.Fatalf("DeleteSdlc() error = %v", err)
	}
	if _, err := api.AddSdlcStage(context.Background(), 1, SdlcStageRequest{StageName: "design", SortOrder: 0}); err != nil {
		t.Fatalf("AddSdlcStage() error = %v", err)
	}
	if err := api.RemoveSdlcStage(context.Background(), 1); err != nil {
		t.Fatalf("RemoveSdlcStage() error = %v", err)
	}
	if err := api.ReorderSdlcStages(context.Background(), 1, []int64{1, 2}); err != nil {
		t.Fatalf("ReorderSdlcStages() error = %v", err)
	}
	if _, err := api.ExportSdlc(context.Background(), 1); err != nil {
		t.Fatalf("ExportSdlc() error = %v", err)
	}
	if _, err := api.ImportSdlc(context.Background(), store.SdlcExport{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("ImportSdlc() error = %v", err)
	}
}

func TestRemoteClientTimeTrackingAndLabels(t *testing.T) {
	t.Parallel()
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

	if _, err := api.LogTime(context.Background(), "11", TimeEntryRequest{Minutes: 30, Note: "work"}); err != nil {
		t.Fatalf("LogTime() error = %v", err)
	}
	if _, err := api.ListTimeEntries(context.Background(), "11"); err != nil {
		t.Fatalf("ListTimeEntries() error = %v", err)
	}
	if err := api.DeleteTimeEntry(context.Background(), 1); err != nil {
		t.Fatalf("DeleteTimeEntry() error = %v", err)
	}
	if total, err := api.TotalTimeForTicket(context.Background(), "11"); err != nil {
		t.Fatalf("TotalTimeForTicket() error = %v", err)
	} else if total != 30 {
		t.Fatalf("TotalTimeForTicket() = %d, want 30", total)
	}
	if _, err := api.CreateLabel(context.Background(), 7, LabelRequest{Name: "bug", Color: "red"}); err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if _, err := api.ListLabels(context.Background(), 7); err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if err := api.DeleteLabel(context.Background(), 1); err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}
	if err := api.AddTicketLabel(context.Background(), "11", 1); err != nil {
		t.Fatalf("AddTicketLabel() error = %v", err)
	}
	if err := api.RemoveTicketLabel(context.Background(), "11", 1); err != nil {
		t.Fatalf("RemoveTicketLabel() error = %v", err)
	}
	if _, err := api.ListTicketLabels(context.Background(), "11"); err != nil {
		t.Fatalf("ListTicketLabels() error = %v", err)
	}
}

func TestRemoteClientStoriesCRUD(t *testing.T) {
	t.Parallel()
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

	if _, err := api.CreateStory(context.Background(), 7, "S", "desc"); err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if _, err := api.ListStories(context.Background(), 7); err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if _, err := api.GetStory(context.Background(), 1); err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if _, err := api.UpdateStory(context.Background(), 1, "S2", "desc2"); err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if err := api.DeleteStory(context.Background(), 1); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
}

func TestRemoteClientRegistrationAndPasswordReset(t *testing.T) {
	t.Parallel()
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

	if err := api.SetRegistrationEnabled(context.Background(), true); err != nil {
		t.Fatalf("SetRegistrationEnabled() error = %v", err)
	}
	if _, err := api.ResetUserPassword(context.Background(), "alice", "newpw"); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if _, err := api.Register(context.Background(), "alice", "secret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp, err := api.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatalf("Login() error = %v", err)
	} else if resp.Token != "tok" {
		t.Fatalf("Login() token = %q, want tok", resp.Token)
	}
}
