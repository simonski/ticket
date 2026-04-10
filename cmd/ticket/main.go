package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
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
	trimmedArgs, dbOverride, err := extractDBOverride(args)
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
	case "init":
		return runSetup(trimmedArgs[1:])
	case "initdb":
		return runInitDB(trimmedArgs[1:])
	case "server":
		return runServer(trimmedArgs[1:])
	case "export":
		if resolved.Mode != config.ModeLocal {
			return errors.New("ticket export requires local mode")
		}
		return runExportSnapshot(trimmedArgs[1:])
	case "import":
		if resolved.Mode != config.ModeLocal {
			return errors.New("ticket import requires local mode")
		}
		return runImportSnapshot(trimmedArgs[1:])
	case "version":
		return runVersion(trimmedArgs[1:])
	case "upgrade":
		return runUpgrade(trimmedArgs[1:])
	case "register":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket register requires remote mode (run tk init to configure)")
		}
		return runRegister(trimmedArgs[1:])
	case "login":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket login requires remote mode (run tk init to configure)")
		}
		return runLogin(trimmedArgs[1:])
	case "logout":
		if resolved.Mode != config.ModeRemote {
			return errors.New("ticket logout requires remote mode (run tk init to configure)")
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
	case "sdlc":
		return runSdlc(trimmedArgs[1:])
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
		return runComplete(trimmedArgs[1:])
	case "reopen":
		return runReopen(trimmedArgs[1:])
	case "success":
		return runTicketStateAlias(trimmedArgs[1:], store.StateSuccess, trimmedArgs[0])
	case "next":
		return runNext(trimmedArgs[1:])
	case "previous", "prev":
		return runPrevious(trimmedArgs[1:])
	case "draft":
		return runSetTicketDraft(trimmedArgs[1:], true)
	case "undraft":
		return runSetTicketDraft(trimmedArgs[1:], false)
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
	case "doctor":
		return runDoctor(trimmedArgs[1:])
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
		return runSetTicketDraft(trimmedArgs[1:], true)
	case "notready":
		return runSetTicketDraft(trimmedArgs[1:], false)
	case "rm", "delete":
		return runDeleteTicket(trimmedArgs[1:])
	case "req":
		return runReq(trimmedArgs[1:])
	case "idea":
		return runIdea(trimmedArgs[1:])
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
		if !t.Complete {
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
			lines = append(lines, statusLine{key: "  " + t.ID, value: val, color: "\x1b[32m"})
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
			lines = append(lines, statusLine{key: "  " + t.ID, value: val})
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

func containsFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}
