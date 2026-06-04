package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrPasskeyNotFound     = errors.New("passkey not found")
	ErrPasskeyUnavailable  = errors.New("no passkeys enrolled for this user")
	ErrPasskeyFlowPending  = errors.New("passkey flow pending")
	ErrPasskeyFlowExpired  = errors.New("passkey flow expired")
	ErrPasskeyFlowConsumed = errors.New("passkey flow already consumed")
)

const (
	PasskeyFlowPurposeLogin        = "login"
	PasskeyFlowPurposeRegistration = "registration"
	PasskeyFlowStatusPending       = "pending"
	PasskeyFlowStatusComplete      = "complete"
)

type PasskeyCredential struct {
	CredentialID   string `json:"credential_id"`
	UserID         string `json:"user_id"`
	Name           string `json:"name"`
	CredentialJSON string `json:"credential_json,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	LastUsedAt     string `json:"last_used_at,omitempty"`
}

type PasskeyFlow struct {
	Code           string `json:"code"`
	Purpose        string `json:"purpose"`
	UserID         string `json:"user_id"`
	CredentialName string `json:"credential_name,omitempty"`
	SessionJSON    string `json:"session_json,omitempty"`
	OptionsJSON    string `json:"options_json,omitempty"`
	Status         string `json:"status"`
	Token          string `json:"token,omitempty"`
	ErrorMessage   string `json:"error,omitempty"`
	CreatedAt      string `json:"created_at"`
	ExpiresAt      string `json:"expires_at"`
	CompletedAt    string `json:"completed_at,omitempty"`
	ConsumedAt     string `json:"consumed_at,omitempty"`
}

func ListPasskeyCredentials(ctx context.Context, db *sql.DB, userID string) ([]PasskeyCredential, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT credential_id, user_id, name, credential_json, created_at, updated_at, COALESCE(last_used_at, '')
		FROM passkey_credentials
		WHERE user_id = ?
		ORDER BY created_at ASC, credential_id ASC
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	credentials := []PasskeyCredential{}
	for rows.Next() {
		var credential PasskeyCredential
		if err := rows.Scan(
			&credential.CredentialID,
			&credential.UserID,
			&credential.Name,
			&credential.CredentialJSON,
			&credential.CreatedAt,
			&credential.UpdatedAt,
			&credential.LastUsedAt,
		); err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return credentials, rows.Err()
}

func SavePasskeyCredential(ctx context.Context, db *sql.DB, userID, name, credentialID, credentialJSON string) error {
	userID = strings.TrimSpace(userID)
	credentialID = strings.TrimSpace(credentialID)
	credentialJSON = strings.TrimSpace(credentialJSON)
	if userID == "" || credentialID == "" || credentialJSON == "" {
		return errors.New("user id, credential id, and credential json are required")
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO passkey_credentials (credential_id, user_id, name, credential_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(credential_id) DO UPDATE SET
			user_id = excluded.user_id,
			name = excluded.name,
			credential_json = excluded.credential_json,
			updated_at = CURRENT_TIMESTAMP
	`, credentialID, userID, strings.TrimSpace(name), credentialJSON)
	return err
}

func UpdatePasskeyCredential(ctx context.Context, db *sql.DB, credentialID, credentialJSON string) error {
	credentialID = strings.TrimSpace(credentialID)
	credentialJSON = strings.TrimSpace(credentialJSON)
	if credentialID == "" || credentialJSON == "" {
		return errors.New("credential id and credential json are required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE passkey_credentials
		SET credential_json = ?, last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE credential_id = ?
	`, credentialJSON, credentialID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func RenamePasskeyCredential(ctx context.Context, db *sql.DB, userID, credentialID, name string) error {
	userID = strings.TrimSpace(userID)
	credentialID = strings.TrimSpace(credentialID)
	name = strings.TrimSpace(name)
	if userID == "" || credentialID == "" || name == "" {
		return errors.New("user id, credential id, and name are required")
	}
	result, err := db.ExecContext(ctx, `
		UPDATE passkey_credentials
		SET name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE credential_id = ? AND user_id = ?
	`, name, credentialID, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func DeletePasskeyCredential(ctx context.Context, db *sql.DB, userID, credentialID string) error {
	userID = strings.TrimSpace(userID)
	credentialID = strings.TrimSpace(credentialID)
	if userID == "" || credentialID == "" {
		return errors.New("user id and credential id are required")
	}
	result, err := db.ExecContext(ctx, `
		DELETE FROM passkey_credentials
		WHERE credential_id = ? AND user_id = ?
	`, credentialID, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func CreatePasskeyFlow(ctx context.Context, db *sql.DB, purpose, userID, credentialName, sessionJSON, optionsJSON string) (PasskeyFlow, error) {
	code, err := randomSecret(24)
	if err != nil {
		return PasskeyFlow{}, err
	}
	purpose = strings.TrimSpace(purpose)
	userID = strings.TrimSpace(userID)
	if purpose == "" || userID == "" {
		return PasskeyFlow{}, errors.New("purpose and user id are required")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO passkey_flows (
			flow_code, purpose, user_id, credential_name, session_json, options_json,
			status, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '+10 minutes'))
	`, code, purpose, userID, strings.TrimSpace(credentialName), strings.TrimSpace(sessionJSON), strings.TrimSpace(optionsJSON), PasskeyFlowStatusPending); err != nil {
		return PasskeyFlow{}, err
	}
	return GetPasskeyFlow(ctx, db, code)
}

func GetPasskeyFlow(ctx context.Context, db *sql.DB, code string) (PasskeyFlow, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return PasskeyFlow{}, ErrPasskeyNotFound
	}
	row := db.QueryRowContext(ctx, `
		SELECT flow_code, purpose, user_id, COALESCE(credential_name, ''), session_json, options_json,
		       status, COALESCE(token, ''), COALESCE(error_message, ''), created_at, expires_at,
		       COALESCE(completed_at, ''), COALESCE(consumed_at, '')
		FROM passkey_flows
		WHERE flow_code = ?
	`, code)
	var flow PasskeyFlow
	if err := row.Scan(
		&flow.Code,
		&flow.Purpose,
		&flow.UserID,
		&flow.CredentialName,
		&flow.SessionJSON,
		&flow.OptionsJSON,
		&flow.Status,
		&flow.Token,
		&flow.ErrorMessage,
		&flow.CreatedAt,
		&flow.ExpiresAt,
		&flow.CompletedAt,
		&flow.ConsumedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasskeyFlow{}, ErrPasskeyNotFound
		}
		return PasskeyFlow{}, err
	}
	var active int
	err := db.QueryRowContext(ctx, `SELECT 1 WHERE datetime(?) > CURRENT_TIMESTAMP`, flow.ExpiresAt).Scan(&active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasskeyFlow{}, ErrPasskeyFlowExpired
		}
		return PasskeyFlow{}, err
	}
	if strings.TrimSpace(flow.ConsumedAt) != "" {
		return PasskeyFlow{}, ErrPasskeyFlowConsumed
	}
	return flow, nil
}

func CompletePasskeyFlow(ctx context.Context, db *sql.DB, code, token string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrPasskeyNotFound
	}
	result, err := db.ExecContext(ctx, `
		UPDATE passkey_flows
		SET status = ?, token = ?, completed_at = CURRENT_TIMESTAMP, error_message = ''
		WHERE flow_code = ?
		  AND consumed_at IS NULL
		  AND datetime(expires_at) > CURRENT_TIMESTAMP
	`, PasskeyFlowStatusComplete, strings.TrimSpace(token), code)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		if _, lookupErr := GetPasskeyFlow(ctx, db, code); lookupErr != nil {
			return lookupErr
		}
	}
	return nil
}

func ConsumePasskeyFlow(ctx context.Context, db *sql.DB, code string) (PasskeyFlow, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return PasskeyFlow{}, err
	}
	defer conn.Close()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PasskeyFlow{}, err
	}
	rollback := func(cause error) (PasskeyFlow, error) {
		_ = tx.Rollback()
		return PasskeyFlow{}, cause
	}
	row := tx.QueryRowContext(ctx, `
		SELECT flow_code, purpose, user_id, COALESCE(credential_name, ''), session_json, options_json,
		       status, COALESCE(token, ''), COALESCE(error_message, ''), created_at, expires_at,
		       COALESCE(completed_at, ''), COALESCE(consumed_at, '')
		FROM passkey_flows
		WHERE flow_code = ?
	`, strings.TrimSpace(code))
	var flow PasskeyFlow
	if scanErr := row.Scan(
		&flow.Code,
		&flow.Purpose,
		&flow.UserID,
		&flow.CredentialName,
		&flow.SessionJSON,
		&flow.OptionsJSON,
		&flow.Status,
		&flow.Token,
		&flow.ErrorMessage,
		&flow.CreatedAt,
		&flow.ExpiresAt,
		&flow.CompletedAt,
		&flow.ConsumedAt,
	); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return rollback(ErrPasskeyNotFound)
		}
		return rollback(scanErr)
	}
	if strings.TrimSpace(flow.ConsumedAt) != "" {
		return rollback(ErrPasskeyFlowConsumed)
	}
	var active int
	err = tx.QueryRowContext(ctx, `SELECT 1 WHERE datetime(?) > CURRENT_TIMESTAMP`, flow.ExpiresAt).Scan(&active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rollback(ErrPasskeyFlowExpired)
		}
		return rollback(err)
	}
	if flow.Status != PasskeyFlowStatusComplete {
		return rollback(ErrPasskeyFlowPending)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE passkey_flows
		SET consumed_at = CURRENT_TIMESTAMP,
		    token = CASE WHEN purpose = ? THEN '' ELSE token END
		WHERE flow_code = ?
	`, PasskeyFlowPurposeLogin, flow.Code); err != nil {
		return rollback(err)
	}
	if err := tx.Commit(); err != nil {
		return PasskeyFlow{}, err
	}
	return flow, nil
}
