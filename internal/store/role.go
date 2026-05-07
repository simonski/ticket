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
	WorkflowID             *int64      `json:"workflow_id,omitempty"`
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
	ID                 *int64
	WorkflowID             *int64
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

func CreateRole(ctx context.Context, db *sql.DB, workflowID *int64, title, description, ac string) (Role, error) {
	return CreateRoleWithParams(ctx, db, RoleCreateParams{
		WorkflowID:             workflowID,
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
	explicitID, hasExplicitID, err := normalizeExplicitID(params.ID)
	if err != nil {
		return Role{}, err
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
	query := `
		INSERT INTO roles (workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	args := []any{nullableInt64(params.WorkflowID), title, strings.TrimSpace(params.Description), acceptanceCriteria, dorJSON, dodJSON, acJSON}
	if hasExplicitID {
		query = `
			INSERT INTO roles (role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`
		args = append([]any{explicitID}, args...)
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return Role{}, err
	}
	id := explicitID
	if !hasExplicitID {
		id, err = result.LastInsertId()
		if err != nil {
			return Role{}, err
		}
	}
	return GetRoleByID(ctx, db, id)
}

func scanRoleValues(scan func(dest ...any) error) (Role, error) {
	var role Role
	var workflowID sql.NullInt64
	var dorJSON, dodJSON, acJSON string
	if err := scan(&role.ID, &workflowID, &role.Title, &role.Description, &role.AcceptanceCriteria, &dorJSON, &dodJSON, &acJSON, &role.CreatedAt, &role.UpdatedAt); err != nil {
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
	if workflowID.Valid {
		role.WorkflowID = &workflowID.Int64
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
		SELECT role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
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

func ListRolesByWorkflow(ctx context.Context, db *sql.DB, workflowID int64) ([]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE workflow_id = ?
		ORDER BY title
	`, workflowID)
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
		SELECT role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE role_id = ?
	`, id)
	return scanRoleValues(row.Scan)
}

func GetRoleByTitle(ctx context.Context, db *sql.DB, title string) (Role, error) {
	row := db.QueryRowContext(ctx, `
		SELECT role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
		FROM roles
		WHERE title = ?
	`, strings.TrimSpace(title))
	return scanRoleValues(row.Scan)
}

func getRoleByTitleTx(ctx context.Context, tx *sql.Tx, title string) (Role, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT role_id, workflow_id, title, description, acceptance_criteria, dor_map, dod_map, ac_map, created_at, updated_at
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

func AddWorkflowStageRole(ctx context.Context, db *sql.DB, workflowID, stageID, roleID int64) error {
	// Auto-assign sort_order as max+1
	var maxOrder int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) FROM workflow_stage_roles WHERE workflow_id = ? AND stage_id = ?`, workflowID, stageID).Scan(&maxOrder); err != nil {
		log.Printf("store: read max stage role sort order (workflow=%d stage=%d): %v", workflowID, stageID, err)
		maxOrder = -1
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO workflow_stage_roles (workflow_id, stage_id, role_id, sort_order)
		VALUES (?, ?, ?, ?)
	`, workflowID, stageID, roleID, maxOrder+1)
	return err
}

func RemoveWorkflowStageRole(ctx context.Context, db *sql.DB, workflowID, stageID, roleID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM workflow_stage_roles WHERE workflow_id = ? AND stage_id = ? AND role_id = ?`, workflowID, stageID, roleID)
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

func ReorderWorkflowStageRoles(ctx context.Context, db *sql.DB, workflowID, stageID int64, roleIDs []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, rid := range roleIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_stage_roles SET sort_order = ? WHERE workflow_id = ? AND stage_id = ? AND role_id = ?`, i, workflowID, stageID, rid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ListWorkflowStageRoles(ctx context.Context, db *sql.DB, workflowID, stageID int64) ([]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.role_id, r.workflow_id, r.title, r.description, r.acceptance_criteria, r.dor_map, r.dod_map, r.ac_map, r.created_at, r.updated_at
		FROM workflow_stage_roles sr
		JOIN roles r ON r.role_id = sr.role_id
		WHERE sr.workflow_id = ? AND sr.stage_id = ?
		ORDER BY sr.sort_order
	`, workflowID, stageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}
