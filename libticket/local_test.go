package libticket_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/testutil"
	"github.com/simonski/ticket/libticket"
)

func localServiceConfig(dbPath string) config.Config {
	return config.Config{Location: dbPath}
}

func seededLocalDBPath(t *testing.T) string {
	t.Helper()

	dbPath := testutil.SeededDBPath(t, "secret12")
	t.Setenv("TICKET_HOME", filepath.Dir(dbPath))
	return dbPath
}

func TestLocalServiceContract(t *testing.T) {

	RunServiceContractTests(t, func(t *testing.T) libticket.Service {
		dbPath := testutil.SeededDBPath(t, "secret12")
		t.Setenv("TICKET_HOME", filepath.Dir(dbPath))
		return libticket.NewLocal(localServiceConfig(dbPath))
	}, ContractOptions{RequireStatusOwnership: false})
}

func TestLocalServiceStatusDefaultsToAdmin(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Authenticated || status.User == nil {
		t.Fatalf("Status() = %#v, want authenticated admin", status)
	}
	if status.User.Username != "admin" {
		t.Fatalf("Status().User.Username = %q, want admin", status.User.Username)
	}
}

func TestLocalServiceRemoteAuthCommandsFail(t *testing.T) {
	t.Parallel()
	svc := libticket.NewLocal(config.Config{})

	if _, err := svc.Register(context.Background(), "alice", "secret12"); err == nil {
		t.Fatal("Register() error = nil, want remote-mode error")
	}
	if _, _, err := svc.Login(context.Background(), "alice", "secret12"); err == nil {
		t.Fatal("Login() error = nil, want remote-mode error")
	}
	if err := svc.Logout(context.Background()); err == nil {
		t.Fatal("Logout() error = nil, want remote-mode error")
	}
}

func TestLocalServiceStatusFailsWhenDatabaseMissing(t *testing.T) {

	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	svc := libticket.NewLocal(localServiceConfig(filepath.Join(tempDir, "ticket.db")))
	if _, err := svc.Status(context.Background()); err == nil {
		t.Fatal("Status() error = nil, want missing database error")
	}
}

func TestLocalUsernameUsesEnvironmentFallbacks(t *testing.T) {

	t.Setenv("USER", "env-user")
	t.Setenv("USERNAME", "env-username")

	got := libticket.LocalUsername()
	if got != "admin" {
		t.Fatalf("LocalUsername() = %q, want admin", got)
	}
}

func TestLocalServiceUsesTicketHomeDatabasePath(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("ListProjects() returned no projects")
	}
}

func TestLocalServiceSetTicketParent(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateTicket(parent) error = %v", err)
	}
	child, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Child"})
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

func TestLocalServiceProjectAccessRequestManagement(t *testing.T) {
	dbPath := seededLocalDBPath(t)

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	adminSvc := libticket.NewLocal(localServiceConfig(dbPath))
	project, err := adminSvc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:            "GATE",
		Title:             "Gated Project",
		Visibility:        "private",
		AcceptsNewMembers: true,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	requester, err := store.CreateUser(context.Background(), db, "requester", "pass1234!", "user")
	if err != nil {
		t.Fatalf("CreateUser(requester) error = %v", err)
	}
	request, err := store.CreateProjectAccessRequest(context.Background(), db, project.ID, requester.ID, "please let me in")
	if err != nil {
		t.Fatalf("CreateProjectAccessRequest(store) error = %v", err)
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

	members, err := adminSvc.ListProjectMembers(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ListProjectMembers() error = %v", err)
	}
	found := false
	for _, member := range members {
		if member.UserID == requester.ID && member.Role == store.ProjectRoleObserver {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("approved requester %q not added as observer: %#v", requester.ID, members)
	}

	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	adminRequest, err := store.CreateProjectAccessRequest(context.Background(), db, project.ID, adminUser.ID, "admin needs access history")
	if err != nil {
		t.Fatalf("CreateProjectAccessRequest(admin) error = %v", err)
	}
	myRequests, err := adminSvc.ListMyProjectAccessRequests(context.Background(), "pending")
	if err != nil {
		t.Fatalf("ListMyProjectAccessRequests() error = %v", err)
	}
	found = false
	for _, item := range myRequests {
		if item.ID == adminRequest.ID && item.ProjectPrefix == "GATE" && item.ProjectTitle == "Gated Project" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListMyProjectAccessRequests() missing admin request: %#v", myRequests)
	}

	rejectedRequest, err := adminSvc.SetProjectAccessRequestStatus(context.Background(), project.Prefix, adminRequest.ID, "rejected", "Need more context first")
	if err != nil {
		t.Fatalf("SetProjectAccessRequestStatus(admin request) error = %v", err)
	}
	if rejectedRequest.Status != "rejected" {
		t.Fatalf("SetProjectAccessRequestStatus(admin request).Status = %q, want rejected", rejectedRequest.Status)
	}
	if rejectedRequest.DecisionMessage != "Need more context first" {
		t.Fatalf("SetProjectAccessRequestStatus(admin request).DecisionMessage = %q", rejectedRequest.DecisionMessage)
	}

	notifications, err := adminSvc.ListMyNotifications(context.Background(), store.UserNotificationStatusUnread, 10)
	if err != nil {
		t.Fatalf("ListMyNotifications() error = %v", err)
	}
	if len(notifications) != 1 || notifications[0].Kind != store.UserNotificationKindProjectAccessRejected {
		t.Fatalf("ListMyNotifications() = %#v", notifications)
	}
	if !strings.Contains(notifications[0].Message, "Need more context first") {
		t.Fatalf("notification message = %q", notifications[0].Message)
	}
	readNotification, err := adminSvc.MarkNotificationRead(context.Background(), notifications[0].ID)
	if err != nil {
		t.Fatalf("MarkNotificationRead() error = %v", err)
	}
	if readNotification.Status != store.UserNotificationStatusRead {
		t.Fatalf("MarkNotificationRead() = %#v", readNotification)
	}
}

func TestLocalServiceUpdateTicketSupportsExpandedFields(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	parent, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "epic", Title: "Parent"})
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

func TestLocalServiceCoversLifecycleAliasesWorkflowStagesAndAgentOps(t *testing.T) {
	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, libticket.ProjectCreateRequest{Title: "Advanced Project"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	renamedCount, err := svc.RenameProjectPrefix(ctx, project.ID, "ADV")
	if err != nil {
		t.Fatalf("RenameProjectPrefix() error = %v", err)
	}
	if renamedCount < 0 {
		t.Fatalf("RenameProjectPrefix() = %d, want non-negative count", renamedCount)
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
	if stage.Description != "updated ways" || stage.DefinitionOfReady != "updated ready" {
		t.Fatalf("UpdateWorkflowStage() = %#v", stage)
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
	if _, err := svc.AgentUpdateTicket(ctx, ticket.ID, libticket.AgentTicketUpdateRequest{ID: agent.ID, Password: password, Result: "done"}); err != nil {
		t.Fatalf("AgentUpdateTicket() error = %v", err)
	}
}

func TestLocalServiceIgnoresOwnershipForStatusChanges(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{
		ProjectID: 1,
		Type:      "task",
		Title:     "Unassigned local task",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	// Advance through all stages: design -> develop -> test -> done (4-stage Workflow)
	// Each state=success on a non-final stage auto-advances to next stage with state=idle
	for _, wantStatus := range []string{"develop/idle", "test/idle", "done/idle", "done/success"} {
		ticket, err = svc.GetTicketByID(context.Background(), ticket.ID)
		if err != nil {
			t.Fatalf("GetTicketByID() error = %v", err)
		}
		updated, err := svc.UpdateTicket(context.Background(), ticket.ID, libticket.TicketUpdateRequest{
			Title:       ticket.Title,
			Description: ticket.Description,
			ParentID:    ticket.ParentID,
			Assignee:    ticket.Assignee,
			State:       "success",
		})
		if err != nil {
			t.Fatalf("UpdateTicket() error = %v", err)
		}
		if updated.Status != wantStatus {
			t.Fatalf("UpdateTicket().Status = %q, want %s", updated.Status, wantStatus)
		}
	}
}

func TestLocalServiceDeleteTicket(t *testing.T) {

	dbPath := seededLocalDBPath(t)

	svc := libticket.NewLocal(localServiceConfig(dbPath))
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
	if _, err := svc.GetTicketByID(context.Background(), ticket.ID); !errors.Is(err, store.ErrTicketNotFound) {
		t.Fatalf("GetTicket(deleted) error = %v, want ErrTicketNotFound", err)
	}
}

func newLocalSvc(t *testing.T) libticket.Service {
	t.Helper()
	dbPath := testutil.SeededDBPath(t, "secret12")
	t.Setenv("TICKET_HOME", filepath.Dir(dbPath))
	return libticket.NewLocal(localServiceConfig(dbPath))
}
func TestLocalServiceDeleteProject(t *testing.T) {
	svc := newLocalSvc(t)
	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) == 0 {
		t.Fatal("no projects to delete")
	}
	if err := svc.DeleteProject(context.Background(), projects[0].ID); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
}

func TestLocalServiceResetUserPassword(t *testing.T) {
	svc := newLocalSvc(t)
	user, err := svc.ResetUserPassword(context.Background(), "admin", "newsecret")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("ResetUserPassword().Username = %q, want admin", user.Username)
	}
}

func TestLocalServiceNotReadyTicket(t *testing.T) {
	svc := newLocalSvc(t)
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "Ready test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	_, err = svc.ReadyTicket(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("ReadyTicket() error = %v", err)
	}
	updated, err := svc.NotReadyTicket(context.Background(), ticket.ID, "")
	if err != nil {
		t.Fatalf("NotReadyTicket() error = %v", err)
	}
	if !updated.Draft {
		t.Fatal("NotReadyTicket() did not set draft flag")
	}
}

func TestLocalServiceSetUnsetTicketWorkflow(t *testing.T) {
	svc := newLocalSvc(t)
	wf, err := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{Name: "Test WF"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}
	ticket, err := svc.CreateTicket(context.Background(), libticket.TicketCreateRequest{ProjectID: 1, Type: "task", Title: "WF test"})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	updated, err := svc.SetTicketWorkflow(context.Background(), ticket.ID, wf.ID)
	if err != nil {
		t.Fatalf("SetTicketWorkflow() error = %v", err)
	}
	if updated.WorkflowID == nil || *updated.WorkflowID != wf.ID {
		t.Fatalf("SetTicketWorkflow() workflow_id = %v, want %d", updated.WorkflowID, wf.ID)
	}
	unset, err := svc.UnsetTicketWorkflow(context.Background(), ticket.ID)
	if err != nil {
		t.Fatalf("UnsetTicketWorkflow() error = %v", err)
	}
	if unset.WorkflowID != nil {
		t.Fatalf("UnsetTicketWorkflow() workflow_id = %v, want nil", unset.WorkflowID)
	}
}

func TestLocalServiceListAgentStatuses(t *testing.T) {
	svc := newLocalSvc(t)
	statuses, err := svc.ListAgentStatuses(context.Background())
	if err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	// No agents yet, should return empty.
	if statuses == nil {
		t.Fatal("ListAgentStatuses() returned nil, want empty slice")
	}
}

func TestLocalServiceAgentConfig(t *testing.T) {
	svc := newLocalSvc(t)
	agent, _, err := svc.CreateAgent(context.Background(), libticket.AgentCreateRequest{})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if err := svc.SetAgentConfig(context.Background(), agent.ID, "poll_interval", "5"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	entries, err := svc.ListAgentConfig(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "poll_interval" || entries[0].Value != "5" {
		t.Fatalf("ListAgentConfig() = %v, want [{poll_interval 5}]", entries)
	}
	if err := svc.DeleteAgentConfig(context.Background(), agent.ID, "poll_interval"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	entries, err = svc.ListAgentConfig(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() after delete error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ListAgentConfig() after delete = %v, want empty", entries)
	}
}

func TestLocalServiceStoryCRUD(t *testing.T) {
	svc := newLocalSvc(t)
	story, err := svc.CreateStory(context.Background(), 1, "Test Story", "Story description")
	if err != nil {
		t.Fatalf("CreateStory() error = %v", err)
	}
	if story.Title != "Test Story" {
		t.Fatalf("CreateStory().Title = %q, want %q", story.Title, "Test Story")
	}

	stories, err := svc.ListStories(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListStories() error = %v", err)
	}
	if len(stories) != 1 {
		t.Fatalf("ListStories() len = %d, want 1", len(stories))
	}

	got, err := svc.GetStory(context.Background(), story.ID)
	if err != nil {
		t.Fatalf("GetStory() error = %v", err)
	}
	if got.Title != "Test Story" {
		t.Fatalf("GetStory().Title = %q, want %q", got.Title, "Test Story")
	}

	updated, err := svc.UpdateStory(context.Background(), story.ID, "Updated Story", "New desc")
	if err != nil {
		t.Fatalf("UpdateStory() error = %v", err)
	}
	if updated.Title != "Updated Story" {
		t.Fatalf("UpdateStory().Title = %q, want %q", updated.Title, "Updated Story")
	}

	if err := svc.DeleteStory(context.Background(), story.ID); err != nil {
		t.Fatalf("DeleteStory() error = %v", err)
	}
	stories, err = svc.ListStories(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListStories() after delete error = %v", err)
	}
	if len(stories) != 0 {
		t.Fatalf("ListStories() after delete len = %d, want 0", len(stories))
	}
}

func TestLocalServiceListProjectHistory(t *testing.T) {
	svc := newLocalSvc(t)
	_, err := svc.ListProjectHistory(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("ListProjectHistory() error = %v", err)
	}
}
