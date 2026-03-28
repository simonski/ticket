package store

import (
	"os"
	"testing"
)

func TestEncryptDecryptEmailWithKey(t *testing.T) {
	os.Setenv("TICKET_ENCRYPTION_KEY", "test-key-for-encryption-32bytes!")
	defer os.Unsetenv("TICKET_ENCRYPTION_KEY")

	email := "user@example.com"
	encrypted, err := EncryptEmail(email)
	if err != nil {
		t.Fatalf("EncryptEmail() error = %v", err)
	}
	if encrypted == email {
		t.Fatal("EncryptEmail() did not encrypt")
	}

	decrypted, err := DecryptEmail(encrypted)
	if err != nil {
		t.Fatalf("DecryptEmail() error = %v", err)
	}
	if decrypted != email {
		t.Fatalf("DecryptEmail() = %q, want %q", decrypted, email)
	}
}

func TestEncryptEmailWithoutKey(t *testing.T) {
	os.Unsetenv("TICKET_ENCRYPTION_KEY")

	email := "user@example.com"
	result, err := EncryptEmail(email)
	if err != nil {
		t.Fatalf("EncryptEmail() error = %v", err)
	}
	if result != email {
		t.Fatalf("EncryptEmail() = %q, want plaintext %q", result, email)
	}
}

func TestDecryptEmailPlaintext(t *testing.T) {
	os.Unsetenv("TICKET_ENCRYPTION_KEY")

	result, err := DecryptEmail("plain@example.com")
	if err != nil {
		t.Fatalf("DecryptEmail() error = %v", err)
	}
	if result != "plain@example.com" {
		t.Fatalf("DecryptEmail() = %q, want plaintext", result)
	}
}

func TestDecryptEmailWithoutKeyFails(t *testing.T) {
	os.Unsetenv("TICKET_ENCRYPTION_KEY")

	if _, err := DecryptEmail("enc:somedata"); err == nil {
		t.Fatal("DecryptEmail(enc: without key) error = nil, want error")
	}
}
