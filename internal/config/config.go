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
	Location  string `json:"location"`
	Token     string `json:"token"`
	Username  string `json:"username"`
	ProjectID string `json:"project_id"`
	CurrentEpicID  string `json:"current_epic_id"`

	// TUI state — persisted between sessions by default.
	// Set TUIDisablePersist=true to skip save/restore.
	TUIDisablePersist bool    `json:"tui_disable_persist,omitempty"`
	TUITheme          string  `json:"tui_theme,omitempty"`
	TUIMode           string  `json:"tui_mode,omitempty"`   // "summary" | "projects" | "ideas" | "list" | "settings"
	TUICursor         int     `json:"tui_cursor,omitempty"`
	TUIExpandedEpics  []string `json:"tui_expanded_epics,omitempty"`

	// Temporary delete confirmation state
	DeleteConfirmToken   string `json:"delete_confirm_token,omitempty"`
	DeleteConfirmProject string `json:"delete_confirm_project,omitempty"`
	DeleteConfirmTicket  string `json:"delete_confirm_ticket,omitempty"`
}

type Credentials struct {
	Token string `json:"token"`
}

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// ResolveURL determines mode and target from the Location field in config.json.
//
//	file:///path/to/ticket.db  → local mode
//	http(s)://host             → remote mode
//	(empty)                    → local mode, DBPath = <Home()>/ticket.db
func ResolveURL() (Resolved, error) {
	cfg, _ := Load()
	return ResolveLocation(cfg.Location)
}

// ResolveLocation parses a location string into a Resolved struct.
// This is the core logic, separated so callers with an already-loaded config
// can avoid re-reading the file.
func ResolveLocation(location string) (Resolved, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		home, err := Home()
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{Mode: ModeLocal, DBPath: filepath.Join(home, "ticket.db")}, nil
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
		// No scheme — treat as a path relative to the .ticket/ directory.
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
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	// Use a raw map to handle type migrations (e.g. current_epic_id changed from int to string).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: config.json is not valid JSON (%v); using defaults\n", err)
		return Config{}, nil
	}
	// Fix current_epic_id: if stored as a number (legacy), convert to string.
	if v, ok := raw["current_epic_id"]; ok {
		s := strings.TrimSpace(string(v))
		if s != "" && s[0] != '"' {
			// It's a number literal (e.g. 0 or 42); wrap as string. Treat 0 as empty.
			if s == "0" {
				raw["current_epic_id"] = json.RawMessage(`""`)
			} else {
				raw["current_epic_id"] = json.RawMessage(`"` + s + `"`)
			}
		}
	}
	// Fix tui_expanded_epics: if stored as int array (legacy), convert to string array.
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
						json.Unmarshal(item, &sv)
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
		// Some field has an unexpected type. Strip invalid fields and warn.
		cleaned, removed := removeInvalidFields(raw)
		if len(removed) > 0 {
			fmt.Fprintf(os.Stderr, "warning: config.json has invalid values for %v; resetting those fields\n", removed)
		}
		cleanData, merr := json.Marshal(cleaned)
		if merr != nil {
			return Config{}, merr
		}
		if merr := json.Unmarshal(cleanData, &cfg); merr != nil {
			// Still broken — return empty config rather than failing.
			fmt.Fprintf(os.Stderr, "warning: config.json could not be parsed (%v); using defaults\n", merr)
			cfg = Config{}
		}
		// Persist the fixed config so future invocations are clean.
		if saveErr := saveRaw(path, cleaned); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save fixed config.json: %v\n", saveErr)
		}
	}
	creds, err := LoadCredentials()
	if err != nil {
		return Config{}, err
	}
	cfg.Token = creds.Token

	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	saved := cfg
	saved.Token = ""

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadCredentials() (Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}

	data, err := os.ReadFile(path)
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
	return creds, nil
}

func SaveCredentials(creds Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
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

// Path returns the path to the config file ($TICKET_HOME/config.json).
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

// Home returns the ticket home directory used for config and (in local mode) the database.
// Resolution order:
//  1. $TICKET_HOME if set
//  2. Walk up from CWD looking for .git/, then use .ticket/ as a sibling
//  3. ${CWD}/.ticket (default fallback, may not yet exist)
func Home() (string, error) {
	if dir := envValue("TICKET_HOME"); dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if gitRoot, ok := FindGitRoot(cwd); ok {
		return filepath.Join(gitRoot, ".ticket"), nil
	}
	return filepath.Join(cwd, ".ticket"), nil
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

// removeInvalidFields tries removing fields from raw one at a time until the
// map can be unmarshalled into Config without error. Returns the cleaned map
// and the names of fields that were removed.
func removeInvalidFields(raw map[string]json.RawMessage) (map[string]json.RawMessage, []string) {
	var removed []string
	for key := range raw {
		// Build a candidate map without this key.
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
			// Removing this key fixed the parse — keep it removed.
			removed = append(removed, key)
			raw = candidate
		}
	}
	return raw, removed
}

// saveRaw writes a raw map back to the config file as indented JSON,
// preserving only the data that's already been validated.
func saveRaw(path string, raw map[string]json.RawMessage) error {
	// Strip token from persisted config.
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
