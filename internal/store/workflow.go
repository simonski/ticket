package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Workflow struct {
	ID          int64  `json:"workflow_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type WorkflowStage struct {
	ID                int64  `json:"workflow_stage_id"`
	WorkflowID        int64  `json:"workflow_id"`
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

type WorkflowWithStages struct {
	Workflow
	Stages []WorkflowStage `json:"stages"`
}

// Export types use role title instead of ID for portability.

type WorkflowStageExport struct {
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	Role        string `json:"role"`
	SortOrder   int    `json:"sort_order"`
}

type WorkflowExport struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Stages      []WorkflowStageExport `json:"stages"`
}

func CreateWorkflow(ctx context.Context, db *sql.DB, name, description string) (Workflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO workflows (name, description, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, name, strings.TrimSpace(description))
	if err != nil {
		return Workflow{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Workflow{}, err
	}
	return getWorkflowRow(ctx, db, id)
}

func ListWorkflows(ctx context.Context, db *sql.DB) ([]Workflow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT workflow_id, name, description, created_at, updated_at
		FROM workflows
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workflows := make([]Workflow, 0)
	for rows.Next() {
		var w Workflow
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func GetWorkflow(ctx context.Context, db *sql.DB, id int64) (WorkflowWithStages, error) {
	w, err := getWorkflowRow(ctx, db, id)
	if err != nil {
		return WorkflowWithStages{}, err
	}
	stages, err := listWorkflowStages(ctx, db, id)
	if err != nil {
		return WorkflowWithStages{}, err
	}
	return WorkflowWithStages{Workflow: w, Stages: stages}, nil
}

func DeleteWorkflow(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM workflow_stages WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM workflows WHERE workflow_id = ?`, id)
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

func AddWorkflowStage(ctx context.Context, db *sql.DB, workflowID int64, stageName, description string, roleID *int64, sortOrder int) (WorkflowStage, error) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return WorkflowStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO workflow_stages (workflow_id, stage_name, description, role_id, sort_order, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, workflowID, stageName, strings.TrimSpace(description), roleID, sortOrder)
	if err != nil {
		return WorkflowStage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WorkflowStage{}, err
	}
	return getWorkflowStageRow(ctx, db, id)
}

func RemoveWorkflowStage(ctx context.Context, db *sql.DB, stageID int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM workflow_stages WHERE workflow_stage_id = ?`, stageID)
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

func ReorderWorkflowStages(ctx context.Context, db *sql.DB, workflowID int64, orderedStageIDs []int64) error {
	for i, id := range orderedStageIDs {
		result, err := db.ExecContext(ctx, `
			UPDATE workflow_stages SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE workflow_stage_id = ? AND workflow_id = ?
		`, i, id, workflowID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("workflow stage %d not found in workflow %d", id, workflowID)
		}
	}
	return nil
}

func ExportWorkflow(ctx context.Context, db *sql.DB, id int64) (WorkflowExport, error) {
	wf, err := GetWorkflow(ctx, db, id)
	if err != nil {
		return WorkflowExport{}, err
	}
	export := WorkflowExport{
		Name:        wf.Name,
		Description: wf.Description,
		Stages:      make([]WorkflowStageExport, len(wf.Stages)),
	}
	for i, s := range wf.Stages {
		export.Stages[i] = WorkflowStageExport{
			StageName:   s.StageName,
			Description: s.Description,
			Role:        s.RoleTitle,
			SortOrder:   s.SortOrder,
		}
	}
	return export, nil
}

func ImportWorkflow(ctx context.Context, db *sql.DB, export WorkflowExport) (Workflow, error) {
	name := strings.TrimSpace(export.Name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	wf, err := CreateWorkflow(ctx, db, name, export.Description)
	if err != nil {
		return Workflow{}, err
	}
	for _, s := range export.Stages {
		var roleID *int64
		if strings.TrimSpace(s.Role) != "" {
			role, err := GetRoleByTitle(ctx, db, s.Role)
			if err != nil {
				return Workflow{}, fmt.Errorf("role %q not found: %w", s.Role, err)
			}
			roleID = &role.ID
		}
		if _, err := AddWorkflowStage(ctx, db, wf.ID, s.StageName, s.Description, roleID, s.SortOrder); err != nil {
			return Workflow{}, err
		}
	}
	return wf, nil
}

// internal helpers

func getWorkflowRow(ctx context.Context, db *sql.DB, id int64) (Workflow, error) {
	row := db.QueryRowContext(ctx, `
		SELECT workflow_id, name, description, created_at, updated_at
		FROM workflows
		WHERE workflow_id = ?
	`, id)
	var w Workflow
	if err := row.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return Workflow{}, err
	}
	return w, nil
}

func getWorkflowStageRow(ctx context.Context, db *sql.DB, id int64) (WorkflowStage, error) {
	row := db.QueryRowContext(ctx, `
		SELECT ws.workflow_stage_id, ws.workflow_id, ws.stage_name, ws.description,
		       ws.definition_of_ready, ws.definition_of_done,
		       ws.role_id, COALESCE(r.title, ''), ws.sort_order, ws.created_at, ws.updated_at
		FROM workflow_stages ws
		LEFT JOIN roles r ON r.role_id = ws.role_id
		WHERE ws.workflow_stage_id = ?
	`, id)
	var s WorkflowStage
	if err := row.Scan(&s.ID, &s.WorkflowID, &s.StageName, &s.Description,
		&s.DefinitionOfReady, &s.DefinitionOfDone,
		&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return WorkflowStage{}, err
	}
	return s, nil
}

func listWorkflowStages(ctx context.Context, db *sql.DB, workflowID int64) ([]WorkflowStage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ws.workflow_stage_id, ws.workflow_id, ws.stage_name, ws.description,
		       ws.definition_of_ready, ws.definition_of_done,
		       ws.role_id, COALESCE(r.title, ''), ws.sort_order, ws.created_at, ws.updated_at
		FROM workflow_stages ws
		LEFT JOIN roles r ON r.role_id = ws.role_id
		WHERE ws.workflow_id = ?
		ORDER BY ws.sort_order, ws.workflow_stage_id
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stages := make([]WorkflowStage, 0)
	for rows.Next() {
		var s WorkflowStage
		if err := rows.Scan(&s.ID, &s.WorkflowID, &s.StageName, &s.Description,
			&s.DefinitionOfReady, &s.DefinitionOfDone,
			&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, s)
	}
	return stages, rows.Err()
}
