// Package libticket provides the core service interface and LocalService implementation
// for interacting with ticket data. Both local (SQLite) and remote (HTTP) implementations
// satisfy the Service interface, enabling identical behaviour regardless of deployment mode.
package libticket

import "github.com/simonski/ticket/internal/store"

// AuthService covers user registration, login, logout, and session management.
type AuthService interface {
	Status() (StatusResponse, error)
	SetRegistrationEnabled(enabled bool) error
	Register(username, password string) (store.User, error)
	Login(username, password string) (store.User, string, error)
	Logout() error
}

// UserService covers user and role management.
type UserService interface {
	Count(projectID *int64) (CountSummary, error)
	CreateUser(username, password string) (store.User, error)
	SetUserEnabled(username string, enabled bool) error
	ListUsers() ([]store.User, error)
	DeleteUser(username string) error
	ResetUserPassword(username, newPassword string) (store.User, error)
	CreateRole(request RoleRequest) (store.Role, error)
	ListRoles() ([]store.Role, error)
	UpdateRole(id int64, request RoleRequest) (store.Role, error)
	DeleteRole(id int64) error
}

// AgentService covers AI agent lifecycle, configuration, and work assignment.
type AgentService interface {
	CreateAgent(request AgentCreateRequest) (store.Agent, string, error)
	SetAgentEnabled(id string, enabled bool) (store.Agent, error)
	ListAgents() ([]store.Agent, error)
	ListAgentStatuses() ([]store.AgentStatus, error)
	UpdateAgent(id string, request AgentUpdateRequest) (store.Agent, error)
	DeleteAgent(id string) error
	SetAgentConfig(agentID string, key, value string) error
	ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error)
	DeleteAgentConfig(agentID string, key string) error
	RegisterAgent(request AgentRegisterRequest) (store.Agent, error)
	HeartbeatAgent(agentID, password, status string) error
	RequestAgentWork(request AgentRequest) (AgentWorkResponse, error)
	AgentUpdateTicket(id string, request AgentTicketUpdateRequest) (store.Ticket, error)
}

// ProjectService covers project CRUD, membership, and team association.
type ProjectService interface {
	CreateProject(request ProjectCreateRequest) (store.Project, error)
	ListProjects() ([]store.Project, error)
	GetProject(id string) (store.Project, error)
	UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error)
	DeleteProject(id int64) error
	RenameProjectPrefix(id int64, newPrefix string) (int, error)
	SetProjectEnabled(id int64, enabled bool) (store.Project, error)
	AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error)
	RemoveProjectMember(projectID int64, userID string) error
	ListProjectMembers(projectID int64) ([]store.ProjectMember, error)
	AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error)
	RemoveProjectTeamMember(projectID, teamID int64) error
	ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error)
}

// TeamService covers team management and membership.
type TeamService interface {
	CreateTeam(request TeamRequest) (store.Team, error)
	ListTeams() ([]store.Team, error)
	UpdateTeam(id int64, request TeamRequest) (store.Team, error)
	DeleteTeam(id int64) error
	AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error)
	RemoveTeamMember(teamID int64, userID string) error
	ListTeamMembers(teamID int64) ([]store.TeamMember, error)
	AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error)
	RemoveTeamAgent(teamID int64, agentID string) error
	ListTeamAgents(teamID int64) ([]store.TeamAgent, error)
}

// SdlcService covers sdlc templates and stage management.
type SdlcService interface {
	CreateSdlc(request SdlcRequest) (store.Sdlc, error)
	ListSdlcs() ([]store.Sdlc, error)
	GetSdlc(id int64) (store.SdlcWithStages, error)
	DeleteSdlc(id int64) error
	AddSdlcStage(sdlcID int64, request SdlcStageRequest) (store.SdlcStage, error)
	RemoveSdlcStage(stageID int64) error
	ReorderSdlcStages(sdlcID int64, stageIDs []int64) error
	ExportSdlc(id int64) (store.SdlcExport, error)
	ImportSdlc(export store.SdlcExport) (store.Sdlc, error)
}

// TicketService covers ticket CRUD, lifecycle, labels, time, dependencies, and history.
type TicketService interface {
	CreateLabel(projectID int64, request LabelRequest) (store.Label, error)
	ListLabels(projectID int64) ([]store.Label, error)
	DeleteLabel(id int64) error
	AddTicketLabel(ticketID string, labelID int64) error
	RemoveTicketLabel(ticketID string, labelID int64) error
	ListTicketLabels(ticketID string) ([]store.Label, error)
	LogTime(ticketID string, request TimeEntryRequest) (store.TimeEntry, error)
	ListTimeEntries(ticketID string) ([]store.TimeEntry, error)
	DeleteTimeEntry(id int64) error
	TotalTimeForTicket(ticketID string) (int, error)
	CreateTicket(request TicketCreateRequest) (store.Ticket, error)
	ListTickets(projectID int64) ([]store.Ticket, error)
	ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error)
	UpdateTicket(id string, request TicketUpdateRequest) (store.Ticket, error)
	CloseTicket(id string, message string) (store.Ticket, error)
	OpenTicket(id string, message string) (store.Ticket, error)
	ArchiveTicket(id string, message string) (store.Ticket, error)
	UnarchiveTicket(id string, message string) (store.Ticket, error)
	ReadyTicket(id string, message string) (store.Ticket, error)
	NotReadyTicket(id string, message string) (store.Ticket, error)
	SetTicketSdlc(id string, sdlcID int64) (store.Ticket, error)
	UnsetTicketSdlc(id string) (store.Ticket, error)
	DeleteTicket(id string) error
	SetTicketParent(id string, parentID string, message string) (store.Ticket, error)
	UnsetTicketParent(id string, message string) (store.Ticket, error)
	SetTicketHealth(id string, score int) (store.Ticket, error)
	GetTicketByID(id string) (store.Ticket, error)
	GetTicket(ref string) (store.Ticket, error)
	CloneTicket(id string, message string) (store.Ticket, error)
	ListHistory(id string) ([]store.HistoryEvent, error)
	ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error)
	ListProjectHistoryFiltered(projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error)
	AddComment(id string, comment string) (store.Comment, error)
	ListComments(id string) ([]store.Comment, error)
	AddDependency(request DependencyRequest) (store.Dependency, error)
	RemoveDependency(request DependencyRequest) error
	ListDependencies(id string) ([]store.Dependency, error)
	RequestTicket(request TicketRequest) (TicketRequestResponse, error)
	CreateStory(projectID int64, title, description string) (store.Story, error)
	ListStories(projectID int64) ([]store.Story, error)
	GetStory(id int64) (store.Story, error)
	UpdateStory(id int64, title, description string) (store.Story, error)
	DeleteStory(id int64) error
}

// Service defines all ticket management operations. It is implemented by
// LocalService (direct SQLite access) and libtickethttp.Service (HTTP client).
// It composes all sub-interfaces for convenient use as a single dependency.
type Service interface {
	AuthService
	UserService
	AgentService
	ProjectService
	TeamService
	SdlcService
	TicketService
}
