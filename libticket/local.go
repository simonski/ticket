// Package libticket provides the core service interface and LocalService implementation
// for interacting with ticket data. Both local (SQLite) and remote (HTTP) implementations
// satisfy the Service interface, enabling identical behaviour regardless of deployment mode.
package libticket

import (
	"context"
	"database/sql"
	"errors"
	"os"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

// LocalService implements Service directly against a local SQLite database.
// Obtain one via NewLocal after loading a config with a local location.
type LocalService struct {
	cfg config.Config
}

// resolveRequestLifecycle derives the canonical stage+state pair from the three
// possible ways a caller may express lifecycle: explicit stage/state flags, a
// rendered status string (e.g. "design/active"), or nothing (no-op).
func resolveRequestLifecycle(status, stage, state string) (string, string, error) {
	if stage != "" || state != "" {
		return stage, state, nil
	}
	if status == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

// NewLocal returns a LocalService bound to the given configuration.
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
	user, err := store.GetUserByUsername(context.Background(), db, LocalUsername())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		enabled, regErr := store.RegistrationEnabled(context.Background(), db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	case err != nil:
		return StatusResponse{}, err
	case !user.Enabled:
		enabled, regErr := store.RegistrationEnabled(context.Background(), db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	}
	enabled, err := store.RegistrationEnabled(context.Background(), db)
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
	return store.SetRegistrationEnabled(context.Background(), db, enabled)
}

func (s *LocalService) Register(username, password string) (store.User, error) {
	return store.User{}, errors.New("ticket register requires remote mode (run tk init to configure)")
}

func (s *LocalService) Login(username, password string) (store.User, string, error) {
	return store.User{}, "", errors.New("ticket login requires remote mode (run tk init to configure)")
}

func (s *LocalService) Logout() error {
	return errors.New("ticket logout requires remote mode (run tk init to configure)")
}

func (s *LocalService) Count(projectID *int64) (CountSummary, error) {
	db, err := s.openDB()
	if err != nil {
		return CountSummary{}, err
	}
	defer db.Close()
	return store.CountEverything(context.Background(), db, projectID)
}

func (s *LocalService) CreateUser(username, password string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	defer db.Close()
	return store.CreateUser(context.Background(), db, username, password, "user")
}

func (s *LocalService) SetUserEnabled(username string, enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetUserEnabled(context.Background(), db, username, enabled)
}

func (s *LocalService) ListUsers() ([]store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListUsers(context.Background(), db)
}

func (s *LocalService) DeleteUser(username string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteUser(context.Background(), db, username)
}

func (s *LocalService) ResetUserPassword(username, newPassword string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	defer db.Close()
	return store.ResetUserPassword(context.Background(), db, username, newPassword)
}

func (s *LocalService) CreateRole(request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	defer db.Close()
	return store.CreateRole(context.Background(), db, request.SdlcID, request.Title, request.Description, request.AcceptanceCriteria)
}

func (s *LocalService) ListRoles() ([]store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListRoles(context.Background(), db)
}

func (s *LocalService) UpdateRole(id int64, request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	defer db.Close()
	return store.UpdateRole(context.Background(), db, id, request.Title, request.Description, request.AcceptanceCriteria)
}

func (s *LocalService) DeleteRole(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteRole(context.Background(), db, id)
}

func (s *LocalService) CreateAgent(request AgentCreateRequest) (store.Agent, string, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, "", err
	}
	defer db.Close()
	return store.CreateAgent(context.Background(), db, request.Password)
}

func (s *LocalService) SetAgentEnabled(id string, enabled bool) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	return store.SetAgentEnabled(context.Background(), db, id, enabled)
}

func (s *LocalService) ListAgents() ([]store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgents(context.Background(), db)
}

func (s *LocalService) ListAgentStatuses() ([]store.AgentStatus, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgentStatuses(context.Background(), db)
}

func (s *LocalService) UpdateAgent(id string, request AgentUpdateRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	return store.UpdateAgent(context.Background(), db, id, store.AgentUpdateParams{
		Password: request.Password,
	})
}

func (s *LocalService) DeleteAgent(id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteAgent(context.Background(), db, id)
}

func (s *LocalService) SetAgentConfig(agentID string, key, value string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetAgentConfig(context.Background(), db, agentID, key, value)
}

func (s *LocalService) ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListAgentConfig(context.Background(), db, agentID)
}

func (s *LocalService) DeleteAgentConfig(agentID string, key string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteAgentConfig(context.Background(), db, agentID, key)
}

func (s *LocalService) RegisterAgent(request AgentRegisterRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
	if err != nil {
		return store.Agent{}, err
	}
	return store.TouchAgent(context.Background(), db, agent.ID, "online")
}

func (s *LocalService) HeartbeatAgent(agentID, password, status string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(context.Background(), db, agentID, password)
	if err != nil {
		return err
	}
	_, err = store.TouchAgent(context.Background(), db, agent.ID, status)
	return err
}

func (s *LocalService) RequestAgentWork(request AgentRequest) (AgentWorkResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return AgentWorkResponse{}, err
	}
	defer db.Close()
	agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	projectID := request.ProjectID
	if request.TicketID != nil {
		projectID = 0
	}
	if request.TicketID == nil && projectID == 0 {
		projects, err := store.ListProjects(context.Background(), db)
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
	currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(context.Background(), db, projectID, agent.Username)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	ticket, status, err := store.RequestTicket(context.Background(), db, store.TicketRequestParams{
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
		_, _ = store.TouchAgent(context.Background(), db, agent.ID, "working")
	} else {
		_, _ = store.TouchAgent(context.Background(), db, agent.ID, "soliciting")
	}
	response := AgentWorkResponse{Status: agentStatus, Parents: []store.Ticket{}}
	if agentStatus == "NEW" || agentStatus == "CURRENT" {
		project, err := store.GetProjectByID(context.Background(), db, ticket.ProjectID)
		if err == nil {
			response.Project = &project
		}
		response.Ticket = &ticket
		parents, err := store.ListTicketParents(context.Background(), db, ticket.ID)
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
	agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
	if err != nil {
		return store.Ticket{}, err
	}
	current, err := store.GetTicket(context.Background(), db, id)
	if err != nil {
		return store.Ticket{}, err
	}
	updated, err := store.UpdateTicket(context.Background(), db, id, store.TicketUpdateParams{
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
	_, _ = store.TouchAgent(context.Background(), db, agent.ID, "soliciting")
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
	return store.CreateProjectWithParams(context.Background(), db, store.ProjectCreateParams{
		Prefix:             request.Prefix,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		Notes:              request.Notes,
		Visibility:         request.Visibility,
		CreatedBy:          user.ID,
		SdlcID:         request.SdlcID,
	})
}

func (s *LocalService) ListProjects() ([]store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjects(context.Background(), db)
}

func (s *LocalService) GetProject(id string) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.GetProject(context.Background(), db, id)
}

func (s *LocalService) UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.UpdateProjectWithParams(context.Background(), db, id, store.ProjectUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		GitRepository:      request.GitRepository,
		GitBranch:          request.GitBranch,
		Notes:              request.Notes,
		Status:             request.Status,
		Visibility:         request.Visibility,
		SdlcID:         request.SdlcID,
	})
}

func (s *LocalService) DeleteProject(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteProject(context.Background(), db, id)
}

func (s *LocalService) RenameProjectPrefix(id int64, newPrefix string) (int, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return store.RenameProjectPrefix(context.Background(), db, id, newPrefix)
}

func (s *LocalService) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.SetProjectStatus(context.Background(), db, id, enabled)
}

func (s *LocalService) AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectMember{}, err
	}
	defer db.Close()
	return store.AddProjectMember(context.Background(), db, projectID, request.UserID, request.Role)
}

func (s *LocalService) RemoveProjectMember(projectID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveProjectMember(context.Background(), db, projectID, userID)
}

func (s *LocalService) ListProjectMembers(projectID int64) ([]store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectMembers(context.Background(), db, projectID)
}

func (s *LocalService) AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectTeamMember{}, err
	}
	defer db.Close()
	return store.AddProjectTeamMember(context.Background(), db, projectID, request.TeamID, request.Role)
}

func (s *LocalService) RemoveProjectTeamMember(projectID, teamID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveProjectTeamMember(context.Background(), db, projectID, teamID)
}

func (s *LocalService) ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectTeamMembers(context.Background(), db, projectID)
}

func (s *LocalService) CreateTeam(request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	defer db.Close()
	return store.CreateTeam(context.Background(), db, request.Name, request.ParentTeamID)
}

func (s *LocalService) ListTeams() ([]store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeams(context.Background(), db)
}

func (s *LocalService) UpdateTeam(id int64, request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	defer db.Close()
	return store.UpdateTeam(context.Background(), db, id, request.Name, request.ParentTeamID)
}

func (s *LocalService) DeleteTeam(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTeam(context.Background(), db, id)
}

func (s *LocalService) AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamMember{}, err
	}
	defer db.Close()
	return store.AddTeamMember(context.Background(), db, teamID, request.UserID, request.Role, request.JobTitle)
}

func (s *LocalService) RemoveTeamMember(teamID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTeamMember(context.Background(), db, teamID, userID)
}

func (s *LocalService) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeamMembers(context.Background(), db, teamID)
}

func (s *LocalService) AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamAgent{}, err
	}
	defer db.Close()
	return store.AddTeamAgent(context.Background(), db, teamID, agentID)
}

func (s *LocalService) RemoveTeamAgent(teamID int64, agentID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTeamAgent(context.Background(), db, teamID, agentID)
}

func (s *LocalService) ListTeamAgents(teamID int64) ([]store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTeamAgents(context.Background(), db, teamID)
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
	ticket, err := store.CreateTicket(context.Background(), db, store.TicketCreateParams{
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
		Author:             user.Username,
		CreatedBy:          user.ID,
	})
	if err != nil {
		return ticket, err
	}
	if request.Message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, request.Message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
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
	return store.ListTickets(context.Background(), db, store.TicketListParams{
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
	ticket, err := store.UpdateTicket(context.Background(), db, id, store.TicketUpdateParams{
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
		Type:               request.Type,
		UpdatedBy:          user.ID,
		ActorUsername:      user.Username,
		// Local mode bypasses server-side ownership restrictions.
		ActorRole: "admin",
	})
	if err != nil {
		return ticket, err
	}
	if request.Message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, request.Message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) CloseTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	// Add comment before close — AddComment rejects closed tickets.
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, id, user.ID, message); err != nil {
			return store.Ticket{}, err
		}
	}
	return store.SetTicketComplete(context.Background(), db, id, true, user.Username, user.ID)
}

func (s *LocalService) OpenTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketComplete(context.Background(), db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ArchiveTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	// Add comment before archive — AddComment rejects archived tickets.
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, id, user.ID, message); err != nil {
			return store.Ticket{}, err
		}
	}
	return store.SetTicketArchived(context.Background(), db, id, true, user.Username, user.ID)
}

func (s *LocalService) UnarchiveTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketArchived(context.Background(), db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ReadyTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketDraft(context.Background(), db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) NotReadyTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketDraft(context.Background(), db, id, true, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) SetTicketSdlc(id string, sdlcID int64) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.SetTicketSdlc(context.Background(), db, id, sdlcID)
}

func (s *LocalService) UnsetTicketSdlc(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.UnsetTicketSdlc(context.Background(), db, id)
}

func (s *LocalService) DeleteTicket(id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTicket(context.Background(), db, id)
}

func (s *LocalService) SetTicketParent(id string, parentID string, message string) (store.Ticket, error) {
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
		Message:            message,
	})
}

func (s *LocalService) SetTicketHealth(id string, score int) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.SetTicketHealth(context.Background(), db, id, score)
}

func (s *LocalService) UnsetTicketParent(id string, message string) (store.Ticket, error) {
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
		Message:            message,
	})
}

func (s *LocalService) GetTicketByID(id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicket(context.Background(), db, id)
}

func (s *LocalService) GetTicket(ref string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicketByRef(context.Background(), db, ref)
}

func (s *LocalService) CloneTicket(id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.CloneTicket(context.Background(), db, id, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ListHistory(id string) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListHistoryEvents(context.Background(), db, id)
}

func (s *LocalService) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.ListProjectHistoryFiltered(projectID, limit, store.HistoryFilter{})
}

func (s *LocalService) ListProjectHistoryFiltered(projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjectHistoryFiltered(context.Background(), db, projectID, limit, filter)
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
	return store.AddComment(context.Background(), db, id, user.ID, comment)
}

func (s *LocalService) ListComments(id string) ([]store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListComments(context.Background(), db, id)
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
	return store.AddDependency(context.Background(), db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
}

func (s *LocalService) RemoveDependency(request DependencyRequest) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteDependency(context.Background(), db, request.ProjectID, request.TicketID, request.DependsOn)
}

func (s *LocalService) ListDependencies(id string) ([]store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListDependencies(context.Background(), db, id)
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
	ticket, status, err := store.RequestTicket(context.Background(), db, store.TicketRequestParams{
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
		ctx := store.EnrichTicketContext(context.Background(), db, ticket)
		response.Project = ctx.Project
		response.Parents = ctx.Parents
		response.Sdlc = ctx.Sdlc
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
	if user, err := store.GetUserByUsername(context.Background(), db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(context.Background(), db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(context.Background(), db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	return store.CreateUser(context.Background(), db, username, "local-mode", "admin")
}

func LocalUsername() string {
	return "admin"
}

func (s *LocalService) CreateSdlc(request SdlcRequest) (store.Sdlc, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Sdlc{}, err
	}
	defer db.Close()
	return store.CreateSdlc(context.Background(), db, request.Name, request.Description)
}

func (s *LocalService) ListSdlcs() ([]store.Sdlc, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListSdlcs(context.Background(), db)
}

func (s *LocalService) GetSdlc(id int64) (store.SdlcWithStages, error) {
	db, err := s.openDB()
	if err != nil {
		return store.SdlcWithStages{}, err
	}
	defer db.Close()
	return store.GetSdlc(context.Background(), db, id)
}

func (s *LocalService) DeleteSdlc(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteSdlc(context.Background(), db, id)
}

func (s *LocalService) AddSdlcStage(sdlcID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	db, err := s.openDB()
	if err != nil {
		return store.SdlcStage{}, err
	}
	defer db.Close()
	return store.AddSdlcStage(context.Background(), db, sdlcID, request.StageName, request.Description, request.SortOrder)
}

func (s *LocalService) RemoveSdlcStage(stageID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveSdlcStage(context.Background(), db, stageID)
}

func (s *LocalService) ReorderSdlcStages(sdlcID int64, stageIDs []int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.ReorderSdlcStages(context.Background(), db, sdlcID, stageIDs)
}

func (s *LocalService) ExportSdlc(id int64) (store.SdlcExport, error) {
	db, err := s.openDB()
	if err != nil {
		return store.SdlcExport{}, err
	}
	defer db.Close()
	return store.ExportSdlc(context.Background(), db, id)
}

func (s *LocalService) ImportSdlc(export store.SdlcExport) (store.Sdlc, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Sdlc{}, err
	}
	defer db.Close()
	return store.ImportSdlc(context.Background(), db, export)
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
	return store.LogTime(context.Background(), db, ticketID, user.ID, request.Minutes, request.Note)
}

func (s *LocalService) ListTimeEntries(ticketID string) ([]store.TimeEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTimeEntries(context.Background(), db, ticketID)
}

func (s *LocalService) DeleteTimeEntry(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTimeEntry(context.Background(), db, id)
}

func (s *LocalService) TotalTimeForTicket(ticketID string) (int, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return store.TotalTimeForTicket(context.Background(), db, ticketID)
}

func (s *LocalService) CreateLabel(projectID int64, request LabelRequest) (store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Label{}, err
	}
	defer db.Close()
	return store.CreateLabel(context.Background(), db, projectID, request.Name, request.Color)
}

func (s *LocalService) ListLabels(projectID int64) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListLabels(context.Background(), db, projectID)
}

func (s *LocalService) DeleteLabel(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteLabel(context.Background(), db, id)
}

func (s *LocalService) AddTicketLabel(ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.AddTicketLabel(context.Background(), db, ticketID, labelID)
}

func (s *LocalService) RemoveTicketLabel(ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.RemoveTicketLabel(context.Background(), db, ticketID, labelID)
}

func (s *LocalService) ListTicketLabels(ticketID string) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTicketLabels(context.Background(), db, ticketID)
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
	return store.CreateStory(context.Background(), db, projectID, title, description, user.ID)
}

func (s *LocalService) ListStories(projectID int64) ([]store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListStoriesByProject(context.Background(), db, projectID)
}

func (s *LocalService) GetStory(id int64) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	defer db.Close()
	return store.GetStory(context.Background(), db, id)
}

func (s *LocalService) UpdateStory(id int64, title, description string) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	defer db.Close()
	return store.UpdateStory(context.Background(), db, id, title, description)
}

func (s *LocalService) DeleteStory(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteStory(context.Background(), db, id)
}
