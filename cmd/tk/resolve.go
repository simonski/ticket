package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osuser "os/user"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

const defaultTicketURL = "https://ticket.localhost"

func resolveCurrentProjectClient() (config.Config, libticket.Service, store.Project, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	projectID := strings.TrimSpace(cfg.ProjectID)
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
	if location == "" && !isTestBinary() {
		location = defaultTicketURL
	}
	if isTestBinary() {
		if location == "" {
			location = strings.TrimSpace(cfg.Location)
		}
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
		if username == "" {
			username = strings.TrimSpace(cfg.Username)
		}
		if password == "" {
			password = strings.TrimSpace(cfg.Token)
		}
		// Keep tests aligned with runtime server-first behavior: if a legacy
		// config stores a local DB path as a remote URL, fall back to the
		// default server URL so credential hints remain consistent.
		if location != "" {
			if resolvedLegacy, resolveErr := config.ResolveLocation(location); resolveErr == nil && strings.TrimSpace(resolvedLegacy.ServerURL) == "" {
				location = defaultTicketURL
			}
		}
	}
	if location == "" {
		if isTestBinary() {
			return libticket.NewLocal(cfg), nil
		}
		return nil, errors.New("missing required environment variable: TICKET_URL")
	}
	if username == "" && (!isTestBinary() || location == defaultTicketURL) {
		return nil, errors.New("missing required environment variable: TICKET_USERNAME")
	}
	if password == "" && (!isTestBinary() || location == defaultTicketURL) {
		return nil, errors.New("missing required environment variable: TICKET_PASSWORD")
	}
	resolved, err := config.ResolveLocation(location)
	if err != nil {
		return nil, err
	}
	effectiveCfg := cfg
	if resolved.ServerURL == "" {
		return nil, errors.New("ticket requires a running server; set TICKET_URL")
	}
	effectiveCfg.Location = resolved.ServerURL
	effectiveCfg.Username = username
	effectiveCfg.Token = password
	return libticket.NewHTTP(effectiveCfg), nil
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
