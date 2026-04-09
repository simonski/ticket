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
	"path/filepath"
	"strconv"
	"strings"

	"net/http"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

func runOnboard(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket onboard")
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

// tkSkillContent is installed into ~/.claude/skills/tk/SKILL.md so that
// Claude Code automatically knows about the tk CLI while working in any project.
//
//go:embed SKILL.md
var tkSkillContent string

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

func runSetup(args []string) error {
	_ = args
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("tk init")
	fmt.Println()

	// Require a git repository — walk up from cwd looking for .git
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	gitRoot, hasGit := config.FindGitRoot(cwd)
	if !hasGit {
		return fmt.Errorf("no .git directory found.\n  tk requires a git repository. Run `git init` first, then re-run `tk init`")
	}
	ticketDir := filepath.Join(gitRoot, ".ticket")
	fmt.Printf("git root   : %s\n", gitRoot)
	fmt.Printf("config dir : %s\n", ticketDir)
	fmt.Println()

	if existingSetup() {
		return runSetupExisting(reader)
	}
	return runSetupNew(reader)
}

func existingSetup() bool {
	home, err := config.Home()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, "config.json"))
	return err == nil
}

func detectGitOrigin() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func runSetupExisting(reader *bufio.Reader) error {
	cfg, _ := config.Load()
	resolved, _ := config.ResolveURL()

	// Show current state
	fmt.Println("Current configuration:")
	fmt.Printf("  location : %s\n", cfg.Location)
	if resolved.Mode == config.ModeRemote {
		fmt.Printf("  mode     : remote (%s)\n", resolved.ServerURL)
		if u := envValue("TICKET_USERNAME"); u != "" {
			fmt.Printf("  TICKET_USERNAME : %s\n", u)
		}
		if envValue("TICKET_PASSWORD") != "" {
			fmt.Printf("  TICKET_PASSWORD : ***\n")
		}
	} else {
		fmt.Printf("  mode     : local\n")
		fmt.Printf("  database : %s\n", resolved.DBPath)
	}
	fmt.Printf("  user     : %s\n", cfg.Username)
	fmt.Printf("  project  : %s\n", cfg.ProjectID)
	fmt.Println()

	if resolved.Mode == config.ModeRemote {
		if promptYN(reader, "verify remote connection?", true) {
			fmt.Printf("connecting : %s ... ", resolved.ServerURL)
			resp, err := http.Get(resolved.ServerURL + "/api/healthz")
			if err != nil {
				fmt.Println("FAILED")
				fmt.Printf("  error: %v\n", err)
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					fmt.Println("OK")
				} else {
					fmt.Printf("FAILED (status %d)\n", resp.StatusCode)
				}
			}
			fmt.Println()
		}
	}

	choice := promptChoice(reader, "What would you like to do?", []string{
		"Review current setup (no changes)",
		"Reinitialise local database (deletes all data)",
		"Connect to a remote server",
		"Switch to local mode",
	})

	switch choice {
	case 0:
		fmt.Println()
		return runSetupPostInit(reader)
	case 1:
		dbPath, err := defaultDatabasePath()
		if err != nil {
			return err
		}
		if err := removeDBFiles(dbPath); err != nil {
			return err
		}
		return runSetupLocal(reader)
	case 2:
		return runSetupRemote(reader)
	case 3:
		return runSetupLocal(reader)
	}
	return nil
}

func runSetupNew(reader *bufio.Reader) error {
	home, _ := config.Home()
	fmt.Printf("config     : %s/config.json\n", home)
	fmt.Println()

	choice := promptChoice(reader, "How do you want to use ticket?", []string{
		"Local mode — standalone SQLite, no server needed",
		"Remote server — connect to a running ticket server",
	})
	fmt.Println()

	switch choice {
	case 0:
		return runSetupLocal(reader)
	case 1:
		return runSetupRemote(reader)
	}
	return nil
}

func runSetupLocal(reader *bufio.Reader) error {
	dbPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	dirName := strings.ToUpper(filepath.Base(cwd))
	if len(dirName) > 4 {
		dirName = dirName[:4]
	}

	projectPrefix := prompt(reader, "project prefix", dirName)
	projectPrefix = strings.ToUpper(strings.TrimSpace(projectPrefix))
	if projectPrefix == "" {
		projectPrefix = dirName
	}
	projectName := prompt(reader, "project name", filepath.Base(cwd))

	var gitRepo string
	if origin := detectGitOrigin(); origin != "" {
		fmt.Printf("detected   : git origin %s\n", origin)
		if promptYN(reader, "set as project git repository?", true) {
			gitRepo = origin
		}
	}

	password, err := generatePassword(24)
	if err != nil {
		return err
	}
	if err := store.Init(dbPath, "admin", password); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Location = "ticket.db"
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	project, err := svc.CreateProject(libticket.ProjectCreateRequest{
		Prefix:        projectPrefix,
		Title:         projectName,
		GitRepository: gitRepo,
	})
	if err != nil {
		return err
	}
	cfg.ProjectID = fmt.Sprintf("%d", project.ID)
	cfg.Username = "admin"
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  database : %s\n", dbPath)
	fmt.Printf("  project  : %s (%s)\n", project.Prefix, project.Title)
	fmt.Printf("  user     : admin\n")
	fmt.Printf("  password : %s\n", password)
	fmt.Println()

	return runSetupPostInit(reader)
}

func runSetupRemote(reader *bufio.Reader) error {
	// 1. Server URL
	defaultURL := "http://localhost:8080"
	serverURL := prompt(reader, "server URL", defaultURL)
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return errors.New("server URL is required")
	}

	// 2. Verify connectivity
	fmt.Printf("connecting : %s ... ", serverURL)
	resp, err := http.Get(serverURL + "/api/healthz") // #nosec G107 G704 -- URL is entered by the operator during setup, not constructed from untrusted input
	if err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("could not reach server: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Println("FAILED")
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	fmt.Println("OK")

	// 3. Save server URL to config now so resolveService picks it up
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Location = serverURL
	if err := config.Save(cfg); err != nil {
		return err
	}

	// 4. Authentication
	fmt.Println()

	// Pick up defaults from env vars
	defaultUsername := cfg.Username
	if defaultUsername == "" {
		defaultUsername = envValue("TICKET_USERNAME")
	}
	defaultPassword := envValue("TICKET_PASSWORD")

	if defaultUsername != "" {
		fmt.Printf("  TICKET_USERNAME : %s\n", defaultUsername)
	}
	if defaultPassword != "" {
		fmt.Printf("  TICKET_PASSWORD : ***\n")
	}

	hasAccount := promptYN(reader, "do you have an account on this server?", true)

	username, password, err := promptForCredentials(os.Stdin, os.Stdout, defaultUsername, defaultPassword)
	if err != nil {
		return err
	}

	cfg, _ = config.Load()
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if !hasAccount {
		if _, err := svc.Register(username, password); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		fmt.Printf("  registered user: %s\n", username)
	}
	_, token, err := svc.Login(username, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	cfg.Username = username
	cfg.Token = token

	// Save credentials
	if err := config.SaveCredentials(config.Credentials{Token: cfg.Token}); err != nil {
		return err
	}
	fmt.Printf("  user     : %s\n", cfg.Username)
	fmt.Println()

	// 5. Project selection
	// Reload service with token
	cfg.Location = serverURL
	if err := config.Save(cfg); err != nil {
		return err
	}
	cfg, _ = config.Load()
	svc, err = resolveService(cfg)
	if err != nil {
		return err
	}

	projects, err := svc.ListProjects()
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
			cfg.ProjectID = fmt.Sprintf("%d", match.ID)
			if err := config.Save(cfg); err != nil {
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
	cfg.ProjectID = fmt.Sprintf("%d", selected.ID)
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("  project  : %s (%s)\n", selected.Prefix, selected.Title)
	fmt.Println()
	return runSetupPostInit(reader)
}

func setupCreateRemoteProject(reader *bufio.Reader, svc libticket.Service, cfg config.Config) error {
	cwd, _ := os.Getwd()
	dirName := strings.ToUpper(filepath.Base(cwd))
	if len(dirName) > 4 {
		dirName = dirName[:4]
	}

	prefix := prompt(reader, "project prefix", dirName)
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		prefix = dirName
	}
	title := prompt(reader, "project name", filepath.Base(cwd))

	var gitRepo string
	if origin := detectGitOrigin(); origin != "" {
		fmt.Printf("detected   : git origin %s\n", origin)
		if promptYN(reader, "set as project git repository?", true) {
			gitRepo = origin
		}
	}

	project, err := svc.CreateProject(libticket.ProjectCreateRequest{
		Prefix:        prefix,
		Title:         title,
		GitRepository: gitRepo,
	})
	if err != nil {
		return err
	}
	cfg.ProjectID = fmt.Sprintf("%d", project.ID)
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("  project  : %s (%s)\n", project.Prefix, project.Title)
	fmt.Println()
	return runSetupPostInit(reader)
}

func runSetupPostInit(reader *bufio.Reader) error {
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
	credEntry := ".ticket/credentials.json" // #nosec G101 -- this is a path string, not a credential
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
						_, _ = f.WriteString("\n")
					}
					_, _ = f.WriteString(credEntry + "\n")
					_ = f.Close()
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
		if err := runInitCheckDefaults(reader, cfg); err != nil {
			fmt.Printf("warning: could not check defaults: %v\n", err)
		}
	}

	fmt.Println("setup complete. run `tk` to list tickets.")
	return nil
}

func runInitDB(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defaultDBPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}
	dbPath := fs.String("f", defaultDBPath, "SQLite database file")
	passwordFlag := fs.String("password", "", "bootstrap password")
	force := fs.Bool("force", false, "overwrite the database file if it exists")
	populate := fs.Bool("populate", false, "seed example projects, stories, tickets, users, and teams")

	if err := fs.Parse(args); err != nil {
		return err
	}

	password := strings.TrimSpace(*passwordFlag)
	generated := false
	if password == "" {
		var err error
		password, err = generatePassword(24)
		if err != nil {
			return err
		}
		generated = true
	}

	if *force {
		if err := removeDBFiles(*dbPath); err != nil {
			return err
		}
	}

	dbExists := false
	if _, statErr := os.Stat(*dbPath); statErr == nil {
		dbExists = true
	}

	if dbExists && !*force {
		// DB already exists — skip creation, just update the config to point at it.
		fmt.Printf("database already exists at %s (use -force to overwrite)\n", *dbPath)
	} else {
		if err := store.Init(*dbPath, "admin", password); err != nil {
			return err
		}
		if *populate {
			db, err := store.Open(*dbPath)
			if err != nil {
				return err
			}
			if err := seedExampleData(db); err != nil {
				_ = db.Close()
				return err
			}
			if err := db.Close(); err != nil {
				return err
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ProjectID = "1"
	cfg.Username = "admin"
	// If the db is in the .ticket dir, use a relative path; otherwise file:// URI.
	if home, hErr := config.Home(); hErr == nil && filepath.Dir(*dbPath) == home {
		cfg.Location = filepath.Base(*dbPath)
	} else {
		cfg.Location = "file://" + *dbPath
	}
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("initialized database at %s\n", *dbPath)
	if !dbExists || *force {
		fmt.Printf("admin user: admin\n")
		fmt.Printf("admin password: %s\n", password)
		fmt.Printf("default project: 1\n")
		if *populate {
			fmt.Println("example data: seeded")
		}
		if generated {
			fmt.Println("admin password was generated because -password was not provided")
		}
	}

	// Prompt to populate missing workflows and roles.
	reader := bufio.NewReader(os.Stdin)
	if err := runInitCheckDefaults(reader, cfg); err != nil {
		fmt.Printf("warning: could not check defaults: %v\n", err)
	}
	return nil
}

// runInitCheckDefaults checks whether the current project has a workflow with
// stages, and whether any roles exist. If not, it prompts the user to create
// sensible defaults.
func runInitCheckDefaults(reader *bufio.Reader, cfg config.Config) error {
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	// ── Project workflow ──────────────────────────────────────────────────────
	project, err := svc.GetProject(cfg.ProjectID)
	if err != nil {
		return err
	}

	var wfID int64
	if project.WorkflowID == nil {
		// No workflow assigned to the project.
		fmt.Println()
		if promptYN(reader, "project has no workflow — create and assign a default workflow (design→develop→test→done)?", true) {
			wf, wfErr := svc.CreateWorkflow(libticket.WorkflowRequest{
				Name:        "default",
				Description: "Standard engineering lifecycle",
			})
			if wfErr != nil {
				fmt.Printf("  warning: could not create workflow: %v\n", wfErr)
			} else {
				wfID = wf.ID
				if err := addDefaultWorkflowStages(svc, wfID); err != nil {
					fmt.Printf("  warning: could not add stages: %v\n", err)
				}
				fmt.Printf("  created workflow %q (id %d) with stages: design, develop, test, done\n", wf.Name, wf.ID)
				projectID, parseErr := strconv.ParseInt(cfg.ProjectID, 10, 64)
				if parseErr != nil {
					fmt.Printf("  warning: could not parse project id: %v\n", parseErr)
				} else if _, pErr := svc.UpdateProject(projectID, libticket.ProjectUpdateRequest{WorkflowID: &wfID}); pErr != nil {
					fmt.Printf("  warning: could not assign workflow: %v\n", pErr)
				} else {
					fmt.Printf("  workflow assigned to project %s\n", cfg.ProjectID)
				}
			}
		}
	} else {
		wfID = *project.WorkflowID

		// ── Workflow stages ───────────────────────────────────────────────────
		wf, wfErr := svc.GetWorkflow(wfID)
		if wfErr == nil && len(wf.Stages) == 0 {
			fmt.Println()
			if promptYN(reader, fmt.Sprintf("workflow %q has no stages — add default stages (design→develop→test→done)?", wf.Name), true) {
				if err := addDefaultWorkflowStages(svc, wfID); err != nil {
					fmt.Printf("  warning: could not add stages: %v\n", err)
				} else {
					fmt.Println("  added stages: design, develop, test, done")
				}
			}
		} else if wfErr == nil {
			fmt.Printf("workflow   : %q (%d stages)\n", wf.Name, len(wf.Stages))
		}
	}

	// ── Roles ────────────────────────────────────────────────────────────────
	roles, err := svc.ListRoles()
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		fmt.Println()
		if promptYN(reader, "no roles found — create default roles (engineer, tech lead, QA engineer)?", true) {
			defaults := []libticket.RoleRequest{
				{Title: "Engineer", Motivation: "Build reliable, well-tested software", Goals: "Ship features, fix bugs, write tests"},
				{Title: "Tech Lead", Motivation: "Guide the technical direction of the team", Goals: "Architecture decisions, code quality, mentoring"},
				{Title: "QA Engineer", Motivation: "Ensure quality across the product", Goals: "Test coverage, bug detection, release confidence"},
			}
			for _, r := range defaults {
				if _, rErr := svc.CreateRole(r); rErr != nil {
					fmt.Printf("  warning: could not create role %q: %v\n", r.Title, rErr)
				} else {
					fmt.Printf("  created role: %s\n", r.Title)
				}
			}
		}
	} else {
		fmt.Printf("roles      : %d found\n", len(roles))
	}

	return nil
}

// addDefaultWorkflowStages adds the standard engineering lifecycle stages to a workflow.
func addDefaultWorkflowStages(svc libticket.Service, workflowID int64) error {
	stages := []struct {
		name string
		desc string
	}{
		{"design", "Discovery and specification"},
		{"develop", "Implementation"},
		{"test", "Verification and QA"},
		{"done", "Complete and shipped"},
	}
	for i, s := range stages {
		if _, err := svc.AddWorkflowStage(workflowID, libticket.WorkflowStageRequest{
			StageName:   s.name,
			Description: s.desc,
			SortOrder:   i,
		}); err != nil {
			return fmt.Errorf("stage %q: %w", s.name, err)
		}
	}
	return nil
}

func runExportSnapshot(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outputPath := fs.String("o", "ticket-snapshot.json", "snapshot output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: ticket export [-o <snapshot-file>]")
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

func runImportSnapshot(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	inputPath := fs.String("i", "", "snapshot input file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("usage: ticket import -i <snapshot-file>")
	}
	path := strings.TrimSpace(*inputPath)
	if path == "" {
		return errors.New("usage: ticket import -i <snapshot-file>")
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is a CLI flag provided by the operator, not untrusted input
	if err != nil {
		return err
	}
	var snapshot store.Snapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&snapshot); err != nil {
		return err
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

	defaultDBPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}
	dbPath := fs.String("f", defaultDBPath, "SQLite database file")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	port := fs.Int("p", 0, "HTTP listen port (shorthand for -addr :<port>)")
	verbose := fs.Bool("v", false, "print verbose request/response logs to stdout")
	staticPath := fs.String("path", "", "serve static files from this filesystem path instead of embedded assets")

	if err := fs.Parse(args); err != nil {
		return err
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

	db, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	srv, err := server.New(listenAddr, db, strings.TrimSpace(embeddedVersion), *verbose, os.Stdout, *staticPath)
	if err != nil {
		return err
	}

	fmt.Print(renderBanner())
	fmt.Printf("VERSION    %s\n", strings.TrimSpace(embeddedVersion))
	fmt.Printf("TICKETDB   %s\n\n", *dbPath)
	fmt.Printf("serving ticket on http://localhost%s\n", listenAddr)
	return srv.ListenAndServe()
}
