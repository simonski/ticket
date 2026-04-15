package store

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
)

type Role struct {
	ID                 int64       `json:"role_id"`
	SdlcID             *int64      `json:"sdlc_id,omitempty"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	AcceptanceCriteria string      `json:"acceptance_criteria"`
	DORMap             GuidanceMap `json:"dor_map,omitempty"`
	DODMap             GuidanceMap `json:"dod_map,omitempty"`
	ACMap              GuidanceMap `json:"ac_map,omitempty"`
	CreatedAt          string      `json:"created_at"`
	UpdatedAt          string      `json:"updated_at"`
}

type RoleCreateParams struct {
	SdlcID             *int64
	Title              string
	Description        string
	AcceptanceCriteria string
	DORMap             GuidanceMap
	DODMap             GuidanceMap
	ACMap              GuidanceMap
}

type RoleUpdateParams struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	DORMap             GuidanceMap
	DODMap             GuidanceMap
	ACMap              GuidanceMap
}

func (r Role) ResolveGuidance(stage string) ResolvedGuidance {
	return resolveGuidance(stage, r.DORMap, r.DODMap, r.ACMap)
}

func CreateRole(ctx context.Context, db *sql.DB, sdlcID *int64, title, description, ac string) (Role, error) {
	return CreateRoleWithParams(ctx, db, RoleCreateParams{
		SdlcID:             sdlcID,
		Title:              title,
		Description:        description,
		AcceptanceCriteria: ac,
	})
}

func CreateRoleWithParams(ctx context.Context, db *sql.DB, params RoleCreateParams) (Role, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	dorJSON, err := guidanceMapJSON(params.DORMap)
	if err != nil {
		return Role{}, err
	}
	dodJSON, err := guidanceMapJSON(params.DODMap)
	if err != nil {
		return Role{}, err
	}
	acMap := withLegacyAcceptanceCriteria(params.AcceptanceCriteria, params.ACMap)
	acJSON, err := guidanceMapJSON(acMap)
	if err != nil {
		return Role{}, err
	}
	acceptanceCriteria := strings.TrimSpace(params.AcceptanceCriteria)
	if acceptanceCriteria == "" && acMap != nil {
		acceptanceCriteria = acMap[DefaultGuidanceStageKey]
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO roles (sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, nullableInt64(params.SdlcID), title, strings.TrimSpace(params.Description), acceptanceCriteria, dorJSON, dodJSON, acJSON)
	if err != nil {
		return Role{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Role{}, err
	}
	return GetRoleByID(ctx, db, id)
}

func scanRoleValues(scan func(dest ...any) error) (Role, error) {
	var role Role
	var sdlcID sql.NullInt64
	var dorJSON, dodJSON, acJSON string
	if err := scan(&role.ID, &sdlcID, &role.Title, &role.Description, &role.AcceptanceCriteria, &dorJSON, &dodJSON, &acJSON, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return Role{}, err
	}
	var err error
	role.DORMap, err = parseGuidanceMap(dorJSON)
	if err != nil {
		return Role{}, err
	}
	role.DODMap, err = parseGuidanceMap(dodJSON)
	if err != nil {
		return Role{}, err
	}
	role.ACMap, err = parseGuidanceMap(acJSON)
	if err != nil {
		return Role{}, err
	}
	role.ACMap = withLegacyAcceptanceCriteria(role.AcceptanceCriteria, role.ACMap)
	if sdlcID.Valid {
		role.SdlcID = &sdlcID.Int64
	}
	return role, nil
}

func UpdateRole(ctx context.Context, db *sql.DB, id int64, title, description, ac string) (Role, error) {
	return UpdateRoleWithParams(ctx, db, id, RoleUpdateParams{
		Title:              title,
		Description:        description,
		AcceptanceCriteria: ac,
	})
}

func UpdateRoleWithParams(ctx context.Context, db *sql.DB, id int64, params RoleUpdateParams) (Role, error) {
	current, err := GetRoleByID(ctx, db, id)
	if err != nil {
		return Role{}, err
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Role{}, errors.New("role title is required")
	}
	description := strings.TrimSpace(params.Description)
	if description == "" {
		description = current.Description
	}
	acceptanceCriteria := strings.TrimSpace(params.AcceptanceCriteria)
	if acceptanceCriteria == "" {
		acceptanceCriteria = current.AcceptanceCriteria
	}
	nextDORMap := current.DORMap
	if params.DORMap != nil {
		nextDORMap = normalizeGuidanceMap(params.DORMap)
	}
	nextDODMap := current.DODMap
	if params.DODMap != nil {
		nextDODMap = normalizeGuidanceMap(params.DODMap)
	}
	nextACMap := current.ACMap
	if params.ACMap != nil {
		nextACMap = params.ACMap
	}
	if strings.TrimSpace(params.AcceptanceCriteria) != "" && params.ACMap == nil {
		nextACMap = withLegacyAcceptanceCriteria(params.AcceptanceCriteria, current.ACMap)
	}
	nextACMap = withLegacyAcceptanceCriteria(acceptanceCriteria, nextACMap)
	dorJSON, err := guidanceMapJSON(nextDORMap)
	if err != nil {
		return Role{}, err
	}
	dodJSON, err := guidanceMapJSON(nextDODMap)
	if err != nil {
		return Role{}, err
	}
	acJSON, err := guidanceMapJSON(nextACMap)
	if err != nil {
		return Role{}, err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE roles
		SET title = ?, description = ?, acceptance_criteria = ?, dor_map = ?, dod_map = ?, ac_map = ?, updated_at = CURRENT_TIMESTAMP
		WHERE role_id = ?
	`, title, description, acceptanceCriteria, dorJSON, dodJSON, acJSON, id)
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

func ListRoles(ctx context.Context, db *sql.DB, limit int) ([]Role, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.QueryContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
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
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
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
		role, err := scanRoleValues(rows.Scan)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func GetRoleByID(ctx context.Context, db *sql.DB, id int64) (Role, error) {
	row := db.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE role_id = ?
	`, id)
	return scanRoleValues(row.Scan)
}

func GetRoleByTitle(ctx context.Context, db *sql.DB, title string) (Role, error) {
	row := db.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	return scanRoleValues(row.Scan)
}

func getRoleByTitleTx(ctx context.Context, tx *sql.Tx, title string) (Role, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT role_id, sdlc_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	return scanRoleValues(row.Scan)
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
		SELECT r.role_id, r.sdlc_id, r.title, r.description, r.acceptance_criteria, r.dor_map, r.dod_map, r.ac_map, r.created_at, r.updated_at
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
