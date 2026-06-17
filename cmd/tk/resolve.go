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

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

const defaultTicketURL = "https://ticket.simonski.com"

// init wires the default remote URL into the config package so that credential
// resolution (config.Load) and connection resolution agree on the target
// server when nothing is explicitly configured. Test binaries leave it unset
// so they continue to resolve to a local store.
func init() {
	if !isTestBinary() {
		config.DefaultRemoteURL = defaultTicketURL
	}
}

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
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	explicitRef := resolveConfiguredProjectReference(cfg)
	if explicitRef != "" {
		cfg.ProjectID = explicitRef
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	project, projectRef, err := resolveProjectContext(context.Background(), cfg, svc, explicitRef)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	cfg.ProjectID = projectRef
	return cfg, svc, project, nil
}

// resolveProjectContext determines which project a CLI command targets.
//
// Resolution order:
//  1. Explicit ref from -project_id flag or TICKET_PROJECT env.
//  2. Nearest git remote (walking up to home dir) matched against registered repositories.
//  3. The caller's configured default project.
func resolveProjectContext(ctx context.Context, cfg config.Config, svc libticket.Service, configuredRef string) (store.Project, string, error) {
	if ref := strings.TrimSpace(configuredRef); ref != "" {
		project, err := svc.GetProject(ctx, ref)
		if err == nil {
			return project, ref, nil
		}
		return project, ref, err
	}
	if repo := nearestGitRemoteFromCLI(); repo != "" {
		project, err := svc.FindProjectByGitRepository(ctx, repo)
		switch {
		case err == nil:
			return project, project.Prefix, nil
		case !errors.Is(err, store.ErrProjectNotFound):
			return store.Project{}, "", err
		}
	}
	project, err := svc.GetMyDefaultProject(ctx)
	if err != nil {
		return store.Project{}, "", err
	}
	return project, project.Prefix, nil
}

func resolveProjectFromFlagOrConfig(ctx context.Context, cfg config.Config, svc libticket.Service, explicitRef string) (store.Project, error) {
	project, _, err := resolveProjectContext(ctx, cfg, svc, firstNonEmpty(strings.TrimSpace(explicitRef), resolveConfiguredProjectReference(cfg)))
	return project, err
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
	username := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	password := strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
	token := strings.TrimSpace(os.Getenv("TICKET_TOKEN"))
	projectID := firstNonEmpty(currentProjectOverride(), strings.TrimSpace(os.Getenv("TICKET_PROJECT")))
	effectiveCfg := cfg

	resolved, hasExplicitLocation, err := resolveServiceLocation(cfg)
	if err != nil {
		return nil, err
	}
	if hasExplicitLocation {
		switch resolved.Mode {
		case config.ModeLocal:
			effectiveCfg.Location = resolved.DBPath
			if projectID != "" {
				effectiveCfg.ProjectID = projectID
			}
			return libticket.NewLocal(effectiveCfg), nil
		case config.ModeRemote:
			effectiveCfg.Location = resolved.ServerURL
		}
	} else {
		effectiveCfg.Location = configuredServiceLocation(cfg)
	}
	location := strings.TrimSpace(effectiveCfg.Location)
	if location == "" {
		return nil, missingRemoteAuthError("")
	}
	if token == "" && password != "" && username == "" {
		return nil, missingRemoteAuthError(location)
	}
	resolved, resolveErr := config.ResolveLocation(location)
	if resolveErr != nil {
		return nil, resolveErr
	}
	if strings.TrimSpace(resolved.ServerURL) == "" {
		return nil, missingRemoteAuthError("")
	}
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
		if strings.TrimSpace(effectiveCfg.Token) != "" {
			effectiveCfg.Username = ""
		}
		if username != "" && strings.TrimSpace(effectiveCfg.Token) == "" {
			return nil, missingRemoteAuthError(resolved.ServerURL)
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

func resolveServiceLocation(cfg config.Config) (config.Resolved, bool, error) {
	if config.HasLocationOverride() {
		resolved, err := config.ResolveURL()
		return resolved, true, err
	}
	if envURL := strings.TrimSpace(os.Getenv("TICKET_URL")); envURL != "" {
		resolved, err := config.ResolveLocation(envURL)
		return resolved, true, err
	}
	if location := strings.TrimSpace(cfg.Location); location != "" {
		resolved, err := config.ResolveLocation(location)
		return resolved, true, err
	}
	return config.Resolved{}, false, nil
}

func configuredServiceLocation(cfg config.Config) string {
	if isTestBinary() {
		return strings.TrimSpace(cfg.Location)
	}
	return defaultTicketURL
}

func missingRemoteAuthError(serverURL string) error {
	serverURL = strings.TrimSpace(serverURL)
	if resolvedURL, _, err := currentConfiguredRemoteServer(); err == nil && strings.TrimSpace(resolvedURL) != "" {
		serverURL = strings.TrimSpace(resolvedURL)
	}
	lines := []string{
		"incomplete remote authentication configuration.",
		fmt.Sprintf("attempting to connect to TICKET_URL %s", remoteAuthDisplayValue(serverURL, false)),
		fmt.Sprintf("TICKET_USERNAME = %s", remoteAuthDisplayValue(firstNonEmpty(strings.TrimSpace(os.Getenv("TICKET_USERNAME")), ""), false)),
		fmt.Sprintf("TICKET_PASSWORD = %s", remoteAuthDisplayValue(strings.TrimSpace(os.Getenv("TICKET_PASSWORD")), true)),
		"Run `tk login`, set TICKET_TOKEN, or set both TICKET_USERNAME and TICKET_PASSWORD.",
	}
	return errors.New(strings.Join(lines, "\n"))
}

func remoteAuthDisplayValue(value string, secret bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		if noColorOutput {
			return "UNSET"
		}
		return "\x1b[31mUNSET\x1b[0m"
	}
	if secret {
		return "********"
	}
	return value
}

func resolveConfiguredProjectReference(cfg config.Config) string {
	return firstNonEmpty(currentProjectOverride(), strings.TrimSpace(os.Getenv("TICKET_PROJECT")))
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

func resolveCredentials(usernameFlag, passwordFlag string, useEnv bool) (username, password string, err error) {
	username = strings.TrimSpace(usernameFlag)
	password = strings.TrimSpace(passwordFlag)
	if useEnv {
		if username == "" {
			username = strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
		}
		if password == "" {
			password = strings.TrimSpace(os.Getenv("TICKET_PASSWORD"))
		}
	}
	if username == "" {
		username = currentOSUser()
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
		case "add", "create", "new", "update", "import":
			// Ticket commands support -f for batch file input.
			return args, "", nil
		case "ticket":
			if len(args) > 1 {
				switch args[1] {
				case "add", "create", "new", "update", "import":
					// Ticket namespace subcommands support -f for file input.
					return args, "", nil
				}
			}
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
