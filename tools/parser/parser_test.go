package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRequirementsAndBuildCommands(t *testing.T) {
	path := writeRequirementsFixture(t, `
EPIC: Authentication
ID: E1
DESCRIPTION: Add login flow
AC:
- user can log in
PRIORITY: 1
DEPENDS-ON: NONE

    STORY: Password reset
    ID: E1-S1
    DESCRIPTION: Add password reset
    AC:
    - user can request reset
    PRIORITY: 2
    DEPENDS-ON: E1
`)

	entries, err := parseRequirements(path)
	if err != nil {
		t.Fatalf("parseRequirements() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("parseRequirements() len = %d, want 2", len(entries))
	}
	if entries[0].Kind != "epic" || entries[1].Kind != "story" {
		t.Fatalf("entries = %#v", entries)
	}

	commands := buildCommands(entries)
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"E1=$(task create -t 'epic'",
		"E1_S1=$(task create -t 'task'",
		`-parent "${E1}"`,
		`task dependency add "${E1_S1}" "${E1}"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("buildCommands() missing %q:\n%s", want, joined)
		}
	}
}

func TestParseRequirementsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duplicate id",
			body: `
EPIC: One
ID: E1
DESCRIPTION: Desc
AC:
- a
PRIORITY: 1
DEPENDS-ON: NONE

EPIC: Two
ID: E1
DESCRIPTION: Desc
AC:
- a
PRIORITY: 1
DEPENDS-ON: NONE
`,
			want: "duplicate ID E1",
		},
		{
			name: "unknown dependency",
			body: `
EPIC: One
ID: E1
DESCRIPTION: Desc
AC:
- a
PRIORITY: 1
DEPENDS-ON: E9
`,
			want: "unknown dependency E9",
		},
		{
			name: "story before epic",
			body: `
    STORY: Child
    ID: E1-S1
    DESCRIPTION: Desc
    AC:
    - a
    PRIORITY: 1
    DEPENDS-ON: NONE
`,
			want: "content found before first epic",
		},
		{
			name: "missing ac",
			body: `
EPIC: One
ID: E1
DESCRIPTION: Desc
PRIORITY: 1
DEPENDS-ON: NONE
`,
			want: "AC must contain at least one bullet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeRequirementsFixture(t, tt.body)
			_, err := parseRequirements(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseRequirements() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestRunPrintsCommands(t *testing.T) {
	path := writeRequirementsFixture(t, `
EPIC: Authentication
ID: E1
DESCRIPTION: Add login flow
AC:
- user can log in
PRIORITY: 1
DEPENDS-ON: NONE
`)

	stdout := captureParserStdout(t, func() {
		if err := run([]string{"-f", path}); err != nil {
			t.Fatalf("run() error = %v", err)
		}
	})
	if !strings.Contains(stdout, "task create -t 'epic'") {
		t.Fatalf("run() output = %q", stdout)
	}
}

func TestRunRejectsMissingFileFlag(t *testing.T) {
	if err := run(nil); err == nil || err.Error() != "usage: parser -f REQUIREMENTS.md" {
		t.Fatalf("run(nil) error = %v", err)
	}
}

func TestShellQuoteAndVarName(t *testing.T) {
	if got := shellVarName("E1-S1"); got != "E1_S1" {
		t.Fatalf("shellVarName() = %q", got)
	}
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Fatalf("shellQuote() = %q", got)
	}
}

func writeRequirementsFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "requirements.md")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func captureParserStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return string(data)
}
