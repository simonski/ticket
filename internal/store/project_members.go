package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	ProjectRoleViewer = "viewer"
	ProjectRoleEditor = "editor"
	ProjectRoleOwner  = "owner"
)

var ErrProjectMembershipNotFound = errors.New("project membership not found")

type ProjectMember struct {
	ProjectID int64  `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
}

func normalizeProjectRole(role string) string {
	return strings.TrimSpace(strings.ToLower(role))
}

func validProjectRole(role string) bool {
	switch normalizeProjectRole(role) {
	case ProjectRoleViewer, ProjectRoleEditor, ProjectRoleOwner:
		return true
	default:
		return false
	}
}

func AddProjectMember(ctx context.Context, db *sql.DB, projectID int64, userID, role string) (ProjectMember, error) {
	role = normalizeProjectRole(role)
	if !validProjectRole(role) {
		return ProjectMember{}, fmt.Errorf("invalid role %q", role)
	}
	if _, err := GetProjectByID(ctx, db, projectID); err != nil {
		return ProjectMember{}, err
	}
	if _, err := GetUserByID(ctx, db, userID); err != nil {
		return ProjectMember{}, err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)
		ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role
	`, projectID, userID, role); err != nil {
		return ProjectMember{}, err
	}
	return GetProjectMember(ctx, db, projectID, userID)
}

func RemoveProjectMember(ctx context.Context, db *sql.DB, projectID int64, userID string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProjectMembershipNotFound
	}
	return nil
}

func GetProjectMember(ctx context.Context, db *sql.DB, projectID int64, userID string) (ProjectMember, error) {
	row := db.QueryRowContext(ctx, `
		SELECT pm.project_id, pm.user_id, u.username, pm.role
		FROM project_members pm
		JOIN users u ON u.user_id = pm.user_id
		WHERE pm.project_id = ? AND pm.user_id = ?
	`, projectID, userID)
	var member ProjectMember
	if err := row.Scan(&member.ProjectID, &member.UserID, &member.Username, &member.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectMember{}, ErrProjectMembershipNotFound
		}
		return ProjectMember{}, err
	}
	return member, nil
}

func ListProjectMembers(ctx context.Context, db *sql.DB, projectID int64) ([]ProjectMember, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT pm.project_id, pm.user_id, u.username, pm.role
		FROM project_members pm
		JOIN users u ON u.user_id = pm.user_id
		WHERE pm.project_id = ?
		ORDER BY u.username
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []ProjectMember
	for rows.Next() {
		var member ProjectMember
		if err := rows.Scan(&member.ProjectID, &member.UserID, &member.Username, &member.Role); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func ProjectRoleForUser(ctx context.Context, db *sql.DB, projectID int64, userID string) (role string, found bool, err error) {
	row := db.QueryRowContext(ctx, `SELECT role FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	if err = row.Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return normalizeProjectRole(role), true, nil
}
