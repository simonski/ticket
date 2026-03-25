package store

import (
	"database/sql"
	"encoding/json"
)

type HistoryEvent struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	TicketID  int64  `json:"ticket_id"`
	TicketKey string `json:"ticket_key,omitempty"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type TicketHistoryEvent = HistoryEvent

type Comment struct {
	ID        int64  `json:"-"`
	ItemID    int64  `json:"-"`
	UserID    string `json:"-"`
	Author    string `json:"author"`
	Comment   string `json:"-"`
	Text      string `json:"text"`
	CreatedAt string `json:"date"`
}

func AddHistoryEvent(db *sql.DB, projectID, ticketID int64, eventType string, payload any, createdBy string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO history_events (project_id, ticket_id, event_type, payload, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, ticketID, eventType, string(data), nullableUserID(createdBy))
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO ticket_history (project_id, ticket_id, event_type, payload, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, ticketID, eventType, string(data), nullableUserID(createdBy))
	return err
}

func ListHistoryEvents(db *sql.DB, ticketID int64) ([]HistoryEvent, error) {
	rows, err := db.Query(`
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

// ListProjectHistory returns the most recent history events for all tickets in
// a project, ordered newest first, limited to limit rows.
func ListProjectHistory(db *sql.DB, projectID int64, limit int) ([]HistoryEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.Query(`
		SELECT h.id, h.project_id, h.ticket_id, COALESCE(t.key, ''), h.event_type, h.payload, COALESCE(h.created_by, ''), h.created_at
		FROM ticket_history h
		LEFT JOIN tickets t ON t.ticket_id = h.ticket_id
		WHERE h.project_id = ?
		ORDER BY h.id DESC
		LIMIT ?
	`, projectID, limit)
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

func AddComment(db *sql.DB, ticketID int64, userID string, comment string) (Comment, error) {
	ticket, err := GetTicket(db, ticketID)
	if err != nil {
		return Comment{}, err
	}
	if !ticket.Open {
		return Comment{}, ErrTicketClosed
	}
	if ticket.Archived {
		return Comment{}, ErrTicketClosed
	}
	result, err := db.Exec(`
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
	row := db.QueryRow(`
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

func ListComments(db *sql.DB, ticketID int64) ([]Comment, error) {
	rows, err := db.Query(`
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
