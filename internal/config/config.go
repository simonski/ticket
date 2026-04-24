package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	ModeLocal  = "local"
	ModeRemote = "remote"
)

// Resolved holds the parsed result of config.Location.
type Resolved struct {
	Mode      string // "local" or "remote"
	DBPath    string // populated when Mode == "local"
	ServerURL string // populated when Mode == "remote"
}

type Config struct {
	Location      string `json:"location"`
	Token         string `json:"token"`
	Username      string `json:"username"`
	ProjectID     string `json:"project_id"`
	CurrentEpicID string `json:"current_epic_id"`

	// TUI state — persisted between sessions by default.
	// Set TUIDisablePersist=true to skip save/restore.
	TUIDisablePersist bool     `json:"tui_disable_persist,omitempty"`
	TUITheme          string   `json:"tui_theme,omitempty"`
	TUIMode           string   `json:"tui_mode,omitempty"` // "summary" | "projects" | "ideas" | "list" | "settings"
	TUICursor         int      `json:"tui_cursor,omitempty"`
	TUIExpandedEpics  []string `json:"tui_expanded_epics,omitempty"`

	// Temporary delete confirmation state
	DeleteConfirmToken   string `json:"delete_confirm_token,omitempty"`
	DeleteConfirmProject string `json:"delete_confirm_project,omitempty"`
	DeleteConfirmTicket  string `json:"delete_confirm_ticket,omitempty"`
}

type RemoteCredentials struct {
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
}

type Credentials struct {
	// Legacy single-token field kept for backward compatibility.
	Token string `json:"token,omitempty"`

	Remotes map[string]RemoteCredentials `json:"remotes,omitempty"`
}

type projectDiskConfig struct {
	Version int    `json:"version,omitempty"`
	Backend string `json:"backend,omitempty"`
	Config
}

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// HasLocationEnvOverride returns true when the effective location is explicitly
// configured via TICKET_URL.
func HasLocationEnvOverride() bool {
	return envValue("TICKET_URL") != ""
}

// HasRemoteEnvOverride returns true when remote mode is explicitly configured
// via environment variables only.
func HasRemoteEnvOverride() bool {
	if !HasLocationEnvOverride() ||
		envValue("TICKET_USERNAME") == "" ||
		envValue("TICKET_PASSWORD") == "" {
		return false
	}
	resolved, err := ResolveLocation(envValue("TICKET_URL"))
	return err == nil && resolved.Mode == ModeRemote
}

// ResolveURL determines mode and target from the effective config.
func ResolveURL() (Resolved, error) {
	if HasLocationEnvOverride() {
		return ResolveLocation(envValue("TICKET_URL"))
	}
	cfg, err := Load()
	if err != nil {
		return Resolved{}, err
	}
	return ResolveLocation(cfg.Location)
}

// ResolveLocation parses a location string into a Resolved struct.
func ResolveLocation(location string) (Resolved, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		dbPath, err := LocalDBPath()
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{Mode: ModeLocal, DBPath: dbPath}, nil
	}
	u, err := url.Parse(location)
	if err != nil {
		return Resolved{}, fmt.Errorf("invalid location %q: %w", location, err)
	}
	switch u.Scheme {
	case "file":
		return Resolved{Mode: ModeLocal, DBPath: u.Path}, nil
	case "http", "https":
		return Resolved{Mode: ModeRemote, ServerURL: location}, nil
	case "":
		if filepath.IsAbs(location) {
			return Resolved{Mode: ModeLocal, DBPath: location}, nil
		}
		home, err := Home()
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{Mode: ModeLocal, DBPath: filepath.Join(home, location)}, nil
	default:
		return Resolved{}, fmt.Errorf("location scheme %q not supported (use file://, http://, or https://)", u.Scheme)
	}
}

func Load() (Config, error) {
	globalPath, err := Path()
	if err != nil {
		return Config{}, err
	}
	globalCfg, err := loadConfigFile(globalPath)
	if err != nil {
		return Config{}, err
	}

	cfg := globalCfg
	projectPath, hasProject, err := ProjectPath()
	if err != nil {
		return Config{}, err
	}
	if hasProject {
		projectCfg, err := loadConfigFile(projectPath)
		if err != nil {
			return Config{}, err
		}
		if strings.TrimSpace(projectCfg.Location) != "" {
			cfg.Location = projectCfg.Location
		}
		if strings.TrimSpace(cfg.ProjectID) == "" {
			cfg.ProjectID = projectCfg.ProjectID
		}
		cfg.CurrentEpicID = projectCfg.CurrentEpicID
		cfg.DeleteConfirmToken = projectCfg.DeleteConfirmToken
		cfg.DeleteConfirmProject = projectCfg.DeleteConfirmProject
		cfg.DeleteConfirmTicket = projectCfg.DeleteConfirmTicket
	}
	if HasLocationEnvOverride() {
		cfg.Location = envValue("TICKET_URL")
	}

	creds, err := LoadCredentials()
	if err != nil {
		return Config{}, err
	}
	if resolved, rErr := ResolveLocation(cfg.Location); rErr == nil && resolved.Mode == ModeRemote {
		if remote, ok := creds.Remote(cfg.Location); ok {
			if strings.TrimSpace(cfg.Username) == "" {
				cfg.Username = remote.Username
			}
			cfg.Token = remote.Token
		} else {
			cfg.Token = creds.Token
		}
	}
	if HasRemoteEnvOverride() {
		cfg.Username = envValue("TICKET_USERNAME")
		cfg.Token = ""
	}

	return cfg, nil
}

func Save(cfg Config) error {
	globalPath, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o750); err != nil {
		return err
	}

	projectPath, hasProject, err := ProjectPath()
	if err != nil {
		return err
	}
	if hasProject {
		projectCfg, err := loadConfigFile(projectPath)
		if err != nil {
			return err
		}
		projectCfg.CurrentEpicID = cfg.CurrentEpicID
		projectCfg.DeleteConfirmToken = cfg.DeleteConfirmToken
		projectCfg.DeleteConfirmProject = cfg.DeleteConfirmProject
		projectCfg.DeleteConfirmTicket = cfg.DeleteConfirmTicket
		if err := saveProjectConfig(projectPath, projectCfg); err != nil {
			return err
		}
	}

	saved := cfg
	saved.Token = ""
	if hasProject {
		saved.CurrentEpicID = ""
		saved.DeleteConfirmToken = ""
		saved.DeleteConfirmProject = ""
		saved.DeleteConfirmTicket = ""
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(globalPath, data, 0o600)
}

func LoadCredentials() (Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is resolved from application state
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, err
	}
	if creds.Remotes == nil {
		creds.Remotes = map[string]RemoteCredentials{}
	}
	return creds, nil
}

func (c Credentials) Remote(location string) (RemoteCredentials, bool) {
	if c.Remotes == nil {
		return RemoteCredentials{}, false
	}
	remote, ok := c.Remotes[strings.TrimSpace(location)]
	return remote, ok
}

func SaveCredentials(creds Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func SaveRemoteCredentials(location, username, token string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	if creds.Remotes == nil {
		creds.Remotes = map[string]RemoteCredentials{}
	}
	location = strings.TrimSpace(location)
	if location == "" {
		creds.Token = token
		return SaveCredentials(creds)
	}
	creds.Remotes[location] = RemoteCredentials{
		Username: strings.TrimSpace(username),
		Token:    strings.TrimSpace(token),
	}
	if creds.Token == "" {
		creds.Token = strings.TrimSpace(token)
	}
	return SaveCredentials(creds)
}

func ClearCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ClearRemoteCredentials(location string) error {
	location = strings.TrimSpace(location)
	if location == "" {
		return ClearCredentials()
	}
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	if creds.Remotes == nil {
		return nil
	}
	delete(creds.Remotes, location)
	if len(creds.Remotes) == 0 {
		return ClearCredentials()
	}
	return SaveCredentials(creds)
}

// Path returns the path to the global config file ($TICKET_HOME/config.json).
func Path() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

func CredentialsPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "credentials.json"), nil
}

func LocalDBPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "ticket.db"), nil
}

// Home returns the global Ticket home directory used for credentials,
// global config, and the central local database.
func Home() (string, error) {
	if dir := envValue("TICKET_HOME"); dir != "" {
		return dir, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, ".ticket"), nil
}

// ProjectPath returns the nearest project-local .ticket/config.json path.
func ProjectPath() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	return ProjectPathFrom(cwd)
}

func ProjectPathFrom(startDir string) (string, bool, error) {
	root, ok := FindTicketRoot(startDir)
	if !ok {
		return "", false, nil
	}
	return ProjectPathAtRoot(root), true, nil
}

func ProjectPathAtRoot(root string) string {
	return filepath.Join(root, ".ticket", "config.json")
}

func SaveProjectConfigAt(root string, cfg Config) error {
	return saveProjectConfig(ProjectPathAtRoot(root), cfg)
}

// FindTicketRoot walks up the directory tree from startDir looking for a
// .ticket directory. Returns the parent of .ticket/.
func FindTicketRoot(startDir string) (string, bool) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".ticket")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// FindGitRoot walks up the directory tree from startDir looking for a .git
// directory. Returns the parent of .git/ (the project root). Stops at the
// filesystem root.
func FindGitRoot(startDir string) (string, bool) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".git")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func loadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is resolved from application state
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: config at %s is not valid JSON (%v); using defaults\n", path, err)
		return Config{}, nil
	}
	if v, ok := raw["current_epic_id"]; ok {
		s := strings.TrimSpace(string(v))
		if s != "" && s[0] != '"' {
			if s == "0" {
				raw["current_epic_id"] = json.RawMessage(`""`)
			} else {
				raw["current_epic_id"] = json.RawMessage(`"` + s + `"`)
			}
		}
	}
	if v, ok := raw["tui_expanded_epics"]; ok {
		s := strings.TrimSpace(string(v))
		if len(s) > 0 && s[0] == '[' {
			var items []json.RawMessage
			if json.Unmarshal(v, &items) == nil {
				converted := make([]string, 0, len(items))
				for _, item := range items {
					is := strings.TrimSpace(string(item))
					if len(is) > 0 && is[0] == '"' {
						var sv string
						_ = json.Unmarshal(item, &sv)
						converted = append(converted, sv)
					} else {
						converted = append(converted, strings.Trim(is, " "))
					}
				}
				if b, err := json.Marshal(converted); err == nil {
					raw["tui_expanded_epics"] = json.RawMessage(b)
				}
			}
		}
	}
	fixedData, err := json.Marshal(raw)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(fixedData, &cfg); err != nil {
		cleaned, removed := removeInvalidFields(raw)
		if len(removed) > 0 {
			fmt.Fprintf(os.Stderr, "warning: config at %s has invalid values for %v; resetting those fields\n", path, removed)
		}
		cleanData, merr := json.Marshal(cleaned)
		if merr != nil {
			return Config{}, merr
		}
		if merr := json.Unmarshal(cleanData, &cfg); merr != nil {
			fmt.Fprintf(os.Stderr, "warning: config at %s could not be parsed (%v); using defaults\n", path, merr)
			cfg = Config{}
		}
		if saveErr := saveRaw(path, cleaned); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save fixed config at %s: %v\n", path, saveErr)
		}
	}
	return cfg, nil
}

func saveProjectConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	disk := projectDiskConfig{
		Version: 1,
		Config: Config{
			ProjectID:            cfg.ProjectID,
			CurrentEpicID:        cfg.CurrentEpicID,
			DeleteConfirmToken:   cfg.DeleteConfirmToken,
			DeleteConfirmProject: cfg.DeleteConfirmProject,
			DeleteConfirmTicket:  cfg.DeleteConfirmTicket,
		},
	}
	resolved, err := ResolveLocation(cfg.Location)
	if err == nil {
		disk.Backend = resolved.Mode
		if resolved.Mode == ModeRemote {
			disk.Location = strings.TrimSpace(cfg.Location)
		}
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// removeInvalidFields tries removing fields from raw one at a time until the
// map can be unmarshalled into Config without error. Returns the cleaned map
// and the names of fields that were removed.
func removeInvalidFields(raw map[string]json.RawMessage) (map[string]json.RawMessage, []string) {
	var removed []string
	for key := range raw {
		candidate := make(map[string]json.RawMessage, len(raw))
		for k, v := range raw {
			if k != key {
				candidate[k] = v
			}
		}
		data, err := json.Marshal(candidate)
		if err != nil {
			continue
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			removed = append(removed, key)
			raw = candidate
		}
	}
	return raw, removed
}

// saveRaw writes a raw map back to the config file as indented JSON,
// preserving only the data that's already been validated.
func saveRaw(path string, raw map[string]json.RawMessage) error {
	cleaned := make(map[string]json.RawMessage, len(raw))
	for k, v := range raw {
		if k != "token" {
			cleaned[k] = v
		}
	}
	data, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
