package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type WorkflowVersion struct {
	VersionID     int64  `json:"version_id"`
	WorkflowID    int64  `json:"workflow_id"`
	VersionNumber int    `json:"version_number"`
	SnapshotJSON  string `json:"snapshot_json,omitempty"`
	ChangeSummary string `json:"change_summary"`
	Approved      bool   `json:"approved"`
	Active        bool   `json:"active"`
	CreatedBy     string `json:"created_by,omitempty"`
	ApprovedBy    string `json:"approved_by,omitempty"`
	CreatedAt     string `json:"created_at"`
	ApprovedAt    string `json:"approved_at,omitempty"`
}

func ensureWorkflowVersionTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS workflow_versions (
			version_id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id INTEGER NOT NULL,
			version_number INTEGER NOT NULL,
			snapshot_json TEXT NOT NULL,
			change_summary TEXT NOT NULL,
			approved INTEGER NOT NULL DEFAULT 0,
			active INTEGER NOT NULL DEFAULT 0,
			created_by TEXT,
			approved_by TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			approved_at TEXT,
			FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE,
			UNIQUE(workflow_id, version_number)
		);
		CREATE INDEX IF NOT EXISTS idx_workflow_versions_workflow_id ON workflow_versions(workflow_id, version_number DESC);
	`)
	return err
}

func SaveWorkflowVersion(ctx context.Context, db *sql.DB, workflowID int64, changeSummary, actorID string) (WorkflowVersion, error) {
	if err := ensureWorkflowVersionTable(ctx, db); err != nil {
		return WorkflowVersion{}, err
	}
	export, err := ExportWorkflow(ctx, db, workflowID)
	if err != nil {
		return WorkflowVersion{}, err
	}
	snapshot, err := json.Marshal(export)
	if err != nil {
		return WorkflowVersion{}, err
	}
	versionNumber := 1
	if scanErr := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM workflow_versions
		WHERE workflow_id = ?
	`, workflowID).Scan(&versionNumber); scanErr != nil {
		return WorkflowVersion{}, scanErr
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO workflow_versions (workflow_id, version_number, snapshot_json, change_summary, created_by, active)
		VALUES (?, ?, ?, ?, ?, CASE WHEN ? = 1 THEN 1 ELSE 0 END)
	`, workflowID, versionNumber, string(snapshot), changeSummary, nullableUserID(actorID), versionNumber)
	if err != nil {
		return WorkflowVersion{}, err
	}
	versionID, err := result.LastInsertId()
	if err != nil {
		return WorkflowVersion{}, err
	}
	if versionNumber == 1 {
		if _, err := db.ExecContext(ctx, `
			UPDATE workflow_versions SET approved = 1, approved_by = ?, approved_at = CURRENT_TIMESTAMP
			WHERE version_id = ?
		`, nullableUserID(actorID), versionID); err != nil {
			return WorkflowVersion{}, err
		}
	}
	return GetWorkflowVersion(ctx, db, workflowID, versionID)
}

func GetWorkflowVersion(ctx context.Context, db *sql.DB, workflowID, versionID int64) (WorkflowVersion, error) {
	if err := ensureWorkflowVersionTable(ctx, db); err != nil {
		return WorkflowVersion{}, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT version_id, workflow_id, version_number, snapshot_json, change_summary, approved, active, COALESCE(created_by, ''), COALESCE(approved_by, ''), created_at, COALESCE(approved_at, '')
		FROM workflow_versions
		WHERE workflow_id = ? AND version_id = ?
	`, workflowID, versionID)
	var version WorkflowVersion
	var approvedInt int
	var activeInt int
	if err := row.Scan(&version.VersionID, &version.WorkflowID, &version.VersionNumber, &version.SnapshotJSON, &version.ChangeSummary, &approvedInt, &activeInt, &version.CreatedBy, &version.ApprovedBy, &version.CreatedAt, &version.ApprovedAt); err != nil {
		return WorkflowVersion{}, err
	}
	version.Approved = approvedInt == 1
	version.Active = activeInt == 1
	return version, nil
}

func ListWorkflowVersions(ctx context.Context, db *sql.DB, workflowID int64, limit int) ([]WorkflowVersion, error) {
	if err := ensureWorkflowVersionTable(ctx, db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, `
		SELECT version_id, workflow_id, version_number, snapshot_json, change_summary, approved, active, COALESCE(created_by, ''), COALESCE(approved_by, ''), created_at, COALESCE(approved_at, '')
		FROM workflow_versions
		WHERE workflow_id = ?
		ORDER BY version_number DESC
		LIMIT ?
	`, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]WorkflowVersion, 0)
	for rows.Next() {
		var version WorkflowVersion
		var approvedInt int
		var activeInt int
		if scanErr := rows.Scan(&version.VersionID, &version.WorkflowID, &version.VersionNumber, &version.SnapshotJSON, &version.ChangeSummary, &approvedInt, &activeInt, &version.CreatedBy, &version.ApprovedBy, &version.CreatedAt, &version.ApprovedAt); scanErr != nil {
			return nil, scanErr
		}
		version.Approved = approvedInt == 1
		version.Active = activeInt == 1
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func ApproveWorkflowVersion(ctx context.Context, db *sql.DB, workflowID, versionID int64, approverID string) (WorkflowVersion, error) {
	if err := ensureWorkflowVersionTable(ctx, db); err != nil {
		return WorkflowVersion{}, err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE workflow_versions
		SET approved = 1, approved_by = ?, approved_at = CURRENT_TIMESTAMP
		WHERE workflow_id = ? AND version_id = ?
	`, nullableUserID(approverID), workflowID, versionID)
	if err != nil {
		return WorkflowVersion{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return WorkflowVersion{}, err
	}
	if affected == 0 {
		return WorkflowVersion{}, sql.ErrNoRows
	}
	return GetWorkflowVersion(ctx, db, workflowID, versionID)
}

func ActivateWorkflowVersion(ctx context.Context, db *sql.DB, workflowID, versionID int64) (WorkflowVersion, error) {
	if err := ensureWorkflowVersionTable(ctx, db); err != nil {
		return WorkflowVersion{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return WorkflowVersion{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var approved int
	if scanErr := tx.QueryRowContext(ctx, `
		SELECT approved FROM workflow_versions WHERE workflow_id = ? AND version_id = ?
	`, workflowID, versionID).Scan(&approved); scanErr != nil {
		return WorkflowVersion{}, scanErr
	}
	if approved != 1 {
		return WorkflowVersion{}, fmt.Errorf("workflow version %d is not approved", versionID)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_versions SET active = 0 WHERE workflow_id = ?`, workflowID); err != nil {
		return WorkflowVersion{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflow_versions SET active = 1 WHERE workflow_id = ? AND version_id = ?`, workflowID, versionID); err != nil {
		return WorkflowVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkflowVersion{}, err
	}
	return GetWorkflowVersion(ctx, db, workflowID, versionID)
}
