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
	"initdb": {
		usage:   "ticket initdb [-f <db-path>] [--force] [-password <password>] [--populate]",
		details: []string{"Creates a new SQLite database, bootstraps the fixed `admin` account, and creates the default project.", "If `-f` is omitted, the database path is derived from TICKET_HOME (default: .ticket/ticket.db in the current directory).", "If `-password` is omitted, a random admin password is generated and printed to stdout.", "If `--force` is supplied, any existing database file is overwritten.", "If `--populate` is supplied, example projects/stories/tickets/users/teams are also seeded.", "Alias: `ticket init`."},
		example: "ticket initdb -f /path/to/ticket.db --force -password secret --populate",
	},
	"export": {
		usage:   "ticket export [-o <snapshot-file>]",
		details: []string{"Local mode only (no TICKET_URL set). Exports all persisted entities to a JSON snapshot file.", "Snapshot includes `schema_version`, export timestamp, table columns, and row values with ids preserved."},
		example: "ticket export -o ./ticket-snapshot.json",
	},
	"import": {
		usage:   "ticket import -i <snapshot-file>",
		details: []string{"Local mode only (no TICKET_URL set). Replaces current database contents from a JSON snapshot file.", "Import preserves ids for all entities and validates foreign-key integrity after load."},
		example: "ticket import -i ./ticket-snapshot.json",
	},
	"server": {
		usage:   "ticket server [-f <db-path>] [-p <port>] [-addr <host:port>] [-v]",
		details: []string{"Starts the HTTP API server and the embedded web UI.", "If `-f` is omitted, the server uses the database path from TICKET_HOME (default: .ticket/ticket.db in the current directory).", "Use `-p` as a shorthand port flag (for example `-p 9999`); `-addr` is still supported for explicit host/port binding.", "If `-v` is supplied, requests and responses are printed verbosely to stdout."},
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
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Logs into the server and stores the session token in $TICKET_HOME/credentials.json.", "Login resolution order: valid credentials.json, then username in config.json, then `-username` / `-password`, then `TICKET_USERNAME` / `TICKET_PASSWORD`, then prompts.", "If prompting is needed, discovered values are used as editable defaults.", "Server resolution: `-url`, then `TICKET_URL`, then configured URL, then `http://localhost:8080`."},
		example: "ticket login -username simon -password secret -url http://localhost:8080",
	},
	"register": {
		usage:   "ticket register [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Creates a user account on the configured server but does not log the user in.", "Credential resolution: `-username`, then `TICKET_USERNAME`, then OS `whoami`; `-password`, then `TICKET_PASSWORD`, then `password`."},
		example: "ticket register -username simon -password secret",
	},
	"logout": {
		usage:   "ticket logout [-url <server-url>]",
		details: []string{"Remote mode only (TICKET_URL=http(s)://...). Logs out from the configured server and removes $TICKET_HOME/credentials.json."},
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
		usage:   "ticket ticket <verb> [flags]",
		details: []string{"Namespace for all ticket operations: list, search, board, add, get, update, state changes, ownership, hierarchy, comments, lifecycle.", "Run `ticket ticket help` for the full verb list."},
		example: "ticket ticket list --type bug",
	},
	"req": {
		usage:   "ticket req <verb> [flags]",
		details: []string{"Namespace for requirements: capture ideas, shape them, accept/reject/revise.", "Shortcuts: `ticket idea \"title\"` = `ticket req add \"title\"`, `ticket ideas` = `ticket req list`.", "Run `ticket req help` for the full verb list."},
		example: "ticket req add \"offline mode\" -d \"the app should work without network\"",
	},
	"dep": {
		usage:   "ticket dep <add|remove> -id <id> <dependency-id>",
		details: []string{"Manages `depends_on` links for a ticket.", "`add` creates dependency links; `remove` deletes them.", "Alias for `ticket dependency`."},
		example: "ticket dep add -id TK-4 TK-1",
	},
	"idea": {
		usage:   "ticket idea \"title\" [-d description] [-ac criteria]",
		details: []string{"Shortcut for `ticket req add`. Captures a new requirement."},
		example: "ticket idea \"dark mode support\"",
	},
	"ideas": {
		usage:   "ticket ideas [-status raw|shaping|accepted|rejected]",
		details: []string{"Shortcut for `ticket req list`. Lists all requirements."},
		example: "ticket ideas -status proposed",
	},
	"health": {
		usage:   "ticket health [-id] <id>|execute",
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
		usage:   "ticket list|ls [--type <type>] [--stage <stage>] [--state <state>] [--status <stage/state>] [-u <user>] [-n <limit>] [-a] [-d] [--unicode] [--plain]",
		details: []string{"Lists tickets in the active project with optional type, lifecycle, assignee, and limit filters.", "`status` is a rendered composite such as `develop/active`. `-n` is applied server-side. `0` means no limit.", "By default closed and archived tickets are hidden; use `-a` to include closed tickets, `-d` to also include archived. Combined flags like `-ad` are supported."},
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
	"edit": {
		usage:   "ticket edit [-id] <id>",
		details: []string{"Opens the TUI editor for the specified ticket.", "If no ID is given, opens the most recently modified ticket in the current project."},
		example: "ticket edit TK-42",
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
		usage:   "ticket set-parent [-id] <id> <parent-id>",
		details: []string{"Sets the parent of a ticket or epic.", "Both ids must be numeric ticket ids in the active project.", "If the child is an epic, the parent must also be an epic."},
		example: "ticket set-parent TK-1 TK-2",
	},
	"attach": {
		usage:   "ticket attach [-id] <id> <parent-id>",
		details: []string{"Alias for `ticket set-parent`."},
		example: "ticket attach CUS-T-12 CUS-E-3",
	},
	"unset-parent": {
		usage:   "ticket unset-parent [-id] <id>",
		details: []string{"Clears the parent of a ticket or story.", "After this, the ticket becomes an orphan."},
		example: "ticket unset-parent TK-1",
	},
	"detach": {
		usage:   "ticket detach [-id] <id>",
		details: []string{"Alias for `ticket unset-parent`."},
		example: "ticket detach CUS-T-12",
	},
	"idle": {
		usage:   "ticket idle [-id] <id>",
		details: []string{"Sets the ticket state to `idle` without changing the stage."},
		example: "ticket idle TK-42",
	},
	"state": {
		usage:   "ticket state -id <id> <idle|active|success|fail>",
		details: []string{"Sets a ticket state directly while preserving the current stage."},
		example: "ticket state -id 42 active",
	},
	"active": {
		usage:   "ticket active [-id] <id>",
		details: []string{"Sets the ticket state to `active` without changing the stage.", "`active` requires an assignee; if the ticket is unassigned the CLI claims it for the current user first."},
		example: "ticket active TK-42",
	},
	"complete": {
		usage:   "ticket complete [-id] <id>",
		details: []string{"Sets the ticket state to `success` without changing the stage."},
		example: "ticket complete TK-42",
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
		usage:   "ticket clone|cp [-id] <id>",
		details: []string{"Clones a ticket or epic.", "Cloned items are unassigned, reset to `design/idle`, and keep a `clone_of` reference to the source item. Cloning an epic also clones its child tickets."},
		example: "ticket clone TK-42",
	},
	"close": {
		usage:   "ticket close [-id] <id>",
		details: []string{"Closes a ticket so it remains visible but frozen.", "Closed tickets cannot be modified until reopened."},
		example: "ticket close TK-1",
	},
	"open": {
		usage:   "ticket open [-id] <id>",
		details: []string{"Reopens a closed ticket so it can be updated again.", "Open and close actions are recorded in ticket history."},
		example: "ticket open TK-1",
	},
	"archive": {
		usage:   "ticket archive [-id] <id>",
		details: []string{"Archives a ticket.", "Archived tickets are hidden from default `ticket ls` output."},
		example: "ticket archive TK-1",
	},
	"unarchive": {
		usage:   "ticket unarchive [-id] <id>",
		details: []string{"Unarchives a ticket so it appears in default `ticket ls` output."},
		example: "ticket unarchive TK-1",
	},
	"ready": {
		usage:   "ticket ready [-id] <id>",
		details: []string{"Marks a ticket as ready to be picked up for work.", "Only ready tickets are eligible for automatic assignment via `claim` or `request`."},
		example: "ticket ready TK-42",
	},
	"notready": {
		usage:   "ticket notready [-id] <id>",
		details: []string{"Marks a ticket as not ready.", "Not-ready tickets are excluded from automatic assignment."},
		example: "ticket notready TK-42",
	},
	"delete": {
		usage:   "ticket rm|delete [-id] <id>",
		details: []string{"Deletes a ticket permanently.", "Fails if the ticket still has child tickets."},
		example: "ticket delete TK-42",
	},
	"assign": {
		usage:   "ticket assign [-id] <id> <name>",
		details: []string{"Admin-only command that assigns a ticket to a user.", "The target user must exist and be enabled."},
		example: "ticket assign TK-42 alice",
	},
	"unassign": {
		usage:   "ticket unassign [-id] <id> <name>",
		details: []string{"Admin-only command that clears a ticket assignment from the named user.", "The named user must exist and be enabled."},
		example: "ticket unassign TK-42 alice",
	},
	"claim": {
		usage:   "ticket claim [-id] <id>",
		details: []string{"Assigns the caller to the ticket.", "Fails if the ticket is already assigned to another user."},
		example: "ticket claim TK-42",
	},
	"unclaim": {
		usage:   "ticket unclaim [-id] <id>",
		details: []string{"Clears the caller's assignment from the ticket.", "Fails unless the caller is the current assignee."},
		example: "ticket unclaim TK-42",
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
	"story": {
		usage:   "ticket story <create|list|get|update|delete>",
		details: []string{"Manages stories within the active project.", "Stories provide a lightweight grouping layer within a project."},
		example: "ticket story create -title \"User onboarding flow\"",
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
	b.WriteString("  ticket <noun> <verb> [flags]\n\n")
	commandRows := [][2]string{
		{"ticket", "Manage tickets — create, update, state, assign, comment, close"},
		{"req", "Capture and refine requirements — add, shape, accept, reject"},
		{"project", "Manage projects and active project context"},
		{"dep", "Manage dependency links between tickets"},
		{"label", "Manage project labels and ticket tagging"},
		{"time", "Log and view time entries on tickets"},
		{"story", "Manage stories within a project"},
		{"decision", "Record and list architectural decisions"},
	}
	b.WriteString(h + "COMMANDS" + r + "\n")
	printCommandUsageRows(&b, commandRows, 10)
	adminRows := [][2]string{
		{"role", "Manage roles (title, motivation, goals)"},
		{"workflow", "Manage workflow definitions and stages"},
		{"team", "Manage teams, hierarchy, and team membership"},
		{"agent", "Manage autonomous agents and run agent workers"},
		{"user", "Admin-only user management"},
	}
	b.WriteString("\n" + h + "ADMIN" + r + "\n")
	printCommandUsageRows(&b, adminRows, 10)
	shortcutRows := [][2]string{
		{"tk", "List tickets in the active project (alias: tk ticket list)"},
		{"tk add", "Create a ticket (alias: tk ticket add)"},
		{"tk bug", "Create a bug (alias: tk ticket add -type bug)"},
		{"tk epic", "Create an epic (alias: tk ticket add -type epic)"},
		{"tk idea", "Capture a requirement (alias: tk req add)"},
		{"tk ideas", "List requirements (alias: tk req list)"},
	}
	b.WriteString("\n" + h + "SHORTCUTS" + r + "\n")
	printCommandUsageRows(&b, shortcutRows, 10)
	systemRows := [][2]string{
		{"status", "Show connection and authentication status"},
		{"server", "Start the API server and web UI"},
		{"login", "Log into the server"},
		{"logout", "Clear the local session"},
		{"register", "Create a user account on the server"},
		{"config", "Manage local config keys"},
		{"init", "Interactive project setup (alias: setup)"},
		{"initdb", "Initialize the database"},
		{"export", "Export entities to a JSON snapshot"},
		{"import", "Import entities from a JSON snapshot"},
		{"version", "Print the current version"},
		{"upgrade", "Check for a newer version"},
		{"help", "Show command help"},
	}
	b.WriteString("\n" + h + "SYSTEM" + r + "\n")
	printCommandUsageRows(&b, systemRows, 10)
	b.WriteString("\n" + h + "EXAMPLES" + r + "\n")
	b.WriteString("  tk                                          List open tickets\n")
	b.WriteString("  tk add -title \"Fix login bug\" -type bug     Create a bug ticket\n")
	b.WriteString("  tk idea -title \"Dark mode support\"          Capture a requirement\n")
	b.WriteString("  tk ticket get -id 42                        Show ticket details\n")
	b.WriteString("  tk ls -json | jq '.[].key' | xargs -I {} ticket close -id {}   Close all tickets\n")
	b.WriteString("  tk summary                                  Your daily starting point\n")

	b.WriteString("\n" + h + "HELP" + r + "\n")
	b.WriteString("  ticket <noun> help        Show verbs for a namespace\n")
	b.WriteString("  ticket help <command>     Show detailed command help\n")
	return strings.TrimSpace(b.String()) + "\n"
}

func printCommandUsageRows(b *strings.Builder, rows [][2]string, commandWidth int) {
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintf(w, "  %-*s\t%s\n", commandWidth, row[0], row[1])
	}
	_ = w.Flush()
}

// ---------------------------------------------------------------------------
// Per-namespace help text — consistent format across all nouns
// ---------------------------------------------------------------------------

const depUsage = `Usage: ticket dep <command> [flags]

Commands:
  add      -id <id> <depends-on-id>   Add a dependency
  remove   -id <id> <depends-on-id>   Remove a dependency`

const labelUsage = `Usage: ticket label <command> [flags]

Commands:
  list                                List all project labels
  create   -name <name> [-color hex]  Create a label
  delete   -id <label-id>             Delete a label
  add      -id <ticket-id> <label-id> Tag a ticket with a label
  remove   -id <ticket-id> <label-id> Remove a label from a ticket
  show     -id <ticket-id>            Show labels on a ticket`

const timeUsage = `Usage: ticket time <command> [flags]

Commands:
  log      -id <ticket-id> -m <minutes> [-note text]   Log time
  list     -id <ticket-id>                              List entries
  total    -id <ticket-id>                              Sum total time
  delete   -id <entry-id>                               Delete an entry`

const projectUsage = `Usage: ticket project <command> [flags]

Commands:
  list, ls                            List all projects
  create   -title <name>              Create a project
  get      <id>                       Show project details
  use, default  [<id>]                Switch active project (or show current)
  rm       [-id] <id> [--confirm tok]  Delete a project (two-step)
  init                                Init project in current directory
  add-user                            Add a user to a project
  remove-user                         Remove a user from a project
  add-team                            Add a team to a project
  remove-team                         Remove a team from a project`

const roleUsage = `Usage: ticket role <command> [flags]

Commands:
  list                                List all roles
  create   -title <t> [-motivation m] [-goals g]   Create a role
  update   -id <id> [-title t] [-motivation m] [-goals g]   Update a role
  delete   -id <id>                   Delete a role`

const workflowUsage = `Usage: ticket workflow <command> [flags]

Commands:
  list                                List all workflows
  create   -name <n> [-d desc]        Create a workflow
  get      -id <id>                   Show workflow details
  delete   -id <id>                   Delete a workflow
  add-stage    -id <wf-id> -name <n>  Add a stage
  remove-stage -stage-id <id>         Remove a stage
  reorder-stages -id <wf-id> <ids>    Reorder stages
  export   -id <id> [-o file]         Export a workflow`

const decisionUsage = `Usage: ticket decision <command> [flags]

Commands:
  add      "text"                     Record a decision
  list                                List all decisions`

const teamUsage = `Usage: ticket team <command> [flags]

Commands:
  list                                List all teams
  create   -name <name>              Create a team
  update   -id <id> -name <name>     Update a team
  delete   -id <id>                   Delete a team
  add-user     -team_id <id> -user_id <id>    Add a user
  remove-user  -team_id <id> -user_id <id>    Remove a user
  users        -id <id>                       List team users
  add-agent    -team_id <id> -agent_id <id>   Add an agent
  remove-agent -team_id <id> -agent_id <id>   Remove an agent
  agents       -id <id>                       List team agents`

const configUsage = `Usage: ticket config <command> [flags]

Commands:
  ls, list                              List all config values
  get      <key>                        Get a config value
  set      <key> <value>                Set a config value
  rm       <key>                        Remove a config value
  registration-enable                   Enable user registration (server)
  registration-disable                  Disable user registration (server)

Keys: server, username, current_project, current_epic_id, registration_enabled`

const agentUsage = `Usage: ticket agent <command> [flags]

Commands:
  list                                List all agents
  create   -name <n> [-description d] Create an agent
  update   -id <id> [-name n]         Update an agent
  delete   -id <id>                   Delete an agent
  enable   -id <id>                   Enable an agent
  disable  -id <id>                   Disable an agent
  request  -id <id>                   Request work for an agent
  run      [flags]                    Run an agent worker loop
  reset-password -id <id> [-password] Reset an agent's password
  config-set -id <id> <key> <value>  Set a config value on an agent
  config-ls  -id <id>                List agent config values
  config-rm  -id <id> <key>          Remove a config value from an agent

Run flags:
  -name <name>             Agent name (or AGENT_NAME env)
  -password <password>     Agent password (or AGENT_PASSWORD env)
  -url <url>               Server URL (or TICKET_URL env)
  -llm <command>           LLM to use (default: claude)
                           Values: claude (Sonnet 4.5), codex, or path to binary
  -project-id <id>         Project ID override (default: first open project)
  -poll-seconds <n>        Idle poll interval in seconds (default: 2)
  -v                       Verbose: stream LLM I/O to terminal`

const userUsage = `Usage: ticket user <command> [flags]

Commands:
  list                                List all users
  create   --username <u> --password <p>   Create a user
  delete   -id <id>                   Delete a user
  enable   -id <id>                   Enable a user
  disable  -id <id>                   Disable a user
  reset-password -username <u> [-password]  Reset password and invalidate sessions`

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
		"TICKET_HOME",
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
