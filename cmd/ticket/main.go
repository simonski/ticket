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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/internal/tui"
	"github.com/simonski/ticket/libticket"
)

var (
	loginPromptInput  io.Reader = os.Stdin
	loginPromptOutput io.Writer = os.Stdout
	outputJSON        bool
	noColorOutput     bool
	runAgentCommand   = defaultRunTicketAgentCommand
	selectBannerWord  = randomBannerWord
	fetchRepoVersion  = defaultFetchRepoVersion
)

const repoVersionURL = "https://raw.githubusercontent.com/simonski/ticket/refs/heads/main/cmd/ticket/VERSION"

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

//go:embed VERSION
var embeddedVersion string

//go:embed TICKETS.md
var embeddedAgents string

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
	var guiTheme string
	trimmedArgs, guiTheme = extractGUIFlag(trimmedArgs)
	// -f /path/to/dir (or db file) sets TICKET_HOME to the directory
	if dbOverride != "" {
		absPath, pathErr := filepath.Abs(dbOverride)
		if pathErr != nil {
			return pathErr
		}
		// If user passed a .db file path, use its directory as TICKET_HOME
		dir := absPath
		if strings.HasSuffix(absPath, ".db") || strings.HasSuffix(absPath, ".sqlite") {
			dir = filepath.Dir(absPath)
		}
		if err := os.Setenv("TICKET_HOME", dir); err != nil {
			return err
		}
	}
	if urlOverride != "" {
		if err := os.Setenv("TICKET_URL", urlOverride); err != nil {
			return err
		}
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	// -g launches the TUI (may appear before or after other args)
	if guiTheme != "" || (len(trimmedArgs) > 0 && (trimmedArgs[0] == "-g" || trimmedArgs[0] == "gui")) {
		if len(trimmedArgs) > 0 && (trimmedArgs[0] == "-g" || trimmedArgs[0] == "gui") {
			trimmedArgs = trimmedArgs[1:]
		}
		return runGUI(guiTheme)
	}

	if len(trimmedArgs) == 0 {
		fmt.Print(renderRootUsage())
		return nil
	}

	switch trimmedArgs[0] {
	case "help", "-h", "--help":
		return runHelp(trimmedArgs[1:])
	case "summary":
		return runSummary(trimmedArgs[1:])
	case "onboard":
		return runOnboard(trimmedArgs[1:])
	case "init", "setup":
		return runSetup(trimmedArgs[1:])
	case "initdb":
		return runInitDB(trimmedArgs[1:])
	case "server":
		return runServer(trimmedArgs[1:])
	case "export":
		if resolved.Mode != config.ModeLocal {
			return errors.New("ticket export requires local mode (no TICKET_URL set)")
		}
		return runExportSnapshot(trimmedArgs[1:])
	case "import":
		if resolved.Mode != config.ModeLocal {
			return errors.New("ticket import requires local mode (no TICKET_URL set)")
		}
		return runImportSnapshot(trimmedArgs[1:])
	case "version":
		return runVersion(trimmedArgs[1:])
	case "upgrade":
		return runUpgrade(trimmedArgs[1:])
	case "register":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket register requires TICKET_URL=http(s)://...")
		}
		return runRegister(trimmedArgs[1:])
	case "login":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket login requires TICKET_URL=http(s)://...")
		}
		return runLogin(trimmedArgs[1:])
	case "logout":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket logout requires TICKET_URL=http(s)://...")
		}
		return runLogout(trimmedArgs[1:])
	case "status":
		return runStatus(trimmedArgs[1:])
	case "whoami":
		return runWhoami(trimmedArgs[1:])
	case "count":
		return runCount(trimmedArgs[1:])
	case "ticket":
		return runTicketNS(trimmedArgs[1:])
	case "agent":
		return runAgent(trimmedArgs[1:])
	case "user":
		return runUser(trimmedArgs[1:])
	case "project":
		return runProject(trimmedArgs[1:])
	case "team":
		return runTeam(trimmedArgs[1:])
	case "role":
		return runRole(trimmedArgs[1:])
	case "story":
		return runStory(trimmedArgs[1:])
	case "workflow":
		return runWorkflow(trimmedArgs[1:])
	case "board":
		return runBoard(trimmedArgs[1:])
	case "label":
		return runLabel(trimmedArgs[1:])
	case "time":
		return runTime(trimmedArgs[1:])
	case "ls":
		return runList(trimmedArgs[1:])
	case "list":
		return runList(trimmedArgs[1:])
	case "orphans":
		return runOrphans(trimmedArgs[1:])
	case "get", "show":
		return runGet(trimmedArgs[1:])
	case "edit":
		return runEdit(trimmedArgs[1:])
	case "search":
		return runSearch(trimmedArgs[1:])
	case "update":
		return runUpdate(trimmedArgs[1:])
	case "set-parent", "attach":
		return runSetParent(trimmedArgs[1:], trimmedArgs[0])
	case "unset-parent", "detach":
		return runUnsetParent(trimmedArgs[1:], trimmedArgs[0])
	case "stage":
		return runTicketState(trimmedArgs[1:])
	case "idle":
		return runTicketStateAlias(trimmedArgs[1:], store.StateIdle, trimmedArgs[0])
	case "state":
		return runTicketState(trimmedArgs[1:])
	case "active":
		return runTicketStateAlias(trimmedArgs[1:], store.StateActive, trimmedArgs[0])
	case "complete":
		return runTicketStateAlias(trimmedArgs[1:], store.StateSuccess, trimmedArgs[0])
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
	case "dep", "dependency":
		return runDependency(trimmedArgs[1:])
	case "request":
		return runRequest(trimmedArgs[1:])
	case "request-dryrun":
		return runRequestDryRun(trimmedArgs[1:])
	case "history":
		return runHistory(trimmedArgs[1:])
	case "health", "heatlh":
		return runHealth(trimmedArgs[1:])
	case "comment":
		return runComment(trimmedArgs[1:])
	case "clone", "cp":
		return runClone(trimmedArgs[1:])
	case "close":
		return runSetTicketClosed(trimmedArgs[1:], true)
	case "open":
		return runSetTicketClosed(trimmedArgs[1:], false)
	case "archive":
		return runSetTicketArchived(trimmedArgs[1:], true)
	case "unarchive":
		return runSetTicketArchived(trimmedArgs[1:], false)
	case "ready":
		return runSetTicketReady(trimmedArgs[1:], true)
	case "notready":
		return runSetTicketReady(trimmedArgs[1:], false)
	case "rm", "delete":
		return runDeleteTicket(trimmedArgs[1:])
	case "req":
		return runReq(trimmedArgs[1:])
	case "idea":
		return runReqAdd(trimmedArgs[1:])
	case "ideas":
		return runReqList(trimmedArgs[1:])
	case "curate":
		return runCurate(trimmedArgs[1:])
	case "review":
		return runReview(trimmedArgs[1:])
	case "accept":
		return runRequirementStatus("accepted", trimmedArgs[1:])
	case "reject":
		return runRequirementStatus("rejected", trimmedArgs[1:])
	case "revise":
		return runRevise(trimmedArgs[1:])
	case "decision":
		return runDecision(trimmedArgs[1:])
	case "conversation":
		return runConversation(trimmedArgs[1:])
	case "add", "create", "new":
		return runTicketCreate(trimmedArgs[1:])
	case "note":
		return runTypedTicketCreate("note", trimmedArgs[1:])
	case "question":
		return runTypedTicketCreate("question", trimmedArgs[1:])
	case "bug":
		return runTypedTicketCreate("bug", trimmedArgs[1:])
	case "epic":
		return runEpic(trimmedArgs[1:])
	case "config":
		return runConfig(trimmedArgs[1:])
	default:
		return fmt.Errorf("no such command %q", trimmedArgs[0])
	}
}

func runHelp(args []string) error {
	if len(args) == 0 {
		fmt.Print(renderRootUsage())
		printTicketEnvironment()
		return nil
	}
	if !hasCommandHelp(args[0]) {
		return fmt.Errorf("no such command %q", args[0])
	}
	fmt.Print(renderBanner())
	fmt.Print(renderCommandHelp(args[0]))
	printTicketEnvironment()
	return nil
}

func runSummary(_ []string) error {
	cfg, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	_ = cfg

	// All tickets for this project (non-archived), then keep only open ones
	all, _ := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, false)
	var allTickets []store.Ticket
	var activeTickets []store.Ticket
	for _, t := range all {
		if t.Open {
			allTickets = append(allTickets, t)
			if t.State == store.StateActive {
				activeTickets = append(activeTickets, t)
			}
		}
	}

	// Count open tickets by type
	typeCounts := map[string]int{}
	for _, t := range allTickets {
		typeCounts[t.Type]++
	}

	// Last 5 recently-updated tickets (sort by UpdatedAt desc)
	recent := make([]store.Ticket, len(allTickets))
	copy(recent, allTickets)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].UpdatedAt > recent[j].UpdatedAt
	})
	if len(recent) > 5 {
		recent = recent[:5]
	}

	// Environment
	ticketHome, _ := config.Home()
	resolved, _ := config.ResolveURL()
	cfgPath, _ := config.Path()
	envHome := envValue("TICKET_HOME")

	if outputJSON {
		return printJSON(map[string]any{
			"project":     project,
			"type_counts": typeCounts,
			"active":      activeTickets,
			"recent":      recent,
			"db_path":     resolved.DBPath,
			"config_file": cfgPath,
		})
	}

	// Build box lines
	var lines []statusLine

	// Project header
	projectLabel := project.Prefix + " — " + project.Title
	lines = append(lines, statusLine{key: "project", value: projectLabel})
	if strings.TrimSpace(project.Description) != "" {
		lines = append(lines, statusLine{key: "description", value: strings.TrimSpace(project.Description)})
	}

	// Ticket counts
	lines = append(lines, statusLine{})
	total := len(allTickets)
	typeOrder := []string{"task", "epic", "bug", "story", "requirement", "decision", "question", "note"}
	var typeBreakdown []string
	for _, t := range typeOrder {
		if n := typeCounts[t]; n > 0 {
			label := t + "s"
			if t == "story" {
				label = "stories"
			}
			typeBreakdown = append(typeBreakdown, fmt.Sprintf("%d %s", n, label))
		}
	}
	ticketVal := fmt.Sprintf("%d open", total)
	if len(typeBreakdown) > 0 {
		ticketVal += "  (" + strings.Join(typeBreakdown, ", ") + ")"
	}
	lines = append(lines, statusLine{key: "open tickets", value: ticketVal})

	// Active tickets (state=active)
	if len(activeTickets) > 0 {
		lines = append(lines, statusLine{})
		lines = append(lines, statusLine{key: "active", value: fmt.Sprintf("%d in progress", len(activeTickets))})
		for _, t := range activeTickets {
			assignee := t.Assignee
			if assignee == "" {
				assignee = "unassigned"
			}
			val := fmt.Sprintf("%-*s  %s  %s", 30, t.Title, t.Stage, assignee)
			lines = append(lines, statusLine{key: "  " + t.Key, value: val, color: "\x1b[32m"})
		}
	}

	// Recent activity
	if len(recent) > 0 {
		lines = append(lines, statusLine{})
		lines = append(lines, statusLine{key: "recently active", value: ""})
		now := time.Now().UTC()
		for _, t := range recent {
			sym := formatTicketStatusSymbol(t.Status, true)
			ago := timeAgo(t.UpdatedAt, now)
			val := fmt.Sprintf("%s  %-*s  %s  %s", sym, 30, t.Title, t.Status, ago)
			lines = append(lines, statusLine{key: "  " + t.Key, value: val})
		}
	}

	// System counts
	lines = append(lines, statusLine{})
	projects, _ := svc.ListProjects()
	users, _ := svc.ListUsers()
	agents, _ := svc.ListAgents()
	lines = append(lines, statusLine{key: "projects", value: fmt.Sprintf("%d", len(projects))})
	lines = append(lines, statusLine{key: "users", value: fmt.Sprintf("%d", len(users))})
	lines = append(lines, statusLine{key: "agents", value: fmt.Sprintf("%d", len(agents))})

	// Environment
	lines = append(lines, statusLine{})
	lines = append(lines, statusLine{key: "database", value: resolved.DBPath})
	lines = append(lines, statusLine{key: "config", value: cfgPath})
	if envHome != "" {
		lines = append(lines, statusLine{key: "TICKET_HOME", value: envHome})
	} else {
		lines = append(lines, statusLine{key: "TICKET_HOME", value: ticketHome + "  (auto-discovered)"})
	}

	printStatusBox(lines)
	return nil
}

// timeAgo returns a human-friendly relative time string.
func timeAgo(ts string, now time.Time) string {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	var t time.Time
	for _, l := range layouts {
		if p, err := time.Parse(l, ts); err == nil {
			t = p.UTC()
			break
		}
	}
	if t.IsZero() {
		return ts
	}
	d := now.Sub(t)
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

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

	fmt.Println("tk setup — singleplayer local database")
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
			Prefix: projectPrefix,
			Title:  projectName,
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
		if promptYN(reader, "install tk skill for Claude Code?", true) {
			cwd, _ := os.Getwd()
			localSkillDir := filepath.Join(cwd, ".claude", "skills", "tk")
			globalSkillDir := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "tk")
			fmt.Printf("  [1] local   %s\n", localSkillDir+"/SKILL.md")
			fmt.Printf("  [2] global  %s\n", globalSkillDir+"/SKILL.md")
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

	for _, projectSeed := range projects {
		project, err := store.CreateProjectWithParams(db, store.ProjectCreateParams{
			Prefix:      projectSeed.prefix,
			Title:       projectSeed.title,
			Description: projectSeed.description,
			CreatedBy:   1,
			Visibility:  store.ProjectVisibilityPublic,
		})
		if err != nil {
			return err
		}
		for _, storySeed := range projectSeed.stories {
			story, err := store.CreateStory(db, project.ID, storySeed.title, storySeed.description, 1)
			if err != nil {
				return err
			}
			epic, err := store.CreateTicket(db, store.TicketCreateParams{
				ProjectID: project.ID,
				Type:      "epic",
				Title:     storySeed.epicTitle,
				CreatedBy: 1,
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
					CreatedBy: 1,
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

func runVersion(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket version")
	}
	fmt.Println(strings.TrimSpace(embeddedVersion))
	return nil
}

func runUpgrade(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket upgrade")
	}

	localVersion := strings.TrimSpace(embeddedVersion)
	repoVersion, err := fetchRepoVersion()
	if err != nil {
		return errors.New("Unable to check for updates right now. Check your network connection and try again.")
	}

	switch compareVersions(localVersion, repoVersion) {
	case 0:
		fmt.Printf("You are on the latest version (%s)\n", localVersion)
	case -1:
		fmt.Println("A newer version of ticket is available, upgrade using `go install github.com/simonski/ticket@latest`")
	default:
		fmt.Println("Your local copy is newer than the repo")
	}
	return nil
}

func defaultFetchRepoVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoVersionURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version lookup failed with status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", errors.New("empty repo version")
	}
	return version, nil
}

func compareVersions(left, right string) int {
	leftParts := parseVersionParts(left)
	rightParts := parseVersionParts(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		var leftPart, rightPart int
		if i < len(leftParts) {
			leftPart = leftParts[i]
		}
		if i < len(rightParts) {
			rightPart = rightParts[i]
		}
		switch {
		case leftPart < rightPart:
			return -1
		case leftPart > rightPart:
			return 1
		}
	}
	return 0
}

func parseVersionParts(raw string) []int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(raw), "v"))
	if trimmed == "" {
		return []int{0}
	}
	parts := strings.Split(trimmed, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil {
			values = append(values, 0)
			continue
		}
		values = append(values, n)
	}
	return values
}

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
	if err != nil {
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
	user, err := svc.Register(username, password)
	if err != nil {
		return err
	}
	cfg.Username = user.Username
	if err := config.Save(cfg); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(user)
	}
	fmt.Printf("registered user %s\n", user.Username)
	return nil
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	usernameFlag := fs.String("username", "", "username")
	passwordFlag := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
	if err != nil {
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

	if cfg.Token != "" {
		status, err := svc.Status()
		if err == nil && status.Authenticated && status.User != nil {
			cfg.Username = status.User.Username
			if r, rErr := config.ResolveURL(); rErr == nil {
				cfg.ServerURL = r.ServerURL
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(status)
			}
			fmt.Printf("logged in as %s\n", status.User.Username)
			return nil
		}
	}

	username = resolveLoginUsername(cfg.Username, *usernameFlag)
	password = resolveLoginPassword(*passwordFlag)

	if username != "" && password != "" {
		user, token, err := svc.Login(username, password)
		if err == nil {
			return finishLogin(cfg, user, token)
		}
		if err.Error() != "invalid credentials" {
			return err
		}
		fmt.Println("invalid credentials")
	}

	username, password, err = promptForCredentials(loginPromptInput, loginPromptOutput, username, password)
	if err != nil {
		return err
	}
	user, token, err := svc.Login(username, password)
	if err != nil {
		return err
	}
	return finishLogin(cfg, user, token)
}

func finishLogin(cfg config.Config, user store.User, token string) error {
	cfg.Username = user.Username
	if r, rErr := config.ResolveURL(); rErr == nil {
		cfg.ServerURL = r.ServerURL
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	if err := config.SaveCredentials(config.Credentials{Token: token}); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"token": token, "user": user})
	}
	fmt.Printf("logged in as %s\n", user.Username)
	return nil
}

func runLogout(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket logout")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if err := svc.Logout(); err != nil {
		if clearErr := config.ClearCredentials(); clearErr != nil {
			return clearErr
		}
		cfg.Token = ""
		return err
	}
	if err := config.ClearCredentials(); err != nil {
		return err
	}
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]string{"status": "logged_out"})
	}
	return nil
}

func runStatus(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket status")
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch resolved.Mode {
	case config.ModeRemote:
		return runRemoteStatus(cfg)
	case config.ModeLocal:
		return runLocalStatus()
	default:
		return fmt.Errorf("unsupported mode %q", resolved.Mode)
	}
}

func runCount(args []string) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectID := fs.Int64("project_id", 0, "limit counts to a project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: ticket count [-project_id <id>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	var projectFilter *int64
	if *projectID != 0 {
		projectFilter = projectID
		if _, err := svc.GetProject(fmt.Sprintf("%d", *projectID)); err != nil {
			return err
		}
	}
	summary, err := svc.Count(projectFilter)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(summary)
	}
	printCountSummary(summary, projectFilter != nil)
	return nil
}

func runWhoami(args []string) error {
	_ = args
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	resolved, err := config.ResolveURL()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	// User info
	username := cfg.Username
	if username == "" {
		username = "admin"
	}
	users, _ := svc.ListUsers()
	var currentUser *store.User
	for _, u := range users {
		if u.Username == username {
			currentUser = &u
			break
		}
	}

	fmt.Println("USER")
	if currentUser != nil {
		fmt.Printf("  username : %s\n", currentUser.Username)
		fmt.Printf("  role     : %s\n", currentUser.Role)
		fmt.Printf("  user_id  : %d\n", currentUser.ID)
	} else {
		fmt.Printf("  username : %s\n", username)
	}

	// Connection info
	fmt.Println()
	fmt.Println("CONNECTION")
	fmt.Printf("  mode     : %s\n", resolved.Mode)
	if resolved.Mode == config.ModeRemote {
		fmt.Printf("  server   : %s\n", resolved.ServerURL)
	} else {
		fmt.Printf("  database : %s\n", resolved.DBPath)
	}

	// Projects with user role
	fmt.Println()
	fmt.Println("PROJECTS")
	projects, err := svc.ListProjects()
	if err != nil {
		fmt.Println("  (unable to list projects)")
		return nil
	}
	if len(projects) == 0 {
		fmt.Println("  (none)")
		return nil
	}
	for _, p := range projects {
		marker := "  "
		if p.Prefix == cfg.CurrentProject || fmt.Sprintf("%d", p.ID) == cfg.CurrentProject {
			marker = "* "
		}
		role := ""
		if currentUser != nil {
			members, _ := svc.ListProjectMembers(p.ID)
			for _, m := range members {
				if m.UserID == currentUser.ID {
					role = m.Role
					break
				}
			}
		}
		if role != "" {
			fmt.Printf("  %s%-6s  %-20s  (%s)\n", marker, p.Prefix, p.Title, role)
		} else {
			fmt.Printf("  %s%-6s  %s\n", marker, p.Prefix, p.Title)
		}
	}

	return nil
}

func runUser(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(userUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("user create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		usernameFlag := fs.String("username", "", "username")
		passwordFlag := fs.String("password", "", "password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		username, password, err := resolveCredentials(*usernameFlag, *passwordFlag, true)
		if err != nil {
			return err
		}
		user, err := svc.CreateUser(username, password)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(user)
		}
		fmt.Printf("created user %s\n", user.Username)
		return nil
	case "rm", "delete", "del":
		fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *username == "" {
			return errors.New("user rm/delete/del requires -username")
		}
		if err := svc.DeleteUser(*username); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]string{"status": "deleted", "username": *username})
		}
		fmt.Printf("deleted user %s\n", *username)
		return nil
	case "enable", "disable":
		fs := flag.NewFlagSet("user "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *username == "" {
			return errors.New("user enable/disable requires -username")
		}
		if err := svc.SetUserEnabled(*username, args[0] == "enable"); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]string{"status": args[0] + "d", "username": *username})
		}
		fmt.Printf("%sd user %s\n", args[0], *username)
		return nil
	case "list", "ls":
		users, err := svc.ListUsers()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(users)
		}
		printUserTable(users)
		return nil
	case "reset-password":
		fs := flag.NewFlagSet("user reset-password", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		newPassword := fs.String("password", "", "new password (generated if omitted)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*username) == "" {
			return errors.New("usage: ticket user reset-password -username <name> [-password <new-password>]")
		}
		pw := strings.TrimSpace(*newPassword)
		if pw == "" {
			generated, err := generatePassword(24)
			if err != nil {
				return err
			}
			pw = generated
		}
		user, err := svc.ResetUserPassword(strings.TrimSpace(*username), pw)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"user_id": user.ID, "username": user.Username, "password": pw})
		}
		fmt.Printf("username : %s\n", user.Username)
		fmt.Printf("password : %s\n", pw)
		fmt.Println("all sessions invalidated")
		return nil
	default:
		return fmt.Errorf("unknown user command %q; see: ticket user help", args[0])
	}
}

func runAgent(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(agentUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("agent create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		description := fs.String("description", "", "agent description")
		password := fs.String("password", "", "agent password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agent, generatedPassword, err := svc.CreateAgent(libticket.AgentCreateRequest{
			Description: strings.TrimSpace(*description),
			Password:    strings.TrimSpace(*password),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"agent": agent, "password": generatedPassword})
		}
		fmt.Printf("agent_id: %s\n", agent.UUID)
		fmt.Printf("password: %s\n", generatedPassword)
		return nil
	case "ls", "list":
		agents, err := svc.ListAgents()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agents)
		}
		printAgentTable(agents)
		return nil
	case "udpate", "update":
		fs := flag.NewFlagSet("agent update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "agent id")
		var description, password string
		fs.StringVar(&description, "desc", "", "agent description")
		fs.StringVar(&description, "description", "", "agent description")
		fs.StringVar(&password, "password", "", "agent password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("agent update requires -id")
		}
		visited := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
		descriptionSet := visited["desc"] || visited["description"]
		passwordSet := visited["password"]
		if !descriptionSet && !passwordSet {
			return errors.New("agent update requires at least one of -desc|-description, -password")
		}
		var descPtr, passPtr *string
		if descriptionSet {
			trimmed := strings.TrimSpace(description)
			descPtr = &trimmed
		}
		if passwordSet {
			trimmed := strings.TrimSpace(password)
			passPtr = &trimmed
		}
		agent, err := svc.UpdateAgent(*id, libticket.AgentUpdateRequest{
			Description: descPtr,
			Password:    passPtr,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agent)
		}
		fmt.Printf("updated agent %s\n", agent.UUID)
		return nil
	case "rm", "delete":
		fs := flag.NewFlagSet("agent "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("agent rm/delete requires -id")
		}
		if err := svc.DeleteAgent(*id); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "agent_id": *id})
		}
		fmt.Printf("deleted agent %d\n", *id)
		return nil
	case "enable", "disable":
		fs := flag.NewFlagSet("agent "+args[0], flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("agent enable/disable requires -id")
		}
		agent, err := svc.SetAgentEnabled(*id, args[0] == "enable")
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agent)
		}
		fmt.Printf("%sd agent %s\n", args[0], agent.UUID)
		return nil
	case "run":
		fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentID := fs.String("id", "", "agent UUID")
		password := fs.String("password", "", "agent password")
		url := fs.String("url", "", "ticket server url")
		projectID := fs.Int64("project-id", 0, "project id override")
		pollSeconds := fs.Int("poll-seconds", 2, "idle poll interval seconds")
		llmCommand := fs.String("llm", envValue("TICKET_AGENT_LLM"), "llm command (claude, codex, or path to binary)")
		verbose := fs.Bool("v", false, "verbose logging")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agentIDVal := strings.TrimSpace(*agentID)
		if agentIDVal == "" {
			agentIDVal = envValue("AGENT_ID")
		}
		agentPassword := strings.TrimSpace(*password)
		if agentPassword == "" {
			agentPassword = envValue("AGENT_PASSWORD")
		}
		serverURL := strings.TrimSpace(*url)
		if serverURL == "" {
			serverURL = envValue("TICKET_URL")
		}
		missing := make([]string, 0, 3)
		if agentIDVal == "" {
			missing = append(missing, "AGENT_ID or -id")
		}
		if agentPassword == "" {
			missing = append(missing, "AGENT_PASSWORD or -password")
		}
		if serverURL == "" {
			missing = append(missing, "TICKET_URL or -url")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
		}
		if *pollSeconds < 1 {
			return errors.New("poll-seconds must be >= 1")
		}
		if err := os.Setenv("TICKET_URL", serverURL); err != nil {
			return err
		}
		if resolved, rErr := config.ResolveURL(); rErr != nil || resolved.Mode != config.ModeRemote {
			return errors.New("agent run requires a remote server (TICKET_URL must be set to an http(s) URL)")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if _, err := svc.RegisterAgent(libticket.AgentRegisterRequest{
			ID:       agentIDVal,
			Password: agentPassword,
		}); err != nil {
			return err
		}
		if !outputJSON {
			fmt.Printf("agent %s registered\n", agentIDVal)
		}
		modelCommand := strings.TrimSpace(*llmCommand)
		if modelCommand == "" {
			modelCommand = "claude"
		}
		agentVerbose := *verbose
		idleDelay := time.Duration(*pollSeconds) * time.Second
		for {
			if agentVerbose {
				fmt.Printf("[agent] requesting work (project=%d)\n", *projectID)
			}
			response, err := svc.RequestAgentWork(libticket.AgentRequest{
				ID:        agentIDVal,
				Password:  agentPassword,
				ProjectID: *projectID,
			})
			if err != nil {
				return err
			}
			if agentVerbose {
				fmt.Printf("[agent] response status=%s", response.Status)
				if response.Ticket != nil {
					fmt.Printf(" ticket=%s type=%s title=%q", response.Ticket.Key, response.Ticket.Type, response.Ticket.Title)
				}
				fmt.Println()
			}
			if (response.Status != "NEW" && response.Status != "CURRENT") || response.Ticket == nil {
				if agentVerbose {
					fmt.Printf("[agent] no work, sleeping %s\n", idleDelay)
				}
				time.Sleep(idleDelay)
				continue
			}
			ticket := response.Ticket
			if agentVerbose {
				fmt.Printf("[agent] processing %s %q\n", ticketLabel(*ticket), ticket.Title)
			}
			prompt := buildAgentPrompt(*ticket)
			result, err := runAgentCommand(modelCommand, prompt, agentVerbose, ticket.Key)
			if err != nil {
				fmt.Printf("failed %s: %v\n", ticketLabel(*ticket), err)
				return fmt.Errorf("agent llm processing failed for ticket %s: %w", ticketLabel(*ticket), err)
			}
			if agentVerbose {
				fmt.Printf("[agent] submitting result for %s (%d bytes)\n", ticketLabel(*ticket), len(result))
			}
			updated, err := svc.AgentUpdateTicket(ticket.ID, libticket.AgentTicketUpdateRequest{
				ID:       agentIDVal,
				Password: agentPassword,
				Result:   strings.TrimSpace(result),
			})
			if err != nil {
				fmt.Printf("failed %s: could not submit result: %v\n", ticketLabel(*ticket), err)
				return err
			}
			if outputJSON {
				if err := printJSON(updated); err != nil {
					return err
				}
			}
			fmt.Printf("completed %s -> %s\n", ticketLabel(*ticket), updated.Status)
		}
	case "request":
		fs := flag.NewFlagSet("agent request", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		reqAgentID := fs.String("agent-id", "", "agent UUID")
		password := fs.String("password", "", "agent password")
		url := fs.String("url", "", "ticket server url")
		id := fs.Int64("id", 0, "specific ticket id")
		dryRun := fs.Bool("dryrun", false, "simulate assignment only")
		loop := fs.Int("loop", 1, "number of request loops")
		sleepSeconds := fs.Int("sleep", 1, "sleep seconds between loops")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		reqAgentIDVal := strings.TrimSpace(*reqAgentID)
		if reqAgentIDVal == "" {
			reqAgentIDVal = envValue("AGENT_ID")
		}
		agentPassword := strings.TrimSpace(*password)
		if agentPassword == "" {
			agentPassword = envValue("AGENT_PASSWORD")
		}
		serverURL := strings.TrimSpace(*url)
		if serverURL == "" {
			serverURL = envValue("TICKET_URL")
		}
		missing := make([]string, 0, 3)
		if reqAgentIDVal == "" {
			missing = append(missing, "AGENT_ID or -agent-id")
		}
		if agentPassword == "" {
			missing = append(missing, "AGENT_PASSWORD or -password")
		}
		if serverURL == "" {
			missing = append(missing, "TICKET_URL or -url")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
		}
		if *loop < 0 {
			return errors.New("loop must be >= 0")
		}
		if *sleepSeconds < 0 {
			return errors.New("sleep must be >= 0")
		}
		if err := os.Setenv("TICKET_URL", serverURL); err != nil {
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
		var requestedID *int64
		if *id > 0 {
			requestedID = id
		}
		for i := 0; *loop == 0 || i < *loop; i++ {
			response, err := svc.RequestAgentWork(libticket.AgentRequest{
				ID:       reqAgentIDVal,
				Password: agentPassword,
				TicketID: requestedID,
				DryRun:   *dryRun,
			})
			if err != nil {
				return err
			}
			if err := printJSON(response); err != nil {
				return err
			}
			if *loop == 0 || i < *loop-1 {
				time.Sleep(time.Duration(*sleepSeconds) * time.Second)
			}
		}
		return nil
	case "reset-password":
		fs := flag.NewFlagSet("agent reset-password", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "agent id")
		newPassword := fs.String("password", "", "new password (generated if omitted)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket agent reset-password -id <agent-id> [-password <new-password>]")
		}
		pw := strings.TrimSpace(*newPassword)
		if pw == "" {
			generated, err := generatePassword(24)
			if err != nil {
				return err
			}
			pw = generated
		}
		agent, err := svc.UpdateAgent(*id, libticket.AgentUpdateRequest{Password: &pw})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"agent_id": agent.UUID, "password": pw})
		}
		fmt.Printf("agent    : %s (%s)\n", agent.Username, agent.UUID)
		fmt.Printf("password : %s\n", pw)
		return nil
	case "config-set":
		if len(args) < 4 {
			return errors.New("usage: ticket agent config-set -id <agent-id> <key> <value>")
		}
		fs := flag.NewFlagSet("agent config-set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentID := fs.Int64("id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *agentID == 0 || fs.NArg() < 2 {
			return errors.New("usage: ticket agent config-set -id <agent-id> <key> <value>")
		}
		if err := svc.SetAgentConfig(*agentID, fs.Arg(0), fs.Arg(1)); err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", fs.Arg(0), fs.Arg(1))
		return nil
	case "config-ls":
		fs := flag.NewFlagSet("agent config-ls", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentID := fs.Int64("id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *agentID == 0 {
			return errors.New("usage: ticket agent config-ls -id <agent-id>")
		}
		entries, err := svc.ListAgentConfig(*agentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("(no config)")
			return nil
		}
		for _, e := range entries {
			fmt.Printf("%s=%s\n", e.Key, e.Value)
		}
		return nil
	case "config-rm":
		fs := flag.NewFlagSet("agent config-rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		agentID := fs.Int64("id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *agentID == 0 || fs.NArg() < 1 {
			return errors.New("usage: ticket agent config-rm -id <agent-id> <key>")
		}
		if err := svc.DeleteAgentConfig(*agentID, fs.Arg(0)); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", fs.Arg(0))
		return nil
	default:
		return fmt.Errorf("unknown agent command %q; see: ticket agent help", args[0])
	}
}

func buildAgentPrompt(ticket store.Ticket) string {
	var b strings.Builder
	// b.WriteString("You are an autonomous software agent working a ticket.\n")
	// b.WriteString("Return only the final ticket update text.\n\n")
	b.WriteString(fmt.Sprintf("Ticket: %s\n", ticketLabel(ticket)))
	b.WriteString(fmt.Sprintf("Title: %s\n", strings.TrimSpace(ticket.Title)))
	if strings.TrimSpace(ticket.Description) != "" {
		b.WriteString("Description:\n")
		b.WriteString(strings.TrimSpace(ticket.Description))
		b.WriteString("\n")
	}
	if strings.TrimSpace(ticket.AcceptanceCriteria) != "" {
		b.WriteString("Acceptance Criteria:\n")
		b.WriteString(strings.TrimSpace(ticket.AcceptanceCriteria))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func printAgentTable(agents []store.Agent) {
	if len(agents) == 0 {
		fmt.Println("no agents")
		return
	}
	sort.SliceStable(agents, func(i, j int) bool {
		return strings.ToLower(agents[i].Username) < strings.ToLower(agents[j].Username)
	})
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tENABLED\tSTATUS\tLAST_SEEN")
	for _, agent := range agents {
		lastSeen := strings.TrimSpace(agent.LastSeen)
		if lastSeen == "" {
			lastSeen = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\t%s\n", agent.UUID, agent.Username, agent.Description, agent.Enabled, agent.Status, lastSeen)
	}
	_ = w.Flush()
}

func printUserTable(users []store.User) {
	if len(users) == 0 {
		fmt.Println("no users")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tROLE\tENABLED\tCREATED")
	for _, user := range users {
		created := user.CreatedAt
		if len(created) > 10 {
			created = created[:10]
		}
		fmt.Fprintf(w, "%s\t%s\t%t\t%s\n", user.Username, user.Role, user.Enabled, created)
	}
	_ = w.Flush()
}

func runStory(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: ticket story <command>

Commands:
  create, add, new -title <title> [-d <desc>]    Create a story
  list, ls                                        List stories in active project
  get <id>                                        Show story detail
  update <id> -title <title> [-d <desc>]          Update a story
  delete <id>                                     Delete a story`)
		return nil
	}

	cfg, _, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "create", "add", "new":
		fs := flag.NewFlagSet("story create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "story title")
		description := fs.String("d", "", "story description")
		// Pull positional title before flags so flag parser sees flags only.
		rest := args[1:]
		var positional []string
		for len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			positional = append(positional, rest[0])
			rest = rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if *title == "" && len(positional) > 0 {
			*title = strings.Join(positional, " ")
		}
		if strings.TrimSpace(*title) == "" {
			return errors.New("usage: ticket story create -title <title> [-d description]")
		}
		story, err := svc.CreateStory(project.ID, *title, *description)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		fmt.Printf("story %d: %s\n", story.ID, story.Title)
		return nil
	case "list", "ls":
		stories, err := svc.ListStories(project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stories)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tTITLE")
		for _, s := range stories {
			fmt.Fprintf(w, "%d\t%s\t%s\n", s.ID, s.Status, s.Title)
		}
		_ = w.Flush()
		return nil
	case "get":
		if len(args) != 2 {
			return errors.New("usage: ticket story get <id>")
		}
		var id int64
		if _, err := fmt.Sscan(args[1], &id); err != nil {
			return fmt.Errorf("invalid story id %q", args[1])
		}
		story, err := svc.GetStory(id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		fmt.Printf("ID          : %d\n", story.ID)
		fmt.Printf("ProjectID   : %d\n", story.ProjectID)
		fmt.Printf("Title       : %s\n", story.Title)
		fmt.Printf("Description : %s\n", story.Description)
		fmt.Printf("Status      : %s\n", story.Status)
		fmt.Printf("Created     : %s\n", story.CreatedAt)
		fmt.Printf("Updated     : %s\n", story.UpdatedAt)
		return nil
	case "update":
		if len(args) < 2 {
			return errors.New("usage: ticket story update <id> -title <title> [-d description]")
		}
		var id int64
		if _, err := fmt.Sscan(args[1], &id); err != nil {
			return fmt.Errorf("invalid story id %q", args[1])
		}
		fs := flag.NewFlagSet("story update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "story title")
		description := fs.String("d", "", "story description")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		// Fetch current to use as defaults
		current, err := svc.GetStory(id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" {
			*title = current.Title
		}
		if strings.TrimSpace(*description) == "" {
			*description = current.Description
		}
		story, err := svc.UpdateStory(id, *title, *description)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(story)
		}
		fmt.Printf("story %d updated: %s\n", story.ID, story.Title)
		return nil
	case "delete":
		if len(args) != 2 {
			return errors.New("usage: ticket story delete <id>")
		}
		var id int64
		if _, err := fmt.Sscan(args[1], &id); err != nil {
			return fmt.Errorf("invalid story id %q", args[1])
		}
		if err := svc.DeleteStory(id); err != nil {
			return err
		}
		fmt.Printf("deleted story %d\n", id)
		return nil
	default:
		return fmt.Errorf("unknown story command %q", args[0])
	}
}

func runEpic(args []string) error {
	// Subcommands: use <id>, clear, list/ls — otherwise fall through to create
	if len(args) > 0 {
		switch args[0] {
		case "use":
			if len(args) != 2 {
				return errors.New("usage: ticket epic use <id>")
			}
			cfg, svc, _, err := resolveCurrentProjectClient()
			if err != nil {
				return err
			}
			ticket, err := svc.GetTicket(args[1])
			if err != nil {
				return err
			}
			if ticket.Type != "epic" {
				return fmt.Errorf("ticket %s is not an epic", args[1])
			}
			cfg.CurrentEpicID = ticket.ID
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("using epic %s: %s\n", ticket.Key, ticket.Title)
			return nil
		case "clear":
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.CurrentEpicID = 0
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Println("active epic cleared")
			return nil
		case "list", "ls":
			cfg, _, project, err := resolveCurrentProjectClient()
			if err != nil {
				return err
			}
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			epics, err := svc.ListTicketsFiltered(project.ID, "epic", "", "", "", "", "", 0, false)
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(epics)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tSTATUS\tTITLE")
			for _, t := range epics {
				active := " "
				if t.ID == cfg.CurrentEpicID {
					active = "*"
				}
				fmt.Fprintf(w, "%s%s\t%s\t%s\n", active, t.Key, t.Status, t.Title)
			}
			_ = w.Flush()
			return nil
		}
	}
	return runTypedTicketCreate("epic", args)
}

func runProject(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(projectUsage)
		return nil
	}

	if projectID, ok := parseProjectCommandID(args[0]); ok {
		return runProjectByID(svc, projectID, args[1:])
	}

	switch args[0] {
	case "add-user":
		return runProjectAddUser(svc, args[1:])
	case "remove-user":
		return runProjectRemoveUser(svc, args[1:])
	case "add-team":
		return runProjectAddTeam(svc, args[1:])
	case "remove-team":
		return runProjectRemoveTeam(svc, args[1:])
	case "create", "add", "new":
		fs := flag.NewFlagSet("project create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		prefix := fs.String("prefix", "", "project prefix")
		title := fs.String("title", "", "project title")
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitBranch := fs.String("git-branch", "", "project git branch")
		workflowID := fs.Int64("workflow", 0, "workflow id to associate")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: ticket project create -title <title> -prefix <prefix> [-description text] [-ac text] [-workflow id]")
		}
		if strings.TrimSpace(*prefix) == "" {
			return errors.New("project prefix is required")
		}
		if strings.TrimSpace(*title) == "" {
			return errors.New("project title is required")
		}
		var wfID *int64
		if *workflowID > 0 {
			wfID = workflowID
		}
		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Prefix:             *prefix,
			Title:              *title,
			Description:        *description,
			AcceptanceCriteria: *acceptanceCriteria,
			GitRepository:      strings.TrimSpace(*gitRepository),
			GitBranch:          strings.TrimSpace(*gitBranch),
			WorkflowID:         wfID,
		})
		if err != nil {
			return err
		}
		cfg.CurrentProject = project.Prefix
		cfg.CurrentEpicID = 0
		if err := config.Save(cfg); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "list", "ls":
		projects, err := svc.ListProjects()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(projects)
		}
		workflowNames := map[int64]string{}
		if wfs, err := svc.ListWorkflows(); err == nil {
			for _, wf := range wfs {
				workflowNames[wf.ID] = wf.Name
			}
		}
		printProjectTable(projects, cfg.CurrentProject, workflowNames)
		return nil
	case "get":
		if len(args) != 2 {
			return errors.New("usage: ticket project get <id>")
		}
		project, err := svc.GetProject(args[1])
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "use", "default":
		if len(args) < 2 {
			// No ID: print the current project
			if cfg.CurrentProject == "" {
				fmt.Println("no project set")
				return nil
			}
			project, err := svc.GetProject(cfg.CurrentProject)
			if err != nil {
				fmt.Println(cfg.CurrentProject)
				return nil
			}
			fmt.Printf("%s — %s\n", project.Prefix, project.Title)
			return nil
		}
		project, err := svc.GetProject(args[1])
		if err != nil {
			return err
		}
		cfg.CurrentProject = project.Prefix
		cfg.CurrentEpicID = 0
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("using project %s\n", project.Prefix)
		return nil
	case "update":
		if cfg.CurrentProject == "" {
			return errors.New("no current project set; use: tk project use <id>")
		}
		project, err := svc.GetProject(cfg.CurrentProject)
		if err != nil {
			return err
		}
		return runProjectByID(svc, project.ID, args)
	case "init":
		return runProjectInit(cfg, svc, args[1:])
	case "rm", "delete":
		fs := flag.NewFlagSet("project rm", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "project id or prefix")
		confirm := fs.String("confirm", "", "confirmation token from first run")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
			v := fs.Arg(0)
			id = &v
		}
		if strings.TrimSpace(*id) == "" {
			return errors.New("usage: ticket project rm [-id] <id> [--confirm <token>]")
		}
		project, err := svc.GetProject(strings.TrimSpace(*id))
		if err != nil {
			return err
		}
		if strings.TrimSpace(*confirm) == "" {
			// Phase 1: generate confirmation token
			token, err := generateConfirmToken()
			if err != nil {
				return err
			}
			tickets, _ := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, true)
			fmt.Printf("project  : %s — %s\n", project.Prefix, project.Title)
			fmt.Printf("tickets  : %d\n", len(tickets))
			fmt.Printf("\nThis will permanently delete the project and all associated data.\n")
			fmt.Printf("To confirm, run:\n\n")
			fmt.Printf("  ticket project rm -id %s --confirm %s\n\n", *id, token)
			// Store token temporarily in config
			cfg.DeleteConfirmToken = token
			cfg.DeleteConfirmProject = fmt.Sprintf("%d", project.ID)
			return config.Save(cfg)
		}
		// Phase 2: verify token and delete
		if *confirm != cfg.DeleteConfirmToken || fmt.Sprintf("%d", project.ID) != cfg.DeleteConfirmProject {
			return errors.New("invalid confirmation token")
		}
		if err := svc.DeleteProject(project.ID); err != nil {
			return err
		}
		// Clear stored token and switch project if needed
		cfg.DeleteConfirmToken = ""
		cfg.DeleteConfirmProject = ""
		if cfg.CurrentProject == project.Prefix || cfg.CurrentProject == fmt.Sprintf("%d", project.ID) {
			cfg.CurrentProject = ""
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("deleted project %s — %s\n", project.Prefix, project.Title)
		return nil
	default:
		return fmt.Errorf("unknown project command %q; see: ticket project help", args[0])
	}
}

func runProjectInit(cfg config.Config, svc libticket.Service, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dirName := filepath.Base(cwd)

	fs := flag.NewFlagSet("project init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	prefix := fs.String("prefix", strings.ToUpper(dirName[:min(3, len(dirName))]), "project prefix (default: first 3 chars of dir name)")
	title := fs.String("title", dirName, "project title (default: directory name)")
	description := fs.String("description", dirName, "project description (default: directory name)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if a project is already initialised
	if cfg.CurrentProject != "" {
		cfgPath, _ := config.Path()
		return fmt.Errorf("project already initialised: %s (in %s)", cfg.CurrentProject, cfgPath)
	}

	// Try to find existing project by prefix
	project, err := svc.GetProject(*prefix)
	if err != nil {
		// Project doesn't exist — create it
		project, err = svc.CreateProject(libticket.ProjectCreateRequest{
			Prefix:      *prefix,
			Title:       *title,
			Description: *description,
		})
		if err != nil {
			return err
		}
		fmt.Printf("created project %s (%s)\n", project.Prefix, project.Title)
	} else {
		fmt.Printf("found existing project %s (%s)\n", project.Prefix, project.Title)
	}

	cfg.CurrentProject = project.Prefix
	cfg.CurrentEpicID = 0
	if err := config.Save(cfg); err != nil {
		return err
	}
	return nil
}

func runProjectAddUser(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project add-user", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	userID := fs.Int64("user_id", 0, "user id")
	projectID := fs.Int64("project_id", 0, "project id")
	role := fs.String("role", "", "project role [viewer,editor,owner]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == 0 || *projectID == 0 || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: ticket project add-user -user_id <id> -project_id <id> -role <viewer|editor|owner>")
	}
	member, err := svc.AddProjectMember(*projectID, libticket.ProjectMemberRequest{
		UserID: *userID,
		Role:   strings.TrimSpace(*role),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(member)
	}
	fmt.Printf("added project user: project_id=%d user_id=%d role=%s\n", member.ProjectID, member.UserID, member.Role)
	return nil
}

func runProjectRemoveUser(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project remove-user", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	userID := fs.Int64("user_id", 0, "user id")
	projectID := fs.Int64("project_id", 0, "project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *userID == 0 || *projectID == 0 || fs.NArg() != 0 {
		return errors.New("usage: ticket project remove-user -user_id <id> -project_id <id>")
	}
	if err := svc.RemoveProjectMember(*projectID, *userID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": *projectID, "user_id": *userID})
	}
	fmt.Printf("removed project user: project_id=%d user_id=%d\n", *projectID, *userID)
	return nil
}

func runProjectAddTeam(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project add-team", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	teamID := fs.Int64("team_id", 0, "team id")
	projectID := fs.Int64("project_id", 0, "project id")
	role := fs.String("role", "", "project role [viewer,editor,owner]")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || *projectID == 0 || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
		return errors.New("usage: ticket project add-team -team_id <id> -project_id <id> -role <viewer|editor|owner>")
	}
	member, err := svc.AddProjectTeamMember(*projectID, libticket.ProjectTeamMemberRequest{
		TeamID: *teamID,
		Role:   strings.TrimSpace(*role),
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(member)
	}
	fmt.Printf("added project team: project_id=%d team_id=%d role=%s\n", member.ProjectID, member.TeamID, member.Role)
	return nil
}

func runProjectRemoveTeam(svc libticket.Service, args []string) error {
	fs := flag.NewFlagSet("project remove-team", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	teamID := fs.Int64("team_id", 0, "team id")
	projectID := fs.Int64("project_id", 0, "project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *teamID == 0 || *projectID == 0 || fs.NArg() != 0 {
		return errors.New("usage: ticket project remove-team -team_id <id> -project_id <id>")
	}
	if err := svc.RemoveProjectTeamMember(*projectID, *teamID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "project_id": *projectID, "team_id": *teamID})
	}
	fmt.Printf("removed project team: project_id=%d team_id=%d\n", *projectID, *teamID)
	return nil
}

func runTeam(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		teams, err := svc.ListTeams()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(teams)
		}
		printTeamTable(teams)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(teamUsage)
		return nil
	case "list", "ls":
		teams, err := svc.ListTeams()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(teams)
		}
		printTeamTable(teams)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("team create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "team name")
		parentID := fs.Int64("parent_id", 0, "optional parent team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" || fs.NArg() != 0 {
			return errors.New("usage: ticket team create -name <name> [-parent_id <id>]")
		}
		var parent *int64
		if *parentID > 0 {
			parent = parentID
		}
		team, err := svc.CreateTeam(libticket.TeamRequest{
			Name:         strings.TrimSpace(*name),
			ParentTeamID: parent,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(team)
		}
		fmt.Printf("created team #%d %s\n", team.ID, team.Name)
		return nil
	case "update":
		fs := flag.NewFlagSet("team update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "team id")
		name := fs.String("name", "", "team name")
		parentID := fs.Int64("parent_id", -1, "parent team id (0 clears)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team update -id <id> [-name <name>] [-parent_id <id|0>]")
		}
		var parent *int64
		if *parentID > 0 {
			parent = parentID
		}
		team, err := svc.UpdateTeam(*id, libticket.TeamRequest{
			Name:         strings.TrimSpace(*name),
			ParentTeamID: parent,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(team)
		}
		fmt.Printf("updated team #%d %s\n", team.ID, team.Name)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("team delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team delete -id <id>")
		}
		if err := svc.DeleteTeam(*id); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *id})
		}
		fmt.Printf("deleted team #%d\n", *id)
		return nil
	case "add-user":
		fs := flag.NewFlagSet("team add-user", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		userID := fs.Int64("user_id", 0, "user id")
		role := fs.String("role", "", "team role [member,owner]")
		jobTitle := fs.String("job_title", "", "job title")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *userID == 0 || strings.TrimSpace(*role) == "" || fs.NArg() != 0 {
			return errors.New("usage: ticket team add-user -team_id <id> -user_id <id> -role <member|owner> [-job_title <title>]")
		}
		member, err := svc.AddTeamMember(*teamID, libticket.TeamMemberRequest{
			UserID:   *userID,
			Role:     strings.TrimSpace(*role),
			JobTitle: strings.TrimSpace(*jobTitle),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(member)
		}
		fmt.Printf("added team user: team_id=%d user_id=%d role=%s job_title=%s\n", member.TeamID, member.UserID, member.Role, member.JobTitle)
		return nil
	case "remove-user":
		fs := flag.NewFlagSet("team remove-user", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		userID := fs.Int64("user_id", 0, "user id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *userID == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team remove-user -team_id <id> -user_id <id>")
		}
		if err := svc.RemoveTeamMember(*teamID, *userID); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *teamID, "user_id": *userID})
		}
		fmt.Printf("removed team user: team_id=%d user_id=%d\n", *teamID, *userID)
		return nil
	case "users":
		fs := flag.NewFlagSet("team users", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team users -team_id <id>")
		}
		members, err := svc.ListTeamMembers(*teamID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(members)
		}
		printTeamMemberTable(members)
		return nil
	case "add-agent":
		fs := flag.NewFlagSet("team add-agent", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		agentID := fs.Int64("agent_id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *agentID == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team add-agent -team_id <id> -agent_id <id>")
		}
		item, err := svc.AddTeamAgent(*teamID, *agentID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(item)
		}
		fmt.Printf("added team agent: team_id=%d agent_id=%d\n", item.TeamID, item.AgentID)
		return nil
	case "remove-agent":
		fs := flag.NewFlagSet("team remove-agent", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		agentID := fs.Int64("agent_id", 0, "agent id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || *agentID == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team remove-agent -team_id <id> -agent_id <id>")
		}
		if err := svc.RemoveTeamAgent(*teamID, *agentID); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "team_id": *teamID, "agent_id": *agentID})
		}
		fmt.Printf("removed team agent: team_id=%d agent_id=%d\n", *teamID, *agentID)
		return nil
	case "agents":
		fs := flag.NewFlagSet("team agents", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		teamID := fs.Int64("team_id", 0, "team id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *teamID == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket team agents -team_id <id>")
		}
		items, err := svc.ListTeamAgents(*teamID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(items)
		}
		printTeamAgentTable(items)
		return nil
	default:
		return fmt.Errorf("unknown team command %q; see: ticket team help", args[0])
	}
}

func printTeamTable(teams []store.Team) {
	if len(teams) == 0 {
		fmt.Println("no teams")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPARENT_TEAM_ID")
	for _, team := range teams {
		parent := "-"
		if team.ParentTeamID != nil {
			parent = fmt.Sprintf("%d", *team.ParentTeamID)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", team.ID, team.Name, parent)
	}
	_ = w.Flush()
}

func printTeamMemberTable(members []store.TeamMember) {
	if len(members) == 0 {
		fmt.Println("no team members")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TEAM_ID\tUSER_ID\tUSERNAME\tROLE\tJOB_TITLE")
	for _, m := range members {
		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n", m.TeamID, m.UserID, m.Username, m.Role, m.JobTitle)
	}
	_ = w.Flush()
}

func printTeamAgentTable(items []store.TeamAgent) {
	if len(items) == 0 {
		fmt.Println("no team agents")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TEAM_ID\tAGENT_ID\tNAME\tENABLED\tSTATUS")
	for _, item := range items {
		fmt.Fprintf(w, "%d\t%d\t%s\t%t\t%s\n", item.TeamID, item.AgentID, item.AgentName, item.Enabled, item.Status)
	}
	_ = w.Flush()
}

func runRole(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		roles, err := svc.ListRoles()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(roles)
		}
		printRoleTable(roles)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(roleUsage)
		return nil
	case "list", "ls":
		roles, err := svc.ListRoles()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(roles)
		}
		printRoleTable(roles)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("role create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "role title")
		motivation := fs.String("motivation", "", "role motivation")
		goals := fs.String("goals", "", "role goals")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: ticket role create -title <title> [-motivation <text>] [-goals <text>]")
		}
		role, err := svc.CreateRole(libticket.RoleRequest{
			Title:      strings.TrimSpace(*title),
			Motivation: strings.TrimSpace(*motivation),
			Goals:      strings.TrimSpace(*goals),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		fmt.Printf("created role #%d %s\n", role.ID, role.Title)
		return nil
	case "update":
		fs := flag.NewFlagSet("role update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "role id")
		title := fs.String("title", "", "role title")
		motivation := fs.String("motivation", "", "role motivation")
		goals := fs.String("goals", "", "role goals")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || strings.TrimSpace(*title) == "" || fs.NArg() != 0 {
			return errors.New("usage: ticket role update -id <id> -title <title> [-motivation <text>] [-goals <text>]")
		}
		role, err := svc.UpdateRole(*id, libticket.RoleRequest{
			Title:      strings.TrimSpace(*title),
			Motivation: strings.TrimSpace(*motivation),
			Goals:      strings.TrimSpace(*goals),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(role)
		}
		fmt.Printf("updated role #%d %s\n", role.ID, role.Title)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("role delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "role id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 || fs.NArg() != 0 {
			return errors.New("usage: ticket role delete -id <id>")
		}
		if err := svc.DeleteRole(*id); err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"status": "deleted", "role_id": *id})
		}
		fmt.Printf("deleted role #%d\n", *id)
		return nil
	default:
		return fmt.Errorf("unknown role command %q; see: ticket role help", args[0])
	}
}

func runWorkflow(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		workflows, err := svc.ListWorkflows()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(workflows)
		}
		printWorkflowTable(workflows)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(workflowUsage)
		return nil
	case "list", "ls":
		workflows, err := svc.ListWorkflows()
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(workflows)
		}
		printWorkflowTable(workflows)
		return nil
	case "create", "add", "new":
		fs := flag.NewFlagSet("workflow create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "workflow name")
		desc := fs.String("d", "", "workflow description")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return errors.New("usage: ticket workflow create -name <name> [-d <description>]")
		}
		wf, err := svc.CreateWorkflow(libticket.WorkflowRequest{Name: *name, Description: *desc})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("workflow: %s\nworkflow_id: %d\n", wf.Name, wf.ID)
		return nil
	case "get":
		fs := flag.NewFlagSet("workflow get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket workflow get -id <id>")
		}
		wf, err := svc.GetWorkflow(*id)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		printWorkflowDetail(wf)
		return nil
	case "delete", "rm":
		fs := flag.NewFlagSet("workflow delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket workflow delete -id <id>")
		}
		if err := svc.DeleteWorkflow(*id); err != nil {
			return err
		}
		fmt.Printf("deleted workflow %d\n", *id)
		return nil
	case "add-stage":
		fs := flag.NewFlagSet("workflow add-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		name := fs.String("name", "", "stage name")
		desc := fs.String("d", "", "stage description")
		roleID := fs.Int64("role", 0, "role id")
		order := fs.Int("order", 0, "sort order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || *name == "" {
			return errors.New("usage: ticket workflow add-stage -id <workflow_id> -name <stage> [-role <role_id>] [-d <desc>] [-order <n>]")
		}
		var rID *int64
		if *roleID > 0 {
			rID = roleID
		}
		stage, err := svc.AddWorkflowStage(*wfID, libticket.WorkflowStageRequest{
			StageName:   *name,
			Description: *desc,
			RoleID:      rID,
			SortOrder:   *order,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(stage)
		}
		fmt.Printf("added stage: %s (id %d)\n", stage.StageName, stage.ID)
		return nil
	case "remove-stage":
		fs := flag.NewFlagSet("workflow remove-stage", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		stageID := fs.Int64("stage-id", 0, "workflow stage id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *stageID == 0 {
			return errors.New("usage: ticket workflow remove-stage -stage-id <id>")
		}
		if err := svc.RemoveWorkflowStage(*stageID); err != nil {
			return err
		}
		fmt.Printf("removed stage %d\n", *stageID)
		return nil
	case "reorder-stages":
		fs := flag.NewFlagSet("workflow reorder-stages", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		wfID := fs.Int64("id", 0, "workflow id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *wfID == 0 || fs.NArg() < 1 {
			return errors.New("usage: ticket workflow reorder-stages -id <workflow_id> <stage_id,stage_id,...>")
		}
		parts := strings.Split(fs.Arg(0), ",")
		ids := make([]int64, 0, len(parts))
		for _, p := range parts {
			v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid stage id %q", p)
			}
			ids = append(ids, v)
		}
		if err := svc.ReorderWorkflowStages(*wfID, ids); err != nil {
			return err
		}
		fmt.Println("stages reordered")
		return nil
	case "export":
		fs := flag.NewFlagSet("workflow export", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.Int64("id", 0, "workflow id")
		outFile := fs.String("o", "", "output file (default stdout)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return errors.New("usage: ticket workflow export -id <id> [-o file]")
		}
		export, err := svc.ExportWorkflow(*id)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(export, "", "  ")
		if err != nil {
			return err
		}
		if *outFile != "" {
			return os.WriteFile(*outFile, append(data, '\n'), 0o644)
		}
		fmt.Println(string(data))
		return nil
	case "import":
		fs := flag.NewFlagSet("workflow import", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		inFile := fs.String("file", "", "input file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *inFile == "" {
			return errors.New("usage: ticket workflow import -file <file>")
		}
		data, err := os.ReadFile(*inFile)
		if err != nil {
			return err
		}
		var export store.WorkflowExport
		if err := json.Unmarshal(data, &export); err != nil {
			return err
		}
		wf, err := svc.ImportWorkflow(export)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(wf)
		}
		fmt.Printf("imported workflow: %s (id %d)\n", wf.Name, wf.ID)
		return nil
	default:
		return fmt.Errorf("unknown workflow command %q; see: ticket workflow help", args[0])
	}
}

func printWorkflowTable(workflows []store.Workflow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")
	for _, wf := range workflows {
		fmt.Fprintf(w, "%d\t%s\t%s\n", wf.ID, wf.Name, wf.Description)
	}
	_ = w.Flush()
}

func printWorkflowDetail(wf store.WorkflowWithStages) {
	fmt.Printf("ID          : %d\n", wf.ID)
	fmt.Printf("Name        : %s\n", wf.Name)
	fmt.Printf("Description : %s\n", wf.Description)
	fmt.Printf("Stages      :\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  ORDER\tID\tSTAGE\tROLE\tDESCRIPTION")
	for _, s := range wf.Stages {
		fmt.Fprintf(w, "  %d\t%d\t%s\t%s\t%s\n", s.SortOrder, s.ID, s.StageName, s.RoleTitle, s.Description)
	}
	_ = w.Flush()
}

func runLabel(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(labelUsage)
		return nil
	}
	_, svc, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	switch args[0] {
	case "list", "ls":
		labels, err := svc.ListLabels(project.ID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(labels)
		}
		if len(labels) == 0 {
			fmt.Println("no labels")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tCOLOR")
		for _, l := range labels {
			fmt.Fprintf(w, "%d\t%s\t%s\n", l.ID, l.Name, l.Color)
		}
		return w.Flush()
	case "create":
		fs := flag.NewFlagSet("label create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "label name")
		color := fs.String("color", "", "label color (e.g. #ff0000)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" && fs.NArg() > 0 {
			*name = fs.Arg(0)
		}
		if *name == "" {
			return errors.New("usage: ticket label create <name> [-color <color>]")
		}
		label, err := svc.CreateLabel(project.ID, libticket.LabelRequest{Name: *name, Color: *color})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(label)
		}
		fmt.Printf("label created: %d %s\n", label.ID, label.Name)
		return nil
	case "delete":
		fs := flag.NewFlagSet("label delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "label ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		idStr := *idFlag
		if idStr == "" && fs.NArg() > 0 {
			idStr = fs.Arg(0)
		}
		if idStr == "" {
			return errors.New("usage: ticket label delete -id <label-id>")
		}
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			return errors.New("label id must be numeric")
		}
		return svc.DeleteLabel(id)
	case "add":
		fs := flag.NewFlagSet("label add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var ticketID, labelID int64
		if *idFlag != "" && fs.NArg() > 0 {
			if _, err := fmt.Sscan(*idFlag, &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
			if _, err := fmt.Sscan(fs.Arg(0), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else if fs.NArg() >= 2 {
			// positional fallback
			if _, err := fmt.Sscan(fs.Arg(0), &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
			if _, err := fmt.Sscan(fs.Arg(1), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else {
			return errors.New("usage: ticket label add -id <ticket-id> <label-id>")
		}
		return svc.AddTicketLabel(ticketID, labelID)
	case "remove":
		fs := flag.NewFlagSet("label remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var ticketID, labelID int64
		if *idFlag != "" && fs.NArg() > 0 {
			if _, err := fmt.Sscan(*idFlag, &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
			if _, err := fmt.Sscan(fs.Arg(0), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else if fs.NArg() >= 2 {
			if _, err := fmt.Sscan(fs.Arg(0), &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
			if _, err := fmt.Sscan(fs.Arg(1), &labelID); err != nil {
				return errors.New("label id must be numeric")
			}
		} else {
			return errors.New("usage: ticket label remove -id <ticket-id> <label-id>")
		}
		return svc.RemoveTicketLabel(ticketID, labelID)
	case "show":
		fs := flag.NewFlagSet("label show", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.String("id", "", "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		idStr := *idFlag
		if idStr == "" && fs.NArg() > 0 {
			idStr = fs.Arg(0)
		}
		if idStr == "" {
			return errors.New("usage: ticket label show -id <ticket-id>")
		}
		var ticketID int64
		if _, err := fmt.Sscan(idStr, &ticketID); err != nil {
			return errors.New("ticket id must be numeric")
		}
		labels, err := svc.ListTicketLabels(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(labels)
		}
		if len(labels) == 0 {
			fmt.Println("no labels")
			return nil
		}
		for _, l := range labels {
			fmt.Printf("%d\t%s\n", l.ID, l.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown label command %q; see: ticket label help", args[0])
	}
}

func runTime(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(timeUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "log", "add":
		fs := flag.NewFlagSet("time log", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		ticketID := fs.Int64("id", 0, "ticket id")
		minutes := fs.Int("m", 0, "minutes spent")
		note := fs.String("note", "", "optional note")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *ticketID == 0 || *minutes <= 0 {
			return errors.New("usage: ticket time log -id <ticket-id> -m <minutes> [-note <text>]")
		}
		entry, err := svc.LogTime(*ticketID, libticket.TimeEntryRequest{Minutes: *minutes, Note: *note})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entry)
		}
		fmt.Printf("logged %d min on ticket %d\n", entry.Minutes, entry.TicketID)
		return nil
	case "list", "ls":
		fs := flag.NewFlagSet("time list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.Int64("id", 0, "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID := *idFlag
		if ticketID == 0 && fs.NArg() > 0 {
			if _, err := fmt.Sscan(fs.Arg(0), &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
		}
		if ticketID == 0 {
			return errors.New("usage: ticket time list -id <ticket-id>")
		}
		entries, err := svc.ListTimeEntries(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(entries)
		}
		if len(entries) == 0 {
			fmt.Println("no time entries")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMINUTES\tUSER\tNOTE\tDATE")
		for _, e := range entries {
			fmt.Fprintf(w, "%d\t%d\t%d\t%s\t%s\n", e.ID, e.Minutes, e.UserID, e.Note, e.CreatedAt)
		}
		return w.Flush()
	case "total":
		fs := flag.NewFlagSet("time total", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.Int64("id", 0, "ticket ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ticketID := *idFlag
		if ticketID == 0 && fs.NArg() > 0 {
			if _, err := fmt.Sscan(fs.Arg(0), &ticketID); err != nil {
				return errors.New("ticket id must be numeric")
			}
		}
		if ticketID == 0 {
			return errors.New("usage: ticket time total -id <ticket-id>")
		}
		total, err := svc.TotalTimeForTicket(ticketID)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]int{"total": total})
		}
		hours := total / 60
		mins := total % 60
		if hours > 0 {
			fmt.Printf("%dh %dm (%d min total)\n", hours, mins, total)
		} else {
			fmt.Printf("%d min\n", total)
		}
		return nil
	case "delete":
		fs := flag.NewFlagSet("time delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		idFlag := fs.Int64("id", 0, "time entry ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		id := *idFlag
		if id == 0 && fs.NArg() > 0 {
			if _, err := fmt.Sscan(fs.Arg(0), &id); err != nil {
				return errors.New("time entry id must be numeric")
			}
		}
		if id == 0 {
			return errors.New("usage: ticket time delete -id <entry-id>")
		}
		return svc.DeleteTimeEntry(id)
	default:
		return fmt.Errorf("unknown time command %q; see: ticket time help", args[0])
	}
}

// ---------------------------------------------------------------------------
// tk ticket — ticket namespace
// ---------------------------------------------------------------------------

func runTicketNS(args []string) error {
	if len(args) == 0 {
		return runList(nil)
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(ticketNSUsage)
		return nil

	// List & search
	case "list", "ls":
		return runList(args[1:])
	case "search":
		return runSearch(args[1:])
	case "board":
		return runBoard(args[1:])
	case "count":
		return runCount(args[1:])
	case "orphans":
		return runOrphans(args[1:])

	// Create
	case "add", "create", "new":
		return runTicketCreate(args[1:])

	// View
	case "get", "show":
		return runGet(args[1:])
	case "tree":
		return runGet(args[1:]) // TODO: dedicated tree view

	// Edit (TUI)
	case "edit":
		return runEdit(args[1:])

	// Update
	case "update":
		return runUpdate(args[1:])

	// State
	case "active":
		return runTicketStateAlias(args[1:], store.StateActive, "active")
	case "idle":
		return runTicketStateAlias(args[1:], store.StateIdle, "idle")
	case "complete":
		return runTicketStateAlias(args[1:], store.StateSuccess, "complete")
	case "fail":
		return runTicketStateAlias(args[1:], store.StateFail, "fail")
	case "state":
		return runTicketState(args[1:])

	// Ownership
	case "claim":
		return runClaim(args[1:])
	case "unclaim":
		return runUnclaim(args[1:])
	case "assign":
		return runAssign(args[1:])
	case "unassign":
		return runUnassign(args[1:])
	case "request":
		return runRequest(args[1:])

	// Hierarchy
	case "attach":
		return runSetParent(args[1:], "attach")
	case "detach":
		return runUnsetParent(args[1:], "detach")

	// Comments & history
	case "comment":
		return runComment(args[1:])
	case "history":
		return runHistory(args[1:])
	case "conversation":
		return runConversation(args[1:])

	// Lifecycle
	case "close":
		return runSetTicketClosed(args[1:], true)
	case "open":
		return runSetTicketClosed(args[1:], false)
	case "archive":
		return runSetTicketArchived(args[1:], true)
	case "unarchive":
		return runSetTicketArchived(args[1:], false)
	case "ready":
		return runSetTicketReady(args[1:], true)
	case "notready":
		return runSetTicketReady(args[1:], false)
	case "clone", "cp":
		return runClone(args[1:])
	case "delete", "rm":
		return runDeleteTicket(args[1:])

	// Legacy: agent-based ticket generation
	case "gen":
		return runTicketGen(args[1:])

	default:
		return fmt.Errorf("unknown ticket command %q; see: ticket ticket help", args[0])
	}
}

const ticketNSUsage = `Usage: ticket ticket <command> [flags]

Commands:
  list    [--type T] [--status S] [-u user]   List tickets
  search  "query"                             Full-text search
  board                                       Kanban view
  count                                       Aggregate counts
  orphans                                     Tickets with no parent

  add     "title" [-type T] [-d desc] [-ac criteria]   Create a ticket
  get     -id <id> [-json]                    View ticket detail
  edit    [-id] <id>                          Open TUI editor for ticket
  update  -id <id> [field flags]              Update ticket fields

  active   -id <id>                           Start work
  idle     -id <id>                           Pause work
  complete -id <id>                           Finish stage, advance
  fail     -id <id>                           Mark failed

  claim    -id <id>                           Assign to self
  unclaim  -id <id>                           Unassign self
  assign   -id <id> <user>                    Assign to someone
  unassign -id <id> <user>                    Unassign someone
  request                                     Next available ticket

  attach   -id <id> <parent-id>               Set parent
  detach   -id <id>                           Remove parent

  comment  add -id <id> "text"                Add comment
  history  <id>                               Activity log
  conversation show <id>                      Full thread

  close    -id <id>                           Close ticket
  open     -id <id>                           Reopen ticket
  archive  -id <id>                           Archive
  unarchive -id <id>                          Unarchive
  ready    -id <id>                           Mark ready for work
  notready -id <id>                          Mark not ready
  clone    -id <id>                           Duplicate
  delete   -id <id>                           Delete permanently

  gen      -f <files> -o <output>             Generate tickets via agent`

func runTicketGen(args []string) error {
	fs := flag.NewFlagSet("ticket", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filesArg := fs.String("f", "", "comma-separated input files")
	outputFile := fs.String("o", "", "output file")
	agent := fs.String("agent", "codex", "agent command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*filesArg) == "" || strings.TrimSpace(*outputFile) == "" {
		return errors.New("usage: ticket ticket -f <file1,file2,...> -o <output-file> [-agent <agent>]")
	}

	files := splitCSV(*filesArg)
	if len(files) == 0 {
		return errors.New("at least one input file is required")
	}
	prompt, err := buildTicketPrompt(files, *outputFile)
	if err != nil {
		return err
	}
	response, err := runAgentCommand(strings.TrimSpace(*agent), prompt, false, "")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outputFile, []byte(response), 0o644); err != nil {
		return err
	}
	fmt.Print(response)
	if response != "" && !strings.HasSuffix(response, "\n") {
		fmt.Println()
	}
	if outputJSON {
		return printJSON(map[string]string{
			"status": "ok",
			"agent":  strings.TrimSpace(*agent),
			"output": *outputFile,
		})
	}
	fmt.Printf("wrote %s using %s\n", *outputFile, strings.TrimSpace(*agent))
	return nil
}

func splitCSV(raw string) []string {
	var values []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func buildTicketPrompt(files []string, outputFile string) (string, error) {
	var b strings.Builder
	b.WriteString("Write an example breakdown of implementation requirements as ")
	b.WriteString(outputFile)
	b.WriteString(" in the format:\n\n")
	b.WriteString("EPIC: title\n")
	b.WriteString("ID: E1, E2, E3 etc\n")
	b.WriteString("DESCRIPTION: description\n")
	b.WriteString("AC: list of acceptance criteria\n")
	b.WriteString("PRIORITY: 1-N (1 highest, do this first)\n")
	b.WriteString("DEPENDS-ON: E2, E4\n\n")
	b.WriteString("<indent for stories \"in\" the epic (the story ID should increment and be EPIC-STORY)>\n")
	b.WriteString("    STORY: title\n")
	b.WriteString("    ID: E1-S1, E1-2, E1-S3 etc.\n")
	b.WriteString("    DESCRIPTION: description\n")
	b.WriteString("    AC: list of acceptance criteria\n")
	b.WriteString("    PRIORITY: 1-N (1 highest, do this first)\n")
	b.WriteString("    DEPENDS-ON: E1-S2\n\n")
	b.WriteString("Use the following input files as source material:\n\n")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		b.WriteString("FILE: ")
		b.WriteString(file)
		b.WriteString("\n")
		b.WriteString("-----\n")
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteString("\n")
		}
		b.WriteString("-----\n\n")
	}
	return b.String(), nil
}

// prefixWriter prepends a prefix to each line written.
type prefixWriter struct {
	w      io.Writer
	prefix string
	bol    bool // at beginning of line
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		if pw.bol || !pw.bol && total == len(p) {
			// First write or start of new line: emit prefix.
			if _, err := fmt.Fprint(pw.w, pw.prefix); err != nil {
				return total - len(p), err
			}
			pw.bol = false
		}
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			_, err := pw.w.Write(p)
			return total, err
		}
		if _, err := pw.w.Write(p[:idx+1]); err != nil {
			return total - len(p), err
		}
		p = p[idx+1:]
		if len(p) > 0 {
			pw.bol = true
		}
	}
	return total, nil
}

// prefixReader wraps a reader and echoes what's read with a prefix.
type prefixReader struct {
	r      io.Reader
	prefix string
	w      io.Writer
}

func (pr *prefixReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		fmt.Fprintf(pr.w, "%s%s\n", pr.prefix, strings.TrimRight(string(p[:n]), "\n"))
	}
	return n, err
}

func defaultRunTicketAgentCommand(agent, prompt string, stream bool, ticketKey string) (string, error) {
	if agent == "" {
		return "", errors.New("agent is required")
	}

	// Write the prompt to a file so large prompts don't hit arg-length
	// limits and to work around CLI escaping issues.
	promptFile := ""
	if ticketKey != "" {
		promptFile = fmt.Sprintf("prompt_%s.md", ticketKey)
	} else {
		promptFile = "prompt_agent.md"
	}
	prompt += "\n"
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	var cmd *exec.Cmd
	switch agent {
	case "claude":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("cat %s | claude -p --model claude-sonnet-4-5", promptFile))
	case "codex":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("codex exec < %s", promptFile))
	default:
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s -p < %s", agent, promptFile))
	}
	// Always stream stdout to the terminal so the operator can see
	// the LLM working. With -v, add > / < prefixes.
	var buf bytes.Buffer
	if stream {
		fmt.Printf("> %s\n\n", strings.Join(cmd.Args, " "))
		cmd.Stdout = io.MultiWriter(&prefixWriter{w: os.Stdout, prefix: "< "}, &buf)
		cmd.Stderr = &prefixWriter{w: os.Stderr, prefix: "< "}
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return "", err
	}
	if stream {
		fmt.Println()
	}
	return buf.String(), nil
}

func parseProjectCommandID(raw string) (int64, bool) {
	var id int64
	if _, err := fmt.Sscan(raw, &id); err != nil {
		return 0, false
	}
	return id, true
}

func runProjectByID(svc libticket.Service, projectID int64, args []string) error {
	if len(args) == 0 {
		project, err := svc.GetProject(strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	}
	switch args[0] {
	case "update":
		fs := flag.NewFlagSet("project update", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		title := fs.String("title", "", "project title")
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitBranch := fs.String("git-branch", "", "project git branch")
		status := fs.String("status", "", "project status (open|closed)")
		workflowID := fs.Int64("workflow", 0, "workflow ID to associate with project")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		current, err := svc.GetProject(strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		nextDescription := current.Description
		nextAC := current.AcceptanceCriteria
		nextRepo := current.GitRepository
		nextBranch := current.GitBranch
		nextStatus := current.Status
		if fs.Lookup("description") != nil && strings.TrimSpace(*description) != "" || containsFlag(args[1:], "-description") {
			nextDescription = *description
		}
		if containsFlag(args[1:], "-ac") {
			nextAC = *acceptanceCriteria
		}
		if containsFlag(args[1:], "-git-repository") {
			nextRepo = strings.TrimSpace(*gitRepository)
		}
		if containsFlag(args[1:], "-git-branch") {
			nextBranch = strings.TrimSpace(*gitBranch)
		}
		if containsFlag(args[1:], "-status") && strings.TrimSpace(*status) != "" {
			nextStatus = strings.TrimSpace(*status)
		}
		if nextStatus == "closed" {
			if err := guardProjectClose(svc, projectID); err != nil {
				return err
			}
		}
		var nextWorkflowID *int64
		if containsFlag(args[1:], "-workflow") {
			if *workflowID > 0 {
				nextWorkflowID = workflowID
			}
		} else {
			nextWorkflowID = current.WorkflowID
		}
		project, err := svc.UpdateProject(projectID, libticket.ProjectUpdateRequest{
			Title:              *title,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
			GitRepository:      nextRepo,
			GitBranch:          nextBranch,
			Status:             nextStatus,
			WorkflowID:         nextWorkflowID,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "enable":
		project, err := svc.SetProjectEnabled(projectID, true)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	case "disable":
		if err := guardProjectClose(svc, projectID); err != nil {
			return err
		}
		project, err := svc.SetProjectEnabled(projectID, false)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	default:
		return fmt.Errorf("unknown project command %q; see: ticket project help", args[0])
	}
}

// guardProjectClose returns an error if closing the given project is not allowed.
// A project may not be closed if it is the current project and there are no other
// open projects to switch to.
func guardProjectClose(svc libticket.Service, projectID int64) error {
	cfg, _ := config.Load()
	projects, err := svc.ListProjects()
	if err != nil {
		return err
	}
	// Count open projects and check whether this project is the current one.
	isCurrent := false
	openCount := 0
	for _, p := range projects {
		if p.Status == "open" {
			openCount++
		}
		if p.ID == projectID {
			if strings.EqualFold(p.Prefix, cfg.CurrentProject) || strconv.FormatInt(p.ID, 10) == cfg.CurrentProject {
				isCurrent = true
			}
		}
	}
	if isCurrent && openCount <= 1 {
		return errors.New("cannot close the current project when it is the only open project; create another project or switch to one first")
	}
	if isCurrent {
		return errors.New("cannot close the current project; switch to another project first (tk project use <id>)")
	}
	return nil
}

func containsFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

// resolveIDFlag extracts a ticket ID from either an -id flag value or a
// positional argument. It returns the resolved ID and remaining positional
// args, or an error if neither form provides an ID.
func resolveIDFlag(flagVal string, positional []string) (string, []string, error) {
	idVal := strings.TrimSpace(flagVal)
	if idVal != "" {
		return idVal, positional, nil
	}
	if len(positional) > 0 {
		return positional[0], positional[1:], nil
	}
	return "", nil, errors.New("missing ticket id")
}

func resolveLifecycleInput(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return "", "", nil
	}
	return store.ParseLifecycleStatus(status)
}

// expandListShortFlags expands combined POSIX-style boolean flags for the
// list command. For example: -ad → -a -d, -la → -l -a.
// Only boolean short flags (a, d, l) are expanded. Flags that take values
// (t, u, n) are left as-is to avoid consuming the next argument.
func expandListShortFlags(args []string) []string {
	boolFlags := map[byte]bool{
		'a': true, // include all (closed)
		'd': true, // include archived/deleted
	}
	var expanded []string
	for _, arg := range args {
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
			allBool := true
			for i := 1; i < len(arg); i++ {
				if !boolFlags[arg[i]] {
					allBool = false
					break
				}
			}
			if allBool {
				for i := 1; i < len(arg); i++ {
					expanded = append(expanded, "-"+string(arg[i]))
				}
				continue
			}
		}
		expanded = append(expanded, arg)
	}
	return expanded
}

func runList(args []string) error {
	// Expand combined boolean short flags: -ad → -a -d, -la → -l -a, etc.
	args = expandListShortFlags(args)

	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	taskType := fs.String("type", "", "filter by ticket type")
	fs.StringVar(taskType, "t", "", "filter by ticket type (shorthand)")
	stage := fs.String("stage", "", "filter by ticket stage")
	state := fs.String("state", "", "filter by ticket state")
	status := fs.String("status", "", "filter by rendered ticket status")
	assignee := fs.String("user", "", "filter by assignee")
	fs.StringVar(assignee, "u", "", "filter by assignee")
	limit := fs.Int("n", 0, "maximum number of tickets to return; 0 means all")
	useUnicode := fs.Bool("unicode", true, "render status symbols as unicode")
	plain := fs.Bool("plain", false, "render status as plain text")
	includeAll := fs.Bool("a", false, "include all tickets (closed and archived)")
	includeDeleted := fs.Bool("d", false, "include archived (deleted) tickets")
	labelFilter := fs.String("label", "", "filter by label name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow positional type: tk ls epic  ==  tk ls -type epic
	if *taskType == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		taskType = &v
	}
	if *limit < 0 {
		return errors.New("usage: ticket list|ls [<type>] [-type <type>] [-t <type>] [-stage <stage>] [-state <state>] [-status <stage/state>] [-u <user>] [-n <limit>] [-a] [-label <name>]")
	}
	// -d implies -a (archived tickets are a superset of closed)
	if *includeDeleted {
		*includeAll = true
	}
	statusUnicode := *useUnicode && !*plain
	resolvedStage, resolvedState, err := resolveLifecycleInput(*status, *stage, *state)
	if err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTicketsFiltered(project.ID, *taskType, resolvedStage, resolvedState, "", "", *assignee, *limit, *includeAll)
	if err != nil {
		return err
	}
	if !*includeAll {
		open := tickets[:0]
		for _, t := range tickets {
			if t.Open {
				open = append(open, t)
			}
		}
		tickets = open
	} else if !*includeDeleted {
		// -a without -d: show closed but hide archived
		nonArchived := tickets[:0]
		for _, t := range tickets {
			if !t.Archived {
				nonArchived = append(nonArchived, t)
			}
		}
		tickets = nonArchived
	}
	if *labelFilter != "" {
		filtered := tickets[:0]
		for _, ticket := range tickets {
			labels, err := api.ListTicketLabels(ticket.ID)
			if err != nil {
				return err
			}
			for _, l := range labels {
				if strings.EqualFold(l.Name, *labelFilter) {
					filtered = append(filtered, ticket)
					break
				}
			}
		}
		tickets = filtered
	}
	if len(tickets) == 0 {
		if outputJSON {
			return printJSON(tickets)
		}
		fmt.Printf("no tickets for project %s\n", project.Prefix)
		return nil
	}
	// Build parent key map: ticket ID → parent's key string.
	// Cache lookups so shared parents are only fetched once.
	parentKeys := make(map[int64]string, len(tickets))
	parentCache := make(map[int64]string)
	for _, ticket := range tickets {
		if ticket.ParentID == nil {
			continue
		}
		pid := *ticket.ParentID
		if key, ok := parentCache[pid]; ok {
			parentKeys[ticket.ID] = key
		} else if p, err := api.GetTicket(strconv.FormatInt(pid, 10)); err == nil {
			key := ticketLabel(p)
			parentCache[pid] = key
			parentKeys[ticket.ID] = key
		}
	}
	if outputJSON {
		return printJSON(tickets)
	}
	printTicketTable(tickets, parentKeys, statusUnicode, *includeAll)
	return nil
}

func runBoard(args []string) error {
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	includeArchived := fs.Bool("a", false, "include archived tickets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, *includeArchived)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(tickets)
	}
	var workflowStages []store.WorkflowStage
	if project.WorkflowID != nil {
		if wf, err := api.GetWorkflow(*project.WorkflowID); err == nil {
			workflowStages = wf.Stages
		}
	}
	if len(workflowStages) == 0 {
		fmt.Println("no workflow stages defined for this project")
		return nil
	}

	// Group tickets by stage
	byStage := make(map[string][]store.Ticket)
	for _, t := range tickets {
		byStage[t.Stage] = append(byStage[t.Stage], t)
	}

	// Print each stage as a lane
	for _, ws := range workflowStages {
		stageTickets := byStage[ws.StageName]
		fmt.Printf("── %s (%d) ──\n", strings.ToUpper(ws.StageName), len(stageTickets))
		if len(stageTickets) == 0 {
			fmt.Println("  (empty)")
		}
		for _, t := range stageTickets {
			assignee := t.Assignee
			if strings.TrimSpace(assignee) == "" {
				assignee = "-"
			}
			key := t.Key
			if strings.TrimSpace(key) == "" {
				key = strconv.FormatInt(t.ID, 10)
			}
			stateIcon := ""
			switch t.State {
			case "idle":
				stateIcon = "○"
			case "active":
				stateIcon = "◑"
			case "success":
				stateIcon = "◉"
			case "fail":
				stateIcon = "✗"
			}
			fmt.Printf("  %s %s  %s  [%s]  @%s\n", stateIcon, key, t.Title, t.Type, assignee)
		}
		fmt.Println()
	}
	return nil
}

func runOrphans(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket orphans")
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tickets, err := api.ListTickets(project.ID)
	if err != nil {
		return err
	}
	var orphans []store.Ticket
	for _, ticket := range tickets {
		if ticket.ParentID == nil && strings.TrimSpace(ticket.Type) != "epic" {
			orphans = append(orphans, ticket)
		}
	}
	if outputJSON {
		return printJSON(orphans)
	}
	for _, ticket := range orphans {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(ticket), ticket.Type, ticket.Status, ticket.Title)
	}
	return nil
}

func runGet(args []string) error {
	usage := "ticket get [-id] <id>"
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow positional: tk get FOO is the same as tk get -id FOO
	if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		id = &v
	} else if fs.NArg() != 0 {
		return errors.New("usage: " + usage)
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	dependencies, _ := svc.ListDependencies(ticket.ID)
	history, _ := svc.ListHistory(ticket.ID)
	if outputJSON {
		return printJSON(ticket)
	}
	// Look up workflow stages for progress display
	var workflowStages []store.WorkflowStage
	project, projectErr := svc.GetProject(fmt.Sprintf("%d", ticket.ProjectID))
	if projectErr == nil && project.WorkflowID != nil {
		if wf, err := svc.GetWorkflow(*project.WorkflowID); err == nil {
			workflowStages = wf.Stages
		}
	}
	ticketLabels, _ := svc.ListTicketLabels(ticket.ID)
	totalTime, _ := svc.TotalTimeForTicket(ticket.ID)
	parentKey := ""
	if ticket.ParentID != nil {
		if p, err := svc.GetTicket(fmt.Sprintf("%d", *ticket.ParentID)); err == nil {
			parentKey = ticketLabel(p)
		}
	}
	cloneKey := ""
	if ticket.CloneOf != nil {
		if c, err := svc.GetTicket(fmt.Sprintf("%d", *ticket.CloneOf)); err == nil {
			cloneKey = ticketLabel(c)
		}
	}
	printTicketDetails(ticket, dependencies, history, workflowStages, ticketLabels, totalTime, parentKey, cloneKey)
	// Show children if any
	if projectErr == nil {
		all, _ := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, true)
		var children []store.Ticket
		for _, t := range all {
			if t.ParentID != nil && *t.ParentID == ticket.ID {
				children = append(children, t)
			}
		}
		if len(children) > 0 {
			printTicketChildren(children)
		}
	}
	return nil
}

func runEdit(args []string) error {
	usage := "ticket edit [-id] <id>"
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id or key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" && fs.NArg() == 1 {
		v := fs.Arg(0)
		id = &v
	} else if strings.TrimSpace(*id) == "" && fs.NArg() == 0 {
		// No ID: open the most recently modified ticket in the current project.
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		var project store.Project
		if cfg.CurrentProject != "" {
			project, err = svc.GetProject(cfg.CurrentProject)
			if err != nil {
				return err
			}
		} else {
			return errors.New(usage)
		}
		tickets, err := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		if len(tickets) == 0 {
			return errors.New("no tickets in project")
		}
		// Find most recently updated ticket.
		latest := tickets[0]
		for _, t := range tickets[1:] {
			if t.UpdatedAt > latest.UpdatedAt {
				latest = t
			}
		}
		return tui.RunEdit(svc, cfg, project, latest)
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	var project store.Project
	if cfg.CurrentProject != "" {
		project, _ = svc.GetProject(cfg.CurrentProject)
	}
	return tui.RunEdit(svc, cfg, project, ticket)
}

func runSearch(args []string) error {
	query, filters, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	if query == "" {
		return errors.New("usage: ticket search <free form query> [-status <status>] [-title <text>] [-description <text>] [-priority <n>] [-owner <user>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	var projects []store.Project
	if filters.allProjects {
		projects, err = svc.ListProjects()
		if err != nil {
			return err
		}
	} else {
		_, _, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		projects = []store.Project{project}
	}
	var tickets []store.Ticket
	for _, project := range projects {
		projectTasks, err := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		for _, ticket := range projectTasks {
			if !ticketMatchesSearch(ticket, query, filters.stage, filters.state, filters.status, filters.title, filters.description, filters.priority, filters.owner) {
				continue
			}
			tickets = append(tickets, ticket)
		}
	}
	if outputJSON {
		return printJSON(tickets)
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(ticket), ticket.Type, ticket.Status, ticket.Title)
	}
	return nil
}

type searchFilters struct {
	stage       string
	state       string
	status      string
	title       string
	description string
	priority    int
	owner       string
	allProjects bool
}

func parseSearchArgs(args []string) (string, searchFilters, error) {
	var filters searchFilters
	var terms []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-stage":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -stage requires a value")
			}
			filters.stage = args[i+1]
			i++
		case "-state":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -state requires a value")
			}
			filters.state = args[i+1]
			i++
		case "-status":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -status requires a value")
			}
			filters.status = args[i+1]
			i++
		case "-title":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -title requires a value")
			}
			filters.title = args[i+1]
			i++
		case "-description":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -description requires a value")
			}
			filters.description = args[i+1]
			i++
		case "-priority":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -priority requires a value")
			}
			priority, err := strconv.Atoi(args[i+1])
			if err != nil {
				return "", filters, errors.New("search priority must be numeric")
			}
			filters.priority = priority
			i++
		case "-owner":
			if i+1 >= len(args) {
				return "", filters, errors.New("search flag -owner requires a value")
			}
			filters.owner = args[i+1]
			i++
		case "-allprojects":
			filters.allProjects = true
		default:
			terms = append(terms, args[i])
		}
	}
	return strings.TrimSpace(strings.Join(terms, " ")), filters, nil
}

func ticketMatchesSearch(ticket store.Ticket, query, stage, state, status, title, description string, priority int, owner string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			ticket.Title,
			ticket.Description,
			ticket.AcceptanceCriteria,
			ticket.Assignee,
			ticket.Status,
			strconv.Itoa(ticket.Priority),
		}, "\n"))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		stageFilter, stateFilter, err := resolveLifecycleInput(trimmed, "", "")
		if err == nil {
			if ticket.Stage != strings.TrimSpace(stageFilter) || ticket.State != strings.TrimSpace(stateFilter) {
				return false
			}
		} else if ticket.Status != trimmed {
			return false
		}
	}
	if trimmed := strings.TrimSpace(stage); trimmed != "" && ticket.Stage != trimmed {
		return false
	}
	if trimmed := strings.TrimSpace(state); trimmed != "" && ticket.State != trimmed {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(title)); trimmed != "" && !strings.Contains(strings.ToLower(ticket.Title), trimmed) {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(description)); trimmed != "" {
		descriptionFields := strings.ToLower(ticket.Description + "\n" + ticket.AcceptanceCriteria)
		if !strings.Contains(descriptionFields, trimmed) {
			return false
		}
	}
	if priority != 0 && ticket.Priority != priority {
		return false
	}
	if trimmed := strings.TrimSpace(owner); trimmed != "" && ticket.Assignee != trimmed {
		return false
	}
	return true
}

func runSetParent(args []string, command string) error {
	usage := fmt.Sprintf("ticket %s [-id] <id> <parent-id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	child, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	parent, err := svc.GetTicket(rest[0])
	if err != nil {
		return err
	}
	updated, err := svc.SetTicketParent(child.ID, parent.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runUnsetParent(args []string, command string) error {
	usage := fmt.Sprintf("ticket %s [-id] <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	updated, err := svc.UnsetTicketParent(ticket.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runTicketStateAlias(args []string, state, command string) error {
	fs := flag.NewFlagSet("ticket "+command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("usage: ticket %s [-id] <id>", command)
	}
	return updateTicketState(idVal, state)
}

func runTicketState(args []string) error {
	usage := "ticket state <id> <idle|active|success|fail|design|develop|test|done>"
	fs := flag.NewFlagSet("ticket state", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	var stateArg string
	switch {
	case idVal != "" && fs.NArg() == 1:
		stateArg = fs.Args()[0]
	case idVal == "" && fs.NArg() == 2:
		idVal = fs.Args()[0]
		stateArg = fs.Args()[1]
	default:
		return errors.New("usage: " + usage)
	}
	normalized := strings.ToLower(strings.TrimSpace(stateArg))
	switch {
	case store.ValidState(normalized):
		return updateTicketState(idVal, normalized)
	case store.ValidStage(normalized):
		return updateTicketStage(idVal, normalized)
	default:
		return errors.New("usage: " + usage)
	}
}

func updateTicketStage(idArg, stage string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(idArg)
	if err != nil {
		return err
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Stage:              stage,
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
	printTicket(updated)
	return nil
}

func updateTicketState(idArg, state string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(idArg)
	if err != nil {
		return err
	}
	return updateTicketLifecycleRequest(svc, current.ID, current, state)
}

func updateTicketLifecycleRequest(svc libticket.Service, id int64, current store.Ticket, state string) error {
	assignee := current.Assignee
	if state == store.StateActive && strings.TrimSpace(assignee) == "" {
		status, err := svc.Status()
		if err == nil && status.User != nil && strings.TrimSpace(status.User.Username) != "" {
			assignee = status.User.Username
		} else {
			assignee = fallbackCommandUsername()
		}
	}
	updated, err := svc.UpdateTicket(id, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           assignee,
		State:              state,
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
	printTicket(updated)
	return nil
}

func runUpdate(args []string) error {
	usage := "ticket update -id <id>\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-ac <acceptance-criteria>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]"
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	title := fs.String("title", "", "ticket title")
	description := fs.String("description", "", "ticket description")
	desc := fs.String("desc", "", "ticket description")
	acceptanceCriteria := fs.String("ac", "", "ticket acceptance criteria")
	gitRepository := fs.String("git-repository", "", "ticket git repository")
	gitBranch := fs.String("git-branch", "", "ticket git branch")
	priority := fs.Int("priority", 0, "ticket priority")
	order := fs.Int("order", 0, "ticket order")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	status := fs.String("status", "", "rendered ticket status (<stage>/<state>)")
	state := fs.String("state", "", "ticket state")
	parentIDRaw := fs.String("parent_id", "", "ticket parent id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	if fs.NArg() != 0 {
		return errors.New("usage: " + usage)
	}
	hasTitle := containsFlag(args, "-title")
	hasDescription := containsFlag(args, "-description")
	hasDesc := containsFlag(args, "-desc")
	hasAC := containsFlag(args, "-ac")
	hasPriority := containsFlag(args, "-priority")
	hasGitRepository := containsFlag(args, "-git-repository")
	hasGitBranch := containsFlag(args, "-git-branch")
	hasOrder := containsFlag(args, "-order")
	hasEstimateEffort := containsFlag(args, "-estimate_effort")
	hasEstimateComplete := containsFlag(args, "-estimate_complete")
	hasStatus := containsFlag(args, "-status")
	hasState := containsFlag(args, "-state")
	hasParentID := containsFlag(args, "-parent_id")
	if !hasTitle && !hasDescription && !hasDesc && !hasAC && !hasGitRepository && !hasGitBranch && !hasPriority && !hasOrder && !hasEstimateEffort && !hasEstimateComplete && !hasStatus && !hasState && !hasParentID {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	next := libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		GitRepository:      current.GitRepository,
		GitBranch:          current.GitBranch,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	}
	if hasTitle {
		next.Title = *title
	}
	if hasDescription {
		next.Description = *description
	}
	if hasDesc {
		next.Description = *desc
	}
	if hasAC {
		next.AcceptanceCriteria = *acceptanceCriteria
	}
	if hasGitRepository {
		next.GitRepository = strings.TrimSpace(*gitRepository)
	}
	if hasGitBranch {
		next.GitBranch = strings.TrimSpace(*gitBranch)
	}
	if hasPriority {
		next.Priority = *priority
	}
	if hasOrder {
		next.Order = *order
	}
	if hasEstimateEffort {
		next.EstimateEffort = *estimateEffort
	}
	if hasEstimateComplete {
		next.EstimateComplete = *estimateComplete
	}
	if hasStatus {
		_, resolvedState, err := resolveLifecycleInput(*status, "", "")
		if err != nil {
			return err
		}
		next.State = resolvedState
	}
	if hasState {
		next.State = *state
	}
	if strings.TrimSpace(next.State) == store.StateActive && strings.TrimSpace(next.Assignee) == "" {
		status, err := svc.Status()
		if err == nil && status.User != nil && strings.TrimSpace(status.User.Username) != "" {
			next.Assignee = status.User.Username
		} else {
			next.Assignee = fallbackCommandUsername()
		}
	}
	if hasParentID {
		parent, err := svc.GetTicket(strings.TrimSpace(*parentIDRaw))
		if err != nil {
			return err
		}
		next.ParentID = &parent.ID
	}
	updated, err := svc.UpdateTicket(current.ID, next)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	dependencies, _ := svc.ListDependencies(current.ID)
	history, _ := svc.ListHistory(updated.ID)
	parentKey := ""
	if updated.ParentID != nil {
		if p, err := svc.GetTicket(fmt.Sprintf("%d", *updated.ParentID)); err == nil {
			parentKey = ticketLabel(p)
		}
	}
	printTicketDetails(updated, dependencies, history, nil, nil, 0, parentKey, "")
	return nil
}

func runAssign(args []string) error {
	fs := flag.NewFlagSet("assign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: ticket assign [-id] <id> <name>")
	}
	return assignTicket(idVal, rest[0], true)
}

func runUnassign(args []string) error {
	fs := flag.NewFlagSet("unassign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 1 {
		return errors.New("usage: ticket unassign [-id] <id> <name>")
	}
	return unassignTicket(idVal, rest[0], true)
}

func runClaim(args []string) error {
	rewritten := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-id":
			if i+1 >= len(args) {
				return errors.New("usage: ticket claim [-id <id>] [-dry-run]")
			}
			rewritten = append(rewritten, args[i+1])
			i++
		case "-dry-run":
			rewritten = append(rewritten, "--dryrun")
		default:
			rewritten = append(rewritten, args[i])
		}
	}
	return runRequest(rewritten)
}

func runUnclaim(args []string) error {
	fs := flag.NewFlagSet("unclaim", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: ticket unclaim [-id] <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	username := strings.TrimSpace(cfg.Username)
	if resolved, rErr := config.ResolveURL(); rErr == nil && resolved.Mode == config.ModeLocal {
		username = localModeUsername()
	}
	if strings.TrimSpace(username) == "" {
		return errors.New("no current username; log in first")
	}
	return unassignTicket(idVal, username, false)
}

func assignTicket(idArg, assignee string, requireAdmin bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	if err != nil {
		return err
	}
	if requireAdmin && (status.User == nil || status.User.Role != "admin") {
		return errors.New("user is not an admin")
	}
	current, err := svc.GetTicket(idArg)
	if err != nil {
		return err
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           assignee,
		State:              current.State,
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
	if strings.TrimSpace(updated.Assignee) == "" {
		fmt.Printf("unassigned %s\n", ticketLabel(updated))
		return nil
	}
	fmt.Printf("assigned %s to %s\n", ticketLabel(updated), updated.Assignee)
	return nil
}

func unassignTicket(idArg, expectedAssignee string, requireAdmin bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	if err != nil {
		return err
	}
	if requireAdmin && (status.User == nil || status.User.Role != "admin") {
		return errors.New("user is not an admin")
	}
	if requireAdmin {
		users, err := svc.ListUsers()
		if err != nil {
			return err
		}
		var found bool
		for _, user := range users {
			if user.Username == expectedAssignee {
				found = true
				if !user.Enabled {
					return errors.New("user is disabled")
				}
				break
			}
		}
		if !found {
			return errors.New("user not found")
		}
	}
	current, err := svc.GetTicket(idArg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current.Assignee) != strings.TrimSpace(expectedAssignee) {
		return fmt.Errorf("ticket is not assigned to %s", expectedAssignee)
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
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	limit := fs.Int("n", 10, "maximum number of events to show; 0 means all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	// Merge -id flag into remaining for uniform handling below.
	if strings.TrimSpace(*id) != "" {
		remaining = append([]string{strings.TrimSpace(*id)}, remaining...)
	}

	// No positional args: show recent events for the active project.
	if len(remaining) == 0 {
		_, svc, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		events, err := svc.ListProjectHistory(project.ID, *limit)
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
			key := event.TicketKey
			if key == "" {
				key = fmt.Sprintf("#%d", event.TicketID)
			}
			fmt.Printf("[%s] %-10s %s\n", event.CreatedAt, key, formatHistoryEvent(event))
		}
		return nil
	}

	if len(remaining) != 1 {
		return errors.New("usage: ticket history [-n <limit>] [<id>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(remaining[0])
	if err != nil {
		return err
	}
	events, err := svc.ListHistory(ticket.ID)
	if err != nil {
		return err
	}
	// Apply limit for per-ticket history too
	if *limit > 0 && len(events) > *limit {
		events = events[len(events)-*limit:]
	}
	if outputJSON {
		return printJSON(events)
	}
	if len(events) == 0 {
		fmt.Println("no history")
		return nil
	}
	for _, event := range events {
		fmt.Printf("[%s] %s\n", event.CreatedAt, formatHistoryEvent(event))
	}
	return nil
}

func runHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, _, err := resolveIDFlag(*id, fs.Args())
	if err != nil {
		return errors.New("usage: ticket health [-id] <id>|execute")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if strings.EqualFold(idVal, "execute") {
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		projectTickets, err := api.ListTickets(project.ID)
		if err != nil {
			return err
		}

		results := make([]map[string]any, 0, len(projectTickets))
		for _, ticket := range projectTickets {
			comments, err := svc.ListComments(ticket.ID)
			if err != nil {
				return err
			}
			checks := ticketHealthCheck(ticket, comments)
			updated, err := svc.SetTicketHealth(ticket.ID, checks.score)
			if err != nil {
				return err
			}
			result := map[string]any{
				"ticket_id":                  ticket.ID,
				"ticket_key":                 ticket.Key,
				"score":                      checks.score,
				"not_an_orphan":              checks.notOrphan,
				"has_acceptance_criteria":    checks.hasAC,
				"reviewed_by_reviewer_agent": checks.reviewedByReviewer,
				"definition_of_ready":        checks.ready,
				"persisted_score":            updated.HealthScore,
			}
			results = append(results, result)
		}

		if outputJSON {
			return printJSON(map[string]any{
				"ticket_health_execute": map[string]any{
					"tickets": len(results),
					"results": results,
				},
			})
		}

		fmt.Println("TICKET HEALTH EXECUTE")
		fmt.Printf("tickets: %d\n", len(results))
		for _, result := range results {
			label := fmt.Sprintf("%v", result["ticket_id"])
			if key, ok := result["ticket_key"].(string); ok && key != "" {
				label = key
			}
			if score, ok := result["score"].(int); ok {
				fmt.Printf("%s\t%.2f\n", label, float64(score)/4.0)
			} else {
				fmt.Printf("%s\t%s\n", label, fmt.Sprintf("%v", result["score"]))
			}
		}
		return nil
	}

	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	comments, err := svc.ListComments(ticket.ID)
	if err != nil {
		return err
	}

	checks := ticketHealthCheck(ticket, comments)
	updated, err := svc.SetTicketHealth(ticket.ID, checks.score)
	if err != nil {
		return err
	}
	section := map[string]any{
		"score":                      checks.score,
		"not_an_orphan":              checks.notOrphan,
		"has_acceptance_criteria":    checks.hasAC,
		"reviewed_by_reviewer_agent": checks.reviewedByReviewer,
		"definition_of_ready":        checks.ready,
	}
	if outputJSON {
		return printJSON(map[string]any{
			"ticket_health": section,
			"ticket": map[string]any{
				"ticket_id":    ticket.ID,
				"ticket_key":   ticketLabel(ticket),
				"health_score": updated.HealthScore,
			},
		})
	}
	fmt.Println("TICKET HEALTH")
	fmt.Printf("score: %.2f\n", float64(checks.score)/4.0)
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

func ticketHealthCheck(ticket store.Ticket, comments []store.Comment) ticketHealthResult {
	notOrphan := ticket.Type == "epic" || ticket.ParentID != nil
	hasAC := strings.TrimSpace(ticket.AcceptanceCriteria) != ""
	reviewedByReviewer := hasReviewerAgentComment(comments)
	ready := ticket.Status == "design/idle"
	if !ready {
		stage, state, err := store.ParseLifecycleStatus(ticket.Status)
		if err == nil {
			ready = stage == store.StageDesign && state == store.StateIdle
		}
	}
	checks := []bool{notOrphan, hasAC, reviewedByReviewer, ready}
	score := 0
	for _, ok := range checks {
		if ok {
			score++
		}
	}
	return ticketHealthResult{
		score:              score,
		notOrphan:          notOrphan,
		hasAC:              hasAC,
		reviewedByReviewer: reviewedByReviewer,
		ready:              ready,
	}
}

func hasReviewerAgentComment(comments []store.Comment) bool {
	for _, comment := range comments {
		if isReviewerAuthor(comment.Author) || isReviewerCommentText(comment.Text) {
			return true
		}
	}
	return false
}

func isReviewerAuthor(author string) bool {
	a := strings.ToLower(strings.TrimSpace(author))
	return strings.Contains(a, "reviewer")
}

func isReviewerCommentText(commentText string) bool {
	text := strings.ToLower(strings.TrimSpace(commentText))
	for _, term := range []string{"reviewer", "reviewed", "approved", "approval"} {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func runDependencyCommand(args []string, add bool) error {
	command := "add-dependency"
	if !add {
		command = "remove-dependency"
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: ticket %s <id> <dependency-id[,dependency-id...]>", command)
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	ticket, err := api.GetTicket(args[0])
	if err != nil {
		return err
	}
	dependencyRefs := strings.Split(args[1], ",")
	if len(dependencyRefs) == 0 {
		return errors.New("at least one dependency id is required")
	}
	for _, depRef := range dependencyRefs {
		dependencyTicket, err := api.GetTicket(strings.TrimSpace(depRef))
		if err != nil {
			return err
		}
		dependencyRequest := libticket.DependencyRequest{
			ProjectID: project.ID,
			TicketID:  ticket.ID,
			DependsOn: dependencyTicket.ID,
		}
		if add {
			if _, err := api.AddDependency(dependencyRequest); err != nil {
				return err
			}
			continue
		}
		if err := api.RemoveDependency(dependencyRequest); err != nil {
			return err
		}
	}
	if outputJSON {
		return printJSON(map[string]any{
			"task_id":      ticket.ID,
			"dependencies": args[1],
			"action":       map[bool]string{true: "added", false: "removed"}[add],
		})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Printf("%s dependencies for %s: %s\n", action, ticketLabel(ticket), args[1])
	return nil
}

func runDependency(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(depUsage)
		return nil
	}
	depUsageErr := "usage: ticket dep <add|remove> -id <id> <dependency-id>"
	action := args[0]
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("dependency add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
			return errors.New(depUsageErr)
		}
		return runDependencyCommand([]string{strings.TrimSpace(*id), strings.TrimSpace(fs.Args()[0])}, true)
	case "remove":
		fs := flag.NewFlagSet("dependency remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
			return errors.New(depUsageErr)
		}
		return runDependencyCommand([]string{strings.TrimSpace(*id), strings.TrimSpace(fs.Args()[0])}, false)
	default:
		if action == "" {
			return errors.New(depUsageErr)
		}
		return fmt.Errorf("unknown dep command %q; see: ticket dep help", action)
	}
}

func runRequest(args []string) error {
	dryRun := false
	var requestedRef string
	for _, arg := range args {
		switch arg {
		case "--dryrun", "-dryrun":
			dryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("usage: ticket request [--dryrun] [<id>]")
			}
			if requestedRef != "" {
				return fmt.Errorf("usage: ticket request [--dryrun] [<id>]")
			}
			requestedRef = arg
		}
	}
	if dryRun {
		if requestedRef == "" {
			return runRequestDryRun(nil)
		}
		return runRequestDryRun([]string{requestedRef})
	}

	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	taskRequest := libticket.TicketRequest{ProjectID: project.ID}
	if requestedRef != "" {
		taskRequest.TicketRef = requestedRef
	}
	response, err := api.RequestTicket(taskRequest)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(response)
	}
	if response.Ticket != nil {
		printTicket(*response.Ticket)
		return nil
	}
	fmt.Println(response.Status)
	return nil
}

func runRequestDryRun(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: ticket request-dryrun [<id>]")
	}

	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}

	var requestedRef string
	if len(args) == 1 {
		requestedRef = args[0]
	}

	response, err := api.RequestTicket(libticket.TicketRequest{
		ProjectID: project.ID,
		TicketRef: requestedRef,
		DryRun:    true,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{
			"dryrun": true,
			"status": response.Status,
			"task":   response.Ticket,
		})
	}
	fmt.Printf("dry run: %s\n", response.Status)
	if response.Ticket == nil {
		return nil
	}
	fmt.Printf("would assign ticket: %s\n", ticketLabel(*response.Ticket))
	printTicket(*response.Ticket)
	return nil
}

func parseIDList(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return nil, errors.New("at least one dependency id is required")
	}
	var ids []int64
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("dependency ids must be numeric")
		}
		var id int64
		if _, err := fmt.Sscan(part, &id); err != nil {
			return nil, errors.New("dependency ids must be numeric")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func runComment(args []string) error {
	usage := "ticket comment <id> \"comment\""
	if len(args) == 0 {
		return errors.New("usage: " + usage)
	}
	// Support "add" subcommand for backwards compatibility
	addArgs := args
	if args[0] == "add" {
		addArgs = args[1:]
	} else if args[0] == "help" || args[0] == "-help" || args[0] == "--help" {
		fmt.Println("usage: " + usage)
		return nil
	}
	fs := flag.NewFlagSet("ticket comment", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(addArgs); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	var commentText string
	switch {
	case idVal != "" && len(fs.Args()) == 1:
		commentText = fs.Args()[0]
	case idVal == "" && len(fs.Args()) == 2:
		idVal = fs.Args()[0]
		commentText = fs.Args()[1]
	default:
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	comment, err := svc.AddComment(ticket.ID, commentText)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(comment)
	}
	fmt.Printf("commented on %s: %s\n", ticketLabel(ticket), comment.Comment)
	return nil
}

func runClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: ticket clone|cp [-id] <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	taskRef, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	ticket, err := svc.CloneTicket(taskRef.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	printTicket(ticket)
	fmt.Printf("clone_of: %s\n", ticketLabel(taskRef))
	return nil
}

func runDeleteTicket(args []string) error {
	usage := "ticket rm|delete [-id] <id>"
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	if err := svc.DeleteTicket(ticket.ID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "ticket_id": ticket.ID, "key": ticket.Key})
	}
	fmt.Printf("deleted ticket %s\n", ticketLabel(ticket))
	return nil
}

func runSetTicketClosed(args []string, closed bool) error {
	command := "open"
	if closed {
		command = "close"
	}
	usage := fmt.Sprintf("ticket %s <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal := strings.TrimSpace(*id)
	switch {
	case idVal != "" && fs.NArg() == 0:
		// -id flag form
	case idVal == "" && fs.NArg() == 1:
		idVal = fs.Args()[0]
	default:
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if closed {
		updated, err = svc.CloseTicket(ticket.ID)
	} else {
		updated, err = svc.OpenTicket(ticket.ID)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runSetTicketArchived(args []string, archived bool) error {
	command := "unarchive"
	if archived {
		command = "archive"
	}
	usage := fmt.Sprintf("ticket %s [-id] <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if archived {
		updated, err = svc.ArchiveTicket(ticket.ID)
	} else {
		updated, err = svc.UnarchiveTicket(ticket.ID)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runSetTicketReady(args []string, ready bool) error {
	command := "notready"
	if ready {
		command = "ready"
	}
	usage := fmt.Sprintf("ticket %s [-id] <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idVal, rest, err := resolveIDFlag(*id, fs.Args())
	if err != nil || len(rest) != 0 {
		return errors.New("usage: " + usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	ticket, err := svc.GetTicket(idVal)
	if err != nil {
		return err
	}
	var updated store.Ticket
	if ready {
		updated, err = svc.ReadyTicket(ticket.ID)
	} else {
		updated, err = svc.NotReadyTicket(ticket.ID)
	}
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runCurate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ticket curate <id> [id...]")
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	var sourceIDs []int64
	var titles []string
	for _, arg := range args {
		ticket, err := api.GetTicket(arg)
		if err != nil {
			return err
		}
		sourceIDs = append(sourceIDs, ticket.ID)
		titles = append(titles, ticket.Title)
	}
	title := "Curated requirement"
	if len(titles) > 0 {
		title = titles[0]
	}
	requirement, err := api.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:   project.ID,
		Type:        "requirement",
		Title:       title,
		Description: "Curated from source items.",
	})
	if err != nil {
		return err
	}
	printTicket(requirement)
	return nil
}

// reviewStatusFilter holds stage/state filters for review vocabulary.
// "proposed" = design/idle, "accepted" = moved past design (develop stage), "rejected" = design/fail
type reviewFilter struct{ stage, state string }

var reviewStatusFilters = map[string]reviewFilter{
	"proposed": {"design", store.StateIdle},
	"accepted": {store.StageDevelop, ""},
	"rejected": {"design", store.StateFail},
}

func runReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	status := fs.String("status", "", "filter by review status: proposed, accepted, rejected")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	var stageFilter, stateFilter string
	if *status != "" {
		f, ok := reviewStatusFilters[strings.ToLower(strings.TrimSpace(*status))]
		if !ok {
			return fmt.Errorf("unknown review status %q; use proposed, accepted, or rejected", *status)
		}
		stageFilter = f.stage
		stateFilter = f.state
	}
	tickets, err := api.ListTicketsFiltered(project.ID, "requirement", stageFilter, stateFilter, "", "", "", 0, false)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
	}
	return nil
}

func runRequirementStatus(reviewStatus string, args []string) error {
	commandName := map[string]string{"accepted": "accept", "rejected": "reject"}[reviewStatus]
	if len(args) != 2 || args[0] != "requirement" {
		return fmt.Errorf("usage: ticket %s requirement <id>", commandName)
	}
	stateToSet := map[string]string{"accepted": store.StateSuccess, "rejected": store.StateFail}[reviewStatus]
	return updateTicketState(args[1], stateToSet)
}

func runRevise(args []string) error {
	if len(args) != 2 || args[0] != "requirement" {
		return errors.New("usage: ticket revise requirement <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(args[1])
	if err != nil {
		return err
	}
	updated, err := svc.UpdateTicket(current.ID, libticket.TicketUpdateRequest{
		Title:              current.Title + " (revised)",
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
	if err != nil {
		return err
	}
	printTicket(updated)
	return nil
}

// ---------------------------------------------------------------------------
// tk req — requirements namespace
// ---------------------------------------------------------------------------

func runReq(args []string) error {
	if len(args) == 0 {
		return runReqList(nil)
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(reqUsage)
		return nil
	case "add":
		return runReqAdd(args[1:])
	case "list", "ls":
		return runReqList(args[1:])
	case "get":
		return runReqGet(args[1:])
	case "shape":
		return runReqShape(args[1:])
	case "accept":
		return runReqAcceptReject("accept", args[1:])
	case "reject":
		return runReqAcceptReject("reject", args[1:])
	case "revise":
		return runReqRevise(args[1:])
	case "break":
		return runReqBreak(args[1:])
	case "pin":
		return runReqPin(args[1:])
	default:
		return fmt.Errorf("unknown req command %q; see: ticket req help", args[0])
	}
}

const reqUsage = `Usage: ticket req <command> [flags]

Commands:
  add    "title" [-d description] [-ac criteria]   Capture a new requirement
  list   [-status raw|shaping|accepted|rejected]    List requirements
  get    -id <id>                                   View requirement detail
  shape  -id <id> [-d text] [-ac text]              Refine a requirement
  break  -id <id> [--retry] [--reset]              Show/manage breakdown
  accept -id <id>                                   Approve a requirement
  reject -id <id>                                   Reject a requirement
  revise -id <id>                                   Send back for rethinking

Shortcuts:
  tk idea "title"    →  tk req add "title"
  tk ideas           →  tk req list`

func runReqAdd(args []string) error {
	return runTicketCreate(append([]string{"-type", "requirement"}, args...))
}

func runReqList(args []string) error {
	return runReview(args)
}

func runReqGet(args []string) error {
	return runGet(args)
}

func runReqShape(args []string) error {
	fs := flag.NewFlagSet("req shape", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	desc := fs.String("d", "", "description")
	ac := fs.String("ac", "", "acceptance criteria")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: ticket req shape -id <id> [-d description] [-ac criteria]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(*id)
	if err != nil {
		return err
	}
	if current.Type != "requirement" {
		return fmt.Errorf("%s is a %s, not a requirement", current.Key, current.Type)
	}
	update := libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	}
	if *desc != "" {
		update.Description = *desc
	}
	if *ac != "" {
		update.AcceptanceCriteria = *ac
	}
	updated, err := svc.UpdateTicket(current.ID, update)
	if err != nil {
		return err
	}
	printTicket(updated)
	return nil
}

func runReqAcceptReject(verb string, args []string) error {
	fs := flag.NewFlagSet("req "+verb, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("usage: ticket req %s -id <id>", verb)
	}
	stateToSet := map[string]string{"accept": store.StateSuccess, "reject": store.StateFail}[verb]
	return updateTicketState(*id, stateToSet)
}

func runReqRevise(args []string) error {
	fs := flag.NewFlagSet("req revise", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: ticket req revise -id <id>")
	}
	return runRevise([]string{"requirement", *id})
}

func runReqBreak(args []string) error {
	fs := flag.NewFlagSet("req break", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "requirement ID")
	retry := fs.Bool("retry", false, "regenerate breakdown, keeping pinned items")
	reset := fs.Bool("reset", false, "discard all children and regenerate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("usage: ticket req break -id <id> [--retry] [--reset]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	req, err := svc.GetTicket(*id)
	if err != nil {
		return err
	}
	if req.Type != "requirement" {
		return fmt.Errorf("%s is a %s, not a requirement", req.Key, req.Type)
	}

	// List all tickets in the project, filter to children of this requirement.
	tickets, err := svc.ListTicketsFiltered(req.ProjectID, "", "", "", "", "", "", 0, false)
	if err != nil {
		return err
	}
	var children []store.Ticket
	for _, t := range tickets {
		if t.ParentID != nil && *t.ParentID == req.ID {
			children = append(children, t)
		}
	}

	if *reset {
		// Delete all unpinned children — for now, delete all (pin not yet tracked).
		for _, child := range children {
			if err := svc.DeleteTicket(child.ID); err != nil {
				return fmt.Errorf("failed to delete %s: %w", child.Key, err)
			}
			fmt.Printf("deleted %s: %s\n", child.Key, child.Title)
		}
		children = nil
	}

	_ = *retry // retry keeps pinned items; without pin tracking, behaves like showing current state

	if len(children) == 0 {
		fmt.Printf("no breakdown items for %s\n", req.Key)
		fmt.Println("hint: create child tickets with `tk add -parent <id> \"title\"` then re-run `tk req break -id <id>`")
		return nil
	}

	if outputJSON {
		return printJSON(children)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Breakdown of %s: %s\n\n", req.Key, req.Title)
	fmt.Fprintln(w, "KEY\tTYPE\tSTATUS\tTITLE")
	for _, child := range children {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", child.Key, child.Type, child.Status, child.Title)
	}
	return w.Flush()
}

func runReqPin(args []string) error {
	return errors.New("req pin is not yet implemented; planned for a future release")
}

func runDecision(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(decisionUsage)
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) != 2 {
			return errors.New("usage: ticket decision add \"text\"")
		}
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		ticket, err := api.CreateTicket(libticket.TicketCreateRequest{
			ProjectID:   project.ID,
			Type:        "decision",
			Title:       args[1],
			Description: args[1],
		})
		if err != nil {
			return err
		}
		printTicket(ticket)
		return nil
	case "list":
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		tickets, err := api.ListTicketsFiltered(project.ID, "decision", "", "", "", "", "", 0, false)
		if err != nil {
			return err
		}
		for _, ticket := range tickets {
			fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
		}
		return nil
	default:
		return fmt.Errorf("unknown decision command %q; see: ticket decision help", args[0])
	}
}

func runConversation(args []string) error {
	if len(args) < 2 || args[0] != "show" {
		return errors.New("usage: ticket conversation show [-id] <id>")
	}
	return runHistory(args[1:])
}

func runTypedTicketCreate(ticketType string, args []string) error {
	return runTicketCreate(append([]string{"-type", ticketType}, args...))
}

type ticketCreateOptions struct {
	TicketType         string
	Title              string
	Description        string
	AcceptanceCriteria string
	GitRepository      string
	GitBranch          string
	Priority           int
	EstimateEffort     int
	EstimateComplete   string
	Assignee           string
	ParentID           *int64
	Project            string
}

func runTicketCreate(args []string) error {
	normalizedArgs, err := normalizeTicketCreateArgs(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	taskType := fs.String("type", "task", "ticket type")
	fs.StringVar(taskType, "t", "task", "ticket type")
	titleFlag := fs.String("title", "", "ticket title")
	priority := fs.Int("priority", 1, "ticket priority")
	fs.IntVar(priority, "p", 1, "ticket priority")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	assignee := fs.String("assignee", "", "ticket assignee")
	fs.StringVar(assignee, "a", "", "ticket assignee")
	description := fs.String("description", "", "ticket description")
	fs.StringVar(description, "d", "", "ticket description")
	acceptanceCriteria := fs.String("ac", "", "acceptance criteria")
	gitRepository := fs.String("git-repository", "", "ticket git repository")
	gitBranch := fs.String("git-branch", "", "ticket git branch")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	parent := fs.Int64("parent", 0, "parent ticket id")
	project := fs.String("project", "", "project id")
	if err := fs.Parse(normalizedArgs); err != nil {
		return err
	}
	title := strings.TrimSpace(*titleFlag)
	if title == "" {
		title = strings.Join(fs.Args(), " ")
	}
	if title == "" {
		return errors.New("usage: ticket add|create|new [-title title] [-t type] [-p priority] [-a assignee] [-d description] [-ac criteria] [-parent id] [-project project] [-estimate_effort n] [-estimate_complete rfc3339] [title words]")
	}
	opts := ticketCreateOptions{
		TicketType:         *taskType,
		Title:              title,
		Description:        *description,
		AcceptanceCriteria: *acceptanceCriteria,
		GitRepository:      strings.TrimSpace(*gitRepository),
		GitBranch:          strings.TrimSpace(*gitBranch),
		Priority:           *priority,
		EstimateEffort:     *estimateEffort,
		EstimateComplete:   *estimateComplete,
		Assignee:           *assignee,
		Project:            *project,
	}
	if *parent != 0 {
		opts.ParentID = parent
	}
	return createTicket(opts)
}

func normalizeTicketCreateArgs(args []string) ([]string, error) {
	knownValueFlags := map[string]bool{
		"-type":              true,
		"-t":                 true,
		"-title":             true,
		"-priority":          true,
		"-p":                 true,
		"-estimate_effort":   true,
		"-assignee":          true,
		"-a":                 true,
		"-description":       true,
		"-d":                 true,
		"-ac":                true,
		"-git-repository":    true,
		"-git-branch":        true,
		"-estimate_complete": true,
		"-parent":            true,
		"-project":           true,
	}

	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if knownValueFlags[arg] {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: %s", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
			continue
		}
		positional = append(positional, arg)
	}
	// Insert "--" before positional args so flag.Parse won't treat
	// words like "-id" in the title as unknown flags.
	if len(positional) > 0 {
		flagArgs = append(flagArgs, "--")
		return append(flagArgs, positional...), nil
	}
	return flagArgs, nil
}

func createTicket(opts ticketCreateOptions) error {
	cfg, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Project) != "" {
		project, err = api.GetProject(opts.Project)
		if err != nil {
			return err
		}
	}
	parentID := opts.ParentID
	ticketType := strings.TrimSpace(strings.ToLower(opts.TicketType))
	if parentID == nil && cfg.CurrentEpicID > 0 && (ticketType == "task" || ticketType == "bug" || ticketType == "chore") {
		epic, err := api.GetTicket(strconv.FormatInt(cfg.CurrentEpicID, 10))
		if err != nil {
			return fmt.Errorf("current epic id %d is invalid: %w", cfg.CurrentEpicID, err)
		}
		if strings.TrimSpace(strings.ToLower(epic.Type)) != "epic" {
			return fmt.Errorf("current epic id %d is not an epic", cfg.CurrentEpicID)
		}
		if epic.ProjectID != project.ID {
			return fmt.Errorf("current epic id %d belongs to project %d, active project is %d", cfg.CurrentEpicID, epic.ProjectID, project.ID)
		}
		parentID = &epic.ID
	}
	ticket, err := api.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:          project.ID,
		ParentID:           parentID,
		Type:               opts.TicketType,
		Title:              opts.Title,
		Description:        opts.Description,
		AcceptanceCriteria: opts.AcceptanceCriteria,
		GitRepository:      opts.GitRepository,
		GitBranch:          opts.GitBranch,
		Priority:           opts.Priority,
		EstimateEffort:     opts.EstimateEffort,
		EstimateComplete:   opts.EstimateComplete,
		Assignee:           opts.Assignee,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	if ticket.Type == "epic" {
		cfg.CurrentEpicID = ticket.ID
		if err := config.Save(cfg); err != nil {
			return err
		}
	}
	fmt.Println(ticketLabel(ticket))
	return nil
}

func runConfig(args []string) error {
	if len(args) < 1 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(configUsage)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch args[0] {
	case "registration-enable":
		if len(args) != 1 {
			return errors.New("usage: ticket config registration-enable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(true); err != nil {
			return err
		}
		fmt.Println("registration_enabled=true")
		return nil
	case "registration-disable":
		if len(args) != 1 {
			return errors.New("usage: ticket config registration-disable")
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		if err := svc.SetRegistrationEnabled(false); err != nil {
			return err
		}
		fmt.Println("registration_enabled=false")
		return nil
	case "set":
		if len(args) != 3 {
			return errors.New("usage: ticket config set <key> <value>")
		}
		switch args[1] {
		case "server":
			cfg.ServerURL = args[2]
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", args[1], args[2])
		return nil
	case "get":
		if len(args) != 2 {
			return errors.New("usage: ticket config get <key>")
		}
		switch args[1] {
		case "server":
			if r, rErr := config.ResolveURL(); rErr == nil && r.ServerURL != "" {
				fmt.Println(r.ServerURL)
			} else if cfg.ServerURL != "" {
				fmt.Println(cfg.ServerURL)
			}
			return nil
		case "registration_enabled":
			svc, err := resolveService(cfg)
			if err != nil {
				return err
			}
			status, err := svc.Status()
			if err != nil {
				return err
			}
			fmt.Println(status.RegistrationEnabled)
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
	case "ls", "list":
		if len(args) != 1 {
			return errors.New("usage: ticket config ls")
		}
		r, _ := config.ResolveURL()
		serverURL := r.ServerURL
		if serverURL == "" {
			serverURL = cfg.ServerURL
		}
		fmt.Printf("url=%s\n", envValue("TICKET_URL"))
		fmt.Printf("mode=%s\n", r.Mode)
		fmt.Printf("server=%s\n", serverURL)
		fmt.Printf("username=%s\n", cfg.Username)
		fmt.Printf("current_project=%s\n", cfg.CurrentProject)
		fmt.Printf("current_epic_id=%d\n", cfg.CurrentEpicID)
		return nil
	case "rm", "delete":
		if len(args) != 2 {
			return errors.New("usage: ticket config rm|delete <key>")
		}
		switch args[1] {
		case "server":
			cfg.ServerURL = ""
		case "username":
			cfg.Username = ""
		case "current_project":
			cfg.CurrentProject = ""
		case "current_epic_id":
			cfg.CurrentEpicID = 0
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", args[1])
		return nil
	default:
		fmt.Println(configUsage)
		return fmt.Errorf("unknown config action %q", args[0])
	}
}
