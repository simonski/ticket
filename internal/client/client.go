package client

import (
	"context"
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
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	mode    string
}


func New(cfg config.Config) *Client {
	resolved, err := config.ResolveLocation(cfg.Location)
	if err != nil {
		resolved = config.Resolved{Mode: config.ModeLocal}
	}
	baseURL := strings.TrimRight(resolved.ServerURL, "/")
	return &Client{
		baseURL: baseURL,
		token:   cfg.Token,
		http:    http.DefaultClient,
		mode:    resolved.Mode,
	}
}

func (c *Client) Register(username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		return store.User{}, errors.New("ticket register requires remote mode (run tk init to configure)")
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
		return AuthResponse{}, errors.New("ticket login requires remote mode (run tk init to configure)")
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
		return errors.New("ticket logout requires remote mode (run tk init to configure)")
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

		user, err := store.GetUserByUsername(context.Background(), db, localUsername())
		registrationEnabled, regErr := store.RegistrationEnabled(context.Background(), db)
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
		return store.CountEverything(context.Background(), db, projectID)
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
		return store.SetRegistrationEnabled(context.Background(), db, enabled)
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
		return store.CreateUser(context.Background(), db, username, password, "user")
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
		return store.SetUserEnabled(context.Background(), db, username, enabled)
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
		return store.ListUsers(context.Background(), db, 0)
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
		return store.DeleteUser(context.Background(), db, username)
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
		return store.ResetUserPassword(context.Background(), db, username, newPassword)
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
		return store.CreateRole(context.Background(), db, request.SdlcID, request.Title, request.Description, request.AcceptanceCriteria)
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
		return store.ListRoles(context.Background(), db, 0)
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
		return store.UpdateRole(context.Background(), db, id, request.Title, request.Description, request.AcceptanceCriteria)
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
		return store.DeleteRole(context.Background(), db, id)
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
		return store.CreateAgent(context.Background(), db, request.Password)
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
		return store.SetAgentEnabled(context.Background(), db, id, enabled)
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
		return store.ListAgents(context.Background(), db)
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
		return store.ListAgentStatuses(context.Background(), db)
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
		return store.UpdateAgent(context.Background(), db, id, store.AgentUpdateParams{
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
		return store.DeleteAgent(context.Background(), db, id)
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
		return store.SetAgentConfig(context.Background(), db, agentID, key, value)
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
		return store.ListAgentConfig(context.Background(), db, agentID)
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
		return store.DeleteAgentConfig(context.Background(), db, agentID, key)
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
		agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
		if err != nil {
			return store.Agent{}, err
		}
		return store.TouchAgent(context.Background(), db, agent.ID, "online")
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
		agent, err := store.AuthenticateAgent(context.Background(), db, agentID, password)
		if err != nil {
			return err
		}
		_, err = store.TouchAgent(context.Background(), db, agent.ID, status)
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
		agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
		if err != nil {
			return AgentWorkResponse{}, err
		}
		projectID := request.ProjectID
		if request.TicketID != nil {
			projectID = 0
		}
		if projectID == 0 {
			projects, err := store.ListProjects(context.Background(), db, 0)
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
		agent, err := store.AuthenticateAgent(context.Background(), db, request.ID, request.Password)
		if err != nil {
			return store.Ticket{}, err
		}
		current, err := store.GetTicket(context.Background(), db, id)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.UpdateTicket(context.Background(), db, id, store.TicketUpdateParams{
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
		return store.ListProjects(context.Background(), db, 0)
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
		return store.GetProject(context.Background(), db, id)
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
		return store.UpdateProjectWithParams(context.Background(), db, id, store.ProjectUpdateParams{
			Title:              request.Title,
			Description:        request.Description,
			AcceptanceCriteria: request.AcceptanceCriteria,
			GitRepository:      request.GitRepository,
			GitBranch:          request.GitBranch,
			Notes:              request.Notes,
			Visibility:         request.Visibility,
			SdlcID:         request.SdlcID,
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
		return store.SetProjectStatus(context.Background(), db, id, enabled)
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	var project store.Project
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/projects/%d/%s", id, action), nil, &project)
	return project, err
}

func (c *Client) SetProjectDefaultDraft(projectID int64, draft bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.SetProjectDefaultDraft(context.Background(), db, projectID, draft)
	}
	return c.doJSON(http.MethodPut, fmt.Sprintf("/api/projects/%d/set-draft", projectID), map[string]bool{"draft": draft}, nil)
}

func (c *Client) DeleteProject(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteProject(context.Background(), db, id)
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
		return store.AddProjectMember(context.Background(), db, projectID, request.UserID, request.Role)
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
		return store.RemoveProjectMember(context.Background(), db, projectID, userID)
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
		return store.ListProjectMembers(context.Background(), db, projectID)
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
		return store.AddProjectTeamMember(context.Background(), db, projectID, request.TeamID, request.Role)
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
		return store.RemoveProjectTeamMember(context.Background(), db, projectID, teamID)
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
		return store.ListProjectTeamMembers(context.Background(), db, projectID)
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
		return store.CreateTeam(context.Background(), db, request.Name, request.ParentTeamID)
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
		return store.ListTeams(context.Background(), db, 0)
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
		return store.UpdateTeam(context.Background(), db, id, request.Name, request.ParentTeamID)
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
		return store.DeleteTeam(context.Background(), db, id)
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
		return store.AddTeamMember(context.Background(), db, teamID, request.UserID, request.Role, request.JobTitle)
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
		return store.RemoveTeamMember(context.Background(), db, teamID, userID)
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
		return store.ListTeamMembers(context.Background(), db, teamID)
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
		return store.AddTeamAgent(context.Background(), db, teamID, agentID)
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
		return store.RemoveTeamAgent(context.Background(), db, teamID, agentID)
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
		return store.ListTeamAgents(context.Background(), db, teamID)
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
			Author:             user.Username,
			State:              state,
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
		ticket, err := store.UpdateTicket(context.Background(), db, id, store.TicketUpdateParams{
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
			if _, err := store.AddComment(context.Background(), db, ticket.ID, user.ID, request.Message); err != nil {
				return ticket, err
			}
		}
		return ticket, nil
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPut, "/api/tickets/"+url.PathEscape(id), request, &ticket)
	return ticket, err
}

func (c *Client) CloseTicket(id string, message string) (store.Ticket, error) {
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
		if message != "" {
			if _, err := store.AddComment(context.Background(), db, id, user.ID, message); err != nil {
				return store.Ticket{}, err
			}
		}
		return store.SetTicketComplete(context.Background(), db, id, true, user.Username, user.ID)
	}
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/close", body, &ticket)
	return ticket, err
}

func (c *Client) OpenTicket(id string, message string) (store.Ticket, error) {
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/open", body, &ticket)
	return ticket, err
}

func (c *Client) ArchiveTicket(id string, message string) (store.Ticket, error) {
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
		if message != "" {
			if _, err := store.AddComment(context.Background(), db, id, user.ID, message); err != nil {
				return store.Ticket{}, err
			}
		}
		return store.SetTicketArchived(context.Background(), db, id, true, user.Username, user.ID)
	}
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/archive", body, &ticket)
	return ticket, err
}

func (c *Client) UnarchiveTicket(id string, message string) (store.Ticket, error) {
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/unarchive", body, &ticket)
	return ticket, err
}

func (c *Client) ReadyTicket(id string, message string) (store.Ticket, error) {
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/ready", body, &ticket)
	return ticket, err
}

func (c *Client) NotReadyTicket(id string, message string) (store.Ticket, error) {
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/notready", body, &ticket)
	return ticket, err
}

func (c *Client) SetTicketSdlc(id string, sdlcID int64) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.SetTicketSdlc(context.Background(), db, id, sdlcID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/sdlc", map[string]int64{"sdlc_id": sdlcID}, &ticket)
	return ticket, err
}

func (c *Client) UnsetTicketSdlc(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.UnsetTicketSdlc(context.Background(), db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodDelete, "/api/tickets/"+url.PathEscape(id)+"/sdlc", nil, &ticket)
	return ticket, err
}

func (c *Client) DeleteTicket(id string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteTicket(context.Background(), db, id)
	}
	return c.doJSON(http.MethodDelete, "/api/tickets/"+url.PathEscape(id), nil, nil)
}

func (c *Client) SetTicketParent(id, parentID string, message string) (store.Ticket, error) {
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
		Message:            message,
	})
}

func (c *Client) UnsetTicketParent(id string, message string) (store.Ticket, error) {
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
		Message:            message,
	})
}

func (c *Client) GetTicketByID(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.GetTicket(context.Background(), db, id)
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
		return store.GetTicketByRef(context.Background(), db, ref)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(strings.TrimSpace(ref)), nil, &ticket)
	return ticket, err
}

func (c *Client) CloneTicket(id string, message string) (store.Ticket, error) {
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
	var body any
	if message != "" {
		body = messageRequest{Message: message}
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/clone", body, &ticket)
	return ticket, err
}

func (c *Client) ListHistory(id string) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListHistoryEvents(context.Background(), db, id)
	}
	var events []store.HistoryEvent
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(id)+"/history", nil, &events)
	return events, err
}

func (c *Client) ListProjectHistory(projectID int64, limit int) ([]store.HistoryEvent, error) {
	return c.ListProjectHistoryFiltered(projectID, limit, store.HistoryFilter{})
}

func (c *Client) ListProjectHistoryFiltered(projectID int64, limit int, filter store.HistoryFilter) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListProjectHistoryFiltered(context.Background(), db, projectID, limit, filter)
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
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/projects/%d/history?%s", projectID, params.Encode()), nil, &events)
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
		return store.AddComment(context.Background(), db, id, user.ID, comment)
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
		return store.ListComments(context.Background(), db, id)
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
		return store.SetTicketHealth(context.Background(), db, id, score)
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
		return store.AddDependency(context.Background(), db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
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
		return store.DeleteDependency(context.Background(), db, request.ProjectID, request.TicketID, request.DependsOn)
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
		return store.ListDependencies(context.Background(), db, id)
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
	var reader *bytes.Reader
	payload, err := json.Marshal(request)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	reader = bytes.NewReader(payload)

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.baseURL+"/api/tickets/claim", reader)
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


func (c *Client) CreateSdlc(request SdlcRequest) (store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Sdlc{}, err
		}
		defer db.Close()
		return store.CreateSdlc(context.Background(), db, request.Name, request.Description)
	}
	var wf store.Sdlc
	err := c.doJSON(http.MethodPost, "/api/sdlcs", request, &wf)
	return wf, err
}

func (c *Client) ListSdlcs() ([]store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListSdlcs(context.Background(), db)
	}
	var sdlcs []store.Sdlc
	err := c.doJSON(http.MethodGet, "/api/sdlcs", nil, &sdlcs)
	return sdlcs, err
}

func (c *Client) GetSdlc(id int64) (store.SdlcWithStages, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcWithStages{}, err
		}
		defer db.Close()
		return store.GetSdlc(context.Background(), db, id)
	}
	var wf store.SdlcWithStages
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/sdlcs/%d", id), nil, &wf)
	return wf, err
}

func (c *Client) DeleteSdlc(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteSdlc(context.Background(), db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/sdlcs/%d", id), nil, nil)
}

func (c *Client) AddSdlcStage(sdlcID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		defer db.Close()
		return store.AddSdlcStage(context.Background(), db, sdlcID, request.StageName, request.Description, request.AcceptanceCriteria, request.SortOrder)
	}
	var stage store.SdlcStage
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/sdlcs/%d/stages", sdlcID), request, &stage)
	return stage, err
}

func (c *Client) UpdateSdlcStage(stageID int64, request SdlcStageRequest) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		defer db.Close()
		return store.UpdateSdlcStage(context.Background(), db, stageID, request.StageName, request.Description, request.AcceptanceCriteria)
	}
	var stage store.SdlcStage
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), request, &stage)
	return stage, err
}

func (c *Client) GetSdlcStage(stageID int64) (store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcStage{}, err
		}
		defer db.Close()
		return store.GetSdlcStage(context.Background(), db, stageID)
	}
	var stage store.SdlcStage
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), nil, &stage)
	return stage, err
}

func (c *Client) ListSdlcStages(sdlcID int64) ([]store.SdlcStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListSdlcStages(context.Background(), db, sdlcID)
	}
	// Derive from GetSdlc
	var wf store.SdlcWithStages
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/sdlcs/%d", sdlcID), nil, &wf)
	if err != nil {
		return nil, err
	}
	return wf.Stages, nil
}

func (c *Client) RemoveSdlcStage(stageID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveSdlcStage(context.Background(), db, stageID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/sdlcs/stages/%d", stageID), nil, nil)
}

func (c *Client) ReorderSdlcStages(sdlcID int64, stageIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.ReorderSdlcStages(context.Background(), db, sdlcID, stageIDs)
	}
	return c.doJSON(http.MethodPut, fmt.Sprintf("/api/sdlcs/%d/reorder", sdlcID), SdlcReorderRequest{StageIDs: stageIDs}, nil)
}

func (c *Client) ExportSdlc(id int64) (store.SdlcExport, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.SdlcExport{}, err
		}
		defer db.Close()
		return store.ExportSdlc(context.Background(), db, id)
	}
	var export store.SdlcExport
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/sdlcs/%d/export", id), nil, &export)
	return export, err
}

func (c *Client) ImportSdlc(export store.SdlcExport) (store.Sdlc, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Sdlc{}, err
		}
		defer db.Close()
		return store.ImportSdlc(context.Background(), db, export)
	}
	var wf store.Sdlc
	err := c.doJSON(http.MethodPost, "/api/sdlcs/import", export, &wf)
	return wf, err
}

func (c *Client) AddSdlcStageRole(sdlcID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.AddSdlcStageRole(context.Background(), db, sdlcID, stageID, roleID)
	}
	return c.doJSON(http.MethodPost, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d", sdlcID, stageID), map[string]int64{"role_id": roleID}, nil)
}

func (c *Client) RemoveSdlcStageRole(sdlcID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveSdlcStageRole(context.Background(), db, sdlcID, stageID, roleID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d/%d", sdlcID, stageID, roleID), nil, nil)
}

func (c *Client) ReorderSdlcStageRoles(sdlcID, stageID int64, roleIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.ReorderSdlcStageRoles(context.Background(), db, sdlcID, stageID, roleIDs)
	}
	return c.doJSON(http.MethodPut, fmt.Sprintf("/api/sdlcs/stages/roles/%d/%d", sdlcID, stageID), map[string][]int64{"role_ids": roleIDs}, nil)
}

func (c *Client) CompleteTicket(id string, message string) (store.Ticket, error) {
	return c.CloseTicket(id, message)
}

func (c *Client) ReopenTicket(id string, message string) (store.Ticket, error) {
	return c.OpenTicket(id, message)
}

func (c *Client) DraftTicket(id string, message string) (store.Ticket, error) {
	return c.NotReadyTicket(id, message)
}

func (c *Client) UndraftTicket(id string, message string) (store.Ticket, error) {
	return c.ReadyTicket(id, message)
}

func (c *Client) NextTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, _ := c.localUser(db)
		return store.NextTicket(context.Background(), db, id, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%s/next", id), nil, &ticket)
	return ticket, err
}

func (c *Client) PreviousTicket(id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		user, _ := c.localUser(db)
		return store.PreviousTicket(context.Background(), db, id, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%s/previous", id), nil, &ticket)
	return ticket, err
}

func (c *Client) LogTime(ticketID string, request TimeEntryRequest) (store.TimeEntry, error) {
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
		return store.LogTime(context.Background(), db, ticketID, user.ID, request.Minutes, request.Note)
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
		return store.ListTimeEntries(context.Background(), db, ticketID)
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
		return store.DeleteTimeEntry(context.Background(), db, id)
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
		return store.TotalTimeForTicket(context.Background(), db, ticketID)
	}
	var result struct {
		Total int `json:"total"`
	}
	err := c.doJSON(http.MethodGet, "/api/tickets/"+url.PathEscape(ticketID)+"/time/total", nil, &result)
	return result.Total, err
}

func (c *Client) CreateLabel(projectID int64, request LabelRequest) (store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Label{}, err
		}
		defer db.Close()
		return store.CreateLabel(context.Background(), db, projectID, request.Name, request.Color)
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
		return store.ListLabels(context.Background(), db, projectID)
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
		return store.DeleteLabel(context.Background(), db, id)
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
		return store.AddTicketLabel(context.Background(), db, ticketID, labelID)
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
		return store.RemoveTicketLabel(context.Background(), db, ticketID, labelID)
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
		return store.ListTicketLabels(context.Background(), db, ticketID)
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
		return store.CreateStory(context.Background(), db, projectID, title, description, user.ID)
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
		return store.ListStoriesByProject(context.Background(), db, projectID)
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
		return store.GetStory(context.Background(), db, id)
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
		return store.UpdateStory(context.Background(), db, id, title, description)
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
		return store.DeleteStory(context.Background(), db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/stories/%d", id), nil, nil)
}
