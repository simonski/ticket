package store

import (
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

	agent, generatedPassword, err := CreateAgent(db, "")
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
	authenticated, err := AuthenticateAgent(db, agent.ID, generatedPassword)
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

	agent, password, err := CreateAgent(db, "secret-1")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if password != "secret-1" {
		t.Fatalf("password = %q, want secret-1", password)
	}

	updatedPassword := "secret-2"
	_, err = UpdateAgent(db, agent.ID, AgentUpdateParams{
		Password: &updatedPassword,
	})
	if err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	if _, err := AuthenticateAgent(db, agent.ID, "secret-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("AuthenticateAgent(old password) err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := AuthenticateAgent(db, agent.ID, "secret-2"); err != nil {
		t.Fatalf("AuthenticateAgent(new password) error = %v", err)
	}

	disabled, err := SetAgentEnabled(db, agent.ID, false)
	if err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled.Enabled = true, want false")
	}
	if _, err := AuthenticateAgent(db, agent.ID, "secret-2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthenticateAgent(disabled) err = %v, want ErrForbidden", err)
	}

	enabled, err := SetAgentEnabled(db, agent.ID, true)
	if err != nil {
		t.Fatalf("SetAgentEnabled(true) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatalf("enabled.Enabled = false, want true")
	}
	touched, err := TouchAgent(db, agent.ID, "working")
	if err != nil {
		t.Fatalf("TouchAgent() error = %v", err)
	}
	if touched.Status != "working" {
		t.Fatalf("touched.Status = %q, want working", touched.Status)
	}
	if touched.LastSeen == "" {
		t.Fatalf("touched.LastSeen empty, want timestamp")
	}

	if err := DeleteAgent(db, agent.ID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	if _, err := GetAgentByID(db, agent.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetAgentByID(deleted) err = %v, want sql.ErrNoRows", err)
	}
}

func TestAgentDoesNotAppearInListUsers(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	users, err := ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	for _, u := range users {
		if u.Username == agent.ID {
			t.Fatalf("ListUsers() should not include agents, found %q", u.Username)
		}
	}
}

func TestAgentFoundByGetUserByUsername(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, _, err := CreateAgent(db, "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	user, err := GetUserByUsername(db, agent.ID)
	if err != nil {
		t.Fatalf("GetUserByUsername() error = %v", err)
	}
	if user.UserType != "agent" {
		t.Fatalf("user.UserType = %q, want agent", user.UserType)
	}
}
