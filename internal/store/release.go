package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Releases are the top-level delivery container: a Release contains Features
// (type=feature tickets), each Feature contains Epics, each Epic contains
// Stories/Bugs. A release moves through in_design -> in_progress -> complete.
// Features can only be added to / removed from a release while it is in_design.

var (
	ErrReleaseNotFound   = errors.New("release not found")
	ErrReleaseLocked     = errors.New("release is not in design — its features cannot be changed")
	ErrReleaseBadStatus  = errors.New("invalid release status")
	ErrNotAFeatureTicket = errors.New("only feature tickets can be added to a release")
)

const (
	ReleaseInDesign   = "in_design"
	ReleaseInProgress = "in_progress"
	ReleaseComplete   = "complete"
)

func validReleaseStatus(s string) bool {
	switch s {
	case ReleaseInDesign, ReleaseInProgress, ReleaseComplete:
		return true
	}
	return false
}

type Release struct {
	ID           int       `json:"id"`
	ProjectID    int       `json:"project_id"`
	Title        string    `json:"title"`
	Purpose      string    `json:"purpose"`
	TargetDate   string    `json:"target_date"`
	Status       string    `json:"status"`
	DesignedAt   string    `json:"designed_at"`
	StartedAt    string    `json:"started_at"`
	CompletedAt  string    `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	FeatureCount int       `json:"feature_count"`
	StoryCount   int       `json:"story_count"`
}

func scanRelease(row interface{ Scan(...any) error }) (Release, error) {
	var r Release
	var createdAt, updatedAt string
	if err := row.Scan(&r.ID, &r.ProjectID, &r.Title, &r.Purpose, &r.TargetDate, &r.Status,
		&r.DesignedAt, &r.StartedAt, &r.CompletedAt, &createdAt, &updatedAt); err != nil {
		return Release{}, err
	}
	r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	r.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return r, nil
}

const releaseColumns = `id, project_id, title, purpose, target_date, status, designed_at, started_at, completed_at, created_at, updated_at`

// withReleaseCounts populates FeatureCount and StoryCount for each release.
func withReleaseCounts(ctx context.Context, db *sql.DB, releases []Release) error {
	for i := range releases {
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tickets WHERE release_id = ? AND deleted = 0 AND type = 'feature'`,
			releases[i].ID).Scan(&releases[i].FeatureCount); err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tickets WHERE release_id = ? AND deleted = 0 AND type IN ('story','bug','task')`,
			releases[i].ID).Scan(&releases[i].StoryCount); err != nil {
			return err
		}
	}
	return nil
}

func ListReleases(ctx context.Context, db *sql.DB, projectID int) ([]Release, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+releaseColumns+` FROM releases WHERE project_id = ? ORDER BY
		CASE status WHEN 'in_progress' THEN 0 WHEN 'in_design' THEN 1 ELSE 2 END, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	releases := make([]Release, 0)
	for rows.Next() {
		r, scanErr := scanRelease(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		releases = append(releases, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := withReleaseCounts(ctx, db, releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func GetRelease(ctx context.Context, db *sql.DB, id int) (Release, error) {
	row := db.QueryRowContext(ctx, `SELECT `+releaseColumns+` FROM releases WHERE id = ?`, id)
	r, err := scanRelease(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Release{}, ErrReleaseNotFound
	}
	if err != nil {
		return Release{}, err
	}
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE release_id = ? AND deleted = 0 AND type = 'feature'`, r.ID).Scan(&r.FeatureCount)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE release_id = ? AND deleted = 0 AND type IN ('story','bug','task')`, r.ID).Scan(&r.StoryCount)
	return r, nil
}

func CreateRelease(ctx context.Context, db *sql.DB, projectID int, title, purpose, targetDate string) (Release, error) {
	res, err := db.ExecContext(ctx, `
		INSERT INTO releases (project_id, title, purpose, target_date, status, designed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'in_design', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, projectID, title, purpose, targetDate)
	if err != nil {
		return Release{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Release{}, err
	}
	return GetRelease(ctx, db, int(id))
}

func UpdateRelease(ctx context.Context, db *sql.DB, id int, title, purpose, targetDate string) (Release, error) {
	res, err := db.ExecContext(ctx, `
		UPDATE releases SET title = ?, purpose = ?, target_date = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, title, purpose, targetDate, id)
	if err != nil {
		return Release{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return Release{}, ErrReleaseNotFound
	}
	return GetRelease(ctx, db, id)
}

// SetReleaseStatus transitions a release and stamps the corresponding timestamp
// (started_at on entering in_progress, completed_at on entering complete).
func SetReleaseStatus(ctx context.Context, db *sql.DB, id int, status string) (Release, error) {
	if !validReleaseStatus(status) {
		return Release{}, ErrReleaseBadStatus
	}
	stamp := ""
	switch status {
	case ReleaseInProgress:
		stamp = "started_at"
	case ReleaseComplete:
		stamp = "completed_at"
	}
	query := `UPDATE releases SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	if stamp != "" {
		// Only stamp the first time the status is reached.
		query = `UPDATE releases SET status = ?, ` + stamp + ` = CASE WHEN ` + stamp + ` = '' THEN CURRENT_TIMESTAMP ELSE ` + stamp + ` END, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	}
	res, err := db.ExecContext(ctx, query, status, id)
	if err != nil {
		return Release{}, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return Release{}, ErrReleaseNotFound
	}
	return GetRelease(ctx, db, id)
}

func DeleteRelease(ctx context.Context, db *sql.DB, id int) error {
	if _, err := db.ExecContext(ctx, `UPDATE tickets SET release_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE release_id = ?`, id); err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, `DELETE FROM releases WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return ErrReleaseNotFound
	}
	return nil
}

// subtreeIDsClause returns the recursive-CTE SQL that yields a ticket and all of
// its descendants, for use inside `WHERE ticket_id IN (...)`.
const subtreeIDsClause = `WITH RECURSIVE sub(id) AS (
	SELECT ticket_id FROM tickets WHERE ticket_id = ?
	UNION ALL
	SELECT t.ticket_id FROM tickets t JOIN sub ON t.parent_id = sub.id
) SELECT id FROM sub`

// AssignFeatureToRelease adds a feature (and its whole epic/story subtree) to a
// release. Only allowed while the release is in_design.
func AssignFeatureToRelease(ctx context.Context, db *sql.DB, featureTicketID string, releaseID int) error {
	rel, err := GetRelease(ctx, db, releaseID)
	if err != nil {
		return err
	}
	if rel.Status != ReleaseInDesign {
		return ErrReleaseLocked
	}
	ticket, err := GetTicket(ctx, db, featureTicketID)
	if err != nil {
		return err
	}
	if ticket.Type != "feature" {
		return ErrNotAFeatureTicket
	}
	_, err = db.ExecContext(ctx, `UPDATE tickets SET release_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id IN (`+subtreeIDsClause+`)`, releaseID, featureTicketID)
	return err
}

// RemoveFeatureFromRelease detaches a feature subtree from its release. Only
// allowed while the release is in_design.
func RemoveFeatureFromRelease(ctx context.Context, db *sql.DB, featureTicketID string) error {
	ticket, err := GetTicket(ctx, db, featureTicketID)
	if err != nil {
		return err
	}
	if ticket.ReleaseID != nil {
		rel, relErr := GetRelease(ctx, db, *ticket.ReleaseID)
		if relErr == nil && rel.Status != ReleaseInDesign {
			return ErrReleaseLocked
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE tickets SET release_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE ticket_id IN (`+subtreeIDsClause+`)`, featureTicketID)
	return err
}

// CloneFeature deep-clones a feature and its entire epic/story subtree into a
// fresh, unreleased draft so a user can extend its functionality. Returns the
// new root (feature) ticket. clone_of on each new ticket points at its origin.
func CloneFeature(ctx context.Context, db *sql.DB, featureTicketID, actorUsername, actorID string) (Ticket, error) {
	root, err := GetTicket(ctx, db, featureTicketID)
	if err != nil {
		return Ticket{}, err
	}
	// Breadth-first walk: clone each node, mapping origin ID -> new ticket ID so
	// children attach under their cloned parent.
	idMap := map[string]string{}
	var newRoot Ticket
	queue := []Ticket{root}
	for len(queue) > 0 {
		orig := queue[0]
		queue = queue[1:]

		var parentID *string
		if orig.ID == root.ID {
			parentID = nil
		} else if orig.ParentID != nil {
			if mapped, ok := idMap[*orig.ParentID]; ok {
				p := mapped
				parentID = &p
			}
		}
		cloneOf := orig.ID
		title := orig.Title
		if orig.ID == root.ID {
			title = orig.Title + " (clone)"
		}
		created, cErr := CreateTicket(ctx, db, TicketCreateParams{
			ProjectID:          orig.ProjectID,
			ParentID:           parentID,
			CloneOf:            &cloneOf,
			Type:               orig.Type,
			Title:              title,
			Description:        orig.Description,
			AcceptanceCriteria: orig.AcceptanceCriteria,
			Priority:           orig.Priority,
			Author:             actorUsername,
			CreatedBy:          actorID,
			State:              StateIdle,
		})
		if cErr != nil {
			return Ticket{}, cErr
		}
		idMap[orig.ID] = created.ID
		if orig.ID == root.ID {
			newRoot = created
		}

		children, childErr := listStoredChildTickets(ctx, db, orig.ID)
		if childErr != nil {
			return Ticket{}, childErr
		}
		queue = append(queue, children...)
	}
	_ = AddHistoryEvent(ctx, db, root.ProjectID, newRoot.ID, "feature_cloned", map[string]any{
		"from":  root.ID,
		"actor": actorUsername,
	}, actorID)
	return newRoot, nil
}
