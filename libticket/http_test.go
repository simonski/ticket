package libticket_test

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/testutil"
	"github.com/simonski/ticket/libticket"
)

func TestHTTPServiceContract(t *testing.T) {
	RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		_, svc := newRemoteFixture(t)
		return svc
	}, ContractOptions{RequireStatusOwnership: false})
}

func TestHTTPServiceStatusUnauthenticated(t *testing.T) {
	fixture, _ := newRemoteFixture(t)

	svc := libticket.NewHTTP(config.Config{Location: fixture.server.URL})
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Authenticated {
		t.Fatalf("Status().Authenticated = true, want false: %#v", status)
	}
}

func TestHTTPServiceRegisterLoginLogoutRoundTrip(t *testing.T) {
	fixture, _ := newRemoteFixture(t)

	svc := libticket.NewHTTP(config.Config{Location: fixture.server.URL})
	user, err := svc.Register(context.Background(), "alice", "secret12")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("Register() = %#v", user)
	}

	loggedIn, token, err := svc.Login(context.Background(), "alice", "secret12")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if loggedIn.Username != "alice" || token == "" {
		t.Fatalf("Login() = %#v, token=%q", loggedIn, token)
	}

	authed := libticket.NewHTTP(config.Config{Location: fixture.server.URL, Token: "secret12", Username: "alice"})
	status, err := authed.Status(context.Background())
	if err != nil {
		t.Fatalf("Status(authenticated) error = %v", err)
	}
	if !status.Authenticated || status.User == nil || status.User.Username != "alice" {
		t.Fatalf("Status(authenticated) = %#v", status)
	}

	if err := authed.Logout(context.Background()); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
}

func TestHTTPServiceSetTicketParent(t *testing.T) {
	_, svc := newRemoteFixture(t)

	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	child, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Child",
	})
	if err != nil {
		t.Fatalf("CreateTicket(child) error = %v", err)
	}

	updated, err := svc.SetTicketParent(context.Background(), child.ID, parent.ID, "")
	if err != nil {
		t.Fatalf("SetTicketParent() error = %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("SetTicketParent() = %#v", updated)
	}

	detached, err := svc.UnsetTicketParent(context.Background(), child.ID, "")
	if err != nil {
		t.Fatalf("UnsetTicketParent() error = %v", err)
	}
	if detached.ParentID != nil {
		t.Fatalf("UnsetTicketParent() = %#v", detached)
	}
}

func TestHTTPServiceDeleteTicket(t *testing.T) {
	_, svc := newRemoteFixture(t)

	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := svc.DeleteTicket(context.Background(), ticket.ID); err != nil {
		t.Fatalf("DeleteTicket() error = %v", err)
	}
	if _, err := svc.GetTicketByID(context.Background(), ticket.ID); !errors.Is(err, store.ErrTicketNotFound) && (err == nil || err.Error() != "ticket not found") {
		t.Fatalf("GetTicket(deleted) error = %v, want ticket not found", err)
	}
}

func TestHTTPServiceProjectAccessRequestManagement(t *testing.T) {
	fixture, adminSvc := newRemoteFixture(t)

	project, err := adminSvc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:            "GATE",
		Title:             "Gated Project",
		Visibility:        "private",
		AcceptsNewMembers: true,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := store.CreateUser(context.Background(), fixture.db, "requester", "pass1234!", "user"); err != nil {
		t.Fatalf("CreateUser(requester) error = %v", err)
	}

	requesterSvc := libticket.NewHTTP(config.Config{
		Location: fixture.server.URL,
		Username: "requester",
		Token:    "pass1234!",
	})
	request, err := requesterSvc.CreateProjectAccessRequest(context.Background(), project.Prefix, "please let me in")
	if err != nil {
		t.Fatalf("CreateProjectAccessRequest() error = %v", err)
	}
	if request.Status != "pending" {
		t.Fatalf("CreateProjectAccessRequest().Status = %q, want pending", request.Status)
	}
	if request.ProjectPrefix != "GATE" || request.ProjectTitle != "Gated Project" {
		t.Fatalf("CreateProjectAccessRequest() project metadata = %#v", request)
	}

	myRequests, err := requesterSvc.ListMyProjectAccessRequests(context.Background(), "pending")
	if err != nil {
		t.Fatalf("ListMyProjectAccessRequests() error = %v", err)
	}
	if len(myRequests) != 1 || myRequests[0].ID != request.ID || myRequests[0].ProjectPrefix != "GATE" {
		t.Fatalf("ListMyProjectAccessRequests() = %#v", myRequests)
	}

	requests, err := adminSvc.ListProjectAccessRequests(context.Background(), project.Prefix, "pending")
	if err != nil {
		t.Fatalf("ListProjectAccessRequests() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("ListProjectAccessRequests() len = %d, want 1", len(requests))
	}

	approved, err := adminSvc.SetProjectAccessRequestStatus(context.Background(), project.Prefix, request.ID, "approved", "Approved for design review")
	if err != nil {
		t.Fatalf("SetProjectAccessRequestStatus() error = %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("SetProjectAccessRequestStatus().Status = %q, want approved", approved.Status)
	}
	if approved.DecisionMessage != "Approved for design review" {
		t.Fatalf("SetProjectAccessRequestStatus().DecisionMessage = %q", approved.DecisionMessage)
	}

	approvedRequests, err := requesterSvc.ListMyProjectAccessRequests(context.Background(), "approved")
	if err != nil {
		t.Fatalf("ListMyProjectAccessRequests(approved) error = %v", err)
	}
	if len(approvedRequests) != 1 || approvedRequests[0].ID != request.ID || approvedRequests[0].Status != "approved" {
		t.Fatalf("ListMyProjectAccessRequests(approved) = %#v", approvedRequests)
	}
	if approvedRequests[0].DecisionMessage != "Approved for design review" {
		t.Fatalf("ListMyProjectAccessRequests(approved).DecisionMessage = %q", approvedRequests[0].DecisionMessage)
	}

	members, err := adminSvc.ListProjectMembers(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	found := false
	for _, member := range members {
		if member.UserID == request.UserID && member.Role == store.ProjectRoleObserver {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("approved requester %q not added as observer: %#v", request.UserID, members)
	}

	notifications, err := requesterSvc.ListMyNotifications(context.Background(), store.UserNotificationStatusUnread, 10)
	if err != nil {
		t.Fatalf("ListMyNotifications() error = %v", err)
	}
	if len(notifications) != 1 || notifications[0].Kind != store.UserNotificationKindProjectAccessApproved {
		t.Fatalf("ListMyNotifications() = %#v", notifications)
	}
	readNotification, err := requesterSvc.MarkNotificationRead(context.Background(), notifications[0].ID)
	if err != nil {
		t.Fatalf("MarkNotificationRead() error = %v", err)
	}
	if readNotification.Status != store.UserNotificationStatusRead {
		t.Fatalf("MarkNotificationRead() = %#v", readNotification)
	}
}

func TestHTTPServiceUpdateTicketSupportsExpandedFields(t *testing.T) {
	_, svc := newRemoteFixture(t)

	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "epic",
		Title:     "Parent",
	})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID:          1,
		Type:               "task",
		Title:              "Child",
		Description:        "old description",
		AcceptanceCriteria: "old ac",
		Priority:           1,
		EstimateEffort:     2,
		EstimateComplete:   "2026-04-01T09:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateTicket(task) error = %v", err)
	}
	// Assign the ticket directly and set it active.
	updated, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{
		Title:              "Updated Child",
		Description:        "new description",
		AcceptanceCriteria: "new ac",
		ParentID:           &parent.ID,
		Assignee:           "admin",
		Status:             "design/active",
		Priority:           3,
		Order:              7,
		EstimateEffort:     5,
		EstimateComplete:   "2026-04-15T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}
	if updated.Title != "Updated Child" || updated.Description != "new description" || updated.AcceptanceCriteria != "new ac" || updated.Status != "design/active" || updated.Priority != 3 || updated.Order != 7 || updated.EstimateEffort != 5 || updated.EstimateComplete != "2026-04-15T12:00:00Z" {
		t.Fatalf("UpdateTicket() = %#v", updated)
	}
	if updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("UpdateTicket() parent = %#v", updated)
	}
}

func TestHTTPServiceCoversLifecycleAliasesWorkflowStagesAndAgentOps(t *testing.T) {
	_, svc := newRemoteFixture(t)
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, libticket.ProjectCreateRequest{Title: "Advanced Project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := svc.RenameProjectPrefix(ctx, project.ID, "ADV"); err == nil {
		t.Fatal("RenameProjectPrefix() error = nil, want remote unsupported error")
	}
	if err := svc.SetProjectDefaultDraft(ctx, project.ID, true); err != nil {
		t.Fatalf("SetProjectDefaultDraft() error = %v", err)
	}
	projectAfter, err := svc.GetProject(ctx, strconv.FormatInt(project.ID, 10))
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if !projectAfter.DefaultDraft {
		t.Fatalf("GetProject().DefaultDraft = %v, want true", projectAfter.DefaultDraft)
	}

	wf, err := svc.CreateWorkflow(ctx, libticket.WorkflowRequest{Name: "wf-advanced", Description: "d"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	stage, err := svc.AddWorkflowStage(ctx, wf.ID, libticket.WorkflowStageRequest{
		StageName:          "develop",
		Description:        "ways",
		AcceptanceCriteria: "ready",
		DefinitionOfDone:   "done",
		SortOrder:          1,
	})
	if err != nil {
		t.Fatalf("AddWorkflowStage() error = %v", err)
	}
	stage, err = svc.UpdateWorkflowStage(ctx, stage.ID, libticket.WorkflowStageRequest{
		StageName:          "develop",
		Description:        "updated ways",
		AcceptanceCriteria: "updated ready",
		DefinitionOfDone:   "updated done",
	})
	if err != nil {
		t.Fatalf("UpdateWorkflowStage() error = %v", err)
	}
	if _, err := svc.GetWorkflowStage(ctx, stage.ID); err != nil {
		t.Fatalf("GetWorkflowStage() error = %v", err)
	}
	if stages, err := svc.ListWorkflowStages(ctx, wf.ID); err != nil {
		t.Fatalf("ListWorkflowStages() error = %v", err)
	} else if len(stages) != 1 {
		t.Fatalf("ListWorkflowStages() len = %d, want 1", len(stages))
	}

	role, err := svc.CreateRole(ctx, libticket.RoleRequest{Title: "Engineer"})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if err := svc.AddWorkflowStageRole(ctx, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("AddWorkflowStageRole() error = %v", err)
	}
	if err := svc.ReorderWorkflowStageRoles(ctx, wf.ID, stage.ID, []int64{role.ID}); err != nil {
		t.Fatalf("ReorderWorkflowStageRoles() error = %v", err)
	}
	if err := svc.RemoveWorkflowStageRole(ctx, wf.ID, stage.ID, role.ID); err != nil {
		t.Fatalf("RemoveWorkflowStageRole() error = %v", err)
	}

	ticket, err := svc.CreateTicket(ctx, libticket.TicketCreateRequest{ProjectID: project.ID, Type: "task", Title: "Lifecycle task"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if _, err := svc.DraftTicket(ctx, ticket.ID, "draft"); err != nil {
		t.Fatalf("DraftTicket() error = %v", err)
	}
	if _, err := svc.UndraftTicket(ctx, ticket.ID, "ready"); err != nil {
		t.Fatalf("UndraftTicket() error = %v", err)
	}
	if _, err := svc.CompleteTicket(ctx, ticket.ID, "complete"); err != nil {
		t.Fatalf("CompleteTicket() error = %v", err)
	}
	if _, err := svc.ReopenTicket(ctx, ticket.ID, "reopen"); err != nil {
		t.Fatalf("ReopenTicket() error = %v", err)
	}
	if _, err := svc.NextTicket(ctx, ticket.ID); err == nil {
		t.Fatal("NextTicket() error = nil, want state precondition error")
	}
	if _, err := svc.PreviousTicket(ctx, ticket.ID); err == nil {
		t.Fatal("PreviousTicket() error = nil, want state precondition error")
	}
	if _, err := svc.ListHistoryPaged(ctx, ticket.ID, 5, 0); err != nil {
		t.Fatalf("ListHistoryPaged() error = %v", err)
	}

	agent, password, err := svc.CreateAgent(ctx, libticket.AgentCreateRequest{Password: "agentpw"})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if password == "" {
		t.Fatal("CreateAgent() password = empty")
	}
	if _, err := svc.RegisterAgent(ctx, libticket.AgentRegisterRequest{ID: agent.ID, Password: password}); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}
	if err := svc.HeartbeatAgent(ctx, agent.ID, password, "online"); err != nil {
		t.Fatalf("HeartbeatAgent() error = %v", err)
	}
	if _, err := svc.RequestAgentWork(ctx, libticket.AgentRequest{ID: agent.ID, Password: password, ProjectID: project.ID, DryRun: true}); err != nil {
		t.Fatalf("RequestAgentWork() error = %v", err)
	}
	// Push model: the orchestrator assigns work; simulate that with a fresh ticket
	// put at develop/active and assigned to the agent before it reports a result.
	workTicket, err := svc.CreateTicket(ctx, libticket.TicketCreateRequest{ProjectID: project.ID, Type: "task", Title: "Agent work item"})
	if err != nil {
		t.Fatalf("CreateTicket(work) error = %v", err)
	}
	if _, err := svc.UpdateTicket(ctx, workTicket.ID, libticket.TicketUpdateRequest{
		Title: workTicket.Title, Stage: "develop", State: "active", Assignee: agent.Username,
	}); err != nil {
		t.Fatalf("assign ticket to agent: %v", err)
	}
	if _, err := svc.AgentUpdateTicket(ctx, workTicket.ID, libticket.AgentTicketUpdateRequest{ID: agent.ID, Password: password, Result: "done"}); err != nil {
		t.Fatalf("AgentUpdateTicket() error = %v", err)
	}
}

func TestHTTPServiceCountRequiresAuth(t *testing.T) {
	fixture, _ := newRemoteFixture(t)

	svc := libticket.NewHTTP(config.Config{Location: fixture.server.URL})
	if _, err := svc.Count(context.Background(), nil); err == nil {
		t.Fatal("Count() error = nil, want auth error")
	}
}

func TestHTTPServicePropagatesMalformedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	svc := libticket.NewHTTP(config.Config{Location: server.URL})
	if _, err := svc.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want JSON decode error")
	}
}

func TestHTTPServicePropagatesNonJSONErrorBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain failure", http.StatusForbidden)
	}))
	defer server.Close()

	svc := libticket.NewHTTP(config.Config{Location: server.URL})
	if _, err := svc.Count(context.Background(), nil); err == nil {
		t.Fatal("Count() error = nil, want HTTP status error")
	}
}

func TestHTTPServiceHandlesNetworkFailure(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	svc := libticket.NewHTTP(config.Config{Location: "http://" + addr})
	if _, err := svc.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want network error")
	}
}

type remoteFixture struct {
	server *httptest.Server
	db     *sql.DB
}

func newRemoteFixture(t *testing.T) (*remoteFixture, *libticket.HTTPService) {
	t.Helper()
	dbPath := testutil.SeededDBPath(t, "secret12")
	t.Setenv("TICKET_HOME", filepath.Dir(dbPath))
	t.Setenv("TICKET_WRITE_RATE_LIMIT", "100000")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	handler, err := server.Handler(db, "test-version", false, nil, "", "")
	if err != nil {
		t.Fatalf("server.Handler() error = %v", err)
	}
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	svc := libticket.NewHTTP(config.Config{
		Location: httpServer.URL,
		Username: "admin",
		Token:    "secret12",
	})
	return &remoteFixture{server: httpServer, db: db}, svc
}
