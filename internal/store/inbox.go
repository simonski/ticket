package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	InboxKindFailureEscalation = "failure_escalation"
	InboxStatusOpen            = "open"
	InboxStatusResolved        = "resolved"

	InboxDecisionStartAgain         = "start_again"
	InboxDecisionRefineRequirements = "refine_requirements"
)

type InboxEntry struct {
	ID              int64    `json:"inbox_id"`
	ProjectID       int64    `json:"project_id"`
	TicketID        string   `json:"ticket_id"`
	Kind            string   `json:"kind"`
	Status          string   `json:"status"`
	Recommendations []string `json:"recommendations"`
	Decision        string   `json:"decision,omitempty"`
	Message         string   `json:"message,omitempty"`
	CreatedBy       string   `json:"created_by,omitempty"`
	DecidedBy       string   `json:"decided_by,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

func defaultFailureEscalationRecommendations() []string {
	return []string{
		InboxDecisionStartAgain,
		InboxDecisionRefineRequirements,
	}
}

func encodeRecommendations(recommendations []string) (string, error) {
	data, err := json.Marshal(recommendations)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeRecommendations(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func scanInboxEntry(scan func(dest ...any) error) (InboxEntry, error) {
	var entry InboxEntry
	var recJSON string
	if err := scan(
		&entry.ID, &entry.ProjectID, &entry.TicketID, &entry.Kind, &entry.Status,
		&recJSON, &entry.Decision, &entry.Message, &entry.CreatedBy, &entry.DecidedBy,
		&entry.CreatedAt, &entry.UpdatedAt,
	); err != nil {
		return InboxEntry{}, err
	}
	recommendations, err := decodeRecommendations(recJSON)
	if err != nil {
		return InboxEntry{}, err
	}
	entry.Recommendations = recommendations
	return entry, nil
}

func GetInboxEntry(ctx context.Context, db *sql.DB, inboxID int64) (InboxEntry, error) {
	row := db.QueryRowContext(ctx, `
		SELECT inbox_id, project_id, ticket_id, kind, status, recommendations_json,
		       COALESCE(decision, ''), COALESCE(message, ''), COALESCE(created_by, ''), COALESCE(decided_by, ''),
		       created_at, updated_at
		FROM inbox_entries
		WHERE inbox_id = ?
	`, inboxID)
	return scanInboxEntry(row.Scan)
}

func ListInboxEntriesByTicket(ctx context.Context, db *sql.DB, ticketID, status string) ([]InboxEntry, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	query := `
		SELECT inbox_id, project_id, ticket_id, kind, status, recommendations_json,
		       COALESCE(decision, ''), COALESCE(message, ''), COALESCE(created_by, ''), COALESCE(decided_by, ''),
		       created_at, updated_at
		FROM inbox_entries
		WHERE ticket_id = ?
	`
	args := []any{ticketID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY inbox_id DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]InboxEntry, 0)
	for rows.Next() {
		entry, scanErr := scanInboxEntry(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func CreateFailureEscalationInboxEntry(ctx context.Context, db *sql.DB, ticket Ticket, message, createdBy string) (InboxEntry, error) {
	if strings.TrimSpace(strings.ToLower(ticket.State)) != StateFail {
		return InboxEntry{}, errors.New("ticket is not in fail state")
	}
	existing, err := ListInboxEntriesByTicket(ctx, db, ticket.ID, InboxStatusOpen)
	if err != nil {
		return InboxEntry{}, err
	}
	for _, entry := range existing {
		if entry.Kind == InboxKindFailureEscalation {
			return entry, nil
		}
	}
	recommendations := defaultFailureEscalationRecommendations()
	recommendationsJSON, err := encodeRecommendations(recommendations)
	if err != nil {
		return InboxEntry{}, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO inbox_entries (
			project_id, ticket_id, kind, status, recommendations_json, decision, message, created_by, decided_by, updated_at
		)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, NULL, CURRENT_TIMESTAMP)
	`, ticket.ProjectID, ticket.ID, InboxKindFailureEscalation, InboxStatusOpen, recommendationsJSON, strings.TrimSpace(message), nullableUserID(createdBy))
	if err != nil {
		return InboxEntry{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return InboxEntry{}, err
	}
	return GetInboxEntry(ctx, db, id)
}

func EnsureFailureEscalationInboxEntry(ctx context.Context, db *sql.DB, ticket Ticket, message, createdBy string) (InboxEntry, error) {
	return CreateFailureEscalationInboxEntry(ctx, db, ticket, message, createdBy)
}

func DecideInboxEntry(ctx context.Context, db *sql.DB, inboxID int64, decision, message, decidedBy string) (InboxEntry, error) {
	decision = strings.TrimSpace(strings.ToLower(decision))
	switch decision {
	case InboxDecisionStartAgain, InboxDecisionRefineRequirements:
	default:
		return InboxEntry{}, fmt.Errorf("invalid decision %q", decision)
	}
	result, err := db.ExecContext(ctx, `
		UPDATE inbox_entries
		SET status = ?, decision = ?, message = ?, decided_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE inbox_id = ?
	`, InboxStatusResolved, decision, strings.TrimSpace(message), nullableUserID(decidedBy), inboxID)
	if err != nil {
		return InboxEntry{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return InboxEntry{}, err
	}
	if affected == 0 {
		return InboxEntry{}, sql.ErrNoRows
	}
	return GetInboxEntry(ctx, db, inboxID)
}
