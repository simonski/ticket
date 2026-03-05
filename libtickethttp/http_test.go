package libtickethttp

import (
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"github.com/simonski/ticket/libtickettest"
)

func TestHTTPServiceContract(t *testing.T) {
	libtickettest.RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		_, svc := newRemoteFixture(t)
		return svc
	}, libtickettest.ContractOptions{RequireStatusOwnership: false})
}

func TestHTTPServiceStatusUnauthenticated(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	fixture, _ := newRemoteFixture(t)

	svc := New(config.Config{ServerURL: fixture.server.URL})
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Authenticated {
		t.Fatalf("Status().Authenticated = true, want false: %#v", status)
	}
}

func TestHTTPServiceRegisterLoginLogoutRoundTrip(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	fixture, _ := newRemoteFixture(t)

	svc := New(config.Config{ServerURL: fixture.server.URL})
	user, err := svc.Register("alice", "secret")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("Register() = %#v", user)
	}

	loggedIn, token, err := svc.Login("alice", "secret")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if loggedIn.Username != "alice" || token == "" {
		t.Fatalf("Login() = %#v, token=%q", loggedIn, token)
	}

	authed := New(config.Config{ServerURL: fixture.server.URL, Token: token, Username: "alice"})
	status, err := authed.Status()
	if err != nil {
		t.Fatalf("Status(authenticated) error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" {
		t.Fatalf("Status(authenticated) = %#v", status)
	}

	if err := authed.Logout(); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
}

func TestHTTPServiceSetTicketParent(t *testing.T) {
	_, svc := newRemoteFixture(t)

	parent, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	child, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Child",
	})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	updated, err := svc.SetTicketParent(child.ID, parent.ID)
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", updated)
	}

	detached, err := svc.UnsetTicketParent(child.ID)
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}
}

func TestHTTPServiceDeleteTicket(t *testing.T) {
	_, svc := newRemoteFixture(t)

	task, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := svc.DeleteTicket(task.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := svc.GetTicketByID(task.ID); !errors.Is(err, store.ErrTicketNotFound) && (err == nil || err.Error() != "ticket not found") {
		t.Fatalf("GetTicket(deleted) error = %v, want ticket not found", err)
	}
}

func TestHTTPServiceUpdateTicketSupportsExpandedFields(t *testing.T) {
	_, svc := newRemoteFixture(t)

	parent, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	task, err := svc.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:          1,
		Type:               "task",
		Title:              "Child",
		Description:        "old description",
		AcceptanceCriteria: "old ac",
		Priority:           1,
		EstimateEffort:     2,
		EstimateComplete:   "2026-04-01T09:00:00Z",
		Stage:              "develop",
		State:              "idle",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	requested, err := svc.RequestTicket(libticket.TicketRequest{ProjectID: 1, TicketID: &task.ID})
	if err != nil {
		t.Fatalf("RequestTicket() error = %v", err)
	}

	updated, err := svc.UpdateTicket(task.ID, libticket.TicketUpdateRequest{
		Title:              "Updated Child",
		Description:        "new description",
		AcceptanceCriteria: "new ac",
		ParentID:           &parent.ID,
		Assignee:           requested.Ticket.Assignee,
		Status:             "develop/active",
		Priority:           3,
		Order:              7,
		EstimateEffort:     5,
		EstimateComplete:   "2026-04-15T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Title != "Updated Child" || updated.Description != "new description" || updated.AcceptanceCriteria != "new ac" || updated.Status != "develop/active" || updated.Priority != 3 || updated.Order != 7 || updated.EstimateEffort != 5 || updated.EstimateComplete != "2026-04-15T12:00:00Z" {
		t.Fatalf("UpdateTicket() = %#v", updated)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("UpdateTicket() parent = %#v", updated)
	}
}

func TestHTTPServiceCountRequiresAuth(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	fixture, _ := newRemoteFixture(t)

	svc := New(config.Config{ServerURL: fixture.server.URL})
	if _, err := svc.Count(nil); err == nil {
		t.Fatal("Count() error = nil, want auth error")
	}
}

func TestHTTPServicePropagatesMalformedJSON(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	svc := New(config.Config{ServerURL: server.URL})
	if _, err := svc.Status(); err == nil {
		t.Fatal("Status() error = nil, want JSON decode error")
	}
}

func TestHTTPServicePropagatesNonJSONErrorBody(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain failure", http.StatusForbidden)
	}))
	defer server.Close()

	svc := New(config.Config{ServerURL: server.URL})
	if _, err := svc.Count(nil); err == nil {
		t.Fatal("Count() error = nil, want HTTP status error")
	}
}

func TestHTTPServiceHandlesNetworkFailure(t *testing.T) {
	t.Setenv("TICKET_MODE", "remote")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	svc := New(config.Config{ServerURL: "http://" + addr})
	if _, err := svc.Status(); err == nil {
		t.Fatal("Status() error = nil, want network error")
	}
}

type remoteFixture struct {
	server *httptest.Server
	db     *sql.DB
}

func newRemoteFixture(t *testing.T) (*remoteFixture, *Service) {
	t.Helper()
	t.Setenv("TICKET_MODE", "remote")
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	dbPath := filepath.Join(tempDir, "ticket.db")
	if err := store.Init(dbPath, "admin", "secret"); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	handler, err := server.Handler(db, "test-version", false, nil)
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	raw := client.New(config.Config{ServerURL: httpServer.URL})
	auth, err := raw.Login("admin", "secret")
	if err != nil {
		t.Fatalf("raw Login() error = %v", err)
	}

	svc := New(config.Config{
		ServerURL: httpServer.URL,
		Token:     auth.Token,
		Username:  auth.User.Username,
	})
	return &remoteFixture{server: httpServer, db: db}, svc
}
