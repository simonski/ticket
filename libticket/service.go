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
	CreateAgent(request AgentCreateRequest) (store.Agent, string, error)
	SetAgentEnabled(id int64, enabled bool) (store.Agent, error)
	ListAgents() ([]store.Agent, error)
	UpdateAgent(id int64, request AgentUpdateRequest) (store.Agent, error)
	DeleteAgent(id int64) error
	RegisterAgent(request AgentRegisterRequest) (store.Agent, error)
	RequestAgentWork(request AgentRequest) (AgentWorkResponse, error)
	AgentUpdateTicket(id int64, request AgentTicketUpdateRequest) (store.Ticket, error)
	CreateProject(request ProjectCreateRequest) (store.Project, error)
	ListProjects() ([]store.Project, error)
	GetProject(id string) (store.Project, error)
	UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error)
	SetProjectEnabled(id int64, enabled bool) (store.Project, error)
	AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error)
	RemoveProjectMember(projectID, userID int64) error
	ListProjectMembers(projectID int64) ([]store.ProjectMember, error)
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
	AddComment(id int64, comment string) (store.Comment, error)
	ListComments(id int64) ([]store.Comment, error)
	AddDependency(request DependencyRequest) (store.Dependency, error)
	RemoveDependency(request DependencyRequest) error
	ListDependencies(id int64) ([]store.Dependency, error)
	RequestTicket(request TicketRequest) (TicketRequestResponse, error)
}
