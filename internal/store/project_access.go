package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrProjectAccessRequestNotFound = errors.New("project access request not found")

type ProjectAccessRequest struct {
	ID        int64  `json:"request_id"`
	ProjectID int64  `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ensureProjectAccessTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS project_access_policies (
	project_id INTEGER PRIMARY KEY,
	accepts_new_members INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS project_access_requests (
	request_id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	user_id TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(project_id, user_id, status),
	FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE,
	FOREIGN KEY(user_id) REFERENCES users(user_id) ON DELETE CASCADE
);`); err != nil {
		return err
	}
	return nil
}

func AcceptsNewMembers(ctx context.Context, db *sql.DB, projectID int64) (bool, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return false, err
	}
	var value int
	err := db.QueryRowContext(ctx, `SELECT accepts_new_members FROM project_access_policies WHERE project_id = ?`, projectID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return value == 1, err
}

func SetProjectAcceptsNewMembers(ctx context.Context, db *sql.DB, projectID int64, enabled bool) error {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return err
	}
	value := 0
	if enabled {
		value = 1
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO project_access_policies (project_id, accepts_new_members) VALUES (?, ?)
		ON CONFLICT(project_id) DO UPDATE SET accepts_new_members = excluded.accepts_new_members
	`, projectID, value)
	return err
}

func CreateProjectAccessRequest(ctx context.Context, db *sql.DB, projectID int64, userID, message string) (ProjectAccessRequest, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return ProjectAccessRequest{}, err
	}
	if _, err := GetProjectByID(ctx, db, projectID); err != nil {
		return ProjectAccessRequest{}, err
	}
	if _, err := GetUserByID(ctx, db, userID); err != nil {
		return ProjectAccessRequest{}, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO project_access_requests (project_id, user_id, message, status)
		VALUES (?, ?, ?, 'pending')
	`, projectID, userID, strings.TrimSpace(message))
	if err != nil {
		return ProjectAccessRequest{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return ProjectAccessRequest{}, err
	}
	return GetProjectAccessRequestByID(ctx, db, id)
}

func GetProjectAccessRequestByID(ctx context.Context, db *sql.DB, id int64) (ProjectAccessRequest, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return ProjectAccessRequest{}, err
	}
	var req ProjectAccessRequest
	err := db.QueryRowContext(ctx, `
		SELECT r.request_id, r.project_id, r.user_id, u.username, r.message, r.status, r.created_at, r.updated_at
		FROM project_access_requests r
		JOIN users u ON u.user_id = r.user_id
		WHERE r.request_id = ?
	`, id).Scan(&req.ID, &req.ProjectID, &req.UserID, &req.Username, &req.Message, &req.Status, &req.CreatedAt, &req.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectAccessRequest{}, ErrProjectAccessRequestNotFound
	}
	return req, err
}

func ListProjectAccessRequests(ctx context.Context, db *sql.DB, projectID int64, status string) ([]ProjectAccessRequest, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return nil, err
	}
	query := `
		SELECT r.request_id, r.project_id, r.user_id, u.username, r.message, r.status, r.created_at, r.updated_at
		FROM project_access_requests r
		JOIN users u ON u.user_id = r.user_id
		WHERE r.project_id = ?
	`
	args := []any{projectID}
	if strings.TrimSpace(status) != "" {
		query += ` AND r.status = ?`
		args = append(args, strings.TrimSpace(status))
	}
	query += ` ORDER BY r.created_at DESC, r.request_id DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []ProjectAccessRequest
	for rows.Next() {
		var req ProjectAccessRequest
		if err := rows.Scan(&req.ID, &req.ProjectID, &req.UserID, &req.Username, &req.Message, &req.Status, &req.CreatedAt, &req.UpdatedAt); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func SetProjectAccessRequestStatus(ctx context.Context, db *sql.DB, requestID int64, status string) (ProjectAccessRequest, error) {
	if err := ensureProjectAccessTables(ctx, db); err != nil {
		return ProjectAccessRequest{}, err
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "approved" && status != "rejected" {
		return ProjectAccessRequest{}, errors.New("invalid project access request status")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE project_access_requests
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE request_id = ?
	`, status, requestID)
	if err != nil {
		return ProjectAccessRequest{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ProjectAccessRequest{}, err
	}
	if affected == 0 {
		return ProjectAccessRequest{}, ErrProjectAccessRequestNotFound
	}
	req, err := GetProjectAccessRequestByID(ctx, db, requestID)
	if err != nil {
		return ProjectAccessRequest{}, err
	}
	if status == "approved" {
		if _, err := AddProjectMember(ctx, db, req.ProjectID, req.UserID, ProjectRoleObserver); err != nil {
			return ProjectAccessRequest{}, err
		}
	}
	return req, nil
}
