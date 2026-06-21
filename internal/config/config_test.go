package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func setupConfigTestHome(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("TICKET_HOME", tempDir)
	t.Setenv("TICKET_URL", "")
	t.Setenv("TICKET_USERNAME", "")
	t.Setenv("TICKET_TOKEN", "")
	t.Setenv("TICKET_PASSWORD", "")
	t.Setenv("TICKET_PROJECT", "")
	ClearLocationOverride()
	t.Cleanup(ClearLocationOverride)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(tempDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return tempDir
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	cfg := Config{Location: "http://example.test:9000", TUITheme: "night"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Location != "" {
		t.Fatalf("Load().Location = %q, want empty because server config is not persisted", got.Location)
	}
	if got.TUITheme != cfg.TUITheme {
		t.Fatalf("Load().TUITheme = %q, want %q", got.TUITheme, cfg.TUITheme)
	}

	path := filepath.Join(tempDir, "preferences.json")
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected preferences file at %s: %v", path, statErr)
	}

	if got.Token != "" {
		t.Fatalf("Load().Token = %q, want empty because credentials are stored separately", got.Token)
	}

	got.ProjectID = "2"
	if saveErr := Save(got); saveErr != nil {
		t.Fatalf("Save(updated) error = %v", saveErr)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load(reloaded) error = %v", err)
	}
	if reloaded.ProjectID != "" {
		t.Fatalf("Load().ProjectID = %q, want empty because project selection is not persisted globally", reloaded.ProjectID)
	}
}

func TestLoadMigratesLegacyConfig(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"tui_theme":"night"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TUITheme != "night" {
		t.Fatalf("Load().TUITheme = %q, want night", cfg.TUITheme)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("legacy config.json should be removed after migration, err = %v", err)
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
	setupConfigTestHome(t)
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
	setupConfigTestHome(t)
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

func TestResolveURLFileScheme(t *testing.T) {
	setupConfigTestHome(t)
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

func TestResolveURLRejectsUnsupportedScheme(t *testing.T) {
	setupConfigTestHome(t)
	t.Setenv("TICKET_URL", "ftp://example.com")

	if _, err := ResolveURL(); err == nil {
		t.Fatal("ResolveURL() error = nil, want unsupported scheme error")
	}
}

func TestResolveURLUsesProcessLocationOverrideForRemote(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"file:///tmp/local.db"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	SetLocationOverride("https://tickets.example.com")
	t.Cleanup(ClearLocationOverride)

	resolved, err := ResolveURL()
	if err != nil {
		t.Fatalf("ResolveURL() error = %v", err)
	}
	if resolved.Mode != ModeRemote {
		t.Fatalf("Mode = %q, want %q", resolved.Mode, ModeRemote)
	}
	if resolved.ServerURL != "https://tickets.example.com" {
		t.Fatalf("ServerURL = %q, want override URL", resolved.ServerURL)
	}
}

func TestResolveURLUsesProcessLocationOverrideForLocalPath(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	localDB := filepath.Join(tempDir, "override.db")
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"location":"https://tickets.example.com"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}

	SetLocationOverride(localDB)
	t.Cleanup(ClearLocationOverride)

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

func TestLoadUsesDefaultRemoteAndStoredCredentials(t *testing.T) {
	setupConfigTestHome(t)
	t.Setenv("TICKET_URL", "https://tickets.example.com")
	if err := SaveCredentials(Credentials{
		Remotes: map[string]RemoteCredentials{
			"https://tickets.example.com": {Username: "alice", Token: "stored-token"},
		},
	}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://tickets.example.com" {
		t.Fatalf("Load().Location = %q, want default remote URL", cfg.Location)
	}
	if cfg.Username != "alice" {
		t.Fatalf("Load().Username = %q, want stored username", cfg.Username)
	}
	if cfg.Token != "stored-token" {
		t.Fatalf("Load().Token = %q, want stored-token", cfg.Token)
	}
}

func TestLoadFallsBackToDefaultRemoteForStoredCredentials(t *testing.T) {
	// Reproduces the case where TICKET_URL is unset and no location is stored
	// in preferences, but credentials were saved under the default remote URL
	// (as `tk login` does). Load() must still surface that stored token.
	setupConfigTestHome(t)
	t.Setenv("TICKET_URL", "")

	prev := DefaultRemoteURL
	DefaultRemoteURL = "https://ticket.example.com"
	t.Cleanup(func() { DefaultRemoteURL = prev })

	if err := SaveCredentials(Credentials{
		Remotes: map[string]RemoteCredentials{
			"https://ticket.example.com": {Username: "admin", Token: "stored-token"},
		},
	}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://ticket.example.com" {
		t.Fatalf("Load().Location = %q, want default remote URL", cfg.Location)
	}
	if cfg.Username != "admin" {
		t.Fatalf("Load().Username = %q, want stored username", cfg.Username)
	}
	if cfg.Token != "stored-token" {
		t.Fatalf("Load().Token = %q, want stored token", cfg.Token)
	}
}

func TestHomeDefaultsToDotConfigTicket(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("TICKET_HOME", "")
	t.Setenv("HOME", tempHome)

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	want := filepath.Join(tempHome, ".config", "ticket")
	if got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestHomeIgnoresLegacyDotTicket(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("TICKET_HOME", "")
	t.Setenv("HOME", tempHome)

	// A legacy ~/.ticket directory is no longer consulted; Home() always
	// resolves to ~/.config/ticket.
	if err := os.MkdirAll(filepath.Join(tempHome, ".ticket"), 0o700); err != nil {
		t.Fatalf("MkdirAll(legacy) error = %v", err)
	}

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	want := filepath.Join(tempHome, ".config", "ticket")
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
	t.Setenv("TICKET_URL", "http://example.test:9000")
	if err := SaveCredentials(Credentials{Token: "session-token"}); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "preferences.json")); !os.IsNotExist(err) {
		t.Fatalf("preferences.json should not be created by auth-only saves, err = %v", err)
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
	if err := os.WriteFile(configPath, []byte(`{"tui_theme":"night","tui_cursor":{"bad":"value"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for repairable config", err)
	}
	// Valid field should survive; invalid field should be zeroed.
	if cfg.TUITheme != "night" {
		t.Fatalf("Load().TUITheme = %q, want %q", cfg.TUITheme, "night")
	}
	if cfg.TUICursor != 0 {
		t.Fatalf("Load().TUICursor = %d, want 0 after bad value stripped", cfg.TUICursor)
	}

	// The migrated preferences file should have been rewritten without the bad field.
	data, err := os.ReadFile(filepath.Join(tempDir, "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"bad"`) {
		t.Fatalf("preferences.json still contains bad value after healing: %s", data)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("legacy config.json should be removed after migration, err = %v", err)
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

func TestCanonicalizeRemoteURL(t *testing.T) {
	setupConfigTestHome(t)

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "remote lowercases scheme and host", input: "HTTPS://Tickets.EXAMPLE.com/", want: "https://tickets.example.com"},
		{name: "unsupported scheme", input: "ftp://example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalizeRemoteURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("CanonicalizeRemoteURL() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("CanonicalizeRemoteURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("CanonicalizeRemoteURL() = %q, want %q", got, tt.want)
			}
		})
	}

	got, err := CanonicalizeRemoteURL("ticket.db")
	if err != nil {
		t.Fatalf("CanonicalizeRemoteURL(relative path) error = %v", err)
	}
	if !strings.HasPrefix(got, "file://") || !strings.HasSuffix(got, "/ticket.db") {
		t.Fatalf("CanonicalizeRemoteURL(relative path) = %q, want file://.../ticket.db", got)
	}
}

func TestCanonicalizeGitRepository(t *testing.T) {
	t.Run("canonicalizes remote forms", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			want  string
		}{
			{name: "host path", input: "github.com/acme/repo.git", want: "github.com/acme/repo"},
			{name: "https", input: "https://github.com/acme/repo.git", want: "github.com/acme/repo"},
			{name: "ssh url", input: "ssh://git@github.com/acme/repo.git/", want: "github.com/acme/repo"},
			{name: "scp style", input: "git@github.com:acme/repo.git", want: "github.com/acme/repo"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := CanonicalizeGitRepository(tt.input)
				if err != nil {
					t.Fatalf("CanonicalizeGitRepository() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("CanonicalizeGitRepository() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("canonicalizes local symlinked paths", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "origin.git")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("MkdirAll(target) error = %v", err)
		}
		link := filepath.Join(t.TempDir(), "origin-link.git")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}
		got, err := CanonicalizeGitRepository("file://" + link)
		if err != nil {
			t.Fatalf("CanonicalizeGitRepository(file symlink) error = %v", err)
		}
		want, err := CanonicalizeGitRepository(target)
		if err != nil {
			t.Fatalf("CanonicalizeGitRepository(target) error = %v", err)
		}
		if got != want {
			t.Fatalf("CanonicalizeGitRepository(file symlink) = %q, want %q", got, want)
		}
	})
}

func TestAddRemoteRejectsDuplicateNameAndURL(t *testing.T) {
	setupConfigTestHome(t)

	cfg, err := AddRemote(Config{}, Remote{Name: "prod", URL: "https://tickets.example.com"})
	if err != nil {
		t.Fatalf("AddRemote(first) error = %v", err)
	}
	if _, err := AddRemote(cfg, Remote{Name: "prod", URL: "https://other.example.com"}); err == nil {
		t.Fatal("AddRemote(duplicate name) error = nil, want error")
	}
	if _, err := AddRemote(cfg, Remote{Name: "prod-2", URL: "https://tickets.example.com/"}); err == nil {
		t.Fatal("AddRemote(duplicate URL) error = nil, want error")
	}
}

func TestRemoveRemoteClearsDefaultRemote(t *testing.T) {
	cfg := Config{
		DefaultRemote: "prod",
		Remotes: []Remote{
			{Name: "local", URL: "file:///tmp/local.db"},
			{Name: "prod", URL: "https://tickets.example.com"},
		},
	}

	got, removed := RemoveRemote(cfg, "prod")
	if !removed {
		t.Fatal("RemoveRemote() removed = false, want true")
	}
	if got.DefaultRemote != "" {
		t.Fatalf("RemoveRemote().DefaultRemote = %q, want empty", got.DefaultRemote)
	}
	if len(got.Remotes) != 1 || got.Remotes[0].Name != "local" {
		t.Fatalf("RemoveRemote().Remotes = %#v, want only local", got.Remotes)
	}
}

func TestSortUniqueRemotesNormalizesAndDeduplicates(t *testing.T) {
	setupConfigTestHome(t)

	got := sortUniqueRemotes([]Remote{
		{Name: "prod", URL: "https://tickets.example.com/"},
		{Name: "prod", URL: "https://tickets.example.com"},
		{Name: "local", URL: "ticket.db"},
		{Name: "invalid", URL: "ftp://example.com"},
	})
	if len(got) != 2 {
		t.Fatalf("sortUniqueRemotes() len = %d, want 2 (%#v)", len(got), got)
	}
	want := []Remote{
		{Name: "local", URL: got[0].URL},
		{Name: "prod", URL: "https://tickets.example.com"},
	}
	if !strings.HasPrefix(got[0].URL, "file://") || !strings.HasSuffix(got[0].URL, "/ticket.db") {
		t.Fatalf("sortUniqueRemotes() local URL = %q, want file://.../ticket.db", got[0].URL)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortUniqueRemotes() = %#v, want %#v", got, want)
	}
}

func TestSaveAndClearRemoteCredentials(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	if err := SaveRemoteCredentials("https://tickets.example.com/", "alice", "secret"); err != nil {
		t.Fatalf("SaveRemoteCredentials() error = %v", err)
	}

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	remote, ok := creds.Remote("https://tickets.example.com")
	if !ok {
		t.Fatal("Credentials.Remote() ok = false, want true")
	}
	if remote.Username != "alice" || remote.Token != "secret" {
		t.Fatalf("remote creds = %#v, want alice/secret", remote)
	}
	if creds.Token != "" {
		t.Fatalf("Credentials.Token = %q, want empty for remote-scoped credentials", creds.Token)
	}

	if err := ClearRemoteCredentials("https://tickets.example.com"); err != nil {
		t.Fatalf("ClearRemoteCredentials() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "credentials.json")); !os.IsNotExist(err) {
		t.Fatalf("credentials.json should be removed after clearing last remote, err = %v", err)
	}
}

func TestSavePreferencesDoesNotPersistServerOrProjectFields(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	if err := Save(Config{
		TUITheme:  "night",
		TUIMode:   "summary",
		Location:  "https://tickets.example.com",
		ProjectID: "CUS",
		Username:  "alice",
		Token:     "secret",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	globalData, err := os.ReadFile(filepath.Join(tempDir, "preferences.json"))
	if err != nil {
		t.Fatalf("ReadFile(preferences) error = %v", err)
	}
	for _, unwanted := range []string{"secret", `"project_id"`, "tickets.example.com", "alice"} {
		if strings.Contains(string(globalData), unwanted) {
			t.Fatalf("preferences leaked field %q: %s", unwanted, string(globalData))
		}
	}
	for _, want := range []string{"night", "summary"} {
		if !strings.Contains(string(globalData), want) {
			t.Fatalf("preferences missing %q: %s", want, string(globalData))
		}
	}
}

func TestConfigRemoteByURLMatchesCanonicalForms(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Remotes: []Remote{
			{Name: "prod", URL: "HTTPS://Tickets.Example.com/"},
			{Name: "local", URL: "file:///tmp/ticket.db"},
		},
	}

	remote, ok := cfg.RemoteByURL("https://tickets.example.com")
	if !ok || remote.Name != "prod" {
		t.Fatalf("RemoteByURL(remote) = (%#v, %v), want prod", remote, ok)
	}

	remote, ok = cfg.RemoteByURL("https://missing.example.com")
	if ok {
		t.Fatalf("RemoteByURL(missing) = (%#v, %v), want not found", remote, ok)
	}
}

func TestFindGitRootWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git) error = %v", err)
	}

	got, ok := FindGitRoot(nested)
	if !ok || got != root {
		t.Fatalf("FindGitRoot() = (%q, %v), want (%q, true)", got, ok, root)
	}

	if got, ok := FindGitRoot(t.TempDir()); ok || got != "" {
		t.Fatalf("FindGitRoot(no git) = (%q, %v), want empty false", got, ok)
	}
}

func TestPathsResolveUnderTicketHome(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	if got, err := Path(); err != nil || got != filepath.Join(tempDir, "preferences.json") {
		t.Fatalf("Path() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "preferences.json"))
	}
	if got, err := CredentialsPath(); err != nil || got != filepath.Join(tempDir, "credentials.json") {
		t.Fatalf("CredentialsPath() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "credentials.json"))
	}
	if got, err := LocalDBPath(); err != nil || got != filepath.Join(tempDir, "ticket.db") {
		t.Fatalf("LocalDBPath() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "ticket.db"))
	}
}

func TestRemoteCredentialsLegacyAndMissingCases(t *testing.T) {
	setupConfigTestHome(t)

	if err := SaveRemoteCredentials("", "alice", "legacy-token"); err != nil {
		t.Fatalf("SaveRemoteCredentials(empty) error = %v", err)
	}
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if creds.Token != "legacy-token" {
		t.Fatalf("Credentials.Token = %q, want legacy-token", creds.Token)
	}
	if _, ok := creds.Remote("::::"); ok {
		t.Fatal("Credentials.Remote(invalid) = true, want false")
	}
	if err := ClearRemoteCredentials("https://missing.example.com"); err != nil {
		t.Fatalf("ClearRemoteCredentials(missing) error = %v", err)
	}
}

func TestSaveCredentialsRoundTripAndClearMissingFile(t *testing.T) {
	tempDir := setupConfigTestHome(t)

	want := Credentials{
		Token: "legacy",
		Remotes: map[string]RemoteCredentials{
			"https://tickets.example.com": {Username: "alice", Token: "token-1"},
		},
	}
	if err := SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadCredentials() = %#v, want %#v", got, want)
	}
	if err := os.Remove(filepath.Join(tempDir, "credentials.json")); err != nil {
		t.Fatalf("Remove(credentials.json) error = %v", err)
	}
	if err := ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials(missing) error = %v", err)
	}
}

func TestRemoteHelpersAndValidationBranches(t *testing.T) {
	setupConfigTestHome(t)

	cfg := Config{
		Remotes: []Remote{{Name: "prod", URL: "https://tickets.example.com"}},
	}
	if remote, ok := cfg.RemoteByName("prod"); !ok || remote.URL != "https://tickets.example.com" {
		t.Fatalf("RemoteByName() = (%#v, %v), want prod remote", remote, ok)
	}
	if _, ok := cfg.RemoteByName("missing"); ok {
		t.Fatal("RemoteByName(missing) = true, want false")
	}
	if _, err := AddRemote(Config{}, Remote{Name: "", URL: "https://tickets.example.com"}); err == nil {
		t.Fatal("AddRemote(empty name) error = nil, want error")
	}
	if _, err := AddRemote(Config{}, Remote{Name: "prod", URL: ""}); err == nil {
		t.Fatal("AddRemote(empty url) error = nil, want error")
	}
	if _, removed := RemoveRemote(cfg, ""); removed {
		t.Fatal("RemoveRemote(empty) removed = true, want false")
	}
}
