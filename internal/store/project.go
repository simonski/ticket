package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrProjectNotFound = errors.New("project not found")

type Project struct {
	ID                 int64  `json:"project_id"`
	Prefix             string `json:"prefix"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	Notes              string `json:"notes"`
	Status             string `json:"status"`
	CreatedBy          int64  `json:"created_by"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type ProjectCreateParams struct {
	Prefix             string
	Title              string
	Description        string
	AcceptanceCriteria string
	Notes              string
	CreatedBy          int64
}

type ProjectUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	Notes              string
}

func CreateProject(db *sql.DB, title, description, acceptanceCriteria string, createdBy int64) (Project, error) {
	return CreateProjectWithParams(db, ProjectCreateParams{
		Prefix:             deriveProjectPrefix(title),
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
		CreatedBy:          createdBy,
	})
}

func CreateProjectWithParams(db *sql.DB, params ProjectCreateParams) (Project, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Project{}, errors.New("project title is required")
	}
	prefix := normalizeProjectPrefix(params.Prefix)
	if prefix == "" {
		prefix = deriveProjectPrefix(title)
	}
	if err := validateProjectPrefix(prefix); err != nil {
		return Project{}, err
	}
	uniquePrefix, err := nextUniqueProjectPrefix(db, prefix)
	if err != nil {
		return Project{}, err
	}
	result, err := db.Exec(`
		INSERT INTO projects (prefix, title, description, acceptance_criteria, notes, status, created_by)
		VALUES (?, ?, ?, ?, ?, 'open', ?)
	`, uniquePrefix, title, strings.TrimSpace(params.Description), strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.Notes), params.CreatedBy)
	if err != nil {
		return Project{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	return GetProjectByID(db, id)
}

func ListProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(`
		SELECT project_id, prefix, title, description, acceptance_criteria, notes, status, COALESCE(created_by, 0), created_at, updated_at
		FROM projects
		ORDER BY created_at, project_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.Notes, &project.Status, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func GetProject(db *sql.DB, rawID string) (Project, error) {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return Project{}, ErrProjectNotFound
	}
	var id int64
	if _, err := fmt.Sscan(rawID, &id); err == nil {
		return GetProjectByID(db, id)
	}
	row := db.QueryRow(`
		SELECT project_id, prefix, title, description, acceptance_criteria, notes, status, COALESCE(created_by, 0), created_at, updated_at
		FROM projects
		WHERE prefix = ?
	`, strings.ToUpper(rawID))
	var project Project
	if err := row.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.Notes, &project.Status, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, err
	}
	return project, nil
}

func GetProjectByID(db *sql.DB, id int64) (Project, error) {
	row := db.QueryRow(`
		SELECT project_id, prefix, title, description, acceptance_criteria, notes, status, COALESCE(created_by, 0), created_at, updated_at
		FROM projects
		WHERE project_id = ?
	`, id)
	var project Project
	if err := row.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.Notes, &project.Status, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, err
	}
	return project, nil
}

func UpdateProject(db *sql.DB, id int64, title, description, acceptanceCriteria string) (Project, error) {
	return UpdateProjectWithParams(db, id, ProjectUpdateParams{
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
	})
}

func UpdateProjectWithParams(db *sql.DB, id int64, params ProjectUpdateParams) (Project, error) {
	current, err := GetProjectByID(db, id)
	if err != nil {
		return Project{}, err
	}
	nextTitle := strings.TrimSpace(params.Title)
	if nextTitle == "" {
		nextTitle = current.Title
	}
	_, err = db.Exec(`
		UPDATE projects
		SET title = ?, description = ?, acceptance_criteria = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, nextTitle, params.Description, params.AcceptanceCriteria, strings.TrimSpace(params.Notes), id)
	if err != nil {
		return Project{}, err
	}
	return GetProjectByID(db, id)
}

func SetProjectStatus(db *sql.DB, id int64, enabled bool) (Project, error) {
	status := "closed"
	if enabled {
		status = "open"
	}
	result, err := db.Exec(`UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, status, id)
	if err != nil {
		return Project{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Project{}, err
	}
	if affected == 0 {
		return Project{}, ErrProjectNotFound
	}
	return GetProjectByID(db, id)
}
