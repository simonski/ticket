package password

import (
	"strings"
	"testing"
)

func TestHashParamsUsesFastHashOverride(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		wantMemory uint32
		wantIters  uint32
	}{
		{name: "default", env: "", wantMemory: 64 * 1024, wantIters: 4},
		{name: "fast", env: "1", wantMemory: 1024, wantIters: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TICKET_FAST_HASH", tt.env)
			memory, iterations := hashParams()
			if memory != tt.wantMemory || iterations != tt.wantIters {
				t.Fatalf("hashParams() = (%d, %d), want (%d, %d)", memory, iterations, tt.wantMemory, tt.wantIters)
			}
		})
	}
}

func TestHashUsesArgon2IDFormat(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestParseRejectsInvalidArgon2Formats(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		wantErr string
	}{
		{name: "not argon2", encoded: "plain-text", wantErr: "invalid argon2id hash"},
		{name: "bad params item", encoded: "$argon2id$v=19$m$abcd$efgh", wantErr: "invalid argon2id params"},
		{name: "unknown param", encoded: "$argon2id$v=19$m=65536,t=4,x=2$abcd$efgh", wantErr: `unknown argon2id param "x"`},
		{name: "bad param number", encoded: "$argon2id$v=19$m=oops,t=4,p=2$abcd$efgh", wantErr: "parse argon2id param m"},
		{name: "bad salt", encoded: "$argon2id$v=19$m=65536,t=4,p=2$***$efgh", wantErr: "decode salt"},
		{name: "bad hash", encoded: "$argon2id$v=19$m=65536,t=4,p=2$abcd$***", wantErr: "decode hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parse(tt.encoded)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parse() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyRejectsInvalidHash(t *testing.T) {
	t.Parallel()

	ok, err := Verify("not-a-valid-hash", "secret-password")
	if err == nil {
		t.Fatal("Verify() error = nil, want invalid hash error")
	}
	if ok {
		t.Fatal("Verify() = true, want false on invalid hash")
	}
}
