package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillCommandsManageRepositoryInstallation(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	setTestWorkingDir(t, repo)
	path := filepath.Join(repo, ".claude", "skills", "tk", "SKILL.md")

	output := captureStdout(t, func() {
		if err := runSkill(nil); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"USAGE", "COMMANDS", "print", "install", "uninstall"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skill help missing %q:\n%s", want, output)
		}
	}

	if err := runSkill([]string{"install"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != tkSkillContent {
		t.Fatalf("installed skill differs from embedded: %v", err)
	}

	oldContent := strings.Replace(tkSkillContent, "version: "+embeddedSkillVersion(), "version: 0.0.0", 1)
	if err := os.WriteFile(path, []byte(oldContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSkill([]string{"install"}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(path)
	if err != nil || string(content) != tkSkillContent {
		t.Fatalf("outdated skill was not updated: %v", err)
	}

	modified := tkSkillContent + "\nlocal note\n"
	if err := os.WriteFile(path, []byte(modified), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSkill([]string{"install"}); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("install should preserve local modification: %v", err)
	}
	content, err = os.ReadFile(path)
	if err != nil || string(content) != modified {
		t.Fatalf("local modification was lost: %v", err)
	}
	if err := runSkill([]string{"uninstall"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("skill should be removed: %v", err)
	}
}

func TestSkillVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		local, embedded string
		want            int
		valid           bool
	}{
		{"0.0.4", "0.0.5", -1, true},
		{"0.0.5", "0.0.5", 0, true},
		{"0.1.0", "0.0.5", 1, true},
		{"unknown", "0.0.5", 0, false},
	} {
		got, valid := compareSkillVersions(tc.local, tc.embedded)
		if got != tc.want || valid != tc.valid {
			t.Errorf("compareSkillVersions(%q, %q) = %d, %t", tc.local, tc.embedded, got, valid)
		}
	}
}
