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
	Priority           int
	Order              int
	EstimateEffort     int
	EstimateComplete   string
	Assignee           string
	Stage              string
	State              string
	CreatedBy          int64
}

type TicketUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	ParentID           *int64
	Assignee           string
	Stage              string
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
	ProjectID int64
	Type      string
	Stage     string
	State     string
	Status    string
	Search    string
	Assignee  string
	Limit     int
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
	stage, state, err := resolveLifecycleForCreate(params.Stage, params.State, params.Assignee)
	if err != nil {
		return Ticket{}, err
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
	if err := tx.QueryRow(`SELECT prefix, ticket_sequence + 1 FROM projects WHERE project_id = ?`, params.ProjectID).Scan(&projectPrefix, &nextSequence); err != nil {
		return Ticket{}, err
	}
	key, err := generateTicketKey(projectPrefix, params.Type, nextSequence)
	if err != nil {
		return Ticket{}, err
	}
	result, err := tx.Exec(`
		INSERT INTO tasks (key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key, params.ProjectID, nullableInt64(params.ParentID), nullableInt64(params.CloneOf), params.Type, params.Title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), stage, state, RenderLifecycleStatus(stage, state), priority, order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), 0, strings.TrimSpace(params.Assignee), params.CreatedBy)
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
	assignee := strings.TrimSpace(params.Assignee)
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

	explicitLifecycle := normalizeOptional(params.Stage) != "" || normalizeOptional(params.State) != ""
	if hasChildren && explicitLifecycle {
		return Ticket{}, errors.New("ticket has children; stage/state is derived from descendants")
	}
	stage, state, err := resolveLifecycleForUpdate(current, params.Stage, params.State, assignee)
	if err != nil {
		return Ticket{}, err
	}
	if explicitLifecycle && (stage != current.Stage || state != current.State) {
		if params.ActorRole != "admin" && strings.TrimSpace(current.Assignee) != strings.TrimSpace(params.ActorUsername) {
			return Ticket{}, ErrForbidden
		}
		if current.Stage == StageDone {
			return Ticket{}, errors.New("done ticket cannot be reopened")
		}
	}

	result, err := db.Exec(`
		UPDATE tasks
		SET title = ?, description = ?, acceptance_criteria = ?, parent_id = ?, assignee = ?, stage = ?, state = ?, status = ?, priority = ?, sort_order = ?, estimate_effort = ?, estimate_complete = ?, updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), nullableInt64(params.ParentID), assignee, stage, state, RenderLifecycleStatus(stage, state), params.Priority, params.Order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), id)
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
	if err := AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_updated", map[string]any{
		"key":                 ticket.Key,
		"title":               ticket.Title,
		"description":         ticket.Description,
		"acceptance_criteria": ticket.AcceptanceCriteria,
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

func SetTicketHealth(db *sql.DB, id int64, score int) (Ticket, error) {
	if score < 0 || score > 4 {
		return Ticket{}, errors.New("health score must be between 0 and 4")
	}
	result, err := db.Exec(`
		UPDATE tasks
		SET health_score = ?, updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
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
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE project_id = ?
	`
	args := []any{params.ProjectID}
	if taskType := normalizeOptional(params.Type); taskType != "" {
		query += ` AND type = ?`
		args = append(args, taskType)
	}
	if stage := normalizeOptional(params.Stage); stage != "" {
		if !ValidStage(stage) {
			return nil, fmt.Errorf("invalid stage %q", params.Stage)
		}
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
	query += ` ORDER BY created_at, task_id`
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

	var tickets []Ticket
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
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE project_id = ? AND task_id = ?
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
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE task_id = ?
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
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
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

type scanner interface {
	Scan(dest ...any) error
}

func scanTicket(s scanner) (Ticket, error) {
	var ticket Ticket
	var parentID sql.NullInt64
	var cloneOf sql.NullInt64
	var storedStatus string
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
		&ticket.Stage,
		&ticket.State,
		&storedStatus,
		&ticket.Priority,
		&ticket.Order,
		&ticket.EstimateEffort,
		&ticket.EstimateComplete,
		&ticket.HealthScore,
		&ticket.Assignee,
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
	ticket.Status = RenderLifecycleStatus(ticket.Stage, ticket.State)
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
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ?`, id).Scan(&count); err != nil {
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
	task, err := getStoredTicket(db, id)
	if err != nil {
		return nil, err
	}
	children, err := listStoredChildTickets(db, id)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return task.ParentID, nil
	}

	nextStage := StageDone
	allComplete := true
	anyActive := false
	for _, child := range children {
		if CompareStageOrder(child.Stage, nextStage) < 0 {
			nextStage = child.Stage
		}
		if child.State != StateComplete {
			allComplete = false
		}
		if child.State == StateActive {
			anyActive = true
		}
	}
	nextState := StateIdle
	switch {
	case allComplete:
		nextState = StateComplete
	case anyActive:
		nextState = StateActive
	}
	if nextStage == task.Stage && nextState == task.State {
		return task.ParentID, nil
	}

	if _, err := db.Exec(`
		UPDATE tasks
		SET stage = ?, state = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, nextStage, nextState, RenderLifecycleStatus(nextStage, nextState), id); err != nil {
		return nil, err
	}

	if actorID != 0 {
		_ = AddHistoryEvent(db, task.ProjectID, task.ID, "ticket_parent_lifecycle_changed", map[string]any{
			"key":         task.Key,
			"from_stage":  task.Stage,
			"from_state":  task.State,
			"from_status": RenderLifecycleStatus(task.Stage, task.State),
			"to_stage":    nextStage,
			"to_state":    nextState,
			"to_status":   RenderLifecycleStatus(nextStage, nextState),
		}, actorID)
	}
	return task.ParentID, nil
}

func listStoredChildTickets(db *sql.DB, parentID int64) ([]Ticket, error) {
	rows, err := db.Query(`
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE parent_id = ?
		ORDER BY created_at, task_id
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []Ticket
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
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE task_id = ?
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
	if len(parts) != 2 || !ValidLifecycle(parts[0], parts[1]) {
		return "", "", fmt.Errorf("invalid status %q", status)
	}
	return parts[0], parts[1], nil
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

func resolveLifecycleForCreate(stage, state, assignee string) (string, string, error) {
	rawStage := normalizeOptional(stage)
	rawState := normalizeOptional(state)
	if rawStage == "" && rawState == "" {
		return StageDesign, StateIdle, nil
	}
	if rawStage == "" || rawState == "" {
		return "", "", errors.New("stage and state must be set together")
	}
	if !ValidLifecycle(rawStage, rawState) {
		return "", "", fmt.Errorf("invalid lifecycle %s/%s", rawStage, rawState)
	}
	if rawState == StateActive && strings.TrimSpace(assignee) == "" {
		return "", "", errors.New("active ticket requires assignee")
	}
	return rawStage, rawState, nil
}

func resolveLifecycleForUpdate(current Ticket, stage, state, assignee string) (string, string, error) {
	nextStage := current.Stage
	nextState := current.State
	rawStage := normalizeOptional(stage)
	rawState := normalizeOptional(state)
	if rawStage != "" || rawState != "" {
		if rawStage == "" || rawState == "" {
			return "", "", errors.New("stage and state must be set together")
		}
		nextStage = rawStage
		nextState = rawState
	}
	if !ValidLifecycle(nextStage, nextState) {
		return "", "", fmt.Errorf("invalid lifecycle %s/%s", nextStage, nextState)
	}
	if nextState == StateActive && strings.TrimSpace(assignee) == "" {
		return "", "", errors.New("active ticket requires assignee")
	}
	return nextStage, nextState, nil
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
			return fmt.Errorf("task is already assigned to %s", currentAssignee)
		}
		return nil
	}
	if nextAssignee == "" {
		if currentAssignee != actorUsername {
			if currentAssignee == "" {
				return errors.New("task is not assigned to you")
			}
			return fmt.Errorf("task is assigned to %s", currentAssignee)
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

	if task, ok, err := findAssignedTicketForUser(db, params.ProjectID, username, StageDevelop, StateActive); err != nil {
		return Ticket{}, "", err
	} else if ok {
		return task, "ASSIGNED", nil
	}

	if params.TicketID != nil || strings.TrimSpace(params.TicketRef) != "" {
		task, err := resolveRequestedTicket(db, params)
		if err != nil {
			return Ticket{}, "", err
		}
		if strings.TrimSpace(task.Assignee) == username {
			return task, "ASSIGNED", nil
		}
		ok, err := ticketClaimable(db, task, params.ProjectID)
		if err != nil {
			return Ticket{}, "", err
		}
		if !ok {
			return Ticket{}, "REJECTED", nil
		}
		if params.DryRun {
			return withClaimPreview(task, username), "AVAILABLE", nil
		}
		assigned, err := UpdateTicket(db, task.ID, TicketUpdateParams{
			Title:              task.Title,
			Description:        task.Description,
			AcceptanceCriteria: task.AcceptanceCriteria,
			ParentID:           task.ParentID,
			Assignee:           username,
			Stage:              task.Stage,
			State:              StateActive,
			Priority:           task.Priority,
			Order:              task.Order,
			UpdatedBy:          params.UserID,
			ActorUsername:      username,
			ActorRole:          "admin",
		})
		if err != nil {
			return Ticket{}, "", err
		}
		return assigned, "ASSIGNED", nil
	}

	task, ok, err := findClaimCandidate(db, params.ProjectID)
	if err != nil {
		return Ticket{}, "", err
	}
	if !ok {
		return Ticket{}, "NO-WORK", nil
	}
	if params.DryRun {
		return withClaimPreview(task, username), "AVAILABLE", nil
	}
	assigned, err := UpdateTicket(db, task.ID, TicketUpdateParams{
		Title:              task.Title,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		ParentID:           task.ParentID,
		Assignee:           username,
		Stage:              task.Stage,
		State:              StateActive,
		Priority:           task.Priority,
		Order:              task.Order,
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
	if project.Status != "open" || ticket.Archived {
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

func findAssignedTicketForUser(db *sql.DB, projectID int64, username, stage, state string) (Ticket, bool, error) {
	query := `
		SELECT task_id, key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, archived, COALESCE(created_by, 0), created_at, updated_at
		FROM tasks
		WHERE assignee = ? AND stage = ? AND state = ?
	`
	args := []any{username, stage, state}
	if projectID != 0 {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at, task_id LIMIT 1`
	task, err := scanTicket(db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, false, nil
		}
		return Ticket{}, false, err
	}
	return task, true, nil
}

func findClaimCandidate(db *sql.DB, projectID int64) (Ticket, bool, error) {
	if projectID == 0 {
		return Ticket{}, false, errors.New("project is required")
	}
	task, err := scanTicket(db.QueryRow(`
		SELECT t.task_id, t.key, t.project_id, t.parent_id, t.clone_of, t.type, t.title, t.description, t.acceptance_criteria, t.stage, t.state, t.status, t.priority, t.sort_order, t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, t.archived, COALESCE(t.created_by, 0), t.created_at, t.updated_at
		FROM tasks t
		JOIN projects p ON p.project_id = t.project_id
		WHERE t.project_id = ? AND p.status = 'open' AND t.archived = 0 AND t.stage = ? AND t.state = ? AND TRIM(COALESCE(t.assignee, '')) = ''
		  AND NOT EXISTS (SELECT 1 FROM tasks c WHERE c.parent_id = t.task_id)
		ORDER BY t.priority DESC, t.created_at, t.key, t.task_id
		LIMIT 1
	`, projectID, StageDevelop, StateIdle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Ticket{}, false, nil
		}
		return Ticket{}, false, err
	}
	return task, true, nil
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
		Stage:              StageDesign,
		State:              StateIdle,
		CreatedBy:          createdBy,
	})
	if err != nil {
		return Ticket{}, err
	}
	if original.Type != "epic" {
		return cloned, nil
	}
	children, err := ListTickets(db, TicketListParams{ProjectID: original.ProjectID})
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
	task, err := GetTicket(db, id)
	if err != nil {
		return err
	}
	parentID := task.ParentID

	children, err := ListTickets(db, TicketListParams{ProjectID: task.ProjectID})
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

	if _, err := tx.Exec(`UPDATE tasks SET clone_of = NULL WHERE clone_of = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM dependencies WHERE task_id = ? OR depends_on = ?`, id, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM comments WHERE item_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM history_events WHERE task_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM ticket_history WHERE task_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM tasks WHERE task_id = ?`, id)
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
