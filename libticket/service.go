package libticket

import "github.com/simonski/ticket/internal/store"

type Service interface {
	Status() (StatusResponse, error)
	SetRegistrationEnabled(enabled bool) error
	Register(username, password string) (store.User, error)
	Login(username, password string) (store.User, string, error)
	Logout() error
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
	CreateAgent(request AgentCreateRequest) (store.Agent, string, error)
	SetAgentEnabled(id int64, enabled bool) (store.Agent, error)
	ListAgents() ([]store.Agent, error)
	UpdateAgent(id int64, request AgentUpdateRequest) (store.Agent, error)
	DeleteAgent(id int64) error
	SetAgentConfig(agentID int64, key, value string) error
	ListAgentConfig(agentID int64) ([]store.AgentConfigEntry, error)
	DeleteAgentConfig(agentID int64, key string) error
	RegisterAgent(request AgentRegisterRequest) (store.Agent, error)
	RequestAgentWork(request AgentRequest) (AgentWorkResponse, error)
	AgentUpdateTicket(id int64, request AgentTicketUpdateRequest) (store.Ticket, error)
	CreateProject(request ProjectCreateRequest) (store.Project, error)
	ListProjects() ([]store.Project, error)
	GetProject(id string) (store.Project, error)
	UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error)
	DeleteProject(id int64) error
	SetProjectEnabled(id int64, enabled bool) (store.Project, error)
	AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error)
	RemoveProjectMember(projectID, userID int64) error
	ListProjectMembers(projectID int64) ([]store.ProjectMember, error)
	AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error)
	RemoveProjectTeamMember(projectID, teamID int64) error
	ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error)
	CreateTeam(request TeamRequest) (store.Team, error)
	ListTeams() ([]store.Team, error)
	UpdateTeam(id int64, request TeamRequest) (store.Team, error)
	DeleteTeam(id int64) error
	AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error)
	RemoveTeamMember(teamID, userID int64) error
	ListTeamMembers(teamID int64) ([]store.TeamMember, error)
	AddTeamAgent(teamID, agentID int64) (store.TeamAgent, error)
	RemoveTeamAgent(teamID, agentID int64) error
	ListTeamAgents(teamID int64) ([]store.TeamAgent, error)
	CreateWorkflow(request WorkflowRequest) (store.Workflow, error)
	ListWorkflows() ([]store.Workflow, error)
	GetWorkflow(id int64) (store.WorkflowWithStages, error)
	DeleteWorkflow(id int64) error
	AddWorkflowStage(workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error)
	RemoveWorkflowStage(stageID int64) error
	ReorderWorkflowStages(workflowID int64, stageIDs []int64) error
	ExportWorkflow(id int64) (store.WorkflowExport, error)
	ImportWorkflow(export store.WorkflowExport) (store.Workflow, error)
	CreateLabel(projectID int64, request LabelRequest) (store.Label, error)
	ListLabels(projectID int64) ([]store.Label, error)
	DeleteLabel(id int64) error
	AddTicketLabel(ticketID, labelID int64) error
	RemoveTicketLabel(ticketID, labelID int64) error
	ListTicketLabels(ticketID int64) ([]store.Label, error)
	LogTime(ticketID int64, request TimeEntryRequest) (store.TimeEntry, error)
	ListTimeEntries(ticketID int64) ([]store.TimeEntry, error)
	DeleteTimeEntry(id int64) error
	TotalTimeForTicket(ticketID int64) (int, error)
	CreateTicket(request TicketCreateRequest) (store.Ticket, error)
	ListTickets(projectID int64) ([]store.Ticket, error)
	ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error)
	UpdateTicket(id int64, request TicketUpdateRequest) (store.Ticket, error)
	CloseTicket(id int64) (store.Ticket, error)
	OpenTicket(id int64) (store.Ticket, error)
	ArchiveTicket(id int64) (store.Ticket, error)
	UnarchiveTicket(id int64) (store.Ticket, error)
	DeleteTicket(id int64) error
	SetTicketParent(id, parentID int64) (store.Ticket, error)
	UnsetTicketParent(id int64) (store.Ticket, error)
	SetTicketHealth(id int64, score int) (store.Ticket, error)
	GetTicketByID(id int64) (store.Ticket, error)
	GetTicket(ref string) (store.Ticket, error)
	CloneTicket(id int64) (store.Ticket, error)
	ListHistory(id int64) ([]store.HistoryEvent, error)
	ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error)
	AddComment(id int64, comment string) (store.Comment, error)
	ListComments(id int64) ([]store.Comment, error)
	AddDependency(request DependencyRequest) (store.Dependency, error)
	RemoveDependency(request DependencyRequest) error
	ListDependencies(id int64) ([]store.Dependency, error)
	RequestTicket(request TicketRequest) (TicketRequestResponse, error)
	CreateStory(projectID int64, title, description string) (store.Story, error)
	ListStories(projectID int64) ([]store.Story, error)
	GetStory(id int64) (store.Story, error)
	UpdateStory(id int64, title, description string) (store.Story, error)
	DeleteStory(id int64) error
}
