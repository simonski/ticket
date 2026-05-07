// Package libticket provides the core service interface and LocalService implementation
// for interacting with ticket data. Both local (SQLite) and remote (HTTP) implementations
// satisfy the Service interface, enabling identical behaviour regardless of deployment mode.
package libticket

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

// LocalService implements Service directly against a local SQLite database.
// Obtain one via NewLocal after loading a config with a local location.
type LocalService struct {
	cfg config.Config

	dbMu sync.Mutex
	db   *sql.DB
}

// resolveRequestLifecycle derives the canonical stage+state pair from the three
// possible ways a caller may express lifecycle: explicit stage/state flags, a
// rendered status string (e.g. "design/active"), or nothing (no-op).
func resolveRequestLifecycle(status, stage, state string) (resolvedStage, resolvedState string, err error) {
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

func (s *LocalService) resolvedLocation() (config.Resolved, error) {
	if strings.TrimSpace(s.cfg.Location) != "" {
		return config.ResolveLocation(s.cfg.Location)
	}
	return config.ResolveURL()
}

func (s *LocalService) Status(ctx context.Context) (StatusResponse, error) {
	resolved, err := s.resolvedLocation()
	if err != nil {
		return StatusResponse{}, err
	}
	path := resolved.DBPath
	_, statErr := os.Stat(path)
	if statErr != nil {
		return StatusResponse{}, statErr
	}
	db, err := s.openDB()
	if err != nil {
		return StatusResponse{}, err
	}
	user, err := store.GetUserByUsername(ctx, db, LocalUsername())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		enabled, regErr := store.RegistrationEnabled(ctx, db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	case err != nil:
		return StatusResponse{}, err
	case !user.Enabled:
		enabled, regErr := store.RegistrationEnabled(ctx, db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: enabled}, nil
	}
	enabled, err := store.RegistrationEnabled(ctx, db)
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

func (s *LocalService) SetRegistrationEnabled(ctx context.Context, enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.SetRegistrationEnabled(ctx, db, enabled)
}

func (s *LocalService) Register(ctx context.Context, username, password string) (store.User, error) {
	return store.User{}, errors.New("ticket register requires remote mode (run tk init to configure)")
}

func (s *LocalService) Login(ctx context.Context, username, password string) (store.User, string, error) {
	return store.User{}, "", errors.New("ticket login requires remote mode (run tk init to configure)")
}

func (s *LocalService) Logout(ctx context.Context) error {
	return errors.New("ticket logout requires remote mode (run tk init to configure)")
}

func (s *LocalService) Count(ctx context.Context, projectID *int64) (CountSummary, error) {
	db, err := s.openDB()
	if err != nil {
		return CountSummary{}, err
	}
	return store.CountEverything(ctx, db, projectID)
}

func (s *LocalService) CreateUser(ctx context.Context, username, password string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	return store.CreateUser(ctx, db, username, password, "user")
}

func (s *LocalService) SetUserEnabled(ctx context.Context, username string, enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.SetUserEnabled(ctx, db, username, enabled)
}

func (s *LocalService) ListUsers(ctx context.Context) ([]store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListUsers(ctx, db, 0)
}

func (s *LocalService) DeleteUser(ctx context.Context, username string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteUser(ctx, db, username)
}

func (s *LocalService) ResetUserPassword(ctx context.Context, username, newPassword string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	return store.ResetUserPassword(ctx, db, username, newPassword)
}

func (s *LocalService) CreateRole(ctx context.Context, request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	return store.CreateRoleWithParams(ctx, db, store.RoleCreateParams{
		ID:                 request.ID,
		WorkflowID:         request.WorkflowID,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
	})
}

func (s *LocalService) ListRoles(ctx context.Context) ([]store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListRoles(ctx, db, 0)
}

func (s *LocalService) UpdateRole(ctx context.Context, id int64, request RoleRequest) (store.Role, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Role{}, err
	}
	return store.UpdateRoleWithParams(ctx, db, id, store.RoleUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
	})
}

func (s *LocalService) DeleteRole(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteRole(ctx, db, id)
}

func (s *LocalService) CreateAgent(ctx context.Context, request AgentCreateRequest) (agent store.Agent, password string, err error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, "", err
	}
	return store.CreateAgent(ctx, db, request.Password)
}

func (s *LocalService) SetAgentEnabled(ctx context.Context, id string, enabled bool) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	return store.SetAgentEnabled(ctx, db, id, enabled)
}

func (s *LocalService) ListAgents(ctx context.Context) ([]store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListAgents(ctx, db)
}

func (s *LocalService) ListAgentStatuses(ctx context.Context) ([]store.AgentStatus, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListAgentStatuses(ctx, db)
}

func (s *LocalService) UpdateAgent(ctx context.Context, id string, request AgentUpdateRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	return store.UpdateAgent(ctx, db, id, store.AgentUpdateParams{
		Password: request.Password,
	})
}

func (s *LocalService) DeleteAgent(ctx context.Context, id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteAgent(ctx, db, id)
}

func (s *LocalService) SetAgentConfig(ctx context.Context, agentID, key, value string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.SetAgentConfig(ctx, db, agentID, key, value)
}

func (s *LocalService) ListAgentConfig(ctx context.Context, agentID string) ([]store.AgentConfigEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListAgentConfig(ctx, db, agentID)
}

func (s *LocalService) DeleteAgentConfig(ctx context.Context, agentID, key string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteAgentConfig(ctx, db, agentID, key)
}

func (s *LocalService) RegisterAgent(ctx context.Context, request AgentRegisterRequest) (store.Agent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Agent{}, err
	}
	agent, err := store.AuthenticateAgent(ctx, db, request.ID, request.Password)
	if err != nil {
		return store.Agent{}, err
	}
	return store.TouchAgent(ctx, db, agent.ID, "online")
}

func (s *LocalService) HeartbeatAgent(ctx context.Context, agentID, password, status string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	agent, err := store.AuthenticateAgent(ctx, db, agentID, password)
	if err != nil {
		return err
	}
	_, err = store.TouchAgent(ctx, db, agent.ID, status)
	return err
}

func (s *LocalService) RequestAgentWork(ctx context.Context, request AgentRequest) (AgentWorkResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return AgentWorkResponse{}, err
	}
	agent, err := store.AuthenticateAgent(ctx, db, request.ID, request.Password)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	projectID := request.ProjectID
	if request.TicketID != nil {
		projectID = 0
	}
	if request.TicketID == nil && projectID == 0 {
		projects, listErr := store.ListProjects(ctx, db, 0)
		if listErr != nil {
			return AgentWorkResponse{}, listErr
		}
		for _, p := range projects {
			if p.Status == "open" {
				projectID = p.ID
				break
			}
		}
	}
	currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(ctx, db, projectID, agent.Username)
	if err != nil {
		return AgentWorkResponse{}, err
	}
	ticket, status, err := store.RequestTicket(ctx, db, store.TicketRequestParams{
		ProjectID: projectID,
		TicketID:  request.TicketID,
		Username:  agent.Username,
		UserID:    "",
		DryRun:    request.DryRun,
	})
	if err != nil {
		return AgentWorkResponse{}, err
	}
	var agentStatus string
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
		if _, err := store.TouchAgent(ctx, db, agent.ID, "working"); err != nil {
			log.Printf("libticket: touch agent %s status=working: %v", agent.ID, err)
		}
	} else {
		if _, err := store.TouchAgent(ctx, db, agent.ID, "soliciting"); err != nil {
			log.Printf("libticket: touch agent %s status=soliciting: %v", agent.ID, err)
		}
	}
	response := AgentWorkResponse{Status: agentStatus, Parents: []store.Ticket{}}
	if agentStatus == "NEW" || agentStatus == "CURRENT" {
		project, err := store.GetProjectByID(ctx, db, ticket.ProjectID)
		if err == nil {
			response.Project = &project
		}
		response.Ticket = &ticket
		enriched := store.EnrichTicketContext(ctx, db, ticket)
		response.Workflow = enriched.Workflow
		response.Workflow = enriched.Workflow
		response.Role = enriched.Role
		parents, err := store.ListTicketParents(ctx, db, ticket.ID)
		if err == nil {
			response.Parents = parents
		}
	}
	return response, nil
}

func (s *LocalService) AgentUpdateTicket(ctx context.Context, id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	agent, err := store.AuthenticateAgent(ctx, db, request.ID, request.Password)
	if err != nil {
		return store.Ticket{}, err
	}
	current, err := store.GetTicket(ctx, db, id)
	if err != nil {
		return store.Ticket{}, err
	}
	updated, err := store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
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
	if _, err := store.TouchAgent(ctx, db, agent.ID, "soliciting"); err != nil {
		log.Printf("libticket: touch agent %s status=soliciting: %v", agent.ID, err)
	}
	return updated, nil
}

func (s *LocalService) CreateProject(ctx context.Context, request ProjectCreateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Project{}, err
	}
	return store.CreateProjectWithParams(ctx, db, store.ProjectCreateParams{
		ID:                 request.ID,
		Prefix:             request.Prefix,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
		GitRepository:      request.GitRepository,
		Notes:              request.Notes,
		Visibility:         request.Visibility,
		CreatedBy:          user.ID,
		WorkflowID:         request.WorkflowID,
	})
}

func (s *LocalService) ListProjects(ctx context.Context) ([]store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListProjects(ctx, db, 0)
}

func (s *LocalService) GetProject(ctx context.Context, id string) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	return store.GetProject(ctx, db, id)
}

func (s *LocalService) UpdateProject(ctx context.Context, id int64, request ProjectUpdateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	return store.UpdateProjectWithParams(ctx, db, id, store.ProjectUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
		GitRepository:      request.GitRepository,
		Notes:              request.Notes,
		Status:             request.Status,
		Visibility:         request.Visibility,
		WorkflowID:         request.WorkflowID,
	})
}

func (s *LocalService) DeleteProject(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteProject(ctx, db, id)
}

func (s *LocalService) RenameProjectPrefix(ctx context.Context, id int64, newPrefix string) (int, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	return store.RenameProjectPrefix(ctx, db, id, newPrefix)
}

func (s *LocalService) SetProjectEnabled(ctx context.Context, id int64, enabled bool) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	return store.SetProjectStatus(ctx, db, id, enabled)
}

func (s *LocalService) SetProjectDefaultDraft(ctx context.Context, projectID int64, draft bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.SetProjectDefaultDraft(ctx, db, projectID, draft)
}

func (s *LocalService) AddProjectMember(ctx context.Context, projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectMember{}, err
	}
	return store.AddProjectMember(ctx, db, projectID, request.UserID, request.Role)
}

func (s *LocalService) RemoveProjectMember(ctx context.Context, projectID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveProjectMember(ctx, db, projectID, userID)
}

func (s *LocalService) ListProjectMembers(ctx context.Context, projectID int64) ([]store.ProjectMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListProjectMembers(ctx, db, projectID)
}

func (s *LocalService) AddProjectTeamMember(ctx context.Context, projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.ProjectTeamMember{}, err
	}
	return store.AddProjectTeamMember(ctx, db, projectID, request.TeamID, request.Role)
}

func (s *LocalService) RemoveProjectTeamMember(ctx context.Context, projectID, teamID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveProjectTeamMember(ctx, db, projectID, teamID)
}

func (s *LocalService) ListProjectTeamMembers(ctx context.Context, projectID int64) ([]store.ProjectTeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListProjectTeamMembers(ctx, db, projectID)
}

func (s *LocalService) CreateTeam(ctx context.Context, request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	return store.CreateTeamWithParams(ctx, db, store.TeamCreateParams{
		ID:           request.ID,
		Name:         request.Name,
		ParentTeamID: request.ParentTeamID,
	})
}

func (s *LocalService) ListTeams(ctx context.Context) ([]store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTeams(ctx, db, 0)
}

func (s *LocalService) UpdateTeam(ctx context.Context, id int64, request TeamRequest) (store.Team, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Team{}, err
	}
	return store.UpdateTeam(ctx, db, id, request.Name, request.ParentTeamID)
}

func (s *LocalService) DeleteTeam(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteTeam(ctx, db, id)
}

func (s *LocalService) AddTeamMember(ctx context.Context, teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamMember{}, err
	}
	return store.AddTeamMember(ctx, db, teamID, request.UserID, request.Role, request.JobTitle)
}

func (s *LocalService) RemoveTeamMember(ctx context.Context, teamID int64, userID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveTeamMember(ctx, db, teamID, userID)
}

func (s *LocalService) ListTeamMembers(ctx context.Context, teamID int64) ([]store.TeamMember, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTeamMembers(ctx, db, teamID)
}

func (s *LocalService) AddTeamAgent(ctx context.Context, teamID int64, agentID string) (store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TeamAgent{}, err
	}
	return store.AddTeamAgent(ctx, db, teamID, agentID)
}

func (s *LocalService) RemoveTeamAgent(ctx context.Context, teamID int64, agentID string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveTeamAgent(ctx, db, teamID, agentID)
}

func (s *LocalService) ListTeamAgents(ctx context.Context, teamID int64) ([]store.TeamAgent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTeamAgents(ctx, db, teamID)
}

func (s *LocalService) CreateTicket(ctx context.Context, request TicketCreateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	_, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	ticket, err := store.CreateTicket(ctx, db, store.TicketCreateParams{
		ProjectID:          request.ProjectID,
		ParentID:           request.ParentID,
		CloneOf:            request.CloneOf,
		Type:               request.Type,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
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
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, request.Message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ListTickets(ctx context.Context, projectID int64) ([]store.Ticket, error) {
	return s.ListTicketsFiltered(ctx, projectID, "", "", "", "", "", "", 0, false)
}

func (s *LocalService) ListTicketsFiltered(ctx context.Context, projectID int64, ticketType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTickets(ctx, db, store.TicketListParams{
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

func (s *LocalService) UpdateTicket(ctx context.Context, id string, request TicketUpdateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	stage, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	ticket, err := store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		DORMap:             request.DORMap,
		DODMap:             request.DODMap,
		ACMap:              request.ACMap,
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
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, request.Message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) CloseTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	// Add comment before close — AddComment rejects closed tickets.
	if message != "" {
		if _, err := store.AddComment(ctx, db, id, user.ID, message); err != nil {
			return store.Ticket{}, err
		}
	}
	return store.SetTicketComplete(ctx, db, id, true, user.Username, user.ID)
}

func (s *LocalService) OpenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketComplete(ctx, db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ArchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	// Add comment before archive — AddComment rejects archived tickets.
	if message != "" {
		if _, err := store.AddComment(ctx, db, id, user.ID, message); err != nil {
			return store.Ticket{}, err
		}
	}
	return store.SetTicketArchived(ctx, db, id, true, user.Username, user.ID)
}

func (s *LocalService) UnarchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketArchived(ctx, db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketDraft(ctx, db, id, false, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) NotReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.SetTicketDraft(ctx, db, id, true, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) SetTicketWorkflow(ctx context.Context, id string, workflowID int64) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketWorkflow(ctx, db, id, workflowID)
}

func (s *LocalService) UnsetTicketWorkflow(ctx context.Context, id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	return store.UnsetTicketWorkflow(ctx, db, id)
}

func (s *LocalService) DeleteTicket(ctx context.Context, id string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteTicket(ctx, db, id)
}

func (s *LocalService) SetTicketParent(ctx context.Context, id, parentID, message string) (store.Ticket, error) {
	current, err := s.GetTicketByID(ctx, id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(ctx, id, TicketUpdateRequest{
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

func (s *LocalService) SetTicketHealth(ctx context.Context, id string, score int) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	return store.SetTicketHealth(ctx, db, id, score)
}

func (s *LocalService) UnsetTicketParent(ctx context.Context, id string, message string) (store.Ticket, error) {
	current, err := s.GetTicketByID(ctx, id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(ctx, id, TicketUpdateRequest{
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

func (s *LocalService) GetTicketByID(ctx context.Context, id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	return store.GetTicket(ctx, db, id)
}

func (s *LocalService) GetTicket(ctx context.Context, ref string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	return store.GetTicketByRef(ctx, db, ref)
}

func (s *LocalService) CloneTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	ticket, err := store.CloneTicket(ctx, db, id, user.Username, user.ID)
	if err != nil {
		return ticket, err
	}
	if message != "" {
		if _, err := store.AddComment(ctx, db, ticket.ID, user.ID, message); err != nil {
			return ticket, err
		}
	}
	return ticket, nil
}

func (s *LocalService) ListHistory(ctx context.Context, id string) ([]store.HistoryEvent, error) {
	return s.ListHistoryPaged(ctx, id, 0, 0)
}

func (s *LocalService) ListHistoryPaged(ctx context.Context, id string, limit, offset int) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListHistoryEvents(ctx, db, id, limit, offset)
}

func (s *LocalService) ListProjectHistory(ctx context.Context, projectID int64, limit int) ([]store.HistoryEvent, error) {
	return s.ListProjectHistoryFiltered(ctx, projectID, limit, store.HistoryFilter{})
}

func (s *LocalService) ListProjectHistoryFiltered(ctx context.Context, projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListProjectHistoryFiltered(ctx, db, projectID, limit, filter)
}

func (s *LocalService) AddComment(ctx context.Context, id, comment string) (store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Comment{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Comment{}, err
	}
	return store.AddComment(ctx, db, id, user.ID, comment)
}

func (s *LocalService) ListComments(ctx context.Context, id string) ([]store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListComments(ctx, db, id)
}

func (s *LocalService) AddDependency(ctx context.Context, request DependencyRequest) (store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Dependency{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Dependency{}, err
	}
	return store.AddDependency(ctx, db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
}

func (s *LocalService) RemoveDependency(ctx context.Context, request DependencyRequest) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteDependency(ctx, db, request.ProjectID, request.TicketID, request.DependsOn)
}

func (s *LocalService) ListDependencies(ctx context.Context, id string) ([]store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListDependencies(ctx, db, id)
}

func (s *LocalService) RequestTicket(ctx context.Context, request TicketRequest) (TicketRequestResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return TicketRequestResponse{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	ticket, status, err := store.RequestTicket(ctx, db, store.TicketRequestParams{
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
		ctx := store.EnrichTicketContext(ctx, db, ticket)
		response.Project = ctx.Project
		response.Parents = ctx.Parents
		response.Workflow = ctx.Workflow
		response.Workflow = ctx.Workflow
		response.Role = ctx.Role
	}
	return response, nil
}

func (s *LocalService) InterveneTicket(ctx context.Context, id string, request InterventionRequest) (InterventionResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return InterventionResponse{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return InterventionResponse{}, err
	}
	current, err := store.GetTicket(ctx, db, id)
	if err != nil {
		return InterventionResponse{}, err
	}
	if strings.TrimSpace(strings.ToLower(current.State)) != store.StateFail {
		return InterventionResponse{}, errors.New("ticket must be in fail state to intervene")
	}

	outcome := strings.TrimSpace(strings.ToLower(request.Outcome))
	if outcome == "" {
		return InterventionResponse{}, errors.New("outcome is required")
	}

	var ticket store.Ticket
	var followUp *store.Ticket
	switch outcome {
	case "retry-role":
		ticket, err = store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
			Title:              current.Title,
			Description:        current.Description,
			AcceptanceCriteria: current.AcceptanceCriteria,
			DORMap:             current.DORMap,
			DODMap:             current.DODMap,
			ACMap:              current.ACMap,
			GitRepository:      current.GitRepository,
			GitBranch:          current.GitBranch,
			ParentID:           current.ParentID,
			Assignee:           current.Assignee,
			Stage:              current.Stage,
			State:              store.StateIdle,
			Priority:           current.Priority,
			Order:              current.Order,
			EstimateEffort:     current.EstimateEffort,
			EstimateComplete:   current.EstimateComplete,
			Type:               current.Type,
			UpdatedBy:          user.ID,
			ActorUsername:      user.Username,
			ActorRole:          user.Role,
		})
	case "retry-stage":
		ticket, err = store.PreviousTicket(ctx, db, id, user.Username, user.ID)
	case "split-work":
		followUpTicket, createErr := store.CreateTicket(ctx, db, store.TicketCreateParams{
			ProjectID:          current.ProjectID,
			Type:               "task",
			Title:              "Follow-up: " + strings.TrimSpace(current.Title),
			Description:        strings.TrimSpace("Created from intervention on " + current.ID + ".\n\n" + request.Message),
			AcceptanceCriteria: current.AcceptanceCriteria,
			DORMap:             current.DORMap,
			DODMap:             current.DODMap,
			ACMap:              current.ACMap,
			GitRepository:      current.GitRepository,
			GitBranch:          current.GitBranch,
			Priority:           current.Priority,
			EstimateEffort:     current.EstimateEffort,
			EstimateComplete:   "",
			Author:             user.Username,
			CreatedBy:          user.ID,
		})
		if createErr != nil {
			return InterventionResponse{}, createErr
		}
		followUp = &followUpTicket
		ticket, err = store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
			Title:              current.Title,
			Description:        current.Description,
			AcceptanceCriteria: current.AcceptanceCriteria,
			DORMap:             current.DORMap,
			DODMap:             current.DODMap,
			ACMap:              current.ACMap,
			GitRepository:      current.GitRepository,
			GitBranch:          current.GitBranch,
			ParentID:           current.ParentID,
			Assignee:           current.Assignee,
			Stage:              current.Stage,
			State:              store.StateIdle,
			Priority:           current.Priority,
			Order:              current.Order,
			EstimateEffort:     current.EstimateEffort,
			EstimateComplete:   current.EstimateComplete,
			Type:               current.Type,
			UpdatedBy:          user.ID,
			ActorUsername:      user.Username,
			ActorRole:          user.Role,
		})
	case "cancel":
		ticket, err = store.SetTicketArchived(ctx, db, id, true, user.Username, user.ID)
	default:
		return InterventionResponse{}, errors.New("invalid outcome")
	}
	if err != nil {
		return InterventionResponse{}, err
	}
	if strings.TrimSpace(request.Message) != "" {
		_, _ = store.AddComment(ctx, db, ticket.ID, user.ID, request.Message)
	}
	historyPayload := map[string]any{
		"outcome": outcome,
		"who":     user.Username,
		"message": request.Message,
	}
	if followUp != nil {
		historyPayload["follow_up_ticket_id"] = followUp.ID
		historyPayload["follow_up_ticket_key"] = followUp.ID
	}
	if err := store.AddHistoryEvent(ctx, db, ticket.ProjectID, ticket.ID, "ticket_intervention_decided", historyPayload, user.ID); err != nil {
		return InterventionResponse{}, err
	}
	return InterventionResponse{
		Ticket:       ticket,
		FollowUp:     followUp,
		Decision:     outcome,
		Intervention: true,
	}, nil
}

func (s *LocalService) openDB() (*sql.DB, error) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db != nil {
		return s.db, nil
	}
	resolved, err := s.resolvedLocation()
	if err != nil {
		return nil, err
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		return nil, err
	}
	s.db = db
	return s.db, nil
}

func (s *LocalService) localUser(ctx context.Context, db *sql.DB) (store.User, error) {
	username := LocalUsername()
	if user, err := store.GetUserByUsername(ctx, db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if setErr := store.SetUserEnabled(ctx, db, username, true); setErr != nil {
			return store.User{}, setErr
		}
		return store.GetUserByUsername(ctx, db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	return store.CreateUser(ctx, db, username, "local-mode", "admin")
}

func LocalUsername() string {
	return "admin"
}

func (s *LocalService) CreateWorkflow(ctx context.Context, request WorkflowRequest) (store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Workflow{}, err
	}
	return store.CreateWorkflowWithParams(ctx, db, request.ID, request.Name, request.Description)
}

func (s *LocalService) ListWorkflows(ctx context.Context) ([]store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListWorkflows(ctx, db, 0, 0)
}

func (s *LocalService) GetWorkflow(ctx context.Context, id int64) (store.WorkflowWithStages, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowWithStages{}, err
	}
	return store.GetWorkflow(ctx, db, id)
}

func (s *LocalService) DeleteWorkflow(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteWorkflow(ctx, db, id)
}

func (s *LocalService) AddWorkflowStage(ctx context.Context, workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowStage{}, err
	}
	wow := request.WaysOfWorking
	if strings.TrimSpace(wow) == "" {
		wow = request.Description
	}
	dor := request.DefinitionOfReady
	if strings.TrimSpace(dor) == "" {
		dor = request.AcceptanceCriteria
	}
	return store.AddWorkflowStageWithDefinitions(ctx, db, workflowID, request.StageName, wow, dor, request.DefinitionOfDone, request.SortOrder)
}

func (s *LocalService) UpdateWorkflowStage(ctx context.Context, stageID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowStage{}, err
	}
	wow := request.WaysOfWorking
	if strings.TrimSpace(wow) == "" {
		wow = request.Description
	}
	dor := request.DefinitionOfReady
	if strings.TrimSpace(dor) == "" {
		dor = request.AcceptanceCriteria
	}
	return store.UpdateWorkflowStageWithDefinitions(ctx, db, stageID, request.StageName, wow, dor, request.DefinitionOfDone)
}

func (s *LocalService) GetWorkflowStage(ctx context.Context, stageID int64) (store.WorkflowStage, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowStage{}, err
	}
	return store.GetWorkflowStage(ctx, db, stageID)
}

func (s *LocalService) ListWorkflowStages(ctx context.Context, workflowID int64) ([]store.WorkflowStage, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListWorkflowStages(ctx, db, workflowID)
}

func (s *LocalService) RemoveWorkflowStage(ctx context.Context, stageID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveWorkflowStage(ctx, db, stageID)
}

func (s *LocalService) ReorderWorkflowStages(ctx context.Context, workflowID int64, stageIDs []int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.ReorderWorkflowStages(ctx, db, workflowID, stageIDs)
}

func (s *LocalService) ExportWorkflow(ctx context.Context, id int64) (store.WorkflowExport, error) {
	db, err := s.openDB()
	if err != nil {
		return store.WorkflowExport{}, err
	}
	return store.ExportWorkflow(ctx, db, id)
}

func (s *LocalService) ImportWorkflow(ctx context.Context, export store.WorkflowExport) (store.Workflow, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Workflow{}, err
	}
	return store.ImportWorkflow(ctx, db, export)
}

func (s *LocalService) AddWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.AddWorkflowStageRole(ctx, db, workflowID, stageID, roleID)
}

func (s *LocalService) RemoveWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveWorkflowStageRole(ctx, db, workflowID, stageID, roleID)
}

func (s *LocalService) ReorderWorkflowStageRoles(ctx context.Context, workflowID, stageID int64, roleIDs []int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.ReorderWorkflowStageRoles(ctx, db, workflowID, stageID, roleIDs)
}

func (s *LocalService) CompleteTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.CloseTicket(ctx, id, message)
}

func (s *LocalService) ReopenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.OpenTicket(ctx, id, message)
}

func (s *LocalService) DraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.NotReadyTicket(ctx, id, message)
}

func (s *LocalService) UndraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return s.ReadyTicket(ctx, id, message)
}

func (s *LocalService) NextTicket(ctx context.Context, id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.NextTicket(ctx, db, id, user.Username, user.ID)
}

func (s *LocalService) PreviousTicket(ctx context.Context, id string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.PreviousTicket(ctx, db, id, user.Username, user.ID)
}

func (s *LocalService) LogTime(ctx context.Context, ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return store.TimeEntry{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.TimeEntry{}, err
	}
	return store.LogTime(ctx, db, ticketID, user.ID, request.Minutes, request.Note)
}

func (s *LocalService) ListTimeEntries(ctx context.Context, ticketID string) ([]store.TimeEntry, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTimeEntries(ctx, db, ticketID)
}

func (s *LocalService) DeleteTimeEntry(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteTimeEntry(ctx, db, id)
}

func (s *LocalService) TotalTimeForTicket(ctx context.Context, ticketID string) (int, error) {
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	return store.TotalTimeForTicket(ctx, db, ticketID)
}

func (s *LocalService) CreateLabel(ctx context.Context, projectID int64, request LabelRequest) (store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Label{}, err
	}
	return store.CreateLabelWithParams(ctx, db, store.LabelCreateParams{
		ID:        request.ID,
		ProjectID: projectID,
		Name:      request.Name,
		Color:     request.Color,
	})
}

func (s *LocalService) ListLabels(ctx context.Context, projectID int64) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListLabels(ctx, db, projectID, 0, 0)
}

func (s *LocalService) DeleteLabel(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteLabel(ctx, db, id)
}

func (s *LocalService) AddTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.AddTicketLabel(ctx, db, ticketID, labelID)
}

func (s *LocalService) RemoveTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.RemoveTicketLabel(ctx, db, ticketID, labelID)
}

func (s *LocalService) ListTicketLabels(ctx context.Context, ticketID string) ([]store.Label, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListTicketLabels(ctx, db, ticketID)
}

func (s *LocalService) CreateStory(ctx context.Context, projectID int64, title, description string) (store.Story, error) {
	return s.CreateStoryWithRequest(ctx, StoryCreateRequest{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
	})
}

func (s *LocalService) CreateStoryWithRequest(ctx context.Context, request StoryCreateRequest) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	user, err := s.localUser(ctx, db)
	if err != nil {
		return store.Story{}, err
	}
	return store.CreateStoryWithParams(ctx, db, store.StoryCreateParams{
		ID:          request.ID,
		ProjectID:   request.ProjectID,
		Title:       request.Title,
		Description: request.Description,
		CreatedBy:   user.ID,
	})
}

func (s *LocalService) ListStories(ctx context.Context, projectID int64) ([]store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	return store.ListStoriesByProject(ctx, db, projectID, 0, 0)
}

func (s *LocalService) GetStory(ctx context.Context, id int64) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	return store.GetStory(ctx, db, id)
}

func (s *LocalService) UpdateStory(ctx context.Context, id int64, title, description string) (store.Story, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Story{}, err
	}
	return store.UpdateStory(ctx, db, id, title, description)
}

func (s *LocalService) DeleteStory(ctx context.Context, id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	return store.DeleteStory(ctx, db, id)
}
