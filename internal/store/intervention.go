package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	InterventionStateOpen       = "open"
	InterventionStateTriaged    = "triaged"
	InterventionStateInProgress = "in_progress"
	InterventionStateResolved   = "resolved"
	InterventionStateWontFix    = "wont_fix"
)

type InterventionState struct {
	TicketID    string `json:"ticket_id"`
	ProjectID   int64  `json:"project_id"`
	State       string `json:"state"`
	OwnerUserID string `json:"owner_user_id,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
	UpdatedBy   string `json:"updated_by,omitempty"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type InterventionReport struct {
	ProjectID       int64                    `json:"project_id"`
	OpenCount       int                      `json:"open_count"`
	TriagedCount    int                      `json:"triaged_count"`
	InProgressCount int                      `json:"in_progress_count"`
	ResolvedCount   int                      `json:"resolved_count"`
	WontFixCount    int                      `json:"wont_fix_count"`
	OldestOpenAgeH  int                      `json:"oldest_open_age_h"`
	Trends          []InterventionTrendPoint `json:"trends,omitempty"`
	Items           []InterventionReportItem `json:"items"`
}

type InterventionTrendPoint struct {
	Day             string `json:"day"`
	OpenCount       int    `json:"open_count"`
	TriagedCount    int    `json:"triaged_count"`
	InProgressCount int    `json:"in_progress_count"`
	ResolvedCount   int    `json:"resolved_count"`
	WontFixCount    int    `json:"wont_fix_count"`
}

type InterventionReportItem struct {
	TicketID    string `json:"ticket_id"`
	Title       string `json:"title"`
	State       string `json:"state"`
	OwnerUserID string `json:"owner_user_id,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
	AgeHours    int    `json:"age_hours"`
	Escalated   bool   `json:"escalated"`
	UpdatedAt   string `json:"updated_at"`
}

type InterventionDrilldownBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type InterventionDrilldown struct {
	ProjectID      int64                         `json:"project_id"`
	EscalationH    int                           `json:"escalation_hours"`
	EscalatedCount int                           `json:"escalated_count"`
	ByState        []InterventionDrilldownBucket `json:"by_state"`
	ByOwner        []InterventionDrilldownBucket `json:"by_owner"`
}

func normalizeInterventionState(raw string) (string, error) {
	state := strings.TrimSpace(strings.ToLower(raw))
	if state == "" {
		return InterventionStateOpen, nil
	}
	switch state {
	case InterventionStateOpen, InterventionStateTriaged, InterventionStateInProgress, InterventionStateResolved, InterventionStateWontFix:
		return state, nil
	default:
		return "", errors.New("invalid intervention state")
	}
}

func ensureInterventionStateTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS intervention_states (
			ticket_id TEXT PRIMARY KEY,
			project_id INTEGER NOT NULL,
			state TEXT NOT NULL DEFAULT 'open',
			owner_user_id TEXT,
			updated_by TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(ticket_id) REFERENCES tickets(ticket_id) ON DELETE CASCADE,
			FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
			FOREIGN KEY(owner_user_id) REFERENCES users(user_id),
			FOREIGN KEY(updated_by) REFERENCES users(user_id)
		);
		CREATE INDEX IF NOT EXISTS idx_intervention_states_project_id ON intervention_states(project_id);
		CREATE INDEX IF NOT EXISTS idx_intervention_states_state ON intervention_states(state);
	`)
	return err
}

func GetInterventionState(ctx context.Context, db *sql.DB, ticketID string) (InterventionState, error) {
	if err := ensureInterventionStateTable(ctx, db); err != nil {
		return InterventionState{}, err
	}
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return InterventionState{}, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT i.ticket_id, i.project_id, i.state, i.owner_user_id, COALESCE(u.username, ''), i.updated_by, i.updated_at, i.created_at
		FROM intervention_states i
		LEFT JOIN users u ON u.user_id = i.owner_user_id
		WHERE i.ticket_id = ?
	`, ticketID)
	var state InterventionState
	var ownerUserID sql.NullString
	var updatedBy sql.NullString
	if scanErr := row.Scan(&state.TicketID, &state.ProjectID, &state.State, &ownerUserID, &state.OwnerName, &updatedBy, &state.UpdatedAt, &state.CreatedAt); scanErr == nil {
		if ownerUserID.Valid {
			state.OwnerUserID = ownerUserID.String
		}
		if updatedBy.Valid {
			state.UpdatedBy = updatedBy.String
		}
		return state, nil
	} else if !errors.Is(scanErr, sql.ErrNoRows) {
		return InterventionState{}, scanErr
	}
	fallbackState := InterventionStateOpen
	if strings.EqualFold(strings.TrimSpace(ticket.State), StateFail) {
		fallbackState = InterventionStateOpen
	}
	return InterventionState{
		TicketID:  ticket.ID,
		ProjectID: ticket.ProjectID,
		State:     fallbackState,
	}, nil
}

func SetInterventionState(ctx context.Context, db *sql.DB, ticketID, state, ownerUserID, updatedBy string) (InterventionState, error) {
	if err := ensureInterventionStateTable(ctx, db); err != nil {
		return InterventionState{}, err
	}
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return InterventionState{}, err
	}
	normalizedState, err := normalizeInterventionState(state)
	if err != nil {
		return InterventionState{}, err
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	updatedBy = strings.TrimSpace(updatedBy)
	_, err = db.ExecContext(ctx, `
		INSERT INTO intervention_states (ticket_id, project_id, state, owner_user_id, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(ticket_id) DO UPDATE SET
			state = excluded.state,
			owner_user_id = excluded.owner_user_id,
			updated_by = excluded.updated_by,
			updated_at = CURRENT_TIMESTAMP
	`, ticket.ID, ticket.ProjectID, normalizedState, nullableUserID(ownerUserID), nullableUserID(updatedBy))
	if err != nil {
		return InterventionState{}, err
	}
	return GetInterventionState(ctx, db, ticketID)
}

func ClaimIntervention(ctx context.Context, db *sql.DB, ticketID, ownerUserID, updatedBy string) (InterventionState, error) {
	return SetInterventionState(ctx, db, ticketID, InterventionStateInProgress, ownerUserID, updatedBy)
}

func BuildInterventionReport(ctx context.Context, db *sql.DB, projectID int64, escalationHours int) (InterventionReport, error) {
	if err := ensureInterventionStateTable(ctx, db); err != nil {
		return InterventionReport{}, err
	}
	if escalationHours <= 0 {
		escalationHours = 24
	}
	rows, err := db.QueryContext(ctx, `
		SELECT t.ticket_id, t.title, COALESCE(i.state, 'open'), i.owner_user_id, COALESCE(u.username, ''), COALESCE(i.updated_at, t.updated_at)
		FROM tickets t
		LEFT JOIN intervention_states i ON i.ticket_id = t.ticket_id
		LEFT JOIN users u ON u.user_id = i.owner_user_id
		WHERE t.project_id = ? AND t.state = 'fail' AND t.deleted = 0
		ORDER BY COALESCE(i.updated_at, t.updated_at) ASC
	`, projectID)
	if err != nil {
		return InterventionReport{}, err
	}
	defer rows.Close()
	report := InterventionReport{
		ProjectID: projectID,
		Items:     make([]InterventionReportItem, 0),
	}
	now := time.Now().UTC()
	for rows.Next() {
		var item InterventionReportItem
		var ownerUserID sql.NullString
		if scanErr := rows.Scan(&item.TicketID, &item.Title, &item.State, &ownerUserID, &item.OwnerName, &item.UpdatedAt); scanErr != nil {
			return InterventionReport{}, scanErr
		}
		if ownerUserID.Valid {
			item.OwnerUserID = ownerUserID.String
		}
		updatedAt, parseErr := time.Parse("2006-01-02 15:04:05", item.UpdatedAt)
		if parseErr != nil {
			updatedAt, parseErr = time.Parse(time.RFC3339, item.UpdatedAt)
			if parseErr != nil {
				return InterventionReport{}, fmt.Errorf("invalid intervention timestamp for %s: %w", item.TicketID, parseErr)
			}
		}
		ageHours := int(now.Sub(updatedAt.UTC()).Hours())
		if ageHours < 0 {
			ageHours = 0
		}
		item.AgeHours = ageHours
		item.Escalated = ageHours >= escalationHours && (item.State == InterventionStateOpen || item.State == InterventionStateTriaged || item.State == InterventionStateInProgress)
		switch item.State {
		case InterventionStateOpen:
			report.OpenCount++
		case InterventionStateTriaged:
			report.TriagedCount++
		case InterventionStateInProgress:
			report.InProgressCount++
		case InterventionStateResolved:
			report.ResolvedCount++
		case InterventionStateWontFix:
			report.WontFixCount++
		}
		if (item.State == InterventionStateOpen || item.State == InterventionStateTriaged || item.State == InterventionStateInProgress) && item.AgeHours > report.OldestOpenAgeH {
			report.OldestOpenAgeH = item.AgeHours
		}
		report.Items = append(report.Items, item)
	}
	if err := rows.Err(); err != nil {
		return InterventionReport{}, err
	}
	trends, trendErr := BuildInterventionTrends(ctx, db, projectID, 7)
	if trendErr != nil {
		return InterventionReport{}, trendErr
	}
	report.Trends = trends
	return report, nil
}

func BuildInterventionTrends(ctx context.Context, db *sql.DB, projectID int64, days int) ([]InterventionTrendPoint, error) {
	if err := ensureInterventionStateTable(ctx, db); err != nil {
		return nil, err
	}
	if days <= 0 {
		days = 7
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DATE(COALESCE(i.updated_at, t.updated_at)) AS d,
		       COALESCE(i.state, 'open') AS intervention_state,
		       COUNT(1)
		FROM tickets t
		LEFT JOIN intervention_states i ON i.ticket_id = t.ticket_id
		WHERE t.project_id = ? AND t.state = 'fail' AND t.deleted = 0
		  AND DATE(COALESCE(i.updated_at, t.updated_at)) >= DATE('now', ?)
		GROUP BY d, intervention_state
		ORDER BY d ASC
	`, projectID, fmt.Sprintf("-%d days", days-1))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pointsByDay := map[string]*InterventionTrendPoint{}
	orderedDays := make([]string, 0)
	for rows.Next() {
		var day string
		var state string
		var count int
		if scanErr := rows.Scan(&day, &state, &count); scanErr != nil {
			return nil, scanErr
		}
		point, ok := pointsByDay[day]
		if !ok {
			point = &InterventionTrendPoint{Day: day}
			pointsByDay[day] = point
			orderedDays = append(orderedDays, day)
		}
		switch strings.ToLower(strings.TrimSpace(state)) {
		case InterventionStateOpen:
			point.OpenCount = count
		case InterventionStateTriaged:
			point.TriagedCount = count
		case InterventionStateInProgress:
			point.InProgressCount = count
		case InterventionStateResolved:
			point.ResolvedCount = count
		case InterventionStateWontFix:
			point.WontFixCount = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if _, ok := pointsByDay[day]; !ok {
			pointsByDay[day] = &InterventionTrendPoint{Day: day}
			orderedDays = append(orderedDays, day)
		}
	}
	sort.Strings(orderedDays)
	points := make([]InterventionTrendPoint, 0, len(orderedDays))
	seen := map[string]bool{}
	for _, day := range orderedDays {
		if seen[day] {
			continue
		}
		seen[day] = true
		points = append(points, *pointsByDay[day])
	}
	return points, nil
}

func BuildInterventionDrilldown(ctx context.Context, db *sql.DB, projectID int64, escalationHours int) (InterventionDrilldown, error) {
	report, err := BuildInterventionReport(ctx, db, projectID, escalationHours)
	if err != nil {
		return InterventionDrilldown{}, err
	}
	result := InterventionDrilldown{
		ProjectID:   projectID,
		EscalationH: escalationHours,
		ByState:     make([]InterventionDrilldownBucket, 0),
		ByOwner:     make([]InterventionDrilldownBucket, 0),
	}
	stateCounts := map[string]int{}
	ownerCounts := map[string]int{}
	for _, item := range report.Items {
		key := strings.TrimSpace(item.State)
		if key == "" {
			key = InterventionStateOpen
		}
		stateCounts[key]++
		owner := strings.TrimSpace(item.OwnerName)
		if owner == "" {
			owner = "unassigned"
		}
		ownerCounts[owner]++
		if item.Escalated {
			result.EscalatedCount++
		}
	}
	for key, count := range stateCounts {
		result.ByState = append(result.ByState, InterventionDrilldownBucket{Key: key, Count: count})
	}
	for key, count := range ownerCounts {
		result.ByOwner = append(result.ByOwner, InterventionDrilldownBucket{Key: key, Count: count})
	}
	sort.SliceStable(result.ByState, func(i, j int) bool {
		if result.ByState[i].Count != result.ByState[j].Count {
			return result.ByState[i].Count > result.ByState[j].Count
		}
		return result.ByState[i].Key < result.ByState[j].Key
	})
	sort.SliceStable(result.ByOwner, func(i, j int) bool {
		if result.ByOwner[i].Count != result.ByOwner[j].Count {
			return result.ByOwner[i].Count > result.ByOwner[j].Count
		}
		return result.ByOwner[i].Key < result.ByOwner[j].Key
	})
	return result, nil
}
