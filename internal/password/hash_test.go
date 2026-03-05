package password

import (
	"strings"
	"testing"
)

func TestHashUsesArgon2IDFormat(t *testing.T) {
	hash, err := Hash("secret-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("Hash() = %q, want argon2id prefix", hash)
	}
	if strings.Contains(hash, "secret-password") {
		t.Fatalf("Hash() leaked plaintext password")
	}
}

func TestVerifyMatchesPassword(t *testing.T) {
	hash, err := Hash("secret-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	ok, err := Verify(hash, "secret-password")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok {
		t.Fatalf("Verify() = false, want true")
	}

	ok, err = Verify(hash, "wrong-password")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if ok {
		t.Fatalf("Verify() = true, want false")
	}
}
