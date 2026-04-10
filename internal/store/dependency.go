package store

import (
	"context"
	"database/sql"
	"errors"
)

type Dependency struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	TicketID  string `json:"ticket_id"`
	DependsOn string `json:"depends_on"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

func AddDependency(ctx context.Context, db *sql.DB, projectID int64, ticketID, dependsOn string, createdBy string) (Dependency, error) {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return Dependency{}, err
	}
	if ticket.Complete {
		return Dependency{}, ErrTicketClosed
	}
	if ticket.Archived {
		return Dependency{}, ErrTicketClosed
	}
	result, err := db.ExecContext(ctx, `
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
	row := db.QueryRowContext(ctx, `
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

func ListDependencies(ctx context.Context, db *sql.DB, ticketID string) ([]Dependency, error) {
	rows, err := db.QueryContext(ctx, `
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

func DeleteDependency(ctx context.Context, db *sql.DB, projectID int64, ticketID, dependsOn string) error {
	ticket, err := GetTicket(ctx, db, ticketID)
	if err != nil {
		return err
	}
	if ticket.Complete {
		return ErrTicketClosed
	}
	if ticket.Archived {
		return ErrTicketClosed
	}
	result, err := db.ExecContext(ctx, `
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
