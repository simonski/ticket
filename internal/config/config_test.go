package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupConfigTestHome(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return tempDir
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	cfg := Config{Location: "http://example.test:9000"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Location != cfg.Location {
		t.Fatalf("Load().Location = %q, want %q", got.Location, cfg.Location)
	}

	path := filepath.Join(tempDir, "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}

	if got.Token != "" {
		t.Fatalf("Load().Token = %q, want empty because credentials are stored separately", got.Token)
	}

	got.ProjectID = "2"
	if err := Save(got); err != nil {
		t.Fatalf("Save(updated) error = %v", err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load(reloaded) error = %v", err)
	}
	if reloaded.ProjectID != "2" {
		t.Fatalf("Load().ProjectID = %q, want 2", reloaded.ProjectID)
	}
}

func TestLoadMigratesLegacyEpicID(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	// Write config with numeric current_epic_id (legacy format)
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"current_epic_id": 42}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CurrentEpicID != "42" {
		t.Fatalf("Load().CurrentEpicID = %q, want \"42\"", cfg.CurrentEpicID)
	}
}

func TestLoadMigratesLegacyEpicIDZero(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"current_epic_id": 0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CurrentEpicID != "" {
		t.Fatalf("Load().CurrentEpicID = %q, want empty for legacy 0", cfg.CurrentEpicID)
	}
}

func TestLoadMigratesLegacyExpandedEpics(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"tui_expanded_epics": [1, 2, 3]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.TUIExpandedEpics) != 3 {
		t.Fatalf("Load().TUIExpandedEpics len = %d, want 3", len(cfg.TUIExpandedEpics))
	}
}

func TestLoadMissingFile(t *testing.T) {
	setupConfigTestHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if cfg.Location != "" {
		t.Fatalf("Load().Location = %q, want empty", cfg.Location)
	}
}

func TestResolveURLDefaultsToLocal(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeLocal {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeLocal)
	}
	if resolved.DBPath != filepath.Join(tempDir, "ticket.db") {
		t.Fatalf("DBPath = %q, want %q", resolved.DBPath, filepath.Join(tempDir, "ticket.db"))
	}
}

func TestResolveURLHTTPScheme(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"http://localhost:8080"}`), 0o600)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeRemote {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeRemote)
	}
	if resolved.ServerURL != "http://localhost:8080" {
		t.Fatalf("ServerURL = %q", resolved.ServerURL)
	}
}

func TestResolveURLHTTPSScheme(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"https://tickets.example.com"}`), 0o600)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeRemote {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeRemote)
	}
	if resolved.ServerURL != "https://tickets.example.com" {
		t.Fatalf("ServerURL = %q", resolved.ServerURL)
	}
}

func TestResolveURLFileScheme(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"file:///tmp/test.db"}`), 0o600)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeLocal {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeLocal)
	}
	if resolved.DBPath != "/tmp/test.db" {
		t.Fatalf("DBPath = %q, want /tmp/test.db", resolved.DBPath)
	}
}

func TestResolveURLRejectsUnsupportedScheme(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"ftp://example.com"}`), 0o600)

	if _, err := ResolveURL(); err == nil {
		t.Fatal("ResolveURL() error = nil, want unsupported scheme error")
	}
}

func TestResolveURLUsesEnvOverrideWhenRemoteTrioSet(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"file:///tmp/local.db"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	t.Setenv("TICKET_URL", "https://tickets.example.com")
	t.Setenv("TICKET_USERNAME", "alice")
	t.Setenv("TICKET_PASSWORD", "secret12")

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeRemote {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeRemote)
	}
	if resolved.ServerURL != "https://tickets.example.com" {
		t.Fatalf("ServerURL = %q, want env URL", resolved.ServerURL)
	}
}

func TestResolveURLUsesEnvLocationOverrideForLocalPath(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	localDB := filepath.Join(tempDir, "override.db")
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"https://tickets.example.com"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	t.Setenv("TICKET_URL", localDB)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeLocal {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeLocal)
	}
	if resolved.DBPath != localDB {
		t.Fatalf("DBPath = %q, want %q", resolved.DBPath, localDB)
	}
}

func TestLoadUsesEnvOverrideWhenRemoteTrioSet(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"file:///tmp/local.db","username":"local-user"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := SaveCredentials(Credentials{Token: "persisted-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	t.Setenv("TICKET_URL", "https://tickets.example.com")
	t.Setenv("TICKET_USERNAME", "alice")
	t.Setenv("TICKET_PASSWORD", "secret12")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://tickets.example.com" {
		t.Fatalf("Load().Location = %q, want env URL", cfg.Location)
	}
	if cfg.Username != "alice" {
		t.Fatalf("Load().Username = %q, want env username", cfg.Username)
	}
	if cfg.Token != "" {
		t.Fatalf("Load().Token = %q, want empty when env override is active", cfg.Token)
	}
}

func TestLoadUsesEnvLocationOverrideWithoutRemoteCredentials(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"https://tickets.example.com","username":"local-user"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	t.Setenv("TICKET_URL", filepath.Join(tempDir, "override.db"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != filepath.Join(tempDir, "override.db") {
		t.Fatalf("Load().Location = %q, want env location override", cfg.Location)
	}
	if cfg.Username != "local-user" {
		t.Fatalf("Load().Username = %q, want stored username to remain", cfg.Username)
	}
}

func TestLoadUsesStoredRemoteCredentialsWithEnvLocationOverride(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"file:///tmp/local.db"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := SaveCredentials(Credentials{
		Remotes: map[string]RemoteCredentials{
			"https://tickets.example.com": {Username: "alice", Token: "stored-token"},
		},
	}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	t.Setenv("TICKET_URL", "https://tickets.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://tickets.example.com" {
		t.Fatalf("Load().Location = %q, want env URL", cfg.Location)
	}
	if cfg.Username != "alice" {
		t.Fatalf("Load().Username = %q, want alice", cfg.Username)
	}
	if cfg.Token != "stored-token" {
		t.Fatalf("Load().Token = %q, want stored-token", cfg.Token)
	}
}

func TestHomeDefaultsToDotConfigTicket(t *testing.T) {
	t.Setenv("TICKET_HOME", "")

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	homeDir, _ := os.UserHomeDir()
	want := filepath.Join(homeDir, ".ticket")
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestHomeWalksUpToFindDotTicket(t *testing.T) {
	t.Setenv("TICKET_HOME", "")
	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	homeDir, _ := os.UserHomeDir()
	want := filepath.Join(homeDir, ".ticket")
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestHomeDefaultsToLocalDotTicketWhenNoneFound(t *testing.T) {
	t.Setenv("TICKET_HOME", "")

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	homeDir, _ := os.UserHomeDir()
	want := filepath.Join(homeDir, ".ticket")
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestHomeUsesTicketHome(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if got != tempDir {
		t.Fatalf("Home() = %q, want %q", got, tempDir)
	}
}

func TestCredentialsStoredSeparately(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	cfg := Config{Location: "http://example.test:9000", Username: "alice", Token: "sensitive"}
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

func TestLoadInvalidFieldTypeIsHealedAndSaved(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	// Write config where tui_cursor is an object instead of an int — invalid type.
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"location":"ticket.db","tui_cursor":{"bad":"value"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for repairable config", err)
	}
	// Valid field should survive; invalid field should be zeroed.
	if cfg.Location != "ticket.db" {
		t.Fatalf("Load().Location = %q, want %q", cfg.Location, "ticket.db")
	}
	if cfg.TUICursor != 0 {
		t.Fatalf("Load().TUICursor = %d, want 0 after bad value stripped", cfg.TUICursor)
	}

	// The file should have been rewritten without the bad field.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"bad"`) {
		t.Fatalf("config.json still contains bad value after healing: %s", data)
	}
}

func TestLoadInvalidJSONUsesDefaults(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for invalid JSON", err)
	}
	if cfg.Location != "" {
		t.Fatalf("Load().Location = %q, want empty default", cfg.Location)
	}
}
