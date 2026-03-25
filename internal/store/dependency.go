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
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

func AddDependency(db *sql.DB, projectID, ticketID, dependsOn int64, createdBy string) (Dependency, error) {
	ticket, err := GetTicket(db, ticketID)
	if err != nil {
		return Dependency{}, err
	}
	if !ticket.Open {
		return Dependency{}, ErrTicketClosed
	}
	if ticket.Archived {
		return Dependency{}, ErrTicketClosed
	}
	result, err := db.Exec(`
		INSERT INTO dependencies (project_id, ticket_id, depends_on, created_by)
		VALUES (?, ?, ?, ?)
	`, projectID, ticketID, dependsOn, nullableUserID(createdBy))
	if err != nil {
		return Dependency{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Dependency{}, err
	}
	row := db.QueryRow(`
		SELECT id, project_id, ticket_id, depends_on, COALESCE(created_by, ''), created_at
		FROM dependencies
		WHERE id = ?
	`, id)
	var dependency Dependency
	if err := row.Scan(&dependency.ID, &dependency.ProjectID, &dependency.TicketID, &dependency.DependsOn, &dependency.CreatedBy, &dependency.CreatedAt); err != nil {
		return Dependency{}, err
	}
	return dependency, nil
}

func ListDependencies(db *sql.DB, ticketID int64) ([]Dependency, error) {
	rows, err := db.Query(`
		SELECT id, project_id, ticket_id, depends_on, COALESCE(created_by, ''), created_at
		FROM dependencies
		WHERE ticket_id = ?
		ORDER BY id
	`, ticketID)
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

func DeleteDependency(db *sql.DB, projectID, ticketID, dependsOn int64) error {
	ticket, err := GetTicket(db, ticketID)
	if err != nil {
		return err
	}
	if !ticket.Open {
		return ErrTicketClosed
	}
	if ticket.Archived {
		return ErrTicketClosed
	}
	result, err := db.Exec(`
		DELETE FROM dependencies
		WHERE project_id = ? AND ticket_id = ? AND depends_on = ?
	`, projectID, ticketID, dependsOn)
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
