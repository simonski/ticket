# Ticket Codebase Linear Walkthrough

*2026-03-04T19:12:09Z by Showboat 0.6.1*
<!-- showboat-id: 4d7466c9-3f29-4108-a2a3-ddf8b3f1b980 -->

This walkthrough follows execution order: CLI entrypoint and routing, configuration defaults, service resolution, storage/schema lifecycle, HTTP API handlers, and test/quality gates.

## 1) CLI entrypoint and command routing

`cmd/ticket/main.go` drives execution. `run(args)` resolves mode and global overrides, then dispatches subcommands in a central switch.

```bash
sed -n '300,460p' cmd/ticket/main.go
```

```output
		usage:   "ticket remove-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Removes one or more `depends_on` links from the task to the listed task IDs.", "Comma-separated dependency IDs are supported."},
		example: "ticket remove-dependency 4 2",
	},
	"dependency": {
		usage:   "ticket dependency <add|remove> <id> <dependency-id[,dependency-id...]>",
		details: []string{"Manages `depends_on` links for a task.", "`add` creates dependency links; `remove` deletes them."},
		example: "ticket dependency add 4 1,2,3",
	},
	"request": {
		usage:   "ticket request [--dryrun] [<id>]",
		details: []string{"Requests work for the current user.", "With an id, the server attempts to assign that specific ticket. Without an id, it resumes the user's oldest assigned `develop/active` ticket, then assigned `develop/idle` work, then assigns the oldest unassigned `develop/idle` ticket in the active project."},
		example: "ticket request 42",
	},
	"request-dryrun": {
		usage:   "ticket request-dryrun [<id>]",
		details: []string{"Simulates a request assignment without mutating state and shows what task would be assigned."},
		example: "ticket request-dryrun 42",
	},
	"user": {
		usage:   "ticket user <create|ls|list|rm|delete|enable|disable>",
		details: []string{"Admin-only user management commands.", "If a non-admin user calls these commands, the server returns 403 with `user is not an admin`."},
		example: "ticket user create --username alice --password secret",
	},
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if strings.HasPrefix(err.Error(), "no such command") {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	mode, err := config.ResolveMode()
	if err != nil {
		return err
	}
	trimmedArgs, urlOverride, err := extractURLOverride(args)
	if err != nil {
		return err
	}
	trimmedArgs, dbOverride, err := extractDBOverride(trimmedArgs)
	if err != nil {
		return err
	}
	trimmedArgs, outputJSON, noColorOutput, err = extractOutputFlags(trimmedArgs)
	if err != nil {
		return err
	}
	if urlOverride != "" {
		if err := os.Setenv("TICKET_SERVER", urlOverride); err != nil {
			return err
		}
		if err := os.Setenv("TICKET_URL", urlOverride); err != nil {
			return err
		}
	}
	if dbOverride != "" {
		if err := os.Setenv("TICKET_DB_OVERRIDE", dbOverride); err != nil {
			return err
		}
	}
	if len(trimmedArgs) == 0 {
		fmt.Print(renderRootUsage())
		return nil
	}

	switch trimmedArgs[0] {
	case "help", "-h", "--help":
		return runHelp(trimmedArgs[1:])
	case "onboard":
		return runOnboard(trimmedArgs[1:])
	case "init":
		return errors.New("use `ticket initdb`")
	case "initdb":
		return runInitDB(trimmedArgs[1:])
	case "server":
		return runServer(trimmedArgs[1:])
	case "version":
		return runVersion(trimmedArgs[1:])
	case "register":
		if mode != config.ModeRemote {
			return errors.New("ticket register requires TICKET_MODE=remote")
		}
		return runRegister(trimmedArgs[1:])
	case "login":
		if mode != config.ModeRemote {
			return errors.New("ticket login requires TICKET_MODE=remote")
		}
		return runLogin(trimmedArgs[1:])
	case "logout":
		if mode != config.ModeRemote {
			return errors.New("ticket logout requires TICKET_MODE=remote")
		}
		return runLogout(trimmedArgs[1:])
	case "status":
		return runStatus(trimmedArgs[1:])
	case "count":
		return runCount(trimmedArgs[1:])
	case "ticket":
		return runTicket(trimmedArgs[1:])
	case "user":
		return runUser(trimmedArgs[1:])
	case "project":
		return runProject(trimmedArgs[1:])
	case "ls":
		return runList(trimmedArgs[1:])
	case "list":
		return runList(trimmedArgs[1:])
	case "orphans":
		return runOrphans(trimmedArgs[1:])
	case "get", "show":
		return runGet(trimmedArgs[1:])
	case "search":
		return runSearch(trimmedArgs[1:])
	case "update":
		return runUpdate(trimmedArgs[1:])
	case "set-parent", "attach":
		return runSetParent(trimmedArgs[1:])
	case "unset-parent", "detach":
		return runUnsetParent(trimmedArgs[1:])
	case "design":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDesign, "design")
	case "develop":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDevelop, "develop")
	case "test":
		return runTicketStageAlias(trimmedArgs[1:], store.StageTest, "test")
	case "done":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDone, "done")
	case "idle":
		return runTicketStateAlias(trimmedArgs[1:], store.StateIdle, "idle")
	case "active":
		return runTicketStateAlias(trimmedArgs[1:], store.StateActive, "active")
	case "complete":
		return runTicketStateAlias(trimmedArgs[1:], store.StateComplete, "complete")
	case "assign":
		return runAssign(trimmedArgs[1:])
	case "unassign":
		return runUnassign(trimmedArgs[1:])
	case "claim":
		return runClaim(trimmedArgs[1:])
	case "unclaim":
		return runUnclaim(trimmedArgs[1:])
	case "add-dependency":
		return runDependencyCommand(trimmedArgs[1:], true)
	case "remove-dependency":
		return runDependencyCommand(trimmedArgs[1:], false)
	case "dependency":
		return runDependency(trimmedArgs[1:])
	case "request":
		return runRequest(trimmedArgs[1:])
	case "request-dryrun":
		return runRequestDryRun(trimmedArgs[1:])
	case "history":
		return runHistory(trimmedArgs[1:])
	case "comment":
```

## 2) Configuration defaults and local paths

`internal/config/config.go` resolves mode, server URL, and local state paths. Current defaults place client config and DB under `$TICKET_HOME` (default `~/.config/ticket`).

```bash
sed -n '120,220p' internal/config/config.go
```

```output
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ResolveServerURL(cfg Config) string {
	if env := envValue("TICKET_SERVER"); env != "" {
		return env
	}
	if env := envValue("TICKET_URL"); env != "" {
		return env
	}
	if cfg.ServerURL != "" {
		return cfg.ServerURL
	}
	return defaultServerURL
}

func ResolveMode() (string, error) {
	mode := strings.ToLower(envValue("TICKET_MODE"))
	if mode == "" {
		return ModeLocal, nil
	}
	switch mode {
	case ModeLocal, ModeRemote:
		return mode, nil
	default:
		return "", errors.New("TICKET_MODE must be local or remote")
	}
}

func ResolveDatabasePath() (string, error) {
	if override := envValue("TICKET_DB_OVERRIDE"); override != "" {
		return override, nil
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "ticket.db"), nil
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

func Home() (string, error) {
	if dir := envValue("TICKET_HOME"); dir != "" {
		return dir, nil
	}
	if dir := envValue("TICKET_CONFIG_DIR"); dir != "" {
		return dir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "ticket"), nil
}
```

## 3) Local vs remote service selection

The CLI chooses between in-process local service (`libticket.NewLocal`) and HTTP-backed remote service (`libtickethttp.New`) based on `TICKET_MODE`.

```bash
sed -n '2770,2865p' cmd/ticket/main.go
```

```output
	fmt.Printf("LastModified : %s\n", task.UpdatedAt)
	fmt.Printf("Acceptance Criteria : %s\n", task.AcceptanceCriteria)
	if len(task.Comments) > 0 {
		fmt.Println("Comments     :")
		for _, comment := range task.Comments {
			fmt.Printf("  - [%s] %s: %s\n", comment.CreatedAt, comment.Author, comment.Text)
		}
	}
}

func formatDependsOn(dependencies []store.Dependency) string {
	var ids []string
	for _, dependency := range dependencies {
		ids = append(ids, strconv.FormatInt(dependency.DependsOn, 10))
	}
	if len(ids) == 0 {
		return "[]"
	}
	return "[" + strings.Join(ids, ",") + "]"
}

func heading(label string) {
	if noColorOutput {
		fmt.Printf("%s\n", label)
		return
	}
	fmt.Printf("\x1b[2;36m%s\x1b[0m\n", label)
}

func resolveCurrentProjectClient() (config.Config, libticket.Service, store.Project, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return config.Config{}, nil, store.Project{}, err
	}

	projectID := strings.TrimSpace(cfg.CurrentProject)
	if projectID == "" {
		projectID = "1"
	}

	project, err := svc.GetProject(projectID)
	if err != nil && projectID != "1" {
		project, err = svc.GetProject("1")
		if err == nil {
			projectID = "1"
		}
	}
	if err != nil {
		if strings.TrimSpace(cfg.CurrentProject) == "" {
			return config.Config{}, nil, store.Project{}, errors.New("no active project; use `ticket project create` or `ticket project use <id>` first")
		}
		return config.Config{}, nil, store.Project{}, err
	}

	if cfg.CurrentProject != projectID {
		cfg.CurrentProject = projectID
		if saveErr := config.Save(cfg); saveErr != nil {
			return config.Config{}, nil, store.Project{}, saveErr
		}
	}
	return cfg, svc, project, nil
}

func resolveService(cfg config.Config) (libticket.Service, error) {
	mode, err := config.ResolveMode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case config.ModeLocal:
		return libticket.NewLocal(cfg), nil
	case config.ModeRemote:
		serverURL := strings.TrimSpace(config.ResolveServerURL(cfg))
		if serverURL == "" {
			return nil, errors.New("TICKET_SERVER is required in remote mode")
		}
		return libtickethttp.New(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported TICKET_MODE %q", mode)
	}
}

func resolveCredentials(usernameFlag, passwordFlag string, useEnv bool) (string, string, error) {
	username := strings.TrimSpace(usernameFlag)
	password := strings.TrimSpace(passwordFlag)

	if useEnv {
		if username == "" {
			username = envValue("TICKET_USERNAME")
		}
		if password == "" {
			password = envValue("TICKET_PASSWORD")
```

## 4) Local identity behavior

In local mode, the default acting user is `admin`. No login/password is required for local command flow.

```bash
sed -n '760,805p' internal/client/client.go; sed -n '440,470p' libticket/local.go; sed -n '2840,2870p' cmd/ticket/main.go
```

```output
func (c *Client) localUser(db *sql.DB) (store.User, error) {
	return ensureLocalUser(db, localUsername())
}

func ensureLocalUser(db *sql.DB, username string) (store.User, error) {
	if user, err := store.GetUserByUsername(db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	user, err := store.CreateUser(db, username, "local-mode", "admin")
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

func localUsername() string {
	return "admin"
}

func getenvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	return store.Open(path)
}

func (s *LocalService) localUser(db *sql.DB) (store.User, error) {
	username := LocalUsername()
	if user, err := store.GetUserByUsername(db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	return store.CreateUser(db, username, "local-mode", "admin")
}

func LocalUsername() string {
	return "admin"
}
		return nil, err
	}
	switch mode {
	case config.ModeLocal:
		return libticket.NewLocal(cfg), nil
	case config.ModeRemote:
		serverURL := strings.TrimSpace(config.ResolveServerURL(cfg))
		if serverURL == "" {
			return nil, errors.New("TICKET_SERVER is required in remote mode")
		}
		return libtickethttp.New(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported TICKET_MODE %q", mode)
	}
}

func resolveCredentials(usernameFlag, passwordFlag string, useEnv bool) (string, string, error) {
	username := strings.TrimSpace(usernameFlag)
	password := strings.TrimSpace(passwordFlag)

	if useEnv {
		if username == "" {
			username = envValue("TICKET_USERNAME")
		}
		if password == "" {
			password = envValue("TICKET_PASSWORD")
		}
	}
	if username == "" {
		username = currentOSUser()
	}
```

## 5) Storage opening and schema management

`internal/store/store.go` opens SQLite with WAL and foreign keys, then ensures schema and migrations are applied on open.

```bash
sed -n '1,180p' internal/store/store.go
```

```output
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/simonski/ticket/internal/password"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Init(path, adminUsername, adminPassword string) error {
	if adminUsername == "" || adminPassword == "" {
		return errors.New("admin username and password are required")
	}

	if path != ":memory:" {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("database already exists at %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
			return err
		}
	}

	db, err := Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		return err
	}

	hash, err := password.Hash(adminPassword)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, role, display_name, enabled)
		VALUES (?, ?, 'admin', ?, 1)
	`, adminUsername, hash, adminUsername)
	if err != nil {
		return err
	}

	if _, err := CreateProject(db, "Default Project", "Bootstrap project created during initdb.", "", 1); err != nil {
		return err
	}
	return nil
}

func createSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
	user_id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	display_name TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	session_id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	token TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT,
	FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS projects (
	project_id INTEGER PRIMARY KEY AUTOINCREMENT,
	prefix TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ticket_sequence INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS tasks (
	task_id INTEGER PRIMARY KEY AUTOINCREMENT,
	key TEXT NOT NULL DEFAULT '',
	project_id INTEGER NOT NULL,
	parent_id INTEGER,
	clone_of INTEGER,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	acceptance_criteria TEXT NOT NULL DEFAULT '',
	stage TEXT NOT NULL DEFAULT 'design',
	state TEXT NOT NULL DEFAULT 'idle',
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER NOT NULL DEFAULT 3,
	sort_order INTEGER NOT NULL DEFAULT 0,
	estimate_effort INTEGER NOT NULL DEFAULT 0,
	estimate_complete TEXT NOT NULL DEFAULT '',
	assignee TEXT NOT NULL DEFAULT '',
	archived INTEGER NOT NULL DEFAULT 0,
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(parent_id) REFERENCES tasks(task_id),
	FOREIGN KEY(clone_of) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS history_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	task_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(task_id) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL,
	task_id INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload TEXT NOT NULL DEFAULT '{}',
	created_by INTEGER,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(project_id),
	FOREIGN KEY(task_id) REFERENCES tasks(task_id),
	FOREIGN KEY(created_by) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
```

## 6) Ticket lifecycle logic

Ticket creation and updates normalize type/lifecycle, validate parent rules, maintain derived status (`stage/state`), and record history events.

```bash
sed -n '80,280p' internal/store/task.go
```

```output
	Stage     string
	State     string
	Status    string
	Search    string
	Assignee  string
	Limit     int
}

type TicketRequestParams struct {
	ProjectID int64
	TicketID  *int64
	TicketRef string
	Username  string
	UserID    int64
	DryRun    bool
}

func CreateTicket(db *sql.DB, params TicketCreateParams) (Ticket, error) {
	params.Type = normalizeTicketType(params.Type)
	params.Title = strings.TrimSpace(params.Title)
	if params.ProjectID == 0 {
		return Ticket{}, errors.New("project is required")
	}
	if params.Title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	if !validTicketType(params.Type) {
		return Ticket{}, fmt.Errorf("invalid ticket type %q", params.Type)
	}
	if params.ParentID != nil {
		parent, err := GetTicket(db, *params.ParentID)
		if err != nil {
			return Ticket{}, err
		}
		if parent.ProjectID != params.ProjectID {
			return Ticket{}, errors.New("parent ticket must be in the same project")
		}
		if err := validateTicketParenting(parent.Type, params.Type); err != nil {
			return Ticket{}, err
		}
	}
	if err := validateEstimateComplete(params.EstimateComplete); err != nil {
		return Ticket{}, err
	}
	stage, state, err := resolveLifecycleForCreate(params.Stage, params.State, params.Assignee)
	if err != nil {
		return Ticket{}, err
	}
	priority := params.Priority
	if priority == 0 {
		priority = 1
	}
	order := params.Order

	tx, err := db.Begin()
	if err != nil {
		return Ticket{}, err
	}
	defer tx.Rollback()
	var projectPrefix string
	var nextSequence int64
	if err := tx.QueryRow(`SELECT prefix, ticket_sequence + 1 FROM projects WHERE project_id = ?`, params.ProjectID).Scan(&projectPrefix, &nextSequence); err != nil {
		return Ticket{}, err
	}
	key, err := generateTicketKey(projectPrefix, params.Type, nextSequence)
	if err != nil {
		return Ticket{}, err
	}
	result, err := tx.Exec(`
		INSERT INTO tasks (key, project_id, parent_id, clone_of, type, title, description, acceptance_criteria, stage, state, status, priority, sort_order, estimate_effort, estimate_complete, assignee, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key, params.ProjectID, nullableInt64(params.ParentID), nullableInt64(params.CloneOf), params.Type, params.Title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), stage, state, RenderLifecycleStatus(stage, state), priority, order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), strings.TrimSpace(params.Assignee), params.CreatedBy)
	if err != nil {
		return Ticket{}, err
	}
	if _, err := tx.Exec(`UPDATE projects SET ticket_sequence = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`, nextSequence, params.ProjectID); err != nil {
		return Ticket{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Ticket{}, err
	}
	if err := tx.Commit(); err != nil {
		return Ticket{}, err
	}
	ticket, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if err := AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_created", map[string]any{
		"key":               ticket.Key,
		"type":              ticket.Type,
		"title":             ticket.Title,
		"stage":             ticket.Stage,
		"state":             ticket.State,
		"status":            ticket.Status,
		"estimate_effort":   ticket.EstimateEffort,
		"estimate_complete": ticket.EstimateComplete,
	}, params.CreatedBy); err != nil {
		return Ticket{}, err
	}
	if err := syncAncestorLifecycle(db, params.ParentID, params.CreatedBy); err != nil {
		return Ticket{}, err
	}
	return GetTicket(db, id)
}

func UpdateTicket(db *sql.DB, id int64, params TicketUpdateParams) (Ticket, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Ticket{}, errors.New("ticket title is required")
	}
	if err := validateEstimateComplete(params.EstimateComplete); err != nil {
		return Ticket{}, err
	}
	current, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	hasChildren, err := ticketHasChildren(db, current.ID)
	if err != nil {
		return Ticket{}, err
	}
	if params.ParentID != nil {
		parent, err := GetTicket(db, *params.ParentID)
		if err != nil {
			return Ticket{}, err
		}
		if parent.ID == current.ID {
			return Ticket{}, errors.New("cannot set ticket as its own parent")
		}
		if parent.ProjectID != current.ProjectID {
			return Ticket{}, errors.New("parent ticket must be in the same project")
		}
		if err := validateTicketParenting(parent.Type, current.Type); err != nil {
			return Ticket{}, err
		}
	}
	assignee := strings.TrimSpace(params.Assignee)
	if err := validateTicketAssignmentChange(current.Assignee, assignee, params.ActorUsername, params.ActorRole); err != nil {
		return Ticket{}, err
	}
	if assignee != "" {
		target, err := GetUserByUsername(db, assignee)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Ticket{}, errors.New("user not found")
			}
			return Ticket{}, err
		}
		if !target.Enabled {
			return Ticket{}, errors.New("user is disabled")
		}
	}

	explicitLifecycle := normalizeOptional(params.Stage) != "" || normalizeOptional(params.State) != ""
	if hasChildren && explicitLifecycle {
		return Ticket{}, errors.New("ticket has children; stage/state is derived from descendants")
	}
	stage, state, err := resolveLifecycleForUpdate(current, params.Stage, params.State, assignee)
	if err != nil {
		return Ticket{}, err
	}
	if explicitLifecycle && (stage != current.Stage || state != current.State) {
		if params.ActorRole != "admin" && strings.TrimSpace(current.Assignee) != strings.TrimSpace(params.ActorUsername) {
			return Ticket{}, ErrForbidden
		}
		if current.Stage == StageDone {
			return Ticket{}, errors.New("done ticket cannot be reopened")
		}
	}

	result, err := db.Exec(`
		UPDATE tasks
		SET title = ?, description = ?, acceptance_criteria = ?, parent_id = ?, assignee = ?, stage = ?, state = ?, status = ?, priority = ?, sort_order = ?, estimate_effort = ?, estimate_complete = ?, updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, title, params.Description, strings.TrimSpace(params.AcceptanceCriteria), nullableInt64(params.ParentID), assignee, stage, state, RenderLifecycleStatus(stage, state), params.Priority, params.Order, params.EstimateEffort, strings.TrimSpace(params.EstimateComplete), id)
	if err != nil {
		return Ticket{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Ticket{}, err
	}
	if affected == 0 {
		return Ticket{}, ErrTicketNotFound
	}
	ticket, err := GetTicket(db, id)
	if err != nil {
		return Ticket{}, err
	}
	if err := AddHistoryEvent(db, ticket.ProjectID, ticket.ID, "ticket_updated", map[string]any{
		"key":                 ticket.Key,
		"title":               ticket.Title,
		"description":         ticket.Description,
		"acceptance_criteria": ticket.AcceptanceCriteria,
		"assignee":            ticket.Assignee,
		"stage":               ticket.Stage,
		"state":               ticket.State,
		"status":              ticket.Status,
		"priority":            ticket.Priority,
```

## 7) HTTP API registration and server wiring

`internal/server/api.go` registers REST handlers. `internal/server/server.go` wires API + static SPA assets and optional verbose request/response logging.

```bash
sed -n '1,180p' internal/server/api.go; sed -n '1,140p' internal/server/server.go
```

```output
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func registerAPI(mux *http.ServeMux, db *sql.DB, version string) {
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var ping int
		if err := db.QueryRow("SELECT 1").Scan(&ping); err != nil {
			writeError(w, http.StatusInternalServerError, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})

	mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.RegisterUser(db, credentials.Username, credentials.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var credentials credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		user, err := store.AuthenticateUser(db, credentials.Username, credentials.Password)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrInvalidCredentials):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, store.ErrForbidden):
				writeError(w, http.StatusForbidden, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		token, err := store.CreateSession(db, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
	})

	mux.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		token := bearerToken(r)
		if err := store.DeleteSession(db, token); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		user, err := userFromRequest(db, r)
		if err != nil {
			if errors.Is(err, store.ErrUnauthorized) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":         "ok",
					"authenticated":  false,
					"server_version": version,
				})
				return
			}
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":         "ok",
			"authenticated":  true,
			"server_version": version,
			"user":           user,
		})
	})

	mux.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := requireUser(db, r); err != nil {
			writeAuthError(w, err)
			return
		}
		var projectID *int64
		if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
			var parsed int64
			if _, err := fmt.Sscan(raw, &parsed); err != nil {
				writeError(w, http.StatusBadRequest, "project_id must be numeric")
				return
			}
			projectID = &parsed
		}
		summary, err := store.CountEverything(db, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			users, err := store.ListUsers(db)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, users)
		case http.MethodPost:
			if _, err := requireAdmin(db, r); err != nil {
				writeAuthError(w, err)
				return
			}
			var credentials credentialsRequest
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			user, err := store.CreateUser(db, credentials.Username, credentials.Password, "user")
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, user)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	web "github.com/simonski/ticket/web"
)

type Server struct {
	httpServer *http.Server
}

func New(addr string, db *sql.DB, version string, verbose bool, output io.Writer) (*Server, error) {
	handler, err := Handler(db, version, verbose, output)
	if err != nil {
		return nil, err
	}
	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}, nil
}

func Handler(db *sql.DB, version string, verbose bool, output io.Writer) (http.Handler, error) {
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerAPI(mux, db, version)

	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", spaHandler(fileServer, staticFS))

	var handler http.Handler = mux
	if verbose {
		handler = loggingHandler(handler, output)
	}
	return handler, nil
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func spaHandler(next http.Handler, staticFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(staticFS, r.URL.Path[1:]); err == nil {
			next.ServeHTTP(w, r)
			return
		}

		r.URL.Path = "/"
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func loggingHandler(next http.Handler, output io.Writer) http.Handler {
	if output == nil {
		output = io.Discard
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(requestBody))
		}

		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		if lw.status == 0 {
			lw.status = http.StatusOK
		}

		fmt.Fprintf(output, "REQUEST %s %s\n", r.Method, r.URL.String())
		if len(requestBody) > 0 {
			fmt.Fprintf(output, "request body: %s\n", string(requestBody))
		}
		fmt.Fprintf(output, "RESPONSE %d\n", lw.status)
		if lw.body.Len() > 0 {
			fmt.Fprintf(output, "response body: %s\n", lw.body.String())
		}
	})
}
```

## 8) Build and quality gates

The Makefile defines expected developer quality gates: unit tests, integration tests, Playwright, and aggregate `make test`.

```bash
sed -n '1,140p' Makefile
```

```output
.PHONY: help default build tools bump-version test test-go test-go-cover test-unit test-integration test-playwright clean

VERSION_FILE := cmd/ticket/VERSION

default: help

help:
	@printf "Available targets:\n\n"
	@printf "  make build           Build the ticket binary into ./bin.\n"
	@printf "                       Also increments the patch version in ./VERSION.\n"
	@printf "  make tools           Build helper binaries in the repo root.\n"
	@printf "  make test            Run all tests.\n"
	@printf "  make test-go         Run Go tests.\n"
	@printf "  make test-unit       Run unit-oriented Go test packages.\n"
	@printf "  make test-integration Run integration-oriented Go test packages.\n"
	@printf "  make test-go-cover   Run Go tests with package coverage thresholds.\n"
	@printf "  make test-playwright Run browser/frontend smoke checks.\n"
	@printf "  make clean           Remove built binaries from ./bin.\n"
	@printf "\n"

build:
	@$(MAKE) bump-version
	@mkdir -p bin
	go build -o ./bin/ticket ./cmd/ticket

tools:
	@mkdir -p bin
	@set -e; \
	for tool in $$(find tools -mindepth 2 -maxdepth 2 -type f -name '*.go' ! -name '*_test.go' | sort); do \
		name=$$(basename $$(dirname $$tool)); \
		printf "Building %s -> bin/%s\n" "$$tool" "$$name"; \
		go build -o "bin/$$name" "$$tool"; \
	done

bump-version:
	@if [ ! -f "$(VERSION_FILE)" ]; then \
		printf "0.1.0\n" > "$(VERSION_FILE)"; \
	else \
		version=$$(tr -d '[:space:]' < "$(VERSION_FILE)"); \
		major=$${version%%.*}; \
		rest=$${version#*.}; \
		minor=$${rest%%.*}; \
		patch=$${rest##*.}; \
		patch=$$((patch + 1)); \
		printf "%s.%s.%s\n" "$$major" "$$minor" "$$patch" > "$(VERSION_FILE)"; \
	fi

UNIT_TEST_PKGS := ./internal/config ./internal/password ./tools/parser ./tools/wiggum ./web
INTEGRATION_TEST_PKGS := ./cmd/ticket ./internal/client ./internal/server ./internal/store ./libticket ./libtickethttp

test: test-unit test-integration test-playwright

test-go:
	go test ./...

test-unit:
	go test $(UNIT_TEST_PKGS)

test-integration:
	go test $(INTEGRATION_TEST_PKGS)

test-go-cover:
	@set -e; \
	for entry in \
		"./cmd/ticket 55" \
		"./libticket 65" \
		"./libtickethttp 75" \
		"./internal/client 55" \
		"./internal/store 70" \
		"./internal/config 70" \
		"./tools/parser 75"; do \
		pkg=$${entry% *}; \
		min=$${entry#* }; \
		out=$$(go test "$$pkg" -cover | tail -n 1); \
		printf "%s\n" "$$out"; \
		pct=$$(printf "%s" "$$out" | sed -n 's/.*coverage: \([0-9.]*\)%.*/\1/p'); \
		if [ -z "$$pct" ]; then \
			printf "could not parse coverage for %s\n" "$$pkg" >&2; \
			exit 1; \
		fi; \
		awk -v pct="$$pct" -v min="$$min" 'BEGIN { if (pct + 0 < min + 0) exit 1 }' || { \
			printf "coverage threshold failed for %s: got %s%%, need %s%%\n" "$$pkg" "$$pct" "$$min" >&2; \
			exit 1; \
		}; \
	done

test-playwright:
	npm install
	npx playwright install chromium
	npx playwright test

clean:
	@rm -rf bin
	@rm -f parser
```

## 9) Reproducibility check

Run `showboat verify` to re-run code blocks and confirm the walkthrough remains consistent with current repository state.

This is an updated linear walkthrough generated for the current codebase state.

Command routing begins in run(), which resolves mode/flags, normalizes args, then dispatches into command handlers.

```sed
rg -n "func run\(|case "health"|func runHealth\(|func resolveCredentials|func renderRootUsage|func commandUsageWidth|func printCommandUsageRows" cmd/ticket/main.go
```

```output
sed: illegal option -- c
usage: sed script [-EHalnru] [-i extension] [file ...]
	sed [-EHalnu] [-i extension] [-e script] ... [-f script_file] ... [file ...]
```

Health path and lifecycle checks are implemented in runHealth().

```sed
-n '1,260p' cmd/ticket/main.go
```

```output
sed: illegal option -- c
usage: sed script [-EHalnru] [-i extension] [file ...]
	sed [-EHalnu] [-i extension] [-e script] ... [-f script_file] ... [file ...]
```

```head
-n
```

```output
head: illegal byte count -- -n
```

```head
-260
```

```output
head: illegal byte count -- -260
```

Linear refresh: captured command routing and health/credential/key-format changes after implementing the requested fixes.

```bash
sed -n '1300,1385p' internal/store/keys.go
```

```output
```

```bash
sed -n '3280,3388p' cmd/ticke```output
		return err
	}
	if strings.TrimSpace(current.Assignee) != strings.TrimSpace(expectedAssignee) {
		return fmt.Errorf("task is not assigned to %s", expectedAssignee)
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           "",
		Stage:              current.Stage,
		State:              map[bool]string{true: store.StateIdle, false: current.State}[current.State == store.StateActive],
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	fmt.Printf("unassigned %s from %s\n", ticketLabel(updated), expectedAssignee)
	return nil
}

func runHistory(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket history <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	task, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	events, err := svc.ListHistory(task.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(events)
	}
	if len(events) == 0 {
		fmt.Println("no history")
		return nil
	}
	for _, event := range events {
		fmt.Printf("ID         : %d\n", event.ID)
		fmt.Printf("TicketID     : %d\n", event.TicketID)
		fmt.Printf("Event      : %s\n", event.EventType)
		fmt.Printf("Created    : %s\n", event.CreatedAt)
		fmt.Printf("Created By : %d\n", event.CreatedBy)
		fmt.Printf("Payload    : %s\n\n", event.Payload)
	}
	return nil
}

func runHealth(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket health <id>")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	task, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	comments, err := svc.ListComments(task.ID)
	if err != nil {
		return err
	}

	checks := ticketHealthCheck(task, comments)
	if outputJSON {
		return printJSON(map[string]any{
			"score":                      checks.score,
			"not_an_orphan":              checks.notOrphan,
			"has_acceptance_criteria":    checks.hasAC,
			"reviewed_by_reviewer_agent": checks.reviewedByReviewer,
			"definition_of_ready":        checks.ready,
		})
	}
	fmt.Println("TICKET HEALTH")
	fmt.Printf("score: %d/4\n", checks.score)
	fmt.Printf("not_an_orphan: %t\n", checks.notOrphan)
	fmt.Printf("has_acceptance_criteria: %t\n", checks.hasAC)
	fmt.Printf("reviewed_by_reviewer_agent: %t\n", checks.reviewedByReviewer)
	fmt.Printf("definition_of_ready: %t\n", checks.ready)
	return nil
}

type ticketHealthResult struct {
	score              int
	notOrphan          bool
	hasAC              bool
	reviewedByReviewer bool
	ready              bool
}

func ticketHealthCheck(task store.Ticket, comments []store.Comment) ticketHealthResult {
	notOrphan := task.Type == "epic" || task.ParentID != nil
	hasAC := strings.TrimSpace(task.AcceptanceCriteria) != ""
	reviewedByReviewer := hasReviewerAgentComment(comments)
	ready := task.Status == "design/idle"
	if !ready {
		stage, state, err := store.ParseLifecycleStatus(task.Status)
		if err == nil {
			ready = stage == store.StageDesign && state == store.StateIdle
		}
```

%-*s\t%s\n", commandWidth, row[0], row[1])
	}
	_ = w.Flush()
}

func printCountSummary(summary store.CountSummary, scopedToProject bool) {

```

```

Linear refresh notes were added for recent health/alias/build behavior changes.
