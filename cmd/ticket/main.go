package main

import (
	"bufio"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/server"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
	"github.com/simonski/ticket/libtickethttp"
	"golang.org/x/term"
)

type commandHelp struct {
	usage   string
	details []string
	example string
}

var (
	loginPromptInput  io.Reader = os.Stdin
	loginPromptOutput io.Writer = os.Stdout
	outputJSON        bool
	noColorOutput     bool
	runAgentCommand   = defaultRunTicketAgentCommand
	selectBannerWord  = randomBannerWord
)

func envValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

var bannerWords = []string{"TICKET", "TKT", "TCKT", "TKET", "TICKT"}

var bannerGlyphs = map[rune][]string{
	'T': {
		"TTTTTTT",
		"   T   ",
		"   T   ",
		"   T   ",
		"   T   ",
		"   T   ",
	},
	'I': {
		"IIIIIII",
		"   I   ",
		"   I   ",
		"   I   ",
		"   I   ",
		"IIIIIII",
	},
	'C': {
		" CCCCC ",
		"CC   CC",
		"CC     ",
		"CC     ",
		"CC   CC",
		" CCCCC ",
	},
	'K': {
		"KK   KK",
		"KK  KK ",
		"KKKKK  ",
		"KK  KK ",
		"KK   KK",
		"KK   KK",
	},
	'E': {
		"EEEEEEE",
		"EE     ",
		"EEEEE  ",
		"EE     ",
		"EE     ",
		"EEEEEEE",
	},
}

var bannerColors = []string{
	"\x1b[31m",
	"\x1b[33m",
	"\x1b[32m",
	"\x1b[36m",
	"\x1b[34m",
	"\x1b[35m",
}

//go:embed VERSION
var embeddedVersion string

//go:embed AGENTS.md
var embeddedAgents string

var helpIndex = map[string]commandHelp{
	"onboard": {
		usage:   "ticket onboard",
		details: []string{"Appends the embedded onboarding template to `${CWD}/AGENTS.md`.", "Creates `${CWD}/AGENTS.md` if it does not already exist."},
		example: "ticket onboard",
	},
	"initdb": {
		usage:   "ticket initdb [-f <db-path>] [--force] [-password <password>]",
		details: []string{"Creates a new SQLite database, bootstraps the fixed `admin` account, and creates the default project.", "If `-f` is omitted, the database is created at `$TICKET_HOME/ticket.db`.", "If `-password` is omitted, a random admin password is generated and printed to stdout.", "If `--force` is supplied, any existing database file is overwritten."},
		example: "ticket initdb -f $TICKET_HOME/ticket.db --force -password secret",
	},
	"server": {
		usage:   "ticket server [-f <db-path>] [-addr :8080] [-v]",
		details: []string{"Starts the HTTP API server and the embedded web UI.", "If `-f` is omitted, the server uses `$TICKET_HOME/ticket.db`.", "If `-v` is supplied, requests and responses are printed verbosely to stdout."},
		example: "ticket server -f $TICKET_HOME/ticket.db -addr :8080 -v",
	},
	"version": {
		usage:   "ticket version",
		details: []string{"Prints the semantic version embedded into the binary from the build-time `VERSION` file."},
		example: "ticket version",
	},
	"login": {
		usage:   "ticket login [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"REMOTE mode only. Logs into the configured server and stores the session token in `$TICKET_HOME/credentials.json`.", "Login resolution order: valid `$TICKET_HOME/credentials.json`, then `username` in `$TICKET_HOME/config.json`, then `-username` / `-password`, then `TICKET_USERNAME` / `TICKET_PASSWORD`, then prompts.", "If prompting is needed, discovered values are used as editable defaults.", "Server resolution: `-url`, then `TICKET_SERVER`, then `TICKET_URL`, then configured URL, then `http://localhost:8080`."},
		example: "ticket login -username simon -password secret -url http://localhost:8080",
	},
	"register": {
		usage:   "ticket register [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"REMOTE mode only. Creates a user account on the configured server but does not log the user in.", "Credential resolution: `-username`, then `TICKET_USERNAME`, then OS `whoami`; `-password`, then `TICKET_PASSWORD`, then `password`."},
		example: "ticket register -username simon -password secret",
	},
	"logout": {
		usage:   "ticket logout [-url <server-url>]",
		details: []string{"REMOTE mode only. Logs out from the configured server and removes `$TICKET_HOME/credentials.json`."},
		example: "ticket logout",
	},
	"status": {
		usage:   "ticket status [-url <server-url>] [-f <db-path>] [-nocolor]",
		details: []string{"Prints the current effective configuration first, then performs a mode-specific connectivity check.", "REMOTE mode prints `mode`, `server`, `username`, `authenticated`, then calls the remote status endpoint.", "LOCAL mode prints `mode`, `db_path`, `db_exists`, then opens the database and verifies the schema is usable."},
		example: "ticket status",
	},
	"help": {
		usage:   "ticket help <command>",
		details: []string{"Shows command-specific help when available.", "Without a command, prints the root usage summary."},
		example: "ticket help dependency",
	},
	"count": {
		usage:   "ticket count [-project_id <id>] [-url <server-url>]",
		details: []string{"Counts users and work items by type.", "With `-project_id`, counts work items within that project and omits the global project total."},
		example: "ticket count -project_id 1",
	},
	"ticket": {
		usage:   "ticket ticket -f <file1,file2,...> -o <output-file> [-agent <agent>]",
		details: []string{"Reads the listed input files, sends a requirements-breakdown prompt to an agent, and writes the agent output to the requested output file.", "Default agent is `codex`, which is invoked as `codex exec <prompt>`. Other agents are invoked as `<agent> -p <prompt>`."},
		example: "ticket ticket -f README.md,docs/DESIGN.md -o requirements.md",
	},
	"health": {
		usage:   "ticket health <id>|execute",
		details: []string{"Compute and persist ticket health scores using documented heuristics.", "`execute` scores all tickets in the active project."},
		example: "ticket health TK-1",
	},
	"project": {
		usage:   "ticket project <create|list|get|use>|<id> <update|enable|disable>",
		details: []string{"Manages projects and the active project context used by subsequent commands.", "Projects are addressed by prefix or numeric id."},
		example: "ticket project CUS update -title \"Customer Portal\"",
	},
	"list": {
		usage:   "ticket list|ls [--type <type>] [--stage <stage>] [--state <state>] [--status <stage/state>] [-u <user>] [-n <limit>] [--unicode] [--plain]",
		details: []string{"Lists tasks in the active project with optional type, lifecycle, assignee, and limit filters.", "`status` is a rendered composite such as `develop/active`. `-n` is applied server-side. `0` means no limit."},
		example: "ticket list --type bug --status develop/idle -u alice -n 20",
	},
	"orphans": {
		usage:   "ticket orphans [-url <server-url>]",
		details: []string{"Lists unparented non-epic tasks in the active project."},
		example: "ticket orphans",
	},
	"get": {
		usage:   "ticket get <id> [-url <server-url>]",
		details: []string{"Shows a single task with comments and history.", "Output uses subtle color unless `-nocolor` is supplied."},
		example: "ticket get 42",
	},
	"show": {
		usage:   "ticket show <id> [-url <server-url>]",
		details: []string{"Alias for `ticket get`."},
		example: "ticket show 42",
	},
	"search": {
		usage:   "ticket search <free form query> [-stage <stage>] [-state <state>] [-status <stage/state>] [-title <text>] [-description <text>] [-priority <n>] [-owner <user>] [-allprojects]",
		details: []string{"Searches tasks in the active project by default.", "Use `-allprojects` to search across every project. Optional filters narrow by lifecycle, title text, description text, priority, and owner."},
		example: "ticket search password reset -status develop/active -owner alice -allprojects",
	},
	"update": {
		usage:   "ticket update <id> [-title <title>] [-desc <description>|-description <description>] [-ac <acceptance-criteria>] [-priority <n>] [-order <n>] [-stage <stage>] [-state <state>] [-status <stage/state>] [-parent_id <id>] [-estimate_effort <n>] [-estimate_complete <rfc3339>]",
		details: []string{"Updates one or more task fields in a single command.", "Use `-stage` and `-state` or `-status <stage/state>` to edit the lifecycle directly on leaf tickets. `estimate_complete` must be RFC3339, for example `2026-03-31T17:00:00Z`."},
		example: "ticket update 42 -title \"Customer Portal\" -status develop/active -priority 2 -estimate_effort 5",
	},
	"set-parent": {
		usage:   "ticket set-parent <id> <parent-id>",
		details: []string{"Sets the parent of a task or epic.", "Both ids must be numeric task ids in the active project.", "If the child is an epic, the parent must also be an epic."},
		example: "ticket set-parent 1 2",
	},
	"attach": {
		usage:   "ticket attach <id> <parent-id>",
		details: []string{"Alias for `ticket set-parent`."},
		example: "ticket attach CUS-T-12 CUS-E-3",
	},
	"unset-parent": {
		usage:   "ticket unset-parent <id>",
		details: []string{"Clears the parent of a task or story.", "After this, the task becomes an orphan."},
		example: "ticket unset-parent 1",
	},
	"detach": {
		usage:   "ticket detach <id>",
		details: []string{"Alias for `ticket unset-parent`."},
		example: "ticket detach CUS-T-12",
	},
	"design": {
		usage:   "ticket design <id>",
		details: []string{"Sets the ticket stage to `design` and the state to `idle`."},
		example: "ticket design 42",
	},
	"develop": {
		usage:   "ticket develop <id>",
		details: []string{"Sets the ticket stage to `develop` and the state to `idle`."},
		example: "ticket develop 42",
	},
	"test": {
		usage:   "ticket test <id>",
		details: []string{"Sets the ticket stage to `test` and the state to `idle`."},
		example: "ticket test 42",
	},
	"done": {
		usage:   "ticket done <id>",
		details: []string{"Sets the ticket stage to `done` and the state to `complete`."},
		example: "ticket done 42",
	},
	"idle": {
		usage:   "ticket idle <id>",
		details: []string{"Sets the ticket state to `idle` without changing the stage."},
		example: "ticket idle 42",
	},
	"active": {
		usage:   "ticket active <id>",
		details: []string{"Sets the ticket state to `active` without changing the stage.", "`active` requires an assignee; if the ticket is unassigned the CLI claims it for the current user first."},
		example: "ticket active 42",
	},
	"complete": {
		usage:   "ticket complete <id>",
		details: []string{"Sets the ticket state to `complete` without changing the stage."},
		example: "ticket complete 42",
	},
	"add": {
		usage:   "ticket add|create|new [-title <title>] [-t <type>] [-p <priority>] [-a <assignee>] [-d <description>] [-ac <criteria>] [-parent <id>] [-project <project>] [-estimate_effort <n>] [-estimate_complete <rfc3339>] [title words]",
		details: []string{"Creates a task-like entity in the active project.", "Positional title words and `-title` are equivalent ways to set the title.", "Defaults: `type=task`, `stage=design`, `state=idle`, `priority=1`, blank assignee, blank description, blank acceptance criteria, blank parent, current project, `estimate_effort=0`, blank `estimate_complete`."},
		example: "ticket add \"Customers can reset their password.\"",
	},
	"comment": {
		usage:   "ticket comment add <id> \"comment\" [-url <server-url>]",
		details: []string{"Adds a comment to a task and records a corresponding history event."},
		example: "ticket comment add 42 \"Need product sign-off.\"",
	},
	"clone": {
		usage:   "ticket clone|cp <id>",
		details: []string{"Clones a task or epic.", "Cloned items are unassigned, reset to `design/idle`, and keep a `clone_of` reference to the source item. Cloning an epic also clones its child tasks."},
		example: "ticket clone 42",
	},
	"delete": {
		usage:   "ticket rm|delete <id>",
		details: []string{"Deletes a task permanently.", "Fails if the task still has child tasks."},
		example: "ticket delete 42",
	},
	"assign": {
		usage:   "ticket assign <id> <name>",
		details: []string{"Admin-only command that assigns a task to a user.", "The target user must exist and be enabled."},
		example: "ticket assign 42 alice",
	},
	"unassign": {
		usage:   "ticket unassign <id> <name>",
		details: []string{"Admin-only command that clears a task assignment from the named user.", "The named user must exist and be enabled."},
		example: "ticket unassign 42 alice",
	},
	"claim": {
		usage:   "ticket claim <id>",
		details: []string{"Assigns the caller to the task.", "Fails if the task is already assigned to another user."},
		example: "ticket claim 42",
	},
	"unclaim": {
		usage:   "ticket unclaim <id>",
		details: []string{"Clears the caller's assignment from the task.", "Fails unless the caller is the current assignee."},
		example: "ticket unclaim 42",
	},
	"add-dependency": {
		usage:   "ticket add-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Adds one or more `depends_on` links from the task to the listed task IDs.", "Comma-separated dependency IDs are supported."},
		example: "ticket add-dependency 4 1,2,3",
	},
	"remove-dependency": {
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
	case "health", "heatlh":
		return runHealth(trimmedArgs[1:])
	case "comment":
		return runComment(trimmedArgs[1:])
	case "clone", "cp":
		return runClone(trimmedArgs[1:])
	case "rm", "delete":
		return runDeleteTicket(trimmedArgs[1:])
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
		return runTypedTicketCreate("epic", trimmedArgs[1:])
	case "config":
		return runConfig(trimmedArgs[1:])
	default:
		return fmt.Errorf("no such command %q", trimmedArgs[0])
	}
}

func runHelp(args []string) error {
	if len(args) == 0 {
		fmt.Print(renderRootUsage())
		return nil
	}
	if !hasCommandHelp(args[0]) {
		return fmt.Errorf("no such command %q", args[0])
	}
	fmt.Print(renderCommandHelp(args[0]))
	return nil
}

func runOnboard(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket onboard")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target := filepath.Join(cwd, "AGENTS.md")
	var needsLeadingNewline bool
	if info, err := os.Stat(target); err == nil && info.Size() > 0 {
		existing, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			needsLeadingNewline = true
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if needsLeadingNewline {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := f.WriteString(embeddedAgents); err != nil {
		return err
	}
	if !strings.HasSuffix(embeddedAgents, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if outputJSON {
		return printJSON(map[string]string{"status": "ok", "path": target})
	}
	fmt.Printf("appended onboarding template to %s\n", target)
	return nil
}

func runInitDB(args []string) error {
	fs := flag.NewFlagSet("initdb", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defaultDBPath, err := defaultDatabasePath()
	if err != nil {
		return err
	}
	dbPath := fs.String("f", defaultDBPath, "SQLite database file")
	passwordFlag := fs.String("password", "", "bootstrap password")
	force := fs.Bool("force", false, "overwrite the database file if it exists")

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

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.CurrentProject = "1"
	cfg.Username = "admin"
	cfg.ServerURL = config.ResolveServerURL(cfg)
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("initialized database at %s\n", *dbPath)
	fmt.Printf("admin user: admin\n")
	fmt.Printf("admin password: %s\n", password)
	fmt.Printf("default project: 1\n")
	if generated {
		fmt.Println("admin password was generated because -password was not provided")
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
	verbose := fs.Bool("v", false, "print verbose request/response logs to stdout")

	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	srv, err := server.New(*addr, db, strings.TrimSpace(embeddedVersion), *verbose, os.Stdout)
	if err != nil {
		return err
	}

	fmt.Print(renderBanner())
	fmt.Printf("VERSION    %s\n", strings.TrimSpace(embeddedVersion))
	fmt.Printf("TICKETDB   %s\n\n", *dbPath)
	fmt.Printf("serving ticket on http://localhost%s\n", *addr)
	return srv.ListenAndServe()
}

func runVersion(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ticket version")
	}
	fmt.Println(strings.TrimSpace(embeddedVersion))
	return nil
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
			cfg.ServerURL = config.ResolveServerURL(cfg)
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
	cfg.ServerURL = config.ResolveServerURL(cfg)
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
	mode, err := config.ResolveMode()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch mode {
	case config.ModeRemote:
		return runRemoteStatus(cfg)
	case config.ModeLocal:
		return runLocalStatus()
	default:
		return fmt.Errorf("unsupported TICKET_MODE %q", mode)
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

func runUser(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ticket user <create|ls|list|rm|delete|enable|disable>")
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
	default:
		return fmt.Errorf("unknown user command %q", args[0])
	}
}

func printUserTable(users []store.User) {
	if len(users) == 0 {
		fmt.Println("no users")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tROLE\tENABLED")
	for _, user := range users {
		fmt.Fprintf(w, "%s\t%s\t%t\n", user.Username, user.Role, user.Enabled)
	}
	_ = w.Flush()
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

	if len(args) == 0 {
		if cfg.CurrentProject == "" {
			fmt.Println("no active project")
			return nil
		}
		project, err := svc.GetProject(cfg.CurrentProject)
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(project)
		}
		printProject(project)
		return nil
	}

	if projectID, ok := parseProjectCommandID(args[0]); ok {
		return runProjectByID(svc, projectID, args[1:])
	}

	switch args[0] {
	case "create", "add", "new":
		fs := flag.NewFlagSet("project create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		prefix := fs.String("prefix", "", "project prefix")
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("usage: ticket project create -prefix <prefix> [-description text] [-ac text] \"Project Title\"")
		}
		if strings.TrimSpace(*prefix) == "" {
			return errors.New("project prefix is required")
		}
		project, err := svc.CreateProject(libticket.ProjectCreateRequest{
			Prefix:             *prefix,
			Title:              fs.Arg(0),
			Description:        *description,
			AcceptanceCriteria: *acceptanceCriteria,
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
		printProjectTable(projects, cfg.CurrentProject)
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
	case "use":
		if len(args) != 2 {
			return errors.New("usage: ticket project use <id>")
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
	default:
		return fmt.Errorf("unknown project command %q", args[0])
	}
}

func runTicket(args []string) error {
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
	response, err := runAgentCommand(strings.TrimSpace(*agent), prompt)
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

func defaultRunTicketAgentCommand(agent, prompt string) (string, error) {
	if agent == "" {
		return "", errors.New("agent is required")
	}
	var cmd *exec.Cmd
	if agent == "codex" {
		cmd = exec.Command("codex", "exec", prompt)
	} else {
		cmd = exec.Command(agent, "-p", prompt)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%v: %s", err, message)
	}
	return string(output), nil
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
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		current, err := svc.GetProject(strconv.FormatInt(projectID, 10))
		if err != nil {
			return err
		}
		nextDescription := current.Description
		nextAC := current.AcceptanceCriteria
		if fs.Lookup("description") != nil && strings.TrimSpace(*description) != "" || containsFlag(args[1:], "-description") {
			nextDescription = *description
		}
		if containsFlag(args[1:], "-ac") {
			nextAC = *acceptanceCriteria
		}
		project, err := svc.UpdateProject(projectID, libticket.ProjectUpdateRequest{
			Title:              *title,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
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
		return fmt.Errorf("unknown project command %q", args[0])
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

func resolveLifecycleInput(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return "", "", nil
	}
	return store.ParseLifecycleStatus(status)
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	taskType := fs.String("type", "", "filter by task type")
	stage := fs.String("stage", "", "filter by task stage")
	state := fs.String("state", "", "filter by task state")
	status := fs.String("status", "", "filter by rendered task status")
	assignee := fs.String("user", "", "filter by assignee")
	fs.StringVar(assignee, "u", "", "filter by assignee")
	limit := fs.Int("n", 0, "maximum number of tasks to return; 0 means all")
	useUnicode := fs.Bool("unicode", true, "render status symbols as unicode")
	plain := fs.Bool("plain", false, "render status as plain text")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 0 {
		return errors.New("usage: ticket list|ls [--type <type>] [--stage <stage>] [--state <state>] [--status <stage/state>] [-u <user>] [-n <limit>]")
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
	tasks, err := api.ListTicketsFiltered(project.ID, *taskType, resolvedStage, resolvedState, "", "", *assignee, *limit)
	if err != nil {
		return err
	}
	dependenciesByTask := make(map[int64]string, len(tasks))
	for _, task := range tasks {
		dependencies, err := api.ListDependencies(task.ID)
		if err != nil {
			return err
		}
		dependenciesByTask[task.ID] = formatDependsOn(dependencies)
	}
	if outputJSON {
		return printJSON(tasks)
	}
	printTicketTable(tasks, dependenciesByTask, statusUnicode)
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
	tasks, err := api.ListTickets(project.ID)
	if err != nil {
		return err
	}
	var orphans []store.Ticket
	for _, task := range tasks {
		if task.ParentID == nil && strings.TrimSpace(task.Type) != "epic" {
			orphans = append(orphans, task)
		}
	}
	if outputJSON {
		return printJSON(orphans)
	}
	for _, task := range orphans {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(task), task.Type, task.Status, task.Title)
	}
	return nil
}

func runGet(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket get <id>")
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
	dependencies, _ := svc.ListDependencies(task.ID)
	if outputJSON {
		return printJSON(task)
	}
	printTicketDetails(task, dependencies)
	return nil
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
	var tasks []store.Ticket
	for _, project := range projects {
		projectTasks, err := svc.ListTicketsFiltered(project.ID, "", "", "", "", "", "", 0)
		if err != nil {
			return err
		}
		for _, task := range projectTasks {
			if !taskMatchesSearch(task, query, filters.stage, filters.state, filters.status, filters.title, filters.description, filters.priority, filters.owner) {
				continue
			}
			tasks = append(tasks, task)
		}
	}
	if outputJSON {
		return printJSON(tasks)
	}
	for _, task := range tasks {
		fmt.Printf("%s\t%s\t%s\t%s\n", ticketLabel(task), task.Type, task.Status, task.Title)
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

func taskMatchesSearch(task store.Ticket, query, stage, state, status, title, description string, priority int, owner string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			task.Title,
			task.Description,
			task.AcceptanceCriteria,
			task.Assignee,
			task.Status,
			strconv.Itoa(task.Priority),
		}, "\n"))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		stageFilter, stateFilter, err := resolveLifecycleInput(trimmed, "", "")
		if err == nil {
			if task.Stage != strings.TrimSpace(stageFilter) || task.State != strings.TrimSpace(stateFilter) {
				return false
			}
		} else if task.Status != trimmed {
			return false
		}
	}
	if trimmed := strings.TrimSpace(stage); trimmed != "" && task.Stage != trimmed {
		return false
	}
	if trimmed := strings.TrimSpace(state); trimmed != "" && task.State != trimmed {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(title)); trimmed != "" && !strings.Contains(strings.ToLower(task.Title), trimmed) {
		return false
	}
	if trimmed := strings.ToLower(strings.TrimSpace(description)); trimmed != "" {
		descriptionFields := strings.ToLower(task.Description + "\n" + task.AcceptanceCriteria)
		if !strings.Contains(descriptionFields, trimmed) {
			return false
		}
	}
	if priority != 0 && task.Priority != priority {
		return false
	}
	if trimmed := strings.TrimSpace(owner); trimmed != "" && task.Assignee != trimmed {
		return false
	}
	return true
}

func runSetParent(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: ticket set-parent <id> <parent-id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	child, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	parent, err := svc.GetTicket(args[1])
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

func runUnsetParent(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket unset-parent <id>")
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
	updated, err := svc.UnsetTicketParent(task.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(updated)
	}
	printTicket(updated)
	return nil
}

func runTicketStageAlias(args []string, stage, command string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ticket %s <id>", command)
	}
	return updateTicketStage(args[0], stage)
}

func runTicketStateAlias(args []string, state, command string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ticket %s <id>", command)
	}
	return updateTicketState(args[0], state)
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
	nextState := store.StateIdle
	if stage == store.StageDone {
		nextState = store.StateComplete
	}
	return updateTicketLifecycleRequest(svc, current.ID, current, stage, nextState)
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
	return updateTicketLifecycleRequest(svc, current.ID, current, current.Stage, state)
}

func updateTicketLifecycleRequest(svc libticket.Service, id int64, current store.Ticket, stage, state string) error {
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
		Stage:              stage,
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
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	title := fs.String("title", "", "task title")
	description := fs.String("description", "", "task description")
	desc := fs.String("desc", "", "task description")
	acceptanceCriteria := fs.String("ac", "", "task acceptance criteria")
	priority := fs.Int("priority", 0, "task priority")
	order := fs.Int("order", 0, "task order")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	status := fs.String("status", "", "rendered task status (<stage>/<state>)")
	stage := fs.String("stage", "", "task stage")
	state := fs.String("state", "", "task state")
	parentIDRaw := fs.String("parent_id", "", "task parent id")
	if len(args) == 0 {
		return errors.New("usage: ticket update <id> [-title <title>] [-desc <description>|-description <description>] [-ac <acceptance-criteria>] [-priority <n>] [-order <n>] [-stage <stage>] [-state <state>] [-parent_id <id>] [-estimate_effort <n>] [-estimate_complete <rfc3339>]")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: ticket update <id> [-title <title>] [-desc <description>|-description <description>] [-ac <acceptance-criteria>] [-priority <n>] [-order <n>] [-stage <stage>] [-state <state>] [-parent_id <id>] [-estimate_effort <n>] [-estimate_complete <rfc3339>]")
	}
	hasTitle := containsFlag(args[1:], "-title")
	hasDescription := containsFlag(args[1:], "-description")
	hasDesc := containsFlag(args[1:], "-desc")
	hasAC := containsFlag(args[1:], "-ac")
	hasPriority := containsFlag(args[1:], "-priority")
	hasOrder := containsFlag(args[1:], "-order")
	hasEstimateEffort := containsFlag(args[1:], "-estimate_effort")
	hasEstimateComplete := containsFlag(args[1:], "-estimate_complete")
	hasStatus := containsFlag(args[1:], "-status")
	hasStage := containsFlag(args[1:], "-stage")
	hasState := containsFlag(args[1:], "-state")
	hasParentID := containsFlag(args[1:], "-parent_id")
	if !hasTitle && !hasDescription && !hasDesc && !hasAC && !hasPriority && !hasOrder && !hasEstimateEffort && !hasEstimateComplete && !hasStatus && !hasStage && !hasState && !hasParentID {
		return errors.New("usage: ticket update <id> [-title <title>] [-desc <description>|-description <description>] [-ac <acceptance-criteria>] [-priority <n>] [-order <n>] [-stage <stage>] [-state <state>] [-parent_id <id>] [-estimate_effort <n>] [-estimate_complete <rfc3339>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	current, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	next := libticket.TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
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
		resolvedStage, resolvedState, err := resolveLifecycleInput(*status, "", "")
		if err != nil {
			return err
		}
		next.Stage = resolvedStage
		next.State = resolvedState
	}
	if hasStage {
		next.Stage = *stage
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
	printTicketDetails(updated, dependencies)
	return nil
}

func runAssign(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: ticket assign <id> <name>")
	}
	return assignTicket(args[0], args[1], true)
}

func runUnassign(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: ticket unassign <id> <name>")
	}
	return unassignTicket(args[0], args[1], true)
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
	if len(args) != 1 {
		return errors.New("usage: ticket unclaim <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	username := strings.TrimSpace(cfg.Username)
	if mode, err := config.ResolveMode(); err == nil && mode == config.ModeLocal {
		username = localModeUsername()
	}
	if strings.TrimSpace(username) == "" {
		return errors.New("no current username; log in first")
	}
	return unassignTicket(args[0], username, false)
}

func parseTicketID(idArg string) (int64, error) {
	var id int64
	if _, err := fmt.Sscan(idArg, &id); err != nil {
		return 0, errors.New("ticket id must be numeric")
	}
	return id, nil
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
		Stage:              current.Stage,
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
		return errors.New("usage: ticket health <id>|execute")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	if strings.EqualFold(args[0], "execute") {
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		projectTickets, err := api.ListTickets(project.ID)
		if err != nil {
			return err
		}

		results := make([]map[string]any, 0, len(projectTickets))
		for _, task := range projectTickets {
			comments, err := svc.ListComments(task.ID)
			if err != nil {
				return err
			}
			checks := ticketHealthCheck(task, comments)
			updated, err := svc.SetTicketHealth(task.ID, checks.score)
			if err != nil {
				return err
			}
			result := map[string]any{
				"ticket_id":                  task.ID,
				"ticket_key":                 task.Key,
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

	task, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	comments, err := svc.ListComments(task.ID)
	if err != nil {
		return err
	}

	checks := ticketHealthCheck(task, comments)
	updated, err := svc.SetTicketHealth(task.ID, checks.score)
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
				"ticket_id":    task.ID,
				"ticket_key":   ticketLabel(task),
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
	task, err := api.GetTicket(args[0])
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
			TicketID:  task.ID,
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
			"task_id":      task.ID,
			"dependencies": args[1],
			"action":       map[bool]string{true: "added", false: "removed"}[add],
		})
	}
	action := "added"
	if !add {
		action = "removed"
	}
	fmt.Printf("%s dependencies for %s: %s\n", action, ticketLabel(task), args[1])
	return nil
}

func runDependency(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ticket dependency <add|remove> <id> <dependency-id[,dependency-id...]>")
	}
	switch args[0] {
	case "add":
		return runDependencyCommand(args[1:], true)
	case "remove":
		return runDependencyCommand(args[1:], false)
	default:
		return fmt.Errorf("unknown dependency action %q", args[0])
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

func resolveCurrentRequestUser(cfg config.Config, mode string) (string, error) {
	username := strings.TrimSpace(cfg.Username)
	if mode == config.ModeLocal {
		username = localModeUsername()
	}
	if strings.TrimSpace(username) == "" {
		return "", errors.New("no current username; log in first")
	}

	if mode == config.ModeLocal {
		return username, nil
	}

	_, api, _, err := resolveCurrentProjectClient()
	if err != nil {
		return "", err
	}
	status, err := api.Status()
	if err != nil {
		return "", err
	}
	if status.User == nil {
		return "", errors.New("no current username; log in first")
	}
	if status.User.Username != "" {
		return status.User.Username, nil
	}
	return username, nil
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
	if len(args) == 0 {
		return errors.New("usage: ticket comment add <id> \"comment\"")
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return errors.New("usage: ticket comment add <id> \"comment\"")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, err := resolveService(cfg)
		if err != nil {
			return err
		}
		task, err := svc.GetTicket(args[1])
		if err != nil {
			return err
		}
		comment, err := svc.AddComment(task.ID, args[2])
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(comment)
		}
		fmt.Printf("commented on %s: %s\n", ticketLabel(task), comment.Comment)
		return nil
	default:
		return fmt.Errorf("unknown comment command %q", args[0])
	}
}

func runClone(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket clone|cp <id>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	taskRef, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	task, err := svc.CloneTicket(taskRef.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(task)
	}
	printTicket(task)
	return nil
}

func runDeleteTicket(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ticket rm|delete <id>")
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
	if err := svc.DeleteTicket(task.ID); err != nil {
		return err
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "deleted", "ticket_id": task.ID, "key": task.Key})
	}
	fmt.Printf("deleted ticket %s\n", ticketLabel(task))
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
		task, err := api.GetTicket(arg)
		if err != nil {
			return err
		}
		sourceIDs = append(sourceIDs, task.ID)
		titles = append(titles, task.Title)
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

func runReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	status := fs.String("status", "proposed", "review status")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, api, project, err := resolveCurrentProjectClient()
	if err != nil {
		return err
	}
	tasks, err := api.ListTicketsFiltered(project.ID, "requirement", "", "", *status, "", "", 0)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		fmt.Printf("%s\t%s\t%s\n", ticketLabel(task), task.Status, task.Title)
	}
	return nil
}

func runRequirementStatus(status string, args []string) error {
	commandName := map[string]string{"accepted": "accept", "rejected": "reject"}[status]
	if len(args) != 2 || args[0] != "requirement" {
		return fmt.Errorf("usage: ticket %s requirement <id>", commandName)
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
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           current.ParentID,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
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
		Stage:              current.Stage,
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

func runDecision(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ticket decision <add|list>")
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
		task, err := api.CreateTicket(libticket.TicketCreateRequest{
			ProjectID:   project.ID,
			Type:        "decision",
			Title:       args[1],
			Description: args[1],
		})
		if err != nil {
			return err
		}
		printTicket(task)
		return nil
	case "list":
		_, api, project, err := resolveCurrentProjectClient()
		if err != nil {
			return err
		}
		tasks, err := api.ListTicketsFiltered(project.ID, "decision", "", "", "", "", "", 0)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			fmt.Printf("%s\t%s\t%s\n", ticketLabel(task), task.Status, task.Title)
		}
		return nil
	default:
		return fmt.Errorf("unknown decision command %q", args[0])
	}
}

func runConversation(args []string) error {
	if len(args) != 2 || args[0] != "show" {
		return errors.New("usage: ticket conversation show <id>")
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
	taskType := fs.String("type", "task", "task type")
	fs.StringVar(taskType, "t", "task", "task type")
	titleFlag := fs.String("title", "", "task title")
	priority := fs.Int("priority", 1, "task priority")
	fs.IntVar(priority, "p", 1, "task priority")
	estimateEffort := fs.Int("estimate_effort", 0, "estimated effort")
	assignee := fs.String("assignee", "", "task assignee")
	fs.StringVar(assignee, "a", "", "task assignee")
	description := fs.String("description", "", "task description")
	fs.StringVar(description, "d", "", "task description")
	acceptanceCriteria := fs.String("ac", "", "acceptance criteria")
	estimateComplete := fs.String("estimate_complete", "", "estimated completion time (RFC3339)")
	parent := fs.Int64("parent", 0, "parent task id")
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
	return append(flagArgs, positional...), nil
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
	task, err := api.CreateTicket(libticket.TicketCreateRequest{
		ProjectID:          project.ID,
		ParentID:           opts.ParentID,
		Type:               opts.TicketType,
		Title:              opts.Title,
		Description:        opts.Description,
		AcceptanceCriteria: opts.AcceptanceCriteria,
		Priority:           opts.Priority,
		EstimateEffort:     opts.EstimateEffort,
		EstimateComplete:   opts.EstimateComplete,
		Assignee:           opts.Assignee,
	})
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(task)
	}
	if task.Type == "epic" {
		cfg.CurrentEpicID = task.ID
		if err := config.Save(cfg); err != nil {
			return err
		}
	}
	fmt.Println(ticketLabel(task))
	return nil
}

func runConfig(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: ticket config <set|get> <key> [value]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	switch args[0] {
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
		switch args[1] {
		case "server":
			fmt.Println(config.ResolveServerURL(cfg))
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
	default:
		return fmt.Errorf("unknown config action %q", args[0])
	}
}

func printProject(project store.Project) {
	if outputJSON {
		_ = printJSON(project)
		return
	}
	fmt.Printf("project: %s\n", project.Title)
	fmt.Printf("project_id: %d\n", project.ID)
	fmt.Printf("prefix: %s\n", project.Prefix)
	fmt.Printf("status: %s\n", project.Status)
	if project.Description != "" {
		fmt.Printf("description: %s\n", project.Description)
	}
	if project.AcceptanceCriteria != "" {
		fmt.Printf("acceptance_criteria: %s\n", project.AcceptanceCriteria)
	}
}

func printProjectTable(projects []store.Project, currentProjectID string) {
	if len(projects) == 0 {
		fmt.Println("no projects")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPREFIX\tTITLE\tSTATUS\tCURRENT")
	currentID := strings.TrimSpace(currentProjectID)
	for _, project := range projects {
		current := ""
		if strconv.FormatInt(project.ID, 10) == currentID || strings.EqualFold(project.Prefix, currentID) {
			current = "(current)"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", project.ID, project.Prefix, project.Title, project.Status, current)
	}
	_ = w.Flush()
}

func ticketLabel(task store.Ticket) string {
	if strings.TrimSpace(task.Key) != "" {
		return task.Key
	}
	return strconv.FormatInt(task.ID, 10)
}

func printTicket(task store.Ticket) {
	if outputJSON {
		_ = printJSON(task)
		return
	}
	fmt.Printf("ticket: %s\n", task.Title)
	fmt.Printf("id: %d\n", task.ID)
	fmt.Printf("key: %s\n", task.Key)
	fmt.Printf("type: %s\n", task.Type)
	fmt.Printf("status: %s\n", task.Status)
	fmt.Printf("project_id: %d\n", task.ProjectID)
	if task.ParentID != nil {
		fmt.Printf("parent_id: %d\n", *task.ParentID)
	}
	if task.CloneOf != nil {
		fmt.Printf("clone_of: %d\n", *task.CloneOf)
	}
	if task.Description != "" {
		fmt.Printf("description: %s\n", task.Description)
	}
	if task.EstimateEffort != 0 {
		fmt.Printf("estimate_effort: %d\n", task.EstimateEffort)
	}
	if task.EstimateComplete != "" {
		fmt.Printf("estimate_complete: %s\n", task.EstimateComplete)
	}
}

func printTicketDetails(task store.Ticket, dependencies []store.Dependency) {
	parentID := ""
	if task.ParentID != nil {
		parentID = fmt.Sprintf("%d", *task.ParentID)
	}
	dependsOn := formatDependsOn(dependencies)
	fmt.Printf("ID           : %d\n", task.ID)
	fmt.Printf("Key          : %s\n", task.Key)
	fmt.Printf("Type         : %s\n", task.Type)
	fmt.Printf("Description  : %s\n", task.Description)
	fmt.Printf("ParentID     : %s\n", parentID)
	if task.CloneOf != nil {
		fmt.Printf("CloneOf      : %d\n", *task.CloneOf)
	}
	fmt.Printf("ProjectID    : %d\n", task.ProjectID)
	fmt.Printf("Title        : %s\n", task.Title)
	fmt.Printf("Assignee     : %s\n", task.Assignee)
	fmt.Printf("Order        : %d\n", task.Order)
	fmt.Printf("EstimateEffort   : %d\n", task.EstimateEffort)
	fmt.Printf("EstimateComplete : %s\n", task.EstimateComplete)
	fmt.Printf("DependsOn    : %s\n", dependsOn)
	fmt.Printf("Status       : %s\n", task.Status)
	fmt.Printf("Priority     : %d\n", task.Priority)
	fmt.Printf("Created      : %s\n", task.CreatedAt)
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
	mode, err := config.ResolveMode()
	if err == nil && mode == config.ModeLocal {
		if username == "" {
			username = localModeUsername()
		}
		return username, password, nil
	}

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

func localModeUsername() string {
	return "admin"
}

func fallbackCommandUsername() string {
	if mode, err := config.ResolveMode(); err == nil && mode == config.ModeLocal {
		return localModeUsername()
	}
	return currentOSUser()
}

func extractURLOverride(args []string) ([]string, string, error) {
	if len(args) == 0 {
		return args, "", nil
	}
	var out []string
	var override string
	for i := 0; i < len(args); i++ {
		if args[i] == "-url" {
			if i+1 >= len(args) {
				return nil, "", errors.New("missing value for -url")
			}
			override = args[i+1]
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out, override, nil
}

func extractDBOverride(args []string) ([]string, string, error) {
	if len(args) == 0 {
		return args, "", nil
	}
	var out []string
	var override string
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

func extractOutputFlags(args []string) ([]string, bool, bool, error) {
	var out []string
	var jsonFlag bool
	var nocolor bool
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

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
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
	return config.ResolveDatabasePath()
}

func runRemoteStatus(cfg config.Config) error {
	serverURL := strings.TrimSpace(config.ResolveServerURL(cfg))
	if serverURL == "" {
		return errors.New("TICKET_SERVER is required in remote mode")
	}
	svc, err := resolveService(cfg)
	if err != nil {
		return err
	}
	status, err := svc.Status()
	authenticated := err == nil && status.Authenticated
	username := strings.TrimSpace(cfg.Username)
	if status.User != nil {
		username = status.User.Username
	}
	if outputJSON {
		return printJSON(map[string]any{
			"mode":          config.ModeRemote,
			"server":        serverURL,
			"username":      username,
			"authenticated": authenticated,
			"connection":    map[bool]string{true: "success", false: "failure"}[err == nil],
		})
	}
	fmt.Printf("mode: %s\n", config.ModeRemote)
	fmt.Printf("server: %s\n", serverURL)
	fmt.Printf("username: %s\n", username)
	fmt.Printf("authenticated: %t\n", authenticated)
	printConnectionLine(err == nil)
	return err
}

func runLocalStatus() error {
	dbPath, err := config.ResolveDatabasePath()
	if err != nil {
		return err
	}
	_, statErr := os.Stat(dbPath)
	dbExists := statErr == nil
	if outputJSON {
		return printJSON(map[string]any{
			"mode":       config.ModeLocal,
			"db_path":    dbPath,
			"db_exists":  dbExists,
			"connection": map[bool]string{true: "success", false: "failure"}[localStatusCheck(dbPath) == nil],
		})
	}
	fmt.Printf("mode: %s\n", config.ModeLocal)
	fmt.Printf("db_path: %s\n", dbPath)
	fmt.Printf("db_exists: %t\n", dbExists)
	err = localStatusCheck(dbPath)
	printConnectionLine(err == nil)
	if !dbExists {
		fmt.Println("hint: run ticket initdb")
	}
	return err
}

func localStatusCheck(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count); err != nil {
		return err
	}
	return nil
}

func printConnectionLine(ok bool) {
	status := "failure"
	color := "\x1b[31m"
	if ok {
		status = "success"
		color = "\x1b[32m"
	}
	if noColorOutput {
		fmt.Printf("connection: %s\n", status)
		return
	}
	fmt.Printf("connection: %s%s\x1b[0m\n", color, status)
}

func promptForCredentials(in io.Reader, out io.Writer, defaultUsername, defaultPassword string) (string, string, error) {
	reader := bufio.NewReader(in)
	if defaultUsername != "" {
		fmt.Fprintf(out, "username [%s]: ", defaultUsername)
	} else {
		fmt.Fprint(out, "username: ")
	}
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = defaultUsername
	}
	if defaultPassword != "" {
		fmt.Fprint(out, "password [press enter to use default]: ")
	} else {
		fmt.Fprint(out, "password: ")
	}
	password, err := readPasswordPrompt(reader, in, out)
	if err != nil {
		return "", "", err
	}
	if password == "" {
		password = defaultPassword
	}
	return username, password, nil
}

func readPasswordPrompt(reader *bufio.Reader, in io.Reader, out io.Writer) (string, error) {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(password), nil
	}

	oldState, err := term.MakeRaw(int(inFile.Fd()))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = term.Restore(int(inFile.Fd()), oldState)
	}()

	var buf []byte
	single := make([]byte, 1)
	for {
		if _, err := inFile.Read(single); err != nil {
			return "", err
		}
		switch single[0] {
		case '\r', '\n':
			fmt.Fprint(out, "\n")
			return string(buf), nil
		case 3:
			fmt.Fprint(out, "^C\n")
			return "", errors.New("interrupt")
		case 8, 127:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Fprint(out, "\b \b")
			}
		default:
			if single[0] >= 32 && single[0] <= 126 {
				buf = append(buf, single[0])
				fmt.Fprint(out, "*")
			}
		}
	}
}

func resolveLoginUsername(configUsername, usernameFlag string) string {
	if strings.TrimSpace(configUsername) != "" {
		return strings.TrimSpace(configUsername)
	}
	if strings.TrimSpace(usernameFlag) != "" {
		return strings.TrimSpace(usernameFlag)
	}
	return envValue("TICKET_USERNAME")
}

func resolveLoginPassword(passwordFlag string) string {
	if strings.TrimSpace(passwordFlag) != "" {
		return strings.TrimSpace(passwordFlag)
	}
	return envValue("TICKET_PASSWORD")
}

func renderRootUsage() string {
	var b strings.Builder
	b.WriteString(renderBanner())
	b.WriteString(`
USAGE
  ticket <command> [options]

CLIENT COMMANDS
`)
	clientRows := [][2]string{
		{"add", "Create a task in the active project"},
		{"active", "Set a ticket state to active"},
		{"claim", "Assign yourself to a task"},
		{"clone", "Clone a task or epic"},
		{"comment", "Add comments to a task"},
		{"complete", "Set a ticket state to complete"},
		{"count", "Count users, projects, and work by type"},
		{"design", "Set a ticket stage to design"},
		{"dependency", "Manage dependency links between tasks"},
		{"delete", "Delete a task permanently"},
		{"develop", "Set a ticket stage to develop"},
		{"done", "Set a ticket stage to done"},
		{"get", "Show a task with history and comments"},
		{"help", "Show command help"},
		{"health", "Compute ticket health by project-specific heuristics"},
		{"idle", "Set a ticket state to idle"},
		{"list", "List tasks in the active project"},
		{"login", "Log into the server"},
		{"logout", "Clear the local session"},
		{"onboard", "Append the embedded AGENTS.md template in the current directory"},
		{"orphans", "List tasks with no parent"},
		{"project", "Manage projects and active project context"},
		{"register", "Create a user account on the server"},
		{"ticket", "Generate requirements via an external agent"},
		{"request", "Request work for the current user"},
		{"request-dryrun", "Simulate a request assignment without mutation"},
		{"search", "Search tasks in the active project or across all projects"},
		{"set-parent", "Set the parent of a task"},
		{"attach", "Alias for set-parent"},
		{"status", "Show server and authentication status"},
		{"test", "Set a ticket stage to test"},
		{"unset-parent", "Clear the parent of a task"},
		{"detach", "Alias for unset-parent"},
		{"unclaim", "Remove yourself from a task"},
		{"update", "Update a task"},
		{"version", "Print the current version from VERSION"},
	}
	adminRows := [][2]string{
		{"assign", "Admin-only task assignment"},
		{"initdb", "Initialize the database, bootstrap admin, and create the default project"},
		{"server", "Start the API server and embedded web UI"},
		{"unassign", "Admin-only task unassignment"},
		{"user", "Admin-only user management"},
	}
	commandWidth := commandUsageWidth(clientRows, adminRows)
	printCommandUsageRows(&b, clientRows, commandWidth)
	b.WriteString(`
`)
	b.WriteString("ADMIN COMMANDS\n")
	printCommandUsageRows(&b, adminRows, commandWidth)
	b.WriteString(`
HELP
  ticket help <command>
`)
	return strings.TrimSpace(b.String()) + "\n"
}

func commandUsageWidth(groups ...[][2]string) int {
	max := 0
	for _, rows := range groups {
		for _, row := range rows {
			if len(row[0]) > max {
				max = len(row[0])
			}
		}
	}
	if max < 1 {
		return 2
	}
	return max
}

func printCommandUsageRows(b *strings.Builder, rows [][2]string, commandWidth int) {
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintf(w, "  %-*s\t%s\n", commandWidth, row[0], row[1])
	}
	_ = w.Flush()
}

func printCountSummary(summary store.CountSummary, scopedToProject bool) {
	fmt.Printf("users %d\n", summary.Users)
	if !scopedToProject {
		fmt.Printf("projects %d\n", summary.Projects)
	}
	for _, item := range summary.Types {
		fmt.Printf("%ss %d", item.Type, item.Total)
		if suffix := formatStatusCounts(item.Statuses); suffix != "" {
			fmt.Printf(" (%s)", suffix)
		}
		fmt.Println()
	}
}

func printTicketTable(tasks []store.Ticket, dependencies map[int64]string, statusUnicode bool) {
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MOON\tKEY\tTYPE\tSTATUS\tPARENT_ID\tASSIGNEE\tPRIORITY\tDEPENDSON\tHEALTH\tTITLE")
	for _, task := range tasks {
		symbol := formatTicketStatusSymbol(task.Status, statusUnicode)
		assignee := task.Assignee
		if strings.TrimSpace(assignee) == "" {
			assignee = "-"
		}
		dependsOn := dependencies[task.ID]
		if dependsOn == "[]" {
			dependsOn = ""
		}
		parentID := ""
		if task.ParentID != nil {
			parentID = strconv.FormatInt(*task.ParentID, 10)
		}
		key := task.Key
		if strings.TrimSpace(key) == "" {
			key = strconv.FormatInt(task.ID, 10)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%.2f\t%s\n", symbol, key, task.Type, task.Status, parentID, assignee, task.Priority, dependsOn, float64(task.HealthScore)/4.0, task.Title)
	}
	_ = w.Flush()
}

func formatTicketStatusSymbol(status string, useUnicode bool) string {
	if !useUnicode {
		return ""
	}
	stage, state, err := store.ParseLifecycleStatus(status)
	if err != nil {
		return ""
	}
	switch {
	case stage == store.StageDesign && state == store.StateIdle:
		return "○"
	case stage == store.StageDevelop && state == store.StateIdle:
		return "○"
	case state == store.StateActive:
		return "◑"
	case state == store.StateComplete:
		return "◉"
	default:
		return ""
	}
}

func formatStatusCounts(statuses map[string]int) string {
	order := []string{"design/idle", "design/active", "design/complete", "develop/idle", "develop/active", "develop/complete", "test/idle", "test/active", "test/complete", "done/complete"}
	labels := map[string]string{
		"design/idle":      "design/idle",
		"design/active":    "design/active",
		"design/complete":  "design/complete",
		"develop/idle":     "develop/idle",
		"develop/active":   "develop/active",
		"develop/complete": "develop/complete",
		"test/idle":        "test/idle",
		"test/active":      "test/active",
		"test/complete":    "test/complete",
		"done/complete":    "done/complete",
	}
	var parts []string
	for _, status := range order {
		if count := statuses[status]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, labels[status]))
		}
	}
	return strings.Join(parts, ", ")
}

func renderBanner() string {
	lines := bannerLines(selectBannerWord())
	var b strings.Builder
	for i, line := range lines {
		color := bannerColors[i%len(bannerColors)]
		b.WriteString(color)
		b.WriteString(line)
		b.WriteString("\x1b[0m\n")
	}
	b.WriteString("\n")
	return b.String()
}

func randomBannerWord() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(bannerWords))))
	if err != nil {
		return bannerWords[0]
	}
	return bannerWords[n.Int64()]
}

func bannerLines(word string) []string {
	rows := make([]string, 6)
	upper := strings.ToUpper(strings.TrimSpace(word))
	for _, char := range upper {
		glyph, ok := bannerGlyphs[char]
		if !ok {
			continue
		}
		for i := range rows {
			if rows[i] != "" {
				rows[i] += "  "
			}
			rows[i] += glyph[i]
		}
	}
	return rows
}

func renderCommandHelp(command string) string {
	command = normalizeHelpCommand(command)
	info, ok := helpIndex[command]
	if !ok {
		return renderRootUsage()
	}
	var b strings.Builder
	b.WriteString("USAGE\n  ")
	b.WriteString(info.usage)
	b.WriteString("\n\n")
	if len(info.details) > 0 {
		b.WriteString("DETAILS\n")
		for _, line := range info.details {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("EXAMPLE\n  ")
	b.WriteString(info.example)
	b.WriteString("\n")
	return b.String()
}

func hasCommandHelp(command string) bool {
	_, ok := helpIndex[normalizeHelpCommand(command)]
	return ok
}

func normalizeHelpCommand(command string) string {
	switch command {
	case "show":
		return "get"
	case "create", "new":
		return "add"
	case "ls":
		return "list"
	case "cp":
		return "clone"
	case "rm":
		return "delete"
	default:
		return command
	}
}
