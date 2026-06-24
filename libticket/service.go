// Package libticket provides the core service interface and LocalService implementation
// for interacting with ticket data. Both local (SQLite) and remote (HTTP) implementations
// satisfy the Service interface, enabling identical behaviour regardless of deployment mode.
package libticket

import (
	"context"

	"github.com/simonski/ticket/internal/store"
)

type RegisterParams struct {
	Username string
	Password string
	Email    string
}

type UserCreateParams struct {
	Username string
	Password string
	Email    string
	Role     string
	PlanSlug string
	Enabled  *bool
}

// AuthService covers user registration, login, logout, and session management.
type AuthService interface {
	Status(ctx context.Context) (StatusResponse, error)
	SetRegistrationEnabled(ctx context.Context, enabled bool) error
	SetRegistrationAutoApprove(ctx context.Context, enabled bool) error
	GetEmailConfig(ctx context.Context) (store.EmailConfig, error)
	SetEmailConfig(ctx context.Context, cfg store.EmailConfig, updatePassword bool) error
	SetEmailEnabled(ctx context.Context, enabled bool) error
	ListPlans(ctx context.Context) ([]store.Plan, error)
	DefaultPlan(ctx context.Context) (store.Plan, error)
	SetDefaultPlan(ctx context.Context, slug string) error
	Register(ctx context.Context, username, password string) (store.User, error)
	Login(ctx context.Context, username, password string) (store.User, string, error)
	Logout(ctx context.Context) error
}

// UserService covers user and role management.
type UserService interface {
	Count(ctx context.Context, projectID *int64) (CountSummary, error)
	CreateUser(ctx context.Context, username, password string) (store.User, error)
	SetUserEnabled(ctx context.Context, username string, enabled bool) error
	ListUsers(ctx context.Context) ([]store.User, error)
	GetMyDefaultProject(ctx context.Context) (store.Project, error)
	SetMyDefaultProject(ctx context.Context, projectRef string) (store.Project, error)
	ClearMyDefaultProject(ctx context.Context) error
	ListMyNotifications(ctx context.Context, status string, limit int) ([]store.UserNotification, error)
	MarkNotificationRead(ctx context.Context, notificationID int64) (store.UserNotification, error)
	DeleteUser(ctx context.Context, username string) error
	ResetUserPassword(ctx context.Context, username, newPassword string) (store.User, error)
	CreateRole(ctx context.Context, request RoleRequest) (store.Role, error)
	ListRoles(ctx context.Context) ([]store.Role, error)
	UpdateRole(ctx context.Context, id int64, request RoleRequest) (store.Role, error)
	DeleteRole(ctx context.Context, id int64) error
}

// AgentService covers AI agent lifecycle, configuration, and work assignment.
type AgentService interface {
	CreateAgent(ctx context.Context, request AgentCreateRequest) (store.Agent, string, error)
	SetAgentEnabled(ctx context.Context, id string, enabled bool) (store.Agent, error)
	ListAgents(ctx context.Context) ([]store.Agent, error)
	ListAgentStatuses(ctx context.Context) ([]store.AgentStatus, error)
	UpdateAgent(ctx context.Context, id string, request AgentUpdateRequest) (store.Agent, error)
	DeleteAgent(ctx context.Context, id string) error
	SetAgentConfig(ctx context.Context, agentID string, key, value string) error
	ListAgentConfig(ctx context.Context, agentID string) ([]store.AgentConfigEntry, error)
	DeleteAgentConfig(ctx context.Context, agentID string, key string) error
	RegisterAgent(ctx context.Context, request AgentRegisterRequest) (store.Agent, error)
	HeartbeatAgent(ctx context.Context, agentID, password, status string) error
	RequestAgentWork(ctx context.Context, request AgentRequest) (AgentWorkResponse, error)
	AgentUpdateTicket(ctx context.Context, id string, request AgentTicketUpdateRequest) (store.Ticket, error)
	AgentRecommendReady(ctx context.Context, agentID, password, ticketID string) (store.Ticket, error)
	AgentRefineTicket(ctx context.Context, id string, request AgentRefineRequest) (store.Ticket, error)
}

// ProjectService covers project CRUD, membership, and team association.
type ProjectService interface {
	CreateProject(ctx context.Context, request ProjectCreateRequest) (store.Project, error)
	ListProjects(ctx context.Context) ([]store.Project, error)
	GetProject(ctx context.Context, id string) (store.Project, error)
	FindProjectByGitRepository(ctx context.Context, repository string) (store.Project, error)
	CreateProjectAccessRequest(ctx context.Context, projectRef, message string) (store.ProjectAccessRequest, error)
	ListProjectAccessRequests(ctx context.Context, projectRef, status string) ([]store.ProjectAccessRequest, error)
	ListMyProjectAccessRequests(ctx context.Context, status string) ([]store.ProjectAccessRequest, error)
	SetProjectAccessRequestStatus(ctx context.Context, projectRef string, requestID int64, status, message string) (store.ProjectAccessRequest, error)
	UpdateProject(ctx context.Context, id int64, request ProjectUpdateRequest) (store.Project, error)
	DeleteProject(ctx context.Context, id int64) error
	RenameProjectPrefix(ctx context.Context, id int64, newPrefix string) (int, error)
	SetProjectEnabled(ctx context.Context, id int64, enabled bool) (store.Project, error)
	SetProjectDefaultDraft(ctx context.Context, projectID int64, draft bool) error
	ListProjectGitRepositories(ctx context.Context, projectRef string) ([]string, error)
	AddProjectGitRepository(ctx context.Context, projectRef, repository string) error
	RemoveProjectGitRepository(ctx context.Context, projectRef, repository string) error
	AddProjectMember(ctx context.Context, projectID int64, request ProjectMemberRequest) (store.ProjectMember, error)
	RemoveProjectMember(ctx context.Context, projectID int64, userID string) error
	ListProjectMembers(ctx context.Context, projectID int64) ([]store.ProjectMember, error)
	AddProjectTeamMember(ctx context.Context, projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error)
	RemoveProjectTeamMember(ctx context.Context, projectID, teamID int64) error
	ListProjectTeamMembers(ctx context.Context, projectID int64) ([]store.ProjectTeamMember, error)
}

// TeamService covers team management and membership.
type TeamService interface {
	CreateTeam(ctx context.Context, request TeamRequest) (store.Team, error)
	ListTeams(ctx context.Context) ([]store.Team, error)
	UpdateTeam(ctx context.Context, id int64, request TeamRequest) (store.Team, error)
	DeleteTeam(ctx context.Context, id int64) error
	AddTeamMember(ctx context.Context, teamID int64, request TeamMemberRequest) (store.TeamMember, error)
	RemoveTeamMember(ctx context.Context, teamID int64, userID string) error
	ListTeamMembers(ctx context.Context, teamID int64) ([]store.TeamMember, error)
	AddTeamAgent(ctx context.Context, teamID int64, agentID string) (store.TeamAgent, error)
	RemoveTeamAgent(ctx context.Context, teamID int64, agentID string) error
	ListTeamAgents(ctx context.Context, teamID int64) ([]store.TeamAgent, error)
}

// WorkflowService covers workflow templates, stage management, and stage-role assignments.
type WorkflowService interface {
	CreateWorkflow(ctx context.Context, request WorkflowRequest) (store.Workflow, error)
	ListWorkflows(ctx context.Context) ([]store.Workflow, error)
	GetWorkflow(ctx context.Context, id int64) (store.WorkflowWithStages, error)
	DeleteWorkflow(ctx context.Context, id int64) error
	AddWorkflowStage(ctx context.Context, workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error)
	UpdateWorkflowStage(ctx context.Context, stageID int64, request WorkflowStageRequest) (store.WorkflowStage, error)
	GetWorkflowStage(ctx context.Context, stageID int64) (store.WorkflowStage, error)
	ListWorkflowStages(ctx context.Context, workflowID int64) ([]store.WorkflowStage, error)
	RemoveWorkflowStage(ctx context.Context, stageID int64) error
	ReorderWorkflowStages(ctx context.Context, workflowID int64, stageIDs []int64) error
	ExportWorkflow(ctx context.Context, id int64) (store.WorkflowExport, error)
	ImportWorkflow(ctx context.Context, export store.WorkflowExport) (store.Workflow, error)
	AddWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error
	RemoveWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error
	ReorderWorkflowStageRoles(ctx context.Context, workflowID, stageID int64, roleIDs []int64) error
}

// TicketService covers ticket CRUD, lifecycle, labels, time, dependencies, and history.
type TicketService interface {
	CreateLabel(ctx context.Context, projectID int64, request LabelRequest) (store.Label, error)
	ListLabels(ctx context.Context, projectID int64) ([]store.Label, error)
	DeleteLabel(ctx context.Context, id int64) error
	AddTicketLabel(ctx context.Context, ticketID string, labelID int64) error
	RemoveTicketLabel(ctx context.Context, ticketID string, labelID int64) error
	ListTicketLabels(ctx context.Context, ticketID string) ([]store.Label, error)
	LogTime(ctx context.Context, ticketID string, request TimeEntryRequest) (store.TimeEntry, error)
	ListTimeEntries(ctx context.Context, ticketID string) ([]store.TimeEntry, error)
	DeleteTimeEntry(ctx context.Context, id int64) error
	TotalTimeForTicket(ctx context.Context, ticketID string) (int, error)
	CreateTicket(ctx context.Context, request TicketCreateRequest) (store.Ticket, error)
	ListTickets(ctx context.Context, projectID int64) ([]store.Ticket, error)
	ListTicketsFiltered(ctx context.Context, projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error)
	UpdateTicket(ctx context.Context, id string, request TicketUpdateRequest) (store.Ticket, error)
	ImportTicketMarkdown(ctx context.Context, request TicketMarkdownImportRequest) (store.Ticket, error)
	CloseTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	OpenTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	CompleteTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	ReopenTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	ArchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	UnarchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	ReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	NotReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	DraftTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	UndraftTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	NextTicket(ctx context.Context, id string) (store.Ticket, error)
	PreviousTicket(ctx context.Context, id string) (store.Ticket, error)
	SetTicketWorkflow(ctx context.Context, id string, workflowID int64) (store.Ticket, error)
	UnsetTicketWorkflow(ctx context.Context, id string) (store.Ticket, error)
	DeleteTicket(ctx context.Context, id string) error
	SetTicketParent(ctx context.Context, id string, parentID string, message string) (store.Ticket, error)
	UnsetTicketParent(ctx context.Context, id string, message string) (store.Ticket, error)
	CreatePullRequest(ctx context.Context, request PullRequestRequest) (store.PullRequest, error)
	GetPullRequest(ctx context.Context, id int64) (store.PullRequest, error)
	ListPullRequestsByTicket(ctx context.Context, ticketID string) ([]store.PullRequest, error)
	ListPullRequestsByProject(ctx context.Context, projectRef string) ([]store.PullRequest, error)
	SetPullRequestStatus(ctx context.Context, id int64, status string) (store.PullRequest, error)
	SetTicketHealth(ctx context.Context, id string, score int) (store.Ticket, error)
	GetTicketByID(ctx context.Context, id string) (store.Ticket, error)
	GetTicket(ctx context.Context, ref string) (store.Ticket, error)
	CloneTicket(ctx context.Context, id string, message string) (store.Ticket, error)
	ListHistory(ctx context.Context, id string) ([]store.HistoryEvent, error)
	ListHistoryPaged(ctx context.Context, id string, limit, offset int) ([]store.HistoryEvent, error)
	ListProjectHistory(ctx context.Context, projectID int64, limit int) ([]store.HistoryEvent, error)
	ListProjectHistoryFiltered(ctx context.Context, projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error)
	AddComment(ctx context.Context, id string, comment string) (store.Comment, error)
	ListComments(ctx context.Context, id string) ([]store.Comment, error)
	AddDependency(ctx context.Context, request DependencyRequest) (store.Dependency, error)
	RemoveDependency(ctx context.Context, request DependencyRequest) error
	ListDependencies(ctx context.Context, id string) ([]store.Dependency, error)
	RequestTicket(ctx context.Context, request TicketRequest) (TicketRequestResponse, error)
	InterveneTicket(ctx context.Context, id string, request InterventionRequest) (InterventionResponse, error)
	CreateStoryWithRequest(ctx context.Context, request StoryCreateRequest) (store.Story, error)
	CreateStory(ctx context.Context, projectID int64, title, description string) (store.Story, error)
	ListStories(ctx context.Context, projectID int64) ([]store.Story, error)
	GetStory(ctx context.Context, id int64) (store.Story, error)
	UpdateStory(ctx context.Context, id int64, title, description string) (store.Story, error)
	DeleteStory(ctx context.Context, id int64) error
	ListReleases(ctx context.Context, projectID int64) ([]store.Release, error)
	GetRelease(ctx context.Context, id int64) (store.Release, error)
	CreateRelease(ctx context.Context, projectID int64, title, purpose, targetDate string) (store.Release, error)
	UpdateRelease(ctx context.Context, id int64, title, purpose, targetDate string) (store.Release, error)
	SetReleaseStatus(ctx context.Context, id int64, status string) (store.Release, error)
	DeleteRelease(ctx context.Context, id int64) error
	AddFeatureToRelease(ctx context.Context, featureTicketID string, releaseID int64) error
	RemoveFeatureFromRelease(ctx context.Context, featureTicketID string) error
	CloneFeature(ctx context.Context, featureTicketID string) (store.Ticket, error)
	CreateDocument(ctx context.Context, projectID int64, request DocumentRequest) (store.Document, error)
	ListDocuments(ctx context.Context, projectID int64) ([]store.Document, error)
	GetDocument(ctx context.Context, id int64) (store.Document, error)
	UpdateDocument(ctx context.Context, id int64, request DocumentRequest) (store.Document, error)
	DeleteDocument(ctx context.Context, id int64) error
	AddDocumentLabel(ctx context.Context, documentID int64, request DocumentLabelRequest) error
	RemoveDocumentLabel(ctx context.Context, documentID, labelID int64) error
	ListDocumentLabels(ctx context.Context, documentID int64) ([]store.Label, error)
	AddDocumentFile(ctx context.Context, documentID int64, request DocumentFileUploadRequest) (store.DocumentFile, error)
	ListDocumentFiles(ctx context.Context, documentID int64) ([]store.DocumentFile, error)
	GetDocumentFile(ctx context.Context, documentID, fileID int64) (store.DocumentFile, error)
	DeleteDocumentFile(ctx context.Context, documentID, fileID int64) error
}

// Service defines all ticket management operations. It is implemented by
// LocalService (direct SQLite access) and HTTPService (HTTP client).
// It composes all sub-interfaces for convenient use as a single dependency.
type Service interface {
	AuthService
	UserService
	AgentService
	ProjectService
	TeamService
	WorkflowService
	WorkflowService
	TicketService
}
