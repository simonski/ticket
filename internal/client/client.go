package client

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	mode    string

	localDBMu sync.Mutex
	localDB   *sql.DB
}

func New(cfg config.Config) *Client {
	resolved, err := config.ResolveLocation(cfg.Location)
	if err != nil {
		resolved = config.Resolved{Mode: config.ModeLocal}
	}
	baseURL := strings.TrimRight(resolved.ServerURL, "/")
	timeout := 30 * time.Second
	if resolved.Mode == config.ModeRemote {
		timeout = remoteTimeoutFromEnv()
	}
	return &Client{
		baseURL: baseURL,
		token:   cfg.Token,
		http: &http.Client{
			Timeout: timeout,
		},
		mode: resolved.Mode,
	}
}

func remoteTimeoutFromEnv() time.Duration {
	seconds := 5
	raw := strings.TrimSpace(os.Getenv("TICKET_TIMEOUT"))
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			seconds = parsed
		}
	}
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func (c *Client) Register(ctx context.Context, username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		return store.User{}, errors.New("ticket register requires remote mode (run tk init to configure)")
	}
	var user store.User
	err := c.doJSON(ctx, http.MethodPost, "/api/register", map[string]string{
		"username": username,
		"password": password,
	}, &user)
	return user, err
}

func (c *Client) Login(ctx context.Context, username, password string) (AuthResponse, error) {
	if c.mode == config.ModeLocal {
		return AuthResponse{}, errors.New("ticket login requires remote mode (run tk init to configure)")
	}
	var response AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/login", map[string]string{
		"username": username,
		"password": password,
	}, &response)
	return response, err
}

func (c *Client) Logout(ctx context.Context) error {
	if c.mode == config.ModeLocal {
		return errors.New("ticket logout requires remote mode (run tk init to configure)")
	}
	return c.doJSON(ctx, http.MethodPost, "/api/logout", nil, nil)
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	if c.mode == config.ModeLocal {
		resolved, err := config.ResolveURL()
		if err != nil {
			return StatusResponse{}, err
		}
		if _, err := os.Stat(resolved.DBPath); err != nil {
			return StatusResponse{}, err
		}
		db, err := c.openLocalDB()
		if err != nil {
			return StatusResponse{}, err
		}

		user, err := store.GetUserByUsername(ctx, db, localUsername())
		registrationEnabled, regErr := store.RegistrationEnabled(ctx, db)
		if regErr != nil {
			return StatusResponse{}, regErr
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: registrationEnabled}, nil
		case err != nil:
			return StatusResponse{}, err
		case !user.Enabled:
			return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: registrationEnabled}, nil
		}
		return StatusResponse{
			Status:              "ok",
			Authenticated:       true,
			RegistrationEnabled: registrationEnabled,
			User:                &user,
		}, nil
	}
	var status StatusResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/status", nil, &status)
	return status, err
}

func (c *Client) Count(ctx context.Context, projectID *int64) (CountSummary, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return CountSummary{}, err
		}
		return store.CountEverything(ctx, db, projectID)
	}
	var summary CountSummary
	path := "/api/count"
	if projectID != nil {
		path = fmt.Sprintf("/api/count?project_id=%d", *projectID)
	}
	err := c.doJSON(ctx, http.MethodGet, path, nil, &summary)
	return summary, err
}

func (c *Client) SetRegistrationEnabled(ctx context.Context, enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetRegistrationEnabled(ctx, db, enabled)
	}
	return c.doJSON(ctx, http.MethodPost, "/api/config/registration", map[string]any{"enabled": enabled}, nil)
}

func (c *Client) CreateUser(ctx context.Context, username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.User{}, err
		}
		return store.CreateUser(ctx, db, username, password, "user")
	}
	var user store.User
	err := c.doJSON(ctx, http.MethodPost, "/api/users", map[string]string{
		"username": username,
		"password": password,
	}, &user)
	return user, err
}

func (c *Client) SetUserEnabled(ctx context.Context, username string, enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetUserEnabled(ctx, db, username, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	return c.doJSON(ctx, http.MethodPost, "/api/users/"+username+"/"+action, nil, nil)
}

func (c *Client) ListUsers(ctx context.Context) ([]store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListUsers(ctx, db, 0)
	}
	var users []store.User
	err := c.doJSON(ctx, http.MethodGet, "/api/users", nil, &users)
	return users, err
}

func (c *Client) DeleteUser(ctx context.Context, username string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteUser(ctx, db, username)
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/users/"+username, nil, nil)
}

func (c *Client) ResetUserPassword(ctx context.Context, username, newPassword string) (store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.User{}, err
		}
		return store.ResetUserPassword(ctx, db, username, newPassword)
	}
	var user store.User
	err := c.doJSON(ctx, http.MethodPost, "/api/users/"+username+"/reset-password", map[string]string{"password": newPassword}, &user)
	return user, err
}

func (c *Client) CreateRole(ctx context.Context, request RoleRequest) (store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Role{}, err
		}
		return store.CreateRole(ctx, db, request.SdlcID, request.Title, request.Description, request.AcceptanceCriteria)
	}
	var role store.Role
	err := c.doJSON(ctx, http.MethodPost, "/api/roles", request, &role)
	return role, err
}

func (c *Client) ListRoles(ctx context.Context) ([]store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListRoles(ctx, db, 0)
	}
	var roles []store.Role
	err := c.doJSON(ctx, http.MethodGet, "/api/roles", nil, &roles)
	return roles, err
}

func (c *Client) UpdateRole(ctx context.Context, id int64, request RoleRequest) (store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Role{}, err
		}
		return store.UpdateRole(ctx, db, id, request.Title, request.Description, request.AcceptanceCriteria)
	}
	var role store.Role
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/roles/%d", id), request, &role)
	return role, err
}

func (c *Client) DeleteRole(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteRole(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/roles/%d", id), nil, nil)
}

func (c *Client) CreateAgent(ctx context.Context, request AgentCreateRequest) (store.Agent, string, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, "", err
		}
		return store.CreateAgent(ctx, db, request.Password)
	}
	var response struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/agents", request, &response)
	return response.Agent, response.Password, err
}

func (c *Client) SetAgentEnabled(ctx context.Context, id string, enabled bool) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		return store.SetAgentEnabled(ctx, db, id, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	var agent store.Agent
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/agents/%s/%s", id, action), nil, &agent)
	return agent, err
}

func (c *Client) ListAgents(ctx context.Context) ([]store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListAgents(ctx, db)
	}
	var agents []store.Agent
	err := c.doJSON(ctx, http.MethodGet, "/api/agents", nil, &agents)
	return agents, err
}

func (c *Client) ListAgentStatuses(ctx context.Context) ([]store.AgentStatus, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListAgentStatuses(ctx, db)
	}
	var statuses []store.AgentStatus
	err := c.doJSON(ctx, http.MethodGet, "/api/agents/statuses", nil, &statuses)
	return statuses, err
}

func (c *Client) UpdateAgent(ctx context.Context, id string, request AgentUpdateRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		return store.UpdateAgent(ctx, db, id, store.AgentUpdateParams{
			Password: request.Password,
		})
	}
	var agent store.Agent
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/agents/%s", id), request, &agent)
	return agent, err
}

func (c *Client) DeleteAgent(ctx context.Context, id string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteAgent(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/agents/%s", id), nil, nil)
}

func (c *Client) SetAgentConfig(ctx context.Context, agentID string, key, value string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetAgentConfig(ctx, db, agentID, key, value)
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/agents/%s/config", agentID), map[string]string{"key": key, "value": value}, nil)
}

func (c *Client) ListAgentConfig(ctx context.Context, agentID string) ([]store.AgentConfigEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListAgentConfig(ctx, db, agentID)
	}
	var entries []store.AgentConfigEntry
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/agents/%s/config", agentID), nil, &entries)
	return entries, err
}

func (c *Client) DeleteAgentConfig(ctx context.Context, agentID string, key string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteAgentConfig(ctx, db, agentID, key)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/agents/%s/config/%s", agentID, key), nil, nil)
}

func (c *Client) RegisterAgent(ctx context.Context, request AgentRegisterRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		agent, err := store.AuthenticateAgent(ctx, db, request.ID, request.Password)
		if err != nil {
			return store.Agent{}, err
		}
		return store.TouchAgent(ctx, db, agent.ID, "online")
	}
	var response struct {
		Agent store.Agent `json:"agent"`
	}
	err := c.doJSONBasicAuth(ctx, http.MethodPost, "/api/agents/register", request.ID, request.Password, nil, &response)
	return response.Agent, err
}

func (c *Client) HeartbeatAgent(ctx context.Context, agentID, password, status string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
	var response struct{}
	return c.doJSONBasicAuth(ctx, http.MethodPost, "/api/agents/heartbeat", agentID, password, map[string]string{"status": status}, &response)
}

func (c *Client) RequestAgentWork(ctx context.Context, request AgentRequest) (AgentWorkResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		if projectID == 0 {
			projects, err := store.ListProjects(ctx, db, 0)
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
		response := AgentWorkResponse{Status: agentStatus, Parents: []store.Ticket{}}
		if agentStatus == "NEW" || agentStatus == "CURRENT" {
			project, err := store.GetProjectByID(ctx, db, ticket.ProjectID)
			if err == nil {
				response.Project = &project
			}
			response.Ticket = &ticket
			parents, err := store.ListTicketParents(ctx, db, ticket.ID)
			if err == nil {
				response.Parents = parents
			}
		}
		return response, nil
	}
	var response AgentWorkResponse
	body := map[string]any{
		"project_id": request.ProjectID,
		"dry_run":    request.DryRun,
	}
	if request.TicketID != nil {
		body["ticket_id"] = *request.TicketID
	}
	err := c.doJSONBasicAuth(ctx, http.MethodPost, "/api/agents/request", request.ID, request.Password, body, &response)
	return response, err
}

func (c *Client) AgentUpdateTicket(ctx context.Context, id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		return store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
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
	}
	var ticket store.Ticket
	body := map[string]string{"result": request.Result}
	err := c.doJSONBasicAuth(ctx, http.MethodPost, fmt.Sprintf("/api/agents/tickets/%s/update", id), request.ID, request.Password, body, &ticket)
	return ticket, err
}

func (c *Client) CreateProject(ctx context.Context, request ProjectCreateRequest) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Project{}, err
		}
		return store.CreateProjectWithParams(ctx, db, store.ProjectCreateParams{
			Prefix:             request.Prefix,
			Title:              request.Title,
			Description:        request.Description,
			AcceptanceCriteria: request.AcceptanceCriteria,
			GitRepository:      request.GitRepository,
			Notes:              request.Notes,
			Visibility:         request.Visibility,
			CreatedBy:          user.ID,
			SdlcID:             request.SdlcID,
		})
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodPost, "/api/projects", request, &project)
	return project, err
}

func (c *Client) ListProjects(ctx context.Context) ([]store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListProjects(ctx, db, 0)
	}
	var projects []store.Project
	err := c.doJSON(ctx, http.MethodGet, "/api/projects", nil, &projects)
	return projects, err
}

func (c *Client) GetProject(ctx context.Context, id string) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		return store.GetProject(ctx, db, id)
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodGet, "/api/projects/"+id, nil, &project)
	return project, err
}

func (c *Client) UpdateProject(ctx context.Context, id int64, request ProjectUpdateRequest) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		return store.UpdateProjectWithParams(ctx, db, id, store.ProjectUpdateParams{
			Title:              request.Title,
			Description:        request.Description,
			AcceptanceCriteria: request.AcceptanceCriteria,
			GitRepository:      request.GitRepository,
			Notes:              request.Notes,
			Visibility:         request.Visibility,
			SdlcID:             request.SdlcID,
		})
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/projects/%d", id), request, &project)
	return project, err
}

func (c *Client) SetProjectEnabled(ctx context.Context, id int64, enabled bool) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		return store.SetProjectStatus(ctx, db, id, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/%s", id, action), nil, &project)
	return project, err
}

func (c *Client) SetProjectDefaultDraft(ctx context.Context, projectID int64, draft bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetProjectDefaultDraft(ctx, db, projectID, draft)
	}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/projects/%d/set-draft", projectID), map[string]bool{"draft": draft}, nil)
}

func (c *Client) DeleteProject(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteProject(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), nil, nil)
}

func (c *Client) AddProjectMember(ctx context.Context, projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectMember{}, err
		}
		return store.AddProjectMember(ctx, db, projectID, request.UserID, request.Role)
	}
	var member store.ProjectMember
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/users", projectID), request, &member)
	return member, err
}

func (c *Client) RemoveProjectMember(ctx context.Context, projectID int64, userID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveProjectMember(ctx, db, projectID, userID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%d/users/%s", projectID, userID), nil, nil)
}

func (c *Client) ListProjectMembers(ctx context.Context, projectID int64) ([]store.ProjectMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListProjectMembers(ctx, db, projectID)
	}
	var members []store.ProjectMember
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/users", projectID), nil, &members)
	return members, err
}

func (c *Client) AddProjectTeamMember(ctx context.Context, projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectTeamMember{}, err
		}
		return store.AddProjectTeamMember(ctx, db, projectID, request.TeamID, request.Role)
	}
	var member store.ProjectTeamMember
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/teams", projectID), request, &member)
	return member, err
}

func (c *Client) RemoveProjectTeamMember(ctx context.Context, projectID, teamID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveProjectTeamMember(ctx, db, projectID, teamID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%d/teams/%d", projectID, teamID), nil, nil)
}

func (c *Client) ListProjectTeamMembers(ctx context.Context, projectID int64) ([]store.ProjectTeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListProjectTeamMembers(ctx, db, projectID)
	}
	var members []store.ProjectTeamMember
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/teams", projectID), nil, &members)
	return members, err
}

func (c *Client) CreateTeam(ctx context.Context, request TeamRequest) (store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Team{}, err
		}
		return store.CreateTeam(ctx, db, request.Name, request.ParentTeamID)
	}
	var team store.Team
	err := c.doJSON(ctx, http.MethodPost, "/api/teams", request, &team)
	return team, err
}

func (c *Client) ListTeams(ctx context.Context) ([]store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListTeams(ctx, db, 0)
	}
	var teams []store.Team
	err := c.doJSON(ctx, http.MethodGet, "/api/teams", nil, &teams)
	return teams, err
}

func (c *Client) UpdateTeam(ctx context.Context, id int64, request TeamRequest) (store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Team{}, err
		}
		return store.UpdateTeam(ctx, db, id, request.Name, request.ParentTeamID)
	}
	var team store.Team
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/teams/%d", id), request, &team)
	return team, err
}

func (c *Client) DeleteTeam(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteTeam(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/teams/%d", id), nil, nil)
}

func (c *Client) AddTeamMember(ctx context.Context, teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TeamMember{}, err
		}
		return store.AddTeamMember(ctx, db, teamID, request.UserID, request.Role, request.JobTitle)
	}
	var member store.TeamMember
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/teams/%d/users", teamID), request, &member)
	return member, err
}

func (c *Client) RemoveTeamMember(ctx context.Context, teamID int64, userID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveTeamMember(ctx, db, teamID, userID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/teams/%d/users/%s", teamID, userID), nil, nil)
}

func (c *Client) ListTeamMembers(ctx context.Context, teamID int64) ([]store.TeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListTeamMembers(ctx, db, teamID)
	}
	var members []store.TeamMember
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/teams/%d/users", teamID), nil, &members)
	return members, err
}

func (c *Client) AddTeamAgent(ctx context.Context, teamID int64, agentID string) (store.TeamAgent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TeamAgent{}, err
		}
		return store.AddTeamAgent(ctx, db, teamID, agentID)
	}
	var item store.TeamAgent
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/teams/%d/agents", teamID), map[string]string{"agent_id": agentID}, &item)
	return item, err
}

func (c *Client) RemoveTeamAgent(ctx context.Context, teamID int64, agentID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveTeamAgent(ctx, db, teamID, agentID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/teams/%d/agents/%s", teamID, agentID), nil, nil)
}

func (c *Client) ListTeamAgents(ctx context.Context, teamID int64) ([]store.TeamAgent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListTeamAgents(ctx, db, teamID)
	}
	var items []store.TeamAgent
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/teams/%d/agents", teamID), nil, &items)
	return items, err
}

func (c *Client) CreateTicket(ctx context.Context, request TicketCreateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
			GitRepository:      request.GitRepository,
			GitBranch:          request.GitBranch,
			Priority:           request.Priority,
			EstimateEffort:     request.EstimateEffort,
			EstimateComplete:   request.EstimateComplete,
			Assignee:           request.Assignee,
			Author:             user.Username,
			State:              state,
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
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets", request, &ticket)
	return ticket, err
}

func (c *Client) ListTickets(ctx context.Context, projectID int64) ([]store.Ticket, error) {
	return c.ListTicketsFiltered(ctx, projectID, "", "", "", "", "", "", 0, false)
}

func (c *Client) ListTicketsFiltered(ctx context.Context, projectID int64, ticketType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
	var tickets []store.Ticket
	values := url.Values{}
	if ticketType != "" {
		values.Set("type", ticketType)
	}
	if stage != "" {
		values.Set("stage", stage)
	}
	if state != "" {
		values.Set("state", state)
	}
	if status != "" {
		values.Set("status", status)
	}
	if search != "" {
		values.Set("q", search)
	}
	if assignee != "" {
		values.Set("assignee", assignee)
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	if includeArchived {
		values.Set("include_archived", "1")
	}
	path := fmt.Sprintf("/api/projects/%d/tickets", projectID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	err := c.doJSON(ctx, http.MethodGet, path, nil, &tickets)
	return tickets, err
}

func (c *Client) UpdateTicket(ctx context.Context, id string, request TicketUpdateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Ticket{}, err
		}
		_, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
		ticket, err := store.UpdateTicket(ctx, db, id, store.TicketUpdateParams{
			Title:              request.Title,
			Description:        request.Description,
			AcceptanceCriteria: request.AcceptanceCriteria,
			GitRepository:      request.GitRepository,
			GitBranch:          request.GitBranch,
			ParentID:           request.ParentID,
			Assignee:           request.Assignee,
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
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPut, "/api/tickets/"+url.PathEscape(id), request, &ticket)
	return ticket, err
}

func (c *Client) CloseTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Ticket{}, err
		}
		if message != "" {
			if _, err := store.AddComment(ctx, db, id, user.ID, message); err != nil {
				return store.Ticket{}, err
			}
		}
		return store.SetTicketComplete(ctx, db, id, true, user.Username, user.ID)
	}
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/close", body, &ticket)
	return ticket, err
}

func (c *Client) OpenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/open", body, &ticket)
	return ticket, err
}

func (c *Client) ArchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Ticket{}, err
		}
		if message != "" {
			if _, err := store.AddComment(ctx, db, id, user.ID, message); err != nil {
				return store.Ticket{}, err
			}
		}
		return store.SetTicketArchived(ctx, db, id, true, user.Username, user.ID)
	}
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/archive", body, &ticket)
	return ticket, err
}

func (c *Client) UnarchiveTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/unarchive", body, &ticket)
	return ticket, err
}

func (c *Client) ReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/ready", body, &ticket)
	return ticket, err
}

func (c *Client) NotReadyTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/notready", body, &ticket)
	return ticket, err
}

func (c *Client) SetTicketSdlc(ctx context.Context, id string, sdlcID int64) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketSdlc(ctx, db, id, sdlcID)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/sdlc", map[string]int64{"sdlc_id": sdlcID}, &ticket)
	return ticket, err
}

func (c *Client) UnsetTicketSdlc(ctx context.Context, id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.UnsetTicketSdlc(ctx, db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodDelete, "/api/tickets/"+url.PathEscape(id)+"/sdlc", nil, &ticket)
	return ticket, err
}

func (c *Client) DeleteTicket(ctx context.Context, id string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteTicket(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/tickets/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SetTicketParent(ctx context.Context, id, parentID string, message string) (store.Ticket, error) {
	current, err := c.GetTicketByID(ctx, id)
	if err != nil {
		return store.Ticket{}, err
	}
	return c.UpdateTicket(ctx, id, TicketUpdateRequest{
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

func (c *Client) UnsetTicketParent(ctx context.Context, id string, message string) (store.Ticket, error) {
	current, err := c.GetTicketByID(ctx, id)
	if err != nil {
		return store.Ticket{}, err
	}
	return c.UpdateTicket(ctx, id, TicketUpdateRequest{
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

func (c *Client) GetTicketByID(ctx context.Context, id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.GetTicket(ctx, db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(id), nil, &ticket)
	return ticket, err
}

func (c *Client) GetTicket(ctx context.Context, ref string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.GetTicketByRef(ctx, db, ref)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(strings.TrimSpace(ref)), nil, &ticket)
	return ticket, err
}

func (c *Client) CloneTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/clone", body, &ticket)
	return ticket, err
}

func (c *Client) ListHistory(ctx context.Context, id string) ([]store.HistoryEvent, error) {
	return c.ListHistoryPaged(ctx, id, 0, 0)
}

func (c *Client) ListHistoryPaged(ctx context.Context, id string, limit, offset int) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListHistoryEvents(ctx, db, id, limit, offset)
	}
	path := "/api/tickets/" + url.PathEscape(id) + "/history"
	params := url.Values{}
	if limit != 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset != 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var events []store.HistoryEvent
	err := c.doJSON(ctx, http.MethodGet, path, nil, &events)
	return events, err
}

func (c *Client) ListProjectHistory(ctx context.Context, projectID int64, limit int) ([]store.HistoryEvent, error) {
	return c.ListProjectHistoryFiltered(ctx, projectID, limit, store.HistoryFilter{})
}

func (c *Client) ListProjectHistoryFiltered(ctx context.Context, projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListProjectHistoryFiltered(ctx, db, projectID, limit, filter)
	}
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	if filter.UserID != "" {
		params.Set("user_id", filter.UserID)
	}
	if filter.AgentID != "" {
		params.Set("agent_id", filter.AgentID)
	}
	if filter.TeamID > 0 {
		params.Set("team_id", fmt.Sprintf("%d", filter.TeamID))
	}
	var events []store.HistoryEvent
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/history?%s", projectID, params.Encode()), nil, &events)
	return events, err
}

func (c *Client) AddComment(ctx context.Context, id string, comment string) (store.Comment, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Comment{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Comment{}, err
		}
		return store.AddComment(ctx, db, id, user.ID, comment)
	}
	var created store.Comment
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/comments", id), CommentCreateRequest{Comment: comment}, &created)
	return created, err
}

func (c *Client) ListComments(ctx context.Context, id string) ([]store.Comment, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListComments(ctx, db, id)
	}
	var comments []store.Comment
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/tickets/%s/comments", id), nil, &comments)
	return comments, err
}

func (c *Client) SetTicketHealth(ctx context.Context, id string, score int) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketHealth(ctx, db, id, score)
	}
	var updated store.Ticket
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/health", id), TicketHealthRequest{Score: score}, &updated)
	return updated, err
}

func (c *Client) AddDependency(ctx context.Context, request DependencyRequest) (store.Dependency, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Dependency{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Dependency{}, err
		}
		return store.AddDependency(ctx, db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
	}
	var dependency store.Dependency
	err := c.doJSON(ctx, http.MethodPost, "/api/dependencies", request, &dependency)
	return dependency, err
}

func (c *Client) RemoveDependency(ctx context.Context, request DependencyRequest) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteDependency(ctx, db, request.ProjectID, request.TicketID, request.DependsOn)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/dependencies?project_id=%d&ticket_id=%s&depends_on=%s", request.ProjectID, request.TicketID, request.DependsOn), nil, nil)
}

func (c *Client) ListDependencies(ctx context.Context, id string) ([]store.Dependency, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListDependencies(ctx, db, id)
	}
	var dependencies []store.Dependency
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/tickets/%s/dependencies", id), nil, &dependencies)
	return dependencies, err
}

func (c *Client) RequestTicket(ctx context.Context, request TicketRequest) (TicketRequestResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return TicketRequestResponse{}, err
		}
		user, err := c.localUser(ctx, db)
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
			response.Sdlc = ctx.Sdlc
			response.Role = ctx.Role
		}
		return response, nil
	}
	var reader *bytes.Reader
	payload, err := json.Marshal(request)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	reader = bytes.NewReader(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/tickets/claim", reader)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return TicketRequestResponse{}, friendlyConnectionError(err, c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return TicketRequestResponse{}, errors.New(apiErr.Error)
		}
		return TicketRequestResponse{}, fmt.Errorf("request failed with status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	var response TicketRequestResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return TicketRequestResponse{}, err
	}
	return response, nil
}

func (c *Client) CreateSdlc(ctx context.Context, request SdlcRequest) (store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Sdlc{}, err
		}
		return store.CreateSdlc(ctx, db, request.Name, request.Description)
	}
	var wf store.Sdlc
	err := c.doJSON(ctx, http.MethodPost, "/api/sdlcs", request, &wf)
	return wf, err
}

func (c *Client) ListSdlcs(ctx context.Context) ([]store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListSdlcs(ctx, db, 0, 0)
	}
	var sdlcs []store.Sdlc
	err := c.doJSON(ctx, http.MethodGet, "/api/sdlcs", nil, &sdlcs)
	return sdlcs, err
}

func (c *Client) GetSdlc(ctx context.Context, id int64) (store.SdlcWithStages, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcWithStages{}, err
		}
		return store.GetSdlc(ctx, db, id)
	}
	var wf store.SdlcWithStages
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/sdlcs/%d", id), nil, &wf)
	return wf, err
}

func (c *Client) DeleteSdlc(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteSdlc(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/sdlcs/%d", id), nil, nil)
}

func (c *Client) AddSdlcStage(ctx context.Context, sdlcID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		wow := request.WaysOfWorking
		if strings.TrimSpace(wow) == "" {
			wow = request.Description
		}
		dor := request.DefinitionOfReady
		if strings.TrimSpace(dor) == "" {
			dor = request.AcceptanceCriteria
		}
		return store.AddSdlcStageWithDefinitions(ctx, db, sdlcID, request.StageName, wow, dor, request.DefinitionOfDone, request.SortOrder)
	}
	var stage store.SdlcStage
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/sdlcs/%d/stages", sdlcID), request, &stage)
	return stage, err
}

func (c *Client) UpdateSdlcStage(ctx context.Context, stageID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		wow := request.WaysOfWorking
		if strings.TrimSpace(wow) == "" {
			wow = request.Description
		}
		dor := request.DefinitionOfReady
		if strings.TrimSpace(dor) == "" {
			dor = request.AcceptanceCriteria
		}
		return store.UpdateSdlcStageWithDefinitions(ctx, db, stageID, request.StageName, wow, dor, request.DefinitionOfDone)
	}
	var stage store.SdlcStage
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), request, &stage)
	return stage, err
}

func (c *Client) GetSdlcStage(ctx context.Context, stageID int64) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		return store.GetSdlcStage(ctx, db, stageID)
	}
	var stage store.SdlcStage
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), nil, &stage)
	return stage, err
}

func (c *Client) ListSdlcStages(ctx context.Context, sdlcID int64) ([]store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListSdlcStages(ctx, db, sdlcID)
	}
	// Derive from GetSdlc
	var wf store.SdlcWithStages
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/sdlcs/%d", sdlcID), nil, &wf)
	if err != nil {
		return nil, err
	}
	return wf.Stages, nil
}

func (c *Client) RemoveSdlcStage(ctx context.Context, stageID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveSdlcStage(ctx, db, stageID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), nil, nil)
}

func (c *Client) ReorderSdlcStages(ctx context.Context, sdlcID int64, stageIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.ReorderSdlcStages(ctx, db, sdlcID, stageIDs)
	}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/sdlcs/%d/reorder", sdlcID), SdlcReorderRequest{StageIDs: stageIDs}, nil)
}

func (c *Client) ExportSdlc(ctx context.Context, id int64) (store.SdlcExport, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcExport{}, err
		}
		return store.ExportSdlc(ctx, db, id)
	}
	var export store.SdlcExport
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/sdlcs/%d/export", id), nil, &export)
	return export, err
}

func (c *Client) ImportSdlc(ctx context.Context, export store.SdlcExport) (store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Sdlc{}, err
		}
		return store.ImportSdlc(ctx, db, export)
	}
	var wf store.Sdlc
	err := c.doJSON(ctx, http.MethodPost, "/api/sdlcs/import", export, &wf)
	return wf, err
}

func (c *Client) AddSdlcStageRole(ctx context.Context, sdlcID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.AddSdlcStageRole(ctx, db, sdlcID, stageID, roleID)
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d", sdlcID, stageID), map[string]int64{"role_id": roleID}, nil)
}

func (c *Client) RemoveSdlcStageRole(ctx context.Context, sdlcID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveSdlcStageRole(ctx, db, sdlcID, stageID, roleID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d/%d", sdlcID, stageID, roleID), nil, nil)
}

func (c *Client) ReorderSdlcStageRoles(ctx context.Context, sdlcID, stageID int64, roleIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.ReorderSdlcStageRoles(ctx, db, sdlcID, stageID, roleIDs)
	}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d", sdlcID, stageID), map[string][]int64{"role_ids": roleIDs}, nil)
}

func (c *Client) CompleteTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return c.CloseTicket(ctx, id, message)
}

func (c *Client) ReopenTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return c.OpenTicket(ctx, id, message)
}

func (c *Client) DraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return c.NotReadyTicket(ctx, id, message)
}

func (c *Client) UndraftTicket(ctx context.Context, id string, message string) (store.Ticket, error) {
	return c.ReadyTicket(ctx, id, message)
}

func (c *Client) NextTicket(ctx context.Context, id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, _ := c.localUser(ctx, db)
		return store.NextTicket(ctx, db, id, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/next", id), nil, &ticket)
	return ticket, err
}

func (c *Client) PreviousTicket(ctx context.Context, id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, _ := c.localUser(ctx, db)
		return store.PreviousTicket(ctx, db, id, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/previous", id), nil, &ticket)
	return ticket, err
}

func (c *Client) LogTime(ctx context.Context, ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TimeEntry{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.TimeEntry{}, err
		}
		return store.LogTime(ctx, db, ticketID, user.ID, request.Minutes, request.Note)
	}
	var entry store.TimeEntry
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(ticketID)+"/time", request, &entry)
	return entry, err
}

func (c *Client) ListTimeEntries(ctx context.Context, ticketID string) ([]store.TimeEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListTimeEntries(ctx, db, ticketID)
	}
	var entries []store.TimeEntry
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/time", nil, &entries)
	return entries, err
}

func (c *Client) DeleteTimeEntry(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteTimeEntry(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/time/%d", id), nil, nil)
}

func (c *Client) TotalTimeForTicket(ctx context.Context, ticketID string) (int, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return 0, err
		}
		return store.TotalTimeForTicket(ctx, db, ticketID)
	}
	var result struct {
		Total int `json:"total"`
	}
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/time/total", nil, &result)
	return result.Total, err
}

func (c *Client) CreateLabel(ctx context.Context, projectID int64, request LabelRequest) (store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Label{}, err
		}
		return store.CreateLabel(ctx, db, projectID, request.Name, request.Color)
	}
	var label store.Label
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/labels", projectID), request, &label)
	return label, err
}

func (c *Client) ListLabels(ctx context.Context, projectID int64) ([]store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListLabels(ctx, db, projectID, 0, 0)
	}
	var labels []store.Label
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/labels", projectID), nil, &labels)
	return labels, err
}

func (c *Client) DeleteLabel(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteLabel(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/labels/%d", id), nil, nil)
}

func (c *Client) AddTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.AddTicketLabel(ctx, db, ticketID, labelID)
	}
	return c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(ticketID)+"/labels", map[string]int64{"label_id": labelID}, nil)
}

func (c *Client) RemoveTicketLabel(ctx context.Context, ticketID string, labelID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveTicketLabel(ctx, db, ticketID, labelID)
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/tickets/"+url.PathEscape(ticketID)+"/labels/"+fmt.Sprintf("%d", labelID), nil, nil)
}

func (c *Client) ListTicketLabels(ctx context.Context, ticketID string) ([]store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListTicketLabels(ctx, db, ticketID)
	}
	var labels []store.Label
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/labels", nil, &labels)
	return labels, err
}

type storyRequest struct {
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (c *Client) CreateStory(ctx context.Context, projectID int64, title, description string) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Story{}, err
		}
		return store.CreateStory(ctx, db, projectID, title, description, user.ID)
	}
	var created store.Story
	err := c.doJSON(ctx, http.MethodPost, "/api/stories", storyRequest{ProjectID: projectID, Title: title, Description: description}, &created)
	return created, err
}

func (c *Client) ListStories(ctx context.Context, projectID int64) ([]store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListStoriesByProject(ctx, db, projectID, 0, 0)
	}
	var stories []store.Story
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/stories", projectID), nil, &stories)
	return stories, err
}

func (c *Client) GetStory(ctx context.Context, id int64) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		return store.GetStory(ctx, db, id)
	}
	var story store.Story
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/stories/%d", id), nil, &story)
	return story, err
}

func (c *Client) UpdateStory(ctx context.Context, id int64, title, description string) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		return store.UpdateStory(ctx, db, id, title, description)
	}
	var updated store.Story
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/stories/%d", id), storyRequest{Title: title, Description: description}, &updated)
	return updated, err
}

func (c *Client) DeleteStory(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteStory(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/stories/%d", id), nil, nil)
}
