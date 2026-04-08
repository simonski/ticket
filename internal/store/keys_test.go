package store

import (
	"context"
	"testing"
)

func TestTicketTypeCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"epic", "E"},
		{"task", "T"},
		{"bug", "B"},
		{"spike", "S"},
		{"chore", "C"},
		{"note", "N"},
		{"question", "Q"},
		{"requirement", "R"},
		{"decision", "D"},
	}
	for _, tc := range cases {
		got, err := ticketTypeCode(tc.input)
		if err != nil {
			t.Fatalf("ticketTypeCode(%q) error = %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ticketTypeCode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}

	// Invalid type
	if _, err := ticketTypeCode("invalid"); err == nil {
		t.Fatal("ticketTypeCode(invalid) error = nil, want error")
	}
}

func TestDeriveProjectPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Customer Portal", "CP"},
		{"", "PRJ"},
		{"A", "AXX"},
		{"Hello World Of Things And More", "HWOTA"},
	}
	for _, tc := range cases {
		got := deriveProjectPrefix(tc.input)
		if len(got) < 2 || len(got) > 5 {
			t.Fatalf("deriveProjectPrefix(%q) = %q, invalid length", tc.input, got)
		}
	}
}

func TestGenerateTicketKey(t *testing.T) {
	// Default prefix (TK) uses simple format
	key, err := generateTicketKey("TK", "task", 1)
	if err != nil {
		t.Fatalf("generateTicketKey(TK) error = %v", err)
	}
	if key != "TK-1" {
		t.Fatalf("generateTicketKey(TK) = %q, want TK-1", key)
	}

	// Non-default prefix uses type code
	key, err = generateTicketKey("ABC", "epic", 5)
	if err != nil {
		t.Fatalf("generateTicketKey(ABC) error = %v", err)
	}
	if key != "ABC-E-5" {
		t.Fatalf("generateTicketKey(ABC) = %q, want ABC-E-5", key)
	}

	// Invalid prefix
	if _, err := generateTicketKey("x", "task", 1); err == nil {
		t.Fatal("generateTicketKey(bad prefix) error = nil, want error")
	}

	// Invalid sequence
	if _, err := generateTicketKey("TK", "task", 0); err == nil {
		t.Fatal("generateTicketKey(seq=0) error = nil, want error")
	}
}

func TestValidateProjectPrefix(t *testing.T) {
	if err := validateProjectPrefix("ABC"); err != nil {
		t.Fatalf("validateProjectPrefix(ABC) error = %v", err)
	}
	if err := validateProjectPrefix("x"); err == nil {
		t.Fatal("validateProjectPrefix(x) error = nil, want error")
	}
	if err := validateProjectPrefix("ABCDEF"); err == nil {
		t.Fatal("validateProjectPrefix(ABCDEF) error = nil, want error")
	}
}

func TestNextUniqueProjectPrefix(t *testing.T) {
	db := testDB(t)

	// TK is already used by default project
	prefix, err := nextUniqueProjectPrefix(context.Background(), db, "TK")
	if err != nil {
		t.Fatalf("nextUniqueProjectPrefix(TK) error = %v", err)
	}
	// Should get TK1 or similar since TK is taken
	if prefix == "TK" {
		t.Fatal("nextUniqueProjectPrefix(TK) should not return TK since it is taken")
	}

	// ABC should be available
	prefix, err = nextUniqueProjectPrefix(context.Background(), db, "ABC")
	if err != nil {
		t.Fatalf("nextUniqueProjectPrefix(ABC) error = %v", err)
	}
	if prefix != "ABC" {
		t.Fatalf("nextUniqueProjectPrefix(ABC) = %q, want ABC", prefix)
	}
}
