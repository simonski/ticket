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
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	mode    string
}

type AuthResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}

type StatusResponse struct {
	Status              string      `json:"status"`
	Authenticated       bool        `json:"authenticated"`
	RegistrationEnabled bool        `json:"registration_enabled,omitempty"`
	ServerVersion       string      `json:"server_version"`
	User                *store.User `json:"user,omitempty"`
}

type CountSummary = store.CountSummary

type ProjectCreateRequest struct {
	Prefix             string `json:"prefix"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Notes              string `json:"notes"`
	Visibility         string `json:"visibility"`
}

type ProjectUpdateRequest struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Notes              string `json:"notes"`
	Visibility         string `json:"visibility"`
}

type ProjectMemberRequest struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}

type TicketCreateRequest struct {
	ProjectID          int64  `json:"project_id"`
	ParentID           *int64 `json:"parent_id,omitempty"`
	CloneOf            *int64 `json:"clone_of,omitempty"`
	Type               string `json:"type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Priority           int    `json:"priority"`
	EstimateEffort     int    `json:"estimate_effort"`
	EstimateComplete   string `json:"estimate_complete,omitempty"`
	Assignee           string `json:"assignee"`
	Status             string `json:"status,omitempty"`
	Stage              string `json:"stage,omitempty"`
	State              string `json:"state,omitempty"`
}

type TicketUpdateRequest struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	ParentID           *int64 `json:"parent_id,omitempty"`
	Assignee           string `json:"assignee"`
	Status             string `json:"status,omitempty"`
	Stage              string `json:"stage,omitempty"`
	State              string `json:"state,omitempty"`
	Priority           int    `json:"priority"`
	Order              int    `json:"order"`
	EstimateEffort     int    `json:"estimate_effort"`
	EstimateComplete   string `json:"estimate_complete,omitempty"`
}

type TicketHealthRequest struct {
	Score int `json:"score"`
}

type CommentCreateRequest struct {
	Comment string `json:"comment"`
}

type DependencyRequest struct {
	ProjectID int64 `json:"project_id"`
	TicketID  int64 `json:"ticket_id"`
	DependsOn int64 `json:"depends_on"`
}

type TicketRequest struct {
	ProjectID int64  `json:"project_id,omitempty"`
	TicketID  *int64 `json:"ticket_id,omitempty"`
	TicketRef string `json:"ticket_ref,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type TicketRequestResponse struct {
	Status string        `json:"status"`
	Ticket *store.Ticket `json:"ticket,omitempty"`
}

type AgentWorkResponse struct {
	Status  string         `json:"status"`
	Project *store.Project `json:"project"`
	Ticket  *store.Ticket  `json:"ticket"`
	Parents []store.Ticket `json:"parents"`
}

type AgentCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Password    string `json:"password,omitempty"`
}

type RoleRequest struct {
	Title      string `json:"title"`
	Motivation string `json:"motivation"`
	Goals      string `json:"goals"`
}

type AgentUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Password    *string `json:"password,omitempty"`
}

type AgentRegisterRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type AgentRequest struct {
	Name      string `json:"name"`
	Password  string `json:"password"`
	ProjectID int64  `json:"project_id,omitempty"`
	TicketID  *int64 `json:"ticket_id,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type AgentTicketUpdateRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Result   string `json:"result"`
}

func resolveRequestLifecycle(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

func New(cfg config.Config) *Client {
	mode, err := config.ResolveMode()
	if err != nil {
		mode = config.ModeLocal
	}
	return &Client{
		baseURL: strings.TrimRight(config.ResolveServerURL(cfg), "/"),
		token:   cfg.Token,
		http:    http.DefaultClient,
		mode:    mode,
	}
}

func (c *Client) Register(username, password string) (store.User, error) {
	if c.mode == config.ModeLocal {
		return store.User{}, errors.New("ticket register requires TICKET_MODE=remote")
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
		return AuthResponse{}, errors.New("ticket login requires TICKET_MODE=remote")
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
		return errors.New("ticket logout requires TICKET_MODE=remote")
	}
	return c.doJSON(http.MethodPost, "/api/logout", nil, nil)
}

func (c *Client) Status() (StatusResponse, error) {
	if c.mode == config.ModeLocal {
		dbPath, err := config.ResolveDatabasePath()
		if err != nil {
			return StatusResponse{}, err
		}
		if _, err := os.Stat(dbPath); err != nil {
			return StatusResponse{}, err
		}
		db, err := store.Open(dbPath)
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
		return store.CreateAgent(db, request.Name, request.Description, request.Password)
	}
	var response struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	err := c.doJSON(http.MethodPost, "/api/agents", request, &response)
	return response.Agent, response.Password, err
}

func (c *Client) SetAgentEnabled(id int64, enabled bool) (store.Agent, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/agents/%d/%s", id, action), nil, &agent)
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

func (c *Client) UpdateAgent(id int64, request AgentUpdateRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		defer db.Close()
		return store.UpdateAgent(db, id, store.AgentUpdateParams{
			Name:        request.Name,
			Description: request.Description,
			Password:    request.Password,
		})
	}
	var agent store.Agent
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/agents/%d", id), request, &agent)
	return agent, err
}

func (c *Client) DeleteAgent(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteAgent(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/agents/%d", id), nil, nil)
}

func (c *Client) RegisterAgent(request AgentRegisterRequest) (store.Agent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Agent{}, err
		}
		defer db.Close()
		agent, err := store.AuthenticateAgent(db, request.Name, request.Password)
		if err != nil {
			return store.Agent{}, err
		}
		return store.TouchAgent(db, agent.ID, "online")
	}
	var response struct {
		Agent store.Agent `json:"agent"`
	}
	err := c.doJSON(http.MethodPost, "/api/agents/register", request, &response)
	return response.Agent, err
}

func (c *Client) RequestAgentWork(request AgentRequest) (AgentWorkResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return AgentWorkResponse{}, err
		}
		defer db.Close()
		agent, err := store.AuthenticateAgent(db, request.Name, request.Password)
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
		currentAssigned, hadCurrent, err := store.CurrentAssignedTicketForUser(db, projectID, agent.Name)
		if err != nil {
			return AgentWorkResponse{}, err
		}
		ticket, status, err := store.RequestTicket(db, store.TicketRequestParams{
			ProjectID: projectID,
			TicketID:  request.TicketID,
			Username:  agent.Name,
			UserID:    0,
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
	err := c.doJSON(http.MethodPost, "/api/agents/request", request, &response)
	return response, err
}

func (c *Client) AgentUpdateTicket(id int64, request AgentTicketUpdateRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		agent, err := store.AuthenticateAgent(db, request.Name, request.Password)
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
			Assignee:           agent.Name,
			Stage:              store.StageDone,
			State:              store.StateSuccess,
			Priority:           current.Priority,
			Order:              current.Order,
			EstimateEffort:     current.EstimateEffort,
			EstimateComplete:   current.EstimateComplete,
			UpdatedBy:          0,
			ActorUsername:      agent.Name,
			ActorRole:          "admin",
		})
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/agents/tickets/%d/update", id), request, &ticket)
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
			CreatedBy:          user.ID,
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

func (c *Client) RemoveProjectMember(projectID, userID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.RemoveProjectMember(db, projectID, userID)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/projects/%d/users/%d", projectID, userID), nil, nil)
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
		stage, state, err := resolveRequestLifecycle(request.Status, request.Stage, request.State)
		if err != nil {
			return store.Ticket{}, err
		}
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
			Stage:              stage,
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

func (c *Client) UpdateTicket(id int64, request TicketUpdateRequest) (store.Ticket, error) {
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
		stage, state, err := resolveRequestLifecycle(request.Status, request.Stage, request.State)
		if err != nil {
			return store.Ticket{}, err
		}
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
	var ticket store.Ticket
	err := c.doJSON(http.MethodPut, fmt.Sprintf("/api/tickets/%d", id), request, &ticket)
	return ticket, err
}

func (c *Client) CloseTicket(id int64) (store.Ticket, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/close", id), nil, &ticket)
	return ticket, err
}

func (c *Client) OpenTicket(id int64) (store.Ticket, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/open", id), nil, &ticket)
	return ticket, err
}

func (c *Client) ArchiveTicket(id int64) (store.Ticket, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/archive", id), nil, &ticket)
	return ticket, err
}

func (c *Client) UnarchiveTicket(id int64) (store.Ticket, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/unarchive", id), nil, &ticket)
	return ticket, err
}

func (c *Client) DeleteTicket(id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		defer db.Close()
		return store.DeleteTicket(db, id)
	}
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/tickets/%d", id), nil, nil)
}

func (c *Client) SetTicketParent(id, parentID int64) (store.Ticket, error) {
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

func (c *Client) UnsetTicketParent(id int64) (store.Ticket, error) {
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

func (c *Client) GetTicketByID(id int64) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.GetTicket(db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%d", id), nil, &ticket)
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

func (c *Client) CloneTicket(id int64) (store.Ticket, error) {
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
		return store.CloneTicket(db, id, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/clone", id), nil, &ticket)
	return ticket, err
}

func (c *Client) ListHistory(id int64) ([]store.HistoryEvent, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListHistoryEvents(db, id)
	}
	var events []store.HistoryEvent
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%d/history", id), nil, &events)
	return events, err
}

func (c *Client) AddComment(id int64, comment string) (store.Comment, error) {
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
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/comments", id), CommentCreateRequest{Comment: comment}, &created)
	return created, err
}

func (c *Client) ListComments(id int64) ([]store.Comment, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListComments(db, id)
	}
	var comments []store.Comment
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%d/comments", id), nil, &comments)
	return comments, err
}

func (c *Client) SetTicketHealth(id int64, score int) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		defer db.Close()
		return store.SetTicketHealth(db, id, score)
	}
	var updated store.Ticket
	err := c.doJSON(http.MethodPost, fmt.Sprintf("/api/tickets/%d/health", id), TicketHealthRequest{Score: score}, &updated)
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
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/dependencies?project_id=%d&ticket_id=%d&depends_on=%d", request.ProjectID, request.TicketID, request.DependsOn), nil, nil)
}

func (c *Client) ListDependencies(id int64) ([]store.Dependency, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return store.ListDependencies(db, id)
	}
	var dependencies []store.Dependency
	err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/tickets/%d/dependencies", id), nil, &dependencies)
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
		return TicketRequestResponse{}, err
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

func (c *Client) openLocalDB() (*sql.DB, error) {
	path, err := config.ResolveDatabasePath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

func (c *Client) localUser(db *sql.DB) (store.User, error) {
	return ensureLocalUser(db, localUsername())
}

func ensureLocalUser(db *sql.DB, username string) (store.User, error) {
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
	user, err := store.CreateUser(db, username, "local-mode", "admin")
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

func localUsername() string {
	return "admin"
}

func getenvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	httpRequest, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(httpRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("request failed with status %s", resp.Status)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
