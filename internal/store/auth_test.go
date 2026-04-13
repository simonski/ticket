package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthLifecycle(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	user, err := RegisterUser(context.Background(), db, "carol", "password123")
	if err != nil {
		t.Fatalf("RegisterUser() error = %v", err)
	}
	if user.Role != "user" {
		t.Fatalf("RegisterUser().Role = %q, want user", user.Role)
	}

	authenticated, err := AuthenticateUser(context.Background(), db, "carol", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser() error = %v", err)
	}
	if authenticated.Username != "carol" {
		t.Fatalf("AuthenticateUser().Username = %q, want carol", authenticated.Username)
	}

	token, err := CreateSession(context.Background(), db, authenticated.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sessionUser, err := GetUserByToken(context.Background(), db, token)
	if err != nil {
		t.Fatalf("GetUserByToken() error = %v", err)
	}
	if sessionUser.Username != "carol" {
		t.Fatalf("GetUserByToken().Username = %q, want carol", sessionUser.Username)
	}

	if err := DeleteSession(context.Background(), db, token); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := GetUserByToken(context.Background(), db, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("GetUserByToken() after logout error = %v, want ErrUnauthorized", err)
	}
}

func TestRegisterUserRejectsInvalidUsernameCharacters(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	if _, err := RegisterUser(context.Background(), db, "bad user!", "password123"); err == nil || !strings.Contains(err.Error(), "invalid characters") {
		t.Fatalf("RegisterUser(invalid username) error = %v, want invalid character error", err)
	}
}

func TestAdminUserManagement(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	created, err := CreateUser(context.Background(), db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.Username != "bob" {
		t.Fatalf("CreateUser().Username = %q, want bob", created.Username)
	}

	users, err := ListUsers(context.Background(), db, 0)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers() len = %d, want 2", len(users))
	}

	if err := SetUserEnabled(context.Background(), db, "bob", false); err != nil {
		t.Fatalf("SetUserEnabled(false) error = %v", err)
	}
	if _, err := AuthenticateUser(context.Background(), db, "bob", "password123"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthenticateUser(disabled) error = %v, want ErrForbidden", err)
	}

	if err := SetUserEnabled(context.Background(), db, "bob", true); err != nil {
		t.Fatalf("SetUserEnabled(true) error = %v", err)
	}
	if _, err := AuthenticateUser(context.Background(), db, "bob", "password123"); err != nil {
		t.Fatalf("AuthenticateUser(re-enabled) error = %v", err)
	}

	if err := DeleteUser(context.Background(), db, "bob"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	users, err = ListUsers(context.Background(), db, 0)
	if err != nil {
		t.Fatalf("ListUsers(after delete) error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("ListUsers(after delete) len = %d, want 1", len(users))
	}
}

func TestResetUserPassword(t *testing.T) {
	t.Parallel()
	db := testDB(t)

	user, err := CreateUser(context.Background(), db, "carol", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Create a session so we can verify it gets invalidated
	token, err := CreateSession(context.Background(), db, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Reset password
	updated, err := ResetUserPassword(context.Background(), db, "carol", "newpassword456")
	if err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if updated.Username != "carol" {
		t.Fatalf("ResetUserPassword().Username = %q, want carol", updated.Username)
	}

	// Old password should fail
	if _, err := AuthenticateUser(context.Background(), db, "carol", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("AuthenticateUser(old password) error = %v, want ErrInvalidCredentials", err)
	}

	// New password should work
	if _, err := AuthenticateUser(context.Background(), db, "carol", "newpassword456"); err != nil {
		t.Fatalf("AuthenticateUser(new password) error = %v", err)
	}

	// Session should be invalidated
	if _, err := GetUserByToken(context.Background(), db, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("GetUserByToken(after reset) error = %v, want ErrUnauthorized", err)
	}

	// Reset with empty username should fail
	if _, err := ResetUserPassword(context.Background(), db, "", "password"); err == nil {
		t.Fatal("ResetUserPassword(empty username) error = nil, want error")
	}

	// Reset with empty password should fail
	if _, err := ResetUserPassword(context.Background(), db, "carol", ""); err == nil {
		t.Fatal("ResetUserPassword(empty password) error = nil, want error")
	}

	// Reset for non-existent user should fail
	if _, err := ResetUserPassword(context.Background(), db, "nobody", "password"); err == nil {
		t.Fatal("ResetUserPassword(non-existent) error = nil, want error")
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

// testAdminID returns the user_id of the admin user created by testDB.
func testAdminID(t *testing.T, db *sql.DB) string {
	t.Helper()
	user, err := GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin) error = %v", err)
	}
	return user.ID
}
