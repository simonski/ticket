package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Sdlc struct {
	ID          int64  `json:"sdlc_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SdlcStage struct {
	ID                 int64  `json:"sdlc_stage_id"`
	SdlcID             int64  `json:"sdlc_id"`
	StageName          string `json:"stage_name"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	SortOrder          int    `json:"sort_order"`
	Roles              []Role `json:"roles,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type SdlcWithStages struct {
	Sdlc
	Stages []SdlcStage `json:"stages"`
}

// Export types use role title instead of ID for portability.

type SdlcStageExport struct {
	StageName   string   `json:"stage_name"`
	Description string   `json:"description"`
	Roles       []string `json:"roles,omitempty"`
	SortOrder   int      `json:"sort_order"`
}

type SdlcExport struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Stages      []SdlcStageExport `json:"stages"`
}

var ErrSdlcStageNotFound = errors.New("sdlc stage not found in sdlc")

func CreateSdlc(ctx context.Context, db *sql.DB, name, description string) (Sdlc, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Sdlc{}, errors.New("sdlc name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO sdlcs (name, description, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, name, strings.TrimSpace(description))
	if err != nil {
		return Sdlc{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Sdlc{}, err
	}
	return getSdlcRow(ctx, db, id)
}

func ListSdlcs(ctx context.Context, db *sql.DB) ([]Sdlc, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sdlc_id, name, description, created_at, updated_at
		FROM sdlcs
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sdlcs := make([]Sdlc, 0)
	for rows.Next() {
		var w Sdlc
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		sdlcs = append(sdlcs, w)
	}
	return sdlcs, rows.Err()
}

func GetSdlc(ctx context.Context, db *sql.DB, id int64) (SdlcWithStages, error) {
	w, err := getSdlcRow(ctx, db, id)
	if err != nil {
		return SdlcWithStages{}, err
	}
	stages, err := listSdlcStages(ctx, db, id)
	if err != nil {
		return SdlcWithStages{}, err
	}
	return SdlcWithStages{Sdlc: w, Stages: stages}, nil
}

func DeleteSdlc(ctx context.Context, db *sql.DB, id int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sdlc_stage_roles WHERE sdlc_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sdlc_stages WHERE sdlc_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM sdlcs WHERE sdlc_id = ?`, id)
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
	return tx.Commit()
}

func AddSdlcStage(ctx context.Context, db *sql.DB, sdlcID int64, stageName, description, acceptanceCriteria string, sortOrder int) (SdlcStage, error) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return SdlcStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO sdlc_stages (sdlc_id, stage_name, description, acceptance_criteria, sort_order, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, sdlcID, stageName, strings.TrimSpace(description), strings.TrimSpace(acceptanceCriteria), sortOrder)
	if err != nil {
		return SdlcStage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return SdlcStage{}, err
	}
	return getSdlcStageRow(ctx, db, id)
}

func UpdateSdlcStage(ctx context.Context, db *sql.DB, stageID int64, name, description, acceptanceCriteria string) (SdlcStage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SdlcStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE sdlc_stages
		SET stage_name = ?, description = ?, acceptance_criteria = ?, updated_at = CURRENT_TIMESTAMP
		WHERE sdlc_stage_id = ?
	`, name, strings.TrimSpace(description), strings.TrimSpace(acceptanceCriteria), stageID)
	if err != nil {
		return SdlcStage{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return SdlcStage{}, err
	}
	if affected == 0 {
		return SdlcStage{}, sql.ErrNoRows
	}
	return getSdlcStageRow(ctx, db, stageID)
}

func GetSdlcStage(ctx context.Context, db *sql.DB, stageID int64) (SdlcStage, error) {
	return getSdlcStageRow(ctx, db, stageID)
}

func ListSdlcStages(ctx context.Context, db *sql.DB, sdlcID int64) ([]SdlcStage, error) {
	return listSdlcStages(ctx, db, sdlcID)
}

func RemoveSdlcStage(ctx context.Context, db *sql.DB, stageID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM sdlc_stages WHERE sdlc_stage_id = ?`, stageID)
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

func ReorderSdlcStages(ctx context.Context, db *sql.DB, sdlcID int64, orderedStageIDs []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range orderedStageIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE sdlc_stages SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE sdlc_stage_id = ? AND sdlc_id = ?
		`, i, id, sdlcID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("%w %d in sdlc %d", ErrSdlcStageNotFound, id, sdlcID)
		}
	}
	return tx.Commit()
}

func ExportSdlc(ctx context.Context, db *sql.DB, id int64) (SdlcExport, error) {
	wf, err := GetSdlc(ctx, db, id)
	if err != nil {
		return SdlcExport{}, err
	}
	export := SdlcExport{
		Name:        wf.Name,
		Description: wf.Description,
		Stages:      make([]SdlcStageExport, len(wf.Stages)),
	}
	for i, s := range wf.Stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		export.Stages[i] = SdlcStageExport{
			StageName:   s.StageName,
			Description: s.Description,
			Roles:       roleNames,
			SortOrder:   s.SortOrder,
		}
	}
	return export, nil
}

func ImportSdlc(ctx context.Context, db *sql.DB, export SdlcExport) (Sdlc, error) {
	name := strings.TrimSpace(export.Name)
	if name == "" {
		return Sdlc{}, errors.New("sdlc name is required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Sdlc{}, err
	}
	defer tx.Rollback()

	// Create the SDLC
	result, err := tx.ExecContext(ctx, `
		INSERT INTO sdlcs (name, description, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, name, strings.TrimSpace(export.Description))
	if err != nil {
		return Sdlc{}, err
	}
	sdlcID, err := result.LastInsertId()
	if err != nil {
		return Sdlc{}, err
	}

	// Create stages and assign roles
	for _, s := range export.Stages {
		stageResult, err := tx.ExecContext(ctx, `
			INSERT INTO sdlc_stages (sdlc_id, stage_name, description, sort_order, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, sdlcID, strings.TrimSpace(s.StageName), strings.TrimSpace(s.Description), s.SortOrder)
		if err != nil {
			return Sdlc{}, err
		}
		stageID, err := stageResult.LastInsertId()
		if err != nil {
			return Sdlc{}, err
		}
		for _, roleName := range s.Roles {
			role, err := getRoleByTitleTx(ctx, tx, strings.TrimSpace(roleName))
			if err != nil {
				return Sdlc{}, fmt.Errorf("role %q not found: %w", roleName, err)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO sdlc_stage_roles (sdlc_id, stage_id, role_id, sort_order)
				VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM sdlc_stage_roles WHERE sdlc_id = ? AND stage_id = ?))
			`, sdlcID, stageID, role.ID, sdlcID, stageID); err != nil {
				return Sdlc{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return Sdlc{}, err
	}
	return getSdlcRow(ctx, db, sdlcID)
}

// internal helpers

func getSdlcRow(ctx context.Context, db *sql.DB, id int64) (Sdlc, error) {
	row := db.QueryRowContext(ctx, `
		SELECT sdlc_id, name, description, created_at, updated_at
		FROM sdlcs
		WHERE sdlc_id = ?
	`, id)
	var w Sdlc
	if err := row.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return Sdlc{}, err
	}
	return w, nil
}

func getSdlcStageRow(ctx context.Context, db *sql.DB, id int64) (SdlcStage, error) {
	row := db.QueryRowContext(ctx, `
		SELECT sdlc_stage_id, sdlc_id, stage_name, description, acceptance_criteria, sort_order, created_at, updated_at
		FROM sdlc_stages
		WHERE sdlc_stage_id = ?
	`, id)
	var s SdlcStage
	if err := row.Scan(&s.ID, &s.SdlcID, &s.StageName, &s.Description,
		&s.AcceptanceCriteria, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return SdlcStage{}, err
	}
	s.Roles, _ = ListSdlcStageRoles(ctx, db, s.SdlcID, s.ID)
	return s, nil
}

func listSdlcStages(ctx context.Context, db *sql.DB, sdlcID int64) ([]SdlcStage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sdlc_stage_id, sdlc_id, stage_name, description, acceptance_criteria, sort_order, created_at, updated_at
		FROM sdlc_stages
		WHERE sdlc_id = ?
		ORDER BY sort_order, sdlc_stage_id
	`, sdlcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stages := make([]SdlcStage, 0)
	for rows.Next() {
		var s SdlcStage
		if err := rows.Scan(&s.ID, &s.SdlcID, &s.StageName, &s.Description,
			&s.AcceptanceCriteria, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Batch-load all roles for all stages in one query to avoid N+1.
	rolesByStage, err := listSdlcStageRolesBatch(ctx, db, sdlcID)
	if err == nil {
		for i := range stages {
			if r, ok := rolesByStage[stages[i].ID]; ok {
				stages[i].Roles = r
			}
		}
	}
	return stages, nil
}

// listSdlcStageRolesBatch fetches all roles for every stage belonging to the
// given sdlcID in a single query and returns them grouped by stage_id.
func listSdlcStageRolesBatch(ctx context.Context, db *sql.DB, sdlcID int64) (map[int64][]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sr.stage_id, r.role_id, r.sdlc_id, r.title, r.description, r.acceptance_criteria, r.created_at, r.updated_at
		FROM sdlc_stage_roles sr
		JOIN roles r ON r.role_id = sr.role_id
		WHERE sr.sdlc_id = ?
		ORDER BY sr.stage_id, sr.sort_order
	`, sdlcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]Role)
	for rows.Next() {
		var stageID int64
		var role Role
		var sdlcNullID sql.NullInt64
		if err := rows.Scan(&stageID, &role.ID, &sdlcNullID, &role.Title, &role.Description, &role.AcceptanceCriteria, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		if sdlcNullID.Valid {
			role.SdlcID = &sdlcNullID.Int64
		}
		result[stageID] = append(result[stageID], role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
