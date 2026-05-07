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
	ID                 int64  `json:"workflow_stage_id"`
	WorkflowID         int64  `json:"workflow_id"`
	StageName          string `json:"stage_name"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	DefinitionOfReady  string `json:"definition_of_ready"`
	DefinitionOfDone   string `json:"definition_of_done"`
	SortOrder          int    `json:"sort_order"`
	Roles              []Role `json:"roles,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type WorkflowWithStages struct {
	Workflow
	Stages []WorkflowStage `json:"stages"`
}

// Export types use role title instead of ID for portability.

type WorkflowStageExport struct {
	StageName   string   `json:"stage_name"`
	Description string   `json:"description"`
	Roles       []string `json:"roles,omitempty"`
	SortOrder   int      `json:"sort_order"`
}

type WorkflowExport struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Stages      []WorkflowStageExport `json:"stages"`
}

var ErrWorkflowStageNotFound = errors.New("workflow stage not found in workflow")

func CreateWorkflow(ctx context.Context, db *sql.DB, name, description string) (Workflow, error) {
	return CreateWorkflowWithParams(ctx, db, nil, name, description)
}

func CreateWorkflowWithParams(ctx context.Context, db *sql.DB, id *int64, name, description string) (Workflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	explicitID, hasExplicitID, err := normalizeExplicitID(id)
	if err != nil {
		return Workflow{}, err
	}
	query := `
		INSERT INTO workflows (name, description, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`
	args := []any{name, strings.TrimSpace(description)}
	if hasExplicitID {
		query = `
			INSERT INTO workflows (workflow_id, name, description, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		`
		args = append([]any{explicitID}, args...)
	}
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return Workflow{}, err
	}
	createdID := explicitID
	if !hasExplicitID {
		createdID, err = result.LastInsertId()
		if err != nil {
			return Workflow{}, err
		}
	}
	return getWorkflowRow(ctx, db, createdID)
}

func ListWorkflows(ctx context.Context, db *sql.DB, limit, offset int) ([]Workflow, error) {
	limit, offset, err := normalizePage(limit, offset, DefaultListLimit)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT workflow_id, name, description, created_at, updated_at
		FROM workflows
		ORDER BY name
		LIMIT ? OFFSET ?
	`, limit, offset)
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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_stage_roles WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_stages WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM workflows WHERE workflow_id = ?`, id)
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

func AddWorkflowStage(ctx context.Context, db *sql.DB, workflowID int64, stageName, description, acceptanceCriteria string, sortOrder int) (WorkflowStage, error) {
	return AddWorkflowStageWithDefinitions(ctx, db, workflowID, stageName, description, acceptanceCriteria, "", sortOrder)
}

func AddWorkflowStageWithDefinitions(ctx context.Context, db *sql.DB, workflowID int64, stageName, wow, dor, dod string, sortOrder int) (WorkflowStage, error) {
	stageName = strings.TrimSpace(stageName)
	if stageName == "" {
		return WorkflowStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO workflow_stages (workflow_id, stage_name, description, acceptance_criteria, definition_of_ready, definition_of_done, sort_order, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, workflowID, stageName, strings.TrimSpace(wow), strings.TrimSpace(dor), strings.TrimSpace(dor), strings.TrimSpace(dod), sortOrder)
	if err != nil {
		return WorkflowStage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WorkflowStage{}, err
	}
	return getWorkflowStageRow(ctx, db, id)
}

func UpdateWorkflowStage(ctx context.Context, db *sql.DB, stageID int64, name, description, acceptanceCriteria string) (WorkflowStage, error) {
	return UpdateWorkflowStageWithDefinitions(ctx, db, stageID, name, description, acceptanceCriteria, "")
}

func UpdateWorkflowStageWithDefinitions(ctx context.Context, db *sql.DB, stageID int64, name, wow, dor, dod string) (WorkflowStage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return WorkflowStage{}, errors.New("stage name is required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE workflow_stages
		SET stage_name = ?, description = ?, acceptance_criteria = ?, definition_of_ready = ?, definition_of_done = ?, updated_at = CURRENT_TIMESTAMP
		WHERE workflow_stage_id = ?
	`, name, strings.TrimSpace(wow), strings.TrimSpace(dor), strings.TrimSpace(dor), strings.TrimSpace(dod), stageID)
	if err != nil {
		return WorkflowStage{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return WorkflowStage{}, err
	}
	if affected == 0 {
		return WorkflowStage{}, sql.ErrNoRows
	}
	return getWorkflowStageRow(ctx, db, stageID)
}

func GetWorkflowStage(ctx context.Context, db *sql.DB, stageID int64) (WorkflowStage, error) {
	return getWorkflowStageRow(ctx, db, stageID)
}

func ListWorkflowStages(ctx context.Context, db *sql.DB, workflowID int64) ([]WorkflowStage, error) {
	return listWorkflowStages(ctx, db, workflowID)
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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i, id := range orderedStageIDs {
		result, err := tx.ExecContext(ctx, `
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
			return fmt.Errorf("%w %d in workflow %d", ErrWorkflowStageNotFound, id, workflowID)
		}
	}
	return tx.Commit()
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
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		export.Stages[i] = WorkflowStageExport{
			StageName:   s.StageName,
			Description: s.Description,
			Roles:       roleNames,
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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Create the Workflow
	result, err := tx.ExecContext(ctx, `
		INSERT INTO workflows (name, description, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, name, strings.TrimSpace(export.Description))
	if err != nil {
		return Workflow{}, err
	}
	workflowID, err := result.LastInsertId()
	if err != nil {
		return Workflow{}, err
	}

	// Create stages and assign roles
	for _, s := range export.Stages {
		stageResult, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_stages (workflow_id, stage_name, description, sort_order, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, workflowID, strings.TrimSpace(s.StageName), strings.TrimSpace(s.Description), s.SortOrder)
		if err != nil {
			return Workflow{}, err
		}
		stageID, err := stageResult.LastInsertId()
		if err != nil {
			return Workflow{}, err
		}
		for _, roleName := range s.Roles {
			role, err := getRoleByTitleTx(ctx, tx, strings.TrimSpace(roleName))
			if err != nil {
				return Workflow{}, fmt.Errorf("role %q not found: %w", roleName, err)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO workflow_stage_roles (workflow_id, stage_id, role_id, sort_order)
				VALUES (?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM workflow_stage_roles WHERE workflow_id = ? AND stage_id = ?))
			`, workflowID, stageID, role.ID, workflowID, stageID); err != nil {
				return Workflow{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, err
	}
	return getWorkflowRow(ctx, db, workflowID)
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
		SELECT workflow_stage_id, workflow_id, stage_name, description, acceptance_criteria, definition_of_ready, definition_of_done, sort_order, created_at, updated_at
		FROM workflow_stages
		WHERE workflow_stage_id = ?
	`, id)
	var s WorkflowStage
	if err := row.Scan(&s.ID, &s.WorkflowID, &s.StageName, &s.Description,
		&s.AcceptanceCriteria, &s.DefinitionOfReady, &s.DefinitionOfDone, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return WorkflowStage{}, err
	}
	roles, err := ListWorkflowStageRoles(ctx, db, s.WorkflowID, s.ID)
	if err != nil {
		return WorkflowStage{}, err
	}
	s.Roles = roles
	return s, nil
}

func listWorkflowStages(ctx context.Context, db *sql.DB, workflowID int64) ([]WorkflowStage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT workflow_stage_id, workflow_id, stage_name, description, acceptance_criteria, definition_of_ready, definition_of_done, sort_order, created_at, updated_at
		FROM workflow_stages
		WHERE workflow_id = ?
		ORDER BY sort_order, workflow_stage_id
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stages := make([]WorkflowStage, 0)
	for rows.Next() {
		var s WorkflowStage
		if err := rows.Scan(&s.ID, &s.WorkflowID, &s.StageName, &s.Description,
			&s.AcceptanceCriteria, &s.DefinitionOfReady, &s.DefinitionOfDone, &s.SortOrder, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		stages = append(stages, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Batch-load all roles for all stages in one query to avoid N+1.
	rolesByStage, err := listWorkflowStageRolesBatch(ctx, db, workflowID)
	if err == nil {
		for i := range stages {
			if r, ok := rolesByStage[stages[i].ID]; ok {
				stages[i].Roles = r
			}
		}
	}
	return stages, nil
}

// listWorkflowStageRolesBatch fetches all roles for every stage belonging to the
// given workflowID in a single query and returns them grouped by stage_id.
func listWorkflowStageRolesBatch(ctx context.Context, db *sql.DB, workflowID int64) (map[int64][]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sr.stage_id, r.role_id, r.workflow_id, r.title, r.description, r.acceptance_criteria, r.dor_map, r.dod_map, r.ac_map, r.created_at, r.updated_at
		FROM workflow_stage_roles sr
		JOIN roles r ON r.role_id = sr.role_id
		WHERE sr.workflow_id = ?
		ORDER BY sr.stage_id, sr.sort_order
	`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]Role)
	for rows.Next() {
		var stageID int64
		role, err := scanRoleValues(func(dest ...any) error {
			withStageID := append([]any{&stageID}, dest...)
			return rows.Scan(withStageID...)
		})
		if err != nil {
			return nil, err
		}
		result[stageID] = append(result[stageID], role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
