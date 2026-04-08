package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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
	CreatedBy          string `json:"created_by"`
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
	CreatedBy          string
	WorkflowID         *int64
}

type ProjectUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	Notes              string
	Status             string
	Visibility         string
	WorkflowID         *int64
}

func CreateProject(ctx context.Context, db *sql.DB, title, description, acceptanceCriteria string, createdBy string) (Project, error) {
	return CreateProjectWithParams(ctx, db, ProjectCreateParams{
		Prefix:             deriveProjectPrefix(title),
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
		CreatedBy:          createdBy,
	})
}

func CreateProjectWithParams(ctx context.Context, db *sql.DB, params ProjectCreateParams) (Project, error) {
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
	uniquePrefix, err := nextUniqueProjectPrefix(ctx, db, prefix)
	if err != nil {
		return Project{}, err
	}
	// Default to the "default" workflow if none specified
	workflowID := params.WorkflowID
	if workflowID == nil {
		var wfID int64
		if err := db.QueryRowContext(ctx, `SELECT workflow_id FROM workflows WHERE name = 'default'`).Scan(&wfID); err == nil {
			workflowID = &wfID
		}
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO projects (prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, created_by, workflow_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?)
	`, uniquePrefix, title, strings.TrimSpace(params.Description), strings.TrimSpace(params.AcceptanceCriteria), strings.TrimSpace(params.GitRepository), strings.TrimSpace(params.GitBranch), strings.TrimSpace(params.Notes), visibility, nullableUserID(params.CreatedBy), workflowID)
	if err != nil {
		return Project{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	if params.CreatedBy != "" {
		if _, err := AddProjectMember(ctx, db, id, params.CreatedBy, ProjectRoleOwner); err != nil {
			return Project{}, err
		}
	}
	return GetProjectByID(ctx, db, id)
}

func ListProjects(ctx context.Context, db *sql.DB) ([]Project, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, ''), created_at, updated_at, workflow_id
		FROM projects
		ORDER BY created_at, project_id
		LIMIT 1000
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

func ListProjectsVisibleToUser(ctx context.Context, db *sql.DB, user User) ([]Project, error) {
	if user.Role == "admin" {
		return ListProjects(ctx, db)
	}
	rows, err := db.QueryContext(ctx, `
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
		SELECT DISTINCT p.project_id, p.prefix, p.title, p.description, p.acceptance_criteria, p.git_repository, p.git_branch, p.notes, p.status, p.visibility, COALESCE(p.created_by, ''), p.created_at, p.updated_at, p.workflow_id
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

func GetProject(ctx context.Context, db *sql.DB, rawID string) (Project, error) {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return Project{}, ErrProjectNotFound
	}
	var id int64
	if _, err := fmt.Sscan(rawID, &id); err == nil {
		return GetProjectByID(ctx, db, id)
	}
	row := db.QueryRowContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, ''), created_at, updated_at, workflow_id
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

func GetProjectByID(ctx context.Context, db *sql.DB, id int64) (Project, error) {
	row := db.QueryRowContext(ctx, `
		SELECT project_id, prefix, title, description, acceptance_criteria, git_repository, git_branch, notes, status, visibility, COALESCE(created_by, ''), created_at, updated_at, workflow_id
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

func UpdateProject(ctx context.Context, db *sql.DB, id int64, title, description, acceptanceCriteria string) (Project, error) {
	return UpdateProjectWithParams(ctx, db, id, ProjectUpdateParams{
		Title:              title,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
	})
}

func UpdateProjectWithParams(ctx context.Context, db *sql.DB, id int64, params ProjectUpdateParams) (Project, error) {
	current, err := GetProjectByID(ctx, db, id)
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
	nextStatus := strings.TrimSpace(params.Status)
	if nextStatus == "" {
		nextStatus = current.Status
	}
	nextWorkflowID := params.WorkflowID
	if nextWorkflowID == nil {
		nextWorkflowID = current.WorkflowID
	}
	_, err = db.ExecContext(ctx, `
		UPDATE projects
		SET title = ?, description = ?, acceptance_criteria = ?, git_repository = ?, git_branch = ?, notes = ?, status = ?, visibility = ?, workflow_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, nextTitle, nextDescription, nextAC, nextRepo, nextBranch, nextNotes, nextStatus, nextVisibility, nextWorkflowID, id)
	if err != nil {
		return Project{}, err
	}
	return GetProjectByID(ctx, db, id)
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

func SetProjectStatus(ctx context.Context, db *sql.DB, id int64, enabled bool) (Project, error) {
	status := "closed"
	if enabled {
		status = "open"
	}
	result, err := db.ExecContext(ctx, `UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, status, id)
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
	return GetProjectByID(ctx, db, id)
}

// DeleteProject removes a project and all associated data.
func DeleteProject(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := GetProjectByID(ctx, db, id); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete child data that references tickets in this project
	if _, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE item_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM time_entries WHERE ticket_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_labels WHERE ticket_id IN (SELECT ticket_id FROM tickets WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dependencies WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM history_events WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ticket_history WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM story_ticket_links WHERE story_id IN (SELECT story_id FROM stories WHERE project_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM stories WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tickets WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_teams WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE project_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// RenameProjectPrefix changes a project's prefix and re-keys every ticket
// in that project. All foreign-key references (parent_id, clone_of,
// dependencies, comments, history, labels, time entries, story links) are
// updated in a single transaction.
func RenameProjectPrefix(ctx context.Context, db *sql.DB, projectID int64, newPrefix string) (int, error) {
	newPrefix = normalizeProjectPrefix(newPrefix)
	if err := validateProjectPrefix(newPrefix); err != nil {
		return 0, err
	}

	// Check the new prefix isn't already used by another project.
	var existingID int64
	err := db.QueryRowContext(ctx, `SELECT project_id FROM projects WHERE prefix = ?`, newPrefix).Scan(&existingID)
	if err == nil && existingID != projectID {
		return 0, fmt.Errorf("prefix %q is already used by another project", newPrefix)
	}

	// Load project to get current prefix.
	project, err := GetProjectByID(ctx, db, projectID)
	if err != nil {
		return 0, err
	}
	if project.Prefix == newPrefix {
		return 0, nil // nothing to do
	}

	// Load all tickets for this project and compute new keys.
	rows, err := db.QueryContext(ctx, `SELECT ticket_id, type FROM tickets WHERE project_id = ?`, projectID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type keyMapping struct {
		oldKey     string
		newKey     string
		ticketType string
	}
	var mappings []keyMapping
	for rows.Next() {
		var oldKey, ticketType string
		if err := rows.Scan(&oldKey, &ticketType); err != nil {
			return 0, err
		}
		// Extract the sequence number from the old key.
		seq := extractSequence(oldKey)
		if seq <= 0 {
			return 0, fmt.Errorf("could not extract sequence from key %q", oldKey)
		}
		newKey, err := generateTicketKey(newPrefix, ticketType, seq)
		if err != nil {
			return 0, fmt.Errorf("generating new key for %q: %w", oldKey, err)
		}
		mappings = append(mappings, keyMapping{oldKey: oldKey, newKey: newKey, ticketType: ticketType})
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// PRAGMA foreign_keys must be set outside a transaction in SQLite.
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return 0, err
	}
	defer db.ExecContext(ctx, `PRAGMA foreign_keys = ON`) //nolint:errcheck

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Update each ticket key and all references.
	for _, m := range mappings {
		// Primary key
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, fmt.Errorf("renaming %s → %s: %w", m.oldKey, m.newKey, err)
		}
		// Parent references
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET parent_id = ? WHERE parent_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Clone references
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET clone_of = ? WHERE clone_of = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Dependencies
		if _, err := tx.ExecContext(ctx, `UPDATE dependencies SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE dependencies SET depends_on = ? WHERE depends_on = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Story links
		if _, err := tx.ExecContext(ctx, `UPDATE story_ticket_links SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// History
		if _, err := tx.ExecContext(ctx, `UPDATE history_events SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ticket_history SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Comments
		if _, err := tx.ExecContext(ctx, `UPDATE comments SET item_id = ? WHERE item_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Labels
		if _, err := tx.ExecContext(ctx, `UPDATE ticket_labels SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
		// Time entries
		if _, err := tx.ExecContext(ctx, `UPDATE time_entries SET ticket_id = ? WHERE ticket_id = ?`, m.newKey, m.oldKey); err != nil {
			return 0, err
		}
	}

	// Update the project prefix.
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET prefix = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, newPrefix, projectID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(mappings), nil
}

// extractSequence pulls the numeric suffix from a ticket key.
// E.g. "CUS-T-42" → 42, "TK-7" → 7.
func extractSequence(key string) int64 {
	idx := strings.LastIndex(key, "-")
	if idx < 0 {
		return 0
	}
	n, err := strconv.ParseInt(key[idx+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
}
