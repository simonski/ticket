package store

import (
	"database/sql"
	"errors"
	"strings"
)

type Goal struct {
	ID          int64  `json:"goal_id"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
	ETA         string `json:"eta"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func CreateGoal(db *sql.DB, projectID int64, title, description, notes, eta string, priority int) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("goal title is required")
	}
	if priority == 0 {
		priority = 1
	}
	result, err := db.Exec(`
		INSERT INTO goals (project_id, title, description, notes, eta, priority, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, projectID, title, strings.TrimSpace(description), strings.TrimSpace(notes), strings.TrimSpace(eta), priority)
	if err != nil {
		return Goal{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Goal{}, err
	}
	return GetGoal(db, id)
}

func GetGoal(db *sql.DB, id int64) (Goal, error) {
	row := db.QueryRow(`
		SELECT goal_id, project_id, title, description, notes, eta, priority, created_at, updated_at
		FROM goals WHERE goal_id = ?
	`, id)
	var g Goal
	if err := row.Scan(&g.ID, &g.ProjectID, &g.Title, &g.Description, &g.Notes, &g.ETA, &g.Priority, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return Goal{}, err
	}
	return g, nil
}

func ListGoals(db *sql.DB, projectID int64) ([]Goal, error) {
	rows, err := db.Query(`
		SELECT goal_id, project_id, title, description, notes, eta, priority, created_at, updated_at
		FROM goals WHERE project_id = ? ORDER BY priority, created_at
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var goals []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Title, &g.Description, &g.Notes, &g.ETA, &g.Priority, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func DeleteGoal(db *sql.DB, id int64) error {
	// Unlink any tickets from this goal
	if _, err := db.Exec(`UPDATE tickets SET goal_id = NULL WHERE goal_id = ?`, id); err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM goals WHERE goal_id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("goal not found")
	}
	return nil
}
