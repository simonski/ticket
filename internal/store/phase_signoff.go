package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	PhasePlanning       = "planning"
	PhaseImplementation = "implementation"
	PhaseVerification   = "verification"
)

type TicketPhaseSignoff struct {
	TicketID   string `json:"ticket_id"`
	Phase      string `json:"phase"`
	Approved   bool   `json:"approved"`
	ApprovedBy string `json:"approved_by,omitempty"`
	Note       string `json:"note,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

func normalizeSignoffPhase(phase string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(phase))
	switch value {
	case PhasePlanning, PhaseImplementation, PhaseVerification:
		return value, nil
	default:
		return "", fmt.Errorf("invalid phase %q", phase)
	}
}

func ListTicketPhaseSignoffs(ctx context.Context, db *sql.DB, ticketID string) ([]TicketPhaseSignoff, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.ticket_id, s.phase, s.approved, COALESCE(u.username, ''), COALESCE(s.note, ''), s.updated_at
		FROM ticket_phase_signoffs s
		LEFT JOIN users u ON u.user_id = s.approved_by
		WHERE s.ticket_id = ?
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byPhase := map[string]TicketPhaseSignoff{}
	for rows.Next() {
		var entry TicketPhaseSignoff
		var approved int
		if scanErr := rows.Scan(&entry.TicketID, &entry.Phase, &approved, &entry.ApprovedBy, &entry.Note, &entry.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		entry.Approved = approved == 1
		byPhase[entry.Phase] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	phases := []string{PhasePlanning, PhaseImplementation, PhaseVerification}
	result := make([]TicketPhaseSignoff, 0, len(phases))
	for _, phase := range phases {
		if entry, ok := byPhase[phase]; ok {
			result = append(result, entry)
			continue
		}
		result = append(result, TicketPhaseSignoff{
			TicketID: ticketID,
			Phase:    phase,
			Approved: false,
		})
	}
	return result, nil
}

func SetTicketPhaseSignoff(ctx context.Context, db *sql.DB, ticketID, phase string, approved bool, approvedBy, note string) (TicketPhaseSignoff, error) {
	normalizedPhase, err := normalizeSignoffPhase(phase)
	if err != nil {
		return TicketPhaseSignoff{}, err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO ticket_phase_signoffs (ticket_id, phase, approved, approved_by, note, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(ticket_id, phase) DO UPDATE SET
			approved = excluded.approved,
			approved_by = excluded.approved_by,
			note = excluded.note,
			updated_at = CURRENT_TIMESTAMP
	`, ticketID, normalizedPhase, boolToInt(approved), nullableUserID(approvedBy), strings.TrimSpace(note))
	if err != nil {
		return TicketPhaseSignoff{}, err
	}
	entry, err := GetTicketPhaseSignoff(ctx, db, ticketID, normalizedPhase)
	if err != nil {
		return TicketPhaseSignoff{}, err
	}
	return entry, nil
}

func GetTicketPhaseSignoff(ctx context.Context, db *sql.DB, ticketID, phase string) (TicketPhaseSignoff, error) {
	normalizedPhase, err := normalizeSignoffPhase(phase)
	if err != nil {
		return TicketPhaseSignoff{}, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT s.ticket_id, s.phase, s.approved, COALESCE(u.username, ''), COALESCE(s.note, ''), s.updated_at
		FROM ticket_phase_signoffs s
		LEFT JOIN users u ON u.user_id = s.approved_by
		WHERE s.ticket_id = ? AND s.phase = ?
	`, ticketID, normalizedPhase)
	var entry TicketPhaseSignoff
	var approved int
	if err := row.Scan(&entry.TicketID, &entry.Phase, &approved, &entry.ApprovedBy, &entry.Note, &entry.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TicketPhaseSignoff{
				TicketID: ticketID,
				Phase:    normalizedPhase,
				Approved: false,
			}, nil
		}
		return TicketPhaseSignoff{}, err
	}
	entry.Approved = approved == 1
	return entry, nil
}
