package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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
	ID                 int64     `json:"ticket_id"`
	Key                string    `json:"key"`
	ProjectID          int64     `json:"project_id"`
	ParentID           *int64    `json:"parent_id,omitempty"`
	CloneOf            *int64    `json:"clone_of,omitempty"`
	Type               string    `json:"type"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	GitRepository      string    `json:"git_repository"`
	GitBranch          string    `json:"git_branch"`
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
	Comments           []Comment `json:"comments,omitempty"`
	Open               bool      `json:"open"`
	Archived           bool      `json:"archived"`
	CreatedBy          int64     `json:"created_by"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

type TicketCreateParams struct {
	ProjectID          int64
	ParentID           *int64
	CloneOf            *int64
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
	State              string
	CreatedBy          int64
}

type TicketUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	ParentID           *int64
	Assignee           string
	State              string
	Priority           int
	Order              int
	EstimateEffort     int
	EstimateComplete   string
	UpdatedBy          int64
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
	TicketID  *int64
	TicketRef string
	Username  string
	UserID    int64
	DryRun    bool
}

func CreateTicket(db *sql.DB, params TicketCreateParams) (Ticket, error) {
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
		parent, err := GetTicket(db, *params.ParentID)
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

	tx, err := db.Begin()
	if err != nil {
		return Ticket{}, err
	}
	defer tx.Rollback()
	var projectPrefix string
	var nextSequence int64
	var workflowID sql.NullInt64
	if err := tx.QueryRow(`SELECT prefix, ticket_sequence + 1, workflow_id FROM projects WHERE project_id = ?`, params.ProjectID).Scan(&projectPrefix, &nextSequence, &workflowID); err != nil {
		return Ticket{}, err
	}
	// Resolve initial workflow stage (first stage by sort_order)
	var workflowStageID *int64
	stage := StageDesign // fallback
	if workflowID.Valid {
		var wsID int64
		var stageName string
		err := tx.QueryRow(`SELECT workflow_stage_id, stage_name FROM workflow_stages WHERE workflow_id = ? ORDER BY sort_order LIMIT 1`, workflowID.Int64).Scan(&wsID, &stageName)
		if err == nil {
			workflowStageID = &wsID
			stage = stageName
		}
	}
	key, err := generateTicketKey(projectPrefix, params.Type, nextSequence)
	if err != nil {
		return Ticket{}, err
	}
	result, err := tx.Exec(`
		INSERT INTO tickets (key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key, params.ProjectID, nullableInt64(params.ParentID), nullableInt64(params.CloneOf), params.Type, params.Title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.GitBranch), nullableInt64(workflowStageID), stage, state, RenderLifecycleStatus(stage, state), priority, order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), 0, strings.TrimSpace(params.Assignee), params.CreatedBy)
	if err != nil {
		return Ticket{}, err
	}
	if _, err := tx.Exec(`UPDATE projects SET ticket_sequence = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, nextSequence, params.ProjectID); err != nil {
		return Ticket{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(); err != nil {
		return Ticket{}, err
	}
	ticket, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if err := AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_created", map[string]any{
		"key":               ticket.Key,
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
	if err := syncAncestorLifecycle(db, params.ParentID, params.CreatedBy); err != nil {
		return Ticket{}, err
	}
	return GetTicket(db, id)
}

func UpdateTicket(db *sql.DB, id int64, params TicketUpdateParams) (Ticket, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	if err := validateEstimateComplete(params.EstimateComplete); err != nil {
		return Ticket{}, err
	}
	current, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	hasChildren, err := ticketHasChildren(db, current.ID)
	if err != nil {
		return Ticket{}, err
	}
	if params.ParentID != nil {
		parent, err := GetTicket(db, *params.ParentID)
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
	if !current.Open {
		return Ticket{}, ErrTicketClosed
	}
	if current.Archived {
		return Ticket{}, ErrTicketArchived
	}
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
		target, err := GetUserByUsername(db, assignee)
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

	explicitState := normalizeOptional(params.State) != ""
	if hasChildren && explicitState {
		return Ticket{}, errors.New("ticket has children; state is derived from descendants")
	}
	state := current.State
	stage := current.Stage
	workflowStageID := current.WorkflowStageID
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
			nextID, _, err := getNextWorkflowStage(db, *current.WorkflowStageID)
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
			nextStageID, nextStageName, err := getNextWorkflowStage(db, *workflowStageID)
			if err == nil && nextStageID != nil {
				workflowStageID = nextStageID
				stage = nextStageName
				state = StateIdle
			}
			// If no next stage (final stage), stay at current stage with success state
		}
	}

	result, err := db.Exec(`
		UPDATE tickets
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, git_branch = ?, parent_id = ?, assignee = ?, workflow_stage_id = ?, stage = ?, state = ?, status = ?, priority = ?, sort_order = ?, estimate_effort = ?, estimate_complete = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), nextGitRepository, nextGitBranch, nullableInt64(params.ParentID), assignee, nullableInt64(workflowStageID), stage, state, RenderLifecycleStatus(stage, state), params.Priority, params.Order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), id)
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
	ticket, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Stage != ticket.Stage || current.State != ticket.State {
		if err := addTicketLifecycleHistoryEvent(db, current, ticket.Stage, ticket.State, "manual update", params.ActorUsername, params.UpdatedBy); err != nil {
			return Ticket{}, err
		}
	}
	if err := AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_updated", map[string]any{
		"key":                 ticket.Key,
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
	if err := syncRelatedLifecycle(db, params.UpdatedBy, current.ParentID, params.ParentID, &current.ID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(db, id)
}

func SetTicketOpen(db *sql.DB, id int64, open bool, actorUsername string, actorID int64) (Ticket, error) {
	current, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Open == open {
		return current, nil
	}
	result, err := db.Exec(`
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
	if err := addTicketOpenHistoryEvent(db, current, current.Open, open, actorUsername, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(db, id)
}

func SetTicketArchived(db *sql.DB, id int64, archived bool, actorUsername string, actorID int64) (Ticket, error) {
	current, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Archived == archived {
		return current, nil
	}
	result, err := db.Exec(`
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
	if err := addTicketArchiveHistoryEvent(db, current, current.Archived, archived, actorUsername, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(db, id)
}

func addTicketOpenHistoryEvent(db *sql.DB, current Ticket, from bool, to bool, actorUsername string, actorID int64) error {
	if from == to {
		return nil
	}
	if err := AddHistoryEvent(db, current.ProjectID, current.ID, "ticket_open_changed", map[string]any{
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

func addTicketArchiveHistoryEvent(db *sql.DB, current Ticket, from bool, to bool, actorUsername string, actorID int64) error {
	if from == to {
		return nil
	}
	if err := AddHistoryEvent(db, current.ProjectID, current.ID, "ticket_archived", map[string]any{
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

func addTicketLifecycleHistoryEvent(db *sql.DB, current Ticket, nextStage, nextState, reason, actorUsername string, actorID int64) error {
	fromStatus := RenderLifecycleStatus(current.Stage, current.State)
	toStatus := RenderLifecycleStatus(nextStage, nextState)
	if fromStatus == toStatus {
		return nil
	}
	if err := AddHistoryEvent(db, current.ProjectID, current.ID, "ticket_lifecycle_changed", map[string]any{
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

func SetTicketHealth(db *sql.DB, id int64, score int) (Ticket, error) {
	if score < 0 || score > 4 {
		return Ticket{}, errors.New("health score must be between 0 and 4")
	}
	current, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if !current.Open {
		return Ticket{}, ErrTicketClosed
	}
	if current.Archived {
		return Ticket{}, ErrTicketArchived
	}
	result, err := db.Exec(`
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
	return GetTicket(db, id)
}

func ListTicketsByProject(db *sql.DB, projectID int64) ([]Ticket, error) {
	return ListTickets(db, TicketListParams{ProjectID: projectID})
}

func ListTickets(db *sql.DB, params TicketListParams) ([]Ticket, error) {
	if params.ProjectID == 0 {
		return nil, errors.New("project is required")
	}

	query := `
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
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
		query += ` AND archived = 0`
	}
	query += ` ORDER BY sort_order, created_at, ticket_id`
	if params.Limit < 0 {
		return nil, errors.New("limit must be zero or greater")
	}
	if params.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, params.Limit)
	}

	rows, err := db.Query(query, args...)
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

func SearchTickets(db *sql.DB, projectID int64, query string) ([]Ticket, error) {
	return ListTickets(db, TicketListParams{
		ProjectID: projectID,
		Search:    query,
	})
}

func GetTicketByProject(db *sql.DB, projectID, id int64) (Ticket, error) {
	row := db.QueryRow(`
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
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
	return hydrateTicket(db, ticket)
}

func GetTicket(db *sql.DB, id int64) (Ticket, error) {
	row := db.QueryRow(`
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
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
	return hydrateTicket(db, ticket)
}

func GetTicketByRef(db *sql.DB, raw string) (Ticket, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ticket{}, ErrTicketNotFound
	}
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return GetTicket(db, id)
	}
	row := db.QueryRow(`
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tickets
		WHERE key = ?
	`, strings.ToUpper(raw))
	ticket, err := scanTicket(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, ErrTicketNotFound
		}
		return Ticket{}, err
	}
	return hydrateTicket(db, ticket)
}

func ListTicketParents(db *sql.DB, id int64) ([]Ticket, error) {
	current, err := GetTicket(db, id)
	if err != nil {
		return nil, err
	}
	parents := make([]Ticket, 0)
	parentID := current.ParentID
	for parentID != nil {
		parent, err := GetTicket(db, *parentID)
		if err != nil {
			return nil, err
		}
		parents = append(parents, parent)
		parentID = parent.ParentID
	}
	return parents, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTicket(s scanner) (Ticket, error) {
	var ticket Ticket
	var parentID sql.NullInt64
	var cloneOf sql.NullInt64
	var workflowStageID sql.NullInt64
	var storedStatus string
	var open int
	var archived int
	if err := s.Scan(
		&ticket.ID,
		&ticket.Key,
		&ticket.ProjectID,
		&parentID,
		&cloneOf,
		&ticket.Type,
		&ticket.Title,
		&ticket.Description,
		&ticket.AcceptanceCriteria,
		&ticket.GitRepository,
		&ticket.GitBranch,
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
		&open,
		&archived,
		&ticket.CreatedBy,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	); err != nil {
		return Ticket{}, err
	}
	if parentID.Valid {
		ticket.ParentID = &parentID.Int64
	}
	if cloneOf.Valid {
		ticket.CloneOf = &cloneOf.Int64
	}
	if workflowStageID.Valid {
		ticket.WorkflowStageID = &workflowStageID.Int64
	}
	ticket.Status = RenderLifecycleStatus(ticket.Stage, ticket.State)
	ticket.Open = open == 1
	ticket.Archived = archived == 1
	return ticket, nil
}

func hydrateTicket(db *sql.DB, ticket Ticket) (Ticket, error) {
	comments, err := ListComments(db, ticket.ID)
	if err != nil {
		return Ticket{}, err
	}
	ticket.Comments = comments
	return ticket, nil
}

func ticketHasChildren(db *sql.DB, id int64) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE parent_id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func syncRelatedLifecycle(db *sql.DB, actorID int64, ids ...*int64) error {
	seen := map[int64]bool{}
	for _, rawID := range ids {
		if rawID == nil || seen[*rawID] {
			continue
		}
		seen[*rawID] = true
		if err := syncTicketAndAncestors(db, *rawID, actorID); err != nil {
			return err
		}
	}
	return nil
}

func syncAncestorLifecycle(db *sql.DB, parentID *int64, actorID int64) error {
	if parentID == nil {
		return nil
	}
	return syncTicketAndAncestors(db, *parentID, actorID)
}

func syncTicketAndAncestors(db *sql.DB, id int64, actorID int64) error {
	currentID := &id
	for currentID != nil {
		parentID, err := recalculateParentLifecycle(db, *currentID, actorID)
		if err != nil {
			return err
		}
		currentID = parentID
	}
	return nil
}

func recalculateParentLifecycle(db *sql.DB, id int64, actorID int64) (*int64, error) {
	ticket, err := getStoredTicket(db, id)
	if err != nil {
		return nil, err
	}
	children, err := listStoredChildTickets(db, id)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return ticket.ParentID, nil
	}

	// Find minimum stage among children (by workflow sort_order or fallback to text comparison)
	nextStage := children[0].Stage
	nextWorkflowStageID := children[0].WorkflowStageID
	minOrder := -1
	if children[0].WorkflowStageID != nil {
		if o, err := GetWorkflowStageOrder(db, *children[0].WorkflowStageID); err == nil {
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
		if child.WorkflowStageID != nil && minOrder >= 0 {
			if o, err := GetWorkflowStageOrder(db, *child.WorkflowStageID); err == nil && o < minOrder {
				minOrder = o
				nextStage = child.Stage
				nextWorkflowStageID = child.WorkflowStageID
			}
		} else if CompareStageOrder(child.Stage, nextStage) < 0 {
			nextStage = child.Stage
			nextWorkflowStageID = child.WorkflowStageID
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

	if _, err := db.Exec(`
		UPDATE tickets
		SET workflow_stage_id = ?, stage = ?, state = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, nullableInt64(nextWorkflowStageID), nextStage, nextState, RenderLifecycleStatus(nextStage, nextState), id); err != nil {
		return nil, err
	}

	_ = AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_parent_lifecycle_changed", map[string]any{
		"key":         ticket.Key,
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

func listStoredChildTickets(db *sql.DB, parentID int64) ([]Ticket, error) {
	rows, err := db.Query(`
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
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

func getStoredTicket(db *sql.DB, id int64) (Ticket, error) {
	ticket, err := scanTicket(db.QueryRow(`
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
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
	case "task", "bug", "epic", "spike", "chore":
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

func RequestTicket(db *sql.DB, params TicketRequestParams) (Ticket, string, error) {
	username := strings.TrimSpace(params.Username)
	if username == "" {
		return Ticket{}, "", errors.New("username is required")
	}

	if ticket, ok, err := findAssignedTicketForUser(db, params.ProjectID, username, StateActive); err != nil {
		return Ticket{}, "", err
	} else if ok {
		return ticket, "ASSIGNED", nil
	}

	if params.TicketID != nil || strings.TrimSpace(params.TicketRef) != "" {
		ticket, err := resolveRequestedTicket(db, params)
		if err != nil {
			return Ticket{}, "", err
		}
		if strings.TrimSpace(ticket.Assignee) == username {
			return ticket, "ASSIGNED", nil
		}
		ok, err := ticketClaimable(db, ticket, params.ProjectID)
		if err != nil {
			return Ticket{}, "", err
		}
		if !ok {
			return Ticket{}, "REJECTED", nil
		}
		if params.DryRun {
			return withClaimPreview(ticket, username), "AVAILABLE", nil
		}
		assigned, err := UpdateTicket(db, ticket.ID, TicketUpdateParams{
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

	ticket, ok, err := findClaimCandidate(db, params.ProjectID)
	if err != nil {
		return Ticket{}, "", err
	}
	if !ok {
		return Ticket{}, "NO-WORK", nil
	}
	if params.DryRun {
		return withClaimPreview(ticket, username), "AVAILABLE", nil
	}
	assigned, err := UpdateTicket(db, ticket.ID, TicketUpdateParams{
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

func resolveRequestedTicket(db *sql.DB, params TicketRequestParams) (Ticket, error) {
	if params.TicketID != nil {
		return GetTicket(db, *params.TicketID)
	}
	ticket, err := GetTicketByRef(db, params.TicketRef)
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

func ticketClaimable(db *sql.DB, ticket Ticket, projectID int64) (bool, error) {
	if projectID != 0 && ticket.ProjectID != projectID {
		return false, nil
	}
	project, err := GetProjectByID(db, ticket.ProjectID)
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
	hasChildren, err := ticketHasChildren(db, ticket.ID)
	if err != nil {
		return false, err
	}
	return !hasChildren, nil
}

func findAssignedTicketForUser(db *sql.DB, projectID int64, username, state string) (Ticket, bool, error) {
	query := `
		SELECT ticket_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, workflow_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, open, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tickets
		WHERE assignee = ? AND open = 1 AND archived = 0 AND state = ?
	`
	args := []any{username, state}
	if projectID != 0 {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at, ticket_id LIMIT 1`
	ticket, err := scanTicket(db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, false, nil
		}
		return Ticket{}, false, err
	}
	return ticket, true, nil
}

func CurrentAssignedTicketForUser(db *sql.DB, projectID int64, username string) (Ticket, bool, error) {
	return findAssignedTicketForUser(db, projectID, strings.TrimSpace(username), StateActive)
}

func findClaimCandidate(db *sql.DB, projectID int64) (Ticket, bool, error) {
	if projectID == 0 {
		return Ticket{}, false, errors.New("project is required")
	}
	ticket, err := scanTicket(db.QueryRow(`
		SELECT t.ticket_id, t.key, t.project_id, t.parent_id, t.clone_of, t.type, t.title, t.description, t.acceptance_criteria, t.git_repository, t.git_branch, t.workflow_stage_id, t.stage, t.state, t.status, t.priority, t.sort_order, t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, t.open, t.archived, COALESCE(t.created_by, 0), t.created_at, t.updated_at
		FROM tickets t
		JOIN projects p ON p.project_id = t.project_id
		WHERE t.project_id = ? AND p.status = 'open' AND t.open = 1 AND t.archived = 0 AND t.state = ? AND TRIM(COALESCE(t.assignee, '')) = ''
		  AND NOT EXISTS (SELECT 1 FROM tickets c WHERE c.parent_id = t.ticket_id)
		ORDER BY t.priority DESC, t.created_at, t.key, t.ticket_id
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

func CloneTicket(db *sql.DB, id, createdBy int64) (Ticket, error) {
	original, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	cloned, err := cloneTicketRecursive(db, original, nil, createdBy)
	if err != nil {
		return Ticket{}, err
	}
	return cloned, nil
}

func cloneTicketRecursive(db *sql.DB, original Ticket, parentID *int64, createdBy int64) (Ticket, error) {
	cloned, err := CreateTicket(db, TicketCreateParams{
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
		State:              StateIdle,
		CreatedBy:          createdBy,
	})
	if err != nil {
		return Ticket{}, err
	}
	if original.Type != "epic" {
		return cloned, nil
	}
	children, err := ListTickets(db, TicketListParams{ProjectID: original.ProjectID, IncludeArchived: true})
	if err != nil {
		return Ticket{}, err
	}
	for _, child := range children {
		if child.ParentID != nil && *child.ParentID == original.ID {
			if _, err := cloneTicketRecursive(db, child, &cloned.ID, createdBy); err != nil {
				return Ticket{}, err
			}
		}
	}
	return cloned, nil
}

func DeleteTicket(db *sql.DB, id int64) error {
	ticket, err := GetTicket(db, id)
	if err != nil {
		return err
	}
	parentID := ticket.ParentID

	children, err := ListTickets(db, TicketListParams{ProjectID: ticket.ProjectID, IncludeArchived: true})
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.ParentID != nil && *child.ParentID == id {
			return ErrTicketHasChildren
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE tickets SET clone_of = NULL WHERE clone_of = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM dependencies WHERE ticket_id = ? OR depends_on = ?`, id, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM comments WHERE item_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM history_events WHERE ticket_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM ticket_history WHERE ticket_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM tickets WHERE ticket_id = ?`, id)
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
	return syncAncestorLifecycle(db, parentID, 0)
}
