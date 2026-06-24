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
	"github.com/simonski/ticket/internal/ticketmarkdown"
)

type Client struct {
	baseURL       string
	username      string
	password      string
	token         string
	gitRepository string
	http          *http.Client
	mode          string
	localDBPath   string

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
	username := strings.TrimSpace(cfg.Username)
	password := ""
	if username != "" {
		password = strings.TrimSpace(cfg.Token)
	}
	return &Client{
		baseURL:       baseURL,
		username:      username,
		password:      password,
		token:         cfg.Token,
		gitRepository: nearestGitRemoteFromCWD(),
		http: &http.Client{
			Timeout:   timeout,
			Transport: newHTTPTransport(),
		},
		mode:        resolved.Mode,
		localDBPath: resolved.DBPath,
	}
}

func newHTTPTransport() *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	cloned := transport.Clone()
	cloned.DisableCompression = false
	return cloned
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
	user, _, err := c.RegisterWithParams(ctx, RegisterRequest{Username: username, Password: password})
	return user, err
}

func (c *Client) RegisterDetailed(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	if c.mode == config.ModeLocal {
		return RegisterResponse{}, errors.New("ticket register requires remote mode with a configured server")
	}
	var response RegisterResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/register", req, &response)
	return response, err
}

func (c *Client) RegisterWithParams(ctx context.Context, req RegisterRequest) (store.User, string, error) {
	response, err := c.RegisterDetailed(ctx, req)
	return response.User, response.Password, err
}

func (c *Client) Login(ctx context.Context, username, password string) (AuthResponse, error) {
	if c.mode == config.ModeLocal {
		return AuthResponse{}, errors.New("ticket login requires remote mode with a configured server")
	}
	var response AuthResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/login", map[string]string{
		"username": username,
		"password": password,
	}, &response)
	return response, err
}

func (c *Client) StartPasskeyLogin(ctx context.Context, req PasskeyLoginStartRequest) (PasskeyStartResponse, error) {
	if c.mode == config.ModeLocal {
		return PasskeyStartResponse{}, errors.New("ticket passkey login requires remote mode with a configured server")
	}
	var response PasskeyStartResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/auth/passkey/login/start", req, &response)
	return response, err
}

func (c *Client) StartPasskeyRegistration(ctx context.Context, req PasskeyRegistrationStartRequest) (PasskeyStartResponse, error) {
	if c.mode == config.ModeLocal {
		return PasskeyStartResponse{}, errors.New("ticket passkey enrollment requires remote mode with a configured server")
	}
	var response PasskeyStartResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/auth/passkey/register/start", req, &response)
	return response, err
}

func (c *Client) PollPasskey(ctx context.Context, code string) (PasskeyPollResponse, error) {
	if c.mode == config.ModeLocal {
		return PasskeyPollResponse{}, errors.New("ticket passkey polling requires remote mode with a configured server")
	}
	var response PasskeyPollResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/auth/passkey/poll", map[string]string{"code": strings.TrimSpace(code)}, &response)
	return response, err
}

func (c *Client) Logout(ctx context.Context) error {
	if c.mode == config.ModeLocal {
		return errors.New("ticket logout requires remote mode with a configured server session")
	}
	return c.doJSON(ctx, http.MethodPost, "/api/logout", nil, nil)
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	if c.mode == config.ModeLocal {
		if _, err := os.Stat(c.localDBPath); err != nil {
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
		registrationAutoApprove, regApproveErr := store.RegistrationAutoApprove(ctx, db)
		if regApproveErr != nil {
			return StatusResponse{}, regApproveErr
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: registrationEnabled, RegistrationAutoApprove: registrationAutoApprove}, nil
		case err != nil:
			return StatusResponse{}, err
		case !user.Enabled:
			return StatusResponse{Status: "ok", Authenticated: false, RegistrationEnabled: registrationEnabled, RegistrationAutoApprove: registrationAutoApprove}, nil
		}
		return StatusResponse{
			Status:                  "ok",
			Authenticated:           true,
			RegistrationEnabled:     registrationEnabled,
			RegistrationAutoApprove: registrationAutoApprove,
			User:                    &user,
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

func (c *Client) SetRegistrationAutoApprove(ctx context.Context, enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetRegistrationAutoApprove(ctx, db, enabled)
	}
	return c.doJSON(ctx, http.MethodPost, "/api/config/registration", map[string]any{
		"enabled":      true,
		"auto_approve": enabled,
	}, nil)
}

func (c *Client) GetEmailConfig(ctx context.Context) (store.EmailConfig, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.EmailConfig{}, err
		}
		return store.GetEmailConfig(ctx, db)
	}
	// The server masks the password (returns only has_password). Surface a
	// non-empty sentinel so callers can tell a password is configured without
	// ever receiving the secret; it is display-only and never sent back.
	var resp struct {
		store.EmailConfig
		HasPassword bool `json:"has_password"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/email/settings", nil, &resp); err != nil {
		return store.EmailConfig{}, err
	}
	cfg := resp.EmailConfig
	if resp.HasPassword {
		cfg.Password = "********"
	}
	return cfg, nil
}

func (c *Client) SetEmailConfig(ctx context.Context, cfg store.EmailConfig, updatePassword bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetEmailConfig(ctx, db, cfg, updatePassword)
	}
	body := map[string]any{
		"enabled":      cfg.Enabled,
		"host":         cfg.Host,
		"port":         cfg.Port,
		"username":     cfg.Username,
		"from_address": cfg.FromAddress,
		"from_name":    cfg.FromName,
		"security":     cfg.Security,
	}
	// Only send the password when it should change; an empty password preserves
	// the stored secret server-side.
	if updatePassword {
		body["password"] = cfg.Password
	}
	return c.doJSON(ctx, http.MethodPut, "/api/email/settings", body, nil)
}

func (c *Client) SetEmailEnabled(ctx context.Context, enabled bool) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetEmailEnabled(ctx, db, enabled)
	}
	return c.doJSON(ctx, http.MethodPut, "/api/email/enabled", map[string]any{"enabled": enabled}, nil)
}

func (c *Client) ListPlans(ctx context.Context) ([]store.Plan, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListPlans(ctx, db)
	}
	var plans []store.Plan
	err := c.doJSON(ctx, http.MethodGet, "/api/plans", nil, &plans)
	return plans, err
}

func (c *Client) DefaultPlan(ctx context.Context) (store.Plan, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Plan{}, err
		}
		return store.DefaultPlan(ctx, db)
	}
	var plan store.Plan
	err := c.doJSON(ctx, http.MethodGet, "/api/plans/default", nil, &plan)
	return plan, err
}

func (c *Client) SetDefaultPlan(ctx context.Context, slug string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.SetDefaultPlanSlug(ctx, db, slug)
	}
	return c.doJSON(ctx, http.MethodPost, "/api/plans/default", map[string]any{"slug": strings.TrimSpace(slug)}, nil)
}

func (c *Client) CreateUser(ctx context.Context, username, password string) (store.User, error) {
	user, _, err := c.CreateUserWithParams(ctx, UserCreateRequest{Username: username, Password: password})
	return user, err
}

func (c *Client) CreateUserWithParams(ctx context.Context, req UserCreateRequest) (store.User, string, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.User{}, "", err
		}
		password := strings.TrimSpace(req.Password)
		generatedPassword := ""
		if password == "" {
			generatedPassword, err = store.GeneratePassword(24)
			if err != nil {
				return store.User{}, "", err
			}
			password = generatedPassword
		}
		user, err := store.CreateUserWithParams(ctx, db, store.UserCreateParams{
			Username:      req.Username,
			PlainPassword: password,
			Email:         req.Email,
			Role:          req.Role,
			Enabled:       req.Enabled == nil || *req.Enabled,
			PlanSlug:      req.PlanSlug,
		})
		return user, generatedPassword, err
	}
	var response UserCreateResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/users", req, &response)
	return response.User, response.Password, err
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

func (c *Client) GetMyDefaultProject(ctx context.Context) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Project{}, err
		}
		project, err := store.GetUserDefaultProject(ctx, db, user.ID)
		if err != nil {
			return store.Project{}, err
		}
		role, ok, err := store.ProjectRoleForUser(ctx, db, project.ID, user.ID)
		if err != nil {
			return store.Project{}, err
		}
		if (!ok || role == "") && project.Visibility != store.ProjectVisibilityPublic {
			return store.Project{}, store.ErrProjectNotFound
		}
		return project, nil
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodGet, "/api/users/me/default-project", nil, &project)
	if err == nil {
		return project, nil
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return store.Project{}, store.ErrProjectNotFound
	}
	return project, err
}

func (c *Client) SetMyDefaultProject(ctx context.Context, projectRef string) (store.Project, error) {
	ref := strings.TrimSpace(projectRef)
	if ref == "" {
		return store.Project{}, errors.New("project reference is required")
	}
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Project{}, err
		}
		var project store.Project
		switch strings.ToLower(ref) {
		case "public":
			project, err = store.GetProjectByAlias(ctx, db, "public", "")
		case "private":
			project, err = store.GetProjectByAlias(ctx, db, "private", user.ID)
		default:
			project, err = store.GetProject(ctx, db, ref)
		}
		if err != nil {
			return store.Project{}, err
		}
		role, ok, err := store.ProjectRoleForUser(ctx, db, project.ID, user.ID)
		if err != nil {
			return store.Project{}, err
		}
		if (!ok || role == "") && project.Visibility != store.ProjectVisibilityPublic {
			return store.Project{}, store.ErrUnauthorized
		}
		if err := store.SetUserDefaultProject(ctx, db, user.ID, project.ID); err != nil {
			return store.Project{}, err
		}
		return project, nil
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodPut, "/api/users/me/default-project", map[string]string{"project_ref": ref}, &project)
	if err == nil {
		return project, nil
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return store.Project{}, store.ErrProjectNotFound
	}
	return project, err
}

func (c *Client) ClearMyDefaultProject(ctx context.Context) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return err
		}
		return store.ClearUserDefaultProject(ctx, db, user.ID)
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/users/me/default-project", nil, nil)
}

func (c *Client) ListMyNotifications(ctx context.Context, status string, limit int) ([]store.UserNotification, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return nil, err
		}
		return store.ListUserNotifications(ctx, db, user.ID, strings.TrimSpace(status), limit)
	}
	path := "/api/users/me/notifications"
	var query []string
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		query = append(query, "status="+url.QueryEscape(trimmed))
	}
	if limit > 0 {
		query = append(query, fmt.Sprintf("limit=%d", limit))
	}
	if len(query) > 0 {
		path += "?" + strings.Join(query, "&")
	}
	var notifications []store.UserNotification
	err := c.doJSON(ctx, http.MethodGet, path, nil, &notifications)
	return notifications, err
}

func (c *Client) MarkNotificationRead(ctx context.Context, notificationID int64) (store.UserNotification, error) {
	if notificationID <= 0 {
		return store.UserNotification{}, errors.New("notification id must be greater than zero")
	}
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.UserNotification{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.UserNotification{}, err
		}
		return store.MarkUserNotificationRead(ctx, db, notificationID, user.ID)
	}
	var notification store.UserNotification
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/users/me/notifications/%d/read", notificationID), map[string]any{}, &notification)
	return notification, err
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

func (c *Client) CreateAgent(ctx context.Context, request AgentCreateRequest) (agent store.Agent, password string, err error) {
	if c.mode == config.ModeLocal {
		db, openErr := c.openLocalDB()
		if openErr != nil {
			return store.Agent{}, "", openErr
		}
		return store.CreateAgent(ctx, db, request.Password)
	}
	var response struct {
		Agent    store.Agent `json:"agent"`
		Password string      `json:"password"`
	}
	err = c.doJSON(ctx, http.MethodPost, "/api/agents", request, &response)
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

func (c *Client) SetAgentConfig(ctx context.Context, agentID, key, value string) error {
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

func (c *Client) DeleteAgentConfig(ctx context.Context, agentID, key string) error {
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

func (c *Client) AgentRecommendReady(ctx context.Context, agentID, password, ticketID string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		agent, err := store.AuthenticateAgent(ctx, db, agentID, password)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetRecommendedReady(ctx, db, ticketID, true, agent.Username, agent.ID)
	}
	var ticket store.Ticket
	err := c.doJSONBasicAuth(ctx, http.MethodPost, fmt.Sprintf("/api/agents/tickets/%s/recommend-ready", ticketID), agentID, password, nil, &ticket)
	return ticket, err
}

func (c *Client) AgentRefineTicket(ctx context.Context, id string, request AgentRefineRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		agent, err := store.AuthenticateAgent(ctx, db, request.ID, request.Password)
		if err != nil {
			return store.Ticket{}, err
		}
		stories := make([]store.RefinementStory, 0, len(request.Stories))
		for _, st := range request.Stories {
			stories = append(stories, store.RefinementStory{Title: st.Title, Description: st.Description, AcceptanceCriteria: st.AcceptanceCriteria})
		}
		return store.ApplyRefinementTurn(ctx, db, id, agent.Username, agent.ID, store.RefinementTurnParams{
			Message: request.Message, ProposalKind: request.ProposalKind,
			Description: request.Description, AcceptanceCriteria: request.AcceptanceCriteria, Stories: stories,
		})
	}
	body := map[string]any{
		"message":             request.Message,
		"proposal_kind":       request.ProposalKind,
		"description":         request.Description,
		"acceptance_criteria": request.AcceptanceCriteria,
		"stories":             request.Stories,
	}
	var ticket store.Ticket
	err := c.doJSONBasicAuth(ctx, http.MethodPost, fmt.Sprintf("/api/agents/tickets/%s/refine", id), request.ID, request.Password, body, &ticket)
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
			ID:                 request.ID,
			Prefix:             request.Prefix,
			Title:              request.Title,
			Description:        request.Description,
			AcceptanceCriteria: request.AcceptanceCriteria,
			GitRepository:      request.GitRepository,
			Notes:              request.Notes,
			Visibility:         request.Visibility,
			AcceptsNewMembers:  request.AcceptsNewMembers,
			CreatedBy:          user.ID,
			WorkflowID:         request.WorkflowID,
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

func (c *Client) CreateProjectAccessRequest(ctx context.Context, projectRef, message string) (store.ProjectAccessRequest, error) {
	projectRef = strings.TrimSpace(projectRef)
	if projectRef == "" {
		return store.ProjectAccessRequest{}, errors.New("project reference is required")
	}
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		return store.CreateProjectAccessRequest(ctx, db, project.ID, user.ID, message)
	}
	var request store.ProjectAccessRequest
	err := c.doJSON(ctx, http.MethodPost, "/api/projects/"+url.PathEscape(projectRef)+"/access-requests", map[string]string{
		"message": strings.TrimSpace(message),
	}, &request)
	return request, err
}

func (c *Client) ListProjectAccessRequests(ctx context.Context, projectRef, status string) ([]store.ProjectAccessRequest, error) {
	projectRef = strings.TrimSpace(projectRef)
	if projectRef == "" {
		return nil, errors.New("project reference is required")
	}
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return nil, err
		}
		return store.ListProjectAccessRequests(ctx, db, project.ID, strings.TrimSpace(status))
	}
	path := "/api/projects/" + url.PathEscape(projectRef) + "/access-requests"
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		path += "?status=" + url.QueryEscape(trimmed)
	}
	var requests []store.ProjectAccessRequest
	err := c.doJSON(ctx, http.MethodGet, path, nil, &requests)
	return requests, err
}

func (c *Client) ListMyProjectAccessRequests(ctx context.Context, status string) ([]store.ProjectAccessRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return nil, err
		}
		return store.ListUserProjectAccessRequests(ctx, db, user.ID, strings.TrimSpace(status))
	}
	path := "/api/users/me/access-requests"
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		path += "?status=" + url.QueryEscape(trimmed)
	}
	var requests []store.ProjectAccessRequest
	err := c.doJSON(ctx, http.MethodGet, path, nil, &requests)
	return requests, err
}

func (c *Client) SetProjectAccessRequestStatus(ctx context.Context, projectRef string, requestID int64, status, message string) (store.ProjectAccessRequest, error) {
	projectRef = strings.TrimSpace(projectRef)
	if projectRef == "" {
		return store.ProjectAccessRequest{}, errors.New("project reference is required")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "approved" && status != "rejected" {
		return store.ProjectAccessRequest{}, errors.New("invalid project access request status")
	}
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		request, err := store.GetProjectAccessRequestByID(ctx, db, requestID)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		if request.ProjectID != project.ID {
			return store.ProjectAccessRequest{}, store.ErrProjectAccessRequestNotFound
		}
		request, err = store.SetProjectAccessRequestStatus(ctx, db, requestID, status, message, user.Username)
		if err != nil {
			return store.ProjectAccessRequest{}, err
		}
		if _, err := store.CreateUserNotification(ctx, db, store.BuildProjectAccessDecisionNotification(request, user.Username)); err != nil {
			return store.ProjectAccessRequest{}, err
		}
		return request, nil
	}
	action := "approve"
	if status == "rejected" {
		action = "reject"
	}
	var request store.ProjectAccessRequest
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%s/access-requests/%d/%s", url.PathEscape(projectRef), requestID, action), map[string]any{
		"message": strings.TrimSpace(message),
	}, &request)
	return request, err
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
			AcceptsNewMembers:  request.AcceptsNewMembers,
			WorkflowID:         request.WorkflowID,
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

func (c *Client) RenameProjectPrefix(ctx context.Context, id int64, newPrefix string) (int, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return 0, err
		}
		return store.RenameProjectPrefix(ctx, db, id, newPrefix)
	}
	var response RenameProjectPrefixResponse
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/rename-prefix", id), map[string]string{"prefix": newPrefix}, &response)
	return response.TicketsUpdated, err
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

func (c *Client) ListProjectGitRepositories(ctx context.Context, projectRef string) ([]string, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return nil, err
		}
		return store.ListProjectGitRepositories(ctx, db, project.ID)
	}
	var repositories []string
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/repositories", url.PathEscape(projectRef)), nil, &repositories)
	return repositories, err
}

func (c *Client) FindProjectByGitRepository(ctx context.Context, repository string) (store.Project, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Project{}, err
		}
		return store.GetProjectByGitRepository(ctx, db, repository)
	}
	var project store.Project
	err := c.doJSON(ctx, http.MethodGet, "/api/projects/by-repository?repository="+url.QueryEscape(strings.TrimSpace(repository)), nil, &project)
	if err == nil {
		return project, nil
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return store.Project{}, store.ErrProjectNotFound
	}
	return store.Project{}, err
}

func (c *Client) AddProjectGitRepository(ctx context.Context, projectRef, repository string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return err
		}
		return store.AddProjectGitRepository(ctx, db, project.ID, repository)
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%s/repositories", url.PathEscape(projectRef)), ProjectRepositoryRequest{Repository: repository}, nil)
}

func (c *Client) RemoveProjectGitRepository(ctx context.Context, projectRef, repository string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return err
		}
		return store.RemoveProjectGitRepository(ctx, db, project.ID, repository)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%s/repositories/%s", url.PathEscape(projectRef), url.PathEscape(repository)), nil, nil)
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
		return store.CreateTeamWithParams(ctx, db, store.TeamCreateParams{
			ID:           request.ID,
			Name:         request.Name,
			ParentTeamID: request.ParentTeamID,
		})
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

func (c *Client) ImportTicketMarkdown(ctx context.Context, request TicketMarkdownImportRequest) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Ticket{}, err
		}
		doc, err := ticketmarkdown.Parse(request.Content)
		if err != nil {
			return store.Ticket{}, err
		}
		current, err := store.GetTicket(ctx, db, doc.ID)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.UpdateTicket(ctx, db, current.ID, store.TicketUpdateParams{
			Title:              doc.Title,
			Description:        doc.Description,
			AcceptanceCriteria: doc.AcceptanceCriteria,
			DORMap:             current.DORMap,
			DODMap:             current.DODMap,
			ACMap:              current.ACMap,
			GitRepository:      current.GitRepository,
			GitBranch:          current.GitBranch,
			ParentID:           current.ParentID,
			Assignee:           current.Assignee,
			Priority:           current.Priority,
			Order:              current.Order,
			EstimateEffort:     current.EstimateEffort,
			EstimateComplete:   current.EstimateComplete,
			Type:               doc.Type,
			UpdatedBy:          user.ID,
			ActorUsername:      user.Username,
			ActorRole:          "admin",
		})
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/import-markdown", request, &ticket)
	return ticket, err
}

func (c *Client) CloseTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) OpenTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) ArchiveTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) UnarchiveTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) ReadyTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) NotReadyTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) SetTicketWorkflow(ctx context.Context, id string, workflowID int64) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.SetTicketWorkflow(ctx, db, id, workflowID)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(id)+"/workflow", map[string]int64{"workflow_id": workflowID}, &ticket)
	return ticket, err
}

func (c *Client) UnsetTicketWorkflow(ctx context.Context, id string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		return store.UnsetTicketWorkflow(ctx, db, id)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodDelete, "/api/tickets/"+url.PathEscape(id)+"/workflow", nil, &ticket)
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

func (c *Client) SetTicketParent(ctx context.Context, id, parentID, message string) (store.Ticket, error) {
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

func (c *Client) CreatePullRequest(ctx context.Context, request PullRequestRequest) (store.PullRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.PullRequest{}, err
		}
		createdBy := ""
		if user, userErr := store.GetUserByUsername(ctx, db, localUsername()); userErr == nil {
			createdBy = user.ID
		}
		return store.CreatePullRequest(ctx, db, store.PullRequestParams{
			TicketID:     request.TicketID,
			Title:        request.Title,
			Description:  request.Description,
			Repository:   request.Repository,
			SourceBranch: request.SourceBranch,
			TargetBranch: request.TargetBranch,
			Status:       request.Status,
			Provider:     request.Provider,
			URL:          request.URL,
			CreatedBy:    createdBy,
		})
	}
	var pr store.PullRequest
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(strings.TrimSpace(request.TicketID))+"/pull-requests", request, &pr)
	return pr, err
}

func (c *Client) GetPullRequest(ctx context.Context, id int64) (store.PullRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.PullRequest{}, err
		}
		return store.GetPullRequest(ctx, db, id)
	}
	var pr store.PullRequest
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/pull-requests/%d", id), nil, &pr)
	return pr, err
}

func (c *Client) ListPullRequestsByTicket(ctx context.Context, ticketID string) ([]store.PullRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListPullRequestsByTicket(ctx, db, ticketID)
	}
	var prs []store.PullRequest
	err := c.doJSON(ctx, http.MethodGet, "/api/tickets/"+url.PathEscape(strings.TrimSpace(ticketID))+"/pull-requests", nil, &prs)
	return prs, err
}

func (c *Client) ListPullRequestsByProject(ctx context.Context, projectRef string) ([]store.PullRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		project, err := store.GetProject(ctx, db, projectRef)
		if err != nil {
			return nil, err
		}
		return store.ListPullRequestsByProject(ctx, db, project.ID)
	}
	var prs []store.PullRequest
	err := c.doJSON(ctx, http.MethodGet, "/api/projects/"+url.PathEscape(strings.TrimSpace(projectRef))+"/pull-requests", nil, &prs)
	return prs, err
}

func (c *Client) SetPullRequestStatus(ctx context.Context, id int64, status string) (store.PullRequest, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.PullRequest{}, err
		}
		return store.UpdatePullRequestStatus(ctx, db, id, status)
	}
	var pr store.PullRequest
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/pull-requests/%d/status", id), map[string]string{"status": status}, &pr)
	return pr, err
}

func (c *Client) UnsetTicketParent(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) CloneTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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

func (c *Client) ListProjectWorkItemQueue(ctx context.Context, projectID int64, strategy string, limit int) ([]store.WorkItemQueueCandidate, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListProjectWorkItemQueue(ctx, db, projectID, strategy, limit)
	}
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if strings.TrimSpace(strategy) != "" {
		params.Set("strategy", strings.TrimSpace(strategy))
	}
	path := fmt.Sprintf("/api/projects/%d/work-items/queue", projectID)
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var queue []store.WorkItemQueueCandidate
	err := c.doJSON(ctx, http.MethodGet, path, nil, &queue)
	return queue, err
}

func (c *Client) AddComment(ctx context.Context, id, comment string) (store.Comment, error) {
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
			response.Workflow = ctx.Workflow
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/tickets/claim", reader)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.username != "" || c.password != "" {
		httpReq.SetBasicAuth(c.username, c.password)
	} else if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	setRequestContextHeaders(httpReq, c.gitRepository)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return TicketRequestResponse{}, friendlyConnectionError(err, c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return TicketRequestResponse{}, statusErrorFromResponse(resp)
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

func (c *Client) InterveneTicket(ctx context.Context, id string, request InterventionRequest) (InterventionResponse, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return InterventionResponse{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var response InterventionResponse
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/intervene", id), request, &response)
	return response, err
}

func (c *Client) GetInterventionState(ctx context.Context, ticketID string) (store.InterventionState, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.InterventionState{}, err
		}
		return store.GetInterventionState(ctx, db, ticketID)
	}
	var response store.InterventionState
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/tickets/%s/intervention-state", ticketID), nil, &response)
	return response, err
}

func (c *Client) SetInterventionState(ctx context.Context, ticketID, state string) (store.InterventionState, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.InterventionState{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.InterventionState{}, err
		}
		return store.SetInterventionState(ctx, db, ticketID, state, user.ID, user.ID)
	}
	var response store.InterventionState
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/intervention-state", ticketID), InterventionStateRequest{State: state}, &response)
	return response, err
}

func (c *Client) ListWorkItems(ctx context.Context, ticketID, status, assigneeType string, limit int) ([]store.WorkItem, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListWorkItemsByTicketWithParams(ctx, db, ticketID, store.WorkItemListParams{
			Status:       status,
			AssigneeType: assigneeType,
			Limit:        limit,
		})
	}
	values := url.Values{}
	if strings.TrimSpace(status) != "" {
		values.Set("status", strings.TrimSpace(status))
	}
	if strings.TrimSpace(assigneeType) != "" {
		values.Set("assignee_type", strings.TrimSpace(assigneeType))
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	path := fmt.Sprintf("/api/tickets/%s/work-items", ticketID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var items []store.WorkItem
	err := c.doJSON(ctx, http.MethodGet, path, nil, &items)
	return items, err
}

func (c *Client) ActWorkItem(ctx context.Context, ticketID, workItemID, action string, request WorkItemActionRequest) (store.WorkItem, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkItem{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.WorkItem{}, err
		}
		switch action {
		case "reassign":
			return store.ReassignWorkItem(ctx, db, ticketID, workItemID, request.Assignee, user.Username, user.ID)
		case "cancel":
			return store.CancelWorkItem(ctx, db, ticketID, workItemID, request.Message, user.Username, user.ID)
		case "retry":
			return store.RetryWorkItem(ctx, db, ticketID, workItemID, request.Assignee, user.Username, user.ID)
		case "feedback":
			return store.AddWorkItemFeedback(ctx, db, ticketID, workItemID, request.Message, request.CommitRef, user.Username, user.ID)
		default:
			return store.WorkItem{}, errors.New("invalid work-item action")
		}
	}
	var response store.WorkItem
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/tickets/%s/work-items/%s/%s", ticketID, workItemID, action), request, &response)
	return response, err
}

func (c *Client) CreateWorkflow(ctx context.Context, request WorkflowRequest) (store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Workflow{}, err
		}
		return store.CreateWorkflowWithParams(ctx, db, request.ID, request.Name, request.Description)
	}
	var wf store.Workflow
	err := c.doJSON(ctx, http.MethodPost, "/api/workflows", request, &wf)
	return wf, err
}

func (c *Client) ListWorkflows(ctx context.Context) ([]store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListWorkflows(ctx, db, 0, 0)
	}
	var workflows []store.Workflow
	err := c.doJSON(ctx, http.MethodGet, "/api/workflows", nil, &workflows)
	return workflows, err
}

func (c *Client) GetWorkflow(ctx context.Context, id int64) (store.WorkflowWithStages, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowWithStages{}, err
		}
		return store.GetWorkflow(ctx, db, id)
	}
	var wf store.WorkflowWithStages
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/workflows/%d", id), nil, &wf)
	return wf, err
}

func (c *Client) DeleteWorkflow(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteWorkflow(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/workflows/%d", id), nil, nil)
}

func (c *Client) AddWorkflowStage(ctx context.Context, workflowID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		stage, err := store.AddWorkflowStageWithDefinitions(ctx, db, workflowID, request.StageName, wow, dor, request.DefinitionOfDone, request.SortOrder)
		if err != nil {
			return stage, err
		}
		if request.IsBacklogStage != nil {
			if bErr := store.SetWorkflowStageBacklog(ctx, db, stage.ID, *request.IsBacklogStage); bErr != nil {
				return stage, bErr
			}
			stage.IsBacklogStage = *request.IsBacklogStage
		}
		return stage, nil
	}
	var stage store.WorkflowStage
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/workflows/%d/stages", workflowID), request, &stage)
	return stage, err
}

func (c *Client) UpdateWorkflowStage(ctx context.Context, stageID int64, request WorkflowStageRequest) (store.WorkflowStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
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
		return store.UpdateWorkflowStageWithDefinitions(ctx, db, stageID, request.StageName, wow, dor, request.DefinitionOfDone, request.IsBacklogStage)
	}
	var stage store.WorkflowStage
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/workflows/stages/%d", stageID), request, &stage)
	return stage, err
}

func (c *Client) GetWorkflowStage(ctx context.Context, stageID int64) (store.WorkflowStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowStage{}, err
		}
		return store.GetWorkflowStage(ctx, db, stageID)
	}
	var stage store.WorkflowStage
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/workflows/stages/%d", stageID), nil, &stage)
	return stage, err
}

func (c *Client) ListWorkflowStages(ctx context.Context, workflowID int64) ([]store.WorkflowStage, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListWorkflowStages(ctx, db, workflowID)
	}
	// Derive from GetWorkflow
	var wf store.WorkflowWithStages
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/workflows/%d", workflowID), nil, &wf)
	if err != nil {
		return nil, err
	}
	return wf.Stages, nil
}

func (c *Client) RemoveWorkflowStage(ctx context.Context, stageID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveWorkflowStage(ctx, db, stageID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/workflows/stages/%d", stageID), nil, nil)
}

func (c *Client) ReorderWorkflowStages(ctx context.Context, workflowID int64, stageIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.ReorderWorkflowStages(ctx, db, workflowID, stageIDs)
	}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/workflows/%d/reorder", workflowID), WorkflowReorderRequest{StageIDs: stageIDs}, nil)
}

func (c *Client) ExportWorkflow(ctx context.Context, id int64) (store.WorkflowExport, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.WorkflowExport{}, err
		}
		return store.ExportWorkflow(ctx, db, id)
	}
	var export store.WorkflowExport
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/workflows/%d/export", id), nil, &export)
	return export, err
}

func (c *Client) ImportWorkflow(ctx context.Context, export store.WorkflowExport) (store.Workflow, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Workflow{}, err
		}
		return store.ImportWorkflow(ctx, db, export)
	}
	var wf store.Workflow
	err := c.doJSON(ctx, http.MethodPost, "/api/workflows/import", export, &wf)
	return wf, err
}

func (c *Client) AddWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.AddWorkflowStageRole(ctx, db, workflowID, stageID, roleID)
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/workflows/stages/roles/%d/%d", workflowID, stageID), map[string]int64{"role_id": roleID}, nil)
}

func (c *Client) RemoveWorkflowStageRole(ctx context.Context, workflowID, stageID, roleID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveWorkflowStageRole(ctx, db, workflowID, stageID, roleID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/workflows/stages/roles/%d/%d/%d", workflowID, stageID, roleID), nil, nil)
}

func (c *Client) ReorderWorkflowStageRoles(ctx context.Context, workflowID, stageID int64, roleIDs []int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.ReorderWorkflowStageRoles(ctx, db, workflowID, stageID, roleIDs)
	}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/workflows/stages/roles/%d/%d", workflowID, stageID), map[string][]int64{"role_ids": roleIDs}, nil)
}

func (c *Client) CompleteTicket(ctx context.Context, id, message string) (store.Ticket, error) {
	return c.CloseTicket(ctx, id, message)
}

func (c *Client) ReopenTicket(ctx context.Context, id, message string) (store.Ticket, error) {
	return c.OpenTicket(ctx, id, message)
}

func (c *Client) DraftTicket(ctx context.Context, id, message string) (store.Ticket, error) {
	return c.NotReadyTicket(ctx, id, message)
}

func (c *Client) UndraftTicket(ctx context.Context, id, message string) (store.Ticket, error) {
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
		return store.CreateLabelWithParams(ctx, db, store.LabelCreateParams{
			ID:        request.ID,
			ProjectID: projectID,
			Name:      request.Name,
			Color:     request.Color,
		})
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
	ProjectID   int64  `json:"project_id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (c *Client) CreateStory(ctx context.Context, projectID int64, title, description string) (store.Story, error) {
	return c.CreateStoryWithRequest(ctx, StoryCreateRequest{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
	})
}

func (c *Client) CreateStoryWithRequest(ctx context.Context, request StoryCreateRequest) (store.Story, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Story{}, err
		}
		user, err := c.localUser(ctx, db)
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
	var created store.Story
	err := c.doJSON(ctx, http.MethodPost, "/api/stories", request, &created)
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

type releaseRequest struct {
	Title      string `json:"title"`
	Purpose    string `json:"purpose"`
	TargetDate string `json:"target_date"`
}

type releaseStatusRequest struct {
	Status string `json:"status"`
}

type ticketReleaseRequest struct {
	ReleaseID *int64 `json:"release_id"`
}

func (c *Client) ListReleases(ctx context.Context, projectID int64) ([]store.Release, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListReleases(ctx, db, int(projectID))
	}
	var releases []store.Release
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/releases", projectID), nil, &releases)
	return releases, err
}

func (c *Client) GetRelease(ctx context.Context, id int64) (store.Release, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Release{}, err
		}
		return store.GetRelease(ctx, db, int(id))
	}
	var release store.Release
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/releases/%d", id), nil, &release)
	return release, err
}

func (c *Client) CreateRelease(ctx context.Context, projectID int64, title, purpose, targetDate string) (store.Release, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Release{}, err
		}
		return store.CreateRelease(ctx, db, int(projectID), title, purpose, targetDate)
	}
	var created store.Release
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/releases", projectID), releaseRequest{Title: title, Purpose: purpose, TargetDate: targetDate}, &created)
	return created, err
}

func (c *Client) UpdateRelease(ctx context.Context, id int64, title, purpose, targetDate string) (store.Release, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Release{}, err
		}
		return store.UpdateRelease(ctx, db, int(id), title, purpose, targetDate)
	}
	var updated store.Release
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/releases/%d", id), releaseRequest{Title: title, Purpose: purpose, TargetDate: targetDate}, &updated)
	return updated, err
}

func (c *Client) SetReleaseStatus(ctx context.Context, id int64, status string) (store.Release, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Release{}, err
		}
		return store.SetReleaseStatus(ctx, db, int(id), status)
	}
	var updated store.Release
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/releases/%d/status", id), releaseStatusRequest{Status: status}, &updated)
	return updated, err
}

func (c *Client) DeleteRelease(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteRelease(ctx, db, int(id))
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/releases/%d", id), nil, nil)
}

func (c *Client) AddFeatureToRelease(ctx context.Context, featureTicketID string, releaseID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.AssignFeatureToRelease(ctx, db, featureTicketID, int(releaseID))
	}
	rid := releaseID
	return c.doJSON(ctx, http.MethodPut, "/api/tickets/"+url.PathEscape(featureTicketID)+"/release", ticketReleaseRequest{ReleaseID: &rid}, nil)
}

func (c *Client) RemoveFeatureFromRelease(ctx context.Context, featureTicketID string) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveFeatureFromRelease(ctx, db, featureTicketID)
	}
	return c.doJSON(ctx, http.MethodPut, "/api/tickets/"+url.PathEscape(featureTicketID)+"/release", ticketReleaseRequest{ReleaseID: nil}, nil)
}

func (c *Client) CloneFeature(ctx context.Context, featureTicketID string) (store.Ticket, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Ticket{}, err
		}
		user, err := c.localUser(ctx, db)
		if err != nil {
			return store.Ticket{}, err
		}
		return store.CloneFeature(ctx, db, featureTicketID, user.Username, user.ID)
	}
	var ticket store.Ticket
	err := c.doJSON(ctx, http.MethodPost, "/api/tickets/"+url.PathEscape(featureTicketID)+"/clone", nil, &ticket)
	return ticket, err
}

func (c *Client) CreateDocument(ctx context.Context, projectID int64, request DocumentRequest) (store.Document, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Document{}, err
		}
		return store.CreateDocument(ctx, db, projectID, request.Title, request.Description, request.Notes, request.Content)
	}
	var document store.Document
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/documents", projectID), request, &document)
	return document, err
}

func (c *Client) ListDocuments(ctx context.Context, projectID int64) ([]store.Document, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListDocumentsByProject(ctx, db, projectID)
	}
	var documents []store.Document
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/documents", projectID), nil, &documents)
	return documents, err
}

func (c *Client) GetDocument(ctx context.Context, id int64) (store.Document, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Document{}, err
		}
		return store.GetDocument(ctx, db, id)
	}
	var document store.Document
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/documents/%d", id), nil, &document)
	return document, err
}

func (c *Client) UpdateDocument(ctx context.Context, id int64, request DocumentRequest) (store.Document, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.Document{}, err
		}
		return store.UpdateDocument(ctx, db, id, request.Title, request.Description, request.Notes, request.Content)
	}
	var document store.Document
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/documents/%d", id), request, &document)
	return document, err
}

func (c *Client) DeleteDocument(ctx context.Context, id int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteDocument(ctx, db, id)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/documents/%d", id), nil, nil)
}

func (c *Client) AddDocumentLabel(ctx context.Context, documentID int64, request DocumentLabelRequest) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.AddDocumentLabel(ctx, db, documentID, request.LabelID)
	}
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/documents/%d/labels", documentID), request, nil)
}

func (c *Client) RemoveDocumentLabel(ctx context.Context, documentID, labelID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.RemoveDocumentLabel(ctx, db, documentID, labelID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/documents/%d/labels?label_id=%d", documentID, labelID), nil, nil)
}

func (c *Client) ListDocumentLabels(ctx context.Context, documentID int64) ([]store.Label, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListDocumentLabels(ctx, db, documentID)
	}
	var labels []store.Label
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/documents/%d/labels", documentID), nil, &labels)
	return labels, err
}

func (c *Client) AddDocumentFile(ctx context.Context, documentID int64, request DocumentFileRequest) (store.DocumentFile, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.DocumentFile{}, err
		}
		return store.AddDocumentFile(ctx, db, documentID, request.FileName, request.ContentType, request.Content)
	}
	var file store.DocumentFile
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/documents/%d/files", documentID), request, &file)
	return file, err
}

func (c *Client) ListDocumentFiles(ctx context.Context, documentID int64) ([]store.DocumentFile, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return nil, err
		}
		return store.ListDocumentFiles(ctx, db, documentID)
	}
	var files []store.DocumentFile
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/documents/%d/files", documentID), nil, &files)
	return files, err
}

func (c *Client) GetDocumentFile(ctx context.Context, documentID, fileID int64) (store.DocumentFile, error) {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return store.DocumentFile{}, err
		}
		return store.GetDocumentFile(ctx, db, documentID, fileID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/documents/%d/files/%d", c.baseURL, documentID, fileID), http.NoBody)
	if err != nil {
		return store.DocumentFile{}, err
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	setRequestContextHeaders(req, c.gitRepository)
	resp, err := c.http.Do(req)
	if err != nil {
		return store.DocumentFile{}, friendlyConnectionError(err, c.baseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return store.DocumentFile{}, statusErrorFromResponse(resp)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.DocumentFile{}, err
	}
	file := store.DocumentFile{
		ID:         fileID,
		DocumentID: documentID,
		Content:    data,
	}
	if disp := strings.TrimSpace(resp.Header.Get("Content-Disposition")); disp != "" {
		if parts := strings.Split(disp, "filename="); len(parts) == 2 {
			file.FileName = strings.Trim(parts[1], `"`)
		}
	}
	file.ContentType = strings.TrimSpace(resp.Header.Get("Content-Type"))
	file.SizeBytes = int64(len(data))
	return file, nil
}

func (c *Client) DeleteDocumentFile(ctx context.Context, documentID, fileID int64) error {
	if c.mode == config.ModeLocal {
		db, err := c.openLocalDB()
		if err != nil {
			return err
		}
		return store.DeleteDocumentFile(ctx, db, documentID, fileID)
	}
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/documents/%d/files/%d", documentID, fileID), nil, nil)
}
