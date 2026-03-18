package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

type commandHelp struct {
	usage   string
	details []string
	example string
}

var helpIndex = map[string]commandHelp{
	"onboard": {
		usage:   "ticket onboard",
		details: []string{"Prints ticket CLI instructions to stdout for use by agents.", "Usage: ticket onboard > TICKET.md"},
		example: "ticket onboard > TICKET.md",
	},
	"init": {
		usage:   "ticket init [-f <db-path>] [--force] [-password <password>] [--populate]",
		details: []string{"Creates a new SQLite database, bootstraps the fixed `admin` account, and creates the default project.", "If `-f` is omitted, the database path is derived from TICKET_URL (default: ~/.config/ticket/ticket.db).", "If `-password` is omitted, a random admin password is generated and printed to stdout.", "If `--force` is supplied, any existing database file is overwritten.", "If `--populate` is supplied, example projects/stories/tickets/users/teams are also seeded."},
		example: "ticket init -f /path/to/ticket.db --force -password secret --populate",
	},
	"export": {
		usage:   "ticket export [-o <snapshot-file>]",
		details: []string{"Local mode only (TICKET_URL=file://...). Exports all persisted entities to a JSON snapshot file.", "Snapshot includes `schema_version`, export timestamp, table columns, and row values with ids preserved."},
		example: "ticket export -o ./ticket-snapshot.json",
	},
	"import": {
		usage:   "ticket import -i <snapshot-file>",
		details: []string{"Local mode only (TICKET_URL=file://...). Replaces current database contents from a JSON snapshot file.", "Import preserves ids for all entities and validates foreign-key integrity after load."},
		example: "ticket import -i ./ticket-snapshot.json",
	},
	"server": {
		usage:   "ticket server [-f <db-path>] [-p <port>] [-addr <host:port>] [-v]",
		details: []string{"Starts the HTTP API server and the embedded web UI.", "If `-f` is omitted, the server uses the database path from TICKET_URL (default: ~/.config/ticket/ticket.db).", "Use `-p` as a shorthand port flag (for example `-p 9999`); `-addr` is still supported for explicit host/port binding.", "If `-v` is supplied, requests and responses are printed verbosely to stdout."},
		example: "ticket server -f /path/to/ticket.db -p 9999 -v",
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
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Logs into the server and stores the session token in ~/.config/ticket/credentials.json.", "Login resolution order: valid credentials.json, then username in config.json, then `-username` / `-password`, then `TICKET_USERNAME` / `TICKET_PASSWORD`, then prompts.", "If prompting is needed, discovered values are used as editable defaults.", "Server resolution: `-url`, then `TICKET_URL`, then configured URL, then `http://localhost:8080`."},
		example: "ticket login -username simon -password secret -url http://localhost:8080",
	},
	"register": {
		usage:   "ticket register [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Creates a user account on the configured server but does not log the user in.", "Credential resolution: `-username`, then `TICKET_USERNAME`, then OS `whoami`; `-password`, then `TICKET_PASSWORD`, then `password`."},
		example: "ticket register -username simon -password secret",
	},
	"logout": {
		usage:   "ticket logout [-url <server-url>]",
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Logs out from the configured server and removes ~/.config/ticket/credentials.json."},
		example: "ticket logout",
	},
	"status": {
		usage:   "ticket status [-url <server-url>] [-f <db-path>] [-nocolor]",
		details: []string{"Prints the current effective configuration, then performs a connectivity check.", "Remote mode prints `mode`, `server`, `username`, `authenticated`, then calls the remote status endpoint.", "Local mode prints `mode`, `db_path`, `db_exists`, then opens the database and verifies the schema is usable."},
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
		usage: "ticket update -id <id>\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-ac <acceptance-criteria>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]",
		details: []string{
			"-id <id>: required; ticket id or key",
			"-title <title>: set title",
			"-desc <description>: set description (alias: -description)",
			"-description <description>: set description (alias: -desc)",
			"-ac <acceptance-criteria>: set acceptance criteria",
			"-priority <n>: set numeric priority",
			"-order <n>: set numeric sort order",
			"-state <state>: valid values [idle, active, success, fail]; setting success auto-advances to next workflow stage",
			"-status <stage/state>: set state from rendered status format",
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

func renderRootUsage() string {
	var b strings.Builder
	b.WriteString(renderBanner())
	h := "\x1b[38;5;117m" // pastel blue
	r := "\x1b[0m"
	b.WriteString("\n" + h + "USAGE" + r + "\n")
	b.WriteString("  ticket <command> [options]\n\n")
	b.WriteString(h + "CLIENT COMMANDS" + r + "\n")
	clientRows := [][2]string{
		{"login", "Log into the server"},
		{"register", "Create a user account on the server"},
		{"logout", "Clear the local session"},
		{"status", "Show server and authentication status"},
		{"config", "Manage local config keys and registration controls"},
		{"project", "Manage projects and active project context"},
		{"team", "Manage teams, hierarchy, and team membership"},
		{"agent", "Manage autonomous agents and run agent workers"},
		{"role", "Manage roles (title, motivation, goals)"},
		{"workflow", "Manage workflow definitions and stages"},
		{"label", "Manage project labels and ticket tagging"},
		{"time", "Log and view time entries on tickets"},
		{"add", "Create a ticket in the active project"},
		{"get", "Show a ticket with history and comments"},
		{"board", "Kanban-style board grouped by workflow stage"},
		{"list", "List tickets in the active project"},
		{"search", "Search tickets in the active project or across all projects"},
		{"update", "Update a ticket"},
		{"delete", "Delete a ticket permanently"},
		{"clone", "Clone a ticket or epic"},
		{"claim", "Assign yourself to a ticket"},
		{"unclaim", "Remove yourself from a ticket"},
		{"request", "Request work for the current user"},
		{"request-dryrun", "Simulate a request assignment without mutation"},
		{"set-parent", "Set the parent of a ticket"},
		{"attach", "Alias for set-parent"},
		{"unset-parent", "Clear the parent of a ticket"},
		{"detach", "Alias for unset-parent"},
		{"comment", "Add comments to a ticket"},
		{"dependency", "Manage dependency links between tickets"},
		{"health", "Compute ticket health by project-specific heuristics"},
		{"count", "Count users, projects, and work by type"},
		{"orphans", "List tickets with no parent"},
		{"ticket", "Generate requirements via an external agent"},
		{"onboard", "Print ticket CLI instructions for agents to stdout"},
		{"help", "Show command help"},
		{"upgrade", "Check whether a newer version is available"},
		{"version", "Print the current version from VERSION"},
	}
	lifecycleRows := [][2]string{
		{"archive", "Archive a ticket"},
		{"unarchive", "Unarchive a ticket"},
		{"close", "Close a ticket and freeze modifications"},
		{"open", "Reopen a closed ticket"},
	}
	stageRows := [][2]string{
		{"design", "Set a ticket stage to design"},
		{"develop", "Set a ticket stage to develop"},
		{"test", "Set a ticket stage to test"},
		{"done", "Set a ticket stage to done"},
		{"stage", "Set a ticket stage directly [design, develop, test, done]"},
	}
	stateRows := [][2]string{
		{"idle", "Set a ticket state to idle"},
		{"active", "Set a ticket state to active"},
		{"complete", "Set a ticket state to success"},
		{"state", "Set a ticket state directly [idle, active, success, fail]"},
	}
	adminRows := [][2]string{
		{"assign", "Admin-only ticket assignment"},
		{"export", "Export all entities to a schema-versioned JSON snapshot"},
		{"import", "Import entities from a schema-versioned JSON snapshot"},
		{"init", "Initialize the database, bootstrap admin, and create the default project"},
		{"server", "Start the API server and embedded web UI"},
		{"unassign", "Admin-only ticket unassignment"},
		{"user", "Admin-only user management"},
	}
	commandWidth := commandUsageWidth(clientRows, lifecycleRows, stageRows, stateRows, adminRows)
	printCommandUsageRows(&b, clientRows, commandWidth)
	b.WriteString("\n" + h + "LIFECYCLE COMMANDS" + r + "\n")
	printCommandUsageRows(&b, lifecycleRows, commandWidth)
	b.WriteString("\n" + h + "STAGE COMMANDS" + r + "\n")
	printCommandUsageRows(&b, stageRows, commandWidth)
	b.WriteString("\n" + h + "STATE COMMANDS" + r + "\n")
	printCommandUsageRows(&b, stateRows, commandWidth)
	b.WriteString("\n" + h + "ADMIN COMMANDS" + r + "\n")
	printCommandUsageRows(&b, adminRows, commandWidth)
	b.WriteString("\n" + h + "HELP" + r + "\n")
	b.WriteString("  ticket help <command>\n")
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
		"TICKET_URL",
		"TICKET_CONFIG_DIR",
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
