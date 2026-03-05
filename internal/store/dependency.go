package store

import (
	"database/sql"
	"errors"
)

type Dependency struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	TicketID  int64  `json:"ticket_id"`
	DependsOn int64  `json:"depends_on"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

func AddDependency(db *sql.DB, projectID, taskID, dependsOn, createdBy int64) (Dependency, error) {
	result, err := db.Exec(`
		INSERT INTO dependencies (project_id, task_id, depends_on, created_by)
		VALUES (?, ?, ?, ?)
	`, projectID, taskID, dependsOn, nullableUserID(createdBy))
	if err != nil {
		return Dependency{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Dependency{}, err
	}
	row := db.QueryRow(`
		SELECT id, project_id, task_id, depends_on, COALESCE(created_by, 0), created_at
		FROM dependencies
		WHERE id = ?
	`, id)
	var dependency Dependency
	if err := row.Scan(&dependency.ID, &dependency.ProjectID, &dependency.TicketID, &dependency.DependsOn, &dependency.CreatedBy, &dependency.CreatedAt); err != nil {
		return Dependency{}, err
	}
	return dependency, nil
}

func ListDependencies(db *sql.DB, taskID int64) ([]Dependency, error) {
	rows, err := db.Query(`
		SELECT id, project_id, task_id, depends_on, COALESCE(created_by, 0), created_at
		FROM dependencies
		WHERE task_id = ?
		ORDER BY id
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dependencies []Dependency
	for rows.Next() {
		var dependency Dependency
		if err := rows.Scan(&dependency.ID, &dependency.ProjectID, &dependency.TicketID, &dependency.DependsOn, &dependency.CreatedBy, &dependency.CreatedAt); err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, rows.Err()
}

func DeleteDependency(db *sql.DB, projectID, taskID, dependsOn int64) error {
	result, err := db.Exec(`
		DELETE FROM dependencies
		WHERE project_id = ? AND task_id = ? AND depends_on = ?
	`, projectID, taskID, dependsOn)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

var ErrDependencyNotFound = errors.New("dependency not found")
