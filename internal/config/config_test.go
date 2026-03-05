package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadAndResolveServerURL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	cfg := Config{ServerURL: "http://example.test:9000"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ServerURL != cfg.ServerURL {
		t.Fatalf("Load().ServerURL = %q, want %q", got.ServerURL, cfg.ServerURL)
	}

	path := filepath.Join(tempDir, "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}

	if resolved := ResolveServerURL(got); resolved != cfg.ServerURL {
		t.Fatalf("ResolveServerURL() = %q, want %q", resolved, cfg.ServerURL)
	}
	if got.Token != "" {
		t.Fatalf("Load().Token = %q, want empty because credentials are stored separately", got.Token)
	}

	got.CurrentProject = "2"
	if err := Save(got); err != nil {
		t.Fatalf("Save(updated) error = %v", err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load(reloaded) error = %v", err)
	}
	if reloaded.CurrentProject != "2" {
		t.Fatalf("Load().CurrentProject = %q, want 2", reloaded.CurrentProject)
	}

	t.Setenv("TICKET_SERVER", "http://env.test:7000")
	if resolved := ResolveServerURL(got); resolved != "http://env.test:7000" {
		t.Fatalf("ResolveServerURL() with env = %q", resolved)
	}
}

func TestResolveServerURLDefault(t *testing.T) {
	t.Setenv("TICKET_SERVER", "")
	t.Setenv("TICKET_URL", "")
	if resolved := ResolveServerURL(Config{}); resolved != "http://localhost:8080" {
		t.Fatalf("ResolveServerURL(default) = %q", resolved)
	}
}

func TestResolveModeDefaultsToLocal(t *testing.T) {
	t.Setenv("TICKET_MODE", "")
	mode, err := ResolveMode()
	if err != nil {
		t.Fatalf("ResolveMode() error = %v", err)
	}
	if mode != ModeLocal {
		t.Fatalf("ResolveMode() = %q, want %q", mode, ModeLocal)
	}
}

func TestResolveModeRejectsInvalidValue(t *testing.T) {
	t.Setenv("TICKET_MODE", "bogus")
	if _, err := ResolveMode(); err == nil {
		t.Fatal("ResolveMode() error = nil, want invalid mode error")
	}
}

func TestResolveDatabasePathUsesOverrideAndHomeDefaults(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_DB_OVERRIDE", filepath.Join(tempDir, "override.db"))
	path, err := ResolveDatabasePath()
	if err != nil {
		t.Fatalf("ResolveDatabasePath(override) error = %v", err)
	}
	if path != filepath.Join(tempDir, "override.db") {
		t.Fatalf("ResolveDatabasePath(override) = %q", path)
	}

	t.Setenv("TICKET_DB_OVERRIDE", "")
	t.Setenv("TICKET_HOME", tempDir)
	path, err = ResolveDatabasePath()
	if err != nil {
		t.Fatalf("ResolveDatabasePath(TICKET_HOME) error = %v", err)
	}
	if path != filepath.Join(tempDir, "ticket.db") {
		t.Fatalf("ResolveDatabasePath(TICKET_HOME) = %q", path)
	}

	t.Setenv("TICKET_HOME", "")
	path, err = ResolveDatabasePath()
	if err != nil {
		t.Fatalf("ResolveDatabasePath(default home) error = %v", err)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	wantPath := filepath.Join(userHome, ".config", "ticket", "ticket.db")
	if strings.TrimPrefix(filepath.Clean(path), "/private") != strings.TrimPrefix(filepath.Clean(wantPath), "/private") {
		t.Fatalf("ResolveDatabasePath(default home) = %q, want %q", path, wantPath)
	}
}

func TestHomeDefaultsToDotConfigTicket(t *testing.T) {
	t.Setenv("TICKET_HOME", "")
	t.Setenv("TICKET_CONFIG_DIR", "")

	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	want := filepath.Join(userHome, ".config", "ticket")
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestCredentialsStoredSeparately(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	cfg := Config{ServerURL: "http://example.test:9000", Username: "alice", Token: "sensitive"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := SaveCredentials(Credentials{Token: "session-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile(config.json) error = %v", err)
	}
	if string(data) == "" {
		t.Fatal("config.json is empty")
	}
	if strings.Contains(string(data), "session-token") {
		t.Fatal("config.json should not contain session token")
	}
	if got, err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	} else if got.Token != "session-token" {
		t.Fatalf("Load().Token = %q, want session-token", got.Token)
	}

	if err := ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "credentials.json")); !os.IsNotExist(err) {
		t.Fatalf("credentials.json should be removed, err = %v", err)
	}
}
