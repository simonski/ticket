package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http"

	"golang.org/x/term"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/static"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"github.com/simonski/ticket/web"
)

func runOnboard(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk onboard")
	}
	if outputJSON {
		return printJSON(map[string]string{"status": "ok", "content": embeddedAgents})
	}
	fmt.Print(embeddedAgents)
	if !strings.HasSuffix(embeddedAgents, "\n") {
		fmt.Println()
	}
	return nil
}

func runSkill(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tk skill")
	}
	if outputJSON {
		return printJSON(map[string]string{"status": "ok", "content": tkSkillContent})
	}
	fmt.Print(tkSkillContent)
	if !strings.HasSuffix(tkSkillContent, "\n") {
		fmt.Println()
	}
	return nil
}

// tkSkillContent is installed into ~/.claude/skills/tk/SKILL.md so that
// Claude Code automatically knows about the tk CLI while working in any project.
//
//go:embed SKILL.md
var tkSkillContent string

const defaultAdminPassword = "password"

func prompt(reader *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptYN(reader *bufio.Reader, question string, defaultYes bool) bool {
	suffix := " [Y/n]: "
	if !defaultYes {
		suffix = " [y/N]: "
	}
	fmt.Print(question + suffix)
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// initFlags holds optional flags passed to tk init that override interactive prompts.
type initFlags struct {
	workflow string
	prefix   string
	name     string
	git      string
}

type setupDetectedValues struct {
	url       string
	username  string
	projectID string
	source    string
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	workflowFlag := fs.String("workflow", "", "Workflow to assign (e.g. agile, yolo)")
	prefixFlag := fs.String("prefix", "", "project prefix (e.g. TK, PRJ)")
	nameFlag := fs.String("name", "", "project name")
	gitFlag := fs.String("git", "", "git repository URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	flags := initFlags{
		workflow: strings.TrimSpace(*workflowFlag),
		prefix:   strings.ToUpper(strings.TrimSpace(*prefixFlag)),
		name:     strings.TrimSpace(*nameFlag),
		git:      strings.TrimSpace(*gitFlag),
	}

	// Validate flags upfront before any setup work.
	if flags.prefix != "" {
		if matched, _ := regexp.MatchString(`^[A-Z]{1,5}$`, flags.prefix); !matched {
			return fmt.Errorf("invalid prefix %q: must be 1-5 uppercase letters", flags.prefix)
		}
	}
	if flags.workflow != "" {
		builtinWorkflows, err := static.LoadWorkflows()
		if err != nil {
			return fmt.Errorf("could not load Workflows: %w", err)
		}
		found := false
		for _, s := range builtinWorkflows {
			if strings.EqualFold(s.Name, flags.workflow) {
				found = true
				break
			}
		}
		if !found {
			names := make([]string, len(builtinWorkflows))
			for i, s := range builtinWorkflows {
				names[i] = strings.ToLower(s.Name)
			}
			return fmt.Errorf("unknown workflow %q: available Workflows are %s", flags.workflow, strings.Join(names, ", "))
		}
	}

	root, hasProject, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}
	gitRoot, ok := config.FindGitRoot(root)
	if !ok {
		return errors.New("tk init requires a git repository; run git init or clone a repo first")
	}
	root = gitRoot

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("tk init")
	fmt.Println()
	fmt.Printf("git root   : %s\n", root)
	fmt.Printf("config dir : %s\n", filepath.Join(root, ".ticket"))
	fmt.Println()

	detected, detectErr := detectSetupValues(root)
	if detectErr != nil {
		return detectErr
	}
	if strings.TrimSpace(detected.url) != "" || strings.TrimSpace(detected.username) != "" || strings.TrimSpace(detected.projectID) != "" {
		fmt.Println("detected settings")
		if detected.source != "" {
			fmt.Printf("source     : %s\n", detected.source)
		}
		if detected.url != "" {
			fmt.Printf("url        : %s\n", detected.url)
		}
		if detected.username != "" {
			fmt.Printf("username   : %s\n", detected.username)
		}
		if detected.projectID != "" {
			fmt.Printf("project    : %s\n", detected.projectID)
		}
		fmt.Println()
	}

	if hasProject {
		fmt.Printf("project is already initialised at %s\n", filepath.Join(root, ".ticket", "config.json"))
		return nil
	}
	return runSetupNew(reader, flags, detected)
}

func detectGitOrigin() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectSetupValues(root string) (setupDetectedValues, error) {
	values := setupDetectedValues{}
	envURL := strings.TrimSpace(os.Getenv("TICKET_URL"))
	envUser := strings.TrimSpace(os.Getenv("TICKET_USERNAME"))
	envProject := strings.TrimSpace(os.Getenv("TICKET_PROJECT"))
	if envURL != "" || envUser != "" || envProject != "" {
		values.url = envURL
		values.username = envUser
		values.projectID = envProject
		values.source = "environment"
		return values, nil
	}
	path, ok, err := findTicketJSONPath(root)
	if err != nil {
		return setupDetectedValues{}, err
	}
	if !ok {
		return values, nil
	}
	settings, err := parseTicketJSON(path)
	if err != nil {
		return setupDetectedValues{}, err
	}
	values.url = settings.URL
	values.username = settings.Username
	values.projectID = settings.ProjectID
	values.source = settings.Path
	return values, nil
}

func matchProjectByGitOrigin(projects []store.Project, gitOrigin string) *store.Project {
	if gitOrigin == "" {
		return nil
	}
	for i := range projects {
		if projects[i].GitRepository == gitOrigin {
			return &projects[i]
		}
	}
	return nil
}

func runSetupNew(reader *bufio.Reader, f initFlags, detected setupDetectedValues) error {
	fmt.Println()
	remoteDefault := strings.TrimSpace(detected.url) != ""
	defaultChoice := 0
	if remoteDefault {
		defaultChoice = 1
	}
	choice := promptChoiceWithDefault(reader, "Choose setup mode:", []string{"local", "remote"}, defaultChoice)
	if choice == 0 {
		return runSetupLocal(reader, f)
	}
	return runSetupRemote(reader, detected)
}

//nolint:unused // retained temporarily during server-only migration cleanup
func runSetupLocal(reader *bufio.Reader, flags ...initFlags) error {
	var f initFlags
	if len(flags) > 0 {
		f = flags[0]
	}
	root, _, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}

	projectPrefix := f.prefix
	if projectPrefix == "" {
		projectPrefix = prompt(reader, "project prefix", defaultProjectPrefix(root))
		projectPrefix = strings.ToUpper(strings.TrimSpace(projectPrefix))
	}
	projectName := f.name
	if projectName == "" {
		projectName = prompt(reader, "project name", defaultProjectTitle(root))
	}

	gitRepo := f.git
	if gitRepo == "" {
		if origin := detectGitOriginAt(root); origin != "" {
			fmt.Printf("detected   : git origin %s\n", origin)
			if promptYN(reader, "set as project git repository?", true) {
				gitRepo = origin
			}
		}
	}

	if _, err = ensureLocalDatabase(); err != nil {
		return err
	}
	if bindErr := bindRootToLocalProject(root, projectName, projectPrefix, gitRepo); bindErr != nil {
		return bindErr
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dbPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	project, err := svc.GetProject(context.Background(), cfg.ProjectID)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  database : %s\n", dbPath)
	fmt.Printf("  project  : %s (%s)\n", project.Prefix, project.Title)
	fmt.Printf("  user     : admin\n")
	fmt.Printf("  password : %s\n", defaultAdminPassword)
	fmt.Println()

	return runSetupPostInit(reader, f.workflow)
}

func runSetupRemote(reader *bufio.Reader, detected setupDetectedValues) error {
	// 1. Server URL
	defaultURL := defaultTicketURL
	if strings.TrimSpace(detected.url) != "" {
		defaultURL = strings.TrimSpace(detected.url)
	}
	serverURL := prompt(reader, "server URL", defaultURL)
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return errors.New("server URL is required")
	}

	// 2. Verify connectivity
	fmt.Printf("connecting : %s ... ", serverURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/api/healthz", http.NoBody) // #nosec G107 G704 -- URL is entered by the operator during setup, not constructed from untrusted input
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("could not reach server: %w", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("FAILED")
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	fmt.Println("OK")

	// 3. Ensure the remote exists globally so resolveService can connect during setup.
	remoteNameDefault := defaultRemoteNameForURL(serverURL)
	remoteName := prompt(reader, "remote name", remoteNameDefault)
	cfg, remoteName, err := ensureNamedRemote(remoteName, serverURL)
	if err != nil {
		return err
	}
	cfg.Remote = remoteName
	cfg.Location = serverURL

	// 4. Authentication
	fmt.Println()
	defaultUsername := cfg.Username
	if strings.TrimSpace(defaultUsername) == "" {
		defaultUsername = strings.TrimSpace(detected.username)
	}
	defaultPassword := ""

	hasAccount := promptYN(reader, "do you have an account on this server?", true)

	username, password, err := promptForCredentials(os.Stdin, os.Stdout, defaultUsername, defaultPassword)
	if err != nil {
		return err
	}

	cfg, err = config.Load()
	if err != nil {
		return err
	}
	cfg.Remote = remoteName
	cfg.Location = serverURL
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if !hasAccount {
		if _, registerErr := svc.Register(context.Background(), username, password); registerErr != nil {
			return fmt.Errorf("registration failed: %w", registerErr)
		}
		fmt.Printf("  registered user: %s\n", username)
	}
	_, token, err := svc.Login(context.Background(), username, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	cfg.Username = username
	cfg.Token = token

	// Save credentials
	if saveCredsErr := config.SaveRemoteCredentials(serverURL, cfg.Username, cfg.Token); saveCredsErr != nil {
		return saveCredsErr
	}
	fmt.Printf("  user     : %s\n", cfg.Username)
	fmt.Println()

	// 5. Project selection
	// Reload service with token
	if saveCfgErr := config.Save(cfg); saveCfgErr != nil {
		return saveCfgErr
	}
	cfg, err = config.Load()
	if err != nil {
		return err
	}
	svc, err = resolveService(cfg)
	if err != nil {
		return err
	}

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		return fmt.Errorf("could not list projects: %w", err)
	}

	// Try git-origin matching first
	gitOrigin := detectGitOrigin()
	if gitOrigin != "" {
		fmt.Printf("detected   : git origin %s\n", gitOrigin)
	}

	if match := matchProjectByGitOrigin(projects, gitOrigin); match != nil {
		fmt.Printf("  matches  : %s (%s)\n", match.Prefix, match.Title)
		if promptYN(reader, "use this project?", true) {
			root, _, rootErr := currentOrAncestorProjectRoot()
			if rootErr != nil {
				return rootErr
			}
			if err := bindRootToRemoteProject(root, remoteName, fmt.Sprintf("%d", match.ID)); err != nil {
				return err
			}
			fmt.Printf("  project  : %s (%s)\n", match.Prefix, match.Title)
			fmt.Println()
			return runSetupPostInit(reader)
		}
	} else if gitOrigin != "" {
		// No matching project on server — offer to create one based on git origin
		fmt.Println("  no matching project found on server.")
		if promptYN(reader, "create a project for this repository?", true) {
			return setupCreateRemoteProject(reader, svc, cfg)
		}
	}

	if len(projects) == 0 {
		fmt.Println("no projects found on server.")
		if promptYN(reader, "create a new project?", true) {
			return setupCreateRemoteProject(reader, svc, cfg)
		}
		fmt.Println("no project selected.")
		return runSetupPostInit(reader)
	}

	// Show project list
	fmt.Println()
	options := make([]string, len(projects)+1)
	for i, p := range projects {
		options[i] = fmt.Sprintf("%s — %s", p.Prefix, p.Title)
	}
	options[len(projects)] = "Create a new project"

	choice := promptChoice(reader, "Select a project:", options)

	if choice == len(projects) {
		return setupCreateRemoteProject(reader, svc, cfg)
	}

	selected := projects[choice]
	root, _, rootErr := currentOrAncestorProjectRoot()
	if rootErr != nil {
		return rootErr
	}
	if err := bindRootToRemoteProject(root, remoteName, fmt.Sprintf("%d", selected.ID)); err != nil {
		return err
	}
	fmt.Printf("  project  : %s (%s)\n", selected.Prefix, selected.Title)
	fmt.Println()
	return runSetupPostInit(reader)
}

func setupCreateRemoteProject(reader *bufio.Reader, svc libticket.Service, cfg config.Config) error {
	root, _, err := currentOrAncestorProjectRoot()
	if err != nil {
		return err
	}

	prefix := prompt(reader, "project prefix", defaultProjectPrefix(root))
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	title := prompt(reader, "project name", defaultProjectTitle(root))

	var gitRepo string
	if origin := detectGitOriginAt(root); origin != "" {
		fmt.Printf("detected   : git origin %s\n", origin)
		if promptYN(reader, "set as project git repository?", true) {
			gitRepo = origin
		}
	}

	project, err := svc.CreateProject(context.Background(), libticket.ProjectCreateRequest{
		Prefix:        prefix,
		Title:         title,
		GitRepository: gitRepo,
	})
	if err != nil {
		return err
	}
	if err := bindRootToRemoteProject(root, cfg.Remote, fmt.Sprintf("%d", project.ID)); err != nil {
		return err
	}
	fmt.Printf("  project  : %s (%s)\n", project.Prefix, project.Title)
	fmt.Println()
	return runSetupPostInit(reader)
}

func runSetupPostInit(reader *bufio.Reader, workflowName ...string) error {
	workflow := ""
	if len(workflowName) > 0 {
		workflow = workflowName[0]
	}
	// Detect claude / codex
	claudePath, _ := exec.LookPath("claude")
	codexPath, _ := exec.LookPath("codex")

	if claudePath != "" {
		fmt.Printf("detected   : claude (%s)\n", claudePath)
	}
	if codexPath != "" {
		fmt.Printf("detected   : codex  (%s)\n", codexPath)
	}

	if claudePath != "" {
		fmt.Println()
		cwd, _ := os.Getwd()
		localSkillPath := filepath.Join(cwd, ".claude", "skills", "tk", "SKILL.md")
		globalSkillPath := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "tk", "SKILL.md")

		var existingPath string
		var existingContent []byte
		for _, p := range []string{localSkillPath, globalSkillPath} {
			if data, readErr := os.ReadFile(p); readErr == nil { // #nosec G304 G703 -- p is a well-known skill path, not untrusted user input
				existingPath = p
				existingContent = data
				break
			}
		}

		if existingPath != "" {
			if string(existingContent) == tkSkillContent {
				fmt.Printf("skill      : %s (up to date)\n", existingPath)
			} else {
				fmt.Printf("skill      : %s (out of date)\n", existingPath)
				if promptYN(reader, "update tk skill to latest version?", true) {
					if err := os.WriteFile(existingPath, []byte(tkSkillContent), 0o644); err != nil { // #nosec G306 G703 -- skill file is documentation, 0644 is intentional
						fmt.Printf("  warning: could not update skill: %v\n", err)
					} else {
						fmt.Printf("  updated: %s\n", existingPath)
					}
				}
			}
		} else {
			localSkillDir := filepath.Dir(localSkillPath)
			globalSkillDir := filepath.Dir(globalSkillPath)
			if promptYN(reader, "install tk skill for Claude Code?", true) {
				fmt.Printf("  [1] local   %s\n", localSkillPath)
				fmt.Printf("  [2] global  %s\n", globalSkillPath)
				choice := prompt(reader, "install location", "1")
				var skillDir string
				switch strings.TrimSpace(choice) {
				case "2", "global":
					skillDir = globalSkillDir
				default:
					skillDir = localSkillDir
				}
				if err := os.MkdirAll(skillDir, 0o755); err != nil { // #nosec G301 G703 -- skill directory is public documentation, world-readable is intentional
					fmt.Printf("  warning: could not create skill dir: %v\n", err)
				} else {
					skillPath := filepath.Join(skillDir, "SKILL.md")
					if err := os.WriteFile(skillPath, []byte(tkSkillContent), 0o644); err != nil { // #nosec G306 G703 -- skill file is documentation, 0644 is intentional
						fmt.Printf("  warning: could not write skill: %v\n", err)
					} else {
						fmt.Printf("  installed: %s\n", skillPath)
					}
				}
			}
		}
	}

	// Check for CLAUDE.md and AGENTS.md
	cwd, _ := os.Getwd()
	claudeMD := filepath.Join(cwd, "CLAUDE.md")
	agentsMD := filepath.Join(cwd, "AGENTS.md")

	if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
		fmt.Println()
		if promptYN(reader, "CLAUDE.md not found — create it?", true) {
			content := "Read AGENTS.md\n"
			if writeErr := os.WriteFile(claudeMD, []byte(content), 0o644); writeErr != nil { // #nosec G306 -- CLAUDE.md is documentation, 0644 is intentional
				fmt.Printf("  warning: could not write CLAUDE.md: %v\n", writeErr)
			} else {
				fmt.Printf("  created: %s\n", claudeMD)
			}
		}
	} else {
		fmt.Printf("detected   : %s\n", claudeMD)
	}

	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		if promptYN(reader, "AGENTS.md not found — create it?", true) {
			if writeErr := os.WriteFile(agentsMD, []byte(embeddedAgents), 0o644); writeErr != nil { // #nosec G306 -- AGENTS.md is documentation, 0644 is intentional
				fmt.Printf("  warning: could not write AGENTS.md: %v\n", writeErr)
			} else {
				fmt.Printf("  created: %s\n", agentsMD)
			}
		}
	} else {
		fmt.Printf("detected   : %s\n", agentsMD)
	}

	// Check .gitignore for credentials file
	gitignorePath := filepath.Join(cwd, ".gitignore")
	credEntry := ".ticket/credentials.json"                          // #nosec G101 -- this is a path string, not a credential
	if data, readErr := os.ReadFile(gitignorePath); readErr == nil { // #nosec G304 -- gitignorePath is derived from cwd, not user input
		if !strings.Contains(string(data), credEntry) {
			fmt.Println()
			fmt.Printf("warning    : %s is not in .gitignore\n", credEntry)
			if promptYN(reader, "add .ticket/credentials.json to .gitignore?", true) {
				f, appendErr := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644) // #nosec G302 G304 -- .gitignore is world-readable by convention
				if appendErr != nil {
					fmt.Printf("  warning: could not open .gitignore: %v\n", appendErr)
				} else {
					if len(data) > 0 && data[len(data)-1] != '\n' {
						if _, err := f.WriteString("\n"); err != nil {
							fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
						}
					}
					if _, err := f.WriteString(credEntry + "\n"); err != nil {
						fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
					}
					if err := f.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "warning: could not close .gitignore: %v\n", err)
					}
					fmt.Printf("  added: %s to .gitignore\n", credEntry)
				}
			}
		}
	} else if os.IsNotExist(readErr) {
		fmt.Println()
		fmt.Printf("warning    : no .gitignore found — consider adding %s\n", credEntry)
	}

	fmt.Println()

	// Check that workflows and roles are populated.
	cfg, cfgErr := config.Load()
	if cfgErr == nil {
		if err := runInitCheckDefaults(reader, cfg, workflow); err != nil {
			fmt.Printf("warning: could not check defaults: %v\n", err)
		}
	}

	fmt.Println("setup complete. run `tk` to list tickets.")
	return nil
}

func runInitDB(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPathFlag := fs.String("f", "", "SQLite database file")
	passwordFlag := fs.String("password", "", "bootstrap password")
	force := fs.Bool("force", false, "overwrite the database file if it exists")
	populate := fs.Bool("populate", false, "seed example projects, stories, tickets, users, and teams")
	workflowFlag := fs.String("workflow", "", "Workflow to assign to the project (e.g. agile, yolo)")
	prefixFlag := fs.String("prefix", "", "project prefix (e.g. TK, PRJ)")
	nameFlag := fs.String("name", "", "project name")
	gitFlag := fs.String("git", "", "git repository URL")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: tk initdb [<path>] [-f <db-path>] [--force] [-password <password>] [-populate]")
	}
	if fs.NArg() == 1 && strings.TrimSpace(*dbPathFlag) != "" {
		return errors.New("use either a positional path or -f, not both")
	}

	var (
		dbPath      string
		projectRoot string
		err         error
	)
	switch {
	case strings.TrimSpace(*dbPathFlag) != "":
		dbPath = strings.TrimSpace(*dbPathFlag)
	case fs.NArg() == 1:
		projectRoot, err = filepath.Abs(strings.TrimSpace(fs.Arg(0)))
		if err != nil {
			return err
		}
		dbPath = filepath.Join(projectRoot, ".ticket", "ticket.db")
	default:
		dbPath, err = config.LocalDBPath()
		if err != nil {
			return err
		}
	}

	password := strings.TrimSpace(*passwordFlag)
	if password == "" {
		password = defaultAdminPassword
	}

	if *force {
		if err := removeDBFiles(dbPath); err != nil {
			return err
		}
	}

	dbExists := false
	if _, statErr := os.Stat(dbPath); statErr == nil {
		dbExists = true
	}

	if dbExists && !*force {
		fmt.Printf("database already exists at %s (use --force to overwrite)\n", dbPath)
	} else {
		if err := store.Init(dbPath, "admin", password, static.SeedDatabase); err != nil {
			return err
		}
		if *populate {
			db, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			if err := seedExampleData(db); err != nil {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close database: %v\n", closeErr)
				}
				return err
			}
			if err := db.Close(); err != nil {
				return err
			}
		}
	}

	cfg := config.Config{
		Location:  "file://" + dbPath,
		ProjectID: "TK",
	}

	fmt.Printf("initialized database at %s\n", dbPath)
	if !dbExists || *force {
		fmt.Printf("admin user: admin\n")
		fmt.Printf("admin password: %s\n", password)
		if *populate {
			fmt.Println("example data: seeded")
		}
	}

	// Seed built-in roles and Workflows on a fresh init.
	if !dbExists || *force {
		reader := bufio.NewReader(os.Stdin)
		if err := runInitCheckDefaults(reader, cfg, *workflowFlag); err != nil {
			fmt.Printf("warning: could not check defaults: %v\n", err)
		}
	}

	if projectRoot != "" {
		_, remoteName, err := ensureNamedLocalRemote(projectRoot, dbPath)
		if err != nil {
			return err
		}
		if err := config.SaveProjectConfigAt(projectRoot, config.Config{Remote: remoteName}); err != nil {
			return err
		}
		fmt.Printf("remote    : %s\n", remoteName)
		fmt.Printf("repo      : %s\n", projectRoot)
	} else {
		if _, err := ensureDefaultLocalRemote(dbPath); err != nil {
			return err
		}
	}

	// Apply project settings from flags.
	if *prefixFlag != "" || *nameFlag != "" || *gitFlag != "" {
		svc, svcErr := resolveService(cfg)
		if svcErr == nil {
			update := libticket.ProjectUpdateRequest{}
			if *nameFlag != "" {
				update.Title = *nameFlag
			}
			if *gitFlag != "" {
				update.GitRepository = *gitFlag
			}
			if _, err := svc.UpdateProject(context.Background(), 1, update); err != nil {
				fmt.Printf("warning: could not update project: %v\n", err)
			}
			if *prefixFlag != "" {
				prefix := strings.ToUpper(strings.TrimSpace(*prefixFlag))
				if _, err := svc.RenameProjectPrefix(context.Background(), 1, prefix); err != nil {
					fmt.Printf("warning: could not set prefix: %v\n", err)
				}
			}
		}
	}
	return nil
}

// runInitCheckDefaults checks whether the current project has a workflow with
// stages, and whether any roles exist. If not, it seeds them from the
// built-in role and Workflow templates in internal/static/.
func runInitCheckDefaults(reader *bufio.Reader, cfg config.Config, workflowName string) error {
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	// ── Roles (seed first — Workflows reference them) ────────────────────────
	existingRoles, err := svc.ListRoles(context.Background())
	if err != nil {
		return err
	}
	// Build a map of title → role ID for stage-role assignment later.
	roleIDByRef := make(map[string]int64)
	for _, r := range existingRoles {
		roleIDByRef[strings.ToLower(r.Title)] = r.ID
	}
	if len(existingRoles) == 0 {
		builtinRoles, loadErr := static.LoadRoles()
		if loadErr != nil {
			fmt.Printf("  warning: could not load built-in roles: %v\n", loadErr)
		} else {
			for _, r := range builtinRoles {
				created, rErr := svc.CreateRole(context.Background(), libticket.RoleRequest{
					Title:              r.Title,
					Description:        r.Description,
					AcceptanceCriteria: r.AcceptanceCriteria,
				})
				if rErr != nil {
					fmt.Printf("  warning: could not create role %q: %v\n", r.Title, rErr)
				} else {
					roleIDByRef[r.Filename] = created.ID
					roleIDByRef[strings.ToLower(r.Title)] = created.ID
				}
			}
			fmt.Printf("roles      : %d created\n", len(builtinRoles))
		}
	} else {
		fmt.Printf("roles      : %d found\n", len(existingRoles))
	}

	// ── Workflow ─────────────────────────────────────────────────────────────
	project, err := svc.GetProject(context.Background(), cfg.ProjectID)
	if err != nil {
		return err
	}

	// ── Workflows ────────────────────────────────────────────────────────────
	// Create all built-in Workflows from static seed files.
	builtinWorkflows, loadErr := static.LoadWorkflows()
	if loadErr != nil {
		fmt.Printf("  warning: could not load built-in Workflows: %v\n", loadErr)
	}
	// Track which seed is the default.
	defaultSeedName := ""
	seedNames := make(map[string]bool)
	for _, seed := range builtinWorkflows {
		seedNames[strings.ToLower(seed.Name)] = true
		if seed.Default {
			defaultSeedName = seed.Name
		}
	}
	// Remove the bootstrap "default" Workflow created by store.Init if it's
	// not one of the static seed Workflows.
	existingWorkflows, err := svc.ListWorkflows(context.Background())
	if err != nil {
		return err
	}
	for _, s := range existingWorkflows {
		if !seedNames[strings.ToLower(s.Name)] {
			if delErr := svc.DeleteWorkflow(context.Background(), s.ID); delErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not delete Workflow %q: %v\n", s.Name, delErr)
			}
		}
	}
	// Now create the real Workflows from static files.
	existingWorkflows, err = svc.ListWorkflows(context.Background())
	if err != nil {
		return err
	}
	existingNames := make(map[string]bool)
	for _, s := range existingWorkflows {
		existingNames[strings.ToLower(s.Name)] = true
	}
	for _, seed := range builtinWorkflows {
		if existingNames[strings.ToLower(seed.Name)] {
			continue
		}
		wf, wfErr := svc.CreateWorkflow(context.Background(), libticket.WorkflowRequest{
			Name:        seed.Name,
			Description: seed.Description,
		})
		if wfErr != nil {
			fmt.Printf("  warning: could not create workflow %q: %v\n", seed.Name, wfErr)
			continue
		}
		if err := seedWorkflowStages(svc, wf.ID, seed, roleIDByRef); err != nil {
			fmt.Printf("  warning: could not add stages to %q: %v\n", seed.Name, err)
		}
	}

	// Assign an Workflow to the project.
	allWorkflows, _ := svc.ListWorkflows(context.Background())
	needsWorkflow := project.WorkflowID == nil || workflowName != ""
	if needsWorkflow && len(allWorkflows) > 0 {
		var chosenID int64
		if workflowName != "" {
			// Flag provided — find by name.
			for _, s := range allWorkflows {
				if strings.EqualFold(s.Name, workflowName) {
					chosenID = s.ID
					break
				}
			}
			if chosenID == 0 {
				fmt.Printf("  warning: workflow %q not found, using default\n", workflowName)
			}
		}
		if chosenID == 0 && len(allWorkflows) == 1 {
			chosenID = allWorkflows[0].ID
		}
		if chosenID == 0 {
			defaultIdx := 0
			for i, s := range allWorkflows {
				if s.Name == defaultSeedName {
					defaultIdx = i
				}
			}
			// If stdin is a terminal and no flag was given, prompt the user.
			if workflowName == "" && term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Println()
				options := make([]string, len(allWorkflows))
				for i, s := range allWorkflows {
					workflowDetail, _ := svc.GetWorkflow(context.Background(), s.ID)
					stageNames := make([]string, len(workflowDetail.Stages))
					for j, st := range workflowDetail.Stages {
						stageNames[j] = st.StageName
					}
					label := fmt.Sprintf("%s — %s (%s)", s.Name, s.Description, strings.Join(stageNames, " → "))
					if s.Name == defaultSeedName {
						label += " [default]"
					}
					options[i] = label
				}
				choice := promptChoiceWithDefault(reader, "Choose an Workflow for this project:", options, defaultIdx)
				chosenID = allWorkflows[choice].ID
			} else {
				chosenID = allWorkflows[defaultIdx].ID
			}
		}
		projectID, parseErr := strconv.ParseInt(cfg.ProjectID, 10, 64)
		if parseErr == nil {
			if _, pErr := svc.UpdateProject(context.Background(), projectID, libticket.ProjectUpdateRequest{WorkflowID: &chosenID}); pErr != nil {
				fmt.Printf("  warning: could not assign workflow: %v\n", pErr)
			}
		}
		chosen, _ := svc.GetWorkflow(context.Background(), chosenID)
		stageNames := make([]string, len(chosen.Stages))
		for i, s := range chosen.Stages {
			stageNames[i] = s.StageName
		}
		fmt.Printf("workflow       : %q (%s)\n", chosen.Name, strings.Join(stageNames, " → "))
	} else if project.WorkflowID != nil {
		wf, wfErr := svc.GetWorkflow(context.Background(), *project.WorkflowID)
		if wfErr == nil {
			fmt.Printf("workflow       : %q (%d stages)\n", wf.Name, len(wf.Stages))
		}
	}

	return nil
}

// seedWorkflowStages creates stages and assigns roles from an Workflow seed template.
func seedWorkflowStages(svc libticket.Service, workflowID int64, seed static.Workflow, roleIDByRef map[string]int64) error {
	for _, s := range seed.Stages {
		stage, err := svc.AddWorkflowStage(context.Background(), workflowID, libticket.WorkflowStageRequest{
			StageName:   s.Name,
			Description: s.Description,
			SortOrder:   s.Order,
		})
		if err != nil {
			return fmt.Errorf("stage %q: %w", s.Name, err)
		}
		// Assign roles to the stage.
		for _, roleRef := range s.Roles {
			if rid, ok := roleIDByRef[roleRef.RoleRef]; ok {
				if err := svc.AddWorkflowStageRole(context.Background(), workflowID, stage.ID, rid); err != nil {
					fmt.Printf("  warning: could not assign role %q to stage %q: %v\n", roleRef.RoleRef, s.Name, err)
				}
			}
		}
	}
	return nil
}

//nolint:unused // retained temporarily during server-only migration cleanup
func runExportSnapshot(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outputPath := fs.String("o", "ticket-snapshot.json", "snapshot output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: tk export [-o <snapshot-file>]")
	}
	path := strings.TrimSpace(*outputPath)
	if path == "" {
		return errors.New("snapshot file path is required")
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	snapshot, err := store.ExportSnapshot(context.Background(), db)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil { // #nosec G306 -- snapshot export is a user-facing data file, 0644 is intentional
		return err
	}
	fmt.Printf("exported snapshot to %s\n", path)
	return nil
}

//nolint:unused // retained temporarily during server-only migration cleanup
func runImportSnapshot(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	inputPath := fs.String("i", "", "snapshot input file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: tk import -i <snapshot-file>")
	}
	path := strings.TrimSpace(*inputPath)
	if path == "" {
		return errors.New("usage: tk import -i <snapshot-file>")
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is a CLI flag provided by the operator, not untrusted input
	if err != nil {
		return err
	}
	var snapshot store.Snapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decodeErr := decoder.Decode(&snapshot); decodeErr != nil {
		return decodeErr
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	db, err := store.Open(resolved.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.ImportSnapshot(context.Background(), db, snapshot); err != nil {
		return err
	}
	fmt.Printf("imported snapshot from %s\n", path)
	return nil
}

func seedExampleData(db *sql.DB) error {
	type seedStory struct {
		title       string
		description string
		epicTitle   string
		taskTitle   string
		bugTitle    string
		choreTitle  string
	}
	type seedProject struct {
		prefix      string
		title       string
		description string
		stories     []seedStory
	}
	projects := []seedProject{
		{
			prefix:      "CRM",
			title:       "Customer Relationship Portal",
			description: "Sample CRM modernization project with customer workflows.",
			stories: []seedStory{
				{
					title:       "Customer onboarding lifecycle",
					description: "As operations, we need guided onboarding states and notifications.",
					epicTitle:   "Onboarding workflow foundation",
					taskTitle:   "Implement onboarding timeline UI",
					bugTitle:    "Fix duplicate welcome email trigger",
					choreTitle:  "Refactor onboarding API response contract",
				},
				{
					title:       "Account health insights",
					description: "As account managers, we need a view of customer risk signals.",
					epicTitle:   "Account health scoring",
					taskTitle:   "Build account health dashboard widgets",
					bugTitle:    "Correct stale account-score cache invalidation",
					choreTitle:  "Rotate integration API keys and update docs",
				},
			},
		},
		{
			prefix:      "BIL",
			title:       "Billing Reliability Program",
			description: "Sample billing platform stabilization project.",
			stories: []seedStory{
				{
					title:       "Invoice generation resilience",
					description: "As finance, invoice runs should recover gracefully from partial failures.",
					epicTitle:   "Invoice pipeline hardening",
					taskTitle:   "Add idempotent retry for invoice batches",
					bugTitle:    "Resolve timezone offset in invoice due-date generation",
					choreTitle:  "Archive legacy invoice templates",
				},
				{
					title:       "Payment reconciliation transparency",
					description: "As support, we need clear reconciliation states for customer payments.",
					epicTitle:   "Reconciliation visibility",
					taskTitle:   "Implement reconciliation state timeline",
					bugTitle:    "Fix missing failure reason in reconciliation events",
					choreTitle:  "Backfill historical payment metadata",
				},
			},
		},
		{
			prefix:      "OPS",
			title:       "Operations Automation Suite",
			description: "Sample ops automation project with runbooks and alerts.",
			stories: []seedStory{
				{
					title:       "Incident triage acceleration",
					description: "As SRE, we need faster incident signal correlation.",
					epicTitle:   "Incident triage workbench",
					taskTitle:   "Build correlated alert feed view",
					bugTitle:    "Fix noisy alert dedupe rule for repeated events",
					choreTitle:  "Retire unused pager escalation policies",
				},
				{
					title:       "Runbook execution consistency",
					description: "As platform engineers, runbook runs should be reproducible and auditable.",
					epicTitle:   "Runbook orchestration controls",
					taskTitle:   "Add preflight checks to runbook executor",
					bugTitle:    "Fix race in parallel runbook step logging",
					choreTitle:  "Normalize runbook naming conventions",
				},
			},
		},
	}

	// Look up admin user for CreatedBy
	adminUser, err := store.GetUserByUsername(context.Background(), db, "admin")
	if err != nil {
		return fmt.Errorf("seed: admin user not found: %w", err)
	}
	seedCreatedBy := adminUser.ID

	for _, projectSeed := range projects {
		project, err := store.CreateProjectWithParams(context.Background(), db, store.ProjectCreateParams{
			Prefix:      projectSeed.prefix,
			Title:       projectSeed.title,
			Description: projectSeed.description,
			CreatedBy:   seedCreatedBy,
			Visibility:  store.ProjectVisibilityPublic,
		})
		if err != nil {
			return err
		}
		for _, storySeed := range projectSeed.stories {
			story, err := store.CreateStory(context.Background(), db, project.ID, storySeed.title, storySeed.description, seedCreatedBy)
			if err != nil {
				return err
			}
			epic, err := store.CreateTicket(context.Background(), db, store.TicketCreateParams{
				ProjectID: project.ID,
				Type:      "epic",
				Title:     storySeed.epicTitle,
				CreatedBy: seedCreatedBy,
			})
			if err != nil {
				return err
			}
			if err := store.LinkStoryToTicket(context.Background(), db, story.ID, epic.ID); err != nil {
				return err
			}
			for _, child := range []struct {
				ticketType string
				title      string
			}{
				{ticketType: "task", title: storySeed.taskTitle},
				{ticketType: "bug", title: storySeed.bugTitle},
				{ticketType: "chore", title: storySeed.choreTitle},
			} {
				parentID := epic.ID
				childTicket, err := store.CreateTicket(context.Background(), db, store.TicketCreateParams{
					ProjectID: project.ID,
					ParentID:  &parentID,
					Type:      child.ticketType,
					Title:     child.title,
					CreatedBy: seedCreatedBy,
				})
				if err != nil {
					return err
				}
				if err := store.LinkStoryToTicket(context.Background(), db, story.ID, childTicket.ID); err != nil {
					return err
				}
			}
		}
	}

	seedUsers := []struct {
		username string
		team     string
		role     string
		title    string
	}{
		{username: "alice", team: "Platform Engineering", role: store.TeamRoleOwner, title: "Platform Lead"},
		{username: "bob", team: "Platform Engineering", role: store.TeamRoleMember, title: "Senior Software Engineer"},
		{username: "carol", team: "Product Delivery", role: store.TeamRoleOwner, title: "Product Manager"},
		{username: "dave", team: "Product Delivery", role: store.TeamRoleMember, title: "Delivery Engineer"},
		{username: "erin", team: "Quality Engineering", role: store.TeamRoleOwner, title: "QA Lead"},
		{username: "frank", team: "Quality Engineering", role: store.TeamRoleMember, title: "Test Automation Engineer"},
	}

	teamsByName := map[string]int64{}
	for _, item := range seedUsers {
		if _, ok := teamsByName[item.team]; ok {
			continue
		}
		team, err := store.CreateTeam(context.Background(), db, item.team, nil)
		if err != nil {
			return err
		}
		teamsByName[item.team] = team.ID
	}
	for _, item := range seedUsers {
		user, err := store.CreateUser(context.Background(), db, item.username, "password", "user")
		if err != nil {
			return err
		}
		teamID := teamsByName[item.team]
		if _, err := store.AddTeamMember(context.Background(), db, teamID, user.ID, item.role, item.title); err != nil {
			return err
		}
	}
	return nil
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPath := fs.String("f", "", "SQLite database file")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	port := fs.Int("p", 0, "HTTP listen port (shorthand for -addr :<port>)")
	verbose := fs.Bool("v", false, "print verbose request/response logs to stdout")
	staticPath := fs.String("path", "", "serve static files from this filesystem path instead of embedded assets")
	siteName := fs.String("site", web.Site2, "embedded site bundle to serve (default or site2)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	explicitDBPath := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "f" {
			explicitDBPath = true
		}
	})
	resolvedDBPath := strings.TrimSpace(*dbPath)
	if explicitDBPath {
		if resolvedDBPath == "" {
			return errors.New("missing value for -f")
		}
	} else {
		defaultDBPath, err := defaultDatabasePath()
		if err != nil {
			return err
		}
		resolvedDBPath = defaultDBPath
	}

	listenAddr := strings.TrimSpace(*addr)
	if *port > 0 {
		if *port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
		if strings.TrimSpace(*addr) != ":8080" {
			return errors.New("use either -p or -addr, not both")
		}
		listenAddr = fmt.Sprintf(":%d", *port)
	}

	db, err := store.Open(resolvedDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	selectedSite := strings.TrimSpace(*siteName)
	if selectedSite == "" {
		selectedSite = web.Site2
	}

	srv, err := server.New(listenAddr, db, strings.TrimSpace(embeddedVersion), *verbose, os.Stdout, *staticPath, selectedSite)
	if err != nil {
		return err
	}

	fmt.Print(renderBanner())
	fmt.Printf("VERSION    %s\n", strings.TrimSpace(embeddedVersion))
	fmt.Printf("SITE       %s\n", selectedSite)
	fmt.Printf("TICKETDB   %s\n\n", resolvedDBPath)
	fmt.Printf("serving tk on http://localhost%s\n", listenAddr)

	// Run the server in a goroutine so we can listen for shutdown signals.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		// Server stopped on its own (e.g. bind error).
		return err
	case sig := <-quit:
		fmt.Printf("\nreceived %s, shutting down gracefully...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		fmt.Println("server stopped")
		return nil
	}
}
