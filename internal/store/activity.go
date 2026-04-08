package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type HistoryEvent struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	TicketID  string `json:"ticket_id"`
	TicketKey string `json:"ticket_key,omitempty"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type TicketHistoryEvent = HistoryEvent

type Comment struct {
	ID        int64  `json:"-"`
	ItemID    string `json:"-"`
	UserID    string `json:"-"`
	Author    string `json:"author"`
	Comment   string `json:"-"`
	Text      string `json:"text"`
	CreatedAt string `json:"date"`
}

func AddHistoryEvent(ctx context.Context, db *sql.DB, projectID int64, ticketID string, eventType string, payload any, createdBy string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO ticket_history (project_id, ticket_id, event_type, payload, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, ticketID, eventType, string(data), nullableUserID(createdBy))
	return err
}

func ListHistoryEvents(ctx context.Context, db *sql.DB, ticketID string) ([]HistoryEvent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, project_id, ticket_id, event_type, payload, COALESCE(created_by, ''), created_at
		FROM ticket_history
		WHERE ticket_id = ?
		ORDER BY id
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []HistoryEvent
	for rows.Next() {
		var event HistoryEvent
		if err := rows.Scan(&event.ID, &event.ProjectID, &event.TicketID, &event.EventType, &event.Payload, &event.CreatedBy, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// HistoryFilter holds optional filter criteria for history queries.
type HistoryFilter struct {
	UserID  string // filter by exact user_id (created_by)
	AgentID string // filter by agent user_id (created_by)
	TeamID  int64  // filter by team membership (team_members)
}

// ListProjectHistory returns the most recent history events for all tickets in
// a project, ordered newest first, limited to limit rows.
func ListProjectHistory(ctx context.Context, db *sql.DB, projectID int64, limit int) ([]HistoryEvent, error) {
	return ListProjectHistoryFiltered(ctx, db, projectID, limit, HistoryFilter{})
}

// ListProjectHistoryFiltered returns the most recent history events for all
// tickets in a project, applying optional actor filters.
func ListProjectHistoryFiltered(ctx context.Context, db *sql.DB, projectID int64, limit int, filter HistoryFilter) ([]HistoryEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	var clauses []string
	var args []any

	clauses = append(clauses, "h.project_id = ?")
	args = append(args, projectID)

	if filter.UserID != "" {
		clauses = append(clauses, "h.created_by = ?")
		args = append(args, filter.UserID)
	}
	if filter.AgentID != "" {
		clauses = append(clauses, "h.created_by = ?")
		args = append(args, filter.AgentID)
	}
	if filter.TeamID > 0 {
		clauses = append(clauses, "h.created_by IN (SELECT user_id FROM team_members WHERE team_id = ?)")
		args = append(args, filter.TeamID)
	}

	query := fmt.Sprintf( // #nosec G201 -- clauses are built from hardcoded predicates, not raw user input
		`SELECT h.id, h.project_id, h.ticket_id, COALESCE(h.ticket_id, ''), h.event_type, h.payload, COALESCE(h.created_by, ''), h.created_at
		FROM ticket_history h
		WHERE %s
		ORDER BY h.id DESC
		LIMIT ?
	`, strings.Join(clauses, " AND "))
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []HistoryEvent
	for rows.Next() {
		var event HistoryEvent
		if err := rows.Scan(&event.ID, &event.ProjectID, &event.TicketID, &event.TicketKey, &event.EventType, &event.Payload, &event.CreatedBy, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func AddComment(ctx context.Context, db *sql.DB, ticketID string, userID string, comment string) (Comment, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Comment{}, err
	}
	if !ticket.Open {
		return Comment{}, ErrTicketClosed
	}
	if ticket.Archived {
		return Comment{}, ErrTicketClosed
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO comments (item_id, user_id, comment)
		VALUES (?, ?, ?)
	`, ticketID, userID, comment)
	if err != nil {
		return Comment{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Comment{}, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT c.id, c.item_id, c.user_id, u.username, c.comment, c.created_at
		FROM comments c
		JOIN users u ON u.user_id = c.user_id
		WHERE c.id = ?
	`, id)
	var c Comment
	if err := row.Scan(&c.ID, &c.ItemID, &c.UserID, &c.Author, &c.Comment, &c.CreatedAt); err != nil {
		return Comment{}, err
	}
	c.Text = c.Comment
	return c, nil
}

func ListComments(ctx context.Context, db *sql.DB, ticketID string) ([]Comment, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, c.item_id, c.user_id, u.username, c.comment, c.created_at
		FROM comments c
		JOIN users u ON u.user_id = c.user_id
		WHERE c.item_id = ?
		ORDER BY c.created_at DESC, c.id DESC
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ItemID, &c.UserID, &c.Author, &c.Comment, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Text = c.Comment
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func nullableUserID(userID string) any {
	if userID == "" {
		return nil
	}
	return userID
}

// PurgeExpiredSessions deletes sessions whose expires_at is in the past.
// Returns the number of rows deleted.
func PurgeExpiredSessions(ctx context.Context, db *sql.DB) (int64, error) {
	result, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// PurgeOldHistory deletes ticket_history events older than retentionDays days.
// Returns the number of rows deleted.
func PurgeOldHistory(ctx context.Context, db *sql.DB, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	result, err := db.ExecContext(ctx, 
		`DELETE FROM ticket_history WHERE created_at <= datetime('now', ? || ' days')`,
		fmt.Sprintf("-%d", retentionDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

