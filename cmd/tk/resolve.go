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
		if !config.HasLocationOverride() {
			if saveErr := config.Save(cfg); saveErr != nil {
				return config.Config{}, nil, store.Project{}, saveErr
			}
		}
	}
	return cfg, svc, project, nil
}

func resolveService(cfg config.Config) (libticket.Service, error) {
	resolved, err := config.ResolveURL()
	if err != nil {
		return nil, err
	}
	effectiveCfg := cfg
	switch resolved.Mode {
	case config.ModeLocal:
		return libticket.NewLocal(effectiveCfg), nil
	case config.ModeRemote:
		if resolved.ServerURL == "" {
			return nil, errors.New("remote mode requires a location (run tk init to configure)")
		}
		return libticket.NewHTTP(effectiveCfg), nil
	default:
		return nil, fmt.Errorf("unsupported mode %q", resolved.Mode)
	}
}

func resolveCredentials(usernameFlag, passwordFlag string, useEnv bool) (string, string, error) {
	username := strings.TrimSpace(usernameFlag)
	password := strings.TrimSpace(passwordFlag)
	resolved, err := config.ResolveURL()
	if err == nil && resolved.Mode == config.ModeLocal {
		if username == "" {
			username = localModeUsername()
		}
		return username, password, nil
	}

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

func localModeUsername() string {
	return "admin"
}

func fallbackCommandUsername() string {
	if resolved, err := config.ResolveURL(); err == nil && resolved.Mode == config.ModeLocal {
		return localModeUsername()
	}
	return currentOSUser()
}

func extractDBOverride(args []string) ([]string, string, error) {
	if len(args) == 0 {
		return args, "", nil
	}
	var out []string
	var override string
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
func extractGUIFlag(args []string) ([]string, string) {
	var out []string
	var theme string
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

func extractOutputFlags(args []string) ([]string, bool, bool, error) {
	var out []string
	var jsonFlag bool
	var nocolor bool
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

// jsonKeysToOmit lists internal numeric ID keys that are stripped from all
// CLI JSON output. The fields remain in the structs for internal use.
var jsonKeysToOmit = map[string]bool{
	"ticket_id": true,
}

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
	if err := json.Unmarshal(data, &decoded); err != nil {
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
