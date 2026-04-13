package store

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
)

type Role struct {
	ID                 int64  `json:"role_id"`
	SdlcID             *int64 `json:"sdlc_id,omitempty"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func CreateRole(ctx context.Context, db *sql.DB, sdlcID *int64, title, description, ac string) (Role, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO roles (sdlc_id, title, description, acceptance_criteria, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, nullableInt64(sdlcID), title, strings.TrimSpace(description), strings.TrimSpace(ac))
	if err != nil {
		return Role{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Role{}, err
	}
	return GetRoleByID(ctx, db, id)
}

func ListRoles(ctx context.Context, db *sql.DB, limit int) ([]Role, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.QueryContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		ORDER BY title
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func ListRolesBySdlc(ctx context.Context, db *sql.DB, sdlcID int64) ([]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		WHERE sdlc_id = ?
		ORDER BY title
	`, sdlcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func scanRoles(rows *sql.Rows) ([]Role, error) {
	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		var sdlcID sql.NullInt64
		if err := rows.Scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		if sdlcID.Valid {
			role.SdlcID = &sdlcID.Int64
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func GetRoleByID(ctx context.Context, db *sql.DB, id int64) (Role, error) {
	row := db.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		WHERE role_id = ?
	`, id)
	var role Role
	var sdlcID sql.NullInt64
	if err := row.Scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	if sdlcID.Valid {
		role.SdlcID = &sdlcID.Int64
	}
	return role, nil
}

func GetRoleByTitle(ctx context.Context, db *sql.DB, title string) (Role, error) {
	row := db.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	var role Role
	var sdlcID sql.NullInt64
	if err := row.Scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	if sdlcID.Valid {
		role.SdlcID = &sdlcID.Int64
	}
	return role, nil
}

func getRoleByTitleTx(ctx context.Context, tx *sql.Tx, title string) (Role, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	var role Role
	var sdlcID sql.NullInt64
	if err := row.Scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	if sdlcID.Valid {
		role.SdlcID = &sdlcID.Int64
	}
	return role, nil
}

func UpdateRole(ctx context.Context, db *sql.DB, id int64, title, description, ac string) (Role, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE roles
		SET title = ?, description = ?, acceptance_criteria = ?, updated_at = CURRENT_TIMESTAMP
		WHERE role_id = ?
	`, title, strings.TrimSpace(description), strings.TrimSpace(ac), id)
	if err != nil {
		return Role{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Role{}, err
	}
	if affected == 0 {
		return Role{}, sql.ErrNoRows
	}
	return GetRoleByID(ctx, db, id)
}

func DeleteRole(ctx context.Context, db *sql.DB, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM roles WHERE role_id = ?`, id)
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

// ─── Stage-Role junction ────────────────────────────────────────────────────

func AddSdlcStageRole(ctx context.Context, db *sql.DB, sdlcID, stageID, roleID int64) error {
	// Auto-assign sort_order as max+1
	var maxOrder int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) FROM sdlc_stage_roles WHERE sdlc_id = ? AND stage_id = ?`, sdlcID, stageID).Scan(&maxOrder); err != nil {
		log.Printf("store: read max stage role sort order (sdlc=%d stage=%d): %v", sdlcID, stageID, err)
		maxOrder = -1
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO sdlc_stage_roles (sdlc_id, stage_id, role_id, sort_order)
		VALUES (?, ?, ?, ?)
	`, sdlcID, stageID, roleID, maxOrder+1)
	return err
}

func RemoveSdlcStageRole(ctx context.Context, db *sql.DB, sdlcID, stageID, roleID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM sdlc_stage_roles WHERE sdlc_id = ? AND stage_id = ? AND role_id = ?`, sdlcID, stageID, roleID)
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

func ReorderSdlcStageRoles(ctx context.Context, db *sql.DB, sdlcID, stageID int64, roleIDs []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, rid := range roleIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE sdlc_stage_roles SET sort_order = ? WHERE sdlc_id = ? AND stage_id = ? AND role_id = ?`, i, sdlcID, stageID, rid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ListSdlcStageRoles(ctx context.Context, db *sql.DB, sdlcID, stageID int64) ([]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.role_id, r.sdlc_id, r.title, r.description, r.acceptance_criteria, r.created_at, r.updated_at
		FROM sdlc_stage_roles sr
		JOIN roles r ON r.role_id = sr.role_id
		WHERE sr.sdlc_id = ? AND sr.stage_id = ?
		ORDER BY sr.sort_order
	`, sdlcID, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}
