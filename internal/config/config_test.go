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
	tempDir := setupConfigTestHome(t)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{
		"default_remote":"prod",
		"remotes":[{"name":"prod","url":"https://tickets.example.com"}]
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
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

func TestLoadUsesProjectRemoteBinding(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{
		"default_remote":"local",
		"remotes":[
			{"name":"local","url":"file:///tmp/local.db"},
			{"name":"prod","url":"https://tickets.example.com"}
		]
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config.json) error = %v", err)
	}
	if err := SaveProjectConfigAt(repoDir, Config{Remote: "prod", ProjectID: "CUS"}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://tickets.example.com" {
		t.Fatalf("Load().Location = %q, want project remote URL", cfg.Location)
	}
	if cfg.Remote != "prod" {
		t.Fatalf("Load().Remote = %q, want prod", cfg.Remote)
	}
	if cfg.ProjectID != "CUS" {
		t.Fatalf("Load().ProjectID = %q, want CUS", cfg.ProjectID)
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

func TestSaveProjectConfigAtStripsAuthFields(t *testing.T) {
	setupConfigTestHome(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir) error = %v", err)
	}
	if err := SaveProjectConfigAt(repoDir, Config{
		Location:  "https://tickets.example.com",
		Username:  "alice",
		Token:     "secret-token",
		ProjectID: "1",
	}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}

	data, err := os.ReadFile(ProjectPathAtRoot(repoDir))
	if err != nil {
		t.Fatalf("ReadFile(project config) error = %v", err)
	}
	got := string(data)
	for _, unwanted := range []string{"username", "alice", "token", "secret-token"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("project config should not contain %q:\n%s", unwanted, got)
		}
	}
	for _, want := range []string{"https://tickets.example.com", "\"project_id\": \"1\""} {
		if !strings.Contains(got, want) {
			t.Fatalf("project config missing %q:\n%s", want, got)
		}
	}
}

func TestLoadStripsLegacyProjectAuthFields(t *testing.T) {
	setupConfigTestHome(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	projectPath := ProjectPathAtRoot(repoDir)
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(project config dir) error = %v", err)
	}
	raw := `{"version":1,"backend":"remote","location":"https://tickets.example.com","username":"alice","token":"secret-token","project_id":"1"}`
	if err := os.WriteFile(projectPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(project config) error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Location != "https://tickets.example.com" {
		t.Fatalf("Load().Location = %q, want remote location", cfg.Location)
	}
	if cfg.Username != "" {
		t.Fatalf("Load().Username = %q, want empty because project config auth should be ignored", cfg.Username)
	}
	if cfg.Token != "" {
		t.Fatalf("Load().Token = %q, want empty because project config auth should be ignored", cfg.Token)
	}

	data, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("ReadFile(project config) error = %v", err)
	}
	got := string(data)
	for _, unwanted := range []string{"username", "alice", "token", "secret-token"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("healed project config should not contain %q:\n%s", unwanted, got)
		}
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
	if creds.Token != "secret" {
		t.Fatalf("Credentials.Token = %q, want legacy token mirror", creds.Token)
	}

	if err := ClearRemoteCredentials("https://tickets.example.com"); err != nil {
		t.Fatalf("ClearRemoteCredentials() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "credentials.json")); !os.IsNotExist(err) {
		t.Fatalf("credentials.json should be removed after clearing last remote, err = %v", err)
	}
}

func TestSaveWithProjectConfigSplitsGlobalAndProjectFields(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	projectRoot := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".ticket"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.ticket) error = %v", err)
	}
	if err := SaveProjectConfigAt(projectRoot, Config{}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Chdir(projectRoot) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := Save(Config{
		DefaultRemote:        "prod",
		Remotes:              []Remote{{Name: "prod", URL: "https://tickets.example.com"}},
		Remote:               "prod",
		Location:             "https://tickets.example.com",
		ProjectID:            "CUS",
		CurrentEpicID:        "T-1",
		DeleteConfirmToken:   "token",
		DeleteConfirmProject: "CUS",
		DeleteConfirmTicket:  "CUS-1",
		Username:             "alice",
		Token:                "secret",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	globalData, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile(global config) error = %v", err)
	}
	if strings.Contains(string(globalData), "secret") || strings.Contains(string(globalData), `"remote": "prod"`) || strings.Contains(string(globalData), `"project_id": "CUS"`) {
		t.Fatalf("global config leaked project/auth fields: %s", string(globalData))
	}

	projectData, err := os.ReadFile(filepath.Join(projectRoot, ".ticket", "config.json"))
	if err != nil {
		t.Fatalf("ReadFile(project config) error = %v", err)
	}
	if !strings.Contains(string(projectData), `"current_epic_id": "T-1"`) || !strings.Contains(string(projectData), `"delete_confirm_project": "CUS"`) {
		t.Fatalf("project config = %s, want project-only state fields", string(projectData))
	}
	if strings.Contains(string(projectData), "secret") {
		t.Fatalf("project config leaked token: %s", string(projectData))
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
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".ticket"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.ticket) error = %v", err)
	}
	if err := SaveProjectConfigAt(repoDir, Config{}); err != nil {
		t.Fatalf("SaveProjectConfigAt() error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if got, err := Path(); err != nil || got != filepath.Join(tempDir, "config.json") {
		t.Fatalf("Path() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "config.json"))
	}
	if got, err := CredentialsPath(); err != nil || got != filepath.Join(tempDir, "credentials.json") {
		t.Fatalf("CredentialsPath() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "credentials.json"))
	}
	if got, err := LocalDBPath(); err != nil || got != filepath.Join(tempDir, "ticket.db") {
		t.Fatalf("LocalDBPath() = (%q, %v), want %q", got, err, filepath.Join(tempDir, "ticket.db"))
	}
	if got, ok, err := ProjectPath(); err != nil || !ok || !strings.HasSuffix(got, filepath.Join("repo", ".ticket", "config.json")) {
		t.Fatalf("ProjectPath() = (%q, %v, %v), want repo-local .ticket config path", got, ok, err)
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

func TestProjectPathReturnsFalseWithoutTicketDir(t *testing.T) {
	tempDir := setupConfigTestHome(t)
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if got, ok, err := ProjectPath(); err != nil || ok || got != "" {
		t.Fatalf("ProjectPath() = (%q, %v, %v), want empty false nil", got, ok, err)
	}
}

func TestProjectPathIgnoresAncestorTicketHomeWithoutProjectConfig(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	repoDir := filepath.Join(homeDir, "code", "repo")
	if err := os.MkdirAll(filepath.Join(homeDir, ".ticket"), 0o755); err != nil {
		t.Fatalf("MkdirAll(home .ticket) error = %v", err)
	}
	t.Setenv("TICKET_HOME", filepath.Join(homeDir, ".ticket"))
	if err := os.WriteFile(filepath.Join(homeDir, ".ticket", "config.json"), []byte(`{"default_remote":"local"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(global config) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(repo .git) error = %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir(repoDir) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if got, ok, err := ProjectPath(); err != nil || ok || got != "" {
		t.Fatalf("ProjectPath() with ancestor home .ticket = (%q, %v, %v), want empty false nil", got, ok, err)
	}
}
