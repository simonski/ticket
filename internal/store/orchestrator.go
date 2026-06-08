package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// Orchestrator-related store helpers.
//
// The orchestrator is the single component permitted to assign work to agents.
// It is purely deterministic: it applies workflow rules (advance on success,
// recover on fail, assign idle work, abandon stale work) and never reasons with
// an LLM. See docs/DESIGN_ORCHESTRATOR.md.
//
// Design notes encoded here:
//   - "Sealed" sprint == a sprint in stage 'active'. Activating a sprint already
//     requires every story to have reached 'ready' (see UpdateSprint), so an
//     active sprint is exactly the spec's sealed sprint: ready work, green light.
//   - Per-project enable/disable and the orchestrator timing knobs live in the
//     existing app_settings key/value store, so no schema migration is needed.

const (
	// OrchestratorDefaultIntervalSeconds is the default wake cadence.
	OrchestratorDefaultIntervalSeconds = 30
	// OrchestratorDefaultHeartbeatTimeoutSeconds is how long an assigned agent may
	// go without a heartbeat before its in-flight job is considered abandoned.
	OrchestratorDefaultHeartbeatTimeoutSeconds = 120

	settingOrchestratorInterval         = "orchestrator_interval_seconds"
	settingOrchestratorHeartbeatTimeout = "orchestrator_heartbeat_timeout_seconds"
	settingOrchestratorEnabledPrefix    = "orchestrator_enabled_project_" // + projectID
)

// SprintSealedStage is the sprint stage that means "sealed" — ready work that the
// orchestrator may execute.
const SprintSealedStage = "active"

// SealSprint marks a sprint as sealed (ready for execution). This is the spec's
// "seal" verb; mechanically it activates the sprint, which already validates that
// every story has reached the ready stage.
func SealSprint(ctx context.Context, db *sql.DB, id int) (Sprint, error) {
	s, err := getSprint(ctx, db, id)
	if err != nil {
		return Sprint{}, err
	}
	return UpdateSprint(ctx, db, id, s.Title, SprintSealedStage)
}

// IsSprintSealed reports whether a sprint stage value represents a sealed sprint.
func IsSprintSealed(sprintStage string) bool {
	return sprintStage == SprintSealedStage
}

// ── Config accessors (app_settings KV) ───────────────────────────────────────

// OrchestratorIntervalSeconds returns the orchestrator wake cadence in seconds.
func OrchestratorIntervalSeconds(ctx context.Context, db *sql.DB) (int, error) {
	return appSettingPositiveInt(ctx, db, settingOrchestratorInterval, OrchestratorDefaultIntervalSeconds)
}

// SetOrchestratorIntervalSeconds sets the orchestrator wake cadence.
func SetOrchestratorIntervalSeconds(ctx context.Context, db *sql.DB, seconds int) error {
	if seconds <= 0 {
		seconds = OrchestratorDefaultIntervalSeconds
	}
	return SetAppSetting(ctx, db, settingOrchestratorInterval, strconv.Itoa(seconds))
}

// OrchestratorHeartbeatTimeoutSeconds returns the abandonment timeout in seconds.
func OrchestratorHeartbeatTimeoutSeconds(ctx context.Context, db *sql.DB) (int, error) {
	return appSettingPositiveInt(ctx, db, settingOrchestratorHeartbeatTimeout, OrchestratorDefaultHeartbeatTimeoutSeconds)
}

// SetOrchestratorHeartbeatTimeoutSeconds sets the abandonment timeout.
func SetOrchestratorHeartbeatTimeoutSeconds(ctx context.Context, db *sql.DB, seconds int) error {
	if seconds <= 0 {
		seconds = OrchestratorDefaultHeartbeatTimeoutSeconds
	}
	return SetAppSetting(ctx, db, settingOrchestratorHeartbeatTimeout, strconv.Itoa(seconds))
}

// OrchestratorEnabledForProject reports whether the orchestrator should manage a
// project. Defaults to true (opt-out model).
func OrchestratorEnabledForProject(ctx context.Context, db *sql.DB, projectID int64) (bool, error) {
	key := settingOrchestratorEnabledPrefix + strconv.FormatInt(projectID, 10)
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, err
	}
	return raw == "1" || raw == "true", nil
}

// SetOrchestratorEnabledForProject opts a project in or out of orchestration.
func SetOrchestratorEnabledForProject(ctx context.Context, db *sql.DB, projectID int64, enabled bool) error {
	key := settingOrchestratorEnabledPrefix + strconv.FormatInt(projectID, 10)
	value := "0"
	if enabled {
		value = "1"
	}
	return SetAppSetting(ctx, db, key, value)
}

func appSettingPositiveInt(ctx context.Context, db *sql.DB, key string, def int) (int, error) {
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return def, nil
		}
		return def, err
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(raw))
	if convErr != nil || n <= 0 {
		return def, nil
	}
	return n, nil
}

// ── Candidate enumeration ─────────────────────────────────────────────────────

// OrchestratorTicket is a ticket plus the surrounding facts the orchestrator needs
// to decide an action, gathered in a single sweep query.
type OrchestratorTicket struct {
	TicketID         string
	ProjectID        int64
	Stage            string
	State            string
	RoleID           int64
	RoleTitle        string
	Assignee         string
	Draft            bool
	HasChildren      bool
	WorkflowStageID  int64
	SprintID         *int64
	SprintStage      string
	Priority         int
	AssigneeLastSeen string
	AssigneeIsAgent  bool // true only when the assignee is a known agent user
}

// SprintSealed reports whether this ticket's sprint is sealed (active).
func (t OrchestratorTicket) SprintSealed() bool {
	return t.SprintID != nil && IsSprintSealed(t.SprintStage)
}

// IsLeaf reports whether the ticket has no live children (only leaves are worked).
func (t OrchestratorTicket) IsLeaf() bool { return !t.HasChildren }

// ListOrchestratorCandidates returns every in-flight ticket (not complete,
// archived, or deleted) in an open project, with the joined facts the orchestrator
// needs. Optionally scoped to a single project (projectID != 0) or ticket
// (ticketID != ""). Results are ordered by project, then priority desc, then age.
func ListOrchestratorCandidates(ctx context.Context, db *sql.DB, projectID int64, ticketID string) ([]OrchestratorTicket, error) {
	query := `
		SELECT t.ticket_id, t.project_id, t.stage, t.state,
		       COALESCE(t.role_id, 0), COALESCE(r.title, ''),
		       COALESCE(t.assignee, ''), t.draft,
		       CASE WHEN EXISTS (SELECT 1 FROM tickets c WHERE c.parent_id = t.ticket_id AND c.deleted = 0) THEN 1 ELSE 0 END,
		       COALESCE(t.workflow_stage_id, 0),
		       t.sprint_id, COALESCE(sp.stage, ''),
		       t.priority,
		       COALESCE(au.last_seen, ''),
		       CASE WHEN au.user_id IS NOT NULL THEN 1 ELSE 0 END
		FROM tickets t
		JOIN projects p ON p.project_id = t.project_id
		LEFT JOIN roles r ON r.role_id = t.role_id
		LEFT JOIN sprints sp ON sp.id = t.sprint_id
		LEFT JOIN users au ON au.username = t.assignee AND au.user_type = 'agent'
		WHERE t.complete = 0 AND t.archived = 0 AND t.deleted = 0 AND p.status = 'open'`
	args := []any{}
	if projectID != 0 {
		query += ` AND t.project_id = ?`
		args = append(args, projectID)
	}
	if strings.TrimSpace(ticketID) != "" {
		query += ` AND t.ticket_id = ?`
		args = append(args, strings.TrimSpace(ticketID))
	}
	query += ` ORDER BY t.project_id, t.priority DESC, t.created_at, t.ticket_id`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrchestratorTicket
	for rows.Next() {
		var t OrchestratorTicket
		var draft, hasChildren, isAgent int
		var sprintID sql.NullInt64
		if scanErr := rows.Scan(&t.TicketID, &t.ProjectID, &t.Stage, &t.State,
			&t.RoleID, &t.RoleTitle, &t.Assignee, &draft, &hasChildren,
			&t.WorkflowStageID, &sprintID, &t.SprintStage, &t.Priority,
			&t.AssigneeLastSeen, &isAgent); scanErr != nil {
			return nil, scanErr
		}
		t.AssigneeIsAgent = isAgent != 0
		t.Draft = draft != 0
		t.HasChildren = hasChildren != 0
		if sprintID.Valid {
			id := sprintID.Int64
			t.SprintID = &id
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ── Agent pool (for load-balanced assignment) ─────────────────────────────────

// OrchestratorAgent is an enabled agent plus its current active workload, used to
// pick the least-busy agent able to perform a role.
type OrchestratorAgent struct {
	UserID     string
	Username   string
	Roles      []string
	LastSeen   string
	ActiveLoad int
}

// PerformsRole reports whether the agent can perform the named role
// (case-insensitive). An agent with no roles is a general agent that can take any.
func (a OrchestratorAgent) PerformsRole(roleName string) bool {
	if strings.TrimSpace(roleName) == "" {
		return false
	}
	if len(a.Roles) == 0 {
		return true
	}
	for _, r := range a.Roles {
		if strings.EqualFold(strings.TrimSpace(r), strings.TrimSpace(roleName)) {
			return true
		}
	}
	return false
}

// ListOrchestratorAgents returns all enabled agents with their parsed roles and a
// count of tickets they are currently actively working.
func ListOrchestratorAgents(ctx context.Context, db *sql.DB) ([]OrchestratorAgent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.user_id, u.username, COALESCE(u.agent_role, ''), COALESCE(u.last_seen, ''),
		       (SELECT COUNT(*) FROM tickets t
		          WHERE t.assignee = u.username AND t.state = 'active'
		            AND t.complete = 0 AND t.archived = 0 AND t.deleted = 0) AS active_load
		FROM users u
		WHERE u.user_type = 'agent' AND u.enabled = 1
		ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrchestratorAgent
	for rows.Next() {
		var a OrchestratorAgent
		var roleField string
		if scanErr := rows.Scan(&a.UserID, &a.Username, &roleField, &a.LastSeen, &a.ActiveLoad); scanErr != nil {
			return nil, scanErr
		}
		a.Roles = SplitAgentRoles(roleField)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── Assignment / abandonment mutations ────────────────────────────────────────

// AssignTicketToAgent assigns an idle, unassigned ticket to an agent and marks it
// active. It uses a guarded conditional update so two assigners can never both win
// (belt-and-braces; there is only ever one orchestrator). Returns true if assigned.
func AssignTicketToAgent(ctx context.Context, db *sql.DB, ticketID, agentUsername string, projectID int64) (bool, error) {
	res, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET assignee = ?, state = 'active', status = stage || '/active', updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ? AND state = 'idle' AND TRIM(COALESCE(assignee, '')) = ''
		  AND complete = 0 AND archived = 0 AND deleted = 0`,
		agentUsername, ticketID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	_ = AddHistoryEvent(ctx, db, projectID, ticketID, "orchestrator_assigned", map[string]any{
		"agent": agentUsername,
	}, "orchestrator")
	return true, nil
}

// AbandonTicket releases an in-flight ticket whose agent has gone silent: it
// returns the ticket to idle and clears the assignee so it can be re-assigned. The
// previously-assigned agent learns its ticket is gone the next time it makes
// contact (the work-request path no longer returns it), and drops the work.
func AbandonTicket(ctx context.Context, db *sql.DB, ticketID string, projectID int64, agentUsername, reason string) (bool, error) {
	res, err := db.ExecContext(ctx, `
		UPDATE tickets
		SET assignee = '', state = 'idle', status = stage || '/idle', updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ? AND state = 'active'`,
		ticketID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	_ = AddHistoryEvent(ctx, db, projectID, ticketID, "orchestrator_abandoned", map[string]any{
		"agent":  agentUsername,
		"reason": reason,
	}, "orchestrator")
	return true, nil
}

// ClearTicketAssignee removes the assignee from a ticket (used after the
// orchestrator advances or recovers a story, so the next role starts unassigned).
func ClearTicketAssignee(ctx context.Context, db *sql.DB, ticketID string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE tickets SET assignee = '', updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id = ?`, ticketID)
	return err
}

// AssigneeHeartbeatStale reports whether an assignee's last-seen timestamp is older
// than the timeout (i.e. the agent has gone silent and the job is abandonable).
// An empty lastSeen is treated as stale only when an assignee is actually set.
func AssigneeHeartbeatStale(lastSeen string, now time.Time, timeout time.Duration) bool {
	ls := strings.TrimSpace(lastSeen)
	if ls == "" {
		return true
	}
	parsed, err := time.Parse("2006-01-02 15:04:05", ls)
	if err != nil {
		// Unparseable timestamp — be conservative and do not abandon.
		return false
	}
	return now.UTC().Sub(parsed) > timeout
}
