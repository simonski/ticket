package libticket

import "github.com/simonski/ticket/internal/store"

// StatusResponse is returned by Service.Status and describes server health,
// authentication state, and the currently authenticated user if applicable.
type StatusResponse struct {
	Status                  string      `json:"status"`
	Authenticated           bool        `json:"authenticated"`
	RegistrationEnabled     bool        `json:"registration_enabled,omitempty"`
	RegistrationAutoApprove bool        `json:"registration_auto_approve,omitempty"`
	ChatEnabled             bool        `json:"chat_enabled,omitempty"`
	ServerVersion           string      `json:"server_version,omitempty"`
	User                    *store.User `json:"user,omitempty"`
}

type CountSummary = store.CountSummary

type ProjectCreateRequest struct {
	ID                 *int64            `json:"id,omitempty"`
	Prefix             string            `json:"prefix"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string            `json:"git_repository"`
	Notes              string            `json:"notes"`
	Visibility         string            `json:"visibility"`
	AcceptsNewMembers  bool              `json:"accepts_new_members"`
	WorkflowID         *int64            `json:"workflow_id,omitempty"`
}

type ProjectUpdateRequest struct {
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string            `json:"git_repository"`
	Notes              string            `json:"notes"`
	Status             string            `json:"status"`
	Visibility         string            `json:"visibility"`
	AcceptsNewMembers  bool              `json:"accepts_new_members"`
	WorkflowID         *int64            `json:"workflow_id,omitempty"`
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
	ID           *int64 `json:"id,omitempty"`
	Name         string `json:"name"`
	ParentTeamID *int64 `json:"parent_team_id,omitempty"`
}

type TeamMemberRequest struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type WorkflowRequest struct {
	ID          *int64 `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WorkflowStageRequest struct {
	StageName          string `json:"stage_name"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	WaysOfWorking      string `json:"wow"`
	DefinitionOfReady  string `json:"dor"`
	DefinitionOfDone   string `json:"dod"`
	SortOrder          int    `json:"sort_order"`
	IsBacklogStage     *bool  `json:"is_backlog_stage,omitempty"`
}

type WorkflowPhaseRequest = WorkflowStageRequest

type WorkflowReorderRequest struct {
	StageIDs []int64 `json:"stage_ids"`
}

type TimeEntryRequest struct {
	Minutes int    `json:"minutes"`
	Note    string `json:"note"`
}

type LabelRequest struct {
	ID    *int64 `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type StoryCreateRequest struct {
	ID          *int64 `json:"id,omitempty"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type DocumentRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
	Content     string `json:"content"`
}

type DocumentLabelRequest struct {
	LabelID int64 `json:"label_id"`
}

type DocumentFileUploadRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Content     []byte `json:"content"`
}

type TicketCreateRequest struct {
	ProjectID          int64             `json:"project_id"`
	ParentID           *string           `json:"parent_id,omitempty"`
	CloneOf            *string           `json:"clone_of,omitempty"`
	Type               string            `json:"type"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string            `json:"git_repository"`
	GitBranch          string            `json:"git_branch"`
	Priority           int               `json:"priority"`
	EstimateEffort     int               `json:"estimate_effort"`
	EstimateComplete   string            `json:"estimate_complete,omitempty"`
	Assignee           string            `json:"assignee"`
	Status             string            `json:"status,omitempty"`
	Stage              string            `json:"stage,omitempty"`
	State              string            `json:"state,omitempty"`
	Message            string            `json:"message,omitempty"`
}

// PullRequestRequest creates a pull request for a ticket. Repository, branches,
// provider, and url are optional; the CLI fills sensible defaults by inspecting
// the project's repositories and the current git branch.
type PullRequestRequest struct {
	TicketID     string `json:"ticket_id"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	Repository   string `json:"repository,omitempty"`
	SourceBranch string `json:"source_branch,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
	Status       string `json:"status,omitempty"`
	Provider     string `json:"provider,omitempty"`
	URL          string `json:"url,omitempty"`
}

type TicketUpdateRequest struct {
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string            `json:"git_repository"`
	GitBranch          string            `json:"git_branch"`
	ParentID           *string           `json:"parent_id,omitempty"`
	Assignee           string            `json:"assignee"`
	Status             string            `json:"status,omitempty"`
	Stage              string            `json:"stage,omitempty"`
	State              string            `json:"state,omitempty"`
	Priority           int               `json:"priority"`
	Order              int               `json:"order"`
	EstimateEffort     int               `json:"estimate_effort"`
	EstimateComplete   string            `json:"estimate_complete,omitempty"`
	Message            string            `json:"message,omitempty"`
	Type               string            `json:"type,omitempty"`
}

type TicketMarkdownImportRequest struct {
	Content string `json:"content"`
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
	TicketRef string  `json:"ticket_ref,omitempty"`
	DryRun    bool    `json:"dry_run,omitempty"`
}

type TicketRequestResponse struct {
	Status   string                    `json:"status"`
	Ticket   *store.Ticket             `json:"ticket,omitempty"`
	Project  *store.Project            `json:"project,omitempty"`
	Parents  []store.Ticket            `json:"parents,omitempty"`
	Workflow *store.WorkflowWithStages `json:"workflow,omitempty"`
	Role     *store.Role               `json:"role,omitempty"`
}

type InterventionRequest struct {
	Outcome string `json:"outcome"`
	Message string `json:"message,omitempty"`
}

type InterventionResponse struct {
	Ticket       store.Ticket  `json:"ticket"`
	FollowUp     *store.Ticket `json:"follow_up,omitempty"`
	Decision     string        `json:"decision"`
	Intervention bool          `json:"intervention"`
}

type AgentCreateRequest struct {
	Password string `json:"password,omitempty"`
}

type RoleRequest struct {
	ID                 *int64            `json:"id,omitempty"`
	WorkflowID         *int64            `json:"workflow_id,omitempty"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
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
	DryRun          bool    `json:"dry_run,omitempty"`
	ConfigUpdatedAt string  `json:"config_updated_at,omitempty"` // timestamp of last config received
}

type AgentTicketUpdateRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Result   string `json:"result"`
}

type AgentRefineStory struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
}

type AgentRefineRequest struct {
	ID                 string             `json:"id"`
	Password           string             `json:"password"`
	Message            string             `json:"message"`
	ProposalKind       string             `json:"proposal_kind"`
	Description        string             `json:"description"`
	AcceptanceCriteria string             `json:"acceptance_criteria"`
	Stories            []AgentRefineStory `json:"stories"`
}

type AgentWorkResponse struct {
	Status          string                    `json:"status"`
	Project         *store.Project            `json:"project"`
	Ticket          *store.Ticket             `json:"ticket"`
	Parents         []store.Ticket            `json:"parents"`
	Workflow        *store.WorkflowWithStages `json:"workflow,omitempty"`
	Role            *store.Role               `json:"role,omitempty"`
	Config          map[string]string         `json:"config,omitempty"`
	ConfigUpdatedAt string                    `json:"config_updated_at,omitempty"`
	Reasons         []string                  `json:"reasons,omitempty"`
}
