package main

import (
	"fmt"
	"os"
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
		usage:   "tk onboard",
		details: []string{"Prints ticket CLI instructions to stdout for use by agents.", "Usage: tk onboard > TICKET.md"},
		example: "tk onboard > TICKET.md",
	},
	"skill": {
		usage:   "tk skill",
		details: []string{"Prints the embedded tk SKILL.md template to stdout.", "Usage: tk skill > SKILL.md"},
		example: "tk skill > SKILL.md",
	},
	"initdb": {
		usage:   "tk initdb [-f <db-path>] [-force] [-password <password>] [-populate]",
		details: []string{"Creates a new SQLite database, bootstraps the fixed `admin` account, and creates the default project.", "If `-f` is omitted, the database path defaults to `.ticket/ticket.db` at the nearest Git root, or `.ticket/ticket.db` in the current directory when no Git root exists. `TICKET_URL` can override the effective location.", "If `-password` is omitted, a random admin password is generated and printed to stdout.", "If `-force` is supplied, any existing database file is overwritten.", "If `-populate` is supplied, example projects/stories/tickets/users/teams are also seeded.", "`tk init` is the interactive setup command for local/remote configuration."},
		example: "tk initdb -f /path/to/ticket.db -force -password secret -populate",
	},
	"export": {
		usage:   "tk export [-o <snapshot-file>]",
		details: []string{"Local mode only. Exports all persisted entities to a JSON snapshot file.", "Snapshot includes `schema_version`, export timestamp, table columns, and row values with ids preserved."},
		example: "tk export -o ./ticket-snapshot.json",
	},
	"import": {
		usage:   "tk import -i <snapshot-file>",
		details: []string{"Local mode only. Replaces current database contents from a JSON snapshot file.", "Import preserves ids for all entities and validates foreign-key integrity after load."},
		example: "tk import -i ./ticket-snapshot.json",
	},
	"server": {
		usage:   "tk server [-f <db-path>] [-p <port>] [-addr <host:port>] [-v]",
		details: []string{"Starts the HTTP API server and the embedded web UI.", "If `-f` is omitted, the server uses `.ticket/ticket.db` at the nearest Git root, or `.ticket/ticket.db` in the current directory when no Git root exists. `TICKET_URL` can override the effective location.", "If `-f` is provided, that exact database file is used directly (no env or workspace inference for database selection).", "Use `-p` as a shorthand port flag (for example `-p 9999`); `-addr` is still supported for explicit host/port binding.", "If `-v` is supplied, requests and responses are printed verbosely to stdout."},
		example: "tk server -f /path/to/ticket.db -p 9999 -v",
	},
	"version": {
		usage:   "tk version",
		details: []string{"Prints the semantic version embedded into the binary from the build-time `VERSION` file."},
		example: "tk version",
	},
	"upgrade": {
		usage:   "tk upgrade",
		details: []string{"Checks the repository VERSION file and compares it to the embedded local version.", "The network check fails fast after 3 seconds if the repository cannot be reached."},
		example: "tk upgrade",
	},
	"login": {
		usage:   "tk login [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"Remote mode only. Logs into the server and stores the session token in the active `.ticket/credentials.json` file.", "When `TICKET_URL` points at a remote server and `TICKET_USERNAME` / `TICKET_PASSWORD` are set, those values override local config and are used directly; `tk login` is optional for regular API calls in that mode.", "Login resolution order: valid credentials.json, then username in config.json, then `-username` / `-password`, then `TICKET_USERNAME` / `TICKET_PASSWORD`, then prompts.", "If prompting is needed, discovered values are used as editable defaults."},
		example: "tk login -username simon -password secret -url http://localhost:8080",
	},
	"register": {
		usage:   "tk register [-username <name>] [-password <password>] [-url <server-url>]",
		details: []string{"Remote mode only. Creates a user account on the configured server but does not log the user in.", "When `TICKET_URL`, `TICKET_USERNAME`, and `TICKET_PASSWORD` are all set, those values override local config and are used directly.", "Credential resolution: `-username`, then `TICKET_USERNAME`, then OS `whoami`; `-password`, then `TICKET_PASSWORD`, then `password`."},
		example: "tk register -username simon -password secret",
	},
	"logout": {
		usage:   "tk logout [-url <server-url>]",
		details: []string{"Remote mode only. Logs out from the configured server and removes the active `.ticket/credentials.json` session token."},
		example: "tk logout",
	},
	"status": {
		usage:   "tk status [-url <server-url>] [-f <db-path>] [-nocolor]",
		details: []string{"Prints the current effective configuration, then performs a connectivity check.", "Both local and remote output include the active project, its current SDLC name, and whether new tickets default to draft mode.", "Local mode also prints `db_path` / `db_exists`; remote mode also prints `username` / `authenticated`."},
		example: "tk status",
	},
	"help": {
		usage:   "tk help <command>",
		details: []string{"Shows command-specific help when available.", "Without a command, prints the root usage summary."},
		example: "tk help dependency",
	},
	"count": {
		usage:   "tk count [-project_id <id>] [-url <server-url>]",
		details: []string{"Counts users and work items by type.", "With `-project_id`, counts work items within that project and omits the global project total."},
		example: "tk count -project_id 1",
	},
	"ticket": {
		usage:   "tk ticket <verb> [flags]",
		details: []string{"Namespace for all ticket operations: list, search, board, add, get, update, state changes, ownership, hierarchy, comments, lifecycle.", "Run `tk ticket help` for the full verb list."},
		example: "tk ticket list -type bug",
	},
	"prompt": {
		usage:   "tk prompt <ticket-id>",
		details: []string{"Builds a plaintext agent prompt for the given ticket.", "Includes sections for project, epic, story, ticket, role, and stage acceptance details when available."},
		example: "tk prompt TK-42",
	},
	"req": {
		usage:   "tk req <verb> [flags]",
		details: []string{"Legacy alias for `tk idea`. Routes to the same handlers.", "Run `tk idea help` for the full verb list."},
		example: "tk idea new \"offline mode\" -d \"the app should work without network\"",
	},
	"dep": {
		usage:   "tk dep <add|remove> -id <id> <dependency-id>",
		details: []string{"Manages `depends_on` links for a ticket.", "`add` creates dependency links; `remove` deletes them.", "Alias for `tk dependency`."},
		example: "tk dep add -id TK-4 TK-1",
	},
	"idea": {
		usage:   "tk idea <verb> [flags]",
		details: []string{"Namespace for requirements/ideas. Verbs: new, ls, get, shape, accept, reject, revise.", "Run `tk idea help` for the full verb list."},
		example: "tk idea new \"dark mode support\"",
	},
	"health": {
		usage:   "tk health [-id] <id>|execute",
		details: []string{"Compute and persist ticket health scores using documented heuristics.", "`execute` scores all tickets in the active project."},
		example: "tk health TK-1",
	},
	"project": {
		usage:   "tk project <create|list|get|use|set-draft|sdlc|add-user|remove-user|add-team|remove-team>|<id> <update|enable|disable>",
		details: []string{"Manages projects and the active project context used by subsequent commands.", "Projects are addressed by prefix or numeric id.", "Project membership supports both users and teams.", "`set-draft` controls whether new tickets default to draft mode for the project."},
		example: "tk project CUS update -title \"Customer Portal\"",
	},
	"team": {
		usage:   "tk team <list|create|update|delete|add-user|remove-user|users|add-agent|remove-agent|agents>",
		details: []string{"Manages team hierarchy, team users (member/owner + job title), and team agent assignments.", "Teams can be assigned to projects with `tk project add-team`."},
		example: "tk team create -name \"Platform\"",
	},
	"list": {
		usage:   "tk list|ls [-type <type>] [-stage <stage>] [-state <state>] [-status <stage/state>] [-u <user>] [-n <limit>] [-a] [-d] [-unicode] [-plain]",
		details: []string{"Lists tickets in the active project with optional type, lifecycle, assignee, and limit filters.", "`status` is a rendered composite such as `develop/active`. `-n` is applied server-side. `0` means no limit.", "By default closed and archived tickets are hidden; use `-a` to include closed tickets, `-d` to also include archived. Combined flags like `-ad` are supported."},
		example: "tk list -type bug -status develop/idle -u alice -n 20",
	},
	"orphans": {
		usage:   "tk orphans [-url <server-url>]",
		details: []string{"Lists unparented non-epic tickets in the active project."},
		example: "tk orphans",
	},
	"get": {
		usage:   "tk get -id <id> [-url <server-url>]",
		details: []string{"Shows a single ticket with comments and history.", "Output uses subtle color unless `-nocolor` is supplied."},
		example: "tk get -id 42",
	},
	"show": {
		usage:   "tk show -id <id>",
		details: []string{"Alias for `tk get`."},
		example: "tk show -id 42",
	},
	"edit": {
		usage:   "tk edit [-id] <id>",
		details: []string{"Opens the TUI editor for the specified ticket.", "If no ID is given, opens the most recently modified ticket in the current project."},
		example: "tk edit TK-42",
	},
	"search": {
		usage:   "tk search <free form query> [-stage <stage>] [-state <state>] [-status <stage/state>] [-title <text>] [-description <text>] [-priority <n>] [-owner <user>] [-allprojects]",
		details: []string{"Searches tickets in the active project by default.", "Use `-allprojects` to search across every project. Optional filters narrow by lifecycle, title text, description text, priority, and owner."},
		example: "tk search password reset -status develop/active -owner alice -allprojects",
	},
	"update": {
		usage: "tk update -id <id>\n  [-title <title>]\n  [-desc <description> | -description <description>]\n  [-ac <acceptance-criteria>]\n  [-git-repository <repo>]\n  [-git-branch <branch>]\n  [-priority <n>]\n  [-order <n>]\n  [-stage <stage>]\n  [-state <state>]\n  [-status <stage/state>]\n  [-parent_id <id>]\n  [-estimate_effort <n>]\n  [-estimate_complete <rfc3339>]\n  [-t <type> | -type <type>]",
		details: []string{
			"-id <id>: required; ticket id or key",
			"-title <title>: set title",
			"-desc <description>: set description (alias: -description)",
			"-description <description>: set description (alias: -desc)",
			"-ac <acceptance-criteria>: set acceptance criteria",
			"-priority <n>: set numeric priority",
			"-order <n>: set numeric sort order",
			"-stage <stage>: set the stage; valid stages come from the ticket's current workflow",
			"-state <state>: valid values [idle, active, success, fail]; setting success auto-advances to next sdlc stage",
			"-status <stage/state>: set both stage and state from rendered status format",
			"-parent_id <id>: set parent ticket id",
			"-estimate_effort <n>: set numeric estimate effort",
			"-estimate_complete <rfc3339>: set completion timestamp (example 2026-03-31T17:00:00Z)",
			"-t <type> / -type <type>: change the ticket type (task, bug, epic, spike, chore, story, note, question, requirement, decision)",
		},
		example: "tk update -id 42 -stage develop -state active -priority 2 -estimate_effort 5",
	},
	"set-parent": {
		usage:   "tk set-parent [-id] <id> <parent-id>",
		details: []string{"Sets the parent of a ticket or epic.", "Both ids must be numeric ticket ids in the active project.", "If the child is an epic, the parent must also be an epic."},
		example: "tk set-parent TK-1 TK-2",
	},
	"attach": {
		usage:   "tk attach [-id] <id> <parent-id>",
		details: []string{"Alias for `tk set-parent`."},
		example: "tk attach CUS-T-12 CUS-E-3",
	},
	"unset-parent": {
		usage:   "tk unset-parent [-id] <id>",
		details: []string{"Clears the parent of a ticket or story.", "After this, the ticket becomes an orphan."},
		example: "tk unset-parent TK-1",
	},
	"detach": {
		usage:   "tk detach [-id] <id>",
		details: []string{"Alias for `tk unset-parent`."},
		example: "tk detach CUS-T-12",
	},
	"idle": {
		usage:   "tk idle [-id] <id>",
		details: []string{"Sets the ticket state to `idle` without changing the stage."},
		example: "tk idle TK-42",
	},
	"state": {
		usage:   "tk state -id <id> <idle|active|success|fail>",
		details: []string{"Sets a ticket state directly while preserving the current stage."},
		example: "tk state -id 42 active",
	},
	"active": {
		usage:   "tk active [-id] <id>",
		details: []string{"Sets the ticket state to `active` without changing the stage.", "`active` requires an assignee; if the ticket is unassigned the CLI claims it for the current user first."},
		example: "tk active TK-42",
	},
	"complete": {
		usage:   "tk complete [-id] <id>",
		details: []string{"Sets the ticket state to `success` without changing the stage."},
		example: "tk complete TK-42",
	},
	"add": {
		usage:   "tk add|create|new [-title <title>] [-t <type>] [-p <priority>] [-a <assignee>] [-d <description>] [-ac <criteria>] [-parent <id>] [-project <project>] [-estimate_effort <n>] [-estimate_complete <rfc3339>] [title words]",
		details: []string{"Creates a ticket-like entity in the active project.", "Positional title words and `-title` are equivalent ways to set the title.", "Defaults: `type=ticket`, `stage=design`, `state=idle`, `priority=1`, blank assignee, blank description, blank acceptance criteria, blank parent, current project, `estimate_effort=0`, blank `estimate_complete`."},
		example: "tk add \"Customers can reset their password.\"",
	},
	"comment": {
		usage:   "tk comment add -id <id> \"comment\"",
		details: []string{"Adds a comment to a ticket and records a corresponding history event."},
		example: "tk comment add -id 42 \"Need product sign-off.\"",
	},
	"clone": {
		usage:   "tk clone|cp [-id] <id>",
		details: []string{"Clones a ticket or epic.", "Cloned items are unassigned, reset to `design/idle`, and keep a `clone_of` reference to the source item. Cloning an epic also clones its child tickets."},
		example: "tk clone TK-42",
	},
	"close": {
		usage:   "tk close [-id] <id>",
		details: []string{"Closes a ticket so it remains visible but frozen.", "Closed tickets cannot be modified until reopened."},
		example: "tk close TK-1",
	},
	"open": {
		usage:   "tk open [-id] <id>",
		details: []string{"Reopens a closed ticket so it can be updated again.", "Open and close actions are recorded in ticket history."},
		example: "tk open TK-1",
	},
	"archive": {
		usage:   "tk archive [-id] <id>",
		details: []string{"Archives a ticket.", "Archived tickets are hidden from default `tk ls` output."},
		example: "tk archive TK-1",
	},
	"unarchive": {
		usage:   "tk unarchive [-id] <id>",
		details: []string{"Unarchives a ticket so it appears in default `tk ls` output."},
		example: "tk unarchive TK-1",
	},
	"ready": {
		usage:   "tk ready [-id] <id>",
		details: []string{"Marks a ticket as ready to be picked up for work.", "Only ready tickets are eligible for automatic assignment via `claim` or `request`."},
		example: "tk ready TK-42",
	},
	"notready": {
		usage:   "tk notready [-id] <id>",
		details: []string{"Marks a ticket as not ready.", "Not-ready tickets are excluded from automatic assignment."},
		example: "tk notready TK-42",
	},
	"delete": {
		usage:   "tk rm|delete [-id] <id>",
		details: []string{"Deletes a ticket permanently.", "Fails if the ticket still has child tickets."},
		example: "tk delete TK-42",
	},
	"assign": {
		usage:   "tk assign [-id] <id> <name>",
		details: []string{"Admin-only command that assigns a ticket to a user.", "The target user must exist and be enabled."},
		example: "tk assign TK-42 alice",
	},
	"unassign": {
		usage:   "tk unassign [-id] <id> <name>",
		details: []string{"Admin-only command that clears a ticket assignment from the named user.", "The named user must exist and be enabled."},
		example: "tk unassign TK-42 alice",
	},
	"claim": {
		usage:   "tk claim [-id] <id>",
		details: []string{"Assigns the caller to the ticket.", "Fails if the ticket is already assigned to another user."},
		example: "tk claim TK-42",
	},
	"unclaim": {
		usage:   "tk unclaim [-id] <id>",
		details: []string{"Clears the caller's assignment from the ticket.", "Fails unless the caller is the current assignee."},
		example: "tk unclaim TK-42",
	},
	"add-dependency": {
		usage:   "tk add-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Adds one or more `depends_on` links from the ticket to the listed ticket IDs.", "Comma-separated dependency IDs are supported."},
		example: "tk add-dependency 4 1,2,3",
	},
	"remove-dependency": {
		usage:   "tk remove-dependency <id> <dependency-id[,dependency-id...]>",
		details: []string{"Removes one or more `depends_on` links from the ticket to the listed ticket IDs.", "Comma-separated dependency IDs are supported."},
		example: "tk remove-dependency 4 2",
	},
	"dependency": {
		usage:   "tk dependency <add|remove> -id <id> <dependency-id[,dependency-id...]>",
		details: []string{"Manages `depends_on` links for a ticket.", "`add` creates dependency links; `remove` deletes them."},
		example: "tk dependency add -id 4 1,2,3",
	},
	"request": {
		usage:   "tk request [-dryrun] [<id>]",
		details: []string{"Requests work for the current user.", "With an id, the server attempts to assign that specific ticket. Without an id, it resumes the user's oldest assigned `develop/active` ticket, then assigned `develop/idle` work, then assigns the oldest unassigned `develop/idle` ticket in the active project."},
		example: "tk request 42",
	},
	"request-dryrun": {
		usage:   "tk request-dryrun [<id>]",
		details: []string{"Simulates a request assignment without mutating state and shows what ticket would be assigned."},
		example: "tk request-dryrun 42",
	},
	"user": {
		usage:   "tk user <create|new|ls|list|rm|delete|enable|disable>",
		details: []string{"Admin-only user management commands.", "If a non-admin user calls these commands, the server returns 403 with `user is not an admin`."},
		example: "tk user create -username alice -password secret",
	},
	"agent": {
		usage:   "tk agent <request|run> (agent) | <create|ls|list|update|rm|delete|enable|disable|reset-password|config-*> (admin)",
		details: []string{"Manages API agents for autonomous ticket processing.", "Agent commands: `request` fetches work envelope; `run` continuously processes work.", "Admin commands: manage agent lifecycle, credentials, and configuration."},
		example: "tk agent run -id <uuid>",
	},
	"story": {
		usage:   "tk story <create|list|get|update|delete>",
		details: []string{"Manages stories within the active project.", "Stories provide a lightweight grouping layer within a project."},
		example: "tk story create -title \"User onboarding flow\"",
	},
	"config": {
		usage:   "tk config <set|get|ls|list|rm|delete|registration-enable|registration-disable> [key] [value]",
		details: []string{"Local config supports `set/get/ls/rm` for keys: `server`, `username`, `current_project`, `current_epic_id`.", "Registration controls are server-backed and require admin privileges in remote mode."},
		example: "tk config ls",
	},
	"label": {
		usage:   "tk label <ls|create|rm|add|remove|show> [flags]",
		details: []string{"Manages project-wide labels and per-ticket label assignments.", "Run `tk label help` for the full verb list."},
		example: "tk label create -name bug -color red",
	},
	"time": {
		usage:   "tk time <log|list|total|delete> [flags]",
		details: []string{"Log and view time entries against tickets.", "Run `tk time help` for the full verb list."},
		example: "tk time log -id TK-1 -m 30 -note \"morning session\"",
	},
	"role": {
		usage:   "tk role <ls|create|get|update|rm> [flags]",
		details: []string{"Admin command for managing roles.", "Run `tk role help` for the full verb list."},
		example: "tk role ls",
	},
	"sdlc": {
		usage:   "tk sdlc <ls|create|get|rm|set|unset|add-stage|remove-stage|reorder-stages|role-list|role-add|role-get|role-update|role-rm|stage-role-add|stage-role-rm|stage-role-order|export|import> [flags]",
		details: []string{"Admin command for managing sdlcs and their stages.", "Run `tk sdlc help` for the full verb list."},
		example: "tk sdlc ls",
	},
	"decision": {
		usage:   "tk decision <new|ls> [flags]",
		details: []string{"Record and list architectural or project decisions.", "Run `tk decision help` for the full verb list."},
		example: "tk decision new \"Use Postgres for production\"",
	},
	"summary": {
		usage:   "tk summary",
		details: []string{"Prints a daily starting-point summary: active tickets, recently updated items, and project health for the current project."},
		example: "tk summary",
	},
	"doctor": {
		usage:   "tk doctor",
		details: []string{"Interactive health review that checks project configuration, orphan tickets, and sdlc consistency."},
		example: "tk doctor",
	},
	"whoami": {
		usage:   "tk whoami",
		details: []string{"Prints the current effective username from config or environment."},
		example: "tk whoami",
	},
	"bug": {
		usage:   "tk bug \"title\" [flags]",
		details: []string{"Shortcut for `tk add -type bug`. Accepts the same flags as `tk add`.", "Use `tk bug get <id>` to fetch an existing bug without creating a new ticket."},
		example: "tk bug \"Reset token expires immediately\"",
	},
	"epic": {
		usage:   "tk epic \"title\" [flags]",
		details: []string{"Shortcut for `tk add -type epic`. Accepts the same flags as `tk add`.", "Epic subcommands: `tk epic get <id>`, `tk epic use <id>`, `tk epic clear`, and `tk epic ls`."},
		example: "tk epic \"Authentication\"",
	},
	"note": {
		usage:   "tk note \"title\" [flags]",
		details: []string{"Shortcut for `tk add -type note`. Accepts the same flags as `tk add`.", "Use notes to capture lightweight information that doesn't fit other ticket types.", "Use `tk note get <id>` to fetch an existing note."},
		example: "tk note \"Remember to update the README\"",
	},
	"question": {
		usage:   "tk question \"title\" [flags]",
		details: []string{"Shortcut for `tk add -type question`. Accepts the same flags as `tk add`.", "Use questions to track open decisions that need answering.", "Use `tk question get <id>` to fetch an existing question."},
		example: "tk question \"Should we use Postgres or SQLite?\"",
	},
	"board": {
		usage:   "tk board [-stage <stage>] [-assignee <user>]",
		details: []string{"Displays a kanban-style board view of tickets in the active project grouped by stage.", "Columns: design, develop, test, done. Tickets show key, title, and assignee.", "Filter by stage with `-stage` or by user with `-assignee`."},
		example: "tk board",
	},
	"history": {
		usage:   "tk history [-n <limit>] [-offset <offset>] [-id <id>|<id>]",
		details: []string{"Shows lifecycle history events for a ticket or recent project history when no id is given.", "Per-ticket history supports pagination via `-n` and `-offset`.", "`-user_id`, `-agent_id`, and `-team_id` filters apply to project history mode (no ticket id)."},
		example: "tk history TK-42",
	},
	"stage": {
		usage:   "tk stage [-id] <id> <stage>",
		details: []string{"Alias for `tk state`. Sets the lifecycle stage or state of a ticket.", "Valid stages: design, develop, test, done. Valid states: idle, active, success, fail."},
		example: "tk stage TK-42 develop",
	},
	"ls": {
		usage:   "tk ls [-t <type>] [-stage <stage>] [-state <state>] [-status <status>] [-u <user>] [-n <limit>]",
		details: []string{"Alias for `tk list`. Lists open tickets in the active project.", "Filter by type, stage, state, rendered status, or assignee."},
		example: "tk ls -t bug -stage develop",
	},
	"init": {
		usage:   "tk init [-prefix <prefix>] [-name <name>] [-git <repository-url>] [-sdlc <name>]",
		details: []string{"Interactive setup for local or remote mode.", "In local mode, initializes `.ticket/ticket.db`, generates an admin password, and can apply project bootstrap flags (`-prefix`, `-name`, `-git`, `-sdlc`).", "In remote mode, configures server URL, login/registration, and active project selection."},
		example: "tk init",
	},
	"curate": {
		usage:   "tk curate",
		details: []string{"Merges and curates requirements by finding near-duplicate ideas and presenting them for consolidation.", "Runs an AI-assisted grouping step to identify overlapping requirements."},
		example: "tk curate",
	},
	"review": {
		usage:   "tk review",
		details: []string{"Lists all requirements/ideas grouped by status (pending, accepted, rejected).", "Useful for a quick product-owner review of the current backlog of ideas."},
		example: "tk review",
	},
	"accept": {
		usage:   "tk accept requirement <id>",
		details: []string{"Marks a requirement as accepted (state=success) in the idea pipeline.", "Requires the `requirement` sub-noun before the id."},
		example: "tk accept requirement 3",
	},
	"reject": {
		usage:   "tk reject <id>\n  tk reject requirement <id>",
		details: []string{"`tk reject <id>` sends a ticket back to the first stage in its current workflow, marks it as draft, and sets the state to idle.", "`tk reject requirement <id>` keeps the requirement shortcut that marks a requirement as rejected in the idea pipeline."},
		example: "tk reject TK-T-42",
	},
	"revise": {
		usage:   "tk revise requirement <id>",
		details: []string{"Renames a requirement by appending \"(revised)\" and resets it to the shaping state.", "Use this to reopen an accepted or rejected idea for further refinement."},
		example: "tk revise requirement 3",
	},
	"conversation": {
		usage:   "tk conversation [-id] <id>",
		details: []string{"Shows the full comment and lifecycle conversation thread for a ticket.", "Events and comments are listed in chronological order."},
		example: "tk conversation TK-42",
	},
}

func renderRootUsage() string {
	var b strings.Builder
	b.WriteString(renderBanner())
	h := "\x1b[38;5;117m" // pastel blue
	r := "\x1b[0m"
	b.WriteString("\n" + h + "USAGE" + r + "\n")
	b.WriteString("  tk <noun> <verb> [flags]\n")
	b.WriteString("  Verbs: ls, new, get, update, rm (consistent across commands)\n\n")
	commandRows := [][2]string{
		{"ticket", "Manage tickets (ls, new, get, update, rm, state, assign, close)"},
		{"idea", "Capture and refine requirements (ls, new, get, shape, accept, reject)"},
		{"project", "Manage projects (ls, new, get, use, rm, init)"},
		{"dep", "Manage dependency links (add, remove)"},
		{"label", "Manage labels (ls, new, rm, add, remove, show)"},
		{"time", "Log and view time entries (log, ls, total, rm)"},
		{"story", "Manage stories (ls, new, get, update, rm)"},
		{"decision", "Record and list decisions (ls, new)"},
		{"doctor", "Interactive health review (project, ticket)"},
	}
	b.WriteString(h + "COMMANDS" + r + "\n")
	printCommandUsageRows(&b, commandRows, 10)
	adminRows := [][2]string{
		{"role", "Manage roles (ls, new, get, update, rm)"},
		{"sdlc", "Manage sdlcs (ls, new, get, rm, set, unset)"},
		{"team", "Manage teams (ls, new, update, rm)"},
		{"agent", "Manage agents (ls, new, update, rm, run)"},
		{"user", "Manage users (ls, new, rm, enable, disable)"},
	}
	b.WriteString("\n" + h + "ADMIN" + r + "\n")
	printCommandUsageRows(&b, adminRows, 10)
	shortcutRows := [][2]string{
		{"tk", "Show this usage guide"},
		{"tk add", "Create a ticket (alias: tk ticket add)"},
		{"tk bug", "Create a bug (alias: tk ticket add -type bug)"},
		{"tk epic", "Create an epic (alias: tk ticket add -type epic)"},
		{"tk idea new", "Capture a requirement"},
		{"tk idea ls", "List requirements"},
	}
	b.WriteString("\n" + h + "SHORTCUTS" + r + "\n")
	printCommandUsageRows(&b, shortcutRows, 10)
	systemRows := [][2]string{
		{"status", "Show connection and authentication status"},
		{"summary", "Daily starting-point overview"},
		{"whoami", "Print current username"},
		{"server", "Start the API server and web UI"},
		{"login", "Log into the server"},
		{"logout", "Clear the local session"},
		{"register", "Create a user account on the server"},
		{"config", "Manage local config keys"},
		{"init", "Interactive project setup"},
		{"initdb", "Initialize the database"},
		{"export", "Export entities to a JSON snapshot"},
		{"import", "Import entities from a JSON snapshot"},
		{"version", "Print the current version"},
		{"upgrade", "Check for a newer version"},
		{"skill", "Print the embedded SKILL.md template"},
		{"help", "Show command help"},
	}
	b.WriteString("\n" + h + "SYSTEM" + r + "\n")
	printCommandUsageRows(&b, systemRows, 10)
	b.WriteString("\n" + h + "EXAMPLES" + r + "\n")
	b.WriteString("  tk ls                                       List open tickets\n")
	b.WriteString("  tk add -title \"Fix login bug\" -type bug     Create a bug ticket\n")
	b.WriteString("  tk idea new \"Dark mode support\"             Capture a requirement\n")
	b.WriteString("  tk ticket get -id 42                        Show ticket details\n")
	b.WriteString("  tk ls -json | jq '.[].key' | xargs -I {} tk close -id {}   Close all tickets\n")
	b.WriteString("  tk summary                                  Your daily starting point\n")

	b.WriteString("\n" + h + "HELP" + r + "\n")
	b.WriteString("  tk <noun> help            Show verbs for a namespace\n")
	b.WriteString("  tk help <command>         Show detailed command help\n")
	return strings.TrimSpace(b.String()) + "\n"
}

func printCommandUsageRows(b *strings.Builder, rows [][2]string, commandWidth int) {
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintf(w, "  %-*s\t%s\n", commandWidth, row[0], row[1])
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not flush help table: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// Per-namespace help text — consistent format across all nouns
// ---------------------------------------------------------------------------

const depUsage = `Usage: tk dep <command> [flags]

Commands:
  add      -id <id> <depends-on-id>   Add a dependency
  remove   -id <id> <depends-on-id>   Remove a dependency`

const labelUsage = `Usage: tk label <command> [flags]

Commands:
  ls                                  List all project labels
  new      -name <name> [-color hex]  Create a label
  rm       -id <label-id>             Delete a label
  add      -id <ticket-id> <label-id> Tag a ticket with a label
  remove   -id <ticket-id> <label-id> Remove a label from a ticket
  show     -id <ticket-id>            Show labels on a ticket`

const timeUsage = `Usage: tk time <command> [flags]

Commands:
  log      -id <ticket-id> -m <minutes> [-note text]   Log time
  list     -id <ticket-id>                              List entries
  total    -id <ticket-id>                              Sum total time
  delete   -id <entry-id>                               Delete an entry`

const projectUsage = `Usage: tk project <command> [flags]

Commands:
  ls                                  List all projects
  new      -title <name>              Create a project
  get      <id>                       Show project details
  use      [<id>]                     Switch active project (or show current)
  rm       [-id] <id> [-confirm tok]  Delete a project (two-step)
  rename-prefix <new-prefix>          Rename prefix and re-key all tickets
  set-draft [-project_id <id>] <true|false>  Set project default draft mode
  sdlc     <id>                          Set sdlc on current project (use 0 to clear)
  init                                Init project in current directory
  add-user                            Add a user to a project
  remove-user                         Remove a user from a project
  add-team                            Add a team to a project
  remove-team                         Remove a team from a project`

const roleUsage = `Usage: tk role <command> [flags]

Commands:
  ls                                             List all roles
  get      -id <id>                              Show full role details
  new      -title <t> [-motivation m] [-goals g] Create a role
  update   -id <id> -title <t> [-motivation m] [-goals g] Update a role
  rm       -id <id>                              Delete a role`

const sdlcUsage = `Usage: tk sdlc <command> [flags]

Commands:
  ls                                  List all sdlcs
  new      -name <n> [-d desc]        Create a sdlc
  get      -id <id>                   Show sdlc details
  rm       -id <id>                   Delete a sdlc
  set      -ticket <id> -sdlc <id> Set sdlc on a ticket (overrides inherited)
  unset    -ticket <id>               Clear sdlc from a ticket (inherit from parent/project)
  add-stage    -id <wf-id> -name <n> [-wow text] [-dor text] [-dod text]  Add a stage
  stage-update -stage-id <id> -name <n> [-wow text] [-dor text] [-dod text] [-d desc] [-ac criteria]  Update a stage
  stage-get    -stage-id <id>         Show stage details
  stage-list   -id <sdlc_id>         List stages in a sdlc
  remove-stage -stage-id <id>         Remove a stage
  reorder-stages -id <wf-id> <ids>    Reorder stages
  role-list  -id <sdlc_id>                            List roles scoped to a sdlc
  role-add   -sdlc_id X -title "Role" [-description "..."] [-ac "..."]  Create a role scoped to a sdlc
  role-get   -sdlc_id X -role_id Y                    Show one sdlc-scoped role
  role-update -sdlc_id X -role_id Y -title "Role" [-description "..."] [-ac "..."]  Update a sdlc-scoped role
  role-rm    -sdlc_id X -role_id Y                    Delete a sdlc-scoped role
  stage-role-add -sdlc_id X -stage_id Y -role_id Z   Assign a role to a stage
  stage-role-rm  -sdlc_id X -stage_id Y -role_id Z   Remove a role from a stage
  stage-role-order -sdlc_id X -stage_id Y -roles 1,2  Reorder roles in a stage
  export   -id <id> [-o file]         Export a sdlc
  import   -file <file>               Import a sdlc`

const decisionUsage = `Usage: tk decision <command> [flags]

Commands:
  new      "text"                     Record a decision
  ls                                  List all decisions`

const teamUsage = `Usage: tk team <command> [flags]

Commands:
  ls                                             List all teams
  new      -name <name>                          Create a team
  update   -id <id> -name <name>                 Update a team
  rm       -id <id>                              Delete a team
  add-user     -team_id <id> -user_id <id>       Add a user
  remove-user  -team_id <id> -user_id <id>       Remove a user
  users        -id <id>                          List team users
  add-agent    -team_id <id> -agent_id <id>      Add an agent
  remove-agent -team_id <id> -agent_id <id>      Remove an agent
  agents       -id <id>                          List team agents`

const configUsage = `Usage: tk config <command> [flags]

Commands:
  ls, list                              List all config values
  get      <key>                        Get a config value
  set      <key> <value>                Set a config value
  rm       <key>                        Remove a config value
  registration-enable                   Enable user registration (server)
  registration-disable                  Disable user registration (server)

Keys: server, username, current_project, current_epic_id, registration_enabled`

const agentUsage = `Usage: tk agent <command> [flags]

Agent Commands:
  request  [flags]                    Request work for an agent
  run      [flags]                    Run an agent worker loop

Admin Commands:
  ls                                  List all agents
  new      [-password p]              Create an agent (UUID auto-generated)
  update   -id <id> -password <p>     Update an agent password
  rm       -id <id>                   Delete an agent
  enable   -id <id>                   Enable an agent
  disable  -id <id>                   Disable an agent
  reset-password -id <id> [-password] Reset an agent's password
  config-set -id <id> <key> <value>  Set a config value on an agent
  config-ls  -id <id>                List agent config values
  config-rm  -id <id> <key>          Remove a config value from an agent

Run flags:
  -id <uuid>               Agent UUID (or AGENT_ID env)
  
  -llm <command>           LLM to use (default: claude)
                           Values: claude (Sonnet 4.5), codex, or path to binary
  -project-id <id>         Project ID override (default: first open project)
  -poll-seconds <n>        Idle poll interval in seconds (default: 5)
  -v                       Verbose: stream LLM I/O to terminal

Password: AGENT_PASSWORD env var, or interactive prompt (input masked with *)`

const userUsage = `Usage: tk user <command> [flags]

Commands:
  ls                                       List all users
  new      -username <u> -password <p>     Create a user
  rm       -id <id>                        Delete a user
  enable   -id <id>                        Enable a user
  disable  -id <id>                        Disable a user
  reset-password -username <u> [-password] Reset password and invalidate sessions`

const storyUsage = `Usage: tk story <command> [flags]

Commands:
  ls                                       List stories in active project
  new      -title <title> [-d <desc>]      Create a story
  get      <id>                            Show story detail
  update   <id> -title <title> [-d <desc>] Update a story
  rm       <id>                            Delete a story`

const ideaUsage = `Usage: tk idea <command> [flags]

Commands:
  ls                                       List all ideas/requirements
  new      <title>                         Capture a new idea/requirement
  get      -id <id>                        Show idea detail
  shape    -id <id> [-d desc] [-ac ac]     Refine an idea
  accept   -id <id>                        Accept an idea
  reject   -id <id> -reason <reason>       Reject an idea
  revise   -id <id>                        Revert an accepted/rejected idea to shaping`

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
		"TICKET_USERNAME",
		"TICKET_PASSWORD",
		"TICKET_TIMEOUT",
		"AGENT_ID",
		"AGENT_PASSWORD",
		"TICKET_AGENT_LLM",
	}

	fmt.Println()
	fmt.Println("ENVIRONMENT")
	for _, name := range variableNames {
		secret := name == "TICKET_PASSWORD" || name == "AGENT_PASSWORD"
		value := statusEnvValue(name, secret)
		if value == "UNSET" {
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
	case "stage":
		return "stage"
	default:
		return command
	}
}
