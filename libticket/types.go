package libticket

import "github.com/simonski/ticket/internal/store"

// StatusResponse is returned by Service.Status() and describes server health,
// authentication state, and the currently authenticated user if applicable.
type StatusResponse struct {
	Status              string      `json:"status"`
	Authenticated       bool        `json:"authenticated"`
	RegistrationEnabled bool        `json:"registration_enabled,omitempty"`
	ChatEnabled         bool        `json:"chat_enabled,omitempty"`
	ServerVersion       string      `json:"server_version,omitempty"`
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
	SdlcID         *int64 `json:"sdlc_id,omitempty"`
}

type ProjectUpdateRequest struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Notes              string `json:"notes"`
	Status             string `json:"status"`
	Visibility         string `json:"visibility"`
	SdlcID         *int64 `json:"sdlc_id,omitempty"`
}

type ProjectMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type ProjectTeamMemberRequest struct {
	TeamID int64  `json:"team_id"`
	Role   string `json:"role"`
}

type TeamRequest struct {
	Name         string `json:"name"`
	ParentTeamID *int64 `json:"parent_team_id,omitempty"`
}

type TeamMemberRequest struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type SdlcRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SdlcStageRequest struct {
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

type SdlcReorderRequest struct {
	StageIDs []int64 `json:"stage_ids"`
}

type TimeEntryRequest struct {
	Minutes int    `json:"minutes"`
	Note    string `json:"note"`
}

type LabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TicketCreateRequest struct {
	ProjectID          int64   `json:"project_id"`
	ParentID           *string `json:"parent_id,omitempty"`
	CloneOf            *string `json:"clone_of,omitempty"`
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
	Message            string `json:"message,omitempty"`
}

type TicketUpdateRequest struct {
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	AcceptanceCriteria string  `json:"acceptance_criteria"`
	GitRepository      string  `json:"git_repository"`
	GitBranch          string  `json:"git_branch"`
	ParentID           *string `json:"parent_id,omitempty"`
	Assignee           string `json:"assignee"`
	Status             string `json:"status,omitempty"`
	Stage              string `json:"stage,omitempty"`
	State              string `json:"state,omitempty"`
	Priority           int    `json:"priority"`
	Order              int    `json:"order"`
	EstimateEffort     int    `json:"estimate_effort"`
	EstimateComplete   string `json:"estimate_complete,omitempty"`
	Message            string `json:"message,omitempty"`
	Type               string `json:"type,omitempty"`
}

type CommentCreateRequest struct {
	Comment string `json:"comment"`
}

type DependencyRequest struct {
	ProjectID int64  `json:"project_id"`
	TicketID  string `json:"ticket_id"`
	DependsOn string `json:"depends_on"`
}

type TicketRequest struct {
	ProjectID int64   `json:"project_id,omitempty"`
	TicketID  *string `json:"ticket_id,omitempty"`
	TicketRef string `json:"ticket_ref,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type TicketRequestResponse struct {
	Status   string                    `json:"status"`
	Ticket   *store.Ticket             `json:"ticket,omitempty"`
	Project  *store.Project            `json:"project,omitempty"`
	Parents  []store.Ticket            `json:"parents,omitempty"`
	Sdlc *store.SdlcWithStages `json:"sdlc,omitempty"`
	Role     *store.Role               `json:"role,omitempty"`
}

type AgentCreateRequest struct {
	Password string `json:"password,omitempty"`
}

type RoleRequest struct {
	SdlcID             *int64 `json:"sdlc_id,omitempty"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
}

type AgentUpdateRequest struct {
	Password *string `json:"password,omitempty"`
}

type AgentRegisterRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

type AgentRequest struct {
	ID              string  `json:"id"`
	Password        string  `json:"password"`
	ProjectID       int64   `json:"project_id,omitempty"`
	TicketID        *string `json:"ticket_id,omitempty"`
	DryRun          bool   `json:"dry_run,omitempty"`
	ConfigUpdatedAt string `json:"config_updated_at,omitempty"` // timestamp of last config received
}

type AgentTicketUpdateRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Result   string `json:"result"`
}

type AgentWorkResponse struct {
	Status          string                    `json:"status"`
	Project         *store.Project            `json:"project"`
	Ticket          *store.Ticket             `json:"ticket"`
	Parents         []store.Ticket            `json:"parents"`
	Sdlc        *store.SdlcWithStages `json:"sdlc,omitempty"`
	Role            *store.Role               `json:"role,omitempty"`
	Config          map[string]string         `json:"config,omitempty"`           // agent config (if changed or ticket assigned)
	ConfigUpdatedAt string                    `json:"config_updated_at,omitempty"` // timestamp of config state
}
