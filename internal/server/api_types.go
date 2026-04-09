package server

import "github.com/simonski/ticket/internal/store"

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type agentRequest struct {
	Password string `json:"password,omitempty"`
}


type projectRequest struct {
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

type roleRequest struct {
	Title      string `json:"title"`
	Motivation string `json:"motivation"`
	Goals      string `json:"goals"`
}

type sdlcRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type sdlcStageRequest struct {
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	RoleID      *int64 `json:"role_id,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

type sdlcReorderRequest struct {
	StageIDs []int64 `json:"stage_ids"`
}

type teamRequest struct {
	Name         string `json:"name"`
	ParentTeamID *int64 `json:"parent_team_id"`
}

type teamMemberRequest struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JobTitle string `json:"job_title"`
}

type storyRequest struct {
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ticketRequest struct {
	ProjectID          int64   `json:"project_id"`
	ParentID           *string `json:"parent_id"`
	Type               string `json:"type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Status             string `json:"status"`
	Stage              string `json:"stage"`
	State              string `json:"state"`
	Priority           int    `json:"priority"`
	Order              int    `json:"order"`
	EstimateEffort     int    `json:"estimate_effort"`
	EstimateComplete   string `json:"estimate_complete,omitempty"`
	Assignee           string `json:"assignee"`
	Message            string `json:"message,omitempty"`
}

type messageRequest struct {
	Message string `json:"message,omitempty"`
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
	ProjectID int64  `json:"project_id"`
	TicketID  *string `json:"ticket_id,omitempty"`
	TicketRef string `json:"ticket_ref,omitempty"`
	DryRun    bool   `json:"dry_run"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}
