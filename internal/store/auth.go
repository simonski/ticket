package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/simonski/ticket/internal/password"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrAdminRequired      = errors.New("user is not an admin")
)

type User struct {
	ID          int64  `json:"user_id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
}

func RegisterUser(db *sql.DB, username, plainPassword string) (User, error) {
	return createUser(db, username, plainPassword, "user", true)
}

func CreateUser(db *sql.DB, username, plainPassword, role string) (User, error) {
	if role == "" {
		role = "user"
	}
	return createUser(db, username, plainPassword, role, true)
}

func createUser(db *sql.DB, username, plainPassword, role string, enabled bool) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || plainPassword == "" {
		return User{}, errors.New("username and password are required")
	}

	hash, err := password.Hash(plainPassword)
	if err != nil {
		return User{}, err
	}

	result, err := db.Exec(`
		INSERT INTO users (username, password_hash, role, display_name, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, username, hash, role, username, boolToInt(enabled))
	if err != nil {
		return User{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return GetUserByID(db, id)
}

func AuthenticateUser(db *sql.DB, username, plainPassword string) (User, error) {
	row := db.QueryRow(`
		SELECT user_id, username, password_hash, role, display_name, enabled, created_at
		FROM users
		WHERE username = ?
	`, username)

	var user User
	var hash string
	var enabled int
	if err := row.Scan(&user.ID, &user.Username, &hash, &user.Role, &user.DisplayName, &enabled, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, err
	}
	user.Enabled = enabled == 1
	if !user.Enabled {
		return User{}, ErrForbidden
	}

	ok, err := password.Verify(hash, plainPassword)
	if err != nil {
		return User{}, err
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func CreateSession(db *sql.DB, userID int64) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("create session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	if _, err := db.Exec(`INSERT INTO sessions (user_id, token) VALUES (?, ?)`, userID, token); err != nil {
		return "", err
	}
	return token, nil
}

func DeleteSession(db *sql.DB, token string) error {
	if token == "" {
		return nil
	}
	_, err := db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func GetUserByToken(db *sql.DB, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}

	row := db.QueryRow(`
		SELECT u.user_id, u.username, u.role, u.display_name, u.enabled, u.created_at
		FROM sessions s
		JOIN users u ON u.user_id = s.user_id
		WHERE s.token = ?
	`, token)

	var user User
	var enabled int
	if err := row.Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUnauthorized
		}
		return User{}, err
	}
	user.Enabled = enabled == 1
	if !user.Enabled {
		return User{}, ErrForbidden
	}
	return user, nil
}

func GetUserByID(db *sql.DB, id int64) (User, error) {
	row := db.QueryRow(`
		SELECT user_id, username, role, display_name, enabled, created_at
		FROM users
		WHERE user_id = ?
	`, id)

	var user User
	var enabled int
	if err := row.Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.CreatedAt); err != nil {
		return User{}, err
	}
	user.Enabled = enabled == 1
	return user, nil
}

func GetUserByUsername(db *sql.DB, username string) (User, error) {
	row := db.QueryRow(`
		SELECT user_id, username, role, display_name, enabled, created_at
		FROM users
		WHERE username = ?
	`, strings.TrimSpace(username))

	var user User
	var enabled int
	if err := row.Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.CreatedAt); err != nil {
		return User{}, err
	}
	user.Enabled = enabled == 1
	return user, nil
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`
		SELECT user_id, username, role, display_name, enabled, created_at
		FROM users
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		var enabled int
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.DisplayName, &enabled, &user.CreatedAt); err != nil {
			return nil, err
		}
		user.Enabled = enabled == 1
		users = append(users, user)
	}
	return users, rows.Err()
}

func SetUserEnabled(db *sql.DB, username string, enabled bool) error {
	result, err := db.Exec(`UPDATE users SET enabled = ? WHERE username = ?`, boolToInt(enabled), username)
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

func DeleteUser(db *sql.DB, username string) error {
	result, err := db.Exec(`DELETE FROM sessions WHERE user_id IN (SELECT user_id FROM users WHERE username = ?)`, username)
	if err != nil {
		return err
	}
	_ = result

	result, err = db.Exec(`DELETE FROM users WHERE username = ?`, username)
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
