package libticket

import "github.com/simonski/ticket/internal/store"

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
	WorkflowID         *int64 `json:"workflow_id,omitempty"`
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
	WorkflowID         *int64 `json:"workflow_id,omitempty"`
}

type ProjectMemberRequest struct {
	UserID int64  `json:"user_id"`
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
	UserID   int64  `json:"user_id"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type WorkflowRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WorkflowStageRequest struct {
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	RoleID      *int64 `json:"role_id,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

type WorkflowReorderRequest struct {
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
	Status   string                    `json:"status"`
	Ticket   *store.Ticket             `json:"ticket,omitempty"`
	Project  *store.Project            `json:"project,omitempty"`
	Parents  []store.Ticket            `json:"parents,omitempty"`
	Workflow *store.WorkflowWithStages `json:"workflow,omitempty"`
	Role     *store.Role               `json:"role,omitempty"`
}

type AgentCreateRequest struct {
	Password string `json:"password,omitempty"`
}

type RoleRequest struct {
	Title      string `json:"title"`
	Motivation string `json:"motivation"`
	Goals      string `json:"goals"`
}

type AgentUpdateRequest struct {
	Password *string `json:"password,omitempty"`
}

type AgentRegisterRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

type AgentRequest struct {
	ID        string `json:"id"`
	Password  string `json:"password"`
	ProjectID int64  `json:"project_id,omitempty"`
	TicketID  *int64 `json:"ticket_id,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

type AgentTicketUpdateRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Result   string `json:"result"`
}

type AgentWorkResponse struct {
	Status   string                    `json:"status"`
	Project  *store.Project            `json:"project"`
	Ticket   *store.Ticket             `json:"ticket"`
	Parents  []store.Ticket            `json:"parents"`
	Workflow *store.WorkflowWithStages `json:"workflow,omitempty"`
	Role     *store.Role               `json:"role,omitempty"`
}
