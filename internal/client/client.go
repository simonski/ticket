package client

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	mode    string
}


func New(cfg config.Config) *Client {
	resolved, err := config.ResolveURL()
	if err != nil {
		resolved = config.Resolved{Mode: config.ModeLocal}
	}
	baseURL := strings.TrimRight(resolved.ServerURL, "/")
	if baseURL == "" && cfg.ServerURL != "" {
		baseURL = strings.TrimRight(cfg.ServerURL, "/")
	}
	return &Client{
		baseURL: baseURL,
		token:   cfg.Token,
		http:    http.DefaultClient,
		mode:    resolved.Mode,
	}
}

func (c *Client) Register(username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		return store.User{}, errors.New("ticket register requires TICKET_URL=http(s)://...")
	}
	var user store.User
	err := c.doJSON(http.MethodPost, "/api/register", map[string]string{
		"username": username,
		"password": password,
	}, &user)
	return user, err
}

func (c *Client) Login(username, password string) (AuthResponse, error) {
	if c.mode == config.ModeLocal {
		return AuthResponse{}, errors.New("ticket login requires TICKET_URL=http(s)://...")
	}
	var response AuthResponse
	err := c.doJSON(http.MethodPost, "/api/login", map[string]string{
		"username": username,
		"password": password,
	}, &response)
	return response, err
}

func (c *Client) Logout() error {
	if c.mode == config.ModeLocal {
		return errors.New("ticket logout requires TICKET_URL=http(s)://...")
	}
	return c.doJSON(http.MethodPost, "/api/logout", nil, nil)
}

func (c *Client) Status() (StatusResponse, error) {
	if c.mode == config.ModeLocal {
		resolved, err := config.ResolveURL()
		if err != nil {
			return StatusResponse{}, err
		}
		if _, err := os.Stat(resolved.DBPath); err != nil {
			return StatusResponse{}, err
		}
		db, err := store.Open(resolved.DBPath)
		if err != nil {
			return StatusResponse{}, err
		}
		defer db.Close()

		user, err := store.GetUserByUsername(db, localUsername())
		registrationEnabled, regErr := store.RegistrationEnabled(db)
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
	err := c.doJSON(http.MethodGet, "/api/status", nil, &status)
	return status, err
}

func (c *Client) Count(projectID *int64) (CountSummary, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return CountSummary{}, err
		}
		defer db.Close()
		return store.CountEverything(db, projectID)
	}
	var summary CountSummary
	path := "/api/count"
	if projectID != nil {
		path = fmt.Sprintf("/api/count?project_id=%d", *projectID)
	}
	err := c.doJSON(http.MethodGet, path, nil, &summary)
	return summary, err
}

func (c *Client) SetRegistrationEnabled(enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.SetRegistrationEnabled(db, enabled)
	}
	return c.doJSON(http.MethodPost, "/api/config/registration", map[string]any{"enabled": enabled}, nil)
}

func (c *Client) CreateUser(username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.User{}, err
		}
		defer db.Close()
		return store.CreateUser(db, username, password, "user")
	}
	var user store.User
	err := c.doJSON(http.MethodPost, "/api/users", map[string]string{
		"username": username,
		"password": password,
	}, &user)
	return user, err
}

func (c *Client) SetUserEnabled(username string, enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.SetUserEnabled(db, username, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	return c.doJSON(http.MethodPost, "/api/users/"+username+"/"+action, nil, nil)
}

func (c *Client) ListUsers() ([]store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListUsers(db)
	}
	var users []store.User
	err := c.doJSON(http.MethodGet, "/api/users", nil, &users)
	return users, err
}

func (c *Client) DeleteUser(username string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteUser(db, username)
	}
	return c.doJSON(http.MethodDelete, "/api/users/"+username, nil, nil)
}

func (c *Client) ResetUserPassword(username, newPassword string) (store.User, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.User{}, err
		}
		defer db.Close()
		return store.ResetUserPassword(db, username, newPassword)
	}
	var user store.User
	err := c.doJSON(http.MethodPost, "/api/users/"+username+"/reset-password", map[string]string{"password": newPassword}, &user)
	return user, err
}

func (c *Client) CreateRole(request RoleRequest) (store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Role{}, err
		}
		defer db.Close()
		return store.CreateRole(db, request.Title, request.Motivation, request.Goals)
	}
	var role store.Role
	err := c.doJSON(http.MethodPost, "/api/roles", request, &role)
	return role, err
}

func (c *Client) ListRoles() ([]store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListRoles(db)
	}
	var roles []store.Role
	err := c.doJSON(http.MethodGet, "/api/roles", nil, &roles)
	return roles, err
}

func (c *Client) UpdateRole(id int64, request RoleRequest) (store.Role, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Role{}, err
		}
		defer db.Close()
		return store.UpdateRole(db, id, request.Title, request.Motivation, request.Goals)
	}
	var role store.Role
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/roles/%d", id), request, &role)
	return role, err
}

func (c *Client) DeleteRole(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteRole(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/roles/%d", id), nil, nil)
}

func (c *Client) CreateAgent(request AgentCreateRequest) (store.Agent, string, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, "", err
		}
		defer db.Close()
		return store.CreateAgent(db, request.Password)
	}
	var response struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	err := c.doJSON(http.MethodPost, "/api/agents", request, &response)
	return response.Agent, response.Password, err
}

func (c *Client) SetAgentEnabled(id string, enabled bool) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		defer db.Close()
		return store.SetAgentEnabled(db, id, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	var agent store.Agent
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/agents/%s/%s", id, action), nil, &agent)
	return agent, err
}

func (c *Client) ListAgents() ([]store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListAgents(db)
	}
	var agents []store.Agent
	err := c.doJSON(http.MethodGet, "/api/agents", nil, &agents)
	return agents, err
}

func (c *Client) ListAgentStatuses() ([]store.AgentStatus, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListAgentStatuses(db)
	}
	var statuses []store.AgentStatus
	err := c.doJSON(http.MethodGet, "/api/agents/statuses", nil, &statuses)
	return statuses, err
}

func (c *Client) UpdateAgent(id string, request AgentUpdateRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		defer db.Close()
		return store.UpdateAgent(db, id, store.AgentUpdateParams{
			Password: request.Password,
		})
	}
	var agent store.Agent
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/agents/%s", id), request, &agent)
	return agent, err
}

func (c *Client) DeleteAgent(id string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteAgent(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/agents/%s", id), nil, nil)
}

func (c *Client) SetAgentConfig(agentID string, key, value string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.SetAgentConfig(db, agentID, key, value)
	}
	return c.doJSON(http.MethodPost, fmt.Sprintf("/api/agents/%s/config", agentID), map[string]string{"key": key, "value": value}, nil)
}

func (c *Client) ListAgentConfig(agentID string) ([]store.AgentConfigEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListAgentConfig(db, agentID)
	}
	var entries []store.AgentConfigEntry
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/agents/%s/config", agentID), nil, &entries)
	return entries, err
}

func (c *Client) DeleteAgentConfig(agentID string, key string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteAgentConfig(db, agentID, key)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/agents/%s/config/%s", agentID, key), nil, nil)
}

func (c *Client) RegisterAgent(request AgentRegisterRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
	var response struct {
		Agent store.Agent `json:"agent"`
	}
	err := c.doJSONBasicAuth(http.MethodPost, "/api/agents/register", request.ID, request.Password, nil, &response)
	return response.Agent, err
}

func (c *Client) HeartbeatAgent(agentID, password, status string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
	var response struct{}
	return c.doJSONBasicAuth(http.MethodPost, "/api/agents/heartbeat", agentID, password, map[string]string{"status": status}, &response)
}

func (c *Client) RequestAgentWork(request AgentRequest) (AgentWorkResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		if projectID == 0 {
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
	var response AgentWorkResponse
	body := map[string]any{
		"project_id": request.ProjectID,
		"dry_run":    request.DryRun,
	}
	if request.TicketID != nil {
		body["ticket_id"] = *request.TicketID
	}
	err := c.doJSONBasicAuth(http.MethodPost, "/api/agents/request", request.ID, request.Password, body, &response)
	return response, err
}

func (c *Client) AgentUpdateTicket(id string, request AgentTicketUpdateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		return store.UpdateTicket(db, id, store.TicketUpdateParams{
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
	err := c.doJSONBasicAuth(http.MethodPost, fmt.Sprintf("/api/agents/tickets/%s/update", id), request.ID, request.Password, body, &ticket)
	return ticket, err
}

func (c *Client) CreateProject(request ProjectCreateRequest) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
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
	var project store.Project
	err := c.doJSON(http.MethodPost, "/api/projects", request, &project)
	return project, err
}

func (c *Client) ListProjects() ([]store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListProjects(db)
	}
	var projects []store.Project
	err := c.doJSON(http.MethodGet, "/api/projects", nil, &projects)
	return projects, err
}

func (c *Client) GetProject(id string) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		defer db.Close()
		return store.GetProject(db, id)
	}
	var project store.Project
	err := c.doJSON(http.MethodGet, "/api/projects/"+id, nil, &project)
	return project, err
}

func (c *Client) UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
			Visibility:         request.Visibility,
			WorkflowID:         request.WorkflowID,
		})
	}
	var project store.Project
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/projects/%d", id), request, &project)
	return project, err
}

func (c *Client) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		defer db.Close()
		return store.SetProjectStatus(db, id, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	var project store.Project
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%d/%s", id, action), nil, &project)
	return project, err
}

func (c *Client) DeleteProject(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteProject(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), nil, nil)
}

func (c *Client) AddProjectMember(projectID int64, request ProjectMemberRequest) (store.ProjectMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectMember{}, err
		}
		defer db.Close()
		return store.AddProjectMember(db, projectID, request.UserID, request.Role)
	}
	var member store.ProjectMember
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%d/users", projectID), request, &member)
	return member, err
}

func (c *Client) RemoveProjectMember(projectID int64, userID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveProjectMember(db, projectID, userID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/projects/%d/users/%s", projectID, userID), nil, nil)
}

func (c *Client) ListProjectMembers(projectID int64) ([]store.ProjectMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListProjectMembers(db, projectID)
	}
	var members []store.ProjectMember
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/users", projectID), nil, &members)
	return members, err
}

func (c *Client) AddProjectTeamMember(projectID int64, request ProjectTeamMemberRequest) (store.ProjectTeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectTeamMember{}, err
		}
		defer db.Close()
		return store.AddProjectTeamMember(db, projectID, request.TeamID, request.Role)
	}
	var member store.ProjectTeamMember
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%d/teams", projectID), request, &member)
	return member, err
}

func (c *Client) RemoveProjectTeamMember(projectID, teamID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveProjectTeamMember(db, projectID, teamID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/projects/%d/teams/%d", projectID, teamID), nil, nil)
}

func (c *Client) ListProjectTeamMembers(projectID int64) ([]store.ProjectTeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListProjectTeamMembers(db, projectID)
	}
	var members []store.ProjectTeamMember
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/teams", projectID), nil, &members)
	return members, err
}

func (c *Client) CreateTeam(request TeamRequest) (store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Team{}, err
		}
		defer db.Close()
		return store.CreateTeam(db, request.Name, request.ParentTeamID)
	}
	var team store.Team
	err := c.doJSON(http.MethodPost, "/api/teams", request, &team)
	return team, err
}

func (c *Client) ListTeams() ([]store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListTeams(db)
	}
	var teams []store.Team
	err := c.doJSON(http.MethodGet, "/api/teams", nil, &teams)
	return teams, err
}

func (c *Client) UpdateTeam(id int64, request TeamRequest) (store.Team, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Team{}, err
		}
		defer db.Close()
		return store.UpdateTeam(db, id, request.Name, request.ParentTeamID)
	}
	var team store.Team
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/teams/%d", id), request, &team)
	return team, err
}

func (c *Client) DeleteTeam(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteTeam(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/teams/%d", id), nil, nil)
}

func (c *Client) AddTeamMember(teamID int64, request TeamMemberRequest) (store.TeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TeamMember{}, err
		}
		defer db.Close()
		return store.AddTeamMember(db, teamID, request.UserID, request.Role, request.JobTitle)
	}
	var member store.TeamMember
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/teams/%d/users", teamID), request, &member)
	return member, err
}

func (c *Client) RemoveTeamMember(teamID int64, userID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveTeamMember(db, teamID, userID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/teams/%d/users/%s", teamID, userID), nil, nil)
}

func (c *Client) ListTeamMembers(teamID int64) ([]store.TeamMember, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListTeamMembers(db, teamID)
	}
	var members []store.TeamMember
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/teams/%d/users", teamID), nil, &members)
	return members, err
}

func (c *Client) AddTeamAgent(teamID int64, agentID string) (store.TeamAgent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TeamAgent{}, err
		}
		defer db.Close()
		return store.AddTeamAgent(db, teamID, agentID)
	}
	var item store.TeamAgent
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/teams/%d/agents", teamID), map[string]string{"agent_id": agentID}, &item)
	return item, err
}

func (c *Client) RemoveTeamAgent(teamID int64, agentID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveTeamAgent(db, teamID, agentID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/teams/%d/agents/%s", teamID, agentID), nil, nil)
}

func (c *Client) ListTeamAgents(teamID int64) ([]store.TeamAgent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListTeamAgents(db, teamID)
	}
	var items []store.TeamAgent
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/teams/%d/agents", teamID), nil, &items)
	return items, err
}

func (c *Client) CreateTicket(request TicketCreateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
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
			Author:             user.Username,
			State:              state,
			CreatedBy:          user.ID,
		})
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets", request, &ticket)
	return ticket, err
}

func (c *Client) ListTickets(projectID int64) ([]store.Ticket, error) {
	return c.ListTicketsFiltered(projectID, "", "", "", "", "", "", 0, false)
}

func (c *Client) ListTicketsFiltered(projectID int64, ticketType, stage, state, status, search, assignee string, limit int, includeArchived bool) ([]store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
	err := c.doJSON(http.MethodGet, path, nil, &tickets)
	return tickets, err
}

func (c *Client) UpdateTicket(id string, request TicketUpdateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		_, state, _ := resolveRequestLifecycle(request.Status, request.Stage, request.State)
		return store.UpdateTicket(db, id, store.TicketUpdateParams{
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
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPut, "/api/tickets/"+url.PathEscape(id), request, &ticket)
	return ticket, err
}

func (c *Client) CloseTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketOpen(db, id, false, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/close", nil, &ticket)
	return ticket, err
}

func (c *Client) OpenTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketOpen(db, id, true, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/open", nil, &ticket)
	return ticket, err
}

func (c *Client) ArchiveTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketArchived(db, id, true, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/archive", nil, &ticket)
	return ticket, err
}

func (c *Client) UnarchiveTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketArchived(db, id, false, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/unarchive", nil, &ticket)
	return ticket, err
}

func (c *Client) ReadyTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketReady(db, id, true, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/ready", nil, &ticket)
	return ticket, err
}

func (c *Client) NotReadyTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketReady(db, id, false, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/notready", nil, &ticket)
	return ticket, err
}

func (c *Client) SetTicketWorkflow(id string, workflowID int64) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.SetTicketWorkflow(db, id, workflowID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/workflow", map[string]int64{"workflow_id": workflowID}, &ticket)
	return ticket, err
}

func (c *Client) UnsetTicketWorkflow(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.UnsetTicketWorkflow(db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodDelete, "/api/tickets/"+url.PathEscape(id)+"/workflow", nil, &ticket)
	return ticket, err
}

func (c *Client) DeleteTicket(id string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteTicket(db, id)
	}
	return c.doJSON(http.MethodDelete, "/api/tickets/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SetTicketParent(id, parentID string) (store.Ticket, error) {
	current, err := c.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return c.UpdateTicket(id, TicketUpdateRequest{
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

func (c *Client) UnsetTicketParent(id string) (store.Ticket, error) {
	current, err := c.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return c.UpdateTicket(id, TicketUpdateRequest{
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

func (c *Client) GetTicketByID(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.GetTicket(db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(id), nil, &ticket)
	return ticket, err
}

func (c *Client) GetTicket(ref string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.GetTicketByRef(db, ref)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(strings.TrimSpace(ref)), nil, &ticket)
	return ticket, err
}

func (c *Client) CloneTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.CloneTicket(db, id, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/clone", nil, &ticket)
	return ticket, err
}

func (c *Client) ListHistory(id string) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListHistoryEvents(db, id)
	}
	var events []store.HistoryEvent
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(id)+"/history", nil, &events)
	return events, err
}

func (c *Client) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListProjectHistory(db, projectID, limit)
	}
	if limit <= 0 {
		limit = 10
	}
	var events []store.HistoryEvent
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/history?limit=%d", projectID, limit), nil, &events)
	return events, err
}

func (c *Client) AddComment(id string, comment string) (store.Comment, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Comment{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Comment{}, err
		}
		return store.AddComment(db, id, user.ID, comment)
	}
	var created store.Comment
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%s/comments", id), CommentCreateRequest{Comment: comment}, &created)
	return created, err
}

func (c *Client) ListComments(id string) ([]store.Comment, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListComments(db, id)
	}
	var comments []store.Comment
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%s/comments", id), nil, &comments)
	return comments, err
}

func (c *Client) SetTicketHealth(id string, score int) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.SetTicketHealth(db, id, score)
	}
	var updated store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%s/health", id), TicketHealthRequest{Score: score}, &updated)
	return updated, err
}

func (c *Client) AddDependency(request DependencyRequest) (store.Dependency, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Dependency{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Dependency{}, err
		}
		return store.AddDependency(db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
	}
	var dependency store.Dependency
	err := c.doJSON(http.MethodPost, "/api/dependencies", request, &dependency)
	return dependency, err
}

func (c *Client) RemoveDependency(request DependencyRequest) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteDependency(db, request.ProjectID, request.TicketID, request.DependsOn)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/dependencies?project_id=%d&ticket_id=%s&depends_on=%s", request.ProjectID, request.TicketID, request.DependsOn), nil, nil)
}

func (c *Client) ListDependencies(id string) ([]store.Dependency, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListDependencies(db, id)
	}
	var dependencies []store.Dependency
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%s/dependencies", id), nil, &dependencies)
	return dependencies, err
}

func (c *Client) RequestTicket(request TicketRequest) (TicketRequestResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return TicketRequestResponse{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
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
	var reader *bytes.Reader
	payload, err := json.Marshal(request)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	reader = bytes.NewReader(payload)

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/tickets/claim", reader)
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


func (c *Client) CreateWorkflow(request WorkflowRequest) (store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Workflow{}, err
		}
		defer db.Close()
		return store.CreateWorkflow(db, request.Name, request.Description)
	}
	var wf store.Workflow
	err := c.doJSON(http.MethodPost, "/api/workflows", request, &wf)
	return wf, err
}

func (c *Client) ListWorkflows() ([]store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListWorkflows(db)
	}
	var workflows []store.Workflow
	err := c.doJSON(http.MethodGet, "/api/workflows", nil, &workflows)
	return workflows, err
}

func (c *Client) GetWorkflow(id int64) (store.WorkflowWithStages, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowWithStages{}, err
		}
		defer db.Close()
		return store.GetWorkflow(db, id)
	}
	var wf store.WorkflowWithStages
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/workflows/%d", id), nil, &wf)
	return wf, err
}

func (c *Client) DeleteWorkflow(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteWorkflow(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/workflows/%d", id), nil, nil)
}

func (c *Client) AddWorkflowStage(workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowStage{}, err
		}
		defer db.Close()
		return store.AddWorkflowStage(db, workflowID, request.StageName, request.Description, request.RoleID, request.SortOrder)
	}
	var stage store.WorkflowStage
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/workflows/%d/stages", workflowID), request, &stage)
	return stage, err
}

func (c *Client) RemoveWorkflowStage(stageID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveWorkflowStage(db, stageID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/workflows/stages/%d", stageID), nil, nil)
}

func (c *Client) ReorderWorkflowStages(workflowID int64, stageIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.ReorderWorkflowStages(db, workflowID, stageIDs)
	}
	return c.doJSON(http.MethodPut, fmt.Sprintf("/api/workflows/%d/reorder", workflowID), WorkflowReorderRequest{StageIDs: stageIDs}, nil)
}

func (c *Client) ExportWorkflow(id int64) (store.WorkflowExport, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowExport{}, err
		}
		defer db.Close()
		return store.ExportWorkflow(db, id)
	}
	var export store.WorkflowExport
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/workflows/%d/export", id), nil, &export)
	return export, err
}

func (c *Client) ImportWorkflow(export store.WorkflowExport) (store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Workflow{}, err
		}
		defer db.Close()
		return store.ImportWorkflow(db, export)
	}
	var wf store.Workflow
	err := c.doJSON(http.MethodPost, "/api/workflows/import", export, &wf)
	return wf, err
}

func (c *Client) LogTime(ticketID string, request libticket.TimeEntryRequest) (store.TimeEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.TimeEntry{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.TimeEntry{}, err
		}
		return store.LogTime(db, ticketID, user.ID, request.Minutes, request.Note)
	}
	var entry store.TimeEntry
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(ticketID)+"/time", request, &entry)
	return entry, err
}

func (c *Client) ListTimeEntries(ticketID string) ([]store.TimeEntry, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListTimeEntries(db, ticketID)
	}
	var entries []store.TimeEntry
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/time", nil, &entries)
	return entries, err
}

func (c *Client) DeleteTimeEntry(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteTimeEntry(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/time/%d", id), nil, nil)
}

func (c *Client) TotalTimeForTicket(ticketID string) (int, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return 0, err
		}
		defer db.Close()
		return store.TotalTimeForTicket(db, ticketID)
	}
	var result struct {
		Total int `json:"total"`
	}
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/time/total", nil, &result)
	return result.Total, err
}

func (c *Client) CreateLabel(projectID int64, request libticket.LabelRequest) (store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Label{}, err
		}
		defer db.Close()
		return store.CreateLabel(db, projectID, request.Name, request.Color)
	}
	var label store.Label
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%d/labels", projectID), request, &label)
	return label, err
}

func (c *Client) ListLabels(projectID int64) ([]store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListLabels(db, projectID)
	}
	var labels []store.Label
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/labels", projectID), nil, &labels)
	return labels, err
}

func (c *Client) DeleteLabel(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteLabel(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/labels/%d", id), nil, nil)
}

func (c *Client) AddTicketLabel(ticketID string, labelID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.AddTicketLabel(db, ticketID, labelID)
	}
	return c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(ticketID)+"/labels", map[string]int64{"label_id": labelID}, nil)
}

func (c *Client) RemoveTicketLabel(ticketID string, labelID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveTicketLabel(db, ticketID, labelID)
	}
	return c.doJSON(http.MethodDelete, "/api/tickets/"+url.PathEscape(ticketID)+"/labels/"+fmt.Sprintf("%d", labelID), nil, nil)
}

func (c *Client) ListTicketLabels(ticketID string) ([]store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListTicketLabels(db, ticketID)
	}
	var labels []store.Label
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/labels", nil, &labels)
	return labels, err
}

type storyRequest struct {
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (c *Client) CreateStory(projectID int64, title, description string) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		defer db.Close()
		user, err := c.localUser(db)
		if err != nil {
			return store.Story{}, err
		}
		return store.CreateStory(db, projectID, title, description, user.ID)
	}
	var created store.Story
	err := c.doJSON(http.MethodPost, "/api/stories", storyRequest{ProjectID: projectID, Title: title, Description: description}, &created)
	return created, err
}

func (c *Client) ListStories(projectID int64) ([]store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListStoriesByProject(db, projectID)
	}
	var stories []store.Story
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/stories", projectID), nil, &stories)
	return stories, err
}

func (c *Client) GetStory(id int64) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		defer db.Close()
		return store.GetStory(db, id)
	}
	var story store.Story
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/stories/%d", id), nil, &story)
	return story, err
}

func (c *Client) UpdateStory(id int64, title, description string) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		defer db.Close()
		return store.UpdateStory(db, id, title, description)
	}
	var updated store.Story
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/stories/%d", id), storyRequest{Title: title, Description: description}, &updated)
	return updated, err
}

func (c *Client) DeleteStory(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteStory(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/stories/%d", id), nil, nil)
}
