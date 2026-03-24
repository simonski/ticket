package libtickethttp

import (
	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

type Service struct {
	client *client.Client
}

func New(cfg config.Config) *Service {
	return &Service{client: client.New(cfg)}
}

func (s *Service) Status() (libticket.StatusResponse, error) {
	status, err := s.client.Status()
	if err != nil {
		return libticket.StatusResponse{}, err
	}
	return libticket.StatusResponse(status), nil
}

func (s *Service) SetRegistrationEnabled(enabled bool) error {
	return s.client.SetRegistrationEnabled(enabled)
}

func (s *Service) Register(username, password string) (store.User, error) {
	return s.client.Register(username, password)
}

func (s *Service) Login(username, password string) (store.User, string, error) {
	response, err := s.client.Login(username, password)
	if err != nil {
		return store.User{}, "", err
	}
	return response.User, response.Token, nil
}

func (s *Service) Logout() error {
	return s.client.Logout()
}

func (s *Service) Count(projectID *int64) (libticket.CountSummary, error) {
	return s.client.Count(projectID)
}

func (s *Service) CreateUser(username, password string) (store.User, error) {
	return s.client.CreateUser(username, password)
}

func (s *Service) SetUserEnabled(username string, enabled bool) error {
	return s.client.SetUserEnabled(username, enabled)
}

func (s *Service) ListUsers() ([]store.User, error) {
	return s.client.ListUsers()
}

func (s *Service) DeleteUser(username string) error {
	return s.client.DeleteUser(username)
}

func (s *Service) ResetUserPassword(username, newPassword string) (store.User, error) {
	return s.client.ResetUserPassword(username, newPassword)
}

func (s *Service) CreateRole(request libticket.RoleRequest) (store.Role, error) {
	return s.client.CreateRole(client.RoleRequest(request))
}

func (s *Service) ListRoles() ([]store.Role, error) {
	return s.client.ListRoles()
}

func (s *Service) UpdateRole(id int64, request libticket.RoleRequest) (store.Role, error) {
	return s.client.UpdateRole(id, client.RoleRequest(request))
}

func (s *Service) DeleteRole(id int64) error {
	return s.client.DeleteRole(id)
}

func (s *Service) CreateAgent(request libticket.AgentCreateRequest) (store.Agent, string, error) {
	return s.client.CreateAgent(client.AgentCreateRequest(request))
}

func (s *Service) SetAgentEnabled(id int64, enabled bool) (store.Agent, error) {
	return s.client.SetAgentEnabled(id, enabled)
}

func (s *Service) ListAgents() ([]store.Agent, error) {
	return s.client.ListAgents()
}

func (s *Service) ListAgentStatuses() ([]store.AgentStatus, error) {
	return s.client.ListAgentStatuses()
}

func (s *Service) UpdateAgent(id int64, request libticket.AgentUpdateRequest) (store.Agent, error) {
	return s.client.UpdateAgent(id, client.AgentUpdateRequest(request))
}

func (s *Service) DeleteAgent(id int64) error {
	return s.client.DeleteAgent(id)
}

func (s *Service) SetAgentConfig(agentID int64, key, value string) error {
	return s.client.SetAgentConfig(agentID, key, value)
}

func (s *Service) ListAgentConfig(agentID int64) ([]store.AgentConfigEntry, error) {
	return s.client.ListAgentConfig(agentID)
}

func (s *Service) DeleteAgentConfig(agentID int64, key string) error {
	return s.client.DeleteAgentConfig(agentID, key)
}

func (s *Service) RegisterAgent(request libticket.AgentRegisterRequest) (store.Agent, error) {
	return s.client.RegisterAgent(client.AgentRegisterRequest(request))
}

func (s *Service) RequestAgentWork(request libticket.AgentRequest) (libticket.AgentWorkResponse, error) {
	resp, err := s.client.RequestAgentWork(client.AgentRequest(request))
	if err != nil {
		return libticket.AgentWorkResponse{}, err
	}
	return libticket.AgentWorkResponse(resp), nil
}

func (s *Service) AgentUpdateTicket(id int64, request libticket.AgentTicketUpdateRequest) (store.Ticket, error) {
	return s.client.AgentUpdateTicket(id, client.AgentTicketUpdateRequest(request))
}

func (s *Service) CreateProject(request libticket.ProjectCreateRequest) (store.Project, error) {
	return s.client.CreateProject(client.ProjectCreateRequest(request))
}

func (s *Service) ListProjects() ([]store.Project, error) {
	return s.client.ListProjects()
}

func (s *Service) GetProject(id string) (store.Project, error) {
	return s.client.GetProject(id)
}

func (s *Service) UpdateProject(id int64, request libticket.ProjectUpdateRequest) (store.Project, error) {
	return s.client.UpdateProject(id, client.ProjectUpdateRequest(request))
}

func (s *Service) DeleteProject(id int64) error {
	return s.client.DeleteProject(id)
}

func (s *Service) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	return s.client.SetProjectEnabled(id, enabled)
}

func (s *Service) AddProjectMember(projectID int64, request libticket.ProjectMemberRequest) (store.ProjectMember, error) {
	return s.client.AddProjectMember(projectID, client.ProjectMemberRequest(request))
}

func (s *Service) RemoveProjectMember(projectID, userID int64) error {
	return s.client.RemoveProjectMember(projectID, userID)
}

func (s *Service) ListProjectMembers(projectID int64) ([]store.ProjectMember, error) {
	return s.client.ListProjectMembers(projectID)
}

func (s *Service) AddProjectTeamMember(projectID int64, request libticket.ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	return s.client.AddProjectTeamMember(projectID, client.ProjectTeamMemberRequest(request))
}

func (s *Service) RemoveProjectTeamMember(projectID, teamID int64) error {
	return s.client.RemoveProjectTeamMember(projectID, teamID)
}

func (s *Service) ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error) {
	return s.client.ListProjectTeamMembers(projectID)
}

func (s *Service) CreateTeam(request libticket.TeamRequest) (store.Team, error) {
	return s.client.CreateTeam(client.TeamRequest(request))
}

func (s *Service) ListTeams() ([]store.Team, error) {
	return s.client.ListTeams()
}

func (s *Service) UpdateTeam(id int64, request libticket.TeamRequest) (store.Team, error) {
	return s.client.UpdateTeam(id, client.TeamRequest(request))
}

func (s *Service) DeleteTeam(id int64) error {
	return s.client.DeleteTeam(id)
}

func (s *Service) AddTeamMember(teamID int64, request libticket.TeamMemberRequest) (store.TeamMember, error) {
	return s.client.AddTeamMember(teamID, client.TeamMemberRequest(request))
}

func (s *Service) RemoveTeamMember(teamID, userID int64) error {
	return s.client.RemoveTeamMember(teamID, userID)
}

func (s *Service) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	return s.client.ListTeamMembers(teamID)
}

func (s *Service) AddTeamAgent(teamID, agentID int64) (store.TeamAgent, error) {
	return s.client.AddTeamAgent(teamID, agentID)
}

func (s *Service) RemoveTeamAgent(teamID, agentID int64) error {
	return s.client.RemoveTeamAgent(teamID, agentID)
}

func (s *Service) ListTeamAgents(teamID int64) ([]store.TeamAgent, error) {
	return s.client.ListTeamAgents(teamID)
}

func (s *Service) CreateTicket(request libticket.TicketCreateRequest) (store.Ticket, error) {
	return s.client.CreateTicket(client.TicketCreateRequest(request))
}

func (s *Service) ListTickets(projectID int64) ([]store.Ticket, error) {
	return s.client.ListTickets(projectID)
}

func (s *Service) ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	return s.client.ListTicketsFiltered(projectID, taskType, stage, state, status, search, assignee, limit, includeArchived)
}

func (s *Service) UpdateTicket(id int64, request libticket.TicketUpdateRequest) (store.Ticket, error) {
	return s.client.UpdateTicket(id, client.TicketUpdateRequest(request))
}

func (s *Service) CloseTicket(id int64) (store.Ticket, error) {
	return s.client.CloseTicket(id)
}

func (s *Service) OpenTicket(id int64) (store.Ticket, error) {
	return s.client.OpenTicket(id)
}

func (s *Service) ArchiveTicket(id int64) (store.Ticket, error) {
	return s.client.ArchiveTicket(id)
}

func (s *Service) UnarchiveTicket(id int64) (store.Ticket, error) {
	return s.client.UnarchiveTicket(id)
}

func (s *Service) ReadyTicket(id int64) (store.Ticket, error) {
	return s.client.ReadyTicket(id)
}

func (s *Service) NotReadyTicket(id int64) (store.Ticket, error) {
	return s.client.NotReadyTicket(id)
}

func (s *Service) DeleteTicket(id int64) error {
	return s.client.DeleteTicket(id)
}

func (s *Service) SetTicketParent(id, parentID int64) (store.Ticket, error) {
	return s.client.SetTicketParent(id, parentID)
}

func (s *Service) UnsetTicketParent(id int64) (store.Ticket, error) {
	return s.client.UnsetTicketParent(id)
}

func (s *Service) GetTicketByID(id int64) (store.Ticket, error) {
	return s.client.GetTicketByID(id)
}

func (s *Service) GetTicket(ref string) (store.Ticket, error) {
	return s.client.GetTicket(ref)
}

func (s *Service) CloneTicket(id int64) (store.Ticket, error) {
	return s.client.CloneTicket(id)
}

func (s *Service) ListHistory(id int64) ([]store.HistoryEvent, error) {
	return s.client.ListHistory(id)
}

func (s *Service) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistory(projectID, limit)
}

func (s *Service) AddComment(id int64, comment string) (store.Comment, error) {
	return s.client.AddComment(id, comment)
}

func (s *Service) ListComments(id int64) ([]store.Comment, error) {
	return s.client.ListComments(id)
}

func (s *Service) SetTicketHealth(id int64, score int) (store.Ticket, error) {
	return s.client.SetTicketHealth(id, score)
}

func (s *Service) AddDependency(request libticket.DependencyRequest) (store.Dependency, error) {
	return s.client.AddDependency(client.DependencyRequest(request))
}

func (s *Service) RemoveDependency(request libticket.DependencyRequest) error {
	return s.client.RemoveDependency(client.DependencyRequest(request))
}

func (s *Service) ListDependencies(id int64) ([]store.Dependency, error) {
	return s.client.ListDependencies(id)
}

func (s *Service) RequestTicket(request libticket.TicketRequest) (libticket.TicketRequestResponse, error) {
	response, err := s.client.RequestTicket(client.TicketRequest(request))
	if err != nil {
		return libticket.TicketRequestResponse{}, err
	}
	return libticket.TicketRequestResponse(response), nil
}

func (s *Service) CreateWorkflow(request libticket.WorkflowRequest) (store.Workflow, error) {
	return s.client.CreateWorkflow(client.WorkflowRequest(request))
}

func (s *Service) ListWorkflows() ([]store.Workflow, error) {
	return s.client.ListWorkflows()
}

func (s *Service) GetWorkflow(id int64) (store.WorkflowWithStages, error) {
	return s.client.GetWorkflow(id)
}

func (s *Service) DeleteWorkflow(id int64) error {
	return s.client.DeleteWorkflow(id)
}

func (s *Service) AddWorkflowStage(workflowID int64, request libticket.WorkflowStageRequest) (store.WorkflowStage, error) {
	return s.client.AddWorkflowStage(workflowID, client.WorkflowStageRequest(request))
}

func (s *Service) RemoveWorkflowStage(stageID int64) error {
	return s.client.RemoveWorkflowStage(stageID)
}

func (s *Service) ReorderWorkflowStages(workflowID int64, stageIDs []int64) error {
	return s.client.ReorderWorkflowStages(workflowID, stageIDs)
}

func (s *Service) ExportWorkflow(id int64) (store.WorkflowExport, error) {
	return s.client.ExportWorkflow(id)
}

func (s *Service) ImportWorkflow(export store.WorkflowExport) (store.Workflow, error) {
	return s.client.ImportWorkflow(export)
}

func (s *Service) LogTime(ticketID int64, request libticket.TimeEntryRequest) (store.TimeEntry, error) {
	return s.client.LogTime(ticketID, request)
}

func (s *Service) ListTimeEntries(ticketID int64) ([]store.TimeEntry, error) {
	return s.client.ListTimeEntries(ticketID)
}

func (s *Service) DeleteTimeEntry(id int64) error {
	return s.client.DeleteTimeEntry(id)
}

func (s *Service) TotalTimeForTicket(ticketID int64) (int, error) {
	return s.client.TotalTimeForTicket(ticketID)
}

func (s *Service) CreateLabel(projectID int64, request libticket.LabelRequest) (store.Label, error) {
	return s.client.CreateLabel(projectID, request)
}

func (s *Service) ListLabels(projectID int64) ([]store.Label, error) {
	return s.client.ListLabels(projectID)
}

func (s *Service) DeleteLabel(id int64) error {
	return s.client.DeleteLabel(id)
}

func (s *Service) AddTicketLabel(ticketID, labelID int64) error {
	return s.client.AddTicketLabel(ticketID, labelID)
}

func (s *Service) RemoveTicketLabel(ticketID, labelID int64) error {
	return s.client.RemoveTicketLabel(ticketID, labelID)
}

func (s *Service) ListTicketLabels(ticketID int64) ([]store.Label, error) {
	return s.client.ListTicketLabels(ticketID)
}

func (s *Service) CreateStory(projectID int64, title, description string) (store.Story, error) {
	return s.client.CreateStory(projectID, title, description)
}

func (s *Service) ListStories(projectID int64) ([]store.Story, error) {
	return s.client.ListStories(projectID)
}

func (s *Service) GetStory(id int64) (store.Story, error) {
	return s.client.GetStory(id)
}

func (s *Service) UpdateStory(id int64, title, description string) (store.Story, error) {
	return s.client.UpdateStory(id, title, description)
}

func (s *Service) DeleteStory(id int64) error {
	return s.client.DeleteStory(id)
}
