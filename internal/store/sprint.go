package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrSprintNotFound   = errors.New("sprint not found")
	ErrSprintClosed     = errors.New("sprint is closed — tickets cannot be moved in or out")
	ErrSprintNotReady   = errors.New("sprint cannot be made active: some tickets are not yet ready for development (still in discovery stage)")
)

type Sprint struct {
	ID        int       `json:"id"`
	ProjectID int       `json:"project_id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Stage     string    `json:"stage"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ListSprints(ctx context.Context, db *sql.DB, projectID int) ([]Sprint, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, project_id, number, title, stage, created_at, updated_at
		FROM sprints
		WHERE project_id = ?
		ORDER BY number
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sprints []Sprint
	for rows.Next() {
		var s Sprint
		var createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Number, &s.Title, &s.Stage, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		sprints = append(sprints, s)
	}
	if sprints == nil {
		sprints = []Sprint{}
	}
	return sprints, rows.Err()
}

func CreateSprint(ctx context.Context, db *sql.DB, projectID int, title string) (Sprint, error) {
	var number int
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(number), 0) + 1 FROM sprints WHERE project_id = ?
	`, projectID).Scan(&number); err != nil {
		return Sprint{}, err
	}

	result, err := db.ExecContext(ctx, `
		INSERT INTO sprints (project_id, number, title, stage, created_at, updated_at)
		VALUES (?, ?, ?, 'design', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, projectID, number, title)
	if err != nil {
		return Sprint{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Sprint{}, err
	}
	return getSprint(ctx, db, int(id))
}

func UpdateSprint(ctx context.Context, db *sql.DB, id int, title, stage string) (Sprint, error) {
	// Guard: cannot set active if any tickets are still in discovery stage.
	if stage == "active" {
		var notReadyCount int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tickets WHERE sprint_id = ? AND stage = 'discovery'
		`, id).Scan(&notReadyCount); err != nil {
			return Sprint{}, err
		}
		if notReadyCount > 0 {
			return Sprint{}, ErrSprintNotReady
		}
	}
	result, err := db.ExecContext(ctx, `
		UPDATE sprints SET title = ?, stage = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, title, stage, id)
	if err != nil {
		return Sprint{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Sprint{}, err
	}
	if affected == 0 {
		return Sprint{}, ErrSprintNotFound
	}
	return getSprint(ctx, db, id)
}

func DeleteSprint(ctx context.Context, db *sql.DB, id int) error {
	// Null out sprint_id on tickets referencing this sprint.
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET sprint_id = NULL WHERE sprint_id = ?`, id); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM sprints WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrSprintNotFound
	}
	return nil
}

func SetTicketSprint(ctx context.Context, db *sql.DB, ticketID string, sprintID *int) error {
	// Guard: if the ticket is currently in a closed sprint, block the move.
	var currentSprintID sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT sprint_id FROM tickets WHERE ticket_id = ?`, ticketID).Scan(&currentSprintID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("ticket not found")
		}
		return err
	}
	if currentSprintID.Valid {
		var currentStage string
		if err := db.QueryRowContext(ctx, `SELECT stage FROM sprints WHERE id = ?`, currentSprintID.Int64).Scan(&currentStage); err == nil && currentStage == "closed" {
			return ErrSprintClosed
		}
	}
	// Guard: if moving into a sprint, that sprint must not be closed.
	if sprintID != nil {
		var targetStage string
		if err := db.QueryRowContext(ctx, `SELECT stage FROM sprints WHERE id = ?`, *sprintID).Scan(&targetStage); err == nil && targetStage == "closed" {
			return ErrSprintClosed
		}
	}
	var err error
	if sprintID == nil {
		_, err = db.ExecContext(ctx, `UPDATE tickets SET sprint_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE ticket_id = ?`, ticketID)
	} else {
		_, err = db.ExecContext(ctx, `UPDATE tickets SET sprint_id = ?, updated_at = CURRENT_TIMESTAMP WHERE ticket_id = ?`, *sprintID, ticketID)
	}
	return err
}

func getSprint(ctx context.Context, db *sql.DB, id int) (Sprint, error) {
	var s Sprint
	var createdAt, updatedAt string
	err := db.QueryRowContext(ctx, `
		SELECT id, project_id, number, title, stage, created_at, updated_at
		FROM sprints WHERE id = ?
	`, id).Scan(&s.ID, &s.ProjectID, &s.Number, &s.Title, &s.Stage, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Sprint{}, ErrSprintNotFound
	}
	if err != nil {
		return Sprint{}, err
	}
	s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return s, nil
}
