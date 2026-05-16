package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrProjectGitRepositoryNotFound = errors.New("project git repository not found")

func ensureProjectRepositoryTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS project_git_repositories (
			project_id INTEGER NOT NULL,
			repository TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (project_id, repository),
			FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_git_repositories (project_id, repository)
		SELECT project_id, git_repository
		FROM projects
		WHERE trim(git_repository) <> ''
	`)
	return err
}

func ListProjectGitRepositories(ctx context.Context, db *sql.DB, projectID int64) ([]string, error) {
	if err := ensureProjectRepositoryTable(ctx, db); err != nil {
		return nil, err
	}
	if _, err := GetProjectByID(ctx, db, projectID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT repository
		FROM project_git_repositories
		WHERE project_id = ?
		ORDER BY repository
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var repositories []string
	for rows.Next() {
		var repository string
		if err := rows.Scan(&repository); err != nil {
			return nil, err
		}
		repositories = append(repositories, repository)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return repositories, nil
}

func AddProjectGitRepository(ctx context.Context, db *sql.DB, projectID int64, repository string) error {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return errors.New("repository is required")
	}
	if err := ensureProjectRepositoryTable(ctx, db); err != nil {
		return err
	}
	project, err := GetProjectByID(ctx, db, projectID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_git_repositories (project_id, repository)
		VALUES (?, ?)
	`, projectID, repository); err != nil {
		return err
	}
	if strings.TrimSpace(project.GitRepository) == "" {
		if _, err := db.ExecContext(ctx, `
			UPDATE projects
			SET git_repository = ?, updated_at = CURRENT_TIMESTAMP
			WHERE project_id = ?
		`, repository, projectID); err != nil {
			return err
		}
	}
	return nil
}

func RemoveProjectGitRepository(ctx context.Context, db *sql.DB, projectID int64, repository string) error {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return errors.New("repository is required")
	}
	if err := ensureProjectRepositoryTable(ctx, db); err != nil {
		return err
	}
	project, err := GetProjectByID(ctx, db, projectID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `
		DELETE FROM project_git_repositories
		WHERE project_id = ? AND repository = ?
	`, projectID, repository)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProjectGitRepositoryNotFound
	}
	if strings.TrimSpace(project.GitRepository) != repository {
		return nil
	}
	var nextRepository string
	if err := db.QueryRowContext(ctx, `
		SELECT repository
		FROM project_git_repositories
		WHERE project_id = ?
		ORDER BY repository
		LIMIT 1
	`, projectID).Scan(&nextRepository); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE projects
		SET git_repository = ?, updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
	`, strings.TrimSpace(nextRepository), projectID); err != nil {
		return err
	}
	return nil
}
