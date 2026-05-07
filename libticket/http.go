// http.go implements the Service interface over HTTP, delegating to a running
// ticket server via the internal/client package.
package libticket

import (
	"context"
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

func (s *HTTPService) Status(ctx context.Context) (StatusResponse, error) {
	status, err := s.client.Status(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	return StatusResponse(status), nil
}

func (s *HTTPService) SetRegistrationEnabled(ctx context.Context, enabled bool) error {
	return s.client.SetRegistrationEnabled(ctx, enabled)
}

func (s *HTTPService) Register(ctx context.Context, username, password string) (store.User, error) {
	return s.client.Register(ctx, username, password)
}

func (s *HTTPService) Login(ctx context.Context, username, password string) (store.User, string, error) {
	response, err := s.client.Login(ctx, username, password)
	if err != nil {
		return store.User{}, "", err
	}
	return response.User, response.Token, nil
}

func (s *HTTPService) Logout(ctx context.Context) error {
	return s.client.Logout(ctx)
}

func (s *HTTPService) Count(ctx context.Context, projectID *int64) (CountSummary, error) {
	return s.client.Count(ctx, projectID)
}

func (s *HTTPService) CreateUser(ctx context.Context, username, password string) (store.User, error) {
	return s.client.CreateUser(ctx, username, password)
}

func (s *HTTPService) SetUserEnabled(ctx context.Context, username string, enabled bool) error {
	return s.client.SetUserEnabled(ctx, username, enabled)
}

func (s *HTTPService) ListUsers(ctx context.Context) ([]store.User, error) {
	return s.client.ListUsers(ctx)
}

func (s *HTTPService) DeleteUser(ctx context.Context, username string) error {
	return s.client.DeleteUser(ctx, username)
}

func (s *HTTPService) ResetUserPassword(ctx context.Context, username, newPassword string) (store.User, error) {
	return s.client.ResetUserPassword(ctx, username, newPassword)
}

func (s *HTTPService) CreateRole(ctx context.Context, request RoleRequest) (store.Role, error) {
	return s.client.CreateRole(ctx, client.RoleRequest(request))
}

func (s *HTTPService) ListRoles(ctx context.Context) ([]store.Role, error) {
	return s.client.ListRoles(ctx)
}

func (s *HTTPService) UpdateRole(ctx context.Context, id int64, request RoleRequest) (store.Role, error) {
	return s.client.UpdateRole(ctx, id, client.RoleRequest(request))
}

func (s *HTTPService) DeleteRole(ctx context.Context, id int64) error {
	return s.client.DeleteRole(ctx, id)
}

func (s *HTTPService) CreateAgent(ctx context.Context, request AgentCreateRequest) (store.Agent, string, error) {
	return s.client.CreateAgent(ctx, client.AgentCreateRequest(request))
}

func (s *HTTPService) SetAgentEnabled(ctx context.Context, id string, enabled bool) (store.Agent, error) {
	return s.client.SetAgentEnabled(ctx, id, enabled)
}

func (s *HTTPService) ListAgents(ctx context.Context) ([]store.Agent, error) {
	return s.client.ListAgents(ctx)
}

func (s *HTTPService) ListAgentStatuses(ctx context.Context) ([]store.AgentStatus, error) {
	return s.client.ListAgentStatuses(ctx)
}

func (s *HTTPService) UpdateAgent(ctx context.Context, id string, request AgentUpdateRequest) (store.Agent, error) {
	return s.client.UpdateAgent(ctx, id, client.AgentUpdateRequest(request))
}

func (s *HTTPService) DeleteAgent(ctx context.Context, id string) error {
	return s.client.DeleteAgent(ctx, id)
}

func (s *HTTPService) SetAgentConfig(ctx context.Context, agentID string, key, value string) error {
	return s.client.SetAgentConfig(ctx, agentID, key, value)
}

func (s *HTTPService) ListAgentConfig(ctx context.Context, agentID string) ([]store.AgentConfigEntry, error) {
	return s.client.ListAgentConfig(ctx, agentID)
}

func (s *HTTPService) DeleteAgentConfig(ctx context.Context, agentID string, key string) error {
	return s.client.DeleteAgentConfig(ctx, agentID, key)
}

func (s *HTTPService) RegisterAgent(ctx context.Context, request AgentRegisterRequest) (store.Agent, error) {
	return s.client.RegisterAgent(ctx, client.AgentRegisterRequest(request))
}

func (s *HTTPService) HeartbeatAgent(ctx context.Context, agentID, password, status string) error {
	return s.client.HeartbeatAgent(ctx, agentID, password, status)
}

func (s *HTTPService) RequestAgentWork(ctx context.Context, request AgentRequest) (AgentWorkResponse, error) {
	resp, err := s.client.RequestAgentWork(ctx, client.AgentRequest(request))
	if err != nil {
		return AgentWorkResponse{}, err
	}
	return AgentWorkResponse(resp), nil
}

func (s *HTTPService) AgentUpdateTicket(ctx context.Context, id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	return s.client.AgentUpdateTicket(ctx, id, client.AgentTicketUpdateRequest(request))
}

func (s *HTTPService) CreateProject(ctx context.Context, request ProjectCreateRequest) (store.Project, error) {
	return s.client.CreateProject(ctx, client.ProjectCreateRequest(request))
}

func (s *HTTPService) ListProjects(ctx context.Context) ([]store.Project, error) {
	return s.client.ListProjects(ctx)
}

func (s *HTTPService) GetProject(ctx context.Context, id string) (store.Project, error) {
	return s.client.GetProject(ctx, id)
}

func (s *HTTPService) UpdateProject(ctx context.Context, id int64, request ProjectUpdateRequest) (store.Project, error) {
	return s.client.UpdateProject(ctx, id, client.ProjectUpdateRequest(request))
}

func (s *HTTPService) DeleteProject(ctx context.Context, id int64) error {
	return s.client.DeleteProject(ctx, id)
}

func (s *HTTPService) RenameProjectPrefix(ctx context.Context, id int64, newPrefix string) (int, error) {
	return 0, fmt.Errorf("rename-prefix is not supported in remote mode")
}

func (s *HTTPService) SetProjectEnabled(ctx context.Context, id int64, enabled bool) (store.Project, error) {
	return s.client.SetProjectEnabled(ctx, id, enabled)
}

func (s *HTTPService) SetProjectDefaultDraft(ctx context.Context, projectID int64, draft bool) error {
	return s.client.SetProjectDefaultDraft(ctx, projectID, draft)
}

func (s *HTTPService) AddProjectMember(ctx context.Context, projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	return s.client.AddProjectMember(ctx, projectID, client.ProjectMemberRequest(request))
}

func (s *HTTPService) RemoveProjectMember(ctx context.Context, projectID int64, userID string) error {
	return s.client.RemoveProjectMember(ctx, projectID, userID)
}

func (s *HTTPService) ListProjectMembers(ctx context.Context, projectID int64) ([]store.ProjectMember, error) {
	return s.client.ListProjectMembers(ctx, projectID)
}

func (s *HTTPService) AddProjectTeamMember(ctx context.Context, projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	return s.client.AddProjectTeamMember(ctx, projectID, client.ProjectTeamMemberRequest(request))
}

func (s *HTTPService) RemoveProjectTeamMember(ctx context.Context, projectID, teamID int64) error {
	return s.client.RemoveProjectTeamMember(ctx, projectID, teamID)
}

func (s *HTTPService) ListProjectTeamMembers(ctx context.Context, projectID int64) ([]store.ProjectTeamMember, error) {
	return s.client.ListProjectTeamMembers(ctx, projectID)
}

func (s *HTTPService) CreateTeam(ctx context.Context, request TeamRequest) (store.Team, error) {
	return s.client.CreateTeam(ctx, client.TeamRequest(request))
}

func (s *HTTPService) ListTeams(ctx context.Context) ([]store.Team, error) {
	return s.client.ListTeams(ctx)
}

func (s *HTTPService) UpdateTeam(ctx context.Context, id int64, request TeamRequest) (store.Team, error) {
	return s.client.UpdateTeam(ctx, id, client.TeamRequest(request))
}

func (s *HTTPService) DeleteTeam(ctx context.Context, id int64) error {
	return s.client.DeleteTeam(ctx, id)
}

func (s *HTTPService) AddTeamMember(ctx context.Context, teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	return s.client.AddTeamMember(ctx, teamID, client.TeamMemberRequest(request))
}

func (s *HTTPService) RemoveTeamMember(ctx context.Context, teamID int64, userID string) error {
	return s.client.RemoveTeamMember(ctx, teamID, userID)
}

func (s *HTTPService) ListTeamMembers(ctx context.Context, teamID int64) ([]store.TeamMember, error) {
	return s.client.ListTeamMembers(ctx, teamID)
}

func (s *HTTPService) AddTeamAgent(ctx context.Context, teamID int64, agentID string) (store.TeamAgent, error) {
	return s.client.AddTeamAgent(ctx, teamID, agentID)
}

func (s *HTTPService) RemoveTeamAgent(ctx context.Context, teamID int64, agentID string) error {
	return s.client.RemoveTeamAgent(ctx, teamID, agentID)
}

func (s *HTTPService) ListTeamAgents(ctx context.Context, teamID int64) ([]store.TeamAgent, error) {
	return s.client.ListTeamAgents(ctx, teamID)
}

func (s *HTTPService) CreateTicket(ctx context.Context, request TicketCreateRequest) (store.Ticket, error) {
	return s.client.CreateTicket(ctx, client.TicketCreateRequest(request))
}

func (s *HTTPService) ListTickets(ctx context.Context, projectID int64) ([]store.Ticket, error) {
	return s.client.ListTickets(ctx, projectID)
}

func (s *HTTPService) ListTicketsFiltered(ctx context.Context, projectID int64, taskType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	return s.client.ListTicketsFiltered(ctx, projectID, taskType, stage, state, status, search, assignee, limit, includeArchived)
}

func (s *HTTPService) UpdateTicket(ctx context.Context, id string, request TicketUpdateRequest) (store.Ticket, error) {
	return s.client.UpdateTicket(ctx, id, client.TicketUpdateRequest(request))
}

func (s *HTTPService) CloseTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.CloseTicket(ctx, id, message)
}

func (s *HTTPService) OpenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.OpenTicket(ctx, id, message)
}

func (s *HTTPService) ArchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.ArchiveTicket(ctx, id, message)
}

func (s *HTTPService) UnarchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.UnarchiveTicket(ctx, id, message)
}

func (s *HTTPService) ReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.ReadyTicket(ctx, id, message)
}

func (s *HTTPService) NotReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.NotReadyTicket(ctx, id, message)
}

func (s *HTTPService) SetTicketWorkflow(ctx context.Context, id string, workflowID int64) (store.Ticket, error) {
	return s.client.SetTicketWorkflow(ctx, id, workflowID)
}

func (s *HTTPService) UnsetTicketWorkflow(ctx context.Context, id string) (store.Ticket, error) {
	return s.client.UnsetTicketWorkflow(ctx, id)
}

func (s *HTTPService) DeleteTicket(ctx context.Context, id string) error {
	return s.client.DeleteTicket(ctx, id)
}

func (s *HTTPService) SetTicketParent(ctx context.Context, id string, parentID string, message string) (store.Ticket, error) {
	return s.client.SetTicketParent(ctx, id, parentID, message)
}

func (s *HTTPService) UnsetTicketParent(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.UnsetTicketParent(ctx, id, message)
}

func (s *HTTPService) GetTicketByID(ctx context.Context, id string) (store.Ticket, error) {
	return s.client.GetTicketByID(ctx, id)
}

func (s *HTTPService) GetTicket(ctx context.Context, ref string) (store.Ticket, error) {
	return s.client.GetTicket(ctx, ref)
}

func (s *HTTPService) CloneTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.CloneTicket(ctx, id, message)
}

func (s *HTTPService) ListHistory(ctx context.Context, id string) ([]store.HistoryEvent, error) {
	return s.client.ListHistory(ctx, id)
}

func (s *HTTPService) ListHistoryPaged(ctx context.Context, id string, limit, offset int) ([]store.HistoryEvent, error) {
	return s.client.ListHistoryPaged(ctx, id, limit, offset)
}

func (s *HTTPService) ListProjectHistory(ctx context.Context, projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistory(ctx, projectID, limit)
}

func (s *HTTPService) ListProjectHistoryFiltered(ctx context.Context, projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	return s.client.ListProjectHistoryFiltered(ctx, projectID, limit, filter)
}

func (s *HTTPService) AddComment(ctx context.Context, id string, comment string) (store.Comment, error) {
	return s.client.AddComment(ctx, id, comment)
}

func (s *HTTPService) ListComments(ctx context.Context, id string) ([]store.Comment, error) {
	return s.client.ListComments(ctx, id)
}

func (s *HTTPService) SetTicketHealth(ctx context.Context, id string, score int) (store.Ticket, error) {
	return s.client.SetTicketHealth(ctx, id, score)
}

func (s *HTTPService) AddDependency(ctx context.Context, request DependencyRequest) (store.Dependency, error) {
	return s.client.AddDependency(ctx, client.DependencyRequest(request))
}

func (s *HTTPService) RemoveDependency(ctx context.Context, request DependencyRequest) error {
	return s.client.RemoveDependency(ctx, client.DependencyRequest(request))
}

func (s *HTTPService) ListDependencies(ctx context.Context, id string) ([]store.Dependency, error) {
	return s.client.ListDependencies(ctx, id)
}

func (s *HTTPService) RequestTicket(ctx context.Context, request TicketRequest) (TicketRequestResponse, error) {
	response, err := s.client.RequestTicket(ctx, client.TicketRequest(request))
	if err != nil {
		return TicketRequestResponse{}, err
	}
	return TicketRequestResponse(response), nil
}

func (s *HTTPService) CreateWorkflow(ctx context.Context, request WorkflowRequest) (store.Workflow, error) {
	return s.client.CreateWorkflow(ctx, client.WorkflowRequest(request))
}

func (s *HTTPService) ListWorkflows(ctx context.Context) ([]store.Workflow, error) {
	return s.client.ListWorkflows(ctx)
}

func (s *HTTPService) GetWorkflow(ctx context.Context, id int64) (store.WorkflowWithStages, error) {
	return s.client.GetWorkflow(ctx, id)
}

func (s *HTTPService) DeleteWorkflow(ctx context.Context, id int64) error {
	return s.client.DeleteWorkflow(ctx, id)
}

func (s *HTTPService) AddWorkflowStage(ctx context.Context, workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	return s.client.AddWorkflowStage(ctx, workflowID, client.WorkflowStageRequest(request))
}

func (s *HTTPService) UpdateWorkflowStage(ctx context.Context, stageID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	return s.client.UpdateWorkflowStage(ctx, stageID, client.WorkflowStageRequest(request))
}

func (s *HTTPService) GetWorkflowStage(ctx context.Context, stageID int64) (store.WorkflowStage, error) {
	return s.client.GetWorkflowStage(ctx, stageID)
}

func (s *HTTPService) ListWorkflowStages(ctx context.Context, workflowID int64) ([]store.WorkflowStage, error) {
	return s.client.ListWorkflowStages(ctx, workflowID)
}

func (s *HTTPService) RemoveWorkflowStage(ctx context.Context, stageID int64) error {
	return s.client.RemoveWorkflowStage(ctx, stageID)
}

func (s *HTTPService) ReorderWorkflowStages(ctx context.Context, workflowID int64, stageIDs []int64) error {
	return s.client.ReorderWorkflowStages(ctx, workflowID, stageIDs)
}

func (s *HTTPService) ExportWorkflow(ctx context.Context, id int64) (store.WorkflowExport, error) {
	return s.client.ExportWorkflow(ctx, id)
}

func (s *HTTPService) ImportWorkflow(ctx context.Context, export store.WorkflowExport) (store.Workflow, error) {
	return s.client.ImportWorkflow(ctx, export)
}

func (s *HTTPService) AddWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	return s.client.AddWorkflowStageRole(ctx, workflowID, stageID, roleID)
}

func (s *HTTPService) RemoveWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	return s.client.RemoveWorkflowStageRole(ctx, workflowID, stageID, roleID)
}

func (s *HTTPService) ReorderWorkflowStageRoles(ctx context.Context, workflowID, stageID int64, roleIDs []int64) error {
	return s.client.ReorderWorkflowStageRoles(ctx, workflowID, stageID, roleIDs)
}

func (s *HTTPService) CompleteTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.CompleteTicket(ctx, id, message)
}

func (s *HTTPService) ReopenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.ReopenTicket(ctx, id, message)
}

func (s *HTTPService) DraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.DraftTicket(ctx, id, message)
}

func (s *HTTPService) UndraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.client.UndraftTicket(ctx, id, message)
}

func (s *HTTPService) NextTicket(ctx context.Context, id string) (store.Ticket, error) {
	return s.client.NextTicket(ctx, id)
}

func (s *HTTPService) PreviousTicket(ctx context.Context, id string) (store.Ticket, error) {
	return s.client.PreviousTicket(ctx, id)
}

func (s *HTTPService) LogTime(ctx context.Context, ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
	return s.client.LogTime(ctx, ticketID, client.TimeEntryRequest(request))
}

func (s *HTTPService) ListTimeEntries(ctx context.Context, ticketID string) ([]store.TimeEntry, error) {
	return s.client.ListTimeEntries(ctx, ticketID)
}

func (s *HTTPService) DeleteTimeEntry(ctx context.Context, id int64) error {
	return s.client.DeleteTimeEntry(ctx, id)
}

func (s *HTTPService) TotalTimeForTicket(ctx context.Context, ticketID string) (int, error) {
	return s.client.TotalTimeForTicket(ctx, ticketID)
}

func (s *HTTPService) CreateLabel(ctx context.Context, projectID int64, request LabelRequest) (store.Label, error) {
	return s.client.CreateLabel(ctx, projectID, client.LabelRequest(request))
}

func (s *HTTPService) ListLabels(ctx context.Context, projectID int64) ([]store.Label, error) {
	return s.client.ListLabels(ctx, projectID)
}

func (s *HTTPService) DeleteLabel(ctx context.Context, id int64) error {
	return s.client.DeleteLabel(ctx, id)
}

func (s *HTTPService) AddTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	return s.client.AddTicketLabel(ctx, ticketID, labelID)
}

func (s *HTTPService) RemoveTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	return s.client.RemoveTicketLabel(ctx, ticketID, labelID)
}

func (s *HTTPService) ListTicketLabels(ctx context.Context, ticketID string) ([]store.Label, error) {
	return s.client.ListTicketLabels(ctx, ticketID)
}

func (s *HTTPService) CreateStory(ctx context.Context, projectID int64, title, description string) (store.Story, error) {
	return s.CreateStoryWithRequest(ctx, StoryCreateRequest{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
	})
}

func (s *HTTPService) CreateStoryWithRequest(ctx context.Context, request StoryCreateRequest) (store.Story, error) {
	return s.client.CreateStoryWithRequest(ctx, client.StoryCreateRequest(request))
}

func (s *HTTPService) ListStories(ctx context.Context, projectID int64) ([]store.Story, error) {
	return s.client.ListStories(ctx, projectID)
}

func (s *HTTPService) GetStory(ctx context.Context, id int64) (store.Story, error) {
	return s.client.GetStory(ctx, id)
}

func (s *HTTPService) UpdateStory(ctx context.Context, id int64, title, description string) (store.Story, error) {
	return s.client.UpdateStory(ctx, id, title, description)
}

func (s *HTTPService) DeleteStory(ctx context.Context, id int64) error {
	return s.client.DeleteStory(ctx, id)
}
