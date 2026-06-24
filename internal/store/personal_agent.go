package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// EnsurePersonalAgent returns the caller's personal agent — creating one owned by
// them on first use — and the DM room between the two (TK-142). The agent is an
// ordinary agent user with owner_id set; the DM (slug dm-*) surfaces under the
// "People & Agents" group. Humans get an agent; agents do not.
func EnsurePersonalAgent(ctx context.Context, db *sql.DB, owner User) (Agent, Room, error) {
	if owner.UserType == "agent" {
		return Agent{}, Room{}, fmt.Errorf("agents do not have a personal agent")
	}
	agentID, err := personalAgentID(ctx, db, owner.ID)
	if err != nil {
		return Agent{}, Room{}, err
	}
	if agentID == "" {
		created, _, cerr := CreateAgent(ctx, db, "")
		if cerr != nil {
			return Agent{}, Room{}, cerr
		}
		uname := personalAgentUsername(owner.Username)
		role := "assistant"
		if _, uerr := UpdateAgent(ctx, db, created.ID, AgentUpdateParams{Username: &uname, AgentRole: &role}); uerr != nil {
			return Agent{}, Room{}, uerr
		}
		display := owner.Username + "'s agent"
		if _, derr := db.ExecContext(ctx, `UPDATE users SET owner_id = ?, display_name = ? WHERE user_id = ?`, owner.ID, display, created.ID); derr != nil {
			return Agent{}, Room{}, derr
		}
		agentID = created.ID
	}
	agentUser, gerr := GetUserByID(ctx, db, agentID)
	if gerr != nil {
		return Agent{}, Room{}, gerr
	}
	dm, derr := FindOrCreateDMRoom(ctx, db, owner, agentUser)
	if derr != nil {
		return Agent{}, Room{}, derr
	}
	return agentUser, dm, nil
}

func personalAgentID(ctx context.Context, db *sql.DB, ownerID string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE user_type = 'agent' AND owner_id = ? AND enabled = 1 ORDER BY created_at LIMIT 1`, strings.TrimSpace(ownerID)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func personalAgentUsername(owner string) string {
	base := strings.ToLower(strings.TrimSpace(owner))
	if base == "" {
		base = "user"
	}
	return base + "-assistant"
}
