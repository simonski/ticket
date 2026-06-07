package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrProgrammeNotFound = errors.New("programme not found")

type Programme struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func CreateProgramme(ctx context.Context, db *sql.DB, name, description string) (Programme, error) {
	res, err := db.ExecContext(ctx, `
		INSERT INTO programmes (name, description) VALUES (?, ?)
	`, strings.TrimSpace(name), strings.TrimSpace(description))
	if err != nil {
		return Programme{}, err
	}
	id, _ := res.LastInsertId()
	return GetProgramme(ctx, db, id)
}

func GetProgramme(ctx context.Context, db *sql.DB, id int64) (Programme, error) {
	row := db.QueryRowContext(ctx, `SELECT id, name, description, created_at, updated_at FROM programmes WHERE id = ?`, id)
	var p Programme
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Programme{}, ErrProgrammeNotFound
		}
		return Programme{}, err
	}
	return p, nil
}

func ListProgrammes(ctx context.Context, db *sql.DB) ([]Programme, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, description, created_at, updated_at FROM programmes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Programme
	for rows.Next() {
		var p Programme
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func UpdateProgramme(ctx context.Context, db *sql.DB, id int64, name, description string) (Programme, error) {
	_, err := db.ExecContext(ctx, `UPDATE programmes SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		strings.TrimSpace(name), strings.TrimSpace(description), id)
	if err != nil {
		return Programme{}, err
	}
	return GetProgramme(ctx, db, id)
}

func DeleteProgramme(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := db.ExecContext(ctx, `UPDATE projects SET programme_id = NULL WHERE programme_id = ?`, id); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `DELETE FROM programmes WHERE id = ?`, id)
	return err
}

func SetProjectProgramme(ctx context.Context, db *sql.DB, projectID int64, programmeID *int64) error {
	_, err := db.ExecContext(ctx, `UPDATE projects SET programme_id = ? WHERE project_id = ?`, programmeID, projectID)
	return err
}
