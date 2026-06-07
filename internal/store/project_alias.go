package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func normalizeProjectAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func SetProjectAlias(ctx context.Context, db *sql.DB, projectID int64, alias, userID string) error {
	alias = normalizeProjectAlias(alias)
	if alias == "" {
		return errors.New("project alias is required")
	}
	if _, err := GetProjectByID(ctx, db, projectID); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO project_aliases (alias_name, user_id, project_id, updated_at)
		VALUES (?, NULLIF(?, ''), ?, CURRENT_TIMESTAMP)
		ON CONFLICT(alias_name, user_id) DO UPDATE SET project_id = excluded.project_id, updated_at = CURRENT_TIMESTAMP
	`, alias, strings.TrimSpace(userID), projectID)
	return err
}

func GetProjectByAlias(ctx context.Context, db *sql.DB, alias, userID string) (Project, error) {
	alias = normalizeProjectAlias(alias)
	if alias == "" {
		return Project{}, ErrProjectNotFound
	}
	trimmedUserID := strings.TrimSpace(userID)
	query := `
		SELECT p.project_id, p.prefix, p.title, p.description, p.acceptance_criteria, p.dor_map, p.dod_map, p.ac_map, p.git_repository, p.notes, p.status, p.visibility, p.default_draft, COALESCE(p.created_by, ''), p.created_at, p.updated_at, p.workflow_id, p.agent_model_provider, p.agent_model_name, p.agent_model_url, p.agent_model_api_key, p.programme_id
		FROM project_aliases pa
		JOIN projects p ON p.project_id = pa.project_id
		WHERE pa.alias_name = ? AND ((pa.user_id IS NULL AND ? = '') OR pa.user_id = ?)
		ORDER BY CASE WHEN pa.user_id IS NULL THEN 1 ELSE 0 END
		LIMIT 1
	`
	row := db.QueryRowContext(ctx, query, alias, trimmedUserID, trimmedUserID)
	project, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, err
	}
	return project, nil
}
