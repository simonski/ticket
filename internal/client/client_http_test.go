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
)

func TestRemoteClientSendsAuthHeaderAndParsesStatus(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

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

	api := New(config.Config{ServerURL: server.URL, Token: "token-123"})
	status, err := api.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" || status.ServerVersion != "1.2.3" {
		t.Fatalf("Status() = %#v", status)
	}
}

func TestRemoteClientListTicketsFilteredBuildsQuery(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

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

	api := New(config.Config{ServerURL: server.URL})
	if _, err := api.ListTicketsFiltered(7, "bug", "", "", "develop/idle", "needle", "alice", 25); err != nil {
		t.Fatalf("ListTicketsFiltered() error = %v", err)
	}
}

func TestRemoteClientRequestTicketPostsJSON(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

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
		if payload.ProjectID != 3 || payload.TicketID == nil || *payload.TicketID != 9 {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"REJECTED"}`))
	}))
	defer server.Close()

	taskID := int64(9)
	api := New(config.Config{ServerURL: server.URL, Token: "token-123"})
	resp, err := api.RequestTicket(TicketRequest{ProjectID: 3, TicketID: &taskID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}
	if resp.Status != "REJECTED" {
		t.Fatalf("RequestTicket() = %#v", resp)
	}
}

func TestRemoteClientReturnsAPIErrorMessage(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"denied"}`))
	}))
	defer server.Close()

	api := New(config.Config{ServerURL: server.URL})
	if _, err := api.Count(nil); err == nil || err.Error() != "denied" {
		t.Fatalf("Count() error = %v, want denied", err)
	}
}

func TestRemoteClientReturnsStatusErrorForNonJSONFailures(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain failure", http.StatusBadGateway)
	}))
	defer server.Close()

	api := New(config.Config{ServerURL: server.URL})
	if _, err := api.Count(nil); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("Count() error = %v, want status error containing 502", err)
	}
}

func TestRemoteClientReturnsDecodeErrorOnMalformedJSON(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":`))
	}))
	defer server.Close()

	api := New(config.Config{ServerURL: server.URL})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want decode error")
	}
}

func TestRemoteClientHandlesNetworkFailure(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	api := New(config.Config{ServerURL: "http://" + addr})
	if _, err := api.Status(); err == nil {
		t.Fatal("Status() error = nil, want network error")
	}
}

func TestLocalModeClientRejectsRemoteOnlyAuthCalls(t *testing.T) {
	t.Setenv("TICKET_MODE", "local")

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
	t.Setenv("TICKET_MODE", "remote")

	projectID := int64(7)
	taskID := int64(11)
	dependsOn := int64(12)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/users":
			_, _ = w.Write([]byte(`{"user_id":2,"username":"alice","role":"user","enabled":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/users":
			_, _ = w.Write([]byte(`[{"user_id":2,"username":"alice","role":"user","enabled":true}]`))
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
			_, _ = w.Write([]byte(`{"ticket_id":11,"project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11":
			_, _ = w.Write([]byte(`{"ticket_id":11,"project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/tickets/11":
			_, _ = w.Write([]byte(`{"status":"deleted"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/tickets/11":
			var payload TicketUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode(task update) error = %v", err)
			}
			parentJSON := ""
			if payload.ParentID != nil {
				parentJSON = `,"parent_id":` + strconv.FormatInt(*payload.ParentID, 10)
			}
			_, _ = w.Write([]byte(`{"ticket_id":11,"project_id":7,"title":"T","type":"task","stage":"develop","state":"active","status":"develop/active"` + parentJSON + `}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/clone":
			_, _ = w.Write([]byte(`{"ticket_id":21,"project_id":7,"title":"T","type":"task","stage":"design","state":"idle","status":"design/idle","clone_of":11}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/history":
			_, _ = w.Write([]byte(`[{"id":1,"ticket_id":11,"event_type":"ticket_updated"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/11/comments":
			_, _ = w.Write([]byte(`{"id":1,"item_id":11,"comment":"hello"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/comments":
			_, _ = w.Write([]byte(`[{"id":1,"item_id":11,"comment":"hello"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/dependencies":
			_, _ = w.Write([]byte(`{"id":1,"project_id":7,"ticket_id":11,"depends_on":12}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/dependencies"):
			if r.URL.Query().Get("project_id") != "7" || r.URL.Query().Get("ticket_id") != "11" || r.URL.Query().Get("depends_on") != "12" {
				t.Fatalf("dependency delete query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/tickets/11/dependencies":
			_, _ = w.Write([]byte(`[{"id":1,"project_id":7,"ticket_id":11,"depends_on":12}]`))
		default:
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	api := New(config.Config{ServerURL: server.URL})

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
	if _, err := api.SetTicketParent(taskID, dependsOn); err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if _, err := api.UnsetTicketParent(taskID); err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if _, err := api.CloneTicket(taskID); err != nil {
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
