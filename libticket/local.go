package libticket

import (
	"database/sql"
	"errors"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func resolveRequestLifecycle(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

type LocalService struct {
	cfg config.Config
}

func NewLocal(cfg config.Config) *LocalService {
	return &LocalService{cfg: cfg}
}

func (s *LocalService) Status() (StatusResponse, error) {
	resolved, err := config.ResolveURL()
	if err != nil {
		return StatusResponse{}, err
	}
	path := resolved.DBPath
	if _, err := os.Stat(path); err != nil {
		return StatusResponse{}, err
	}
	db, err := store.Open(path)
	if err != nil {
		return StatusResponse{}, err
	}
	defer db.Close()
	user, err := store.GetUserByUsername(db, LocalUsername())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		enabled, regErr := store.RegistrationEnabled(db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	case err != nil:
		return StatusResponse{}, err
	case !user.Enabled:
		enabled, regErr := store.RegistrationEnabled(db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	}
	enabled, err := store.RegistrationEnabled(db)
	if err != nil {
		return StatusResponse{}, err
	}
	return StatusResponse{
		Status:              "ok",
		Authenticated:       true,
		RegistrationEnabled: enabled,
		User:                &user,
	}, nil
}

func (s *LocalService) SetRegistrationEnabled(enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetRegistrationEnabled(db, enabled)
}

func (s *LocalService) Register(username, password string) (store.User, error) {
	return store.User{}, errors.New("ticket register requires TICKET_URL=http(s)://...")
}

func (s *LocalService) Login(username, password string) (store.User, string, error) {
	return store.User{}, "", errors.New("ticket login requires TICKET_URL=http(s)://...")
}

func (s *LocalService) Logout() error {
	return errors.New("ticket logout requires TICKET_URL=http(s)://...")
}

func (s *LocalService) Count(projectID *int64) (CountSummary, error) {
	db, err := s.openDB()
	if err != nil {
		return CountSummary{}, err
	}
	defer db.Close()
	return store.CountEverything(db, projectID)
}

func (s *LocalService) CreateUser(username, password string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	defer db.Close()
	return store.CreateUser(db, username, password, "user")
}

func (s *LocalService) SetUserEnabled(username string, enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetUserEnabled(db, username, enabled)
}

func (s *LocalService) ListUsers() ([]store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListUsers(db)
}

func (s *LocalService) DeleteUser(username string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteUser(db, username)
}

func (s *LocalService) ResetUserPassword(username, newPassword string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	defer db.Close()
	return store.ResetUserPassword(db, username, newPassword)
}

func (s *LocalService) CreateRole(request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	defer db.Close()
	return store.CreateRole(db, request.Title, request.Motivation, request.Goals)
}

func (s *LocalService) ListRoles() ([]store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListRoles(db)
}

func (s *LocalService) UpdateRole(id int64, request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	defer db.Close()
	return store.UpdateRole(db, id, request.Title, request.Motivation, request.Goals)
}

func (s *LocalService) DeleteRole(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteRole(db, id)
}

func (s *LocalService) CreateAgent(request AgentCreateRequest) (store.Agent, string, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, "", err
	}
	defer db.Close()
	return store.CreateAgent(db, request.Password)
}

func (s *LocalService) SetAgentEnabled(id string, enabled bool) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	return store.SetAgentEnabled(db, id, enabled)
}

func (s *LocalService) ListAgents() ([]store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgents(db)
}

func (s *LocalService) ListAgentStatuses() ([]store.AgentStatus, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgentStatuses(db)
}

func (s *LocalService) UpdateAgent(id string, request AgentUpdateRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	return store.UpdateAgent(db, id, store.AgentUpdateParams{
		Password: request.Password,
	})
}

func (s *LocalService) DeleteAgent(id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteAgent(db, id)
}

func (s *LocalService) SetAgentConfig(agentID string, key, value string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetAgentConfig(db, agentID, key, value)
}

func (s *LocalService) ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgentConfig(db, agentID)
}

func (s *LocalService) DeleteAgentConfig(agentID string, key string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteAgentConfig(db, agentID, key)
}

func (s *LocalService) RegisterAgent(request AgentRegisterRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(db, request.ID, request.Password)
	if err != nil {
		return store.Agent{}, err
	}
	return store.TouchAgent(db, agent.ID, "online")
}

func (s *LocalService) HeartbeatAgent(agentID, password, status string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(db, agentID, password)
	if err != nil {
		return err
	}
	_, err = store.TouchAgent(db, agent.ID, status)
	return err
}

func (s *LocalService) RequestAgentWork(request AgentRequest) (AgentWorkResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return AgentWorkResponse{}, err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(db, request.ID, request.Password)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	projectID := request.ProjectID
	if request.TicketID != nil {
		projectID = 0
	}
	if request.TicketID == nil && projectID == 0 {
		projects, err := store.ListProjects(db)
		if err != nil {
			return AgentWorkResponse{}, err
		}
		for _, p := range projects {
			if p.Status == "open" {
				projectID = p.ID
				break
			}
		}
	}
	currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(db, projectID, agent.Username)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
		ProjectID: projectID,
		TicketID:  request.TicketID,
		Username:  agent.Username,
		UserID:    "",
		DryRun:    request.DryRun,
	})
	if err != nil {
		return AgentWorkResponse{}, err
	}
	agentStatus := "NONE"
	switch status {
	case "NO-WORK", "REJECTED":
		agentStatus = "NONE"
	case "ASSIGNED", "AVAILABLE":
		if hadCurrent && currentAssigned.ID == ticket.ID {
			agentStatus = "CURRENT"
		} else {
			agentStatus = "NEW"
		}
	default:
		agentStatus = status
	}
	if status == "ASSIGNED" && agentStatus == "NEW" {
		_, _ = store.TouchAgent(db, agent.ID, "working")
	} else {
		_, _ = store.TouchAgent(db, agent.ID, "soliciting")
	}
	response := AgentWorkResponse{Status: agentStatus, Parents: []store.Ticket{}}
	if agentStatus == "NEW" || agentStatus == "CURRENT" {
		project, err := store.GetProjectByID(db, ticket.ProjectID)
		if err == nil {
			response.Project = &project
		}
		response.Ticket = &ticket
		parents, err := store.ListTicketParents(db, ticket.ID)
		if err == nil {
			response.Parents = parents
		}
	}
	return response, nil
}

func (s *LocalService) AgentUpdateTicket(id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(db, request.ID, request.Password)
	if err != nil {
		return store.Ticket{}, err
	}
	current, err := store.GetTicket(db, id)
	if err != nil {
		return store.Ticket{}, err
	}
	updated, err := store.UpdateTicket(db, id, store.TicketUpdateParams{
		Title:              current.Title,
		Description:        request.Result,
		AcceptanceCriteria: current.AcceptanceCriteria,
		GitRepository:      current.GitRepository,
		GitBranch:          current.GitBranch,
		ParentID:           current.ParentID,
		Assignee:           agent.Username,
		State:              store.StateSuccess,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
		UpdatedBy:          "",
		ActorUsername:      agent.Username,
		ActorRole:          "admin",
	})
	if err != nil {
		return store.Ticket{}, err
	}
	_, _ = store.TouchAgent(db, agent.ID, "soliciting")
	return updated, nil
}

func (s *LocalService) CreateProject(request ProjectCreateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Project{}, err
	}
	return store.CreateProjectWithParams(db, store.ProjectCreateParams{
		Prefix:             request.Prefix,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		Notes:              request.Notes,
		Visibility:         request.Visibility,
		CreatedBy:          user.ID,
		WorkflowID:         request.WorkflowID,
	})
}

func (s *LocalService) ListProjects() ([]store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjects(db)
}

func (s *LocalService) GetProject(id string) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.GetProject(db, id)
}

func (s *LocalService) UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.UpdateProjectWithParams(db, id, store.ProjectUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		Notes:              request.Notes,
		Status:             request.Status,
		Visibility:         request.Visibility,
		WorkflowID:         request.WorkflowID,
	})
}

func (s *LocalService) DeleteProject(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteProject(db, id)
}

func (s *LocalService) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.SetProjectStatus(db, id, enabled)
}

func (s *LocalService) AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectMember{}, err
	}
	defer db.Close()
	return store.AddProjectMember(db, projectID, request.UserID, request.Role)
}

func (s *LocalService) RemoveProjectMember(projectID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveProjectMember(db, projectID, userID)
}

func (s *LocalService) ListProjectMembers(projectID int64) ([]store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectMembers(db, projectID)
}

func (s *LocalService) AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectTeamMember{}, err
	}
	defer db.Close()
	return store.AddProjectTeamMember(db, projectID, request.TeamID, request.Role)
}

func (s *LocalService) RemoveProjectTeamMember(projectID, teamID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveProjectTeamMember(db, projectID, teamID)
}

func (s *LocalService) ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectTeamMembers(db, projectID)
}

func (s *LocalService) CreateTeam(request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	defer db.Close()
	return store.CreateTeam(db, request.Name, request.ParentTeamID)
}

func (s *LocalService) ListTeams() ([]store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeams(db)
}

func (s *LocalService) UpdateTeam(id int64, request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	defer db.Close()
	return store.UpdateTeam(db, id, request.Name, request.ParentTeamID)
}

func (s *LocalService) DeleteTeam(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTeam(db, id)
}

func (s *LocalService) AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamMember{}, err
	}
	defer db.Close()
	return store.AddTeamMember(db, teamID, request.UserID, request.Role, request.JobTitle)
}

func (s *LocalService) RemoveTeamMember(teamID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTeamMember(db, teamID, userID)
}

func (s *LocalService) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeamMembers(db, teamID)
}

func (s *LocalService) AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamAgent{}, err
	}
	defer db.Close()
	return store.AddTeamAgent(db, teamID, agentID)
}

func (s *LocalService) RemoveTeamAgent(teamID int64, agentID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTeamAgent(db, teamID, agentID)
}

func (s *LocalService) ListTeamAgents(teamID int64) ([]store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeamAgents(db, teamID)
}

func (s *LocalService) CreateTicket(request TicketCreateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	_, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	return store.CreateTicket(db, store.TicketCreateParams{
		ProjectID:          request.ProjectID,
		ParentID:           request.ParentID,
		CloneOf:            request.CloneOf,
		Type:               request.Type,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		Priority:           request.Priority,
		EstimateEffort:     request.EstimateEffort,
		EstimateComplete:   request.EstimateComplete,
		Assignee:           request.Assignee,
		State:              state,
		CreatedBy:          user.ID,
	})
}

func (s *LocalService) ListTickets(projectID int64) ([]store.Ticket, error) {
	return s.ListTicketsFiltered(projectID, "", "", "", "", "", "", 0, false)
}

func (s *LocalService) ListTicketsFiltered(projectID int64, ticketType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTickets(db, store.TicketListParams{
		ProjectID:       projectID,
		Type:            ticketType,
		Stage:           stage,
		State:           state,
		Status:          status,
		Search:          search,
		Assignee:        assignee,
		Limit:           limit,
		IncludeArchived: includeArchived,
	})
}

func (s *LocalService) UpdateTicket(id string, request TicketUpdateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	stage, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	return store.UpdateTicket(db, id, store.TicketUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		ParentID:           request.ParentID,
		Assignee:           request.Assignee,
		Stage:              stage,
		State:              state,
		Priority:           request.Priority,
		Order:              request.Order,
		EstimateEffort:     request.EstimateEffort,
		EstimateComplete:   request.EstimateComplete,
		UpdatedBy:          user.ID,
		ActorUsername:      user.Username,
		// Local mode bypasses server-side ownership restrictions.
		ActorRole: "admin",
	})
}

func (s *LocalService) CloseTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketOpen(db, id, false, user.Username, user.ID)
}

func (s *LocalService) OpenTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketOpen(db, id, true, user.Username, user.ID)
}

func (s *LocalService) ArchiveTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketArchived(db, id, true, user.Username, user.ID)
}

func (s *LocalService) UnarchiveTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketArchived(db, id, false, user.Username, user.ID)
}

func (s *LocalService) ReadyTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketReady(db, id, true, user.Username, user.ID)
}

func (s *LocalService) NotReadyTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketReady(db, id, false, user.Username, user.ID)
}

func (s *LocalService) SetTicketWorkflow(id string, workflowID int64) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.SetTicketWorkflow(db, id, workflowID)
}

func (s *LocalService) UnsetTicketWorkflow(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.UnsetTicketWorkflow(db, id)
}

func (s *LocalService) DeleteTicket(id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTicket(db, id)
}

func (s *LocalService) SetTicketParent(id string, parentID string) (store.Ticket, error) {
	current, err := s.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(id, TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           &parentID,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
}

func (s *LocalService) SetTicketHealth(id string, score int) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.SetTicketHealth(db, id, score)
}

func (s *LocalService) UnsetTicketParent(id string) (store.Ticket, error) {
	current, err := s.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(id, TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           nil,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
}

func (s *LocalService) GetTicketByID(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicket(db, id)
}

func (s *LocalService) GetTicket(ref string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicketByRef(db, ref)
}

func (s *LocalService) CloneTicket(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.CloneTicket(db, id, user.ID)
}

func (s *LocalService) ListHistory(id string) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListHistoryEvents(db, id)
}

func (s *LocalService) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectHistory(db, projectID, limit)
}

func (s *LocalService) AddComment(id string, comment string) (store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Comment{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Comment{}, err
	}
	return store.AddComment(db, id, user.ID, comment)
}

func (s *LocalService) ListComments(id string) ([]store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListComments(db, id)
}

func (s *LocalService) AddDependency(request DependencyRequest) (store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Dependency{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Dependency{}, err
	}
	return store.AddDependency(db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
}

func (s *LocalService) RemoveDependency(request DependencyRequest) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteDependency(db, request.ProjectID, request.TicketID, request.DependsOn)
}

func (s *LocalService) ListDependencies(id string) ([]store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListDependencies(db, id)
}

func (s *LocalService) RequestTicket(request TicketRequest) (TicketRequestResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return TicketRequestResponse{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
		ProjectID: request.ProjectID,
		TicketID:  request.TicketID,
		TicketRef: request.TicketRef,
		Username:  user.Username,
		UserID:    user.ID,
		DryRun:    request.DryRun,
	})
	if err != nil {
		return TicketRequestResponse{}, err
	}
	response := TicketRequestResponse{Status: status}
	if status == "ASSIGNED" || status == "AVAILABLE" {
		response.Ticket = &ticket
		ctx := store.EnrichTicketContext(db, ticket)
		response.Project = ctx.Project
		response.Parents = ctx.Parents
		response.Workflow = ctx.Workflow
		response.Role = ctx.Role
	}
	return response, nil
}

func (s *LocalService) openDB() (*sql.DB, error) {
	resolved, err := config.ResolveURL()
	if err != nil {
		return nil, err
	}
	return store.Open(resolved.DBPath)
}

func (s *LocalService) localUser(db *sql.DB) (store.User, error) {
	username := LocalUsername()
	if user, err := store.GetUserByUsername(db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	return store.CreateUser(db, username, "local-mode", "admin")
}

func LocalUsername() string {
	return "admin"
}

func (s *LocalService) CreateWorkflow(request WorkflowRequest) (store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Workflow{}, err
	}
	defer db.Close()
	return store.CreateWorkflow(db, request.Name, request.Description)
}

func (s *LocalService) ListWorkflows() ([]store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListWorkflows(db)
}

func (s *LocalService) GetWorkflow(id int64) (store.WorkflowWithStages, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowWithStages{}, err
	}
	defer db.Close()
	return store.GetWorkflow(db, id)
}

func (s *LocalService) DeleteWorkflow(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteWorkflow(db, id)
}

func (s *LocalService) AddWorkflowStage(workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowStage{}, err
	}
	defer db.Close()
	return store.AddWorkflowStage(db, workflowID, request.StageName, request.Description, request.RoleID, request.SortOrder)
}

func (s *LocalService) RemoveWorkflowStage(stageID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveWorkflowStage(db, stageID)
}

func (s *LocalService) ReorderWorkflowStages(workflowID int64, stageIDs []int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.ReorderWorkflowStages(db, workflowID, stageIDs)
}

func (s *LocalService) ExportWorkflow(id int64) (store.WorkflowExport, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowExport{}, err
	}
	defer db.Close()
	return store.ExportWorkflow(db, id)
}

func (s *LocalService) ImportWorkflow(export store.WorkflowExport) (store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Workflow{}, err
	}
	defer db.Close()
	return store.ImportWorkflow(db, export)
}

func (s *LocalService) LogTime(ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TimeEntry{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.TimeEntry{}, err
	}
	return store.LogTime(db, ticketID, user.ID, request.Minutes, request.Note)
}

func (s *LocalService) ListTimeEntries(ticketID string) ([]store.TimeEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTimeEntries(db, ticketID)
}

func (s *LocalService) DeleteTimeEntry(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTimeEntry(db, id)
}

func (s *LocalService) TotalTimeForTicket(ticketID string) (int, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return store.TotalTimeForTicket(db, ticketID)
}

func (s *LocalService) CreateLabel(projectID int64, request LabelRequest) (store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Label{}, err
	}
	defer db.Close()
	return store.CreateLabel(db, projectID, request.Name, request.Color)
}

func (s *LocalService) ListLabels(projectID int64) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListLabels(db, projectID)
}

func (s *LocalService) DeleteLabel(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteLabel(db, id)
}

func (s *LocalService) AddTicketLabel(ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.AddTicketLabel(db, ticketID, labelID)
}

func (s *LocalService) RemoveTicketLabel(ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTicketLabel(db, ticketID, labelID)
}

func (s *LocalService) ListTicketLabels(ticketID string) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTicketLabels(db, ticketID)
}

func (s *LocalService) CreateStory(projectID int64, title, description string) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Story{}, err
	}
	return store.CreateStory(db, projectID, title, description, user.ID)
}

func (s *LocalService) ListStories(projectID int64) ([]store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListStoriesByProject(db, projectID)
}

func (s *LocalService) GetStory(id int64) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	defer db.Close()
	return store.GetStory(db, id)
}

func (s *LocalService) UpdateStory(id int64, title, description string) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	defer db.Close()
	return store.UpdateStory(db, id, title, description)
}

func (s *LocalService) DeleteStory(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteStory(db, id)
}
