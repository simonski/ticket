package main

import (
	"bufio"
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	_ = args // no flags for interactive setup
	reader := bufio.NewReader(os.Stdin)

	dbPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}

	resolved, _ := config.ResolveURL()
	if resolved.Mode == config.ModeRemote {
		fmt.Println("tk init — remote server")
		fmt.Printf("server     : %s\n", resolved.ServerURL)
		cfg, _ := config.Load()
		if cfg.Username == "" {
			fmt.Println("warning    : no username configured — run `tk login` or `tk register` first")
		} else {
			fmt.Printf("user       : %s\n", cfg.Username)
		}
	} else {
		fmt.Println("tk init — singleplayer local database")
	}
	fmt.Println()

	// Check existing DB
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	if dbExists {
		fmt.Printf("database   : %s (exists)\n", dbPath)
	} else {
		fmt.Printf("database   : %s (not found)\n", dbPath)
	}

	// Derive defaults from cwd
	cwd, _ := os.Getwd()
	dirName := strings.ToUpper(filepath.Base(cwd))
	if len(dirName) > 4 {
		dirName = dirName[:4]
	}

	fmt.Println()

	reinit := !dbExists
	if dbExists {
		reinit = promptYN(reader, "database already exists — reinitialise? this will delete all data", false)
		if reinit {
			if err := removeDBFiles(dbPath); err != nil {
				return err
			}
		}
	}

	fmt.Println()
	if reinit {
		projectPrefix := prompt(reader, "project prefix", dirName)
		projectPrefix = strings.ToUpper(strings.TrimSpace(projectPrefix))
		if projectPrefix == "" {
			projectPrefix = dirName
		}
		projectName := prompt(reader, "project name", filepath.Base(cwd))

		// Detect git origin on first run
		var gitRepo string
		gitOrigin, gitErr := exec.Command("git", "remote", "get-url", "origin").Output()
		if gitErr == nil {
			origin := strings.TrimSpace(string(gitOrigin))
			if origin != "" {
				fmt.Printf("detected   : git origin %s\n", origin)
				if promptYN(reader, "set as project git repository?", true) {
					gitRepo = origin
				}
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
		cfg.CurrentProject = fmt.Sprintf("%d", project.ID)
		cfg.Username = "admin"
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("  database : %s\n", dbPath)
		fmt.Printf("  project  : %s (%s)\n", project.Prefix, project.Title)
		fmt.Printf("  user     : admin\n")
		fmt.Printf("  password : %s\n", password)
	} else {
		// Existing DB — show current state without touching any config
		cfg, _ := config.Load()
		fmt.Printf("  database : %s (unchanged)\n", dbPath)
		fmt.Printf("  project  : %s\n", cfg.CurrentProject)
		fmt.Printf("  user     : %s\n", cfg.Username)
	}
	fmt.Println()

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

		// Check both locations for an existing skill
		var existingPath string
		var existingContent []byte
		for _, p := range []string{localSkillPath, globalSkillPath} {
			if data, readErr := os.ReadFile(p); readErr == nil {
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
					if err := os.WriteFile(existingPath, []byte(tkSkillContent), 0o644); err != nil {
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
				if err := os.MkdirAll(skillDir, 0o755); err != nil {
					fmt.Printf("  warning: could not create skill dir: %v\n", err)
				} else {
					skillPath := filepath.Join(skillDir, "SKILL.md")
					if err := os.WriteFile(skillPath, []byte(tkSkillContent), 0o644); err != nil {
						fmt.Printf("  warning: could not write skill: %v\n", err)
					} else {
						fmt.Printf("  installed: %s\n", skillPath)
					}
				}
			}
		}
	}

	// Check for CLAUDE.md and AGENTS.md
	cwd2, _ := os.Getwd()
	claudeMD := filepath.Join(cwd2, "CLAUDE.md")
	agentsMD := filepath.Join(cwd2, "AGENTS.md")

	if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
		fmt.Println()
		if promptYN(reader, "CLAUDE.md not found — create it?", true) {
			content := "Read AGENTS.md\n"
			if writeErr := os.WriteFile(claudeMD, []byte(content), 0o644); writeErr != nil {
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
			if writeErr := os.WriteFile(agentsMD, []byte(embeddedAgents), 0o644); writeErr != nil {
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
	credEntry := ".ticket/credentials.json"
	if data, readErr := os.ReadFile(gitignorePath); readErr == nil {
		if !strings.Contains(string(data), credEntry) {
			fmt.Println()
			fmt.Printf("warning    : %s is not in .gitignore\n", credEntry)
			if promptYN(reader, "add .ticket/credentials.json to .gitignore?", true) {
				f, appendErr := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
				if appendErr != nil {
					fmt.Printf("  warning: could not open .gitignore: %v\n", appendErr)
				} else {
					// Ensure we start on a new line
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

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.CurrentProject = "1"
	cfg.Username = "admin"
	if r, rErr := config.ResolveURL(); rErr == nil {
		cfg.ServerURL = r.ServerURL
	}
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("initialized database at %s\n", *dbPath)
	fmt.Printf("admin user: admin\n")
	fmt.Printf("admin password: %s\n", password)
	fmt.Printf("default project: 1\n")
	if *populate {
		fmt.Println("example data: seeded")
	}
	if generated {
		fmt.Println("admin password was generated because -password was not provided")
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
	snapshot, err := store.ExportSnapshot(db)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
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
	raw, err := os.ReadFile(path)
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
	if err := store.ImportSnapshot(db, snapshot); err != nil {
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
	adminUser, err := store.GetUserByUsername(db, "admin")
	if err != nil {
		return fmt.Errorf("seed: admin user not found: %w", err)
	}
	seedCreatedBy := adminUser.ID

	for _, projectSeed := range projects {
		project, err := store.CreateProjectWithParams(db, store.ProjectCreateParams{
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
			story, err := store.CreateStory(db, project.ID, storySeed.title, storySeed.description, seedCreatedBy)
			if err != nil {
				return err
			}
			epic, err := store.CreateTicket(db, store.TicketCreateParams{
				ProjectID: project.ID,
				Type:      "epic",
				Title:     storySeed.epicTitle,
				CreatedBy: seedCreatedBy,
			})
			if err != nil {
				return err
			}
			if err := store.LinkStoryToTicket(db, story.ID, epic.ID); err != nil {
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
				childTicket, err := store.CreateTicket(db, store.TicketCreateParams{
					ProjectID: project.ID,
					ParentID:  &parentID,
					Type:      child.ticketType,
					Title:     child.title,
					CreatedBy: seedCreatedBy,
				})
				if err != nil {
					return err
				}
				if err := store.LinkStoryToTicket(db, story.ID, childTicket.ID); err != nil {
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
		team, err := store.CreateTeam(db, item.team, nil)
		if err != nil {
			return err
		}
		teamsByName[item.team] = team.ID
	}
	for _, item := range seedUsers {
		user, err := store.CreateUser(db, item.username, "password", "user")
		if err != nil {
			return err
		}
		teamID := teamsByName[item.team]
		if _, err := store.AddTeamMember(db, teamID, user.ID, item.role, item.title); err != nil {
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
