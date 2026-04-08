package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrLabelNotFound = errors.New("label not found")

type Label struct {
	ID        int64  `json:"label_id"`
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
}

func CreateLabel(ctx context.Context, db *sql.DB, projectID int64, name, color string) (Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Label{}, errors.New("label name is required")
	}
	color = strings.TrimSpace(color)
	result, err := db.ExecContext(ctx, `INSERT INTO labels (project_id, name, color) VALUES (?, ?, ?)`, projectID, name, color)
	if err != nil {
		return Label{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Label{}, err
	}
	return GetLabel(ctx, db, id)
}

func GetLabel(ctx context.Context, db *sql.DB, id int64) (Label, error) {
	var label Label
	err := db.QueryRowContext(ctx, `SELECT label_id, project_id, name, color, created_at FROM labels WHERE label_id = ?`, id).
		Scan(&label.ID, &label.ProjectID, &label.Name, &label.Color, &label.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Label{}, ErrLabelNotFound
	}
	return label, err
}

func ListLabels(ctx context.Context, db *sql.DB, projectID int64) ([]Label, error) {
	rows, err := db.QueryContext(ctx, `SELECT label_id, project_id, name, color, created_at FROM labels WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := make([]Label, 0)
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.ProjectID, &label.Name, &label.Color, &label.CreatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func DeleteLabel(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM ticket_labels WHERE label_id = ?`, id); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM labels WHERE label_id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrLabelNotFound
	}
	return nil
}

func AddTicketLabel(ctx context.Context, db *sql.DB, ticketID string, labelID int64) error {
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO ticket_labels (ticket_id, label_id) VALUES (?, ?)`, ticketID, labelID)
	return err
}

func RemoveTicketLabel(ctx context.Context, db *sql.DB, ticketID string, labelID int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM ticket_labels WHERE ticket_id = ? AND label_id = ?`, ticketID, labelID)
	return err
}

func ListTicketLabels(ctx context.Context, db *sql.DB, ticketID string) ([]Label, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.label_id, l.project_id, l.name, l.color, l.created_at
		FROM labels l
		JOIN ticket_labels tl ON tl.label_id = l.label_id
		WHERE tl.ticket_id = ?
		ORDER BY l.name
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := make([]Label, 0)
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.ProjectID, &label.Name, &label.Color, &label.CreatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func ListTicketsByLabel(ctx context.Context, db *sql.DB, labelID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT ticket_id FROM ticket_labels WHERE label_id = ? ORDER BY ticket_id`, labelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
