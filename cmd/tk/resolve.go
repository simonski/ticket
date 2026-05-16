package main

import (
	"context"
	"crypto/rand"
	"database/sql"
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

const defaultTicketURL = "https://ticket.localhost"

var localModeWarningOnce sync.Once
var runtimeProjectOverride string

func setProjectOverride(projectRef string) {
	runtimeProjectOverride = strings.TrimSpace(projectRef)
}

func clearProjectOverride() {
	runtimeProjectOverride = ""
}

func currentProjectOverride() string {
	return strings.TrimSpace(runtimeProjectOverride)
}

func resolveCurrentProjectClient() (config.Config, libticket.Service, store.Project, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	projectID := resolveConfiguredProjectReference(cfg)
	if projectID != "" {
		cfg.ProjectID = projectID
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	projectID = strings.TrimSpace(cfg.ProjectID)
	project, projectRef, err := resolveProjectContext(context.Background(), cfg, svc, projectID)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	if cfg.ProjectID != projectRef {
		cfg.ProjectID = projectRef
		if !config.HasLocationOverride() && strings.TrimSpace(os.Getenv("TICKET_URL")) == "" {
			if saveErr := config.Save(cfg); saveErr != nil {
				return config.Config{}, nil, store.Project{}, saveErr
			}
		}
	}
	return cfg, svc, project, nil
}

func resolveProjectContext(ctx context.Context, cfg config.Config, svc libticket.Service, configuredRef string) (store.Project, string, error) {
	if ref := strings.TrimSpace(configuredRef); ref != "" {
		project, err := svc.GetProject(ctx, ref)
		if err == nil {
			return project, ref, nil
		}
		if localProject, ok, localErr := resolveLocalAliasProject(ctx, cfg, ref); localErr == nil && ok {
			return localProject, ref, nil
		}
		return project, ref, err
	}
	if repo := nearestGitRemoteFromCLI(); repo != "" {
		projects, err := svc.ListProjects(ctx)
		if err == nil {
			for _, project := range projects {
				if strings.TrimSpace(project.GitRepository) == repo {
					return project, project.Prefix, nil
				}
			}
		}
	}
	if project, err := svc.GetProject(ctx, "private"); err == nil {
		return project, project.Prefix, nil
	}
	if project, ok, err := resolveLocalAliasProject(ctx, cfg, "private"); err == nil && ok {
		return project, project.Prefix, nil
	}
	project, err := mostRecentProject(svc)
	if err != nil {
		return store.Project{}, "", err
	}
	return project, project.Prefix, nil
}

func resolveProjectFromFlagOrConfig(ctx context.Context, cfg config.Config, svc libticket.Service, explicitRef string) (store.Project, error) {
	project, _, err := resolveProjectContext(ctx, cfg, svc, firstNonEmpty(strings.TrimSpace(explicitRef), resolveConfiguredProjectReference(cfg)))
	return project, err
}

func resolveLocalAliasProject(ctx context.Context, cfg config.Config, ref string) (store.Project, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref != "public" && ref != "private" {
		return store.Project{}, false, nil
	}
	dbPath, ok, err := localDBPathForConfig(cfg)
	if err != nil || !ok {
		return store.Project{}, false, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return store.Project{}, false, err
	}
	defer db.Close()
	if ref == "public" {
		project, projectErr := store.GetProjectByAlias(ctx, db, "public", "")
		if projectErr != nil {
			return store.Project{}, false, projectErr
		}
		return project, true, nil
	}
	username := firstNonEmpty(strings.TrimSpace(os.Getenv("TICKET_USERNAME")), strings.TrimSpace(cfg.Username), "admin")
	user, err := store.GetUserByUsername(ctx, db, username)
	if err != nil {
		return store.Project{}, false, err
	}
	project, err := store.GetProjectByAlias(ctx, db, "private", user.ID)
	if err != nil {
		return store.Project{}, false, err
	}
	return project, true, nil
}

func localDBPathForConfig(cfg config.Config) (dbPath string, ok bool, err error) {
	location := strings.TrimSpace(cfg.Location)
	if location != "" {
		resolved, resolveErr := config.ResolveLocation(location)
		if resolveErr != nil {
			return "", false, resolveErr
		}
		if resolved.Mode != config.ModeRemote && strings.TrimSpace(resolved.DBPath) != "" {
			return strings.TrimSpace(resolved.DBPath), true, nil
		}
	}
	localPath, localPathErr := config.LocalDBPath()
	if localPathErr != nil {
		return "", false, localPathErr
	}
	if config.HasLocationOverride() {
		if resolved, resolveErr := config.ResolveURL(); resolveErr == nil {
			if resolved.Mode == config.ModeRemote {
				return "", false, nil
			}
			if strings.TrimSpace(resolved.DBPath) != "" {
				localPath = strings.TrimSpace(resolved.DBPath)
			}
		}
	}
	if _, statErr := os.Stat(localPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, statErr
	}
	return localPath, true, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nearestGitRemoteFromCLI() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	dir := filepath.Clean(cwd)
	for {
		if remote := detectGitOriginAt(dir); remote != "" {
			return remote
		}
		if homeDir != "" && filepath.Clean(homeDir) == dir {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func resolveService(cfg config.Config) (libticket.Service, error) {
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	token := strings.TrimSpace(os.Getenv("TICKET_TOKEN"))
	projectID := firstNonEmpty(currentProjectOverride(), strings.TrimSpace(os.Getenv("TICKET_PROJECT")), strings.TrimSpace(cfg.ProjectID))

	remoteHintProvided := location != "" || password != "" || token != ""
	if remoteHintProvided {
		if location == "" {
			location = configuredServiceLocation(cfg)
		}
		if location == "" {
			return nil, errors.New("missing required environment variable: TICKET_URL")
		}
		if token == "" && password != "" && username == "" {
			return nil, errors.New("missing required environment variable: TICKET_USERNAME")
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
		if !sameRemoteLocation(cfg.Location, resolved.ServerURL) {
			effectiveCfg.Username = ""
			effectiveCfg.Token = ""
		}
		switch {
		case token != "":
			effectiveCfg.Username = ""
			effectiveCfg.Token = token
		case password != "":
			effectiveCfg.Username = username
			effectiveCfg.Token = password
			effectiveCfg.UseBasicAuth = true
		default:
			creds, credsErr := config.LoadCredentials()
			if credsErr != nil {
				return nil, credsErr
			}
			if remoteCreds, ok := creds.Remote(resolved.ServerURL); ok && strings.TrimSpace(remoteCreds.Token) != "" {
				effectiveCfg.Username = ""
				effectiveCfg.Token = remoteCreds.Token
			} else if strings.TrimSpace(effectiveCfg.Token) != "" {
				effectiveCfg.Username = ""
			}
			if username != "" && strings.TrimSpace(effectiveCfg.Token) == "" {
				return nil, errors.New("missing required environment variable: TICKET_PASSWORD")
			}
		}
		if !effectiveCfg.UseBasicAuth && strings.TrimSpace(effectiveCfg.Token) == "" {
			return nil, missingRemoteAuthError(resolved.ServerURL)
		}
		if projectID != "" {
			effectiveCfg.ProjectID = projectID
		}
		return libticket.NewHTTP(effectiveCfg), nil
	}

	// Compatibility fallback for explicit remote config.
	location = configuredServiceLocation(cfg)
	if location != "" {
		resolvedCfgLocation, resolveErr := config.ResolveLocation(location)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if resolvedCfgLocation.Mode == config.ModeRemote {
			effectiveCfg := cfg
			effectiveCfg.Location = resolvedCfgLocation.ServerURL
			if strings.TrimSpace(os.Getenv("TICKET_USERNAME")) != "" && strings.TrimSpace(os.Getenv("TICKET_PASSWORD")) != "" {
				effectiveCfg.Username = strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
				effectiveCfg.Token = strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
				effectiveCfg.UseBasicAuth = true
			} else if strings.TrimSpace(os.Getenv("TICKET_TOKEN")) != "" {
				effectiveCfg.Username = ""
				effectiveCfg.Token = strings.TrimSpace(os.Getenv("TICKET_TOKEN"))
			}
			if !effectiveCfg.UseBasicAuth && strings.TrimSpace(effectiveCfg.Token) != "" {
				effectiveCfg.Username = ""
			}
			if !effectiveCfg.UseBasicAuth && strings.TrimSpace(effectiveCfg.Token) == "" {
				return nil, missingRemoteAuthError(resolvedCfgLocation.ServerURL)
			}
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

func configuredServiceLocation(cfg config.Config) string {
	location := strings.TrimSpace(cfg.Location)
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
	return location
}

func missingRemoteAuthError(serverURL string) error {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return errors.New("not logged in.\nRun `tk login`, or set TICKET_URL plus TICKET_USERNAME/TICKET_PASSWORD, or set TICKET_TOKEN.")
	}
	return fmt.Errorf("not logged in to %s.\nRun `tk login`, or set TICKET_URL plus TICKET_USERNAME/TICKET_PASSWORD, or set TICKET_TOKEN.", serverURL)
}

func resolveConfiguredProjectReference(cfg config.Config) string {
	return firstNonEmpty(currentProjectOverride(), strings.TrimSpace(os.Getenv("TICKET_PROJECT")), strings.TrimSpace(cfg.ProjectID))
}

func sameRemoteLocation(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	leftCanonical, leftErr := config.CanonicalizeRemoteURL(left)
	rightCanonical, rightErr := config.CanonicalizeRemoteURL(right)
	if leftErr != nil || rightErr != nil {
		return left == right
	}
	return leftCanonical == rightCanonical
}

func hasCompleteRemoteRuntimeConfig() (bool, error) {
	location := strings.TrimSpace(os.Getenv("TICKET_URL"))
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	token := strings.TrimSpace(os.Getenv("TICKET_TOKEN"))
	if location != "" && token != "" {
		return true, nil
	}
	if location != "" && username != "" && password != "" {
		return true, nil
	}
	return strings.TrimSpace(location) != "" && strings.TrimSpace(username) != "" && strings.TrimSpace(password) != "", nil
}

func warnLocalMode(dbPath string) {
	localModeWarningOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "Warning: you are in local-mode under %s\n", dbPath)
	})
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

func extractProjectOverride(args []string) (out []string, override string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-project_id" || arg == "-project":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("missing value for %s", arg)
			}
			override = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "-project_id="):
			override = strings.TrimSpace(strings.TrimPrefix(arg, "-project_id="))
		case strings.HasPrefix(arg, "-project="):
			override = strings.TrimSpace(strings.TrimPrefix(arg, "-project="))
		default:
			out = append(out, arg)
		}
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
