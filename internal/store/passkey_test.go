package store

import (
	"context"
	"errors"
	"testing"
)

func TestSaveAndUpdatePasskeyCredential(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	user, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := SavePasskeyCredential(context.Background(), db, user.ID, "laptop", "cred-1", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential() error = %v", err)
	}
	credentials, err := ListPasskeyCredentials(context.Background(), db, user.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials() error = %v", err)
	}
	if len(credentials) != 1 {
		t.Fatalf("ListPasskeyCredentials() len = %d, want 1", len(credentials))
	}
	if credentials[0].Name != "laptop" {
		t.Fatalf("credential name = %q, want laptop", credentials[0].Name)
	}
	if err := UpdatePasskeyCredential(context.Background(), db, "cred-1", `{"signCount":2}`); err != nil {
		t.Fatalf("UpdatePasskeyCredential() error = %v", err)
	}
	credentials, err = ListPasskeyCredentials(context.Background(), db, user.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(updated) error = %v", err)
	}
	if credentials[0].CredentialJSON != `{"signCount":2}` {
		t.Fatalf("updated credential json = %q", credentials[0].CredentialJSON)
	}
	if credentials[0].LastUsedAt == "" {
		t.Fatal("updated credential should record last_used_at")
	}
}

func TestRenameAndDeletePasskeyCredential(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	alice, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(alice) error = %v", err)
	}
	bob, err := CreateUser(context.Background(), db, "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(bob) error = %v", err)
	}
	if err := SavePasskeyCredential(context.Background(), db, alice.ID, "laptop", "cred-1", `{"signCount":1}`); err != nil {
		t.Fatalf("SavePasskeyCredential(alice) error = %v", err)
	}
	if err := RenamePasskeyCredential(context.Background(), db, alice.ID, "cred-1", "desk key"); err != nil {
		t.Fatalf("RenamePasskeyCredential() error = %v", err)
	}
	credentials, err := ListPasskeyCredentials(context.Background(), db, alice.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(alice) error = %v", err)
	}
	if credentials[0].Name != "desk key" {
		t.Fatalf("renamed credential name = %q, want desk key", credentials[0].Name)
	}
	if err := RenamePasskeyCredential(context.Background(), db, bob.ID, "cred-1", "stolen"); !errors.Is(err, ErrPasskeyNotFound) {
		t.Fatalf("RenamePasskeyCredential(wrong user) error = %v, want ErrPasskeyNotFound", err)
	}
	if err := DeletePasskeyCredential(context.Background(), db, bob.ID, "cred-1"); !errors.Is(err, ErrPasskeyNotFound) {
		t.Fatalf("DeletePasskeyCredential(wrong user) error = %v, want ErrPasskeyNotFound", err)
	}
	if err := DeletePasskeyCredential(context.Background(), db, alice.ID, "cred-1"); err != nil {
		t.Fatalf("DeletePasskeyCredential() error = %v", err)
	}
	credentials, err = ListPasskeyCredentials(context.Background(), db, alice.ID)
	if err != nil {
		t.Fatalf("ListPasskeyCredentials(after delete) error = %v", err)
	}
	if len(credentials) != 0 {
		t.Fatalf("ListPasskeyCredentials(after delete) len = %d, want 0", len(credentials))
	}
}

func TestPasskeyFlowLifecycle(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	user, err := CreateUser(context.Background(), db, "alice", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	flow, err := CreatePasskeyFlow(context.Background(), db, PasskeyFlowPurposeLogin, user.ID, "", `{"challenge":"abc"}`, `{"publicKey":true}`)
	if err != nil {
		t.Fatalf("CreatePasskeyFlow() error = %v", err)
	}
	if _, err := ConsumePasskeyFlow(context.Background(), db, flow.Code); !errors.Is(err, ErrPasskeyFlowPending) {
		t.Fatalf("ConsumePasskeyFlow(pending) error = %v, want ErrPasskeyFlowPending", err)
	}
	if err := CompletePasskeyFlow(context.Background(), db, flow.Code, "session-token"); err != nil {
		t.Fatalf("CompletePasskeyFlow() error = %v", err)
	}
	consumed, err := ConsumePasskeyFlow(context.Background(), db, flow.Code)
	if err != nil {
		t.Fatalf("ConsumePasskeyFlow() error = %v", err)
	}
	if consumed.Token != "session-token" {
		t.Fatalf("ConsumePasskeyFlow().Token = %q, want session-token", consumed.Token)
	}
	if _, err := ConsumePasskeyFlow(context.Background(), db, flow.Code); !errors.Is(err, ErrPasskeyFlowConsumed) {
		t.Fatalf("ConsumePasskeyFlow(consumed) error = %v, want ErrPasskeyFlowConsumed", err)
	}
}
