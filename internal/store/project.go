package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrProjectNotFound = errors.New("project not found")

const (
	ProjectVisibilityPrivate = "private"
	ProjectVisibilityPublic  = "public"
)

type Project struct {
	ID                 int64  `json:"project_id"`
	Prefix             string `json:"prefix"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	GitRepository      string `json:"git_repository"`
	GitBranch          string `json:"git_branch"`
	Notes              string `json:"notes"`
	Status             string `json:"status"`
	Visibility         string `json:"visibility"`
	CreatedBy          int64  `json:"created_by"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	WorkflowID         *int64 `json:"workflow_id,omitempty"`
}

type ProjectCreateParams struct {
	Prefix             string
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	Notes              string
	Visibility         string
	CreatedBy          int64
	WorkflowID         *int64
}

type ProjectUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	Notes              string
	Visibility         string
	WorkflowID         *int64
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
	visibility := normalizeProjectVisibility(params.Visibility)
	if visibility == "" {
		visibility = ProjectVisibilityPublic
	}
	if !validProjectVisibility(visibility) {
		return Project{}, fmt.Errorf("invalid project visibility %q", params.Visibility)
	}
	uniquePrefix, err := nextUniqueProjectPrefix(db, prefix)
	if err != nil {
		return Project{}, err
	}
	// Default to the "default" workflow if none specified
	workflowID := params.WorkflowID
	if workflowID == nil {
		var wfID int64
		if err := db.QueryRow(`SELECT workflow_id FROM workflows WHERE name = 'default'`).Scan(&wfID); err == nil {
			workflowID = &wfID
		}
	}
	result, err := db.Exec(`
		INSERT INTO projects (prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, created_by, workflow_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?)
	`, uniquePrefix, title, strings.TrimSpace(params.Description), strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.GitBranch), strings.TrimSpace(params.Notes), visibility, params.CreatedBy, workflowID)
	if err != nil {
		return Project{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	if params.CreatedBy > 0 {
		if _, err := AddProjectMember(db, id, params.CreatedBy, ProjectRoleOwner); err != nil {
			return Project{}, err
		}
	}
	return GetProjectByID(db, id)
}

func ListProjects(db *sql.DB) ([]Project, error) {
	rows, err := db.Query(`
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, 0), created_at, updated_at, workflow_id
		FROM projects
		ORDER BY created_at, project_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.GitRepository, &project.GitBranch, &project.Notes, &project.Status, &project.Visibility, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.WorkflowID); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func ListProjectsVisibleToUser(db *sql.DB, user User) ([]Project, error) {
	if user.Role == "admin" {
		return ListProjects(db)
	}
	rows, err := db.Query(`
		WITH RECURSIVE team_scope(team_id, parent_team_id) AS (
			SELECT t.team_id, t.parent_team_id
			FROM teams t
			JOIN team_members tm ON tm.team_id = t.team_id
			WHERE tm.user_id = ?
			UNION
			SELECT parent.team_id, parent.parent_team_id
			FROM teams parent
			JOIN team_scope ts ON ts.parent_team_id = parent.team_id
		)
		SELECT DISTINCT p.project_id, p.prefix, p.title, p.description, p.acceptance_criteria, p.git_repository, p.git_branch, p.notes, p.status, p.visibility, COALESCE(p.created_by, 0), p.created_at, p.updated_at, p.workflow_id
		FROM projects p
		LEFT JOIN project_members pm ON pm.project_id = p.project_id AND pm.user_id = ?
		LEFT JOIN project_teams pt ON pt.project_id = p.project_id
		LEFT JOIN team_scope ts ON ts.team_id = pt.team_id
		WHERE p.visibility = ? OR pm.user_id IS NOT NULL OR ts.team_id IS NOT NULL
		ORDER BY p.created_at, p.project_id
	`, user.ID, user.ID, ProjectVisibilityPublic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.GitRepository, &project.GitBranch, &project.Notes, &project.Status, &project.Visibility, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.WorkflowID); err != nil {
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
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, 0), created_at, updated_at, workflow_id
		FROM projects
		WHERE prefix = ?
	`, strings.ToUpper(rawID))
	var project Project
	if err := row.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.GitRepository, &project.GitBranch, &project.Notes, &project.Status, &project.Visibility, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.WorkflowID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, err
	}
	return project, nil
}

func GetProjectByID(db *sql.DB, id int64) (Project, error) {
	row := db.QueryRow(`
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, 0), created_at, updated_at, workflow_id
		FROM projects
		WHERE project_id = ?
	`, id)
	var project Project
	if err := row.Scan(&project.ID, &project.Prefix, &project.Title, &project.Description, &project.AcceptanceCriteria, &project.GitRepository, &project.GitBranch, &project.Notes, &project.Status, &project.Visibility, &project.CreatedBy, &project.CreatedAt, &project.UpdatedAt, &project.WorkflowID); err != nil {
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
	nextDescription := params.Description
	if strings.TrimSpace(nextDescription) == "" {
		nextDescription = current.Description
	}
	nextAC := params.AcceptanceCriteria
	if strings.TrimSpace(nextAC) == "" {
		nextAC = current.AcceptanceCriteria
	}
	nextRepo := strings.TrimSpace(params.GitRepository)
	if nextRepo == "" {
		nextRepo = current.GitRepository
	}
	nextBranch := strings.TrimSpace(params.GitBranch)
	if nextBranch == "" {
		nextBranch = current.GitBranch
	}
	nextNotes := strings.TrimSpace(params.Notes)
	if nextNotes == "" {
		nextNotes = current.Notes
	}
	nextVisibility := normalizeProjectVisibility(params.Visibility)
	if nextVisibility == "" {
		nextVisibility = current.Visibility
	}
	if !validProjectVisibility(nextVisibility) {
		return Project{}, fmt.Errorf("invalid project visibility %q", params.Visibility)
	}
	nextWorkflowID := params.WorkflowID
	if nextWorkflowID == nil {
		nextWorkflowID = current.WorkflowID
	}
	_, err = db.Exec(`
		UPDATE projects
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, git_branch = ?, notes = ?, visibility = ?, workflow_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, nextTitle, nextDescription, nextAC, nextRepo, nextBranch, nextNotes, nextVisibility, nextWorkflowID, id)
	if err != nil {
		return Project{}, err
	}
	return GetProjectByID(db, id)
}

func normalizeProjectVisibility(visibility string) string {
	return strings.TrimSpace(strings.ToLower(visibility))
}

func validProjectVisibility(visibility string) bool {
	switch normalizeProjectVisibility(visibility) {
	case ProjectVisibilityPrivate, ProjectVisibilityPublic:
		return true
	default:
		return false
	}
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
