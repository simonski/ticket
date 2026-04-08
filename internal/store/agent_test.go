package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openAgentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return db
}

func TestCreateAgentGeneratesPasswordAndUUID(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, generatedPassword, err := CreateAgent(context.Background(), db, "")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if agent.ID == "" {
		t.Fatalf("agent ID empty, want non-empty")
	}
	// Username should be the ID (UUID)
	if agent.Username != agent.ID {
		t.Fatalf("agent.Username = %q, want ID %q", agent.Username, agent.ID)
	}
	if generatedPassword == "" {
		t.Fatal("generatedPassword empty, want generated secret")
	}
	if !agent.Enabled {
		t.Fatal("agent enabled = false, want true")
	}
	if agent.Status != "idle" {
		t.Fatalf("agent status = %q, want idle", agent.Status)
	}
	if agent.UserType != "agent" {
		t.Fatalf("agent user_type = %q, want agent", agent.UserType)
	}

	// Authenticate by UUID
	authenticated, err := AuthenticateAgent(context.Background(), db, agent.ID, generatedPassword)
	if err != nil {
		t.Fatalf("AuthenticateAgent() error = %v", err)
	}
	if authenticated.ID != agent.ID {
		t.Fatalf("authenticated ID = %s, want %s", authenticated.ID, agent.ID)
	}
}

func TestAgentUpdateAndLifecycle(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, password, err := CreateAgent(context.Background(), db, "secret-1")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if password != "secret-1" {
		t.Fatalf("password = %q, want secret-1", password)
	}

	updatedPassword := "secret-2"
	_, err = UpdateAgent(context.Background(), db, agent.ID, AgentUpdateParams{
		Password: &updatedPassword,
	})
	if err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	if _, err := AuthenticateAgent(context.Background(), db, agent.ID, "secret-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("AuthenticateAgent(old password) err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := AuthenticateAgent(context.Background(), db, agent.ID, "secret-2"); err != nil {
		t.Fatalf("AuthenticateAgent(new password) error = %v", err)
	}

	disabled, err := SetAgentEnabled(context.Background(), db, agent.ID, false)
	if err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled.Enabled = true, want false")
	}
	if _, err := AuthenticateAgent(context.Background(), db, agent.ID, "secret-2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthenticateAgent(disabled) err = %v, want ErrForbidden", err)
	}

	enabled, err := SetAgentEnabled(context.Background(), db, agent.ID, true)
	if err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatalf("enabled.Enabled = false, want true")
	}
	touched, err := TouchAgent(context.Background(), db, agent.ID, "working")
	if err != nil {
		t.Fatalf("TouchAgent() error = %v", err)
	}
	if touched.Status != "working" {
		t.Fatalf("touched.Status = %q, want working", touched.Status)
	}
	if touched.LastSeen == "" {
		t.Fatalf("touched.LastSeen empty, want timestamp")
	}

	if err := DeleteAgent(context.Background(), db, agent.ID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	if _, err := GetAgentByID(context.Background(), db, agent.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetAgentByID(deleted) err = %v, want sql.ErrNoRows", err)
	}
}

func TestAgentDoesNotAppearInListUsers(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	users, err := ListUsers(context.Background(), db)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	for _, u := range users {
		if u.Username == agent.ID {
			t.Fatalf("ListUsers() should not include agents, found %q", u.Username)
		}
	}
}

func TestGetAgentByUUID(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	found, err := GetAgentByUUID(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentByUUID() error = %v", err)
	}
	if found.ID != agent.ID {
		t.Fatalf("GetAgentByUUID().ID = %q, want %q", found.ID, agent.ID)
	}
}

func TestListAgents(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	if _, _, err := CreateAgent(context.Background(), db, "secret1"); err != nil {
		t.Fatalf("CreateAgent(1) error = %v", err)
	}
	if _, _, err := CreateAgent(context.Background(), db, "secret2"); err != nil {
		t.Fatalf("CreateAgent(2) error = %v", err)
	}

	agents, err := ListAgents(context.Background(), db)
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) < 2 {
		t.Fatalf("ListAgents() len = %d, want >= 2", len(agents))
	}
}

func TestReapStaleAgents(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	// Touch the agent to set last_seen and status to working
	if _, err := TouchAgent(context.Background(), db, agent.ID, "working"); err != nil {
		t.Fatalf("TouchAgent() error = %v", err)
	}

	// Make last_seen old enough to be reaped
	if _, err := db.Exec(`UPDATE users SET last_seen = datetime('now', '-120 minutes') WHERE user_id = ?`, agent.ID); err != nil {
		t.Fatalf("update last_seen error = %v", err)
	}

	reaped, err := ReapStaleAgents(context.Background(), db, 60)
	if err != nil {
		t.Fatalf("ReapStaleAgents() error = %v", err)
	}
	if reaped != 1 {
		t.Fatalf("ReapStaleAgents() = %d, want 1", reaped)
	}

	fetched, err := GetAgentByID(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentByID() error = %v", err)
	}
	if fetched.Status != "idle" {
		t.Fatalf("agent status after reap = %q, want idle", fetched.Status)
	}
}

func TestListAgentStatuses(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	// Create a project and ticket assigned to the agent
	project, err := CreateProject(context.Background(), db, "Agent Work", "", "", "")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	ticket, err := CreateTicket(context.Background(), db, TicketCreateParams{
		ProjectID: project.ID,
		Type:      "task",
		Title:     "Agent task",
		Assignee:  agent.Username,
		CreatedBy: "",
	})
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	// Set ticket to active state
	if _, err := UpdateTicket(context.Background(), db, ticket.ID, TicketUpdateParams{
		Title:         ticket.Title,
		Description:   ticket.Description,
		ParentID:      ticket.ParentID,
		Assignee:      agent.Username,
		State:         StateActive,
		ActorUsername: "admin",
		ActorRole:     "admin",
	}); err != nil {
		t.Fatalf("UpdateTicket(active) error = %v", err)
	}

	statuses, err := ListAgentStatuses(context.Background(), db)
	if err != nil {
		t.Fatalf("ListAgentStatuses() error = %v", err)
	}
	if len(statuses) < 1 {
		t.Fatalf("ListAgentStatuses() len = %d, want >= 1", len(statuses))
	}
	// Find the agent with the assigned ticket
	found := false
	for _, s := range statuses {
		if s.Agent.ID == agent.ID && s.TicketKey != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("ListAgentStatuses() did not find agent with assigned ticket")
	}
}

func TestAgentConfig(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	// Set config
	if err := SetAgentConfig(context.Background(), db, agent.ID, "llm", "gpt-4"); err != nil {
		t.Fatalf("SetAgentConfig() error = %v", err)
	}
	if err := SetAgentConfig(context.Background(), db, agent.ID, "verbose", "true"); err != nil {
		t.Fatalf("SetAgentConfig(verbose) error = %v", err)
	}

	// List config
	entries, err := ListAgentConfig(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConfig() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListAgentConfig() len = %d, want 2", len(entries))
	}

	// Get config map
	configMap, err := GetAgentConfigMap(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentConfigMap() error = %v", err)
	}
	if configMap["llm"] != "gpt-4" {
		t.Fatalf("GetAgentConfigMap()[llm] = %q, want gpt-4", configMap["llm"])
	}

	// Get config updated_at
	updatedAt, err := GetAgentConfigUpdatedAt(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("GetAgentConfigUpdatedAt() error = %v", err)
	}
	if updatedAt == "" {
		t.Fatal("GetAgentConfigUpdatedAt() = empty, want timestamp")
	}

	// Delete config
	if err := DeleteAgentConfig(context.Background(), db, agent.ID, "llm"); err != nil {
		t.Fatalf("DeleteAgentConfig() error = %v", err)
	}
	if err := DeleteAgentConfig(context.Background(), db, agent.ID, "llm"); err == nil {
		t.Fatal("DeleteAgentConfig(again) error = nil, want error")
	}

	// Set config with empty key should fail
	if err := SetAgentConfig(context.Background(), db, agent.ID, "", "val"); err == nil {
		t.Fatal("SetAgentConfig(empty key) error = nil, want error")
	}
}

func TestAgentFoundByGetUserByUsername(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(context.Background(), db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	user, err := GetUserByUsername(context.Background(), db, agent.ID)
	if err != nil {
		t.Fatalf("GetUserByUsername() error = %v", err)
	}
	if user.UserType != "agent" {
		t.Fatalf("user.UserType = %q, want agent", user.UserType)
	}
}
