package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrTicketHasChildren = errors.New("ticket has child tickets")
	ErrTicketClosed      = errors.New("ticket is closed")
	ErrTicketArchived    = errors.New("ticket is archived")
)

type Ticket struct {
	ID                 string    `json:"ticket_id"`
	ProjectID          int64     `json:"project_id"`
	ParentID           *string   `json:"parent_id,omitempty"`
	CloneOf            *string   `json:"clone_of,omitempty"`
	Type               string    `json:"type"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	GitRepository      string    `json:"git_repository"`
	GitBranch          string    `json:"git_branch"`
	WorkflowID         *int64    `json:"workflow_id,omitempty"`
	WorkflowStageID    *int64    `json:"workflow_stage_id,omitempty"`
	Stage              string    `json:"stage"`
	State              string    `json:"state"`
	Status             string    `json:"status"`
	Priority           int       `json:"priority"`
	Order              int       `json:"order"`
	EstimateEffort     int       `json:"estimate_effort"`
	EstimateComplete   string    `json:"estimate_complete,omitempty"`
	HealthScore        int       `json:"health_score"`
	Assignee           string    `json:"assignee"`
	Author             string    `json:"author"`
	Comments           []Comment `json:"comments,omitempty"`
	Ready              bool      `json:"ready"`
	Open               bool      `json:"open"`
	Archived           bool      `json:"archived"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

type TicketCreateParams struct {
	ProjectID          int64
	ParentID           *string
	CloneOf            *string
	WorkflowID         *int64
	Type               string
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	Priority           int
	Order              int
	EstimateEffort     int
	EstimateComplete   string
	Assignee           string
	Author             string
	State              string
	CreatedBy          string
}

type TicketUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	ParentID           *string
	Assignee           string
	Stage              string
	State              string
	Priority           int
	Order              int
	EstimateEffort     int
	EstimateComplete   string
	UpdatedBy          string
	ActorUsername      string
	ActorRole          string
}

type TicketListParams struct {
	ProjectID       int64
	Type            string
	Stage           string
	State           string
	Status          string
	Search          string
	Assignee        string
	Limit           int
	IncludeArchived bool
}

type TicketRequestParams struct {
	ProjectID int64
	TicketID  *string
	TicketRef string
	Username  string
	UserID    string
	DryRun    bool
}

func CreateTicket(ctx context.Context, db *sql.DB, params TicketCreateParams) (Ticket, error) {
	params.Type = normalizeTicketType(params.Type)
	params.Title = strings.TrimSpace(params.Title)
	if params.ProjectID == 0 {
		return Ticket{}, errors.New("project is required")
	}
	if params.Title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	if !validTicketType(params.Type) {
		return Ticket{}, fmt.Errorf("invalid ticket type %q", params.Type)
	}
	if params.ParentID != nil {
		parent, err := GetTicket(ctx, db, *params.ParentID)
		if err != nil {
			return Ticket{}, err
		}
		if parent.ProjectID != params.ProjectID {
			return Ticket{}, errors.New("parent ticket must be in the same project")
		}
		if err := validateTicketParenting(parent.Type, params.Type); err != nil {
			return Ticket{}, err
		}
	}
	if err := validateEstimateComplete(params.EstimateComplete); err != nil {
		return Ticket{}, err
	}
	state := normalizeState(params.State)
	if state == "" {
		state = StateIdle
	}
	if !ValidState(state) {
		return Ticket{}, fmt.Errorf("invalid state %q", params.State)
	}
	if state == StateActive && strings.TrimSpace(params.Assignee) == "" {
		return Ticket{}, errors.New("active ticket requires assignee")
	}
	priority := params.Priority
	if priority == 0 {
		priority = 1
	}
	order := params.Order

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Ticket{}, err
	}
	defer tx.Rollback()
	var projectPrefix string
	var nextSequence int64
	var projectWorkflowID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT prefix, ticket_sequence + 1, workflow_id FROM projects WHERE project_id = ?`, params.ProjectID).Scan(&projectPrefix, &nextSequence, &projectWorkflowID); err != nil {
		return Ticket{}, err
	}
	// Resolve effective workflow: ticket param → parent chain → project
	var effectiveWorkflowID sql.NullInt64
	var ticketWorkflowID *int64 // stored on the ticket itself (NULL = inherit)
	if params.WorkflowID != nil {
		effectiveWorkflowID = sql.NullInt64{Int64: *params.WorkflowID, Valid: true}
		ticketWorkflowID = params.WorkflowID
	} else if params.ParentID != nil {
		// Walk parent chain for explicit workflow
		pid := params.ParentID
		for pid != nil {
			var pwf sql.NullInt64
			var ppid sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT workflow_id, parent_id FROM tickets WHERE ticket_id = ?`, *pid).Scan(&pwf, &ppid); err != nil {
				break
			}
			if pwf.Valid {
				effectiveWorkflowID = pwf
				break
			}
			if ppid.Valid {
				pid = &ppid.String
			} else {
				pid = nil
			}
		}
		if !effectiveWorkflowID.Valid {
			effectiveWorkflowID = projectWorkflowID
		}
	} else {
		effectiveWorkflowID = projectWorkflowID
	}
	// Resolve initial workflow stage (first stage by sort_order)
	var workflowStageID *int64
	stage := StageDesign // fallback
	if effectiveWorkflowID.Valid {
		var wsID int64
		var stageName string
		err := tx.QueryRowContext(ctx, `SELECT workflow_stage_id, stage_name FROM workflow_stages WHERE workflow_id = ? ORDER BY sort_order LIMIT 1`, effectiveWorkflowID.Int64).Scan(&wsID, &stageName)
		if err == nil {
			workflowStageID = &wsID
			stage = stageName
		}
	}
	key, err := generateTicketKey(projectPrefix, params.Type, nextSequence)
	if err != nil {
		return Ticket{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tickets (ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, author, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key, params.ProjectID, nullableString(params.ParentID), nullableString(params.CloneOf), params.Type, params.Title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.GitBranch), nullableInt64(ticketWorkflowID), nullableInt64(workflowStageID), stage, state, RenderLifecycleStatus(stage, state), priority, order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), 0, strings.TrimSpace(params.Assignee), strings.TrimSpace(params.Author), nullableUserID(params.CreatedBy))
	if err != nil {
		return Ticket{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET ticket_sequence = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, nextSequence, params.ProjectID); err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(); err != nil {
		return Ticket{}, err
	}
	ticket, err := GetTicket(ctx, db, key)
	if err != nil {
		return Ticket{}, err
	}
	if err := AddHistoryEvent(ctx, db, ticket.ProjectID, ticket.ID, "ticket_created", map[string]any{
		"key":               ticket.ID,
		"type":              ticket.Type,
		"title":             ticket.Title,
		"stage":             ticket.Stage,
		"state":             ticket.State,
		"status":            ticket.Status,
		"estimate_effort":   ticket.EstimateEffort,
		"estimate_complete": ticket.EstimateComplete,
	}, params.CreatedBy); err != nil {
		return Ticket{}, err
	}
	if err := syncAncestorLifecycle(ctx, db, params.ParentID, params.CreatedBy); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, key)
}

func UpdateTicket(ctx context.Context, db *sql.DB, id string, params TicketUpdateParams) (Ticket, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	if err := validateEstimateComplete(params.EstimateComplete); err != nil {
		return Ticket{}, err
	}
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	hasChildren, err := ticketHasChildren(ctx, db, current.ID)
	if err != nil {
		return Ticket{}, err
	}
	if params.ParentID != nil {
		parent, err := GetTicket(ctx, db, *params.ParentID)
		if err != nil {
			return Ticket{}, err
		}
		if parent.ID == current.ID {
			return Ticket{}, errors.New("cannot set ticket as its own parent")
		}
		if parent.ProjectID != current.ProjectID {
			return Ticket{}, errors.New("parent ticket must be in the same project")
		}
		if err := validateTicketParenting(parent.Type, current.Type); err != nil {
			return Ticket{}, err
		}
	}
	// An explicit stage override (e.g. board drag-and-drop) is allowed to reopen a
	// closed ticket — lifecycle moves take precedence over the closed flag.
	explicitStageOverride := normalizeOptional(params.Stage) != ""
	if !current.Open && !explicitStageOverride {
		return Ticket{}, ErrTicketClosed
	}
	if current.Archived {
		return Ticket{}, ErrTicketArchived
	}
	reopened := !current.Open && explicitStageOverride
	assignee := strings.TrimSpace(params.Assignee)
	nextGitRepository := strings.TrimSpace(params.GitRepository)
	if nextGitRepository == "" {
		nextGitRepository = strings.TrimSpace(current.GitRepository)
	}
	nextGitBranch := strings.TrimSpace(params.GitBranch)
	if nextGitBranch == "" {
		nextGitBranch = strings.TrimSpace(current.GitBranch)
	}
	if err := validateTicketAssignmentChange(current.Assignee, assignee, params.ActorUsername, params.ActorRole); err != nil {
		return Ticket{}, err
	}
	if assignee != "" {
		target, err := GetUserByUsername(ctx, db, assignee)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Ticket{}, errors.New("user not found")
			}
			return Ticket{}, err
		}
		if !target.Enabled {
			return Ticket{}, errors.New("user is disabled")
		}
	}

	explicitStage := normalizeOptional(params.Stage) != ""
	explicitState := normalizeOptional(params.State) != ""
	if hasChildren && (explicitState || explicitStage) {
		return Ticket{}, errors.New("ticket has children; state is derived from descendants")
	}
	state := current.State
	stage := current.Stage
	workflowStageID := current.WorkflowStageID
	// Direct stage override (e.g. drag-and-drop on the board)
	if explicitStage {
		nextStage := strings.ToLower(strings.TrimSpace(params.Stage))
		if !ValidStage(nextStage) {
			return Ticket{}, fmt.Errorf("invalid stage %q", params.Stage)
		}
		if nextStage != current.Stage {
			stage = nextStage
			// Determine appropriate state for the new stage
			if explicitState {
				state = normalizeState(params.State)
			} else {
				if stage == StageDone {
					state = StateSuccess
				} else {
					state = StateIdle
				}
			}
			// Update workflow_stage_id to match the new stage (if a workflow is attached)
			if current.WorkflowStageID != nil {
				var workflowID int64
				if err := db.QueryRowContext(ctx, `SELECT workflow_id FROM workflow_stages WHERE workflow_stage_id = ?`, *current.WorkflowStageID).Scan(&workflowID); err == nil {
					var wsID int64
					if err := db.QueryRowContext(ctx, `SELECT workflow_stage_id FROM workflow_stages WHERE workflow_id = ? AND stage_name = ? LIMIT 1`, workflowID, stage).Scan(&wsID); err == nil {
						workflowStageID = &wsID
					} else {
						workflowStageID = nil
					}
				}
			}
			// Stage changed; state already resolved above — skip state-only processing
			goto writeTicket
		}
		// Stage unchanged; fall through to state-only processing if state was also given
	}
	if explicitState {
		nextState := normalizeState(params.State)
		if !ValidState(nextState) {
			return Ticket{}, fmt.Errorf("invalid state %q", params.State)
		}
		if nextState == StateActive && strings.TrimSpace(assignee) == "" {
			return Ticket{}, errors.New("active ticket requires assignee")
		}
		// Check if ticket is at final workflow stage with success — can't reopen
		if current.State == StateSuccess && current.WorkflowStageID != nil {
			nextID, _, err := getNextWorkflowStage(ctx, db, *current.WorkflowStageID)
			if err == nil && nextID == nil {
				// Final stage with success: ticket is "done"
				return Ticket{}, errors.New("done ticket cannot be reopened")
			}
		}
		if nextState != current.State {
			if params.ActorRole != "admin" && strings.TrimSpace(current.Assignee) != strings.TrimSpace(params.ActorUsername) {
				return Ticket{}, ErrForbidden
			}
		}
		state = nextState
		// Auto-advance: when state becomes success on a non-final stage, advance to next stage
		if state == StateSuccess && workflowStageID != nil {
			nextStageID, nextStageName, err := getNextWorkflowStage(ctx, db, *workflowStageID)
			if err == nil && nextStageID != nil {
				workflowStageID = nextStageID
				stage = nextStageName
				state = StateIdle
			}
			// If no next stage (final stage), stay at current stage with success state
		}
	}

writeTicket:
	openVal := 1
	if !reopened && !current.Open {
		openVal = 0
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, git_branch = ?, parent_id = ?, assignee = ?, workflow_stage_id = ?, stage = ?, state = ?, status = ?, priority = ?, sort_order = ?, estimate_effort = ?, estimate_complete = ?, open = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), nextGitRepository, nextGitBranch, nullableString(params.ParentID), assignee, nullableInt64(workflowStageID), stage, state, RenderLifecycleStatus(stage, state), params.Priority, params.Order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), openVal, id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	ticket, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Stage != ticket.Stage || current.State != ticket.State {
		if err := addTicketLifecycleHistoryEvent(ctx, db, current, ticket.Stage, ticket.State, "manual update", params.ActorUsername, params.UpdatedBy); err != nil {
			return Ticket{}, err
		}
	}
	if err := AddHistoryEvent(ctx, db, ticket.ProjectID, ticket.ID, "ticket_updated", map[string]any{
		"key":                 ticket.ID,
		"title":               ticket.Title,
		"description":         ticket.Description,
		"acceptance_criteria": ticket.AcceptanceCriteria,
		"git_repository":      ticket.GitRepository,
		"git_branch":          ticket.GitBranch,
		"assignee":            ticket.Assignee,
		"stage":               ticket.Stage,
		"state":               ticket.State,
		"status":              ticket.Status,
		"priority":            ticket.Priority,
		"order":               ticket.Order,
		"estimate_effort":     ticket.EstimateEffort,
		"estimate_complete":   ticket.EstimateComplete,
		"parent_id":           ticket.ParentID,
	}, params.UpdatedBy); err != nil {
		return Ticket{}, err
	}
	if err := syncRelatedLifecycle(ctx, db, params.UpdatedBy, current.ParentID, params.ParentID, &current.ID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

func SetTicketOpen(ctx context.Context, db *sql.DB, id string, open bool, actorUsername string, actorID string) (Ticket, error) {
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Open == open {
		return current, nil
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET open = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, boolToInt(open), id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	if err := addTicketOpenHistoryEvent(ctx, db, current, current.Open, open, actorUsername, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

func SetTicketArchived(ctx context.Context, db *sql.DB, id string, archived bool, actorUsername string, actorID string) (Ticket, error) {
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Archived == archived {
		return current, nil
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET archived = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, boolToInt(archived), id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	if err := addTicketArchiveHistoryEvent(ctx, db, current, current.Archived, archived, actorUsername, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

func SetTicketReady(ctx context.Context, db *sql.DB, id string, ready bool, actorUsername string, actorID string) (Ticket, error) {
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Ready == ready {
		return current, nil
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET ready = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, boolToInt(ready), id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	action := "marked_not_ready"
	if ready {
		action = "marked_ready"
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, action, map[string]any{
		"from_ready": current.Ready,
		"to_ready":   ready,
		"who":        actorUsername,
	}, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

func addTicketOpenHistoryEvent(ctx context.Context, db *sql.DB, current Ticket, from bool, to bool, actorUsername string, actorID string) error {
	if from == to {
		return nil
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, "ticket_open_changed", map[string]any{
		"from_open": from,
		"to_open":   to,
		"from":      fmt.Sprintf("%t", from),
		"to":        fmt.Sprintf("%t", to),
		"why":       map[bool]string{true: "open", false: "closed"}[to],
		"who":       actorUsername,
		"who_id":    actorID,
	}, actorID); err != nil {
		return err
	}
	return nil
}

func addTicketArchiveHistoryEvent(ctx context.Context, db *sql.DB, current Ticket, from bool, to bool, actorUsername string, actorID string) error {
	if from == to {
		return nil
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, "ticket_archived", map[string]any{
		"from_archived": from,
		"to_archived":   to,
		"from":          fmt.Sprintf("%t", from),
		"to":            fmt.Sprintf("%t", to),
		"why":           map[bool]string{true: "archive", false: "unarchive"}[to],
		"who":           actorUsername,
		"who_id":        actorID,
	}, actorID); err != nil {
		return err
	}
	return nil
}

func addTicketLifecycleHistoryEvent(ctx context.Context, db *sql.DB, current Ticket, nextStage, nextState, reason, actorUsername string, actorID string) error {
	fromStatus := RenderLifecycleStatus(current.Stage, current.State)
	toStatus := RenderLifecycleStatus(nextStage, nextState)
	if fromStatus == toStatus {
		return nil
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, "ticket_lifecycle_changed", map[string]any{
		"from_stage":  current.Stage,
		"from_state":  current.State,
		"from_status": fromStatus,
		"to_stage":    nextStage,
		"to_state":    nextState,
		"to_status":   toStatus,
		"reason":      reason,
		"who":         actorUsername,
		"who_id":      actorID,
	}, actorID); err != nil {
		return err
	}
	return nil
}

func SetTicketHealth(ctx context.Context, db *sql.DB, id string, score int) (Ticket, error) {
	if score < 0 || score > 4 {
		return Ticket{}, errors.New("health score must be between 0 and 4")
	}
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if !current.Open {
		return Ticket{}, ErrTicketClosed
	}
	if current.Archived {
		return Ticket{}, ErrTicketArchived
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET health_score = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, score, id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	return GetTicket(ctx, db, id)
}

func ListTicketsByProject(ctx context.Context, db *sql.DB, projectID int64) ([]Ticket, error) {
	return ListTickets(ctx, db, TicketListParams{ProjectID: projectID})
}

func ListTickets(ctx context.Context, db *sql.DB, params TicketListParams) ([]Ticket, error) {
	if params.ProjectID == 0 {
		return nil, errors.New("project is required")
	}

	query := `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE project_id = ?
	`
	args := []any{params.ProjectID}
	if ticketType := normalizeOptional(params.Type); ticketType != "" {
		query += ` AND type = ?`
		args = append(args, ticketType)
	}
	if stage := normalizeOptional(params.Stage); stage != "" {
		query += ` AND stage = ?`
		args = append(args, stage)
	}
	if state := normalizeOptional(params.State); state != "" {
		if !ValidState(state) {
			return nil, fmt.Errorf("invalid state %q", params.State)
		}
		query += ` AND state = ?`
		args = append(args, state)
	}
	if status := normalizeOptional(params.Status); status != "" {
		stage, state, err := parseRenderedLifecycle(status)
		if err != nil {
			return nil, err
		}
		query += ` AND stage = ? AND state = ?`
		args = append(args, stage, state)
	}
	if search := strings.TrimSpace(params.Search); search != "" {
		query += ` AND (LOWER(title) LIKE ? OR LOWER(description) LIKE ?)`
		needle := "%" + strings.ToLower(search) + "%"
		args = append(args, needle, needle)
	}
	if assignee := strings.TrimSpace(params.Assignee); assignee != "" {
		query += ` AND assignee = ?`
		args = append(args, assignee)
	}
	if !params.IncludeArchived {
		query += ` AND archived = 0 AND open = 1`
	}
	query += ` ORDER BY updated_at DESC, sort_order, ticket_id`
	if params.Limit < 0 {
		return nil, errors.New("limit must be zero or greater")
	}
	if params.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, params.Limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func SearchTickets(ctx context.Context, db *sql.DB, projectID int64, query string) ([]Ticket, error) {
	return ListTickets(ctx, db, TicketListParams{
		ProjectID: projectID,
		Search:    query,
	})
}

func GetTicketByProject(ctx context.Context, db *sql.DB, projectID int64, id string) (Ticket, error) {
	row := db.QueryRowContext(ctx, `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE project_id = ? AND ticket_id = ?
	`, projectID, id)
	ticket, err := scanTicket(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, err
	}
	return hydrateTicket(ctx, db, ticket)
}

func GetTicket(ctx context.Context, db *sql.DB, id string) (Ticket, error) {
	row := db.QueryRowContext(ctx, `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE ticket_id = ?
	`, id)

	ticket, err := scanTicket(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, err
	}
	return hydrateTicket(ctx, db, ticket)
}

func GetTicketByRef(ctx context.Context, db *sql.DB, raw string) (Ticket, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ticket{}, ErrTicketNotFound
	}
	// ticket_id IS the key now; look up directly (case-insensitive with upper)
	return GetTicket(ctx, db, strings.ToUpper(raw))
}

func ListTicketParents(ctx context.Context, db *sql.DB, id string) ([]Ticket, error) {
	// Load the full ancestor chain in one recursive CTE query instead of
	// making one GetTicket() call per ancestor level (O(depth) queries).
	rows, err := db.QueryContext(ctx, `
		WITH RECURSIVE ancestors(ticket_id, project_id, parent_id, clone_of, type, title,
		  description, acceptance_criteria, git_repository, git_branch,
		  workflow_id, workflow_stage_id, stage, state, status, priority, sort_order,
		  estimate_effort, estimate_complete, health_score, assignee, author,
		  ready, open, archived, created_by, created_at, updated_at) AS (
			SELECT ticket_id, project_id, parent_id, clone_of, type, title,
			  description, acceptance_criteria, git_repository, git_branch,
			  workflow_id, workflow_stage_id, stage, state, status, priority, sort_order,
			  estimate_effort, estimate_complete, health_score, assignee, COALESCE(author,''),
			  ready, open, archived, COALESCE(created_by,''), created_at, updated_at
			FROM tickets WHERE ticket_id = ?
			UNION ALL
			SELECT t.ticket_id, t.project_id, t.parent_id, t.clone_of, t.type, t.title,
			  t.description, t.acceptance_criteria, t.git_repository, t.git_branch,
			  t.workflow_id, t.workflow_stage_id, t.stage, t.state, t.status, t.priority, t.sort_order,
			  t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, COALESCE(t.author,''),
			  t.ready, t.open, t.archived, COALESCE(t.created_by,''), t.created_at, t.updated_at
			FROM tickets t
			JOIN ancestors a ON t.ticket_id = a.parent_id
		)
		SELECT ticket_id, project_id, parent_id, clone_of, type, title,
		  description, acceptance_criteria, git_repository, git_branch,
		  workflow_id, workflow_stage_id, stage, state, status, priority, sort_order,
		  estimate_effort, estimate_complete, health_score, assignee, author,
		  ready, open, archived, created_by, created_at, updated_at
		FROM ancestors
		WHERE ticket_id != ?
	`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parents []Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		parents = append(parents, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Batch-fetch comments for all ancestor tickets in one query.
	if len(parents) > 0 {
		ids := make([]string, len(parents))
		for i, p := range parents {
			ids[i] = p.ID
		}
		commentMap, err := batchFetchComments(ctx, db, ids)
		if err != nil {
			return nil, err
		}
		for i := range parents {
			parents[i].Comments = commentMap[parents[i].ID]
		}
	}
	return parents, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTicket(s scanner) (Ticket, error) {
	var ticket Ticket
	var parentID sql.NullString
	var cloneOf sql.NullString
	var workflowID sql.NullInt64
	var workflowStageID sql.NullInt64
	var storedStatus string
	var ready int
	var open int
	var archived int
	if err := s.Scan(
		&ticket.ID,
		&ticket.ProjectID,
		&parentID,
		&cloneOf,
		&ticket.Type,
		&ticket.Title,
		&ticket.Description,
		&ticket.AcceptanceCriteria,
		&ticket.GitRepository,
		&ticket.GitBranch,
		&workflowID,
		&workflowStageID,
		&ticket.Stage,
		&ticket.State,
		&storedStatus,
		&ticket.Priority,
		&ticket.Order,
		&ticket.EstimateEffort,
		&ticket.EstimateComplete,
		&ticket.HealthScore,
		&ticket.Assignee,
		&ticket.Author,
		&ready,
		&open,
		&archived,
		&ticket.CreatedBy,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	); err != nil {
		return Ticket{}, err
	}
	if parentID.Valid {
		ticket.ParentID = &parentID.String
	}
	if cloneOf.Valid {
		ticket.CloneOf = &cloneOf.String
	}
	if workflowID.Valid {
		ticket.WorkflowID = &workflowID.Int64
	}
	if workflowStageID.Valid {
		ticket.WorkflowStageID = &workflowStageID.Int64
	}
	ticket.Status = RenderLifecycleStatus(ticket.Stage, ticket.State)
	ticket.Ready = ready == 1
	ticket.Open = open == 1
	ticket.Archived = archived == 1
	return ticket, nil
}

func hydrateTicket(ctx context.Context, db *sql.DB, ticket Ticket) (Ticket, error) {
	comments, err := ListComments(ctx, db, ticket.ID)
	if err != nil {
		return Ticket{}, err
	}
	ticket.Comments = comments
	return ticket, nil
}

func batchFetchComments(ctx context.Context, db *sql.DB, ids []string) (map[string][]Comment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.item_id, c.user_id, u.username, c.comment, c.created_at
		FROM comments c
		JOIN users u ON u.user_id = c.user_id
		WHERE c.item_id IN (`+placeholders+`)
		ORDER BY c.created_at DESC, c.id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Comment, len(ids))
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ItemID, &c.UserID, &c.Author, &c.Comment, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Text = c.Comment
		result[c.ItemID] = append(result[c.ItemID], c)
	}
	return result, rows.Err()
}

func ticketHasChildren(ctx context.Context, db *sql.DB, id string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE parent_id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func syncRelatedLifecycle(ctx context.Context, db *sql.DB, actorID string, ids ...*string) error {
	seen := map[string]bool{}
	for _, rawID := range ids {
		if rawID == nil || seen[*rawID] {
			continue
		}
		seen[*rawID] = true
		if err := syncTicketAndAncestors(ctx, db, *rawID, actorID); err != nil {
			return err
		}
	}
	return nil
}

func syncAncestorLifecycle(ctx context.Context, db *sql.DB, parentID *string, actorID string) error {
	if parentID == nil {
		return nil
	}
	return syncTicketAndAncestors(ctx, db, *parentID, actorID)
}

func syncTicketAndAncestors(ctx context.Context, db *sql.DB, id string, actorID string) error {
	currentID := &id
	for currentID != nil {
		parentID, err := recalculateParentLifecycle(ctx, db, *currentID, actorID)
		if err != nil {
			return err
		}
		currentID = parentID
	}
	return nil
}

func recalculateParentLifecycle(ctx context.Context, db *sql.DB, id string, actorID string) (*string, error) {
	ticket, err := getStoredTicket(ctx, db, id)
	if err != nil {
		return nil, err
	}
	children, err := listStoredChildTickets(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return ticket.ParentID, nil
	}

	// Find minimum stage among children by workflow sort_order
	nextStage := children[0].Stage
	nextWorkflowStageID := children[0].WorkflowStageID
	minOrder := -1
	if children[0].WorkflowStageID != nil {
		if o, err := GetWorkflowStageOrder(ctx, db, *children[0].WorkflowStageID); err == nil {
			minOrder = o
		}
	}
	allSuccess := true
	anyFail := false
	anyActive := false
	for _, child := range children {
		childState := normalizeState(child.State)
		if childState != StateSuccess {
			allSuccess = false
		}
		if childState == StateActive {
			anyActive = true
		}
		if childState == StateFail {
			anyFail = true
		}
		if child.WorkflowStageID != nil {
			if o, err := GetWorkflowStageOrder(ctx, db, *child.WorkflowStageID); err == nil && (minOrder < 0 || o < minOrder) {
				minOrder = o
				nextStage = child.Stage
				nextWorkflowStageID = child.WorkflowStageID
			}
		}
	}
	nextState := StateIdle
	switch {
	case allSuccess:
		nextState = StateSuccess
	case anyActive:
		nextState = StateActive
	case anyFail:
		nextState = StateFail
	}
	ticketState := normalizeState(ticket.State)
	if nextStage == ticket.Stage && nextState == ticketState {
		return ticket.ParentID, nil
	}
	ticketStatus := RenderLifecycleStatus(ticket.Stage, ticketState)

	if _, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET workflow_stage_id = ?, stage = ?, state = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, nullableInt64(nextWorkflowStageID), nextStage, nextState, RenderLifecycleStatus(nextStage, nextState), id); err != nil {
		return nil, err
	}

	_ = AddHistoryEvent(ctx, db, ticket.ProjectID, ticket.ID, "ticket_parent_lifecycle_changed", map[string]any{
		"key":         ticket.ID,
		"from_stage":  ticket.Stage,
		"from_state":  ticketState,
		"from_status": ticketStatus,
		"to_stage":    nextStage,
		"to_state":    nextState,
		"to_status":   RenderLifecycleStatus(nextStage, nextState),
		"reason":      "child lifecycle aggregation",
	}, actorID)
	return ticket.ParentID, nil
}

func listStoredChildTickets(ctx context.Context, db *sql.DB, parentID string) ([]Ticket, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE parent_id = ?
		ORDER BY created_at, ticket_id
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func getStoredTicket(ctx context.Context, db *sql.DB, id string) (Ticket, error) {
	ticket, err := scanTicket(db.QueryRowContext(ctx, `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE ticket_id = ?
	`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, err
	}
	return ticket, nil
}

func normalizeTicketType(ticketType string) string {
	ticketType = strings.TrimSpace(strings.ToLower(ticketType))
	if ticketType == "" {
		return "task"
	}
	return ticketType
}

func parseRenderedLifecycle(status string) (string, string, error) {
	parts := strings.SplitN(normalizeOptional(status), "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid status %q", status)
	}
	state := normalizeState(parts[1])
	if !ValidLifecycle(parts[0], state) {
		return "", "", fmt.Errorf("invalid status %q", status)
	}
	return parts[0], state, nil
}

func validateEstimateComplete(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return errors.New("estimate_complete must be RFC3339 datetime")
	}
	return nil
}

func normalizeOptional(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

func validTicketType(ticketType string) bool {
	switch ticketType {
	case "task", "bug", "epic", "spike", "chore", "note", "question", "requirement", "decision":
		return true
	default:
		return false
	}
}

func validateTicketParenting(parentType, childType string) error {
	parentType = normalizeTicketType(parentType)
	childType = normalizeTicketType(childType)
	switch parentType {
	case "epic":
		if validTicketType(childType) {
			return nil
		}
	case "task":
		switch childType {
		case "task", "bug", "spike", "chore":
			return nil
		}
	}
	return fmt.Errorf("%s cannot parent %s", parentType, childType)
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}


func validateTicketAssignmentChange(currentAssignee, nextAssignee, actorUsername, actorRole string) error {
	currentAssignee = strings.TrimSpace(currentAssignee)
	nextAssignee = strings.TrimSpace(nextAssignee)
	actorUsername = strings.TrimSpace(actorUsername)
	actorRole = strings.TrimSpace(actorRole)

	if currentAssignee == nextAssignee {
		return nil
	}
	if actorRole == "admin" {
		return nil
	}
	if actorUsername == "" {
		return errors.New("username is required for assignment changes")
	}
	if nextAssignee == actorUsername {
		if currentAssignee != "" && currentAssignee != actorUsername {
			return fmt.Errorf("ticket is already assigned to %s", currentAssignee)
		}
		return nil
	}
	if nextAssignee == "" {
		if currentAssignee != actorUsername {
			if currentAssignee == "" {
				return errors.New("ticket is not assigned to you")
			}
			return fmt.Errorf("ticket is assigned to %s", currentAssignee)
		}
		return nil
	}
	return ErrAdminRequired
}

func RequestTicket(ctx context.Context, db *sql.DB, params TicketRequestParams) (Ticket, string, error) {
	username := strings.TrimSpace(params.Username)
	if username == "" {
		return Ticket{}, "", errors.New("username is required")
	}

	if ticket, ok, err := findAssignedTicketForUser(ctx, db, params.ProjectID, username, StateActive); err != nil {
		return Ticket{}, "", err
	} else if ok {
		return ticket, "ASSIGNED", nil
	}

	if params.TicketID != nil || strings.TrimSpace(params.TicketRef) != "" {
		ticket, err := resolveRequestedTicket(ctx, db, params)
		if err != nil {
			return Ticket{}, "", err
		}
		if strings.TrimSpace(ticket.Assignee) == username {
			return ticket, "ASSIGNED", nil
		}
		ok, err := ticketClaimable(ctx, db, ticket, params.ProjectID)
		if err != nil {
			return Ticket{}, "", err
		}
		if !ok {
			return Ticket{}, "REJECTED", nil
		}
		if params.DryRun {
			return withClaimPreview(ticket, username), "AVAILABLE", nil
		}
		assigned, err := UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
			Title:              ticket.Title,
			Description:        ticket.Description,
			AcceptanceCriteria: ticket.AcceptanceCriteria,
			ParentID:           ticket.ParentID,
			Assignee:           username,
			State:              StateActive,
			Priority:           ticket.Priority,
			Order:              ticket.Order,
			UpdatedBy:          params.UserID,
			ActorUsername:      username,
			ActorRole:          "admin",
		})
		if err != nil {
			return Ticket{}, "", err
		}
		return assigned, "ASSIGNED", nil
	}

	ticket, ok, err := findClaimCandidate(ctx, db, params.ProjectID)
	if err != nil {
		return Ticket{}, "", err
	}
	if !ok {
		return Ticket{}, "NO-WORK", nil
	}
	if params.DryRun {
		return withClaimPreview(ticket, username), "AVAILABLE", nil
	}
	assigned, err := UpdateTicket(ctx, db, ticket.ID, TicketUpdateParams{
		Title:              ticket.Title,
		Description:        ticket.Description,
		AcceptanceCriteria: ticket.AcceptanceCriteria,
		ParentID:           ticket.ParentID,
		Assignee:           username,
		State:              StateActive,
		Priority:           ticket.Priority,
		Order:              ticket.Order,
		UpdatedBy:          params.UserID,
		ActorUsername:      username,
		ActorRole:          "admin",
	})
	if err != nil {
		return Ticket{}, "", err
	}
	return assigned, "ASSIGNED", nil
}

func resolveRequestedTicket(ctx context.Context, db *sql.DB, params TicketRequestParams) (Ticket, error) {
	if params.TicketID != nil {
		return GetTicket(ctx, db, *params.TicketID)
	}
	ticket, err := GetTicketByRef(ctx, db, params.TicketRef)
	if err != nil {
		return Ticket{}, err
	}
	return ticket, nil
}

func withClaimPreview(ticket Ticket, username string) Ticket {
	ticket.Assignee = username
	ticket.State = StateActive
	ticket.Status = RenderLifecycleStatus(ticket.Stage, ticket.State)
	return ticket
}

func ticketClaimable(ctx context.Context, db *sql.DB, ticket Ticket, projectID int64) (bool, error) {
	if projectID != 0 && ticket.ProjectID != projectID {
		return false, nil
	}
	project, err := GetProjectByID(ctx, db, ticket.ProjectID)
	if err != nil {
		return false, err
	}
	if project.Status != "open" || !ticket.Open || ticket.Archived {
		return false, nil
	}
	if strings.TrimSpace(ticket.Assignee) != "" {
		return false, nil
	}
	if ticket.Stage != StageDevelop || ticket.State != StateIdle {
		return false, nil
	}
	hasChildren, err := ticketHasChildren(ctx, db, ticket.ID)
	if err != nil {
		return false, err
	}
	return !hasChildren, nil
}

func findAssignedTicketForUser(ctx context.Context, db *sql.DB, projectID int64, username, state string) (Ticket, bool, error) {
	query := `
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_id, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), ready, open, archived, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE assignee = ? AND open = 1 AND archived = 0 AND state = ?
	`
	args := []any{username, state}
	if projectID != 0 {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at, ticket_id LIMIT 1`
	ticket, err := scanTicket(db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, false, nil
		}
		return Ticket{}, false, err
	}
	return ticket, true, nil
}

func CurrentAssignedTicketForUser(ctx context.Context, db *sql.DB, projectID int64, username string) (Ticket, bool, error) {
	return findAssignedTicketForUser(ctx, db, projectID, strings.TrimSpace(username), StateActive)
}

func findClaimCandidate(ctx context.Context, db *sql.DB, projectID int64) (Ticket, bool, error) {
	if projectID == 0 {
		return Ticket{}, false, errors.New("project is required")
	}
	ticket, err := scanTicket(db.QueryRowContext(ctx, `
		SELECT t.ticket_id, t.project_id, t.parent_id, t.clone_of, t.type, t.title, t.description, t.acceptance_criteria, t.git_repository, t.git_branch, t.workflow_id, t.workflow_stage_id, t.stage, t.state, t.status, t.priority, t.sort_order, t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, COALESCE(t.author, ''), t.ready, t.open, t.archived, COALESCE(t.created_by, ''), t.created_at, t.updated_at
		FROM tickets t
		JOIN projects p ON p.project_id = t.project_id
		WHERE t.project_id = ? AND p.status = 'open' AND t.open = 1 AND t.archived = 0 AND t.ready = 1 AND t.state = ? AND TRIM(COALESCE(t.assignee, '')) = ''
		  AND NOT EXISTS (SELECT 1 FROM tickets c WHERE c.parent_id = t.ticket_id)
		ORDER BY t.priority DESC, t.created_at, t.ticket_id
		LIMIT 1
	`, projectID, StateIdle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, false, nil
		}
		return Ticket{}, false, err
	}
	return ticket, true, nil
}

// ExplainNoWork returns human-readable reasons why no ticket was available
// for automatic assignment in the given project.
func ExplainNoWork(ctx context.Context, db *sql.DB, projectID int64, username string) ([]string, error) {
	var reasons []string

	// Count total tickets in project.
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE project_id = ?`, projectID).Scan(&total); err != nil {
		return nil, err
	}
	reasons = append(reasons, fmt.Sprintf("total tickets in project: %d", total))

	// Count by state.
	rows, err := db.QueryContext(ctx, `
		SELECT state, COUNT(*) FROM tickets
		WHERE project_id = ? AND open = 1 AND archived = 0
		GROUP BY state
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, err
		}
		reasons = append(reasons, fmt.Sprintf("  open state=%s: %d", state, count))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count idle unassigned.
	var idleUnassigned int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND open = 1 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) = ''
	`, projectID).Scan(&idleUnassigned); err != nil {
		return nil, err
	}
	reasons = append(reasons, fmt.Sprintf("idle + unassigned: %d", idleUnassigned))

	// Count not ready.
	var notReady int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND open = 1 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) = '' AND ready = 0
	`, projectID).Scan(&notReady); err != nil {
		return nil, err
	}
	if notReady > 0 {
		reasons = append(reasons, fmt.Sprintf("not ready (blocked): %d — use 'tk ready <id>' to make eligible", notReady))
	}

	// Count with children (non-leaf).
	var withChildren int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets t
		WHERE t.project_id = ? AND t.open = 1 AND t.archived = 0 AND t.ready = 1
		AND t.state = 'idle' AND TRIM(COALESCE(t.assignee, '')) = ''
		AND EXISTS (SELECT 1 FROM tickets c WHERE c.parent_id = t.ticket_id)
	`, projectID).Scan(&withChildren); err != nil {
		return nil, err
	}
	if withChildren > 0 {
		reasons = append(reasons, fmt.Sprintf("ready but has children (not leaf): %d", withChildren))
	}

	// Count assigned to someone else.
	var assignedOther int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND open = 1 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) != '' AND assignee != ?
	`, projectID, username).Scan(&assignedOther); err != nil {
		return nil, err
	}
	if assignedOther > 0 {
		reasons = append(reasons, fmt.Sprintf("idle but assigned to others: %d", assignedOther))
	}

	// Count closed.
	var closed int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE project_id = ? AND open = 0`, projectID).Scan(&closed); err != nil {
		return nil, err
	}
	if closed > 0 {
		reasons = append(reasons, fmt.Sprintf("closed: %d", closed))
	}

	return reasons, nil
}

func CloneTicket(ctx context.Context, db *sql.DB, id string, author string, createdBy string) (Ticket, error) {
	original, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	cloned, err := cloneTicketRecursive(ctx, db, original, nil, author, createdBy)
	if err != nil {
		return Ticket{}, err
	}
	return cloned, nil
}

func cloneTicketRecursive(ctx context.Context, db *sql.DB, original Ticket, parentID *string, author string, createdBy string) (Ticket, error) {
	cloned, err := CreateTicket(ctx, db, TicketCreateParams{
		ProjectID:          original.ProjectID,
		ParentID:           parentID,
		CloneOf:            &original.ID,
		Type:               original.Type,
		Title:              original.Title,
		Description:        original.Description,
		AcceptanceCriteria: original.AcceptanceCriteria,
		Priority:           original.Priority,
		Order:              original.Order,
		EstimateEffort:     original.EstimateEffort,
		EstimateComplete:   original.EstimateComplete,
		Assignee:           "",
		Author:             author,
		State:              StateIdle,
		CreatedBy:          createdBy,
	})
	if err != nil {
		return Ticket{}, err
	}
	if original.Type != "epic" {
		return cloned, nil
	}
	children, err := ListTickets(ctx, db, TicketListParams{ProjectID: original.ProjectID, IncludeArchived: true})
	if err != nil {
		return Ticket{}, err
	}
	for _, child := range children {
		if child.ParentID != nil && *child.ParentID == original.ID {
			if _, err := cloneTicketRecursive(ctx, db, child, &cloned.ID, author, createdBy); err != nil {
				return Ticket{}, err
			}
		}
	}
	return cloned, nil
}

func DeleteTicket(ctx context.Context, db *sql.DB, id string) error {
	ticket, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	parentID := ticket.ParentID

	children, err := ListTickets(ctx, db, TicketListParams{ProjectID: ticket.ProjectID, IncludeArchived: true})
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.ParentID != nil && *child.ParentID == id {
			return ErrTicketHasChildren
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE tickets SET clone_of = NULL WHERE clone_of = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dependencies WHERE ticket_id = ? OR depends_on = ?`, id, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE item_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM history_events WHERE ticket_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_history WHERE ticket_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM tickets WHERE ticket_id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTicketNotFound
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return syncAncestorLifecycle(ctx, db, parentID, "")
}

// TicketContext holds a ticket and all surrounding context needed to work on it.
type TicketContext struct {
	Project  *Project            `json:"project,omitempty"`
	Parents  []Ticket            `json:"parents,omitempty"`
	Workflow *WorkflowWithStages `json:"workflow,omitempty"`
	Role     *Role               `json:"role,omitempty"`
}

// ResolveWorkflowID returns the effective workflow ID for a ticket by walking:
// ticket.WorkflowID → parent chain → project.WorkflowID.
func ResolveWorkflowID(ctx context.Context, db *sql.DB, ticket Ticket) *int64 {
	if ticket.WorkflowID != nil {
		return ticket.WorkflowID
	}
	// Walk parent chain
	parentID := ticket.ParentID
	for parentID != nil {
		parent, err := GetTicket(ctx, db, *parentID)
		if err != nil {
			break
		}
		if parent.WorkflowID != nil {
			return parent.WorkflowID
		}
		parentID = parent.ParentID
	}
	// Fall back to project
	if project, err := GetProjectByID(ctx, db, ticket.ProjectID); err == nil {
		return project.WorkflowID
	}
	return nil
}

// EnrichTicketContext gathers the project, parent chain, workflow, and
// current-stage role for a ticket. Missing data is silently skipped.
func EnrichTicketContext(ctx context.Context, db *sql.DB, ticket Ticket) TicketContext {
	var result TicketContext
	if project, err := GetProjectByID(ctx, db, ticket.ProjectID); err == nil {
		result.Project = &project
	}
	if wfID := ResolveWorkflowID(ctx, db, ticket); wfID != nil {
		if wf, err := GetWorkflow(ctx, db, *wfID); err == nil {
			result.Workflow = &wf
			if ticket.WorkflowStageID != nil {
				for _, stage := range wf.Stages {
					if stage.ID == *ticket.WorkflowStageID && stage.RoleID != nil {
						if role, err := GetRoleByID(ctx, db, *stage.RoleID); err == nil {
							result.Role = &role
						}
						break
					}
				}
			}
		}
	}
	if parents, err := ListTicketParents(ctx, db, ticket.ID); err == nil && len(parents) > 0 {
		result.Parents = parents
	}
	return result
}

// SetTicketWorkflow sets an explicit workflow on a ticket, resetting the
// workflow stage to the first stage of the new workflow.
func SetTicketWorkflow(ctx context.Context, db *sql.DB, ticketID string, workflowID int64) (Ticket, error) {
	wf, err := GetWorkflow(ctx, db, workflowID)
	if err != nil {
		return Ticket{}, fmt.Errorf("workflow %d not found", workflowID)
	}
	var wsID *int64
	stage := StageDesign
	if len(wf.Stages) > 0 {
		wsID = &wf.Stages[0].ID
		stage = wf.Stages[0].StageName
	}
	_, err = db.ExecContext(ctx, `
		UPDATE tickets
		SET workflow_id = ?, workflow_stage_id = ?, stage = ?, state = 'idle', status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, workflowID, wsID, stage, RenderLifecycleStatus(stage, StateIdle), ticketID)
	if err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, ticketID)
}

// UnsetTicketWorkflow clears the explicit workflow on a ticket, falling back
// to the inherited workflow and resetting the stage accordingly.
func UnsetTicketWorkflow(ctx context.Context, db *sql.DB, ticketID string) (Ticket, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	// Clear the ticket's own workflow_id
	ticket.WorkflowID = nil
	// Resolve inherited workflow
	wfID := ResolveWorkflowID(ctx, db, ticket)
	var wsID *int64
	stage := StageDesign
	if wfID != nil {
		if wf, err := GetWorkflow(ctx, db, *wfID); err == nil && len(wf.Stages) > 0 {
			wsID = &wf.Stages[0].ID
			stage = wf.Stages[0].StageName
		}
	}
	_, err = db.ExecContext(ctx, `
		UPDATE tickets
		SET workflow_id = NULL, workflow_stage_id = ?, stage = ?, state = 'idle', status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, wsID, stage, RenderLifecycleStatus(stage, StateIdle), ticketID)
	if err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, ticketID)
}
