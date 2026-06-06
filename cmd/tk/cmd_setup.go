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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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
		ProjectID: "PUB",
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
		fmt.Printf("repo      : %s\n", projectRoot)
	}

	// Apply project settings from flags.
	if *prefixFlag != "" || *nameFlag != "" || *gitFlag != "" {
		svc := libticket.NewLocal(cfg)
		project, getErr := svc.GetProject(context.Background(), cfg.ProjectID)
		if getErr != nil {
			fmt.Printf("warning: could not resolve project for initdb updates: %v\n", getErr)
			return nil
		}
		update := libticket.ProjectUpdateRequest{}
		if *nameFlag != "" {
			update.Title = *nameFlag
		}
		if *gitFlag != "" {
			update.GitRepository = *gitFlag
		}
		if _, err := svc.UpdateProject(context.Background(), project.ID, update); err != nil {
			fmt.Printf("warning: could not update project: %v\n", err)
		}
		if *prefixFlag != "" {
			prefix := strings.ToUpper(strings.TrimSpace(*prefixFlag))
			if _, err := svc.RenameProjectPrefix(context.Background(), project.ID, prefix); err != nil {
				fmt.Printf("warning: could not set prefix: %v\n", err)
			}
		}
	}
	return nil
}

// runInitCheckDefaults checks whether the current project has a workflow with
// stages, and whether any roles exist. If not, it seeds them from the
// built-in role and Workflow templates in internal/static/.
func runInitCheckDefaults(reader *bufio.Reader, cfg config.Config, workflowName string) error {
	svc := libticket.NewLocal(cfg)

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
		return errors.New("usage: tk admin export [-o <snapshot-file>]")
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
		return errors.New("usage: tk admin import -i <snapshot-file>")
	}
	path := strings.TrimSpace(*inputPath)
	if path == "" {
		return errors.New("usage: tk admin import -i <snapshot-file>")
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
	siteName := fs.String("site", web.DefaultSite, "embedded site bundle to serve")

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
		selectedSite = web.DefaultSite
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
