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
	ID                  string    `json:"ticket_id"`
	ProjectID           int64     `json:"project_id"`
	ParentID            *string   `json:"parent_id,omitempty"`
	CloneOf             *string   `json:"clone_of,omitempty"`
	Type                string    `json:"type"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	AcceptanceCriteria  string    `json:"acceptance_criteria"`
	GitRepository       string    `json:"git_repository"`
	GitBranch           string    `json:"git_branch"`
	SdlcID              *int64    `json:"sdlc_id,omitempty"`
	SdlcStageID         *int64    `json:"sdlc_stage_id,omitempty"`
	RoleID              *int64    `json:"role_id,omitempty"`
	Stage               string    `json:"stage"`
	State               string    `json:"state"`
	Status              string    `json:"status"`
	Priority            int       `json:"priority"`
	Order               int       `json:"order"`
	EstimateEffort      int       `json:"estimate_effort"`
	EstimateComplete    string    `json:"estimate_complete,omitempty"`
	HealthScore         int       `json:"health_score"`
	Assignee            string    `json:"assignee"`
	Author              string    `json:"author"`
	Comments            []Comment `json:"comments,omitempty"`
	Draft               bool      `json:"draft"`
	Complete            bool      `json:"complete"`
	Archived            bool      `json:"archived"`
	PreviousSdlcStageID *int64    `json:"previous_sdlc_stage_id,omitempty"`
	PreviousRoleID      *int64    `json:"previous_role_id,omitempty"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           string    `json:"created_at"`
	UpdatedAt           string    `json:"updated_at"`
}

type TicketCreateParams struct {
	ProjectID          int64
	ParentID           *string
	CloneOf            *string
	SdlcID             *int64
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
	Type               string // if non-empty, update the ticket type
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
	Offset          int
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
	var projectSdlcID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT prefix, ticket_sequence + 1, sdlc_id FROM projects WHERE project_id = ?`, params.ProjectID).Scan(&projectPrefix, &nextSequence, &projectSdlcID); err != nil {
		return Ticket{}, err
	}
	// Resolve effective sdlc: ticket param → parent chain → project
	var effectiveSdlcID sql.NullInt64
	var ticketSdlcID *int64 // stored on the ticket itself (NULL = inherit)
	if params.SdlcID != nil {
		effectiveSdlcID = sql.NullInt64{Int64: *params.SdlcID, Valid: true}
		ticketSdlcID = params.SdlcID
	} else if params.ParentID != nil {
		// Walk parent chain for explicit sdlc
		pid := params.ParentID
		for pid != nil {
			var pwf sql.NullInt64
			var ppid sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT sdlc_id, parent_id FROM tickets WHERE ticket_id = ?`, *pid).Scan(&pwf, &ppid); err != nil {
				break
			}
			if pwf.Valid {
				effectiveSdlcID = pwf
				break
			}
			if ppid.Valid {
				pid = &ppid.String
			} else {
				pid = nil
			}
		}
		if !effectiveSdlcID.Valid {
			effectiveSdlcID = projectSdlcID
		}
	} else {
		effectiveSdlcID = projectSdlcID
	}
	// Resolve initial sdlc stage (first stage by sort_order)
	var sdlcStageID *int64
	stage := StageDesign // fallback
	if effectiveSdlcID.Valid {
		var wsID int64
		var stageName string
		err := tx.QueryRowContext(ctx, `SELECT sdlc_stage_id, stage_name FROM sdlc_stages WHERE sdlc_id = ? ORDER BY sort_order LIMIT 1`, effectiveSdlcID.Int64).Scan(&wsID, &stageName)
		if err == nil {
			sdlcStageID = &wsID
			stage = stageName
		}
	}
	key, err := generateTicketKey(projectPrefix, params.Type, nextSequence)
	if err != nil {
		return Ticket{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tickets (ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, author, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key, params.ProjectID, nullableString(params.ParentID), nullableString(params.CloneOf), params.Type, params.Title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.GitBranch), nullableInt64(ticketSdlcID), nullableInt64(sdlcStageID), stage, state, RenderLifecycleStatus(stage, state), priority, order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), 0, strings.TrimSpace(params.Assignee), strings.TrimSpace(params.Author), nullableUserID(params.CreatedBy))
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
	nextType := current.Type
	if t := strings.TrimSpace(params.Type); t != "" {
		t = normalizeTicketType(t)
		if !validTicketType(t) {
			return Ticket{}, fmt.Errorf("invalid ticket type %q", params.Type)
		}
		nextType = t
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
	if current.Complete && !explicitStageOverride {
		return Ticket{}, ErrTicketClosed
	}
	if current.Archived {
		return Ticket{}, ErrTicketArchived
	}
	reopened := current.Complete && explicitStageOverride
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
	sdlcStageID := current.SdlcStageID
	// Direct stage override (e.g. drag-and-drop on the board)
	if explicitStage {
		nextStage, err := validateTicketStage(ctx, db, current, params.Stage)
		if err != nil {
			return Ticket{}, err
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
			// Update sdlc_stage_id to match the new stage (if a sdlc is attached)
			if current.SdlcStageID != nil {
				var sdlcID int64
				if err := db.QueryRowContext(ctx, `SELECT sdlc_id FROM sdlc_stages WHERE sdlc_stage_id = ?`, *current.SdlcStageID).Scan(&sdlcID); err == nil {
					var wsID int64
					if err := db.QueryRowContext(ctx, `SELECT sdlc_stage_id FROM sdlc_stages WHERE sdlc_id = ? AND stage_name = ? LIMIT 1`, sdlcID, stage).Scan(&wsID); err == nil {
						sdlcStageID = &wsID
					} else {
						sdlcStageID = nil
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
		// Check if ticket is at final sdlc stage with success — can't reopen
		if current.State == StateSuccess && current.SdlcStageID != nil {
			nextID, _, err := getNextSdlcStage(ctx, db, *current.SdlcStageID)
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
		if state == StateSuccess && sdlcStageID != nil {
			nextStageID, nextStageName, err := getNextSdlcStage(ctx, db, *sdlcStageID)
			if err == nil && nextStageID != nil {
				sdlcStageID = nextStageID
				stage = nextStageName
				state = StateIdle
			}
			// If no next stage (final stage), stay at current stage with success state
		}
	}

writeTicket:
	completeVal := 0
	if !reopened && current.Complete {
		completeVal = 1
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, git_branch = ?, parent_id = ?, assignee = ?, sdlc_stage_id = ?, stage = ?, state = ?, status = ?, priority = ?, sort_order = ?, estimate_effort = ?, estimate_complete = ?, complete = ?, type = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), nextGitRepository, nextGitBranch, nullableString(params.ParentID), assignee, nullableInt64(sdlcStageID), stage, state, RenderLifecycleStatus(stage, state), params.Priority, params.Order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), completeVal, nextType, id)
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

func SetTicketComplete(ctx context.Context, db *sql.DB, id string, complete bool, actorUsername string, actorID string) (Ticket, error) {
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Complete == complete {
		return current, nil
	}
	var stage, state string
	if complete {
		// Completing: save current position for reopen, move to done
		stage = StageDone
		state = current.State
		if state == StateActive {
			state = StateIdle
		}
	} else {
		// Reopening: restore previous position or default to develop/idle
		if current.PreviousSdlcStageID != nil {
			stage = current.Stage // will be overridden below
		} else {
			stage = StageDevelop
		}
		state = StateIdle
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET complete = ?, stage = ?, state = ?, status = ?,
			previous_sdlc_stage_id = CASE WHEN ? = 1 THEN sdlc_stage_id ELSE previous_sdlc_stage_id END,
			previous_role_id = CASE WHEN ? = 1 THEN role_id ELSE previous_role_id END,
			sdlc_stage_id = CASE WHEN ? = 0 THEN previous_sdlc_stage_id ELSE sdlc_stage_id END,
			role_id = CASE WHEN ? = 0 THEN previous_role_id ELSE role_id END,
			updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, boolToInt(complete), stage, state, RenderLifecycleStatus(stage, state),
		boolToInt(complete), boolToInt(complete), boolToInt(complete), boolToInt(complete), id)
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
	if err := addTicketCompleteHistoryEvent(ctx, db, current, current.Complete, complete, actorUsername, actorID); err != nil {
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

func SetTicketDraft(ctx context.Context, db *sql.DB, id string, draft bool, actorUsername string, actorID string) (Ticket, error) {
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Draft == draft {
		return current, nil
	}
	result, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET draft = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, boolToInt(draft), id)
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
	action := "marked_draft"
	if !draft {
		action = "marked_ready"
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, action, map[string]any{
		"from_draft": current.Draft,
		"to_draft":   draft,
		"who":        actorUsername,
	}, actorID); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

func addTicketCompleteHistoryEvent(ctx context.Context, db *sql.DB, current Ticket, from bool, to bool, actorUsername string, actorID string) error {
	if from == to {
		return nil
	}
	if err := AddHistoryEvent(ctx, db, current.ProjectID, current.ID, "ticket_complete_changed", map[string]any{
		"from_complete": from,
		"to_complete":   to,
		"from":          fmt.Sprintf("%t", from),
		"to":            fmt.Sprintf("%t", to),
		"why":           map[bool]string{true: "completed", false: "reopened"}[to],
		"who":           actorUsername,
		"who_id":        actorID,
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

// NextTicket advances a ticket to the next role within its stage, or to the first
// role of the next stage if it's at the last role. Requires state=success.
func NextTicket(ctx context.Context, db *sql.DB, id string, actorUsername, actorID string) (Ticket, error) {
	ticket, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.State != StateSuccess {
		return Ticket{}, fmt.Errorf("cannot advance %s — state is %q, must be %q", id, ticket.State, StateSuccess)
	}
	if ticket.Complete {
		return Ticket{}, fmt.Errorf("cannot advance %s — ticket is complete", id)
	}
	if ticket.SdlcStageID == nil {
		return Ticket{}, fmt.Errorf("cannot advance %s — no SDLC stage assigned", id)
	}

	// Find the next role in the current stage, or the first role in the next stage.
	nextStageID, nextRoleID, nextStageName, done, err := findNextStep(ctx, db, *ticket.SdlcStageID, ticket.RoleID)
	if err != nil {
		return Ticket{}, err
	}
	if done {
		// Last step — mark complete
		if _, err := db.ExecContext(ctx, `
			UPDATE tickets SET complete = 1, stage = 'done', state = 'idle', status = 'done/idle',
				sdlc_stage_id = ?, role_id = NULL, updated_at = CURRENT_TIMESTAMP
			WHERE ticket_id = ?`, nextStageID, id); err != nil {
			return Ticket{}, err
		}
	} else {
		if _, err := db.ExecContext(ctx, `
			UPDATE tickets SET sdlc_stage_id = ?, role_id = ?, stage = ?, state = 'idle',
				status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE ticket_id = ?`, nextStageID, nextRoleID, nextStageName, RenderLifecycleStatus(nextStageName, StateIdle), id); err != nil {
			return Ticket{}, err
		}
	}
	return GetTicket(ctx, db, id)
}

// PreviousTicket moves a ticket back to the previous role or stage. Requires state=fail.
func PreviousTicket(ctx context.Context, db *sql.DB, id string, actorUsername, actorID string) (Ticket, error) {
	ticket, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.State != StateFail {
		return Ticket{}, fmt.Errorf("cannot regress %s — state is %q, must be %q", id, ticket.State, StateFail)
	}
	if ticket.SdlcStageID == nil {
		return Ticket{}, fmt.Errorf("cannot regress %s — no SDLC stage assigned", id)
	}

	prevStageID, prevRoleID, prevStageName, atStart, err := findPrevStep(ctx, db, *ticket.SdlcStageID, ticket.RoleID)
	if err != nil {
		return Ticket{}, err
	}
	if atStart {
		return Ticket{}, fmt.Errorf("cannot regress %s — already at the first step", id)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET sdlc_stage_id = ?, role_id = ?, stage = ?, state = 'idle',
			status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?`, prevStageID, prevRoleID, prevStageName, RenderLifecycleStatus(prevStageName, StateIdle), id); err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, id)
}

// findNextStep finds the next role in the current stage, or the first role of the next stage.
// Returns (stageID, roleID, stageName, done, error). done=true means the ticket has completed all steps.
func findNextStep(ctx context.Context, db *sql.DB, currentStageID int64, currentRoleID *int64) (int64, *int64, string, bool, error) {
	// Get the current stage's SDLC ID and stage info
	var sdlcID int64
	var currentOrder int
	var currentStageName string
	if err := db.QueryRowContext(ctx, `SELECT sdlc_id, sort_order, stage_name FROM sdlc_stages WHERE sdlc_stage_id = ?`, currentStageID).Scan(&sdlcID, &currentOrder, &currentStageName); err != nil {
		return 0, nil, "", false, err
	}

	// Get roles for the current stage
	roles, err := ListSdlcStageRoles(ctx, db, sdlcID, currentStageID)
	if err != nil {
		return 0, nil, "", false, err
	}

	// Find current role index
	currentRoleIdx := -1
	if currentRoleID != nil {
		for i, r := range roles {
			if r.ID == *currentRoleID {
				currentRoleIdx = i
				break
			}
		}
	}

	// If there's a next role in this stage, return it
	if currentRoleIdx >= 0 && currentRoleIdx < len(roles)-1 {
		nextRole := roles[currentRoleIdx+1]
		return currentStageID, &nextRole.ID, currentStageName, false, nil
	}

	// Otherwise, move to the next stage
	var nextStageID int64
	var nextStageName string
	err = db.QueryRowContext(ctx, `SELECT sdlc_stage_id, stage_name FROM sdlc_stages WHERE sdlc_id = ? AND sort_order > ? ORDER BY sort_order LIMIT 1`, sdlcID, currentOrder).Scan(&nextStageID, &nextStageName)
	if errors.Is(err, sql.ErrNoRows) {
		// No next stage — done
		return currentStageID, nil, "done", true, nil
	}
	if err != nil {
		return 0, nil, "", false, err
	}

	// Check if the next stage is "done"
	if nextStageName == StageDone {
		return nextStageID, nil, StageDone, true, nil
	}

	// Get the first role in the next stage
	nextRoles, err := ListSdlcStageRoles(ctx, db, sdlcID, nextStageID)
	if err != nil {
		return 0, nil, "", false, err
	}
	if len(nextRoles) > 0 {
		return nextStageID, &nextRoles[0].ID, nextStageName, false, nil
	}
	return nextStageID, nil, nextStageName, false, nil
}

// findPrevStep finds the previous role in the current stage, or the last role of the previous stage.
func findPrevStep(ctx context.Context, db *sql.DB, currentStageID int64, currentRoleID *int64) (int64, *int64, string, bool, error) {
	var sdlcID int64
	var currentOrder int
	var currentStageName string
	if err := db.QueryRowContext(ctx, `SELECT sdlc_id, sort_order, stage_name FROM sdlc_stages WHERE sdlc_stage_id = ?`, currentStageID).Scan(&sdlcID, &currentOrder, &currentStageName); err != nil {
		return 0, nil, "", false, err
	}

	roles, err := ListSdlcStageRoles(ctx, db, sdlcID, currentStageID)
	if err != nil {
		return 0, nil, "", false, err
	}

	currentRoleIdx := -1
	if currentRoleID != nil {
		for i, r := range roles {
			if r.ID == *currentRoleID {
				currentRoleIdx = i
				break
			}
		}
	}

	// If there's a previous role in this stage, return it
	if currentRoleIdx > 0 {
		prevRole := roles[currentRoleIdx-1]
		return currentStageID, &prevRole.ID, currentStageName, false, nil
	}

	// Otherwise, move to the previous stage
	var prevStageID int64
	var prevStageName string
	err = db.QueryRowContext(ctx, `SELECT sdlc_stage_id, stage_name FROM sdlc_stages WHERE sdlc_id = ? AND sort_order < ? ORDER BY sort_order DESC LIMIT 1`, sdlcID, currentOrder).Scan(&prevStageID, &prevStageName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, "", true, nil // at the very start
	}
	if err != nil {
		return 0, nil, "", false, err
	}

	prevRoles, err := ListSdlcStageRoles(ctx, db, sdlcID, prevStageID)
	if err != nil {
		return 0, nil, "", false, err
	}
	if len(prevRoles) > 0 {
		lastRole := prevRoles[len(prevRoles)-1]
		return prevStageID, &lastRole.ID, prevStageName, false, nil
	}
	return prevStageID, nil, prevStageName, false, nil
}

func SetTicketHealth(ctx context.Context, db *sql.DB, id string, score int) (Ticket, error) {
	if score < 0 || score > 4 {
		return Ticket{}, errors.New("health score must be between 0 and 4")
	}
	current, err := GetTicket(ctx, db, id)
	if err != nil {
		return Ticket{}, err
	}
	if current.Complete {
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
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
		query += ` AND archived = 0 AND complete = 0`
	}
	query += ` ORDER BY updated_at DESC, sort_order, ticket_id`
	if params.Limit < 0 {
		return nil, errors.New("limit must be zero or greater")
	}
	if params.Offset < 0 {
		return nil, errors.New("offset must be zero or greater")
	}
	if params.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, params.Limit)
	}
	if params.Offset > 0 {
		if params.Limit == 0 {
			query += ` LIMIT -1`
		}
		query += ` OFFSET ?`
		args = append(args, params.Offset)
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
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
	upper := strings.ToUpper(raw)

	// 1. Try exact match first.
	ticket, err := GetTicket(ctx, db, upper)
	if err == nil {
		return ticket, nil
	}

	// 2. Bare integer (e.g. "124") — try PREFIX-N for each project prefix.
	if isDigitsOnly(upper) {
		return resolveBySequenceNumber(ctx, db, upper)
	}

	// 3. PREFIX-N that didn't match — might be a legacy PREFIX-X-N id with
	//    the type code stripped. Try all single-letter type codes.
	if parts := strings.SplitN(upper, "-", 3); len(parts) == 2 && isDigitsOnly(parts[1]) {
		for _, code := range []string{"E", "T", "B", "S", "C", "N", "Q", "R", "D"} {
			candidate := parts[0] + "-" + code + "-" + parts[1]
			if t, err := GetTicket(ctx, db, candidate); err == nil {
				return t, nil
			}
		}
	}

	return Ticket{}, ErrTicketNotFound
}

// isDigitsOnly returns true if s is non-empty and contains only ASCII digits.
func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveBySequenceNumber tries PREFIX-N for every known project prefix.
func resolveBySequenceNumber(ctx context.Context, db *sql.DB, num string) (Ticket, error) {
	rows, err := db.QueryContext(ctx, `SELECT prefix FROM projects ORDER BY project_id`)
	if err != nil {
		return Ticket{}, err
	}
	defer rows.Close()
	var prefixes []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return Ticket{}, err
		}
		prefixes = append(prefixes, p)
	}
	if err := rows.Err(); err != nil {
		return Ticket{}, err
	}
	// Try new-style PREFIX-N first, then legacy PREFIX-X-N.
	for _, prefix := range prefixes {
		candidate := prefix + "-" + num
		if t, err := GetTicket(ctx, db, candidate); err == nil {
			return t, nil
		}
	}
	for _, prefix := range prefixes {
		for _, code := range []string{"E", "T", "B", "S", "C", "N", "Q", "R", "D"} {
			candidate := prefix + "-" + code + "-" + num
			if t, err := GetTicket(ctx, db, candidate); err == nil {
				return t, nil
			}
		}
	}
	return Ticket{}, ErrTicketNotFound
}

func ListTicketParents(ctx context.Context, db *sql.DB, id string) ([]Ticket, error) {
	// Load the full ancestor chain in one recursive CTE query instead of
	// making one GetTicket() call per ancestor level (O(depth) queries).
	rows, err := db.QueryContext(ctx, `
		WITH RECURSIVE ancestors(ticket_id, project_id, parent_id, clone_of, type, title,
		  description, acceptance_criteria, git_repository, git_branch,
		  sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order,
		  estimate_effort, estimate_complete, health_score, assignee, author,
		  draft, complete, archived, previous_sdlc_stage_id, previous_role_id, created_by, created_at, updated_at) AS (
			SELECT ticket_id, project_id, parent_id, clone_of, type, title,
			  description, acceptance_criteria, git_repository, git_branch,
			  sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order,
			  estimate_effort, estimate_complete, health_score, assignee, COALESCE(author,''),
			  draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by,''), created_at, updated_at
			FROM tickets WHERE ticket_id = ?
			UNION ALL
			SELECT t.ticket_id, t.project_id, t.parent_id, t.clone_of, t.type, t.title,
			  t.description, t.acceptance_criteria, t.git_repository, t.git_branch,
			  t.sdlc_id, t.sdlc_stage_id, t.role_id, t.stage, t.state, t.status, t.priority, t.sort_order,
			  t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, COALESCE(t.author,''),
			  t.draft, t.complete, t.archived, t.previous_sdlc_stage_id, t.previous_role_id, COALESCE(t.created_by,''), t.created_at, t.updated_at
			FROM tickets t
			JOIN ancestors a ON t.ticket_id = a.parent_id
		)
		SELECT ticket_id, project_id, parent_id, clone_of, type, title,
		  description, acceptance_criteria, git_repository, git_branch,
		  sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order,
		  estimate_effort, estimate_complete, health_score, assignee, author,
		  draft, complete, archived, previous_sdlc_stage_id, previous_role_id, created_by, created_at, updated_at
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
	var sdlcID sql.NullInt64
	var sdlcStageID sql.NullInt64
	var roleID sql.NullInt64
	var storedStatus string
	var draft int
	var complete int
	var archived int
	var prevStageID sql.NullInt64
	var prevRoleID sql.NullInt64
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
		&sdlcID,
		&sdlcStageID,
		&roleID,
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
		&draft,
		&complete,
		&archived,
		&prevStageID,
		&prevRoleID,
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
	if sdlcID.Valid {
		ticket.SdlcID = &sdlcID.Int64
	}
	if sdlcStageID.Valid {
		ticket.SdlcStageID = &sdlcStageID.Int64
	}
	if roleID.Valid {
		ticket.RoleID = &roleID.Int64
	}
	if prevStageID.Valid {
		ticket.PreviousSdlcStageID = &prevStageID.Int64
	}
	if prevRoleID.Valid {
		ticket.PreviousRoleID = &prevRoleID.Int64
	}
	ticket.Status = RenderLifecycleStatus(ticket.Stage, ticket.State)
	ticket.Draft = draft == 1
	ticket.Complete = complete == 1
	ticket.Archived = archived == 1
	return ticket, nil
}

func validateTicketStage(ctx context.Context, db *sql.DB, ticket Ticket, stage string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(stage))
	if normalized == "" {
		return "", nil
	}
	validStages, err := validStagesForTicket(ctx, db, ticket)
	if err != nil {
		return "", err
	}
	for _, validStage := range validStages {
		if normalized == validStage {
			return normalized, nil
		}
	}
	return "", fmt.Errorf("invalid stage %q; valid stages: %s", stage, strings.Join(validStages, ", "))
}

func validStagesForTicket(ctx context.Context, db *sql.DB, ticket Ticket) ([]string, error) {
	if wfID := ResolveSdlcID(ctx, db, ticket); wfID != nil {
		stages, err := ListSdlcStages(ctx, db, *wfID)
		if err != nil {
			return nil, err
		}
		if names := normalizeStageNames(stages); len(names) > 0 {
			return names, nil
		}
	}
	return []string{StageDesign, StageDevelop, StageTest, StageDone}, nil
}

func normalizeStageNames(stages []SdlcStage) []string {
	names := make([]string, 0, len(stages))
	seen := make(map[string]bool, len(stages))
	for _, stage := range stages {
		name := strings.ToLower(strings.TrimSpace(stage.StageName))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
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
	`, args...) // #nosec G202 -- placeholders are "?" params built from len(ids), not user data
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
	var childID string
	err := db.QueryRowContext(ctx, `SELECT ticket_id FROM tickets WHERE parent_id = ? LIMIT 1`, id).Scan(&childID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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

	// Batch-fetch sort_order for all unique sdlc_stage_ids to avoid N+1 queries.
	stageOrderMap, _ := batchGetSdlcStageOrders(ctx, db, children)

	// Find minimum stage among children by sdlc sort_order
	nextStage := children[0].Stage
	nextSdlcStageID := children[0].SdlcStageID
	minOrder := -1
	if children[0].SdlcStageID != nil {
		if o, ok := stageOrderMap[*children[0].SdlcStageID]; ok {
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
		if child.SdlcStageID != nil {
			if o, ok := stageOrderMap[*child.SdlcStageID]; ok && (minOrder < 0 || o < minOrder) {
				minOrder = o
				nextStage = child.Stage
				nextSdlcStageID = child.SdlcStageID
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
		SET sdlc_stage_id = ?, stage = ?, state = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, nullableInt64(nextSdlcStageID), nextStage, nextState, RenderLifecycleStatus(nextStage, nextState), id); err != nil {
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
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
	if project.Status != "open" || ticket.Complete || ticket.Archived {
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
		SELECT ticket_id, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, git_repository, git_branch, sdlc_id, sdlc_stage_id, role_id, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, health_score, assignee, COALESCE(author, ''), draft, complete, archived, previous_sdlc_stage_id, previous_role_id, COALESCE(created_by, ''), created_at, updated_at
		FROM tickets
		WHERE assignee = ? AND complete = 0 AND archived = 0 AND state = ?
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
		SELECT t.ticket_id, t.project_id, t.parent_id, t.clone_of, t.type, t.title, t.description, t.acceptance_criteria, t.git_repository, t.git_branch, t.sdlc_id, t.sdlc_stage_id, t.role_id, t.stage, t.state, t.status, t.priority, t.sort_order, t.estimate_effort, t.estimate_complete, t.health_score, t.assignee, COALESCE(t.author, ''), t.draft, t.complete, t.archived, t.previous_sdlc_stage_id, t.previous_role_id, COALESCE(t.created_by, ''), t.created_at, t.updated_at
		FROM tickets t
		JOIN projects p ON p.project_id = t.project_id
		WHERE t.project_id = ? AND p.status = 'open' AND t.complete = 0 AND t.archived = 0 AND t.draft = 0 AND t.state = ? AND TRIM(COALESCE(t.assignee, '')) = ''
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
		WHERE project_id = ? AND complete = 0 AND archived = 0
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
		reasons = append(reasons, fmt.Sprintf("  incomplete state=%s: %d", state, count))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count idle unassigned.
	var idleUnassigned int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND complete = 0 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) = ''
	`, projectID).Scan(&idleUnassigned); err != nil {
		return nil, err
	}
	reasons = append(reasons, fmt.Sprintf("idle + unassigned: %d", idleUnassigned))

	// Count not ready.
	var notReady int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND complete = 0 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) = '' AND draft = 1
	`, projectID).Scan(&notReady); err != nil {
		return nil, err
	}
	if notReady > 0 {
		reasons = append(reasons, fmt.Sprintf("draft (blocked): %d — use 'tk ready <id>' to make eligible", notReady))
	}

	// Count with children (non-leaf).
	var withChildren int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tickets t
		WHERE t.project_id = ? AND t.complete = 0 AND t.archived = 0 AND t.draft = 0
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
		WHERE project_id = ? AND complete = 0 AND archived = 0 AND state = 'idle'
		AND TRIM(COALESCE(assignee, '')) != '' AND assignee != ?
	`, projectID, username).Scan(&assignedOther); err != nil {
		return nil, err
	}
	if assignedOther > 0 {
		reasons = append(reasons, fmt.Sprintf("idle but assigned to others: %d", assignedOther))
	}

	// Count closed.
	var closed int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE project_id = ? AND complete = 1`, projectID).Scan(&closed); err != nil {
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
	Project *Project        `json:"project,omitempty"`
	Parents []Ticket        `json:"parents,omitempty"`
	Sdlc    *SdlcWithStages `json:"sdlc,omitempty"`
	Role    *Role           `json:"role,omitempty"`
}

// ResolveSdlcID returns the effective sdlc ID for a ticket by walking:
// ticket.SdlcID → parent chain → project.SdlcID.
func ResolveSdlcID(ctx context.Context, db *sql.DB, ticket Ticket) *int64 {
	if ticket.SdlcID != nil {
		return ticket.SdlcID
	}
	// Walk parent chain
	parentID := ticket.ParentID
	for parentID != nil {
		parent, err := GetTicket(ctx, db, *parentID)
		if err != nil {
			break
		}
		if parent.SdlcID != nil {
			return parent.SdlcID
		}
		parentID = parent.ParentID
	}
	// Fall back to project
	if project, err := GetProjectByID(ctx, db, ticket.ProjectID); err == nil {
		return project.SdlcID
	}
	return nil
}

// EnrichTicketContext gathers the project, parent chain, sdlc, and
// current-stage role for a ticket. Missing data is silently skipped.
func EnrichTicketContext(ctx context.Context, db *sql.DB, ticket Ticket) TicketContext {
	var result TicketContext
	if project, err := GetProjectByID(ctx, db, ticket.ProjectID); err == nil {
		result.Project = &project
	}
	if wfID := ResolveSdlcID(ctx, db, ticket); wfID != nil {
		if wf, err := GetSdlc(ctx, db, *wfID); err == nil {
			result.Sdlc = &wf
			if ticket.RoleID != nil {
				if role, err := GetRoleByID(ctx, db, *ticket.RoleID); err == nil {
					result.Role = &role
				}
			} else if ticket.SdlcStageID != nil {
				// Fall back to first role in the stage
				for _, stage := range wf.Stages {
					if stage.ID == *ticket.SdlcStageID && len(stage.Roles) > 0 {
						result.Role = &stage.Roles[0]
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

// SetTicketSdlc sets an explicit sdlc on a ticket, resetting the
// sdlc stage to the first stage of the new sdlc.
func SetTicketSdlc(ctx context.Context, db *sql.DB, ticketID string, sdlcID int64) (Ticket, error) {
	wf, err := GetSdlc(ctx, db, sdlcID)
	if err != nil {
		return Ticket{}, fmt.Errorf("sdlc %d not found", sdlcID)
	}
	var wsID *int64
	stage := StageDesign
	if len(wf.Stages) > 0 {
		wsID = &wf.Stages[0].ID
		stage = wf.Stages[0].StageName
	}
	_, err = db.ExecContext(ctx, `
		UPDATE tickets
		SET sdlc_id = ?, sdlc_stage_id = ?, stage = ?, state = 'idle', status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, sdlcID, wsID, stage, RenderLifecycleStatus(stage, StateIdle), ticketID)
	if err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, ticketID)
}

// UnsetTicketSdlc clears the explicit sdlc on a ticket, falling back
// to the inherited sdlc and resetting the stage accordingly.
func UnsetTicketSdlc(ctx context.Context, db *sql.DB, ticketID string) (Ticket, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	// Clear the ticket's own sdlc_id
	ticket.SdlcID = nil
	// Resolve inherited sdlc
	wfID := ResolveSdlcID(ctx, db, ticket)
	var wsID *int64
	stage := StageDesign
	if wfID != nil {
		if wf, err := GetSdlc(ctx, db, *wfID); err == nil && len(wf.Stages) > 0 {
			wsID = &wf.Stages[0].ID
			stage = wf.Stages[0].StageName
		}
	}
	_, err = db.ExecContext(ctx, `
		UPDATE tickets
		SET sdlc_id = NULL, sdlc_stage_id = ?, stage = ?, state = 'idle', status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?
	`, wsID, stage, RenderLifecycleStatus(stage, StateIdle), ticketID)
	if err != nil {
		return Ticket{}, err
	}
	return GetTicket(ctx, db, ticketID)
}
