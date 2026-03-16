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

// Resolved holds the parsed result of TICKET_URL.
type Resolved struct {
	Mode      string // "local" or "remote"
	DBPath    string // populated when Mode == "local"
	ServerURL string // populated when Mode == "remote"
}

type Config struct {
	ServerURL      string `json:"server_url"`
	Token          string `json:"token"`
	Username       string `json:"username"`
	CurrentProject string `json:"current_project"`
	CurrentEpicID  int64  `json:"current_epic_id"`
}

// LocalConfig is a per-directory config file (.ticket.json) that binds a
// directory tree to a specific project.
type LocalConfig struct {
	CurrentProject string `json:"current_project"`
	Path           string `json:"-"` // filesystem path where this was found
}

type Credentials struct {
	Token string `json:"token"`
}

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// ResolveURL parses TICKET_URL to determine mode and target.
//
//	file:///path/to/ticket.db  → local mode, DBPath = /path/to/ticket.db
//	http://host or https://host → remote mode, ServerURL = the URL
//	(unset)                    → local mode, DBPath = <config_home>/ticket.db
func ResolveURL() (Resolved, error) {
	raw := envValue("TICKET_URL")
	if raw == "" {
		// default: local mode, db in config home
		home, err := Home()
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{
			Mode:   ModeLocal,
			DBPath: filepath.Join(home, "ticket.db"),
		}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Resolved{}, fmt.Errorf("invalid TICKET_URL %q: %w", raw, err)
	}

	switch u.Scheme {
	case "file":
		dbPath := u.Path
		if dbPath == "" {
			return Resolved{}, fmt.Errorf("TICKET_URL file:// requires a path")
		}
		return Resolved{Mode: ModeLocal, DBPath: dbPath}, nil
	case "http", "https":
		return Resolved{Mode: ModeRemote, ServerURL: raw}, nil
	default:
		return Resolved{}, fmt.Errorf("TICKET_URL scheme %q not supported (use file://, http://, or https://)", u.Scheme)
	}
}

const LocalConfigFile = ".ticket.json"

// FindLocalConfig walks from startDir up to the user's home directory looking
// for a .ticket.json file. Returns the parsed config and true if found.
func FindLocalConfig(startDir string) (LocalConfig, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return LocalConfig{}, false
	}
	dir := startDir
	for {
		p := filepath.Join(dir, LocalConfigFile)
		data, err := os.ReadFile(p)
		if err == nil {
			var lc LocalConfig
			if json.Unmarshal(data, &lc) == nil && lc.CurrentProject != "" {
				lc.Path = p
				return lc, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		// stop after checking $HOME
		if dir == homeDir {
			break
		}
		dir = parent
	}
	return LocalConfig{}, false
}

// SaveLocalConfig writes a .ticket.json in the given directory.
func SaveLocalConfig(dir string, lc LocalConfig) error {
	data, err := json.MarshalIndent(lc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, LocalConfigFile), data, 0o644)
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

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	creds, err := LoadCredentials()
	if err != nil {
		return Config{}, err
	}
	cfg.Token = creds.Token

	// Local .ticket.json overrides the project from the global config.
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		if lc, ok := FindLocalConfig(cwd); ok {
			cfg.CurrentProject = lc.CurrentProject
		}
	}

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

// Home returns the config directory. This is where config.json and
// credentials.json live. Override with TICKET_CONFIG_DIR.
func Home() (string, error) {
	if dir := envValue("TICKET_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "ticket"), nil
}
