package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
