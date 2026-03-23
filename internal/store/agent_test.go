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

func TestCreateAgentGeneratesPasswordWhenMissing(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, generatedPassword, err := CreateAgent(db, "worker-1", "autonomous worker", "")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if agent.ID == 0 {
		t.Fatalf("agent ID = 0, want non-zero")
	}
	if generatedPassword == "" {
		t.Fatalf("generatedPassword empty, want generated secret")
	}
	if !agent.Enabled {
		t.Fatalf("agent enabled = false, want true")
	}
	if agent.Status != "idle" {
		t.Fatalf("agent status = %q, want idle", agent.Status)
	}
	if agent.UserType != "agent" {
		t.Fatalf("agent user_type = %q, want agent", agent.UserType)
	}

	authenticated, err := AuthenticateAgent(db, "worker-1", generatedPassword)
	if err != nil {
		t.Fatalf("AuthenticateAgent() error = %v", err)
	}
	if authenticated.ID != agent.ID {
		t.Fatalf("authenticated ID = %d, want %d", authenticated.ID, agent.ID)
	}
}

func TestAgentUpdateEnableDisableDeleteLifecycle(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	agent, password, err := CreateAgent(db, "worker-2", "desc", "secret-1")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if password != "secret-1" {
		t.Fatalf("password = %q, want secret-1", password)
	}

	updatedName := "worker-2a"
	updatedDesc := "new desc"
	updatedPassword := "secret-2"
	updated, err := UpdateAgent(db, agent.ID, AgentUpdateParams{
		Name:        &updatedName,
		Description: &updatedDesc,
		Password:    &updatedPassword,
	})
	if err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}
	if updated.Username != updatedName {
		t.Fatalf("updated.Username = %q, want %q", updated.Username, updatedName)
	}
	if updated.Description != updatedDesc {
		t.Fatalf("updated.Description = %q, want %q", updated.Description, updatedDesc)
	}

	if _, err := AuthenticateAgent(db, updatedName, "secret-1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("AuthenticateAgent(old password) err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := AuthenticateAgent(db, updatedName, "secret-2"); err != nil {
		t.Fatalf("AuthenticateAgent(new password) error = %v", err)
	}

	disabled, err := SetAgentEnabled(db, agent.ID, false)
	if err != nil {
		t.Fatalf("SetAgentEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled.Enabled = true, want false")
	}
	if _, err := AuthenticateAgent(db, updatedName, "secret-2"); !errors.Is(err, ErrForbidden) {
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

	_, _, err := CreateAgent(db, "worker-hidden", "should not appear", "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	users, err := ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	for _, u := range users {
		if u.Username == "worker-hidden" {
			t.Fatalf("ListUsers() should not include agents, found %q", u.Username)
		}
	}
}

func TestAgentFoundByGetUserByUsername(t *testing.T) {
	db := openAgentTestDB(t)
	defer db.Close()

	_, _, err := CreateAgent(db, "worker-findable", "findable agent", "secret")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	user, err := GetUserByUsername(db, "worker-findable")
	if err != nil {
		t.Fatalf("GetUserByUsername() error = %v", err)
	}
	if user.UserType != "agent" {
		t.Fatalf("user.UserType = %q, want agent", user.UserType)
	}
}
