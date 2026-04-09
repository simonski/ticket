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
	ID                int64  `json:"sdlc_stage_id"`
	SdlcID        int64  `json:"sdlc_id"`
	StageName         string `json:"stage_name"`
	Description       string `json:"description"`
	DefinitionOfReady string `json:"definition_of_ready"`
	DefinitionOfDone  string `json:"definition_of_done"`
	RoleID            *int64 `json:"role_id"`
	RoleTitle         string `json:"role_title"`
	SortOrder         int    `json:"sort_order"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type SdlcWithStages struct {
	Sdlc
	Stages []SdlcStage `json:"stages"`
}

// Export types use role title instead of ID for portability.

type SdlcStageExport struct {
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	Role        string `json:"role"`
	SortOrder   int    `json:"sort_order"`
}

type SdlcExport struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Stages      []SdlcStageExport `json:"stages"`
}

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
	if _, err := db.ExecContext(ctx, `DELETE FROM sdlc_stages WHERE sdlc_id = ?`, id); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM sdlcs WHERE sdlc_id = ?`, id)
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

func AddSdlcStage(ctx context.Context, db *sql.DB, sdlcID int64, stageName, description string, roleID *int64, sortOrder int) (SdlcStage, error) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return SdlcStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO sdlc_stages (sdlc_id, stage_name, description, role_id, sort_order, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, sdlcID, stageName, strings.TrimSpace(description), roleID, sortOrder)
	if err != nil {
		return SdlcStage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return SdlcStage{}, err
	}
	return getSdlcStageRow(ctx, db, id)
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
	for i, id := range orderedStageIDs {
		result, err := db.ExecContext(ctx, `
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
			return fmt.Errorf("sdlc stage %d not found in sdlc %d", id, sdlcID)
		}
	}
	return nil
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
		export.Stages[i] = SdlcStageExport{
			StageName:   s.StageName,
			Description: s.Description,
			Role:        s.RoleTitle,
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
	wf, err := CreateSdlc(ctx, db, name, export.Description)
	if err != nil {
		return Sdlc{}, err
	}
	for _, s := range export.Stages {
		var roleID *int64
		if strings.TrimSpace(s.Role) != "" {
			role, err := GetRoleByTitle(ctx, db, s.Role)
			if err != nil {
				return Sdlc{}, fmt.Errorf("role %q not found: %w", s.Role, err)
			}
			roleID = &role.ID
		}
		if _, err := AddSdlcStage(ctx, db, wf.ID, s.StageName, s.Description, roleID, s.SortOrder); err != nil {
			return Sdlc{}, err
		}
	}
	return wf, nil
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
		SELECT ws.sdlc_stage_id, ws.sdlc_id, ws.stage_name, ws.description,
		       ws.definition_of_ready, ws.definition_of_done,
		       ws.role_id, COALESCE(r.title, ''), ws.sort_order, ws.created_at, ws.updated_at
		FROM sdlc_stages ws
		LEFT JOIN roles r ON r.role_id = ws.role_id
		WHERE ws.sdlc_stage_id = ?
	`, id)
	var s SdlcStage
	if err := row.Scan(&s.ID, &s.SdlcID, &s.StageName, &s.Description,
		&s.DefinitionOfReady, &s.DefinitionOfDone,
		&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return SdlcStage{}, err
	}
	return s, nil
}

func listSdlcStages(ctx context.Context, db *sql.DB, sdlcID int64) ([]SdlcStage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ws.sdlc_stage_id, ws.sdlc_id, ws.stage_name, ws.description,
		       ws.definition_of_ready, ws.definition_of_done,
		       ws.role_id, COALESCE(r.title, ''), ws.sort_order, ws.created_at, ws.updated_at
		FROM sdlc_stages ws
		LEFT JOIN roles r ON r.role_id = ws.role_id
		WHERE ws.sdlc_id = ?
		ORDER BY ws.sort_order, ws.sdlc_stage_id
	`, sdlcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stages := make([]SdlcStage, 0)
	for rows.Next() {
		var s SdlcStage
		if err := rows.Scan(&s.ID, &s.SdlcID, &s.StageName, &s.Description,
			&s.DefinitionOfReady, &s.DefinitionOfDone,
			&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, s)
	}
	return stages, rows.Err()
}
