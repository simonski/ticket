package server

import "github.com/simonski/ticket/internal/store"

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

type userCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
	PlanSlug string `json:"plan_slug,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type userPlanAssignmentRequest struct {
	PlanID   int64  `json:"plan_id,omitempty"`
	PlanSlug string `json:"plan_slug,omitempty"`
}

type planRequest struct {
	Slug                 string                    `json:"slug"`
	Name                 string                    `json:"name"`
	Description          string                    `json:"description"`
	MaxProjects          int                       `json:"max_projects"`
	MaxPrivateProjects   int                       `json:"max_private_projects"`
	MaxTickets           int                       `json:"max_tickets"`
	MaxTicketsPerProject int                       `json:"max_tickets_per_project"`
	MaxTeamMemberships   int                       `json:"max_team_memberships"`
	MaxAPICallsPerDay    int                       `json:"max_api_calls_per_day"`
	DefaultProjectAlias  string                    `json:"default_project_alias"`
	RegistrationActions  store.RegistrationActions `json:"registration_actions"`
}

type planUpdateRequest struct {
	Slug                 string                     `json:"slug"`
	Name                 string                     `json:"name"`
	Description          string                     `json:"description"`
	MaxProjects          int                        `json:"max_projects"`
	MaxPrivateProjects   int                        `json:"max_private_projects"`
	MaxTickets           int                        `json:"max_tickets"`
	MaxTicketsPerProject int                        `json:"max_tickets_per_project"`
	MaxTeamMemberships   int                        `json:"max_team_memberships"`
	MaxAPICallsPerDay    int                        `json:"max_api_calls_per_day"`
	DefaultProjectAlias  string                     `json:"default_project_alias"`
	RegistrationActions  *store.RegistrationActions `json:"registration_actions,omitempty"`
}

type agentRequest struct {
	Password string `json:"password,omitempty"`
	// AgentRole is a pointer so an explicit empty value (deselecting all roles)
	// clears the agent's roles, while an absent field leaves them unchanged.
	AgentRole *string `json:"agent_role,omitempty"`
	Username  string  `json:"username,omitempty"`
}

type projectRequest struct {
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
	AgentModelProvider string            `json:"agent_model_provider"`
	AgentModelName     string            `json:"agent_model_name"`
	AgentModelURL      string            `json:"agent_model_url"`
	AgentModelAPIKey   string            `json:"agent_model_api_key"`
}

type roleRequest struct {
	ID                 *int64            `json:"id,omitempty"`
	WorkflowID         *int64            `json:"workflow_id,omitempty"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
}

type agentModelConfigRequest struct {
	Provider  string                     `json:"provider"`
	Model     string                     `json:"model"`
	URL       string                     `json:"url"`
	APIKey    string                     `json:"api_key"`
	Providers []store.AgentModelProvider `json:"providers,omitempty"`
}

type workflowRequest struct {
	ID              *int64 `json:"id,omitempty"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	ApprovalPolicy  string `json:"approval_policy,omitempty"`
	ProgressionMode string `json:"progression_mode,omitempty"`
}

type workflowStageRequest struct {
	StageName          string `json:"stage_name"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	WaysOfWorking      string `json:"wow"`
	DefinitionOfReady  string `json:"dor"`
	DefinitionOfDone   string `json:"dod"`
	SortOrder          int    `json:"sort_order"`
	IsBacklogStage     *bool  `json:"is_backlog_stage,omitempty"`
}

type workflowReorderRequest struct {
	StageIDs []int64 `json:"stage_ids"`
}

type workflowStageTransitionRequest struct {
	ToStageIDs []int64 `json:"to_stage_ids"`
}

type teamRequest struct {
	ID           *int64 `json:"id,omitempty"`
	Name         string `json:"name"`
	ParentTeamID *int64 `json:"parent_team_id"`
}

type teamMemberRequest struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type storyRequest struct {
	ID          *int64 `json:"id,omitempty"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type documentRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
	Content     string `json:"content"`
}

type documentLabelRequest struct {
	LabelID int64 `json:"label_id"`
}

type documentFileRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Content     []byte `json:"content"`
}

type ticketRequest struct {
	ProjectID          int64             `json:"project_id"`
	ParentID           *string           `json:"parent_id"`
	Type               string            `json:"type"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AcceptanceCriteria string            `json:"acceptance_criteria"`
	DORMap             store.GuidanceMap `json:"dor_map,omitempty"`
	DODMap             store.GuidanceMap `json:"dod_map,omitempty"`
	ACMap              store.GuidanceMap `json:"ac_map,omitempty"`
	GitRepository      string            `json:"git_repository"`
	GitBranch          string            `json:"git_branch"`
	Status             string            `json:"status"`
	Stage              string            `json:"stage"`
	State              string            `json:"state"`
	Priority           int               `json:"priority"`
	Order              int               `json:"order"`
	EstimateEffort     int               `json:"estimate_effort"`
	EstimateComplete   string            `json:"estimate_complete,omitempty"`
	Assignee           string            `json:"assignee"`
	Message            string            `json:"message,omitempty"`
}

type ticketMarkdownImportRequest struct {
	Content string `json:"content"`
}

type messageRequest struct {
	Message string `json:"message,omitempty"`
}

type interventionRequest struct {
	Outcome string `json:"outcome"`
	Message string `json:"message,omitempty"`
}

type interventionStateRequest struct {
	State string `json:"state"`
}

type phaseSignoffRequest struct {
	Approved bool   `json:"approved"`
	Note     string `json:"note,omitempty"`
}

type inboxEscalateRequest struct {
	Message string `json:"message,omitempty"`
}

type inboxDecisionRequest struct {
	Decision string `json:"decision"`
	Message  string `json:"message,omitempty"`
}

type ticketHealthRequest struct {
	Score int `json:"score"`
}

type commentRequest struct {
	Comment string `json:"comment"`
}

type dependencyRequest struct {
	ProjectID int64  `json:"project_id"`
	TicketID  string `json:"ticket_id"`
	DependsOn string `json:"depends_on"`
}

type ticketClaimRequest struct {
	ProjectID int64   `json:"project_id"`
	TicketID  *string `json:"ticket_id,omitempty"`
	TicketRef string  `json:"ticket_ref,omitempty"`
	DryRun    bool    `json:"dry_run"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}
