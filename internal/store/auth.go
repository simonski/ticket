package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/simonski/ticket/internal/password"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrAdminRequired      = errors.New("user is not an admin")
	ErrAccountLocked      = errors.New("account locked, try again later")

	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

const (
	maxFailedLoginAttempts = 10
	lockoutDuration        = "+15 minutes"
)

type User struct {
	ID               string `json:"user_id"`
	Username         string `json:"username"`
	Email            string `json:"email"`
	EmailConfirmedAt string `json:"email_confirmed_at,omitempty"`
	Role             string `json:"role"`
	PlanID           int64  `json:"plan_id,omitempty"`
	PlanSlug         string `json:"plan_slug,omitempty"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        string `json:"created_at"`
	UserType         string `json:"user_type"`
	Description      string `json:"description,omitempty"`
	Status           string `json:"status,omitempty"`
	LastSeen         string `json:"last_seen,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// userSelectColumns is the standard column list for scanning a User.
const userSelectColumns = `user_id, username, COALESCE(email, ''), COALESCE(email_confirmed_at, ''), role, display_name, enabled, created_at, COALESCE(user_type, 'user'), COALESCE(description, ''), COALESCE(status, ''), COALESCE(last_seen, ''), COALESCE(updated_at, '')`

// scanUser scans a row into a User. The column order must match userSelectColumns.
func scanUser(scan func(dest ...any) error) (User, error) {
	var user User
	var enabled int
	if err := scan(
		&user.ID, &user.Username, &user.Email, &user.EmailConfirmedAt,
		&user.Role, &user.DisplayName, &enabled, &user.CreatedAt,
		&user.UserType, &user.Description, &user.Status,
		&user.LastSeen, &user.UpdatedAt,
	); err != nil {
		return User{}, err
	}
	user.Enabled = enabled == 1
	return user, nil
}

// generateUserID generates a UUID v4 string for use as a user ID.
func generateUserID() string {
	return uuid.NewString()
}

type UserCreateParams struct {
	Username               string
	PlainPassword          string
	Email                  string
	Role                   string
	Enabled                bool
	PlanSlug               string
	SkipProvisioning       bool
	SkipPasswordValidation bool
}

func normalizeUserEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" {
		return "", nil
	}
	if strings.ContainsAny(email, " \t\r\n") {
		return "", errors.New("email must not contain spaces")
	}
	at := strings.Index(email, "@")
	if at <= 0 || at != strings.LastIndex(email, "@") || at == len(email)-1 {
		return "", errors.New("email must be valid")
	}
	return email, nil
}

func RegisterUser(ctx context.Context, db *sql.DB, username, plainPassword string) (User, error) {
	return CreateUserWithParams(ctx, db, UserCreateParams{
		Username:      username,
		PlainPassword: plainPassword,
		Role:          "user",
		Enabled:       true,
	})
}

func GeneratePassword(length int) (string, error) {
	return randomSecret(length)
}

func CreateUser(ctx context.Context, db *sql.DB, username, plainPassword, role string) (User, error) {
	return CreateUserWithParams(ctx, db, UserCreateParams{
		Username:      username,
		PlainPassword: plainPassword,
		Role:          role,
		Enabled:       true,
	})
}

func CreateUserWithParams(ctx context.Context, db *sql.DB, params UserCreateParams) (User, error) {
	username := strings.TrimSpace(params.Username)
	plainPassword := params.PlainPassword
	email, err := normalizeUserEmail(params.Email)
	if err != nil {
		return User{}, err
	}
	role := strings.TrimSpace(params.Role)
	if role == "" {
		role = "user"
	}
	if username == "" || plainPassword == "" {
		return User{}, errors.New("username and password are required")
	}
	if !usernamePattern.MatchString(username) {
		return User{}, errors.New("username contains invalid characters")
	}
	if !params.SkipPasswordValidation && len(plainPassword) < 8 {
		return User{}, errors.New("password must be at least 8 characters")
	}

	hash, err := password.Hash(plainPassword)
	if err != nil {
		return User{}, err
	}

	id := generateUserID()

	plan, err := DefaultPlan(ctx, db)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(params.PlanSlug) != "" {
		plan, err = GetPlanBySlug(ctx, db, params.PlanSlug)
		if err != nil {
			return User{}, err
		}
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (user_id, username, email, password_hash, role, plan_id, display_name, enabled, user_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'user')
	`, id, username, email, hash, role, plan.ID, username, boolToInt(params.Enabled))
	if err != nil {
		return User{}, err
	}

	user, err := getUserByIDQuerier(ctx, db, id)
	if err != nil {
		return User{}, err
	}
	if !params.SkipProvisioning {
		if err := ensurePersonalResources(ctx, db, user, plan); err != nil {
			return User{}, err
		}
	}
	return GetUserByID(ctx, db, id)
}

func AuthenticateUser(ctx context.Context, db *sql.DB, username, plainPassword string) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`, password_hash,
		       COALESCE(failed_login_attempts, 0),
		       COALESCE(locked_until, '')
		FROM users
		WHERE username = ?
	`, username)

	var user User
	var hash string
	var enabled int
	var failedAttempts int
	var lockedUntil string
	if err := row.Scan(
		&user.ID, &user.Username, &user.Email, &user.EmailConfirmedAt,
		&user.Role, &user.DisplayName, &enabled, &user.CreatedAt,
		&user.UserType, &user.Description, &user.Status,
		&user.LastSeen, &user.UpdatedAt,
		&hash, &failedAttempts, &lockedUntil,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, err
	}
	user.Enabled = enabled == 1
	if !user.Enabled {
		return User{}, ErrForbidden
	}

	// Check account lockout.
	if lockedUntil != "" {
		var locked int
		err := db.QueryRowContext(ctx, `SELECT 1 WHERE datetime(?) > datetime('now')`, lockedUntil).Scan(&locked)
		if err == nil {
			// locked_until is in the future
			return User{}, ErrAccountLocked
		}
		// Lock period expired — reset for a clean slate.
		if _, execErr := db.ExecContext(ctx, `UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE user_id = ?`, user.ID); execErr != nil {
			log.Printf("store: reset expired lockout for user %s: %v", user.ID, execErr)
		}
		failedAttempts = 0
	}

	ok, err := password.Verify(hash, plainPassword)
	if err != nil {
		return User{}, err
	}
	if !ok {
		failedAttempts++
		if failedAttempts >= maxFailedLoginAttempts {
			if _, execErr := db.ExecContext(ctx, `UPDATE users SET failed_login_attempts = ?, locked_until = datetime('now', ?) WHERE user_id = ?`,
				failedAttempts, lockoutDuration, user.ID); execErr != nil {
				log.Printf("store: persist lockout for user %s: %v", user.ID, execErr)
			}
		} else {
			if _, execErr := db.ExecContext(ctx, `UPDATE users SET failed_login_attempts = ? WHERE user_id = ?`,
				failedAttempts, user.ID); execErr != nil {
				log.Printf("store: persist failed login attempt for user %s: %v", user.ID, execErr)
			}
		}
		return User{}, ErrInvalidCredentials
	}

	// Successful authentication — reset failed attempts.
	if failedAttempts > 0 {
		if _, execErr := db.ExecContext(ctx, `UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE user_id = ?`, user.ID); execErr != nil {
			log.Printf("store: reset failed login attempts for user %s: %v", user.ID, execErr)
		}
	}
	return user, nil
}

func CreateSession(ctx context.Context, db *sql.DB, userID string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("create session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	expiryDays := 30
	if v := os.Getenv("TICKET_SESSION_EXPIRY_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			expiryDays = n
		}
	}
	expiryModifier := fmt.Sprintf("+%d days", expiryDays)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES (?, ?, datetime('now', ?))
	`, userID, token, expiryModifier); err != nil {
		return "", err
	}
	return token, nil
}

func DeleteSession(ctx context.Context, db *sql.DB, token string) error {
	if token == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func GetUserByToken(ctx context.Context, db *sql.DB, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}

	row := db.QueryRowContext(ctx, `
		SELECT u.user_id, u.username, COALESCE(u.email, ''), COALESCE(u.email_confirmed_at, ''), u.role, u.display_name, u.enabled, u.created_at, COALESCE(u.user_type, 'user'), COALESCE(u.description, ''), COALESCE(u.status, ''), COALESCE(u.last_seen, ''), COALESCE(u.updated_at, '')
		FROM sessions s
		JOIN users u ON u.user_id = s.user_id
		WHERE s.token = ?
		  AND (s.expires_at IS NULL OR s.expires_at > CURRENT_TIMESTAMP)
	`, token)

	user, err := scanUser(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUnauthorized
		}
		return User{}, err
	}
	if !user.Enabled {
		return User{}, ErrForbidden
	}
	return user, nil
}

func GetUserByID(ctx context.Context, db *sql.DB, id string) (User, error) {
	return getUserByIDQuerier(ctx, db, id)
}

func getUserByIDQuerier(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (User, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE user_id = ?
	`, id)

	return scanUser(row.Scan)
}

func GetUserByUsername(ctx context.Context, db *sql.DB, username string) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE username = ?
	`, strings.TrimSpace(username))

	return scanUser(row.Scan)
}

func ListUsers(ctx context.Context, db *sql.DB, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.QueryContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE user_type = 'user' OR user_type = '' OR user_type IS NULL
		ORDER BY username
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		user, err := scanUser(rows.Scan)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func SetUserEnabled(ctx context.Context, db *sql.DB, username string, enabled bool) error {
	result, err := db.ExecContext(ctx, `UPDATE users SET enabled = ? WHERE username = ?`, boolToInt(enabled), username)
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

func DeleteUser(ctx context.Context, db *sql.DB, username string) error {
	// Fetch the user_id before deleting so we can cascade cleanup.
	var userID string
	err := db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE username = ?`, username).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // best-effort rollback; tx may already be committed

	// Anonymise audit trail records so history is preserved without PII.
	// history_events.created_by and ticket_history.created_by are nullable.
	if _, execErr := tx.ExecContext(ctx, `UPDATE history_events SET created_by = NULL WHERE created_by = ?`, userID); execErr != nil {
		return execErr
	}
	if _, execErr := tx.ExecContext(ctx, `UPDATE ticket_history SET created_by = NULL WHERE created_by = ?`, userID); execErr != nil {
		return execErr
	} // Nullify ticket creator reference (nullable FK).
	if _, execErr := tx.ExecContext(ctx, `UPDATE tickets SET created_by = NULL WHERE created_by = ?`, userID); execErr != nil {
		return execErr
	}
	if _, execErr := tx.ExecContext(ctx, `UPDATE projects SET created_by = NULL WHERE created_by = ?`, userID); execErr != nil {
		return execErr
	}
	// Clear free-text assignee field that stores the username.
	if _, execErr := tx.ExecContext(ctx, `UPDATE tickets SET assignee = '' WHERE assignee = (SELECT username FROM users WHERE user_id = ?)`, userID); execErr != nil {
		return execErr
	}
	// Remove personal data: sessions, memberships, time entries, messages, agent config.
	tables := []struct {
		table  string
		column string
	}{
		{"sessions", "user_id"},
		{"project_members", "user_id"},
		{"project_aliases", "user_id"},
		{"team_members", "user_id"},
		{"team_agents", "user_id"},
		{"time_entries", "user_id"},
		{"agent_config", "user_id"},
	}
	for _, t := range tables {
		if _, execErr := tx.ExecContext(ctx, `DELETE FROM `+t.table+` WHERE `+t.column+` = ?`, userID); execErr != nil { // #nosec G202 -- table/column names come from a hardcoded internal list
			return execErr
		}
	}
	// Delete messages to/from the user.
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM messages WHERE from_user_id = ? OR to_user_id = ?`, userID, userID); execErr != nil {
		return execErr
	}
	// Delete comments authored by the user (personal content, not anonymisable
	// because user_id is NOT NULL and the FK references users).
	if _, execErr := tx.ExecContext(ctx, `DELETE FROM comments WHERE user_id = ?`, userID); execErr != nil {
		return execErr
	}
	// Finally remove the user record.
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE user_id = ?`, userID)
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

// ResetUserPassword changes the password and invalidates all sessions.
func ResetUserPassword(ctx context.Context, db *sql.DB, username, newPlainPassword string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, errors.New("username is required")
	}
	if strings.TrimSpace(newPlainPassword) == "" {
		return User{}, errors.New("password cannot be empty")
	}
	if len(newPlainPassword) < 8 {
		return User{}, errors.New("password must be at least 8 characters")
	}
	hash, err := password.Hash(newPlainPassword)
	if err != nil {
		return User{}, err
	}
	result, err := db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE username = ?`, hash, username)
	if err != nil {
		return User{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}
	// Invalidate all sessions for this user
	if _, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id IN (SELECT user_id FROM users WHERE username = ?)`, username); err != nil {
		return User{}, err
	}
	return GetUserByUsername(ctx, db, username)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
