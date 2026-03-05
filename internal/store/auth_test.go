package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestAuthLifecycle(t *testing.T) {
	db := testDB(t)

	user, err := RegisterUser(db, "carol", "password123")
	if err != nil {
		t.Fatalf("RegisterUser() error = %v", err)
	}
	if user.Role != "user" {
		t.Fatalf("RegisterUser().Role = %q, want user", user.Role)
	}

	authenticated, err := AuthenticateUser(db, "carol", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser() error = %v", err)
	}
	if authenticated.Username != "carol" {
		t.Fatalf("AuthenticateUser().Username = %q, want carol", authenticated.Username)
	}

	token, err := CreateSession(db, authenticated.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sessionUser, err := GetUserByToken(db, token)
	if err != nil {
		t.Fatalf("GetUserByToken() error = %v", err)
	}
	if sessionUser.Username != "carol" {
		t.Fatalf("GetUserByToken().Username = %q, want carol", sessionUser.Username)
	}

	if err := DeleteSession(db, token); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := GetUserByToken(db, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("GetUserByToken() after logout error = %v, want ErrUnauthorized", err)
	}
}

func TestAdminUserManagement(t *testing.T) {
	db := testDB(t)

	created, err := CreateUser(db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.Username != "bob" {
		t.Fatalf("CreateUser().Username = %q, want bob", created.Username)
	}

	users, err := ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers() len = %d, want 2", len(users))
	}

	if err := SetUserEnabled(db, "bob", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	if _, err := AuthenticateUser(db, "bob", "password123"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthenticateUser(disabled) error = %v, want ErrForbidden", err)
	}

	if err := SetUserEnabled(db, "bob", true); err != nil {
		t.Fatalf("SetUserEnabled(true) error = %v", err)
	}
	if _, err := AuthenticateUser(db, "bob", "password123"); err != nil {
		t.Fatalf("AuthenticateUser(re-enabled) error = %v", err)
	}

	if err := DeleteUser(db, "bob"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	users, err = ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers(after delete) error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("ListUsers(after delete) len = %d, want 1", len(users))
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ticket.db")
	if err := Init(dbPath, "admin", "password"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
