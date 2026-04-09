// Package libtickethttp implements the libticket.Service interface over HTTP,
// delegating all calls to a running ticket server via the internal/client package.
// Use New to construct a Service from a loaded configuration.
package libtickethttp

import (
	"fmt"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

// Service wraps an HTTP client and satisfies the libticket.Service interface.
// All methods delegate to the underlying client which communicates with the
// ticket server REST API.
type Service struct {
	client *client.Client
}

// New returns a Service configured to talk to the server described by cfg.
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

func (s *Service) SetAgentEnabled(id string, enabled bool) (store.Agent, error) {
	return s.client.SetAgentEnabled(id, enabled)
}

func (s *Service) ListAgents() ([]store.Agent, error) {
	return s.client.ListAgents()
}

func (s *Service) ListAgentStatuses() ([]store.AgentStatus, error) {
	return s.client.ListAgentStatuses()
}

func (s *Service) UpdateAgent(id string, request libticket.AgentUpdateRequest) (store.Agent, error) {
	return s.client.UpdateAgent(id, client.AgentUpdateRequest(request))
}

func (s *Service) DeleteAgent(id string) error {
	return s.client.DeleteAgent(id)
}

func (s *Service) SetAgentConfig(agentID string, key, value string) error {
	return s.client.SetAgentConfig(agentID, key, value)
}

func (s *Service) ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error) {
	return s.client.ListAgentConfig(agentID)
}

func (s *Service) DeleteAgentConfig(agentID string, key string) error {
	return s.client.DeleteAgentConfig(agentID, key)
}

func (s *Service) RegisterAgent(request libticket.AgentRegisterRequest) (store.Agent, error) {
	return s.client.RegisterAgent(client.AgentRegisterRequest(request))
}

func (s *Service) HeartbeatAgent(agentID, password, status string) error {
	return s.client.HeartbeatAgent(agentID, password, status)
}

func (s *Service) RequestAgentWork(request libticket.AgentRequest) (libticket.AgentWorkResponse, error) {
	resp, err := s.client.RequestAgentWork(client.AgentRequest(request))
	if err != nil {
		return libticket.AgentWorkResponse{}, err
	}
	return libticket.AgentWorkResponse(resp), nil
}

func (s *Service) AgentUpdateTicket(id string, request libticket.AgentTicketUpdateRequest) (store.Ticket, error) {
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

func (s *Service) RenameProjectPrefix(id int64, newPrefix string) (int, error) {
	return 0, fmt.Errorf("rename-prefix is not supported in remote mode")
}

func (s *Service) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	return s.client.SetProjectEnabled(id, enabled)
}

func (s *Service) AddProjectMember(projectID int64, request libticket.ProjectMemberRequest) (store.ProjectMember, error) {
	return s.client.AddProjectMember(projectID, client.ProjectMemberRequest(request))
}

func (s *Service) RemoveProjectMember(projectID int64, userID string) error {
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

func (s *Service) RemoveTeamMember(teamID int64, userID string) error {
	return s.client.RemoveTeamMember(teamID, userID)
}

func (s *Service) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	return s.client.ListTeamMembers(teamID)
}

func (s *Service) AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error) {
	return s.client.AddTeamAgent(teamID, agentID)
}

func (s *Service) RemoveTeamAgent(teamID int64, agentID string) error {
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

func (s *Service) UpdateTicket(id string, request libticket.TicketUpdateRequest) (store.Ticket, error) {
	return s.client.UpdateTicket(id, client.TicketUpdateRequest(request))
}

func (s *Service) CloseTicket(id string, message string) (store.Ticket, error) {
	return s.client.CloseTicket(id, message)
}

func (s *Service) OpenTicket(id string, message string) (store.Ticket, error) {
	return s.client.OpenTicket(id, message)
}

func (s *Service) ArchiveTicket(id string, message string) (store.Ticket, error) {
	return s.client.ArchiveTicket(id, message)
}

func (s *Service) UnarchiveTicket(id string, message string) (store.Ticket, error) {
	return s.client.UnarchiveTicket(id, message)
}

func (s *Service) ReadyTicket(id string, message string) (store.Ticket, error) {
	return s.client.ReadyTicket(id, message)
}

func (s *Service) NotReadyTicket(id string, message string) (store.Ticket, error) {
	return s.client.NotReadyTicket(id, message)
}

func (s *Service) SetTicketSdlc(id string, sdlcID int64) (store.Ticket, error) {
	return s.client.SetTicketSdlc(id, sdlcID)
}

func (s *Service) UnsetTicketSdlc(id string) (store.Ticket, error) {
	return s.client.UnsetTicketSdlc(id)
}

func (s *Service) DeleteTicket(id string) error {
	return s.client.DeleteTicket(id)
}

func (s *Service) SetTicketParent(id string, parentID string, message string) (store.Ticket, error) {
	return s.client.SetTicketParent(id, parentID, message)
}

func (s *Service) UnsetTicketParent(id string, message string) (store.Ticket, error) {
	return s.client.UnsetTicketParent(id, message)
}

func (s *Service) GetTicketByID(id string) (store.Ticket, error) {
	return s.client.GetTicketByID(id)
}

func (s *Service) GetTicket(ref string) (store.Ticket, error) {
	return s.client.GetTicket(ref)
}

func (s *Service) CloneTicket(id string, message string) (store.Ticket, error) {
	return s.client.CloneTicket(id, message)
}

func (s *Service) ListHistory(id string) ([]store.HistoryEvent, error) {
	return s.client.ListHistory(id)
}

func (s *Service) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistory(projectID, limit)
}

func (s *Service) ListProjectHistoryFiltered(projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistoryFiltered(projectID, limit, filter)
}

func (s *Service) AddComment(id string, comment string) (store.Comment, error) {
	return s.client.AddComment(id, comment)
}

func (s *Service) ListComments(id string) ([]store.Comment, error) {
	return s.client.ListComments(id)
}

func (s *Service) SetTicketHealth(id string, score int) (store.Ticket, error) {
	return s.client.SetTicketHealth(id, score)
}

func (s *Service) AddDependency(request libticket.DependencyRequest) (store.Dependency, error) {
	return s.client.AddDependency(client.DependencyRequest(request))
}

func (s *Service) RemoveDependency(request libticket.DependencyRequest) error {
	return s.client.RemoveDependency(client.DependencyRequest(request))
}

func (s *Service) ListDependencies(id string) ([]store.Dependency, error) {
	return s.client.ListDependencies(id)
}

func (s *Service) RequestTicket(request libticket.TicketRequest) (libticket.TicketRequestResponse, error) {
	response, err := s.client.RequestTicket(client.TicketRequest(request))
	if err != nil {
		return libticket.TicketRequestResponse{}, err
	}
	return libticket.TicketRequestResponse(response), nil
}

func (s *Service) CreateSdlc(request libticket.SdlcRequest) (store.Sdlc, error) {
	return s.client.CreateSdlc(client.SdlcRequest(request))
}

func (s *Service) ListSdlcs() ([]store.Sdlc, error) {
	return s.client.ListSdlcs()
}

func (s *Service) GetSdlc(id int64) (store.SdlcWithStages, error) {
	return s.client.GetSdlc(id)
}

func (s *Service) DeleteSdlc(id int64) error {
	return s.client.DeleteSdlc(id)
}

func (s *Service) AddSdlcStage(sdlcID int64, request libticket.SdlcStageRequest) (store.SdlcStage, error) {
	return s.client.AddSdlcStage(sdlcID, client.SdlcStageRequest(request))
}

func (s *Service) RemoveSdlcStage(stageID int64) error {
	return s.client.RemoveSdlcStage(stageID)
}

func (s *Service) ReorderSdlcStages(sdlcID int64, stageIDs []int64) error {
	return s.client.ReorderSdlcStages(sdlcID, stageIDs)
}

func (s *Service) ExportSdlc(id int64) (store.SdlcExport, error) {
	return s.client.ExportSdlc(id)
}

func (s *Service) ImportSdlc(export store.SdlcExport) (store.Sdlc, error) {
	return s.client.ImportSdlc(export)
}

func (s *Service) LogTime(ticketID string, request libticket.TimeEntryRequest) (store.TimeEntry, error) {
	return s.client.LogTime(ticketID, request)
}

func (s *Service) ListTimeEntries(ticketID string) ([]store.TimeEntry, error) {
	return s.client.ListTimeEntries(ticketID)
}

func (s *Service) DeleteTimeEntry(id int64) error {
	return s.client.DeleteTimeEntry(id)
}

func (s *Service) TotalTimeForTicket(ticketID string) (int, error) {
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

func (s *Service) AddTicketLabel(ticketID string, labelID int64) error {
	return s.client.AddTicketLabel(ticketID, labelID)
}

func (s *Service) RemoveTicketLabel(ticketID string, labelID int64) error {
	return s.client.RemoveTicketLabel(ticketID, labelID)
}

func (s *Service) ListTicketLabels(ticketID string) ([]store.Label, error) {
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
