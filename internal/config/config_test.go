package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")

	// Chdir to temp dir so local .ticket.json in the repo doesn't override values.
	origDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

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
}

func TestResolveURLDefaultsToLocal(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")

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

func TestResolveURLFileScheme(t *testing.T) {
	t.Setenv("TICKET_URL", "file:///tmp/test.db")
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

func TestResolveURLHTTPScheme(t *testing.T) {
	t.Setenv("TICKET_URL", "http://localhost:8080")
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
	t.Setenv("TICKET_URL", "https://tickets.example.com")
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

func TestResolveURLRejectsUnsupportedScheme(t *testing.T) {
	t.Setenv("TICKET_URL", "ftp://example.com")
	if _, err := ResolveURL(); err == nil {
		t.Fatal("ResolveURL() error = nil, want unsupported scheme error")
	}
}

func TestHomeDefaultsToDotConfigTicket(t *testing.T) {
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

func TestHomeUsesConfigDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if got != tempDir {
		t.Fatalf("Home() = %q, want %q", got, tempDir)
	}
}

func TestCredentialsStoredSeparately(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempDir)
	t.Setenv("TICKET_URL", "")

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

func TestFindLocalConfigWalksUp(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	// Place .ticket.json in root/a
	if err := SaveLocalConfig(filepath.Join(root, "a"), LocalConfig{CurrentProject: "FOO"}); err != nil {
		t.Fatal(err)
	}

	// Should find it from root/a/b/c
	lc, ok := FindLocalConfig(child)
	if !ok {
		t.Fatal("FindLocalConfig() returned false, want true")
	}
	if lc.CurrentProject != "FOO" {
		t.Fatalf("CurrentProject = %q, want FOO", lc.CurrentProject)
	}
	wantPath := filepath.Join(root, "a", LocalConfigFile)
	if lc.Path != wantPath {
		t.Fatalf("Path = %q, want %q", lc.Path, wantPath)
	}

	// Should not find it from root (sibling of a)
	_, ok = FindLocalConfig(root)
	if ok {
		t.Fatal("FindLocalConfig(root) returned true, want false")
	}
}

func TestLocalConfigOverridesGlobalProject(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("TICKET_CONFIG_DIR", tempHome)
	t.Setenv("TICKET_URL", "")

	// Save global config with project BAR
	cfg := Config{CurrentProject: "BAR"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	// Create a dir with .ticket.json pointing to FOO
	projDir := t.TempDir()
	if err := SaveLocalConfig(projDir, LocalConfig{CurrentProject: "FOO"}); err != nil {
		t.Fatal(err)
	}

	// Change to that dir and load
	origDir, _ := os.Getwd()
	if err := os.Chdir(projDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.CurrentProject != "FOO" {
		t.Fatalf("CurrentProject = %q, want FOO (local override)", loaded.CurrentProject)
	}
}

func TestSaveLocalConfig(t *testing.T) {
	dir := t.TempDir()
	lc := LocalConfig{CurrentProject: "ABC"}
	if err := SaveLocalConfig(dir, lc); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, LocalConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ABC") {
		t.Fatalf("saved file does not contain ABC: %s", data)
	}
}
