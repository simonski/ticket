package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Workflow struct {
	ID              int64  `json:"workflow_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	ApprovalPolicy  string `json:"approval_policy"`
	ProgressionMode string `json:"progression_mode"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

const (
	WorkflowApprovalPolicySingleRole = "single_role"
	WorkflowApprovalPolicyAllRoles   = "all_roles"
	WorkflowProgressionModeLinear    = "linear"
	WorkflowProgressionModeStageOnly = "stage_only"
)

type WorkflowStage struct {
	ID                 int64   `json:"workflow_stage_id"`
	WorkflowID         int64   `json:"workflow_id"`
	StageName          string  `json:"stage_name"`
	Description        string  `json:"description"`
	AcceptanceCriteria string  `json:"acceptance_criteria"`
	DefinitionOfReady  string  `json:"definition_of_ready"`
	DefinitionOfDone   string  `json:"definition_of_done"`
	SortOrder          int     `json:"sort_order"`
	IsBacklogStage     bool    `json:"is_backlog_stage"`
	Roles              []Role  `json:"roles,omitempty"`
	NextStageIDs       []int64 `json:"next_stage_ids,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	Attrs              Attrs   `json:"attrs,omitempty"`
}

type WorkflowWithStages struct {
	Workflow
	Stages []WorkflowStage `json:"stages"`
}

// Export types use role title instead of ID for portability.

type WorkflowStageExport struct {
	StageName      string   `json:"stage_name"`
	Description    string   `json:"description"`
	IsBacklogStage bool     `json:"is_backlog_stage,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	NextStages     []string `json:"next_stages,omitempty"`
	SortOrder      int      `json:"sort_order"`
}

type WorkflowExport struct {
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	ApprovalPolicy  string                `json:"approval_policy,omitempty"`
	ProgressionMode string                `json:"progression_mode,omitempty"`
	Stages          []WorkflowStageExport `json:"stages"`
}

var ErrWorkflowStageNotFound = errors.New("workflow stage not found in workflow")

type WorkflowStageTransition struct {
	WorkflowID  int64  `json:"workflow_id"`
	FromStageID int64  `json:"from_stage_id"`
	ToStageID   int64  `json:"to_stage_id"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   string `json:"created_at"`
}

type WorkflowGraphValidation struct {
	WorkflowID          int64    `json:"workflow_id"`
	StageCount          int      `json:"stage_count"`
	TransitionCount     int      `json:"transition_count"`
	TerminalStageIDs    []int64  `json:"terminal_stage_ids"`
	UnreachableStageIDs []int64  `json:"unreachable_stage_ids,omitempty"`
	Issues              []string `json:"issues,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	Valid               bool     `json:"valid"`
}

func CreateWorkflow(ctx context.Context, db *sql.DB, name, description string) (Workflow, error) {
	return CreateWorkflowWithParams(ctx, db, nil, name, description)
}

func CreateWorkflowWithParams(ctx context.Context, db *sql.DB, id *int64, name, description string) (Workflow, error) {
	return createWorkflowWithOptions(ctx, db, id, name, description, WorkflowApprovalPolicySingleRole, WorkflowProgressionModeLinear)
}

func CreateWorkflowWithOptions(ctx context.Context, db *sql.DB, id *int64, name, description, approvalPolicy, progressionMode string) (Workflow, error) {
	return createWorkflowWithOptions(ctx, db, id, name, description, approvalPolicy, progressionMode)
}

func createWorkflowWithOptions(ctx context.Context, db *sql.DB, id *int64, name, description, approvalPolicy, progressionMode string) (Workflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	approvalPolicy, err := normalizeWorkflowApprovalPolicy(approvalPolicy)
	if err != nil {
		return Workflow{}, err
	}
	progressionMode, err = normalizeWorkflowProgressionMode(progressionMode)
	if err != nil {
		return Workflow{}, err
	}
	explicitID, hasExplicitID, err := normalizeExplicitID(id)
	if err != nil {
		return Workflow{}, err
	}
	query := `
		INSERT INTO workflows (name, description, approval_policy, progression_mode, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	args := []any{name, strings.TrimSpace(description), approvalPolicy, progressionMode}
	if hasExplicitID {
		query = `
			INSERT INTO workflows (workflow_id, name, description, approval_policy, progression_mode, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
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

func UpdateWorkflow(ctx context.Context, db *sql.DB, id int64, name, description, approvalPolicy, progressionMode string) (Workflow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workflow{}, errors.New("workflow name is required")
	}
	approvalPolicy, err := normalizeWorkflowApprovalPolicy(approvalPolicy)
	if err != nil {
		return Workflow{}, err
	}
	progressionMode, err = normalizeWorkflowProgressionMode(progressionMode)
	if err != nil {
		return Workflow{}, err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE workflows
		SET name = ?, description = ?, approval_policy = ?, progression_mode = ?, updated_at = CURRENT_TIMESTAMP
		WHERE workflow_id = ?
	`, name, strings.TrimSpace(description), approvalPolicy, progressionMode, id)
	if err != nil {
		return Workflow{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Workflow{}, err
	}
	if affected == 0 {
		return Workflow{}, sql.ErrNoRows
	}
	return getWorkflowRow(ctx, db, id)
}

func ListWorkflows(ctx context.Context, db *sql.DB, limit, offset int) ([]Workflow, error) {
	limit, offset, err := normalizePage(limit, offset, DefaultListLimit)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT workflow_id, name, description, approval_policy, progression_mode, created_at, updated_at
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
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.ApprovalPolicy, &w.ProgressionMode, &w.CreatedAt, &w.UpdatedAt); err != nil {
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
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM workflow_stage_roles WHERE workflow_id = ?`, id); execErr != nil {
		return execErr
	}
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM workflow_stages WHERE workflow_id = ?`, id); execErr != nil {
		return execErr
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
	// guidance text lives in the attrs bag (TK-114); preserve the prior mapping
	// (description=wow, acceptance_criteria=definition_of_ready=dor, ...).
	attrsJSON, err := stageAttrsForWrite(nil, wow, dor, dor, dod)
	if err != nil {
		return WorkflowStage{}, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO workflow_stages (workflow_id, stage_name, sort_order, attrs, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, workflowID, stageName, sortOrder, attrsJSON)
	if err != nil {
		return WorkflowStage{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WorkflowStage{}, err
	}
	return getWorkflowStageRow(ctx, db, id)
}

// SetWorkflowStageBacklog marks a stage as a backlog stage (shown in the backlog board)
// or a sprint stage (shown in the sprint board). This should be called after creating
// stages to configure their board placement.
func SetWorkflowStageBacklog(ctx context.Context, db *sql.DB, stageID int64, isBacklogStage bool) error {
	_, err := db.ExecContext(ctx, `
		UPDATE workflow_stages SET is_backlog_stage = ?, updated_at = CURRENT_TIMESTAMP
		WHERE workflow_stage_id = ?
	`, isBacklogStage, stageID)
	return err
}

// SetWorkflowStageAttrs replaces the extensible attribute bag for a stage. Normal
// stage updates do not touch attrs, so the bag is preserved across them; this is
// the explicit way to write it.
func SetWorkflowStageAttrs(ctx context.Context, db *sql.DB, stageID int64, attrs Attrs) (WorkflowStage, error) {
	// The guidance-text fields also live in the bag (TK-114), so replace only the
	// extra keys and fold the stage's existing guidance text back in.
	current, err := getWorkflowStageRow(ctx, db, stageID)
	if err != nil {
		return WorkflowStage{}, err
	}
	attrsJSON, err := stageAttrsForWrite(attrs, current.Description, current.AcceptanceCriteria, current.DefinitionOfReady, current.DefinitionOfDone)
	if err != nil {
		return WorkflowStage{}, err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE workflow_stages SET attrs = ?, updated_at = CURRENT_TIMESTAMP
		WHERE workflow_stage_id = ?
	`, attrsJSON, stageID)
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

func UpdateWorkflowStage(ctx context.Context, db *sql.DB, stageID int64, name, description, acceptanceCriteria string) (WorkflowStage, error) {
	return UpdateWorkflowStageWithDefinitions(ctx, db, stageID, name, description, acceptanceCriteria, "", nil)
}

func UpdateWorkflowStageWithDefinitions(ctx context.Context, db *sql.DB, stageID int64, name, wow, dor, dod string, isBacklogStage *bool) (WorkflowStage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return WorkflowStage{}, errors.New("stage name is required")
	}
	// Guidance text lives in the attrs bag (TK-114); fold it in over the stage's
	// existing extra bag, preserving the prior column mapping
	// (description=wow, acceptance_criteria=definition_of_ready=dor, ...).
	current, err := getWorkflowStageRow(ctx, db, stageID)
	if err != nil {
		return WorkflowStage{}, err
	}
	attrsJSON, err := stageAttrsForWrite(current.Attrs, wow, dor, dor, dod)
	if err != nil {
		return WorkflowStage{}, err
	}
	query := `UPDATE workflow_stages SET stage_name = ?, attrs = ?, updated_at = CURRENT_TIMESTAMP WHERE workflow_stage_id = ?`
	args := []any{name, attrsJSON, stageID}
	if isBacklogStage != nil {
		query = `UPDATE workflow_stages SET stage_name = ?, attrs = ?, is_backlog_stage = ?, updated_at = CURRENT_TIMESTAMP WHERE workflow_stage_id = ?`
		args = []any{name, attrsJSON, *isBacklogStage, stageID}
	}
	result, err := db.ExecContext(ctx, query, args...)
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
	if len(orderedStageIDs) == 0 {
		return tx.Commit()
	}
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE workflow_stages SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE workflow_stage_id = ? AND workflow_id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range orderedStageIDs {
		result, execErr := stmt.ExecContext(ctx, i, id, workflowID)
		if execErr != nil {
			return execErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if affected == 0 {
			return fmt.Errorf("%w %d in workflow %d", ErrWorkflowStageNotFound, id, workflowID)
		}
	}
	return tx.Commit()
}

func ensureWorkflowTransitionTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS workflow_stage_transitions (
			workflow_id INTEGER NOT NULL,
			from_stage_id INTEGER NOT NULL,
			to_stage_id INTEGER NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(workflow_id, from_stage_id, to_stage_id),
			FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE,
			FOREIGN KEY(from_stage_id) REFERENCES workflow_stages(workflow_stage_id) ON DELETE CASCADE,
			FOREIGN KEY(to_stage_id) REFERENCES workflow_stages(workflow_stage_id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_workflow_stage_transitions_from ON workflow_stage_transitions(from_stage_id);
		CREATE INDEX IF NOT EXISTS idx_workflow_stage_transitions_to ON workflow_stage_transitions(to_stage_id);
	`)
	return err
}

func ListWorkflowStageTransitions(ctx context.Context, db *sql.DB, workflowID int64, fromStageID *int64) ([]WorkflowStageTransition, error) {
	if err := ensureWorkflowTransitionTable(ctx, db); err != nil {
		return nil, err
	}
	query := `
		SELECT workflow_id, from_stage_id, to_stage_id, sort_order, created_at
		FROM workflow_stage_transitions
		WHERE workflow_id = ?
	`
	args := []any{workflowID}
	if fromStageID != nil {
		query += ` AND from_stage_id = ?`
		args = append(args, *fromStageID)
	}
	query += ` ORDER BY from_stage_id, sort_order, to_stage_id`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transitions := make([]WorkflowStageTransition, 0)
	for rows.Next() {
		var item WorkflowStageTransition
		if scanErr := rows.Scan(&item.WorkflowID, &item.FromStageID, &item.ToStageID, &item.SortOrder, &item.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		transitions = append(transitions, item)
	}
	return transitions, rows.Err()
}

func SetWorkflowStageTransitions(ctx context.Context, db *sql.DB, workflowID, fromStageID int64, toStageIDs []int64) error {
	if err := ensureWorkflowTransitionTable(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if scanErr := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM workflow_stages WHERE workflow_id = ? AND workflow_stage_id = ?`, workflowID, fromStageID).Scan(&count); scanErr != nil {
		return scanErr
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM workflow_stage_transitions WHERE workflow_id = ? AND from_stage_id = ?`, workflowID, fromStageID); execErr != nil {
		return execErr
	}
	seen := map[int64]bool{}
	filtered := make([]int64, 0, len(toStageIDs))
	for _, toStageID := range toStageIDs {
		if toStageID == 0 || seen[toStageID] {
			continue
		}
		seen[toStageID] = true
		filtered = append(filtered, toStageID)
	}
	if len(filtered) > 0 {
		rows, qErr := tx.QueryContext(ctx, `SELECT workflow_stage_id FROM workflow_stages WHERE workflow_id = ?`, workflowID)
		if qErr != nil {
			return qErr
		}
		known := make(map[int64]struct{}, len(filtered))
		for rows.Next() {
			var id int64
			if scanErr := rows.Scan(&id); scanErr != nil {
				_ = rows.Close()
				return scanErr
			}
			known[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		for _, id := range filtered {
			if _, ok := known[id]; !ok {
				return fmt.Errorf("%w %d in workflow %d", ErrWorkflowStageNotFound, id, workflowID)
			}
		}
	}
	for i, toStageID := range filtered {
		if _, execErr := tx.ExecContext(ctx, `
			INSERT INTO workflow_stage_transitions (workflow_id, from_stage_id, to_stage_id, sort_order)
			VALUES (?, ?, ?, ?)
		`, workflowID, fromStageID, toStageID, i); execErr != nil {
			return execErr
		}
	}
	if validateErr := validateWorkflowStageGraphTx(ctx, tx, workflowID); validateErr != nil {
		return validateErr
	}
	return tx.Commit()
}

func ExportWorkflow(ctx context.Context, db *sql.DB, id int64) (WorkflowExport, error) {
	wf, err := GetWorkflow(ctx, db, id)
	if err != nil {
		return WorkflowExport{}, err
	}
	export := WorkflowExport{
		Name:            wf.Name,
		Description:     wf.Description,
		ApprovalPolicy:  wf.ApprovalPolicy,
		ProgressionMode: wf.ProgressionMode,
		Stages:          make([]WorkflowStageExport, len(wf.Stages)),
	}
	stageNameByID := make(map[int64]string, len(wf.Stages))
	for _, s := range wf.Stages {
		stageNameByID[s.ID] = s.StageName
	}
	for i, s := range wf.Stages {
		var roleNames []string
		for _, r := range s.Roles {
			roleNames = append(roleNames, r.Title)
		}
		nextStages := make([]string, 0, len(s.NextStageIDs))
		for _, nextID := range s.NextStageIDs {
			if nextName, ok := stageNameByID[nextID]; ok && strings.TrimSpace(nextName) != "" {
				nextStages = append(nextStages, nextName)
			}
		}
		export.Stages[i] = WorkflowStageExport{
			StageName:   s.StageName,
			Description: s.Description,
			Roles:       roleNames,
			NextStages:  nextStages,
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
	if err := ensureWorkflowTransitionTable(ctx, db); err != nil {
		return Workflow{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Create the Workflow
	approvalPolicy, err := normalizeWorkflowApprovalPolicy(export.ApprovalPolicy)
	if err != nil {
		return Workflow{}, err
	}
	progressionMode, err := normalizeWorkflowProgressionMode(export.ProgressionMode)
	if err != nil {
		return Workflow{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO workflows (name, description, approval_policy, progression_mode, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, name, strings.TrimSpace(export.Description), approvalPolicy, progressionMode)
	if err != nil {
		return Workflow{}, err
	}
	workflowID, err := result.LastInsertId()
	if err != nil {
		return Workflow{}, err
	}

	stageIDByName := make(map[string]int64, len(export.Stages))
	type transitionSpec struct {
		from string
		to   []string
	}
	transitionSpecs := make([]transitionSpec, 0, len(export.Stages))
	// Create stages and assign roles
	for _, s := range export.Stages {
		stageName := strings.TrimSpace(s.StageName)
		if stageName == "" {
			return Workflow{}, errors.New("workflow stage name is required")
		}
		stageAttrsJSON, attrsErr := stageAttrsForWrite(nil, s.Description, "", "", "")
		if attrsErr != nil {
			return Workflow{}, attrsErr
		}
		stageResult, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_stages (workflow_id, stage_name, sort_order, attrs, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, workflowID, stageName, s.SortOrder, stageAttrsJSON)
		if err != nil {
			return Workflow{}, err
		}
		stageID, err := stageResult.LastInsertId()
		if err != nil {
			return Workflow{}, err
		}
		stageIDByName[strings.ToLower(stageName)] = stageID
		transitionSpecs = append(transitionSpecs, transitionSpec{
			from: stageName,
			to:   s.NextStages,
		})
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
	for _, spec := range transitionSpecs {
		fromID, ok := stageIDByName[strings.ToLower(strings.TrimSpace(spec.from))]
		if !ok {
			return Workflow{}, fmt.Errorf("stage %q not found while importing transitions", spec.from)
		}
		seen := map[int64]bool{}
		for i, toStageName := range spec.to {
			toID, ok := stageIDByName[strings.ToLower(strings.TrimSpace(toStageName))]
			if !ok {
				return Workflow{}, fmt.Errorf("target stage %q not found while importing transitions", toStageName)
			}
			if seen[toID] {
				continue
			}
			seen[toID] = true
			if _, execErr := tx.ExecContext(ctx, `
				INSERT INTO workflow_stage_transitions (workflow_id, from_stage_id, to_stage_id, sort_order)
				VALUES (?, ?, ?, ?)
			`, workflowID, fromID, toID, i); execErr != nil {
				return Workflow{}, execErr
			}
		}
	}
	if validateErr := validateWorkflowStageGraphTx(ctx, tx, workflowID); validateErr != nil {
		return Workflow{}, validateErr
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, err
	}
	return getWorkflowRow(ctx, db, workflowID)
}

func validateWorkflowStageGraphTx(ctx context.Context, tx *sql.Tx, workflowID int64) error {
	stageOrder, _, edges, err := loadWorkflowGraphTx(ctx, tx, workflowID)
	if err != nil {
		return err
	}
	if len(stageOrder) <= 1 {
		return nil
	}
	visiting := make(map[int64]bool, len(stageOrder))
	visited := make(map[int64]bool, len(stageOrder))
	var dfs func(int64) error
	dfs = func(stageID int64) error {
		if visiting[stageID] {
			return errors.New("workflow transitions contain a cycle")
		}
		if visited[stageID] {
			return nil
		}
		visiting[stageID] = true
		for _, nextID := range edges[stageID] {
			if err := dfs(nextID); err != nil {
				return err
			}
		}
		visiting[stageID] = false
		visited[stageID] = true
		return nil
	}
	for _, stageID := range stageOrder {
		if err := dfs(stageID); err != nil {
			return err
		}
	}
	return nil
}

func ValidateWorkflowGraph(ctx context.Context, db *sql.DB, workflowID int64) (WorkflowGraphValidation, error) {
	wf, err := GetWorkflow(ctx, db, workflowID)
	if err != nil {
		return WorkflowGraphValidation{}, err
	}
	report := WorkflowGraphValidation{
		WorkflowID: workflowID,
		StageCount: len(wf.Stages),
		Issues:     make([]string, 0),
		Warnings:   make([]string, 0),
	}
	if err := ensureWorkflowTransitionTable(ctx, db); err != nil {
		return WorkflowGraphValidation{}, err
	}
	edges := map[int64][]int64{}
	stageOrder := make([]int64, 0, len(wf.Stages))
	stageIndex := make(map[int64]int, len(wf.Stages))
	for idx, stage := range wf.Stages {
		stageOrder = append(stageOrder, stage.ID)
		stageIndex[stage.ID] = idx
		if len(stage.NextStageIDs) > 0 {
			edges[stage.ID] = append([]int64{}, stage.NextStageIDs...)
			report.TransitionCount += len(stage.NextStageIDs)
		}
	}
	for idx, stageID := range stageOrder {
		if len(edges[stageID]) == 0 && idx+1 < len(stageOrder) {
			edges[stageID] = []int64{stageOrder[idx+1]}
		}
		if len(edges[stageID]) == 0 {
			report.TerminalStageIDs = append(report.TerminalStageIDs, stageID)
		}
	}
	visiting := make(map[int64]bool, len(stageOrder))
	visited := make(map[int64]bool, len(stageOrder))
	var dfs func(int64) error
	dfs = func(stageID int64) error {
		if visiting[stageID] {
			return errors.New("workflow transitions contain a cycle")
		}
		if visited[stageID] {
			return nil
		}
		visiting[stageID] = true
		for _, nextID := range edges[stageID] {
			if _, ok := stageIndex[nextID]; !ok {
				return fmt.Errorf("workflow transitions reference unknown stage %d", nextID)
			}
			if err := dfs(nextID); err != nil {
				return err
			}
		}
		visiting[stageID] = false
		visited[stageID] = true
		return nil
	}
	for _, stageID := range stageOrder {
		if err := dfs(stageID); err != nil {
			report.Issues = append(report.Issues, err.Error())
			report.Valid = false
			return report, nil
		}
	}
	if len(stageOrder) > 0 {
		reachable := map[int64]bool{stageOrder[0]: true}
		queue := []int64{stageOrder[0]}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, nextID := range edges[current] {
				if !reachable[nextID] {
					reachable[nextID] = true
					queue = append(queue, nextID)
				}
			}
		}
		for _, stageID := range stageOrder {
			if !reachable[stageID] {
				report.UnreachableStageIDs = append(report.UnreachableStageIDs, stageID)
			}
		}
		if len(report.UnreachableStageIDs) > 0 {
			report.Warnings = append(report.Warnings, "workflow has stages unreachable from its first stage")
		}
	}
	if len(report.TerminalStageIDs) == 0 && len(stageOrder) > 0 {
		report.Warnings = append(report.Warnings, "workflow has no terminal stage")
	}
	report.Valid = len(report.Issues) == 0
	return report, nil
}

func loadWorkflowGraphTx(ctx context.Context, tx *sql.Tx, workflowID int64) (stageOrder []int64, indexByID map[int64]int, edges map[int64][]int64, err error) {
	stageRows, queryErr := tx.QueryContext(ctx, `
		SELECT workflow_stage_id
		FROM workflow_stages
		WHERE workflow_id = ?
		ORDER BY sort_order, workflow_stage_id
	`, workflowID)
	if queryErr != nil {
		return nil, nil, nil, queryErr
	}
	defer stageRows.Close()
	stageOrder = make([]int64, 0)
	for stageRows.Next() {
		var stageID int64
		if scanErr := stageRows.Scan(&stageID); scanErr != nil {
			return nil, nil, nil, scanErr
		}
		stageOrder = append(stageOrder, stageID)
	}
	if rowErr := stageRows.Err(); rowErr != nil {
		return nil, nil, nil, rowErr
	}
	if len(stageOrder) <= 1 {
		indexByID = make(map[int64]int, len(stageOrder))
		for idx, stageID := range stageOrder {
			indexByID[stageID] = idx
		}
		return stageOrder, indexByID, map[int64][]int64{}, nil
	}
	indexByID = make(map[int64]int, len(stageOrder))
	for idx, stageID := range stageOrder {
		indexByID[stageID] = idx
	}
	edges = make(map[int64][]int64, len(stageOrder))
	explicitRows, err := tx.QueryContext(ctx, `
		SELECT from_stage_id, to_stage_id
		FROM workflow_stage_transitions
		WHERE workflow_id = ?
		ORDER BY from_stage_id, sort_order, to_stage_id
	`, workflowID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer explicitRows.Close()
	for explicitRows.Next() {
		var fromID int64
		var toID int64
		if scanErr := explicitRows.Scan(&fromID, &toID); scanErr != nil {
			return nil, nil, nil, scanErr
		}
		edges[fromID] = append(edges[fromID], toID)
	}
	if rowErr := explicitRows.Err(); rowErr != nil {
		return nil, nil, nil, rowErr
	}
	for _, links := range edges {
		for _, nextID := range links {
			if _, ok := indexByID[nextID]; !ok {
				return nil, nil, nil, fmt.Errorf("workflow transitions reference unknown stage %d", nextID)
			}
		}
	}
	return stageOrder, indexByID, edges, nil
}

func normalizeWorkflowApprovalPolicy(raw string) (string, error) {
	policy := strings.TrimSpace(strings.ToLower(raw))
	if policy == "" {
		return WorkflowApprovalPolicySingleRole, nil
	}
	switch policy {
	case WorkflowApprovalPolicySingleRole, WorkflowApprovalPolicyAllRoles:
		return policy, nil
	default:
		return "", fmt.Errorf("invalid approval policy %q", raw)
	}
}

func normalizeWorkflowProgressionMode(raw string) (string, error) {
	mode := strings.TrimSpace(strings.ToLower(raw))
	if mode == "" {
		return WorkflowProgressionModeLinear, nil
	}
	switch mode {
	case WorkflowProgressionModeLinear, WorkflowProgressionModeStageOnly:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid progression mode %q", raw)
	}
}

// internal helpers

func getWorkflowRow(ctx context.Context, db *sql.DB, id int64) (Workflow, error) {
	row := db.QueryRowContext(ctx, `
		SELECT workflow_id, name, description, approval_policy, progression_mode, created_at, updated_at
		FROM workflows
		WHERE workflow_id = ?
	`, id)
	var w Workflow
	if err := row.Scan(&w.ID, &w.Name, &w.Description, &w.ApprovalPolicy, &w.ProgressionMode, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return Workflow{}, err
	}
	return w, nil
}

// hydrateStageAttrs copies the bag-backed guidance-text fields into typed
// WorkflowStage fields and strips them from the visible bag (TK-114).
func hydrateStageAttrs(s *WorkflowStage) {
	s.Description = s.Attrs.GetString("description")
	s.AcceptanceCriteria = s.Attrs.GetString("acceptance_criteria")
	s.DefinitionOfReady = s.Attrs.GetString("definition_of_ready")
	s.DefinitionOfDone = s.Attrs.GetString("definition_of_done")
	for _, k := range []string{"description", "acceptance_criteria", "definition_of_ready", "definition_of_done"} {
		delete(s.Attrs, k)
	}
	if len(s.Attrs) == 0 {
		s.Attrs = nil
	}
}

// stageAttrsForWrite folds the bag-backed guidance-text fields into a base bag.
func stageAttrsForWrite(base Attrs, description, acceptanceCriteria, definitionOfReady, definitionOfDone string) (string, error) {
	merged := Attrs{}
	for k, v := range base {
		merged[k] = v
	}
	merged.SetString("description", strings.TrimSpace(description))
	merged.SetString("acceptance_criteria", strings.TrimSpace(acceptanceCriteria))
	merged.SetString("definition_of_ready", strings.TrimSpace(definitionOfReady))
	merged.SetString("definition_of_done", strings.TrimSpace(definitionOfDone))
	return marshalAttrs(merged)
}

func getWorkflowStageRow(ctx context.Context, db *sql.DB, id int64) (WorkflowStage, error) {
	row := db.QueryRowContext(ctx, `
		SELECT workflow_stage_id, workflow_id, stage_name, sort_order, COALESCE(is_backlog_stage, 0), created_at, updated_at, attrs
		FROM workflow_stages
		WHERE workflow_stage_id = ?
	`, id)
	var s WorkflowStage
	var attrsJSON sql.NullString
	if err := row.Scan(&s.ID, &s.WorkflowID, &s.StageName,
		&s.SortOrder, &s.IsBacklogStage, &s.CreatedAt, &s.UpdatedAt, &attrsJSON); err != nil {
		return WorkflowStage{}, err
	}
	attrs, err := parseAttrs(attrsJSON.String)
	if err != nil {
		return WorkflowStage{}, err
	}
	s.Attrs = attrs
	hydrateStageAttrs(&s)
	roles, err := ListWorkflowStageRoles(ctx, db, s.WorkflowID, s.ID)
	if err != nil {
		return WorkflowStage{}, err
	}
	s.Roles = roles
	if transitions, transitionErr := ListWorkflowStageTransitions(ctx, db, s.WorkflowID, &s.ID); transitionErr == nil {
		s.NextStageIDs = make([]int64, 0, len(transitions))
		for _, transition := range transitions {
			s.NextStageIDs = append(s.NextStageIDs, transition.ToStageID)
		}
	}
	return s, nil
}

func listWorkflowStages(ctx context.Context, db *sql.DB, workflowID int64) ([]WorkflowStage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT workflow_stage_id, workflow_id, stage_name, sort_order, COALESCE(is_backlog_stage, 0), created_at, updated_at, attrs
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
		var attrsJSON sql.NullString
		if scanErr := rows.Scan(&s.ID, &s.WorkflowID, &s.StageName,
			&s.SortOrder, &s.IsBacklogStage, &s.CreatedAt, &s.UpdatedAt, &attrsJSON); scanErr != nil {
			return nil, scanErr
		}
		attrs, parseErr := parseAttrs(attrsJSON.String)
		if parseErr != nil {
			return nil, parseErr
		}
		s.Attrs = attrs
		hydrateStageAttrs(&s)
		stages = append(stages, s)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
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
	if transitions, transitionErr := ListWorkflowStageTransitions(ctx, db, workflowID, nil); transitionErr == nil {
		byStage := make(map[int64][]int64, len(stages))
		for _, transition := range transitions {
			byStage[transition.FromStageID] = append(byStage[transition.FromStageID], transition.ToStageID)
		}
		for i := range stages {
			stages[i].NextStageIDs = byStage[stages[i].ID]
		}
	}
	return stages, nil
}

// listWorkflowStageRolesBatch fetches all roles for every stage belonging to the
// given workflowID in a single query and returns them grouped by stage_id.
func listWorkflowStageRolesBatch(ctx context.Context, db *sql.DB, workflowID int64) (map[int64][]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT sr.stage_id, r.role_id, r.workflow_id, r.title, r.created_at, r.updated_at, r.attrs
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
