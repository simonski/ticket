// http.go implements the Service interface over HTTP, delegating to a running
// ticket server via the internal/client package.
package libticket

import (
	"fmt"

	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

// HTTPService implements the Service interface by delegating all calls to a
// remote ticket server over HTTP.
type HTTPService struct {
	client *client.Client
}

// NewHTTP creates a new HTTPService that connects to the ticket server
// specified in the config.
func NewHTTP(cfg config.Config) *HTTPService {
	return &HTTPService{client: client.New(cfg)}
}

func (s *HTTPService) Status() (StatusResponse, error) {
	status, err := s.client.Status()
	if err != nil {
		return StatusResponse{}, err
	}
	return StatusResponse(status), nil
}

func (s *HTTPService) SetRegistrationEnabled(enabled bool) error {
	return s.client.SetRegistrationEnabled(enabled)
}

func (s *HTTPService) Register(username, password string) (store.User, error) {
	return s.client.Register(username, password)
}

func (s *HTTPService) Login(username, password string) (store.User, string, error) {
	response, err := s.client.Login(username, password)
	if err != nil {
		return store.User{}, "", err
	}
	return response.User, response.Token, nil
}

func (s *HTTPService) Logout() error {
	return s.client.Logout()
}

func (s *HTTPService) Count(projectID *int64) (CountSummary, error) {
	return s.client.Count(projectID)
}

func (s *HTTPService) CreateUser(username, password string) (store.User, error) {
	return s.client.CreateUser(username, password)
}

func (s *HTTPService) SetUserEnabled(username string, enabled bool) error {
	return s.client.SetUserEnabled(username, enabled)
}

func (s *HTTPService) ListUsers() ([]store.User, error) {
	return s.client.ListUsers()
}

func (s *HTTPService) DeleteUser(username string) error {
	return s.client.DeleteUser(username)
}

func (s *HTTPService) ResetUserPassword(username, newPassword string) (store.User, error) {
	return s.client.ResetUserPassword(username, newPassword)
}

func (s *HTTPService) CreateRole(request RoleRequest) (store.Role, error) {
	return s.client.CreateRole(client.RoleRequest(request))
}

func (s *HTTPService) ListRoles() ([]store.Role, error) {
	return s.client.ListRoles()
}

func (s *HTTPService) UpdateRole(id int64, request RoleRequest) (store.Role, error) {
	return s.client.UpdateRole(id, client.RoleRequest(request))
}

func (s *HTTPService) DeleteRole(id int64) error {
	return s.client.DeleteRole(id)
}

func (s *HTTPService) CreateAgent(request AgentCreateRequest) (store.Agent, string, error) {
	return s.client.CreateAgent(client.AgentCreateRequest(request))
}

func (s *HTTPService) SetAgentEnabled(id string, enabled bool) (store.Agent, error) {
	return s.client.SetAgentEnabled(id, enabled)
}

func (s *HTTPService) ListAgents() ([]store.Agent, error) {
	return s.client.ListAgents()
}

func (s *HTTPService) ListAgentStatuses() ([]store.AgentStatus, error) {
	return s.client.ListAgentStatuses()
}

func (s *HTTPService) UpdateAgent(id string, request AgentUpdateRequest) (store.Agent, error) {
	return s.client.UpdateAgent(id, client.AgentUpdateRequest(request))
}

func (s *HTTPService) DeleteAgent(id string) error {
	return s.client.DeleteAgent(id)
}

func (s *HTTPService) SetAgentConfig(agentID string, key, value string) error {
	return s.client.SetAgentConfig(agentID, key, value)
}

func (s *HTTPService) ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error) {
	return s.client.ListAgentConfig(agentID)
}

func (s *HTTPService) DeleteAgentConfig(agentID string, key string) error {
	return s.client.DeleteAgentConfig(agentID, key)
}

func (s *HTTPService) RegisterAgent(request AgentRegisterRequest) (store.Agent, error) {
	return s.client.RegisterAgent(client.AgentRegisterRequest(request))
}

func (s *HTTPService) HeartbeatAgent(agentID, password, status string) error {
	return s.client.HeartbeatAgent(agentID, password, status)
}

func (s *HTTPService) RequestAgentWork(request AgentRequest) (AgentWorkResponse, error) {
	resp, err := s.client.RequestAgentWork(client.AgentRequest(request))
	if err != nil {
		return AgentWorkResponse{}, err
	}
	return AgentWorkResponse(resp), nil
}

func (s *HTTPService) AgentUpdateTicket(id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	return s.client.AgentUpdateTicket(id, client.AgentTicketUpdateRequest(request))
}

func (s *HTTPService) CreateProject(request ProjectCreateRequest) (store.Project, error) {
	return s.client.CreateProject(client.ProjectCreateRequest(request))
}

func (s *HTTPService) ListProjects() ([]store.Project, error) {
	return s.client.ListProjects()
}

func (s *HTTPService) GetProject(id string) (store.Project, error) {
	return s.client.GetProject(id)
}

func (s *HTTPService) UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error) {
	return s.client.UpdateProject(id, client.ProjectUpdateRequest(request))
}

func (s *HTTPService) DeleteProject(id int64) error {
	return s.client.DeleteProject(id)
}

func (s *HTTPService) RenameProjectPrefix(id int64, newPrefix string) (int, error) {
	return 0, fmt.Errorf("rename-prefix is not supported in remote mode")
}

func (s *HTTPService) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	return s.client.SetProjectEnabled(id, enabled)
}

func (s *HTTPService) SetProjectDefaultDraft(projectID int64, draft bool) error {
	return s.client.SetProjectDefaultDraft(projectID, draft)
}

func (s *HTTPService) AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	return s.client.AddProjectMember(projectID, client.ProjectMemberRequest(request))
}

func (s *HTTPService) RemoveProjectMember(projectID int64, userID string) error {
	return s.client.RemoveProjectMember(projectID, userID)
}

func (s *HTTPService) ListProjectMembers(projectID int64) ([]store.ProjectMember, error) {
	return s.client.ListProjectMembers(projectID)
}

func (s *HTTPService) AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	return s.client.AddProjectTeamMember(projectID, client.ProjectTeamMemberRequest(request))
}

func (s *HTTPService) RemoveProjectTeamMember(projectID, teamID int64) error {
	return s.client.RemoveProjectTeamMember(projectID, teamID)
}

func (s *HTTPService) ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error) {
	return s.client.ListProjectTeamMembers(projectID)
}

func (s *HTTPService) CreateTeam(request TeamRequest) (store.Team, error) {
	return s.client.CreateTeam(client.TeamRequest(request))
}

func (s *HTTPService) ListTeams() ([]store.Team, error) {
	return s.client.ListTeams()
}

func (s *HTTPService) UpdateTeam(id int64, request TeamRequest) (store.Team, error) {
	return s.client.UpdateTeam(id, client.TeamRequest(request))
}

func (s *HTTPService) DeleteTeam(id int64) error {
	return s.client.DeleteTeam(id)
}

func (s *HTTPService) AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	return s.client.AddTeamMember(teamID, client.TeamMemberRequest(request))
}

func (s *HTTPService) RemoveTeamMember(teamID int64, userID string) error {
	return s.client.RemoveTeamMember(teamID, userID)
}

func (s *HTTPService) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	return s.client.ListTeamMembers(teamID)
}

func (s *HTTPService) AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error) {
	return s.client.AddTeamAgent(teamID, agentID)
}

func (s *HTTPService) RemoveTeamAgent(teamID int64, agentID string) error {
	return s.client.RemoveTeamAgent(teamID, agentID)
}

func (s *HTTPService) ListTeamAgents(teamID int64) ([]store.TeamAgent, error) {
	return s.client.ListTeamAgents(teamID)
}

func (s *HTTPService) CreateTicket(request TicketCreateRequest) (store.Ticket, error) {
	return s.client.CreateTicket(client.TicketCreateRequest(request))
}

func (s *HTTPService) ListTickets(projectID int64) ([]store.Ticket, error) {
	return s.client.ListTickets(projectID)
}

func (s *HTTPService) ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	return s.client.ListTicketsFiltered(projectID, taskType, stage, state, status, search, assignee, limit, includeArchived)
}

func (s *HTTPService) UpdateTicket(id string, request TicketUpdateRequest) (store.Ticket, error) {
	return s.client.UpdateTicket(id, client.TicketUpdateRequest(request))
}

func (s *HTTPService) CloseTicket(id string, message string) (store.Ticket, error) {
	return s.client.CloseTicket(id, message)
}

func (s *HTTPService) OpenTicket(id string, message string) (store.Ticket, error) {
	return s.client.OpenTicket(id, message)
}

func (s *HTTPService) ArchiveTicket(id string, message string) (store.Ticket, error) {
	return s.client.ArchiveTicket(id, message)
}

func (s *HTTPService) UnarchiveTicket(id string, message string) (store.Ticket, error) {
	return s.client.UnarchiveTicket(id, message)
}

func (s *HTTPService) ReadyTicket(id string, message string) (store.Ticket, error) {
	return s.client.ReadyTicket(id, message)
}

func (s *HTTPService) NotReadyTicket(id string, message string) (store.Ticket, error) {
	return s.client.NotReadyTicket(id, message)
}

func (s *HTTPService) SetTicketSdlc(id string, sdlcID int64) (store.Ticket, error) {
	return s.client.SetTicketSdlc(id, sdlcID)
}

func (s *HTTPService) UnsetTicketSdlc(id string) (store.Ticket, error) {
	return s.client.UnsetTicketSdlc(id)
}

func (s *HTTPService) DeleteTicket(id string) error {
	return s.client.DeleteTicket(id)
}

func (s *HTTPService) SetTicketParent(id string, parentID string, message string) (store.Ticket, error) {
	return s.client.SetTicketParent(id, parentID, message)
}

func (s *HTTPService) UnsetTicketParent(id string, message string) (store.Ticket, error) {
	return s.client.UnsetTicketParent(id, message)
}

func (s *HTTPService) GetTicketByID(id string) (store.Ticket, error) {
	return s.client.GetTicketByID(id)
}

func (s *HTTPService) GetTicket(ref string) (store.Ticket, error) {
	return s.client.GetTicket(ref)
}

func (s *HTTPService) CloneTicket(id string, message string) (store.Ticket, error) {
	return s.client.CloneTicket(id, message)
}

func (s *HTTPService) ListHistory(id string) ([]store.HistoryEvent, error) {
	return s.client.ListHistory(id)
}

func (s *HTTPService) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistory(projectID, limit)
}

func (s *HTTPService) ListProjectHistoryFiltered(projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistoryFiltered(projectID, limit, filter)
}

func (s *HTTPService) AddComment(id string, comment string) (store.Comment, error) {
	return s.client.AddComment(id, comment)
}

func (s *HTTPService) ListComments(id string) ([]store.Comment, error) {
	return s.client.ListComments(id)
}

func (s *HTTPService) SetTicketHealth(id string, score int) (store.Ticket, error) {
	return s.client.SetTicketHealth(id, score)
}

func (s *HTTPService) AddDependency(request DependencyRequest) (store.Dependency, error) {
	return s.client.AddDependency(client.DependencyRequest(request))
}

func (s *HTTPService) RemoveDependency(request DependencyRequest) error {
	return s.client.RemoveDependency(client.DependencyRequest(request))
}

func (s *HTTPService) ListDependencies(id string) ([]store.Dependency, error) {
	return s.client.ListDependencies(id)
}

func (s *HTTPService) RequestTicket(request TicketRequest) (TicketRequestResponse, error) {
	response, err := s.client.RequestTicket(client.TicketRequest(request))
	if err != nil {
		return TicketRequestResponse{}, err
	}
	return TicketRequestResponse(response), nil
}

func (s *HTTPService) CreateSdlc(request SdlcRequest) (store.Sdlc, error) {
	return s.client.CreateSdlc(client.SdlcRequest(request))
}

func (s *HTTPService) ListSdlcs() ([]store.Sdlc, error) {
	return s.client.ListSdlcs()
}

func (s *HTTPService) GetSdlc(id int64) (store.SdlcWithStages, error) {
	return s.client.GetSdlc(id)
}

func (s *HTTPService) DeleteSdlc(id int64) error {
	return s.client.DeleteSdlc(id)
}

func (s *HTTPService) AddSdlcStage(sdlcID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	return s.client.AddSdlcStage(sdlcID, client.SdlcStageRequest(request))
}

func (s *HTTPService) UpdateSdlcStage(stageID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	return s.client.UpdateSdlcStage(stageID, client.SdlcStageRequest(request))
}

func (s *HTTPService) GetSdlcStage(stageID int64) (store.SdlcStage, error) {
	return s.client.GetSdlcStage(stageID)
}

func (s *HTTPService) ListSdlcStages(sdlcID int64) ([]store.SdlcStage, error) {
	return s.client.ListSdlcStages(sdlcID)
}

func (s *HTTPService) RemoveSdlcStage(stageID int64) error {
	return s.client.RemoveSdlcStage(stageID)
}

func (s *HTTPService) ReorderSdlcStages(sdlcID int64, stageIDs []int64) error {
	return s.client.ReorderSdlcStages(sdlcID, stageIDs)
}

func (s *HTTPService) ExportSdlc(id int64) (store.SdlcExport, error) {
	return s.client.ExportSdlc(id)
}

func (s *HTTPService) ImportSdlc(export store.SdlcExport) (store.Sdlc, error) {
	return s.client.ImportSdlc(export)
}

func (s *HTTPService) AddSdlcStageRole(sdlcID, stageID, roleID int64) error {
	return s.client.AddSdlcStageRole(sdlcID, stageID, roleID)
}

func (s *HTTPService) RemoveSdlcStageRole(sdlcID, stageID, roleID int64) error {
	return s.client.RemoveSdlcStageRole(sdlcID, stageID, roleID)
}

func (s *HTTPService) ReorderSdlcStageRoles(sdlcID, stageID int64, roleIDs []int64) error {
	return s.client.ReorderSdlcStageRoles(sdlcID, stageID, roleIDs)
}

func (s *HTTPService) CompleteTicket(id string, message string) (store.Ticket, error) {
	return s.client.CompleteTicket(id, message)
}

func (s *HTTPService) ReopenTicket(id string, message string) (store.Ticket, error) {
	return s.client.ReopenTicket(id, message)
}

func (s *HTTPService) DraftTicket(id string, message string) (store.Ticket, error) {
	return s.client.DraftTicket(id, message)
}

func (s *HTTPService) UndraftTicket(id string, message string) (store.Ticket, error) {
	return s.client.UndraftTicket(id, message)
}

func (s *HTTPService) NextTicket(id string) (store.Ticket, error) {
	return s.client.NextTicket(id)
}

func (s *HTTPService) PreviousTicket(id string) (store.Ticket, error) {
	return s.client.PreviousTicket(id)
}

func (s *HTTPService) LogTime(ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
	return s.client.LogTime(ticketID, client.TimeEntryRequest(request))
}

func (s *HTTPService) ListTimeEntries(ticketID string) ([]store.TimeEntry, error) {
	return s.client.ListTimeEntries(ticketID)
}

func (s *HTTPService) DeleteTimeEntry(id int64) error {
	return s.client.DeleteTimeEntry(id)
}

func (s *HTTPService) TotalTimeForTicket(ticketID string) (int, error) {
	return s.client.TotalTimeForTicket(ticketID)
}

func (s *HTTPService) CreateLabel(projectID int64, request LabelRequest) (store.Label, error) {
	return s.client.CreateLabel(projectID, client.LabelRequest(request))
}

func (s *HTTPService) ListLabels(projectID int64) ([]store.Label, error) {
	return s.client.ListLabels(projectID)
}

func (s *HTTPService) DeleteLabel(id int64) error {
	return s.client.DeleteLabel(id)
}

func (s *HTTPService) AddTicketLabel(ticketID string, labelID int64) error {
	return s.client.AddTicketLabel(ticketID, labelID)
}

func (s *HTTPService) RemoveTicketLabel(ticketID string, labelID int64) error {
	return s.client.RemoveTicketLabel(ticketID, labelID)
}

func (s *HTTPService) ListTicketLabels(ticketID string) ([]store.Label, error) {
	return s.client.ListTicketLabels(ticketID)
}

func (s *HTTPService) CreateStory(projectID int64, title, description string) (store.Story, error) {
	return s.client.CreateStory(projectID, title, description)
}

func (s *HTTPService) ListStories(projectID int64) ([]store.Story, error) {
	return s.client.ListStories(projectID)
}

func (s *HTTPService) GetStory(id int64) (store.Story, error) {
	return s.client.GetStory(id)
}

func (s *HTTPService) UpdateStory(id int64, title, description string) (store.Story, error) {
	return s.client.UpdateStory(id, title, description)
}

func (s *HTTPService) DeleteStory(id int64) error {
	return s.client.DeleteStory(id)
}
