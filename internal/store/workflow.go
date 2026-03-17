package store

import (
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
	ID          int64  `json:"workflow_stage_id"`
	WorkflowID  int64  `json:"workflow_id"`
	StageName   string `json:"stage_name"`
	Description string `json:"description"`
	RoleID      *int64 `json:"role_id"`
	RoleTitle   string `json:"role_title"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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

func CreateWorkflow(db *sql.DB, name, description string) (Workflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	result, err := db.Exec(`
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
	return getWorkflowRow(db, id)
}

func ListWorkflows(db *sql.DB) ([]Workflow, error) {
	rows, err := db.Query(`
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

func GetWorkflow(db *sql.DB, id int64) (WorkflowWithStages, error) {
	w, err := getWorkflowRow(db, id)
	if err != nil {
		return WorkflowWithStages{}, err
	}
	stages, err := listWorkflowStages(db, id)
	if err != nil {
		return WorkflowWithStages{}, err
	}
	return WorkflowWithStages{Workflow: w, Stages: stages}, nil
}

func DeleteWorkflow(db *sql.DB, id int64) error {
	if _, err := db.Exec(`DELETE FROM workflow_stages WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM workflows WHERE workflow_id = ?`, id)
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

func AddWorkflowStage(db *sql.DB, workflowID int64, stageName, description string, roleID *int64, sortOrder int) (WorkflowStage, error) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return WorkflowStage{}, errors.New("stage name is required")
	}
	result, err := db.Exec(`
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
	return getWorkflowStageRow(db, id)
}

func RemoveWorkflowStage(db *sql.DB, stageID int64) error {
	result, err := db.Exec(`DELETE FROM workflow_stages WHERE workflow_stage_id = ?`, stageID)
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

func ReorderWorkflowStages(db *sql.DB, workflowID int64, orderedStageIDs []int64) error {
	for i, id := range orderedStageIDs {
		result, err := db.Exec(`
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

func ExportWorkflow(db *sql.DB, id int64) (WorkflowExport, error) {
	wf, err := GetWorkflow(db, id)
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

func ImportWorkflow(db *sql.DB, export WorkflowExport) (Workflow, error) {
	name := strings.TrimSpace(export.Name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	wf, err := CreateWorkflow(db, name, export.Description)
	if err != nil {
		return Workflow{}, err
	}
	for _, s := range export.Stages {
		var roleID *int64
		if strings.TrimSpace(s.Role) != "" {
			role, err := GetRoleByTitle(db, s.Role)
			if err != nil {
				return Workflow{}, fmt.Errorf("role %q not found: %w", s.Role, err)
			}
			roleID = &role.ID
		}
		if _, err := AddWorkflowStage(db, wf.ID, s.StageName, s.Description, roleID, s.SortOrder); err != nil {
			return Workflow{}, err
		}
	}
	return wf, nil
}

// internal helpers

func getWorkflowRow(db *sql.DB, id int64) (Workflow, error) {
	row := db.QueryRow(`
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

func getWorkflowStageRow(db *sql.DB, id int64) (WorkflowStage, error) {
	row := db.QueryRow(`
		SELECT ws.workflow_stage_id, ws.workflow_id, ws.stage_name, ws.description,
		       ws.role_id, COALESCE(r.title, ''), ws.sort_order, ws.created_at, ws.updated_at
		FROM workflow_stages ws
		LEFT JOIN roles r ON r.role_id = ws.role_id
		WHERE ws.workflow_stage_id = ?
	`, id)
	var s WorkflowStage
	if err := row.Scan(&s.ID, &s.WorkflowID, &s.StageName, &s.Description,
		&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return WorkflowStage{}, err
	}
	return s, nil
}

func listWorkflowStages(db *sql.DB, workflowID int64) ([]WorkflowStage, error) {
	rows, err := db.Query(`
		SELECT ws.workflow_stage_id, ws.workflow_id, ws.stage_name, ws.description,
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
			&s.RoleID, &s.RoleTitle, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, s)
	}
	return stages, rows.Err()
}
