package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

type Remote struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Config struct {
	Location      string   `json:"location"`
	Token         string   `json:"token"`
	Username      string   `json:"username"`
	UseBasicAuth  bool     `json:"-"`
	Remote        string   `json:"remote,omitempty"`
	DefaultRemote string   `json:"default_remote,omitempty"`
	Remotes       []Remote `json:"remotes,omitempty"`
	ProjectID     string   `json:"project_id"`

	// TUI state — persisted between sessions by default.
	// Set TUIDisablePersist=true to skip save/restore.
	TUIDisablePersist bool     `json:"tui_disable_persist,omitempty"`
	TUITheme          string   `json:"tui_theme,omitempty"`
	TUIMode           string   `json:"tui_mode,omitempty"` // "summary" | "projects" | "ideas" | "list" | "settings"
	TUICursor         int      `json:"tui_cursor,omitempty"`
	TUIExpandedEpics  []string `json:"tui_expanded_epics,omitempty"`
}

type preferencesDiskConfig struct {
	Version           int      `json:"version,omitempty"`
	TUIDisablePersist bool     `json:"tui_disable_persist,omitempty"`
	TUITheme          string   `json:"tui_theme,omitempty"`
	TUIMode           string   `json:"tui_mode,omitempty"`
	TUICursor         int      `json:"tui_cursor,omitempty"`
	TUIExpandedEpics  []string `json:"tui_expanded_epics,omitempty"`
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

var locationOverride string

var gitRootCache sync.Map

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func SetLocationOverride(location string) {
	locationOverride = strings.TrimSpace(location)
}

func ClearLocationOverride() {
	locationOverride = ""
}

func HasLocationOverride() bool {
	return strings.TrimSpace(locationOverride) != ""
}

// ResolveURL determines mode and target from the effective config.
func ResolveURL() (Resolved, error) {
	if HasLocationOverride() {
		return ResolveLocation(locationOverride)
	}
	if envURL := envValue("TICKET_URL"); envURL != "" {
		return ResolveLocation(envURL)
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
	globalCfg, err := loadPreferences()
	if err != nil {
		return Config{}, err
	}

	cfg := globalCfg
	creds, err := LoadCredentials()
	if err != nil {
		return Config{}, err
	}
	credentialLocation := firstNonEmpty(envValue("TICKET_URL"), cfg.Location)
	if envURL := envValue("TICKET_URL"); envURL != "" {
		cfg.Location = envURL
	}
	if resolved, rErr := ResolveLocation(credentialLocation); rErr == nil && resolved.Mode == ModeRemote {
		if remote, ok := creds.Remote(credentialLocation); ok {
			if strings.TrimSpace(cfg.Username) == "" {
				cfg.Username = remote.Username
			}
			cfg.Token = remote.Token
		} else {
			cfg.Token = creds.Token
		}
	}
	if envUsername := envValue("TICKET_USERNAME"); envUsername != "" {
		cfg.Username = envUsername
	}
	if envToken := envValue("TICKET_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	return cfg, nil
}

func Save(cfg Config) error {
	if err := savePreferences(cfg); err != nil {
		return err
	}
	return removeLegacyGlobalConfig()
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
	location, err := CanonicalizeRemoteURL(location)
	if err != nil {
		return RemoteCredentials{}, false
	}
	remote, ok := c.Remotes[location]
	return remote, ok
}

func SaveCredentials(creds Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
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
	location, err = CanonicalizeRemoteURL(location)
	if err != nil {
		return err
	}
	if location == "" {
		creds.Token = token
		return SaveCredentials(creds)
	}
	creds.Remotes[location] = RemoteCredentials{
		Username: strings.TrimSpace(username),
		Token:    strings.TrimSpace(token),
	}
	creds.Token = ""
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
	var err error
	location, err = CanonicalizeRemoteURL(location)
	if err != nil {
		return err
	}
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
	creds.Token = ""
	return SaveCredentials(creds)
}

// Path returns the path to the global preferences file ($TICKET_HOME/preferences.json).
func Path() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "preferences.json"), nil
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
// preferences, and the central local database.
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

func (cfg Config) RemoteByName(name string) (Remote, bool) {
	name = strings.TrimSpace(name)
	for _, remote := range cfg.Remotes {
		if remote.Name == name {
			return remote, true
		}
	}
	return Remote{}, false
}

func (cfg Config) RemoteByURL(rawURL string) (Remote, bool) {
	canonical, err := CanonicalizeRemoteURL(rawURL)
	if err != nil {
		return Remote{}, false
	}
	for _, remote := range cfg.Remotes {
		normalized, err := CanonicalizeRemoteURL(remote.URL)
		if err == nil && normalized == canonical {
			return remote, true
		}
	}
	return Remote{}, false
}

func CanonicalizeRemoteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	resolved, err := ResolveLocation(raw)
	if err != nil {
		return "", err
	}
	switch resolved.Mode {
	case ModeRemote:
		u, err := url.Parse(resolved.ServerURL)
		if err != nil {
			return "", err
		}
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
		if u.Path == "/" {
			u.Path = ""
		}
		return u.String(), nil
	case ModeLocal:
		path := resolved.DBPath
		if !filepath.IsAbs(path) {
			abs, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}
			path = abs
		}
		return "file://" + path, nil
	default:
		return raw, nil
	}
}

func CanonicalizeGitRepository(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if looksLikeSCPGitRepository(raw) {
		host, path, ok := splitSCPGitRepository(raw)
		if !ok {
			return "", fmt.Errorf("invalid git repository %q", raw)
		}
		return normalizeRemoteGitRepository(host, path), nil
	}
	if looksLikeHostPathGitRepository(raw) {
		parts := strings.SplitN(raw, "/", 2)
		if len(parts) == 2 {
			return normalizeRemoteGitRepository(parts[0], parts[1]), nil
		}
	}
	if u, err := url.Parse(raw); err == nil && strings.TrimSpace(u.Scheme) != "" {
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "ssh", "git":
			return normalizeRemoteGitRepository(remoteURLHost(u), u.Path), nil
		case "file":
			return canonicalizeLocalGitRepository(u.Path)
		default:
			return raw, nil
		}
	}
	return canonicalizeLocalGitRepository(raw)
}

func normalizeRemoteGitRepository(host, path string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return host
	}
	return host + "/" + path
}

func canonicalizeLocalGitRepository(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
		path = resolved
	}
	return "file://" + filepath.Clean(path), nil
}

func remoteURLHost(u *url.URL) string {
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if port := strings.TrimSpace(u.Port()); port != "" {
		return host + ":" + port
	}
	return host
}

func looksLikeHostPathGitRepository(raw string) bool {
	if strings.Contains(raw, "://") || filepath.IsAbs(raw) || strings.HasPrefix(raw, ".") {
		return false
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return false
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return false
	}
	host := strings.TrimSpace(parts[0])
	return strings.Contains(host, ".") || strings.EqualFold(host, "localhost")
}

func looksLikeSCPGitRepository(raw string) bool {
	if strings.Contains(raw, "://") || filepath.IsAbs(raw) || strings.HasPrefix(raw, ".") {
		return false
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return false
	}
	return strings.Contains(raw, ":") && strings.Contains(raw, "/")
}

func splitSCPGitRepository(raw string) (host, path string, ok bool) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	host = strings.TrimSpace(parts[0])
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	path = strings.TrimSpace(parts[1])
	if host == "" || path == "" {
		return "", "", false
	}
	return host, path, true
}

func AddRemote(cfg Config, remote Remote) (Config, error) {
	name := strings.TrimSpace(remote.Name)
	if name == "" {
		return cfg, errors.New("remote name is required")
	}
	urlValue, err := CanonicalizeRemoteURL(remote.URL)
	if err != nil {
		return cfg, err
	}
	if urlValue == "" {
		return cfg, errors.New("remote url is required")
	}
	for _, existing := range cfg.Remotes {
		if existing.Name == name {
			return cfg, fmt.Errorf("remote %q already exists", name)
		}
		existingURL, err := CanonicalizeRemoteURL(existing.URL)
		if err == nil && existingURL == urlValue {
			return cfg, fmt.Errorf("remote URL %q already exists", urlValue)
		}
	}
	cfg.Remotes = append(cfg.Remotes, Remote{Name: name, URL: urlValue})
	cfg.Remotes = sortUniqueRemotes(cfg.Remotes)
	return cfg, nil
}

func RemoveRemote(cfg Config, name string) (Config, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return cfg, false
	}
	filtered := make([]Remote, 0, len(cfg.Remotes))
	removed := false
	for _, remote := range cfg.Remotes {
		if remote.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, remote)
	}
	cfg.Remotes = filtered
	if cfg.DefaultRemote == name {
		cfg.DefaultRemote = ""
	}
	return cfg, removed
}

func sortUniqueRemotes(remotes []Remote) []Remote {
	seenNames := map[string]bool{}
	seenURLs := map[string]bool{}
	filtered := make([]Remote, 0, len(remotes))
	for _, remote := range remotes {
		name := strings.TrimSpace(remote.Name)
		urlValue, err := CanonicalizeRemoteURL(remote.URL)
		if name == "" || err != nil || urlValue == "" || seenNames[name] || seenURLs[urlValue] {
			continue
		}
		seenNames[name] = true
		seenURLs[urlValue] = true
		filtered = append(filtered, Remote{Name: name, URL: urlValue})
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	return filtered
}

func samePath(a, b string) bool {
	a = filepath.Clean(strings.TrimSpace(a))
	b = filepath.Clean(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if resolvedA, err := filepath.EvalSymlinks(a); err == nil {
		a = filepath.Clean(resolvedA)
	}
	if resolvedB, err := filepath.EvalSymlinks(b); err == nil {
		b = filepath.Clean(resolvedB)
	}
	return a == b
}

// FindGitRoot walks up the directory tree from startDir looking for a .git
// directory. Returns the parent of .git/ (the project root). Stops at the
// filesystem root.
func FindGitRoot(startDir string) (string, bool) {
	startDir = filepath.Clean(strings.TrimSpace(startDir))
	if startDir == "" {
		return "", false
	}
	if cached, ok := gitRootCache.Load(startDir); ok {
		return cached.(string), true
	}
	dir := startDir
	visited := make([]string, 0, 8)
	for {
		visited = append(visited, dir)
		candidate := filepath.Join(dir, ".git")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			for _, path := range visited {
				gitRootCache.Store(path, dir)
			}
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
	err = json.Unmarshal(data, &raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config at %s is not valid JSON (%v); using defaults\n", path, err)
		return Config{}, nil
	}
	if v, ok := raw["tui_expanded_epics"]; ok {
		s := strings.TrimSpace(string(v))
		if s != "" && s[0] == '[' {
			var items []json.RawMessage
			if json.Unmarshal(v, &items) == nil {
				converted := make([]string, 0, len(items))
				for _, item := range items {
					is := strings.TrimSpace(string(item))
					if is != "" && is[0] == '"' {
						var sv string
						_ = json.Unmarshal(item, &sv)
						converted = append(converted, sv)
					} else {
						converted = append(converted, strings.Trim(is, " "))
					}
				}
				if b, marshalErr := json.Marshal(converted); marshalErr == nil {
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

func loadPreferences() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(path); err == nil {
		cfg, loadErr := loadConfigFile(path)
		if loadErr != nil {
			return Config{}, loadErr
		}
		return preferencesOnly(cfg), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	return migrateLegacyPreferences()
}

func savePreferences(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	prefs := preferencesOnly(cfg)
	if !hasPreferences(prefs) {
		removeErr := os.Remove(path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	}
	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750)
	if mkdirErr != nil {
		return mkdirErr
	}
	disk := preferencesDiskConfig{
		Version:           1,
		TUIDisablePersist: prefs.TUIDisablePersist,
		TUITheme:          strings.TrimSpace(prefs.TUITheme),
		TUIMode:           strings.TrimSpace(prefs.TUIMode),
		TUICursor:         prefs.TUICursor,
		TUIExpandedEpics:  append([]string(nil), prefs.TUIExpandedEpics...),
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func preferencesOnly(cfg Config) Config {
	return Config{
		TUIDisablePersist: cfg.TUIDisablePersist,
		TUITheme:          strings.TrimSpace(cfg.TUITheme),
		TUIMode:           strings.TrimSpace(cfg.TUIMode),
		TUICursor:         cfg.TUICursor,
		TUIExpandedEpics:  append([]string(nil), cfg.TUIExpandedEpics...),
	}
}

func hasPreferences(cfg Config) bool {
	return cfg.TUIDisablePersist ||
		strings.TrimSpace(cfg.TUITheme) != "" ||
		strings.TrimSpace(cfg.TUIMode) != "" ||
		cfg.TUICursor != 0 ||
		len(cfg.TUIExpandedEpics) > 0
}

func migrateLegacyPreferences() (Config, error) {
	legacyPath, err := legacyPath()
	if err != nil {
		return Config{}, err
	}
	_, statErr := os.Stat(legacyPath)
	if errors.Is(statErr, os.ErrNotExist) {
		return Config{}, nil
	} else if statErr != nil {
		return Config{}, statErr
	}
	legacyCfg, err := loadConfigFile(legacyPath)
	if err != nil {
		return Config{}, err
	}
	prefs := preferencesOnly(legacyCfg)
	if err := savePreferences(prefs); err != nil {
		return Config{}, err
	}
	if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	return prefs, nil
}

func removeLegacyGlobalConfig() error {
	path, err := legacyPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func legacyPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// removeInvalidFields tries removing fields from raw one at a time until the
// map can be unmarshalled into Config without error. Returns the cleaned map
// and the names of fields that were removed.
func removeInvalidFields(raw map[string]json.RawMessage) (cleaned map[string]json.RawMessage, removed []string) {
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
		if k != "token" && k != "username" {
			cleaned[k] = v
		}
	}
	data, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
