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

type Agent struct {
	ID          int64  `json:"agent_id"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	LastSeen    string `json:"last_seen"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AgentUpdateParams struct {
	Name        *string
	Description *string
	Password    *string
}

func CreateAgent(db *sql.DB, name, description, plainPassword string) (Agent, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return Agent{}, "", errors.New("agent name is required")
	}
	passwordToSet := strings.TrimSpace(plainPassword)
	if passwordToSet == "" {
		var err error
		passwordToSet, err = randomSecret(24)
		if err != nil {
			return Agent{}, "", err
		}
	}
	hash, err := password.Hash(passwordToSet)
	if err != nil {
		return Agent{}, "", err
	}
	uuid, err := generateAgentUUID()
	if err != nil {
		return Agent{}, "", err
	}
	result, err := db.Exec(`
		INSERT INTO agents (uuid, name, description, password_hash, enabled, status, updated_at)
		VALUES (?, ?, ?, ?, 1, 'idle', CURRENT_TIMESTAMP)
	`, uuid, name, description, hash)
	if err != nil {
		return Agent{}, "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Agent{}, "", err
	}
	agent, err := GetAgentByID(db, id)
	if err != nil {
		return Agent{}, "", err
	}
	return agent, passwordToSet, nil
}

func GetAgentByID(db *sql.DB, id int64) (Agent, error) {
	row := db.QueryRow(`
		SELECT agent_id, COALESCE(uuid, ''), name, description, enabled, status, last_seen, created_at, updated_at
		FROM agents
		WHERE agent_id = ?
	`, id)
	return scanAgent(row)
}

func GetAgentByName(db *sql.DB, name string) (Agent, error) {
	row := db.QueryRow(`
		SELECT agent_id, COALESCE(uuid, ''), name, description, enabled, status, last_seen, created_at, updated_at
		FROM agents
		WHERE name = ?
	`, strings.TrimSpace(name))
	return scanAgent(row)
}

func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`
		SELECT agent_id, COALESCE(uuid, ''), name, description, enabled, status, last_seen, created_at, updated_at
		FROM agents
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := make([]Agent, 0)
	for rows.Next() {
		var a Agent
		var enabled int
		if err := rows.Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &enabled, &a.Status, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Enabled = enabled == 1
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func UpdateAgent(db *sql.DB, id int64, params AgentUpdateParams) (Agent, error) {
	current, err := GetAgentByID(db, id)
	if err != nil {
		return Agent{}, err
	}
	name := current.Name
	description := current.Description
	passwordHash := ""
	if params.Name != nil {
		name = strings.TrimSpace(*params.Name)
	}
	if params.Description != nil {
		description = strings.TrimSpace(*params.Description)
	}
	if strings.TrimSpace(name) == "" {
		return Agent{}, errors.New("agent name is required")
	}
	if params.Password != nil {
		if strings.TrimSpace(*params.Password) == "" {
			return Agent{}, errors.New("agent password cannot be empty")
		}
		hash, err := password.Hash(strings.TrimSpace(*params.Password))
		if err != nil {
			return Agent{}, err
		}
		passwordHash = hash
	}
	if passwordHash != "" {
		_, err = db.Exec(`
			UPDATE agents
			SET name = ?, description = ?, password_hash = ?, updated_at = CURRENT_TIMESTAMP
			WHERE agent_id = ?
		`, name, description, passwordHash, id)
	} else {
		_, err = db.Exec(`
			UPDATE agents
			SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
			WHERE agent_id = ?
		`, name, description, id)
	}
	if err != nil {
		return Agent{}, err
	}
	return GetAgentByID(db, id)
}

func DeleteAgent(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM agents WHERE agent_id = ?`, id)
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

func SetAgentEnabled(db *sql.DB, id int64, enabled bool) (Agent, error) {
	status := "disabled"
	if enabled {
		status = "idle"
	}
	result, err := db.Exec(`
		UPDATE agents
		SET enabled = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE agent_id = ?
	`, boolToInt(enabled), status, id)
	if err != nil {
		return Agent{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Agent{}, err
	}
	if affected == 0 {
		return Agent{}, sql.ErrNoRows
	}
	return GetAgentByID(db, id)
}

func AuthenticateAgent(db *sql.DB, name, plainPassword string) (Agent, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(plainPassword) == "" {
		return Agent{}, ErrInvalidCredentials
	}
	row := db.QueryRow(`
		SELECT agent_id, COALESCE(uuid, ''), name, description, password_hash, enabled, status, last_seen, created_at, updated_at
		FROM agents
		WHERE name = ?
	`, name)
	var a Agent
	var hash string
	var enabled int
	if err := row.Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &hash, &enabled, &a.Status, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Agent{}, ErrInvalidCredentials
		}
		return Agent{}, err
	}
	a.Enabled = enabled == 1
	if !a.Enabled {
		return Agent{}, ErrForbidden
	}
	ok, err := password.Verify(hash, strings.TrimSpace(plainPassword))
	if err != nil {
		return Agent{}, err
	}
	if !ok {
		return Agent{}, ErrInvalidCredentials
	}
	return a, nil
}

func TouchAgent(db *sql.DB, id int64, status string) (Agent, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "idle"
	}
	result, err := db.Exec(`
		UPDATE agents
		SET status = ?, last_seen = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE agent_id = ?
	`, status, id)
	if err != nil {
		return Agent{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Agent{}, err
	}
	if affected == 0 {
		return Agent{}, sql.ErrNoRows
	}
	return GetAgentByID(db, id)
}

func scanAgent(row *sql.Row) (Agent, error) {
	var a Agent
	var enabled int
	if err := row.Scan(&a.ID, &a.UUID, &a.Name, &a.Description, &enabled, &a.Status, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return Agent{}, err
	}
	a.Enabled = enabled == 1
	return a, nil
}

func generateAgentUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version 4 and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func randomSecret(n int) (string, error) {
	if n < 12 {
		n = 12
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ─── agent config ─────────────────────────────────────────────────────────────

type AgentConfigEntry struct {
	AgentID int64  `json:"agent_id"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

func SetAgentConfig(db *sql.DB, agentID int64, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("config key is required")
	}
	_, err := db.Exec(`
		INSERT INTO agent_config (agent_id, key, value, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(agent_id, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, agentID, key, value)
	return err
}

func ListAgentConfig(db *sql.DB, agentID int64) ([]AgentConfigEntry, error) {
	rows, err := db.Query(`SELECT agent_id, key, value FROM agent_config WHERE agent_id = ? ORDER BY key`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AgentConfigEntry
	for rows.Next() {
		var e AgentConfigEntry
		if err := rows.Scan(&e.AgentID, &e.Key, &e.Value); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func DeleteAgentConfig(db *sql.DB, agentID int64, key string) error {
	result, err := db.Exec(`DELETE FROM agent_config WHERE agent_id = ? AND key = ?`, agentID, strings.TrimSpace(key))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("config key not found")
	}
	return nil
}
