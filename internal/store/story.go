package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Story struct {
	ID          int64  `json:"story_id"`
	ProjectID   int64  `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func normalizeStoryStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "", "draft":
		return "draft"
	case "ready_for_review":
		return "ready_for_review"
	default:
		return "draft"
	}
}

func CreateStory(ctx context.Context, db *sql.DB, projectID int64, title, description string, createdBy string) (Story, error) {
	title = strings.TrimSpace(title)
	if projectID == 0 {
		return Story{}, errors.New("project is required")
	}
	if title == "" {
		return Story{}, errors.New("story title is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO stories (project_id, title, description, status, created_by, updated_at)
		VALUES (?, ?, ?, 'draft', ?, CURRENT_TIMESTAMP)
	`, projectID, title, strings.TrimSpace(description), nullableUserID(createdBy))
	if err != nil {
		return Story{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Story{}, err
	}
	return GetStory(ctx, db, id)
}

func ListStoriesByProject(ctx context.Context, db *sql.DB, projectID int64) ([]Story, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT story_id, project_id, title, description, status, COALESCE(created_by, ''), created_at, updated_at
		FROM stories
		WHERE project_id = ?
		ORDER BY created_at DESC, story_id DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stories := make([]Story, 0)
	for rows.Next() {
		var story Story
		if err := rows.Scan(&story.ID, &story.ProjectID, &story.Title, &story.Description, &story.Status, &story.CreatedBy, &story.CreatedAt, &story.UpdatedAt); err != nil {
			return nil, err
		}
		stories = append(stories, story)
	}
	return stories, rows.Err()
}

func GetStory(ctx context.Context, db *sql.DB, storyID int64) (Story, error) {
	row := db.QueryRowContext(ctx, `
		SELECT story_id, project_id, title, description, status, COALESCE(created_by, ''), created_at, updated_at
		FROM stories
		WHERE story_id = ?
	`, storyID)
	var story Story
	if err := row.Scan(&story.ID, &story.ProjectID, &story.Title, &story.Description, &story.Status, &story.CreatedBy, &story.CreatedAt, &story.UpdatedAt); err != nil {
		return Story{}, err
	}
	return story, nil
}

func UpdateStoryStatus(ctx context.Context, db *sql.DB, storyID int64, status string) (Story, error) {
	result, err := db.ExecContext(ctx, `
		UPDATE stories
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE story_id = ?
	`, normalizeStoryStatus(status), storyID)
	if err != nil {
		return Story{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Story{}, err
	}
	if affected == 0 {
		return Story{}, sql.ErrNoRows
	}
	return GetStory(ctx, db, storyID)
}

func UpdateStory(ctx context.Context, db *sql.DB, storyID int64, title, description string) (Story, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Story{}, errors.New("story title is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE stories
		SET title = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE story_id = ?
	`, title, strings.TrimSpace(description), storyID)
	if err != nil {
		return Story{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Story{}, err
	}
	if affected == 0 {
		return Story{}, sql.ErrNoRows
	}
	return GetStory(ctx, db, storyID)
}

func LinkStoryToTicket(ctx context.Context, db *sql.DB, storyID int64, ticketID string) error {
	if storyID == 0 || ticketID == "" {
		return errors.New("story and ticket are required")
	}
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO story_ticket_links (story_id, ticket_id)
		VALUES (?, ?)
	`, storyID, ticketID)
	return err
}

func DeleteStory(ctx context.Context, db *sql.DB, storyID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM stories WHERE story_id = ?`, storyID)
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

func StoryIDForTicket(ctx context.Context, db *sql.DB, ticketID string) (int64, bool, error) {
	var storyID int64
	err := db.QueryRowContext(ctx, `SELECT story_id FROM story_ticket_links WHERE ticket_id = ? ORDER BY story_id LIMIT 1`, ticketID).Scan(&storyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return storyID, true, nil
}
