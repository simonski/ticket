package store

import (
	"database/sql"
	"errors"
	"strings"
)

type Role struct {
	ID         int64  `json:"role_id"`
	Title      string `json:"title"`
	Motivation string `json:"motivation"`
	Goals      string `json:"goals"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func CreateRole(db *sql.DB, title, motivation, goals string) (Role, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	result, err := db.Exec(`
		INSERT INTO roles (title, motivation, goals, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, title, strings.TrimSpace(motivation), strings.TrimSpace(goals))
	if err != nil {
		return Role{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Role{}, err
	}
	return GetRoleByID(db, id)
}

func ListRoles(db *sql.DB) ([]Role, error) {
	rows, err := db.Query(`
		SELECT role_id, title, motivation, goals, created_at, updated_at
		FROM roles
		ORDER BY title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Title, &role.Motivation, &role.Goals, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func GetRoleByID(db *sql.DB, id int64) (Role, error) {
	row := db.QueryRow(`
		SELECT role_id, title, motivation, goals, created_at, updated_at
		FROM roles
		WHERE role_id = ?
	`, id)
	var role Role
	if err := row.Scan(&role.ID, &role.Title, &role.Motivation, &role.Goals, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	return role, nil
}

func GetRoleByTitle(db *sql.DB, title string) (Role, error) {
	row := db.QueryRow(`
		SELECT role_id, title, motivation, goals, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	var role Role
	if err := row.Scan(&role.ID, &role.Title, &role.Motivation, &role.Goals, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	return role, nil
}

func UpdateRole(db *sql.DB, id int64, title, motivation, goals string) (Role, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	result, err := db.Exec(`
		UPDATE roles
		SET title = ?, motivation = ?, goals = ?, updated_at = CURRENT_TIMESTAMP
		WHERE role_id = ?
	`, title, strings.TrimSpace(motivation), strings.TrimSpace(goals), id)
	if err != nil {
		return Role{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Role{}, err
	}
	if affected == 0 {
		return Role{}, sql.ErrNoRows
	}
	return GetRoleByID(db, id)
}

func DeleteRole(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM roles WHERE role_id = ?`, id)
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
