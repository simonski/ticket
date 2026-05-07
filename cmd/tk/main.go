package main

import (
	"context"
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

const repoVersionURL = "https://raw.githubusercontent.com/simonski/ticket/refs/heads/main/cmd/tk/VERSION"

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

//go:embed VERSION
var embeddedVersion string

//go:embed TICKETS.md
var embeddedAgents string

func main() {
	if err := run(os.Args[1:]); err != nil {
		err = formatRuntimeError(err)
		if strings.HasPrefix(err.Error(), "no such command") {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	config.ClearLocationOverride()
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
	explicitServerDB := len(trimmedArgs) > 0 && trimmedArgs[0] == "server" && dbOverride != ""
	explicitInitDB := len(trimmedArgs) > 0 && trimmedArgs[0] == "initdb" && dbOverride != ""
	if explicitServerDB {
		trimmedArgs = append([]string{"server", "-f", dbOverride}, trimmedArgs[1:]...)
	}
	if explicitInitDB {
		trimmedArgs = append([]string{"initdb", "-f", dbOverride}, trimmedArgs[1:]...)
	}
	// -f /path/to/dir (or db file) sets the active local location for this run.
	if dbOverride != "" && !explicitServerDB && !explicitInitDB {
		absPath, pathErr := filepath.Abs(dbOverride)
		if pathErr != nil {
			return pathErr
		}
		location := absPath
		if !strings.HasSuffix(absPath, ".db") && !strings.HasSuffix(absPath, ".sqlite") {
			location = filepath.Join(absPath, "ticket.db")
		}
		config.SetLocationOverride(location)
	}
	var resolved config.Resolved
	if !explicitServerDB {
		resolved, err = config.ResolveURL()
		if err != nil {
			return err
		}
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

	// Commands that don't require an initialised project binding.
	noInitRequired := map[string]bool{
		"init": true, "initdb": true, "setup": true, "server": true, "help": true, "version": true, "upgrade": true, "upgrade-database": true, "skill": true, "docker-compose": true, "remote": true,
		"login": true, "logout": true, "register": true, "status": true,
	}
	if len(trimmedArgs) == 1 {
		switch trimmedArgs[0] {
		case "project", "workflow", "team", "story", "user", "label", "dep", "decision", "agent", "role", "idea":
			noInitRequired[trimmedArgs[0]] = true
		}
	}
	if len(trimmedArgs) > 1 && trimmedArgs[0] == "project" && trimmedArgs[1] == "init" {
		noInitRequired["project"] = true
	}
	if len(trimmedArgs) > 1 && trimmedArgs[0] == "project" && trimmedArgs[1] == "remote" {
		noInitRequired["project"] = true
	}
	if len(trimmedArgs) > 1 {
		switch trimmedArgs[1] {
		case "help", "-h", "--help":
			noInitRequired[trimmedArgs[0]] = true
		}
	}
	if !noInitRequired[trimmedArgs[0]] && !explicitServerDB && !config.HasLocationOverride() && resolved.Mode == config.ModeLocal {
		if _, ok, pathErr := config.ProjectPath(); pathErr != nil {
			return pathErr
		} else if !ok {
			if err := maybeBootstrapMutableCommand(trimmedArgs); err != nil {
				return err
			}
			if _, ok, pathErr = config.ProjectPath(); pathErr != nil {
				return pathErr
			}
			if !ok {
				return advisoryNotManagedProject()
			}
		}
	}

	switch trimmedArgs[0] {
	case "help", "-h", "--help":
		return runHelp(trimmedArgs[1:])
	case "summary":
		return runSummary(trimmedArgs[1:])
	case "onboard":
		return runOnboard(trimmedArgs[1:])
	case "skill":
		return runSkill(trimmedArgs[1:])
	case "docker-compose":
		return runDockerCompose(trimmedArgs[1:])
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
	case "upgrade-database":
		return runUpgradeDatabase(trimmedArgs[1:])
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
	case "prompt":
		return runPrompt(trimmedArgs[1:])
	case "agent":
		return runAgent(trimmedArgs[1:])
	case "user":
		return runUser(trimmedArgs[1:])
	case "project":
		return runProject(trimmedArgs[1:])
	case "remote":
		return runRemote(trimmedArgs[1:])
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
		return runComplete(trimmedArgs[1:])
	case "reopen":
		return runReopen(trimmedArgs[1:])
	case "success":
		return runTicketStateAlias(trimmedArgs[1:], store.StateSuccess, trimmedArgs[0])
	case "fail":
		return runTicketStateAlias(trimmedArgs[1:], store.StateFail, trimmedArgs[0])
	case "next":
		return runNext(trimmedArgs[1:])
	case "previous", "prev":
		return runPrevious(trimmedArgs[1:])
	case "draft":
		return runSetTicketDraft(trimmedArgs[1:], false)
	case "undraft":
		return runSetTicketDraft(trimmedArgs[1:], true)
	case "reject":
		if len(trimmedArgs) > 1 && trimmedArgs[1] == "requirement" {
			return runRequirementStatus("rejected", trimmedArgs[1:])
		}
		return runRejectTicket(trimmedArgs[1:])
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

	if outputJSON {
		// All tickets for this project (non-archived), then keep only open ones
		all, _ := svc.ListTicketsFiltered(context.Background(), project.ID, "", "", "", "", "", "", 0, false)
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
		typeCounts := map[string]int{}
		for _, t := range allTickets {
			typeCounts[t.Type]++
		}
		recent := make([]store.Ticket, len(allTickets))
		copy(recent, allTickets)
		sort.Slice(recent, func(i, j int) bool {
			return recent[i].UpdatedAt > recent[j].UpdatedAt
		})
		if len(recent) > 5 {
			recent = recent[:5]
		}
		resolved, _ := config.ResolveURL()
		cfgPath, _ := config.Path()
		return printJSON(map[string]any{
			"project":     project,
			"type_counts": typeCounts,
			"active":      activeTickets,
			"recent":      recent,
			"db_path":     resolved.DBPath,
			"config_file": cfgPath,
		})
	}

	printProjectSummaryBox(svc, project, true)
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

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func containsFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}
