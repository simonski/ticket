package client

import (
	"context"
	"encoding/json"
	"io"
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
		username, password, ok := r.BasicAuth()
		if !ok || username != "alice" || password != "token-123" {
			t.Fatalf("BasicAuth = (%q, %q, %v), want alice/token-123/true", username, password, ok)
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

	api := New(config.Config{Location: server.URL, Username: "alice", Token: "token-123"})
	status, err := api.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" || status.ServerVersion != "1.2.3" {
		t.Fatalf("Status() = %#v", status)
	}
}

func TestRemoteClientDoesNotAutoAuthenticateWithoutToken(t *testing.T) {
	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			statusCalls++
			if _, _, ok := r.BasicAuth(); ok {
				t.Fatal("BasicAuth unexpectedly set")
			}
			_, _ = w.Write([]byte(`{"status":"ok","authenticated":false}`))
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
	if statusCalls != 2 {
		t.Fatalf("status calls = %d, want 2", statusCalls)
	}
}

func TestRemoteClientReturnsUnauthorizedWithoutReauth(t *testing.T) {
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/login":
			loginCalls++
		case "/api/status":
			username, password, ok := r.BasicAuth()
			if !ok || username != "alice" || password != "stale-token" {
				t.Fatalf("BasicAuth = (%q, %q, %v), want alice/stale-token/true", username, password, ok)
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL, Username: "alice", Token: "stale-token"})
	if _, err := api.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want unauthorized")
	}
	if loginCalls != 0 {
		t.Fatalf("login calls = %d, want 0", loginCalls)
	}
}

func TestNewSetsHTTPTimeout(t *testing.T) {
	t.Parallel()
	api := New(config.Config{Location: "http://example.com"})
	if api.http == nil {
		t.Fatal("New().http = nil")
	}
	if api.http.Timeout != 5*time.Second {
		t.Fatalf("New().http.Timeout = %s, want %s", api.http.Timeout, 5*time.Second)
	}
}

func TestNewSetsHTTPTimeoutFromEnvClamped(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "unset defaults to five", env: "", want: 5 * time.Second},
		{name: "within range", env: "12", want: 12 * time.Second},
		{name: "below one", env: "0", want: 1 * time.Second},
		{name: "negative", env: "-5", want: 1 * time.Second},
		{name: "above thirty", env: "99", want: 30 * time.Second},
		{name: "invalid", env: "abc", want: 5 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "" {
				t.Setenv("TICKET_TIMEOUT", "")
			} else {
				t.Setenv("TICKET_TIMEOUT", tc.env)
			}
			api := New(config.Config{Location: "http://example.com"})
			if api.http.Timeout != tc.want {
				t.Fatalf("New().http.Timeout = %s, want %s (env=%q)", api.http.Timeout, tc.want, tc.env)
			}
		})
	}
}

func TestNewLocalModeKeepsDefaultTimeout(t *testing.T) {
	t.Parallel()
	api := New(config.Config{})
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

func TestRemoteClientListProjectHistoryFilteredBuildsQuery(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/7/history" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		if values.Get("limit") != "10" || values.Get("user_id") != "u1" || values.Get("agent_id") != "a1" || values.Get("team_id") != "9" {
			t.Fatalf("query = %#v", values)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if _, err := api.ListProjectHistoryFiltered(context.Background(), 7, 0, store.HistoryFilter{
		UserID:  "u1",
		AgentID: "a1",
		TeamID:  9,
	}); err != nil {
		t.Fatalf("ListProjectHistoryFiltered() error = %v", err)
	}
}

func TestRemoteClientRequestTicketPostsJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/tickets/claim" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "alice" || password != "token-123" {
			t.Fatalf("BasicAuth = (%q, %q, %v), want alice/token-123/true", username, password, ok)
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
	api := New(config.Config{Location: server.URL, Username: "alice", Token: "token-123"})
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
	if _, err := api.SetTicketWorkflow(context.Background(), "11", 1); err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if _, err := api.UnsetTicketWorkflow(context.Background(), "11"); err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
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

func TestRemoteClientWorkflowsCRUD(t *testing.T) {
	t.Parallel()
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

	if _, err := api.CreateWorkflow(context.Background(), WorkflowRequest{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	if _, err := api.ListWorkflows(context.Background()); err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if _, err := api.GetWorkflow(context.Background(), 1); err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if err := api.DeleteWorkflow(context.Background(), 1); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}
	if _, err := api.AddWorkflowStage(context.Background(), 1, WorkflowStageRequest{StageName: "design", SortOrder: 0}); err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	if err := api.RemoveWorkflowStage(context.Background(), 1); err != nil {
		t.Fatalf("RemoveWorkflowStage() error = %v", err)
	}
	if err := api.ReorderWorkflowStages(context.Background(), 1, []int64{1, 2}); err != nil {
		t.Fatalf("ReorderWorkflowStages() error = %v", err)
	}
	if _, err := api.ExportWorkflow(context.Background(), 1); err != nil {
		t.Fatalf("ExportWorkflow() error = %v", err)
	}
	if _, err := api.ImportWorkflow(context.Background(), store.WorkflowExport{Name: "wf1", Description: "d"}); err != nil {
		t.Fatalf("ImportWorkflow() error = %v", err)
	}
}

func TestRemoteClientWorkflowStageRolesAndTicketAliases(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ticketJSON := `{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"develop","state":"active","status":"develop/active"}`
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/projects/7/set-draft":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/workflows/stages/1":
			_, _ = w.Write([]byte(`{"id":1,"workflow_id":9,"stage_name":"develop","ways_of_working":"wow","definition_of_ready":"dor","definition_of_done":"dod","sort_order":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/workflows/stages/1":
			_, _ = w.Write([]byte(`{"id":1,"workflow_id":9,"stage_name":"develop","sort_order":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/workflows/9":
			_, _ = w.Write([]byte(`{"workflow":{"id":9,"name":"wf"},"stages":[{"id":1,"workflow_id":9,"stage_name":"develop","sort_order":1}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/stages/roles/9/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/workflows/stages/roles/9/1/5":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/workflows/stages/roles/9/1":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/ready":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/close":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/open":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/notready":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/next":
			_, _ = w.Write([]byte(ticketJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/previous":
			_, _ = w.Write([]byte(ticketJSON))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if err := api.SetProjectDefaultDraft(context.Background(), 7, true); err != nil {
		t.Fatalf("SetProjectDefaultDraft() error = %v", err)
	}
	if _, err := api.UpdateWorkflowStage(context.Background(), 1, WorkflowStageRequest{StageName: "develop", WaysOfWorking: "wow", DefinitionOfReady: "dor", DefinitionOfDone: "dod"}); err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	if _, err := api.GetWorkflowStage(context.Background(), 1); err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if _, err := api.ListWorkflowStages(context.Background(), 9); err != nil {
		t.Fatalf("ListWorkflowStages() error = %v", err)
	}
	if err := api.AddWorkflowStageRole(context.Background(), 9, 1, 5); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}
	if err := api.RemoveWorkflowStageRole(context.Background(), 9, 1, 5); err != nil {
		t.Fatalf("RemoveWorkflowStageRole() error = %v", err)
	}
	if err := api.ReorderWorkflowStageRoles(context.Background(), 9, 1, []int64{5, 6}); err != nil {
		t.Fatalf("ReorderWorkflowStageRoles() error = %v", err)
	}
	if _, err := api.CompleteTicket(context.Background(), "11", "done"); err != nil {
		t.Fatalf("CompleteTicket() error = %v", err)
	}
	if _, err := api.ReopenTicket(context.Background(), "11", "reopen"); err != nil {
		t.Fatalf("ReopenTicket() error = %v", err)
	}
	if _, err := api.DraftTicket(context.Background(), "11", "draft"); err != nil {
		t.Fatalf("DraftTicket() error = %v", err)
	}
	if _, err := api.UndraftTicket(context.Background(), "11", "ready"); err != nil {
		t.Fatalf("UndraftTicket() error = %v", err)
	}
	if _, err := api.NextTicket(context.Background(), "11"); err != nil {
		t.Fatalf("NextTicket() error = %v", err)
	}
	if _, err := api.PreviousTicket(context.Background(), "11"); err != nil {
		t.Fatalf("PreviousTicket() error = %v", err)
	}
}

func TestRemoteClientLogoutHeartbeatCloneAndAgentRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/logout":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/heartbeat":
			username, password, ok := r.BasicAuth()
			if !ok || username != "agent-1" || password != "pw" {
				t.Fatalf("BasicAuth = (%q, %q, %v), want agent-1/pw/true", username, password, ok)
			}
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/request":
			username, password, ok := r.BasicAuth()
			if !ok || username != "agent-1" || password != "pw" {
				t.Fatalf("BasicAuth = (%q, %q, %v), want agent-1/pw/true", username, password, ok)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(body), `"project_id":7`) && !strings.Contains(string(body), `"ticket_id":"11"`) {
				t.Fatalf("request body = %s, want project_id or ticket_id", string(body))
			}
			_, _ = w.Write([]byte(`{"status":"NEW","ticket":{"ticket_id":"11","project_id":7,"title":"T","type":"task","stage":"develop","state":"active","status":"develop/active"},"parents":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/clone":
			_, _ = w.Write([]byte(`{"ticket_id":"12","project_id":7,"title":"clone","type":"task","stage":"design","state":"idle","status":"design/idle"}`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL, Token: "token-123"})
	if err := api.Logout(context.Background()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if err := api.HeartbeatAgent(context.Background(), "agent-1", "pw", "IDLE"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}
	if _, err := api.RequestAgentWork(context.Background(), AgentRequest{ID: "agent-1", Password: "pw", ProjectID: 7, DryRun: true}); err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	ticketID := "11"
	if _, err := api.RequestAgentWork(context.Background(), AgentRequest{ID: "agent-1", Password: "pw", TicketID: &ticketID}); err != nil {
		t.Fatalf("RequestAgentWork(ticket) error = %v", err)
	}
	if _, err := api.CloneTicket(context.Background(), "11", "clone it"); err != nil {
		t.Fatalf("CloneTicket() error = %v", err)
	}
}

func TestRemoteClientBasicAuthErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	api := New(config.Config{Location: server.URL})
	if err := api.HeartbeatAgent(context.Background(), "agent-1", "pw", "IDLE"); err == nil || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("HeartbeatAgent() error = %v, want status error", err)
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
			var payload RegisterRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode register payload error = %v", err)
			}
			if payload.Email != "alice@example.com" {
				t.Fatalf("register email = %q, want %q", payload.Email, "alice@example.com")
			}
			_, _ = w.Write([]byte(`{"user_id":"u1","username":"alice","email":"alice@example.com","role":"user","enabled":true,"password":"generated-secret"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/users":
			var payload UserCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create-user payload error = %v", err)
			}
			if payload.Email != "bob@example.com" {
				t.Fatalf("create-user email = %q, want %q", payload.Email, "bob@example.com")
			}
			_, _ = w.Write([]byte(`{"user_id":"u2","username":"bob","email":"bob@example.com","role":"user","enabled":true,"password":"managed-secret"}`))
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
	if _, err := api.ResetUserPassword(context.Background(), "alice", "newpassword1"); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	user, password, err := api.RegisterWithParams(context.Background(), RegisterRequest{
		Username: "alice",
		Email:    "alice@example.com",
	})
	if err != nil {
		t.Fatalf("RegisterWithParams() error = %v", err)
	}
	if user.Email != "alice@example.com" || password != "generated-secret" {
		t.Fatalf("RegisterWithParams() = %#v password=%q", user, password)
	}
	created, createdPassword, err := api.CreateUserWithParams(context.Background(), UserCreateRequest{
		Username: "bob",
		Email:    "bob@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUserWithParams() error = %v", err)
	}
	if created.Email != "bob@example.com" || createdPassword != "managed-secret" {
		t.Fatalf("CreateUserWithParams() = %#v password=%q", created, createdPassword)
	}
	if resp, err := api.Login(context.Background(), "alice", "secret"); err != nil {
		t.Fatalf("Login() error = %v", err)
	} else if resp.Token != "tok" {
		t.Fatalf("Login() token = %q, want tok", resp.Token)
	}
}
