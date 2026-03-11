package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	osuser "os/user"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

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
	fetchRepoVersion  = defaultFetchRepoVersion
)

const repoVersionURL = "https://raw.githubusercontent.com/simonski/ticket/refs/heads/main/cmd/ticket/VERSION"

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
		details: []string{"Prints the embedded onboarding template to stdout."},
		example: "ticket onboard",
	},
	"init": {
		usage:   "ticket init [-f <db-path>] [--force] [-password <password>] [--populate]",
		details: []string{"Creates a new SQLite database, bootstraps the fixed `admin` account, and creates the default project.", "If `-f` is omitted, the database is created at `$TICKET_HOME/ticket.db`.", "If `-password` is omitted, a random admin password is generated and printed to stdout.", "If `--force` is supplied, any existing database file is overwritten.", "If `--populate` is supplied, example projects/stories/tickets/users/teams are also seeded."},
		example: "ticket init -f $TICKET_HOME/ticket.db --force -password secret --populate",
	},
	"server": {
		usage:   "ticket server [-f <db-path>] [-p <port>] [-addr <host:port>] [-v]",
		details: []string{"Starts the HTTP API server and the embedded web UI.", "If `-f` is omitted, the server uses `$TICKET_HOME/ticket.db`.", "Use `-p` as a shorthand port flag (for example `-p 9999`); `-addr` is still supported for explicit host/port binding.", "If `-v` is supplied, requests and responses are printed verbosely to stdout."},
		example: "ticket server -f $TICKET_HOME/ticket.db -p 9999 -v",
	},
	"version": {
		usage:   "ticket version",
		details: []string{"Prints the semantic version embedded into the binary from the build-time `VERSION` file."},
		example: "ticket version",
	},
	"upgrade": {
		usage:   "ticket upgrade",
		details: []string{"Checks the repository VERSION file and compares it to the embedded local version.", "The network check fails fast after 3 seconds if the repository cannot be reached."},
		example: "ticket upgrade",
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
		usage:   "ticket project <create|list|get|use|add-user|remove-user|add-team|remove-team>|<id> <update|enable|disable>",
		details: []string{"Manages projects and the active project context used by subsequent commands.", "Projects are addressed by prefix or numeric id.", "Project membership supports both users and teams."},
		example: "ticket project CUS update -title \"Customer Portal\"",
	},
	"team": {
		usage:   "ticket team <list|create|update|delete|add-user|remove-user|users|add-agent|remove-agent|agents>",
		details: []string{"Manages team hierarchy, team users (member/owner + job title), and team agent assignments.", "Teams can be assigned to projects with `ticket project add-team`."},
		example: "ticket team create -name \"Platform\"",
	},
	"list": {
		usage:   "ticket list|ls [--type <type>] [--stage <stage>] [--state <state>] [--status <stage/state>] [-u <user>] [-n <limit>] [-a] [--unicode] [--plain]",
		details: []string{"Lists tickets in the active project with optional type, lifecycle, assignee, and limit filters.", "`status` is a rendered composite such as `develop/active`. `-n` is applied server-side. `0` means no limit.", "By default archived tickets are hidden; use `-a` to include them."},
		example: "ticket list --type bug --status develop/idle -u alice -n 20",
	},
	"orphans": {
		usage:   "ticket orphans [-url <server-url>]",
		details: []string{"Lists unparented non-epic tickets in the active project."},
		example: "ticket orphans",
	},
	"get": {
		usage:   "ticket get -id <id> [-url <server-url>]",
		details: []string{"Shows a single ticket with comments and history.", "Output uses subtle color unless `-nocolor` is supplied."},
		example: "ticket get -id 42",
	},
	"show": {
		usage:   "ticket show -id <id>",
		details: []string{"Alias for `ticket get`."},
		example: "ticket show -id 42",
	},
	"search": {
		usage:   "ticket search <free form query> [-stage <stage>] [-state <state>] [-status <stage/state>] [-title <text>] [-description <text>] [-priority <n>] [-owner <user>] [-allprojects]",
		details: []string{"Searches tickets in the active project by default.", "Use `-allprojects` to search across every project. Optional filters narrow by lifecycle, title text, description text, priority, and owner."},
		example: "ticket search password reset -status develop/active -owner alice -allprojects",
	},
	"update": {
		usage: "ticket update -id <id>\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-ac <acceptance-criteria>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-stage <stage>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]",
		details: []string{
			"-id <id>: required; ticket id or key",
			"-title <title>: set title",
			"-desc <description>: set description (alias: -description)",
			"-description <description>: set description (alias: -desc)",
			"-ac <acceptance-criteria>: set acceptance criteria",
			"-priority <n>: set numeric priority",
			"-order <n>: set numeric sort order",
			"-stage <stage>: valid values [design, develop, test, done]",
			"-state <state>: valid values [idle, active, success, fail]",
			"-status <stage/state>: valid format [design|develop|test|done]/[idle|active|success|fail]",
			"-parent_id <id>: set parent ticket id",
			"-estimate_effort <n>: set numeric estimate effort",
			"-estimate_complete <rfc3339>: set completion timestamp (example 2026-03-31T17:00:00Z)",
		},
		example: "ticket update -id 42 -title \"Customer Portal\" -status develop/active -priority 2 -estimate_effort 5",
	},
	"set-parent": {
		usage:   "ticket set-parent -id <id> <parent-id>",
		details: []string{"Sets the parent of a ticket or epic.", "Both ids must be numeric ticket ids in the active project.", "If the child is an epic, the parent must also be an epic."},
		example: "ticket set-parent -id 1 2",
	},
	"attach": {
		usage:   "ticket attach -id <id> <parent-id>",
		details: []string{"Alias for `ticket set-parent`."},
		example: "ticket attach -id CUS-T-12 CUS-E-3",
	},
	"unset-parent": {
		usage:   "ticket unset-parent -id <id>",
		details: []string{"Clears the parent of a ticket or story.", "After this, the ticket becomes an orphan."},
		example: "ticket unset-parent -id 1",
	},
	"detach": {
		usage:   "ticket detach -id <id>",
		details: []string{"Alias for `ticket unset-parent`."},
		example: "ticket detach -id CUS-T-12",
	},
	"design": {
		usage:   "ticket design -id <id>",
		details: []string{"Sets the ticket stage to `design` and the state to `idle`."},
		example: "ticket design -id 42",
	},
	"stage": {
		usage:   "ticket stage -id <id> <design|develop|test|done>",
		details: []string{"Sets a ticket stage directly. `done` sets state to `success`; other stages set state to `idle`."},
		example: "ticket stage -id 42 develop",
	},
	"develop": {
		usage:   "ticket develop -id <id>",
		details: []string{"Sets the ticket stage to `develop` and the state to `idle`."},
		example: "ticket develop -id 42",
	},
	"test": {
		usage:   "ticket test -id <id>",
		details: []string{"Sets the ticket stage to `test` and the state to `idle`."},
		example: "ticket test -id 42",
	},
	"done": {
		usage:   "ticket done -id <id>",
		details: []string{"Sets the ticket stage to `done` and the state to `success`."},
		example: "ticket done -id 42",
	},
	"idle": {
		usage:   "ticket idle -id <id>",
		details: []string{"Sets the ticket state to `idle` without changing the stage."},
		example: "ticket idle -id 42",
	},
	"state": {
		usage:   "ticket state -id <id> <idle|active|success|fail>",
		details: []string{"Sets a ticket state directly while preserving the current stage."},
		example: "ticket state -id 42 active",
	},
	"active": {
		usage:   "ticket active -id <id>",
		details: []string{"Sets the ticket state to `active` without changing the stage.", "`active` requires an assignee; if the ticket is unassigned the CLI claims it for the current user first."},
		example: "ticket active -id 42",
	},
	"complete": {
		usage:   "ticket complete -id <id>",
		details: []string{"Sets the ticket state to `success` without changing the stage."},
		example: "ticket complete -id 42",
	},
	"add": {
		usage:   "ticket add|create|new [-title <title>] [-t <type>] [-p <priority>] [-a <assignee>] [-d <description>] [-ac <criteria>] [-parent <id>] [-project <project>] [-estimate_effort <n>] [-estimate_complete <rfc3339>] [title words]",
		details: []string{"Creates a ticket-like entity in the active project.", "Positional title words and `-title` are equivalent ways to set the title.", "Defaults: `type=ticket`, `stage=design`, `state=idle`, `priority=1`, blank assignee, blank description, blank acceptance criteria, blank parent, current project, `estimate_effort=0`, blank `estimate_complete`."},
		example: "ticket add \"Customers can reset their password.\"",
	},
	"comment": {
		usage:   "ticket comment add -id <id> \"comment\"",
		details: []string{"Adds a comment to a ticket and records a corresponding history event."},
		example: "ticket comment add -id 42 \"Need product sign-off.\"",
	},
	"clone": {
		usage:   "ticket clone|cp <id>",
		details: []string{"Clones a ticket or epic.", "Cloned items are unassigned, reset to `design/idle`, and keep a `clone_of` reference to the source item. Cloning an epic also clones its child tickets."},
		example: "ticket clone 42",
	},
	"close": {
		usage:   "ticket close -id <id>",
		details: []string{"Closes a ticket so it remains visible but frozen.", "Closed tickets cannot be modified until reopened."},
		example: "ticket close -id TK-1",
	},
	"open": {
		usage:   "ticket open -id <id>",
		details: []string{"Reopens a closed ticket so it can be updated again.", "Open and close actions are recorded in ticket history."},
		example: "ticket open -id TK-1",
	},
	"archive": {
		usage:   "ticket archive -id <id>",
		details: []string{"Archives a ticket.", "Archived tickets are hidden from default `ticket ls` output."},
		example: "ticket archive -id TK-1",
	},
	"unarchive": {
		usage:   "ticket unarchive -id <id>",
		details: []string{"Unarchives a ticket so it appears in default `ticket ls` output."},
		example: "ticket unarchive -id TK-1",
	},
	"delete": {
		usage:   "ticket rm|delete -id <id>",
		details: []string{"Deletes a ticket permanently.", "Fails if the ticket still has child tickets."},
		example: "ticket delete -id 42",
	},
	"assign": {
		usage:   "ticket assign <id> <name>",
		details: []string{"Admin-only command that assigns a ticket to a user.", "The target user must exist and be enabled."},
		example: "ticket assign 42 alice",
	},
	"unassign": {
		usage:   "ticket unassign <id> <name>",
		details: []string{"Admin-only command that clears a ticket assignment from the named user.", "The named user must exist and be enabled."},
		example: "ticket unassign 42 alice",
	},
	"claim": {
		usage:   "ticket claim <id>",
		details: []string{"Assigns the caller to the ticket.", "Fails if the ticket is already assigned to another user."},
		example: "ticket claim 42",
	},
	"unclaim": {
		usage:   "ticket unclaim <id>",
		details: []string{"Clears the caller's assignment from the ticket.", "Fails unless the caller is the current assignee."},
		example: "ticket unclaim 42",
	},
	"add-dependency": {
		usage:   "ticket add-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Adds one or more `depends_on` links from the ticket to the listed ticket IDs.", "Comma-separated dependency IDs are supported."},
		example: "ticket add-dependency 4 1,2,3",
	},
	"remove-dependency": {
		usage:   "ticket remove-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Removes one or more `depends_on` links from the ticket to the listed ticket IDs.", "Comma-separated dependency IDs are supported."},
		example: "ticket remove-dependency 4 2",
	},
	"dependency": {
		usage:   "ticket dependency <add|remove> -id <id> <dependency-id[,dependency-id...]>",
		details: []string{"Manages `depends_on` links for a ticket.", "`add` creates dependency links; `remove` deletes them."},
		example: "ticket dependency add -id 4 1,2,3",
	},
	"request": {
		usage:   "ticket request [--dryrun] [<id>]",
		details: []string{"Requests work for the current user.", "With an id, the server attempts to assign that specific ticket. Without an id, it resumes the user's oldest assigned `develop/active` ticket, then assigned `develop/idle` work, then assigns the oldest unassigned `develop/idle` ticket in the active project."},
		example: "ticket request 42",
	},
	"request-dryrun": {
		usage:   "ticket request-dryrun [<id>]",
		details: []string{"Simulates a request assignment without mutating state and shows what ticket would be assigned."},
		example: "ticket request-dryrun 42",
	},
	"user": {
		usage:   "ticket user <create|ls|list|rm|delete|enable|disable>",
		details: []string{"Admin-only user management commands.", "If a non-admin user calls these commands, the server returns 403 with `user is not an admin`."},
		example: "ticket user create --username alice --password secret",
	},
	"agent": {
		usage:   "ticket agent <create|ls|list|update|rm|delete|enable|disable|request|run>",
		details: []string{"Manages API agents for autonomous ticket processing.", "`request` fetches an enriched work envelope (project, ticket, parents). `run` registers an agent then continuously requests and processes work."},
		example: "ticket agent create -name worker-1 -description \"Autonomous worker\"",
	},
	"config": {
		usage:   "ticket config <set|get|ls|list|rm|delete|registration-enable|registration-disable> [key] [value]",
		details: []string{"Local config supports `set/get/ls/rm` for keys: `server`, `username`, `current_project`, `current_epic_id`.", "Registration controls are server-backed and require admin privileges in remote mode."},
		example: "ticket config ls",
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
		return runInitDB(trimmedArgs[1:])
	case "initdb":
		return errors.New("use `ticket init`")
	case "server":
		return runServer(trimmedArgs[1:])
	case "version":
		return runVersion(trimmedArgs[1:])
	case "upgrade":
		return runUpgrade(trimmedArgs[1:])
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
	case "agent":
		return runAgent(trimmedArgs[1:])
	case "user":
		return runUser(trimmedArgs[1:])
	case "project":
		return runProject(trimmedArgs[1:])
	case "team":
		return runTeam(trimmedArgs[1:])
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
		return runSetParent(trimmedArgs[1:], trimmedArgs[0])
	case "unset-parent", "detach":
		return runUnsetParent(trimmedArgs[1:], trimmedArgs[0])
	case "design":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDesign, "design")
	case "develop":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDevelop, trimmedArgs[0])
	case "test":
		return runTicketStageAlias(trimmedArgs[1:], store.StageTest, trimmedArgs[0])
	case "done":
		return runTicketStageAlias(trimmedArgs[1:], store.StageDone, trimmedArgs[0])
	case "stage":
		return runTicketStage(trimmedArgs[1:])
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
	case "close":
		return runSetTicketClosed(trimmedArgs[1:], true)
	case "open":
		return runSetTicketClosed(trimmedArgs[1:], false)
	case "archive":
		return runSetTicketArchived(trimmedArgs[1:], true)
	case "unarchive":
		return runSetTicketArchived(trimmedArgs[1:], false)
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
	cfg.ServerURL = config.ResolveServerURL(cfg)
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

	srv, err := server.New(listenAddr, db, strings.TrimSpace(embeddedVersion), *verbose, os.Stdout)
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

func runAgent(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ticket agent <create|ls|list|update|rm|delete|enable|disable|request|run>")
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
		name := fs.String("name", "", "agent name")
		description := fs.String("description", "", "agent description")
		password := fs.String("password", "", "agent password")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("agent create requires -name")
		}
		if strings.TrimSpace(*description) == "" {
			return errors.New("agent create requires -description")
		}
		agent, generatedPassword, err := svc.CreateAgent(libticket.AgentCreateRequest{
			Name:        strings.TrimSpace(*name),
			Description: strings.TrimSpace(*description),
			Password:    strings.TrimSpace(*password),
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(map[string]any{"agent": agent, "password": generatedPassword})
		}
		fmt.Printf("agent_id: %d\n", agent.ID)
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
		var name, description, password string
		fs.StringVar(&name, "name", "", "agent name")
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
		nameSet := visited["name"]
		descriptionSet := visited["desc"] || visited["description"]
		passwordSet := visited["password"]
		if !nameSet && !descriptionSet && !passwordSet {
			return errors.New("agent update requires at least one of -name, -desc|-description, -password")
		}
		var namePtr, descPtr, passPtr *string
		if nameSet {
			trimmed := strings.TrimSpace(name)
			namePtr = &trimmed
		}
		if descriptionSet {
			trimmed := strings.TrimSpace(description)
			descPtr = &trimmed
		}
		if passwordSet {
			trimmed := strings.TrimSpace(password)
			passPtr = &trimmed
		}
		agent, err := svc.UpdateAgent(*id, libticket.AgentUpdateRequest{
			Name:        namePtr,
			Description: descPtr,
			Password:    passPtr,
		})
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(agent)
		}
		fmt.Printf("updated agent %d\n", agent.ID)
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
		fmt.Printf("%sd agent %d\n", args[0], agent.ID)
		return nil
	case "run":
		fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "agent name")
		password := fs.String("password", "", "agent password")
		url := fs.String("url", "", "ticket server url")
		projectID := fs.Int64("project-id", 0, "project id override")
		pollSeconds := fs.Int("poll-seconds", 2, "idle poll interval seconds")
		llmCommand := fs.String("llm", envValue("TICKET_AGENT_LLM"), "llm command (default codex)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agentName := strings.TrimSpace(*name)
		if agentName == "" {
			agentName = envValue("AGENT_NAME")
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
		if agentName == "" {
			missing = append(missing, "AGENT_NAME or -name")
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
		if err := os.Setenv("TICKET_MODE", string(config.ModeRemote)); err != nil {
			return err
		}
		if err := os.Setenv("TICKET_SERVER", serverURL); err != nil {
			return err
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
		agent, err := svc.RegisterAgent(libticket.AgentRegisterRequest{
			Name:     agentName,
			Password: agentPassword,
		})
		if err != nil {
			return err
		}
		if !outputJSON {
			fmt.Printf("agent %s registered (id=%d)\n", agent.Name, agent.ID)
		}
		modelCommand := strings.TrimSpace(*llmCommand)
		if modelCommand == "" {
			modelCommand = "codex"
		}
		idleDelay := time.Duration(*pollSeconds) * time.Second
		for {
			response, err := svc.RequestAgentWork(libticket.AgentRequest{
				Name:      agentName,
				Password:  agentPassword,
				ProjectID: *projectID,
			})
			if err != nil {
				return err
			}
			if (response.Status != "NEW" && response.Status != "CURRENT") || response.Ticket == nil {
				time.Sleep(idleDelay)
				continue
			}
			ticket := response.Ticket
			prompt := buildAgentPrompt(*ticket)
			result, err := runAgentCommand(modelCommand, prompt)
			if err != nil {
				return fmt.Errorf("agent llm processing failed for ticket %s: %w", ticketLabel(*ticket), err)
			}
			updated, err := svc.AgentUpdateTicket(ticket.ID, libticket.AgentTicketUpdateRequest{
				Name:     agentName,
				Password: agentPassword,
				Result:   strings.TrimSpace(result),
			})
			if err != nil {
				return err
			}
			if outputJSON {
				if err := printJSON(updated); err != nil {
					return err
				}
			} else {
				fmt.Printf("completed %s -> %s\n", ticketLabel(*ticket), updated.Status)
			}
		}
	case "request":
		fs := flag.NewFlagSet("agent request", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "agent name")
		password := fs.String("password", "", "agent password")
		url := fs.String("url", "", "ticket server url")
		id := fs.Int64("id", 0, "specific ticket id")
		dryRun := fs.Bool("dryrun", false, "simulate assignment only")
		loop := fs.Int("loop", 1, "number of request loops")
		sleepSeconds := fs.Int("sleep", 1, "sleep seconds between loops")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		agentName := strings.TrimSpace(*name)
		if agentName == "" {
			agentName = envValue("AGENT_NAME")
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
		if agentName == "" {
			missing = append(missing, "AGENT_NAME or -name")
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
		if err := os.Setenv("TICKET_MODE", string(config.ModeRemote)); err != nil {
			return err
		}
		if err := os.Setenv("TICKET_SERVER", serverURL); err != nil {
			return err
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
				Name:     agentName,
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
	default:
		return fmt.Errorf("unknown agent command %q", args[0])
	}
}

func buildAgentPrompt(ticket store.Ticket) string {
	var b strings.Builder
	b.WriteString("You are an autonomous software agent working a ticket.\n")
	b.WriteString("Return only the final ticket update text.\n\n")
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
		return strings.ToLower(agents[i].Name) < strings.ToLower(agents[j].Name)
	})
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tENABLED\tSTATUS\tLAST_SEEN")
	for _, agent := range agents {
		lastSeen := strings.TrimSpace(agent.LastSeen)
		if lastSeen == "" {
			lastSeen = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%s\t%s\n", agent.ID, agent.Name, agent.Description, agent.Enabled, agent.Status, lastSeen)
	}
	_ = w.Flush()
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
		description := fs.String("description", "", "project description")
		acceptanceCriteria := fs.String("ac", "", "project acceptance criteria")
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitBranch := fs.String("git-branch", "", "project git branch")
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
			GitRepository:      strings.TrimSpace(*gitRepository),
			GitBranch:          strings.TrimSpace(*gitBranch),
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
		return fmt.Errorf("unknown team command %q", args[0])
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
		gitRepository := fs.String("git-repository", "", "project git repository")
		gitBranch := fs.String("git-branch", "", "project git branch")
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
		project, err := svc.UpdateProject(projectID, libticket.ProjectUpdateRequest{
			Title:              *title,
			Description:        nextDescription,
			AcceptanceCriteria: nextAC,
			GitRepository:      nextRepo,
			GitBranch:          nextBranch,
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
	taskType := fs.String("type", "", "filter by ticket type")
	stage := fs.String("stage", "", "filter by ticket stage")
	state := fs.String("state", "", "filter by ticket state")
	status := fs.String("status", "", "filter by rendered ticket status")
	assignee := fs.String("user", "", "filter by assignee")
	fs.StringVar(assignee, "u", "", "filter by assignee")
	limit := fs.Int("n", 0, "maximum number of tickets to return; 0 means all")
	useUnicode := fs.Bool("unicode", true, "render status symbols as unicode")
	plain := fs.Bool("plain", false, "render status as plain text")
	includeArchived := fs.Bool("a", false, "include archived tickets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 0 {
		return errors.New("usage: ticket list|ls [--type <type>] [--stage <stage>] [--state <state>] [--status <stage/state>] [-u <user>] [-n <limit>] [-a]")
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
	tickets, err := api.ListTicketsFiltered(project.ID, *taskType, resolvedStage, resolvedState, "", "", *assignee, *limit, *includeArchived)
	if err != nil {
		return err
	}
	dependenciesByTicket := make(map[int64]string, len(tickets))
	for _, ticket := range tickets {
		dependencies, err := api.ListDependencies(ticket.ID)
		if err != nil {
			return err
		}
		dependenciesByTicket[ticket.ID] = formatDependsOn(dependencies)
	}
	if outputJSON {
		return printJSON(tickets)
	}
	printTicketTable(tickets, dependenciesByTicket, statusUnicode, *includeArchived)
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
	usage := "ticket get -id <id>"
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	if fs.NArg() != 0 {
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
	printTicketDetails(ticket, dependencies, history)
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
	usage := fmt.Sprintf("ticket %s -id <id> <parent-id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	if fs.NArg() != 1 {
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
	child, err := svc.GetTicket(strings.TrimSpace(*id))
	if err != nil {
		return err
	}
	parent, err := svc.GetTicket(fs.Args()[0])
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
	usage := fmt.Sprintf("ticket %s -id <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 0 {
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

func runTicketStageAlias(args []string, stage, command string) error {
	fs := flag.NewFlagSet("ticket "+command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: ticket %s -id <id>", command)
	}
	return updateTicketStage(strings.TrimSpace(*id), stage)
}

func runTicketStateAlias(args []string, state, command string) error {
	fs := flag.NewFlagSet("ticket "+command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: ticket %s -id <id>", command)
	}
	return updateTicketState(strings.TrimSpace(*id), state)
}

func runTicketStage(args []string) error {
	usage := "ticket stage -id <id> <design|develop|test|done>"
	fs := flag.NewFlagSet("ticket stage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
		return errors.New("usage: " + usage)
	}
	switch strings.ToLower(strings.TrimSpace(fs.Args()[0])) {
	case store.StageDesign, store.StageDevelop, store.StageTest, store.StageDone:
		return updateTicketStage(strings.TrimSpace(*id), strings.ToLower(strings.TrimSpace(fs.Args()[0])))
	default:
		return errors.New("usage: " + usage)
	}
}

func runTicketState(args []string) error {
	usage := "ticket state -id <id> <idle|active|success|fail>"
	fs := flag.NewFlagSet("ticket state", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 1 {
		return errors.New("usage: " + usage)
	}
	switch strings.ToLower(strings.TrimSpace(fs.Args()[0])) {
	case store.StateIdle, store.StateActive, store.StateSuccess, store.StateFail:
		return updateTicketState(strings.TrimSpace(*id), strings.ToLower(strings.TrimSpace(fs.Args()[0])))
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
	nextState := store.StateIdle
	if stage == store.StageDone {
		nextState = store.StateSuccess
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
	usage := "ticket update -id <id>\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-ac <acceptance-criteria>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-stage <stage>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]"
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
	stage := fs.String("stage", "", "ticket stage")
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
	hasStage := containsFlag(args, "-stage")
	hasState := containsFlag(args, "-state")
	hasParentID := containsFlag(args, "-parent_id")
	if !hasTitle && !hasDescription && !hasDesc && !hasAC && !hasGitRepository && !hasGitBranch && !hasPriority && !hasOrder && !hasEstimateEffort && !hasEstimateComplete && !hasStatus && !hasStage && !hasState && !hasParentID {
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
	history, _ := svc.ListHistory(updated.ID)
	printTicketDetails(updated, dependencies, history)
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
	ticket, err := svc.GetTicket(args[0])
	if err != nil {
		return err
	}
	events, err := svc.ListHistory(ticket.ID)
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

	ticket, err := svc.GetTicket(args[0])
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
	usage := "ticket dependency <add|remove> -id <id> <dependency-id[,dependency-id...]>"
	if len(args) == 0 {
		return errors.New("usage: " + usage)
	}
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
			return errors.New("usage: " + usage)
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
			return errors.New("usage: " + usage)
		}
		return runDependencyCommand([]string{strings.TrimSpace(*id), strings.TrimSpace(fs.Args()[0])}, false)
	default:
		if action == "" {
			return errors.New("usage: " + usage)
		}
		return fmt.Errorf("unknown dependency action %q", action)
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
		return errors.New("usage: ticket comment add -id <id> \"comment\"")
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("ticket comment add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "ticket id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		commentValues := fs.Args()
		if strings.TrimSpace(*id) == "" || len(commentValues) != 1 {
			return errors.New("usage: ticket comment add -id <id> \"comment\"")
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
		comment, err := svc.AddComment(ticket.ID, commentValues[0])
		if err != nil {
			return err
		}
		if outputJSON {
			return printJSON(comment)
		}
		fmt.Printf("commented on %s: %s\n", ticketLabel(ticket), comment.Comment)
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
	ticket, err := svc.CloneTicket(taskRef.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return printJSON(ticket)
	}
	printTicket(ticket)
	return nil
}

func runDeleteTicket(args []string) error {
	usage := "ticket rm|delete -id <id>"
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("usage: " + usage)
	}
	if fs.NArg() != 0 {
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
	usage := fmt.Sprintf("ticket %s -id <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 0 {
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
	usage := fmt.Sprintf("ticket %s -id <id>", command)
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "ticket id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" || fs.NArg() != 0 {
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
	tickets, err := api.ListTicketsFiltered(project.ID, "requirement", "", "", *status, "", "", 0, false)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		fmt.Printf("%s\t%s\t%s\n", ticketLabel(ticket), ticket.Status, ticket.Title)
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
	if len(args) < 1 {
		return errors.New("usage: ticket config <set|get|ls|list|rm|delete|registration-enable|registration-disable> [key] [value]")
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
			fmt.Println(config.ResolveServerURL(cfg))
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
		fmt.Printf("server=%s\n", config.ResolveServerURL(cfg))
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
	if project.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", project.GitRepository)
	}
	if project.GitBranch != "" {
		fmt.Printf("git_branch: %s\n", project.GitBranch)
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

func ticketLabel(ticket store.Ticket) string {
	if strings.TrimSpace(ticket.Key) != "" {
		return ticket.Key
	}
	return strconv.FormatInt(ticket.ID, 10)
}

func printTicket(ticket store.Ticket) {
	if outputJSON {
		_ = printJSON(ticket)
		return
	}
	fmt.Printf("ticket: %s\n", ticket.Title)
	fmt.Printf("id: %d\n", ticket.ID)
	fmt.Printf("key: %s\n", ticket.Key)
	fmt.Printf("type: %s\n", ticket.Type)
	fmt.Printf("status: %s\n", ticket.Status)
	fmt.Printf("open: %s\n", ticketOpenLabel(ticket))
	fmt.Printf("archived: %t\n", ticket.Archived)
	fmt.Printf("project_id: %d\n", ticket.ProjectID)
	if ticket.ParentID != nil {
		fmt.Printf("parent_id: %d\n", *ticket.ParentID)
	}
	if ticket.CloneOf != nil {
		fmt.Printf("clone_of: %d\n", *ticket.CloneOf)
	}
	if ticket.Description != "" {
		fmt.Printf("description: %s\n", ticket.Description)
	}
	if ticket.AcceptanceCriteria != "" {
		fmt.Printf("acceptance_criteria: %s\n", ticket.AcceptanceCriteria)
	}
	if ticket.GitRepository != "" {
		fmt.Printf("git_repository: %s\n", ticket.GitRepository)
	}
	if ticket.GitBranch != "" {
		fmt.Printf("git_branch: %s\n", ticket.GitBranch)
	}
	if ticket.EstimateEffort != 0 {
		fmt.Printf("estimate_effort: %d\n", ticket.EstimateEffort)
	}
	if ticket.EstimateComplete != "" {
		fmt.Printf("estimate_complete: %s\n", ticket.EstimateComplete)
	}
}

func printTicketDetails(ticket store.Ticket, dependencies []store.Dependency, history []store.HistoryEvent) {
	parentID := ""
	if ticket.ParentID != nil {
		parentID = fmt.Sprintf("%d", *ticket.ParentID)
	}
	dependsOn := formatDependsOn(dependencies)
	fmt.Printf("ID           : %d\n", ticket.ID)
	fmt.Printf("Key          : %s\n", ticket.Key)
	fmt.Printf("Type         : %s\n", ticket.Type)
	fmt.Printf("Description  : %s\n", ticket.Description)
	fmt.Printf("ParentID     : %s\n", parentID)
	if ticket.CloneOf != nil {
		fmt.Printf("CloneOf      : %d\n", *ticket.CloneOf)
	}
	fmt.Printf("ProjectID    : %d\n", ticket.ProjectID)
	fmt.Printf("Title        : %s\n", ticket.Title)
	fmt.Printf("Assignee     : %s\n", ticket.Assignee)
	fmt.Printf("Order        : %d\n", ticket.Order)
	fmt.Printf("EstimateEffort   : %d\n", ticket.EstimateEffort)
	fmt.Printf("EstimateComplete : %s\n", ticket.EstimateComplete)
	fmt.Printf("DependsOn    : %s\n", dependsOn)
	fmt.Printf("Status       : %s\n", ticket.Status)
	fmt.Printf("Stage        : %s\n", ticket.Stage)
	fmt.Printf("State        : %s\n", ticket.State)
	fmt.Printf("Open         : %s\n", ticketOpenLabel(ticket))
	fmt.Printf("Archived     : %t\n", ticket.Archived)
	fmt.Printf("Priority     : %d\n", ticket.Priority)
	fmt.Printf("Created      : %s\n", ticket.CreatedAt)
	fmt.Printf("LastModified : %s\n", ticket.UpdatedAt)
	fmt.Printf("Acceptance Criteria : %s\n", ticket.AcceptanceCriteria)
	if len(ticket.Comments) > 0 {
		fmt.Println("Comments     :")
		for _, comment := range ticket.Comments {
			fmt.Printf("  - [%s] %s: %s\n", comment.CreatedAt, comment.Author, comment.Text)
		}
	}
	if len(history) > 0 {
		fmt.Println("History      :")
		for _, event := range history {
			fmt.Printf("  - [%s] %s by %d", event.CreatedAt, event.EventType, event.CreatedBy)
			if strings.TrimSpace(event.Payload) != "" && event.Payload != "{}" {
				fmt.Printf(": %s", event.Payload)
			}
			fmt.Println()
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
		fmt.Println("hint: run ticket init")
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
		{"add", "Create a ticket in the active project"},
		{"active", "Set a ticket state to active"},
		{"archive", "Archive a ticket"},
		{"claim", "Assign yourself to a ticket"},
		{"clone", "Clone a ticket or epic"},
		{"close", "Close a ticket and freeze modifications"},
		{"comment", "Add comments to a ticket"},
		{"complete", "Set a ticket state to success"},
		{"config", "Manage local config keys and registration controls"},
		{"count", "Count users, projects, and work by type"},
		{"design", "Set a ticket stage to design"},
		{"dependency", "Manage dependency links between tickets"},
		{"delete", "Delete a ticket permanently"},
		{"develop", "Set a ticket stage to develop"},
		{"done", "Set a ticket stage to done"},
		{"agent", "Manage autonomous agents and run agent workers"},
		{"get", "Show a ticket with history and comments"},
		{"help", "Show command help"},
		{"health", "Compute ticket health by project-specific heuristics"},
		{"idle", "Set a ticket state to idle"},
		{"list", "List tickets in the active project"},
		{"login", "Log into the server"},
		{"logout", "Clear the local session"},
		{"onboard", "Print the embedded AGENTS.md template to stdout"},
		{"open", "Reopen a closed ticket"},
		{"orphans", "List tickets with no parent"},
		{"project", "Manage projects and active project context"},
		{"team", "Manage teams, hierarchy, and team membership"},
		{"register", "Create a user account on the server"},
		{"ticket", "Generate requirements via an external agent"},
		{"request", "Request work for the current user"},
		{"request-dryrun", "Simulate a request assignment without mutation"},
		{"search", "Search tickets in the active project or across all projects"},
		{"set-parent", "Set the parent of a ticket"},
		{"attach", "Alias for set-parent"},
		{"status", "Show server and authentication status"},
		{"stage", "Set a ticket stage directly [design, develop, test, done]"},
		{"state", "Set a ticket state directly [idle, active, success, fail]"},
		{"test", "Set a ticket stage to test"},
		{"unset-parent", "Clear the parent of a ticket"},
		{"detach", "Alias for unset-parent"},
		{"unclaim", "Remove yourself from a ticket"},
		{"unarchive", "Unarchive a ticket"},
		{"upgrade", "Check whether a newer version is available"},
		{"update", "Update a ticket"},
		{"version", "Print the current version from VERSION"},
	}
	adminRows := [][2]string{
		{"assign", "Admin-only ticket assignment"},
		{"init", "Initialize the database, bootstrap admin, and create the default project"},
		{"server", "Start the API server and embedded web UI"},
		{"unassign", "Admin-only ticket unassignment"},
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

func printTicketTable(tickets []store.Ticket, dependencies map[int64]string, statusUnicode bool, includeArchived bool) {
	if len(tickets) == 0 {
		fmt.Println("no tickets")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if includeArchived {
		fmt.Fprintln(w, "MOON\tKEY\tTYPE\tSTATUS\tOPEN\tARCHIVED\tPARENT_ID\tASSIGNEE\tPRIORITY\tDEPENDSON\tHEALTH\tTITLE")
	} else {
		fmt.Fprintln(w, "MOON\tKEY\tTYPE\tSTATUS\tOPEN\tPARENT_ID\tASSIGNEE\tPRIORITY\tDEPENDSON\tHEALTH\tTITLE")
	}
	for _, ticket := range tickets {
		symbol := formatTicketStatusSymbol(ticket.Status, statusUnicode)
		assignee := ticket.Assignee
		if strings.TrimSpace(assignee) == "" {
			assignee = "-"
		}
		dependsOn := dependencies[ticket.ID]
		if dependsOn == "[]" {
			dependsOn = ""
		}
		parentID := ""
		if ticket.ParentID != nil {
			parentID = strconv.FormatInt(*ticket.ParentID, 10)
		}
		key := ticket.Key
		if strings.TrimSpace(key) == "" {
			key = strconv.FormatInt(ticket.ID, 10)
		}
		if includeArchived {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%t\t%s\t%s\t%d\t%s\t%.2f\t%s\n", symbol, key, ticket.Type, ticket.Status, ticketOpenLabel(ticket), ticket.Archived, parentID, assignee, ticket.Priority, dependsOn, float64(ticket.HealthScore)/4.0, ticket.Title)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%.2f\t%s\n", symbol, key, ticket.Type, ticket.Status, ticketOpenLabel(ticket), parentID, assignee, ticket.Priority, dependsOn, float64(ticket.HealthScore)/4.0, ticket.Title)
		}
	}
	_ = w.Flush()
}

func ticketOpenLabel(ticket store.Ticket) string {
	if !ticket.Open {
		return "closed"
	}
	return "open"
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
	case state == store.StateSuccess:
		return "◉"
	default:
		return ""
	}
}

func formatStatusCounts(statuses map[string]int) string {
	order := []string{
		"design/idle", "design/active", "design/success", "design/fail",
		"develop/idle", "develop/active", "develop/success", "develop/fail",
		"test/idle", "test/active", "test/success", "test/fail",
		"done/success", "done/fail",
	}
	labels := map[string]string{
		"design/idle":     "design/idle",
		"design/active":   "design/active",
		"design/success":  "design/success",
		"design/fail":     "design/fail",
		"develop/idle":    "develop/idle",
		"develop/active":  "develop/active",
		"develop/success": "develop/success",
		"develop/fail":    "develop/fail",
		"test/idle":       "test/idle",
		"test/active":     "test/active",
		"test/success":    "test/success",
		"test/fail":       "test/fail",
		"done/success":    "done/success",
		"done/fail":       "done/fail",
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

func printTicketEnvironment() {
	variableNames := []string{
		"TICKET_MODE",
		"TICKET_HOME",
		"TICKET_CONFIG_DIR",
		"TICKET_DB_OVERRIDE",
		"TICKET_SERVER",
		"TICKET_URL",
		"TICKET_USERNAME",
		"TICKET_PASSWORD",
	}

	fmt.Println()
	fmt.Println("ENVIRONMENT")
	for _, name := range variableNames {
		value := envValue(name)
		if value == "" {
			value = "<unset>"
		}
		fmt.Printf("  %s: %s\n", name, value)
	}
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
