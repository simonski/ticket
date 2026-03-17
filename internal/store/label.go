package store

import (
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

func CreateLabel(db *sql.DB, projectID int64, name, color string) (Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Label{}, errors.New("label name is required")
	}
	color = strings.TrimSpace(color)
	result, err := db.Exec(`INSERT INTO labels (project_id, name, color) VALUES (?, ?, ?)`, projectID, name, color)
	if err != nil {
		return Label{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Label{}, err
	}
	return GetLabel(db, id)
}

func GetLabel(db *sql.DB, id int64) (Label, error) {
	var label Label
	err := db.QueryRow(`SELECT label_id, project_id, name, color, created_at FROM labels WHERE label_id = ?`, id).
		Scan(&label.ID, &label.ProjectID, &label.Name, &label.Color, &label.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Label{}, ErrLabelNotFound
	}
	return label, err
}

func ListLabels(db *sql.DB, projectID int64) ([]Label, error) {
	rows, err := db.Query(`SELECT label_id, project_id, name, color, created_at FROM labels WHERE project_id = ? ORDER BY name`, projectID)
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

func DeleteLabel(db *sql.DB, id int64) error {
	if _, err := db.Exec(`DELETE FROM ticket_labels WHERE label_id = ?`, id); err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM labels WHERE label_id = ?`, id)
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

func AddTicketLabel(db *sql.DB, ticketID, labelID int64) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO ticket_labels (ticket_id, label_id) VALUES (?, ?)`, ticketID, labelID)
	return err
}

func RemoveTicketLabel(db *sql.DB, ticketID, labelID int64) error {
	_, err := db.Exec(`DELETE FROM ticket_labels WHERE ticket_id = ? AND label_id = ?`, ticketID, labelID)
	return err
}

func ListTicketLabels(db *sql.DB, ticketID int64) ([]Label, error) {
	rows, err := db.Query(`
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

func ListTicketsByLabel(db *sql.DB, labelID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT ticket_id FROM ticket_labels WHERE label_id = ? ORDER BY ticket_id`, labelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
