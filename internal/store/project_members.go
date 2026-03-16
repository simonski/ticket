package store

import (
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
	UserID    int64  `json:"user_id"`
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

func AddProjectMember(db *sql.DB, projectID, userID int64, role string) (ProjectMember, error) {
	role = normalizeProjectRole(role)
	if !validProjectRole(role) {
		return ProjectMember{}, fmt.Errorf("invalid role %q", role)
	}
	if _, err := GetProjectByID(db, projectID); err != nil {
		return ProjectMember{}, err
	}
	if _, err := GetUserByID(db, userID); err != nil {
		return ProjectMember{}, err
	}
	if _, err := db.Exec(`
		INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)
		ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role
	`, projectID, userID, role); err != nil {
		return ProjectMember{}, err
	}
	return GetProjectMember(db, projectID, userID)
}

func RemoveProjectMember(db *sql.DB, projectID, userID int64) error {
	result, err := db.Exec(`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
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

func GetProjectMember(db *sql.DB, projectID, userID int64) (ProjectMember, error) {
	row := db.QueryRow(`
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

func ListProjectMembers(db *sql.DB, projectID int64) ([]ProjectMember, error) {
	rows, err := db.Query(`
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

func ProjectRoleForUser(db *sql.DB, projectID, userID int64) (string, bool, error) {
	row := db.QueryRow(`SELECT role FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	var role string
	if err := row.Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return normalizeProjectRole(role), true, nil
}
