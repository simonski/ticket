package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osuser "os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

type ticketJSONSettings struct {
	Path      string
	URL       string
	Username  string
	ProjectID string
}

const defaultTicketURL = "https://ticket.localhost"

var localModeWarningOnce sync.Once

func resolveCurrentProjectClient() (config.Config, libticket.Service, store.Project, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	projectID := strings.TrimSpace(os.Getenv("TICKET_PROJECT"))
	if projectID == "" {
		if fileSettings, settingsErr := loadNearestTicketJSONSettings(); settingsErr != nil {
			return config.Config{}, nil, store.Project{}, settingsErr
		} else if strings.TrimSpace(fileSettings.ProjectID) != "" {
			projectID = strings.TrimSpace(fileSettings.ProjectID)
		}
	}
	if projectID != "" {
		cfg.ProjectID = projectID
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	projectID = strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		projectID = "1"
	}

	project, err := svc.GetProject(context.Background(), projectID)
	if err != nil && projectID != "1" {
		project, err = svc.GetProject(context.Background(), "1")
		if err == nil {
			projectID = "1"
		}
	}
	if err != nil {
		if strings.TrimSpace(cfg.ProjectID) == "" {
			return config.Config{}, nil, store.Project{}, errors.New("no active project; use `ticket project create` or `ticket project use <id>` first")
		}
		return config.Config{}, nil, store.Project{}, err
	}

	if cfg.ProjectID != projectID {
		cfg.ProjectID = projectID
		if !config.HasLocationOverride() && strings.TrimSpace(os.Getenv("TICKET_URL")) == "" {
			if saveErr := config.Save(cfg); saveErr != nil {
				return config.Config{}, nil, store.Project{}, saveErr
			}
		}
	}
	return cfg, svc, project, nil
}

func resolveService(cfg config.Config) (libticket.Service, error) {
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	projectID := strings.TrimSpace(os.Getenv("TICKET_PROJECT"))

	fileSettings, err := loadNearestTicketJSONSettings()
	if err != nil {
		return nil, err
	}
	if location == "" {
		location = strings.TrimSpace(fileSettings.URL)
	}
	if username == "" {
		username = strings.TrimSpace(fileSettings.Username)
	}
	if projectID == "" {
		projectID = strings.TrimSpace(fileSettings.ProjectID)
	}

	remoteHintProvided := location != "" || username != "" || password != ""
	if remoteHintProvided {
		if location == "" {
			return nil, errors.New("missing required environment variable: TICKET_URL")
		}
		if username == "" {
			return nil, errors.New("missing required environment variable: TICKET_USERNAME")
		}
		if password == "" {
			return nil, errors.New("missing required environment variable: TICKET_PASSWORD")
		}
		resolved, resolveErr := config.ResolveLocation(location)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if strings.TrimSpace(resolved.ServerURL) == "" {
			return nil, errors.New("ticket requires a running server; set TICKET_URL")
		}
		effectiveCfg := cfg
		effectiveCfg.Location = resolved.ServerURL
		effectiveCfg.Username = username
		effectiveCfg.Token = password
		if projectID != "" {
			effectiveCfg.ProjectID = projectID
		}
		return libticket.NewHTTP(effectiveCfg), nil
	}

	// Compatibility fallback for explicit remote config.
	location = strings.TrimSpace(cfg.Location)
	if location == "" {
		if remote, ok := cfg.RemoteByName(strings.TrimSpace(cfg.Remote)); ok {
			location = strings.TrimSpace(remote.URL)
		}
	}
	if location == "" {
		if remote, ok := cfg.RemoteByName(strings.TrimSpace(cfg.DefaultRemote)); ok {
			location = strings.TrimSpace(remote.URL)
		}
	}
	if location != "" {
		resolvedCfgLocation, resolveErr := config.ResolveLocation(location)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if resolvedCfgLocation.Mode == config.ModeRemote {
			effectiveCfg := cfg
			effectiveCfg.Location = resolvedCfgLocation.ServerURL
			if projectID != "" {
				effectiveCfg.ProjectID = projectID
			}
			return libticket.NewHTTP(effectiveCfg), nil
		}
		if strings.TrimSpace(resolvedCfgLocation.DBPath) != "" {
			localCfg := cfg
			localCfg.Location = resolvedCfgLocation.DBPath
			warnLocalMode(resolvedCfgLocation.DBPath)
			return libticket.NewLocal(localCfg), nil
		}
	}

	localPath, err := config.LocalDBPath()
	if err != nil {
		return nil, err
	}
	if config.HasLocationOverride() {
		if resolvedOverride, resolveErr := config.ResolveURL(); resolveErr == nil && resolvedOverride.Mode == config.ModeLocal && strings.TrimSpace(resolvedOverride.DBPath) != "" {
			localPath = strings.TrimSpace(resolvedOverride.DBPath)
		}
	}
	if _, statErr := os.Stat(localPath); statErr == nil {
		localCfg := cfg
		localCfg.Location = localPath
		if projectID != "" {
			localCfg.ProjectID = projectID
		}
		warnLocalMode(localPath)
		return libticket.NewLocal(localCfg), nil
	}

	localCfg := cfg
	localCfg.Location = localPath
	if projectID != "" {
		localCfg.ProjectID = projectID
	}
	return libticket.NewLocal(localCfg), nil
}

func hasCompleteRemoteRuntimeConfig() (bool, error) {
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	if location != "" && username != "" && password != "" {
		return true, nil
	}
	fileSettings, err := loadNearestTicketJSONSettings()
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(location) == "" {
		location = strings.TrimSpace(fileSettings.URL)
	}
	if strings.TrimSpace(username) == "" {
		username = strings.TrimSpace(fileSettings.Username)
	}
	return strings.TrimSpace(location) != "" && strings.TrimSpace(username) != "" && strings.TrimSpace(password) != "", nil
}

func warnLocalMode(dbPath string) {
	localModeWarningOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "Warning: you are in local-mode under %s\n", dbPath)
	})
}

func loadNearestTicketJSONSettings() (ticketJSONSettings, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return ticketJSONSettings{}, err
	}
	path, ok, err := findTicketJSONPath(cwd)
	if err != nil {
		return ticketJSONSettings{}, err
	}
	if !ok {
		return ticketJSONSettings{}, nil
	}
	return parseTicketJSON(path)
}

func findTicketJSONPath(startDir string) (path string, ok bool, err error) {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		return "", false, nil
	}
	stopDir := ""
	if gitRoot, ok := config.FindGitRoot(startDir); ok {
		stopDir = gitRoot
	}
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".ticket.json")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		if stopDir != "" && filepath.Clean(stopDir) == filepath.Clean(dir) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false, nil
}

func parseTicketJSON(path string) (ticketJSONSettings, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is discovered from local ancestor directories.
	if err != nil {
		return ticketJSONSettings{}, err
	}
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ticketJSONSettings{}, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	for _, key := range []string{"TICKET_PASSWORD", "ticket_password", "password", "token"} {
		if value, ok := raw[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return ticketJSONSettings{}, fmt.Errorf("%s must not contain TICKET_PASSWORD; use environment variable TICKET_PASSWORD instead", path)
		}
	}
	settings := ticketJSONSettings{
		Path:      path,
		URL:       firstNonEmpty(raw, "TICKET_URL", "ticket_url", "url"),
		Username:  firstNonEmpty(raw, "TICKET_USERNAME", "ticket_username", "username"),
		ProjectID: firstNonEmpty(raw, "TICKET_PROJECT", "ticket_project", "project", "project_id"),
	}
	return settings, nil
}

func firstNonEmpty(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(fmt.Sprint(value))
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
}

func resolveCredentials(usernameFlag, passwordFlag string, useEnv bool) (username, password string, err error) {
	username = strings.TrimSpace(usernameFlag)
	password = strings.TrimSpace(passwordFlag)
	_ = useEnv
	if username == "" {
		username = currentOSUser()
	}
	if password == "" {
		password = "password"
	}
	if username == "" || password == "" {
		return "", "", errors.New("username and password are required")
	}
	return username, password, nil
}

func currentOSUser() string {
	user, err := osuser.Current()
	if err == nil && user.Username != "" {
		parts := strings.Split(user.Username, `\`)
		return parts[len(parts)-1]
	}
	if env := os.Getenv("USER"); env != "" {
		return env
	}
	if env := os.Getenv("USERNAME"); env != "" {
		return env
	}
	return "user"
}

func fallbackCommandUsername() string {
	return currentOSUser()
}

func extractDBOverride(args []string) (out []string, override string, err error) {
	if len(args) == 0 {
		return args, "", nil
	}
	if len(args) > 0 {
		switch args[0] {
		case "add", "create", "new", "update":
			// Ticket commands support -f for batch file input.
			return args, "", nil
		}
	}
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" {
			if i+1 >= len(args) {
				return nil, "", errors.New("missing value for -f")
			}
			override = args[i+1]
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out, override, nil
}

// extractGUIFlag removes -g/--gui and optional -theme=<id> from args.
// Returns trimmed args and the theme ID (empty string = use default).
func extractGUIFlag(args []string) (out []string, theme string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-g" || arg == "--gui" || arg == "gui":
			theme = "gui-requested"
		case strings.HasPrefix(arg, "-theme="):
			theme = strings.TrimPrefix(arg, "-theme=")
		case arg == "-theme" && i+1 < len(args):
			i++
			theme = args[i]
		default:
			out = append(out, arg)
		}
	}
	return out, theme
}

func extractOutputFlags(args []string) (out []string, jsonFlag, nocolor bool, err error) {
	for _, arg := range args {
		switch arg {
		case "-json":
			jsonFlag = true
		case "-nocolor":
			nocolor = true
		default:
			out = append(out, arg)
		}
	}
	return out, jsonFlag, nocolor, nil
}

// jsonKeysToOmit lists internal keys that are stripped from all
// CLI JSON output. Keep this list intentionally minimal so human-facing
// identifiers (for example, ticket_id) remain stable for scripts.
var jsonKeysToOmit = map[string]bool{}

// stripJSONKeys recursively removes unwanted keys from a decoded JSON value.
func stripJSONKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k := range val {
			if jsonKeysToOmit[k] {
				delete(val, k)
			} else {
				val[k] = stripJSONKeys(val[k])
			}
		}
	case []any:
		for i, item := range val {
			val[i] = stripJSONKeys(item)
		}
	}
	return v
}

func printJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var decoded any
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		return err
	}
	stripped, err := json.MarshalIndent(stripJSONKeys(decoded), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(stripped))
	return nil
}

func generatePassword(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if length <= 0 {
		return "", errors.New("password length must be positive")
	}
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i, b := range random {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

func generateConfirmToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func removeDBFiles(path string) error {
	for _, suffix := range []string{"", "-shm", "-wal"} {
		candidate := path + suffix
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func defaultDatabasePath() (string, error) {
	return config.LocalDBPath()
}
