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

// Agent is a type alias for User. Agents are users with user_type='agent'.
type Agent = User

type AgentUpdateParams struct {
	Password *string
}

func CreateAgent(db *sql.DB, plainPassword string) (Agent, string, error) {
	uuid, err := generateAgentUUID()
	if err != nil {
		return Agent{}, "", err
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
	result, err := db.Exec(`
		INSERT INTO users (username, password_hash, role, display_name, enabled, user_type, uuid, status, updated_at)
		VALUES (?, ?, 'agent', ?, 1, 'agent', ?, 'idle', CURRENT_TIMESTAMP)
	`, uuid, hash, uuid, uuid)
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
		SELECT `+userSelectColumns+`
		FROM users
		WHERE user_id = ? AND user_type = 'agent'
	`, id)
	return scanUser(row.Scan)
}

func GetAgentByUUID(db *sql.DB, uuid string) (Agent, error) {
	row := db.QueryRow(`
		SELECT `+userSelectColumns+`
		FROM users
		WHERE uuid = ? AND user_type = 'agent'
	`, strings.TrimSpace(uuid))
	return scanUser(row.Scan)
}

func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`
		SELECT `+userSelectColumns+`
		FROM users
		WHERE user_type = 'agent'
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := make([]Agent, 0)
	for rows.Next() {
		agent, err := scanUser(rows.Scan)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func UpdateAgent(db *sql.DB, id int64, params AgentUpdateParams) (Agent, error) {
	if params.Password == nil {
		return Agent{}, errors.New("agent update requires -password")
	}
	if strings.TrimSpace(*params.Password) == "" {
		return Agent{}, errors.New("agent password cannot be empty")
	}
	hash, err := password.Hash(strings.TrimSpace(*params.Password))
	if err != nil {
		return Agent{}, err
	}
	_, err = db.Exec(`
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND user_type = 'agent'
	`, hash, id)
	if err != nil {
		return Agent{}, err
	}
	return GetAgentByID(db, id)
}

func DeleteAgent(db *sql.DB, id int64) error {
	// Delete sessions for this agent first
	if _, err := db.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return err
	}
	result, err := db.Exec(`DELETE FROM users WHERE user_id = ? AND user_type = 'agent'`, id)
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
		UPDATE users
		SET enabled = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND user_type = 'agent'
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

func AuthenticateAgent(db *sql.DB, agentID, plainPassword string) (Agent, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || strings.TrimSpace(plainPassword) == "" {
		return Agent{}, ErrInvalidCredentials
	}
	row := db.QueryRow(`
		SELECT `+userSelectColumns+`, password_hash
		FROM users
		WHERE uuid = ? AND user_type = 'agent'
	`, agentID)
	var a Agent
	var hash string
	var enabled int
	if err := row.Scan(
		&a.ID, &a.Username, &a.Email, &a.EmailConfirmedAt,
		&a.Role, &a.DisplayName, &enabled, &a.CreatedAt,
		&a.UserType, &a.UUID, &a.Description, &a.Status,
		&a.LastSeen, &a.UpdatedAt,
		&hash,
	); err != nil {
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
		UPDATE users
		SET status = ?, last_seen = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
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

// ReapStaleAgents sets any non-idle agent to "idle" if its last_seen is older
// than the given threshold (in minutes). Returns the number of agents reaped.
func ReapStaleAgents(db *sql.DB, thresholdMinutes int) (int64, error) {
	result, err := db.Exec(`
		UPDATE users
		SET status = 'idle', updated_at = CURRENT_TIMESTAMP
		WHERE user_type = 'agent' AND enabled = 1
		  AND status != 'idle' AND status != 'disabled'
		  AND last_seen IS NOT NULL
		  AND last_seen < datetime('now', ? || ' minutes')
	`, fmt.Sprintf("-%d", thresholdMinutes))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// AgentStatus holds an agent and its currently assigned ticket (if any).
type AgentStatus struct {
	Agent        Agent   `json:"agent"`
	TicketKey    *string `json:"ticket_key,omitempty"`
	ProjectName  string  `json:"project_name,omitempty"`
	WorkflowName string  `json:"workflow_name,omitempty"`
	RoleTitle    string  `json:"role_title,omitempty"`
}

// ListAgentStatuses returns all agents with their currently assigned active ticket.
func ListAgentStatuses(db *sql.DB) ([]AgentStatus, error) {
	agents, err := ListAgents(db)
	if err != nil {
		return nil, err
	}
	statuses := make([]AgentStatus, 0, len(agents))
	for _, a := range agents {
		as := AgentStatus{Agent: a}
		var ticketID int64
		var key string
		err := db.QueryRow(`
			SELECT t.ticket_id, t.key FROM tickets t
			WHERE t.assignee = ? AND t.state = 'active' AND t.open = 1
			LIMIT 1
		`, a.Username).Scan(&ticketID, &key)
		if err == nil {
			as.TicketKey = &key
			ticket, err := GetTicket(db, ticketID)
			if err == nil {
				ctx := EnrichTicketContext(db, ticket)
				if ctx.Project != nil {
					as.ProjectName = ctx.Project.Prefix
				}
				if ctx.Workflow != nil {
					as.WorkflowName = ctx.Workflow.Name
				}
				if ctx.Role != nil {
					as.RoleTitle = ctx.Role.Title
				}
			}
		}
		statuses = append(statuses, as)
	}
	return statuses, nil
}

// ─── agent config ─────────────────────────────────────────────────────────────

// Predefined agent config keys
const (
	AgentConfigKeyLLM         = "llm"
	AgentConfigKeyProjectID   = "project_id"
	AgentConfigKeyPollSeconds = "poll_seconds"
	AgentConfigKeyVerbose     = "verbose"
)

type AgentConfigEntry struct {
	UserID int64  `json:"user_id"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func SetAgentConfig(db *sql.DB, agentID int64, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("config key is required")
	}
	_, err := db.Exec(`
		INSERT INTO agent_config (user_id, key, value, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, agentID, key, value)
	return err
}

func ListAgentConfig(db *sql.DB, agentID int64) ([]AgentConfigEntry, error) {
	rows, err := db.Query(`SELECT user_id, key, value FROM agent_config WHERE user_id = ? ORDER BY key`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AgentConfigEntry
	for rows.Next() {
		var e AgentConfigEntry
		if err := rows.Scan(&e.UserID, &e.Key, &e.Value); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func DeleteAgentConfig(db *sql.DB, agentID int64, key string) error {
	result, err := db.Exec(`DELETE FROM agent_config WHERE user_id = ? AND key = ?`, agentID, strings.TrimSpace(key))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("config key not found")
	}
	return nil
}

// GetAgentConfigMap returns agent config as a map[string]string.
func GetAgentConfigMap(db *sql.DB, agentID int64) (map[string]string, error) {
	entries, err := ListAgentConfig(db, agentID)
	if err != nil {
		return nil, err
	}
	configMap := make(map[string]string, len(entries))
	for _, e := range entries {
		configMap[e.Key] = e.Value
	}
	return configMap, nil
}

// GetAgentConfigUpdatedAt returns the most recent updated_at timestamp from agent_config.
// Returns empty string if no config exists.
func GetAgentConfigUpdatedAt(db *sql.DB, agentID int64) (string, error) {
	var updatedAt sql.NullString
	err := db.QueryRow(`
		SELECT MAX(updated_at) FROM agent_config WHERE user_id = ?
	`, agentID).Scan(&updatedAt)
	if err != nil {
		return "", err
	}
	if !updatedAt.Valid {
		return "", nil
	}
	return updatedAt.String, nil
}
