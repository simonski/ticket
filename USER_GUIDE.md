# User Guide

`tk` is a ticket management tool.

This guide describes a single Go binary that provides a CLI, HTTP API/server,
embedded web application, and terminal UI backed by SQLite.

## How `ticket` Works

`tk` has four interfaces:

1. The CLI, which provides fast and explicit terminal workflows.
2. The HTTP API/server, which owns persistence, authentication, and collaboration.
3. The web app, which is embedded in the same binary and uses the same API.
4. The TUI, which provides full-screen terminal navigation and editing.

All project data follows the server data model and API semantics, whether you are working against a remote server or a local workspace.

`tk` keeps client runtime state minimal:

- `$TICKET_HOME` (default `~/.ticket`) stores remote credentials in
  `credentials.json`
- command routing comes from `TICKET_URL`, `TICKET_PROJECT`, explicit
  `-project_id`, and nearest-git-origin project matching

## Getting Started

Write the local agent instructions template into the current repository:

```bash
tk onboard
```

`tk onboard` prints the embedded onboarding template to stdout.

Print the embedded tk skill template:

```bash
tk skill
```

`tk skill` prints the embedded `SKILL.md` content to stdout.

Create the shared local database:

```bash
tk initdb
```

If `-f` is omitted, `tk initdb` creates or ensures the SQLite database at
`$TICKET_HOME/ticket.db` (default `~/.ticket/ticket.db`) and registers that
path as the global `local` remote.

If you run `tk initdb .`, Ticket creates `./.ticket/ticket.db`, registers a
named local remote for that path, and writes repo-local `.ticket/config.json`
so the current repo uses that database from then on.

`tk initdb` creates:

1. an `admin` account
2. the seeded `Public`/`PUB` project/team, the seeded `Private`/`PRIV` project, and a seeded `ticket`/`TK` project bound to `github.com/simonski/ticket.git` with the bootstrap `admin` user added as a project admin
3. default onboarding policy so newly created users join the public team and receive a private-project alias

Bootstrap resolution works like this:

- admin username: always `admin`
- admin password: `-password` if provided; use an explicit password for any shared or persistent server
- existing database file: overwritten only when `--force` is supplied
- optional seed data: include `--populate` to create 3 example projects (with stories, epics, tasks, bugs, chores) and example users across 3 teams
- non-interactive project setup: use `-prefix`, `-name`, and `-git` to rename the default project after bootstrap
- initial workflow selection: use `-workflow <name>` to assign one of the built-in Workflows during bootstrap
- project prefixes must be 1-5 uppercase ASCII letters

Project selection is automatic. For remote commands, Ticket resolves the project
in this order:

1. `-project_id`
2. `TICKET_PROJECT`
3. nearest git remote from the current working directory (walking up until `$HOME`)
4. the caller's default/private project on the server

Configure server access with environment variables:

```bash
export TICKET_URL=https://ticket.example.com
export TICKET_USERNAME=alice
export TICKET_PASSWORD=secret12
export TICKET_PROJECT=public
```

Remote mode uses OpenAPI/HTTP when `TICKET_URL`, `TICKET_USERNAME`, and
`TICKET_PASSWORD` are set. Local mode is used when those credentials are absent
and `~/.ticket/ticket.db` exists.

`TICKET_PROJECT` may be a numeric id, a project prefix, or the aliases
`public` / `private`. CLI flags such as `-project_id` override `TICKET_PROJECT`.
If neither is supplied in remote mode, the CLI sends the nearest git remote URL
and the server resolves the project by explicit ref first, then git-repository
match, then the caller's private project alias. If the git-repository heuristic
lands on a private project that accepts new members, Ticket returns an access
denied error that points at `POST /api/projects/<prefix>/access-requests`.
You can submit that request directly from the CLI with
`tk project request-access -project_id <prefix|id|public|private> [-message "..."]`.
Project admins can review those requests with
`tk project access-requests -project_id <prefix|id|public|private>` and decide
them with `tk project approve-access-request ... [-message "..."]` or
`tk project reject-access-request ... [-message "..."]`. Requesters can review their own pending
and decided requests with
`tk project my-access-requests [-status pending|approved|rejected]`.
The site2 Projects view now also shows an Access requests panel for project
admins on the selected project, with approve/reject actions for pending
requests.
Signed-in site2 users can also submit a project access request from the
Projects view by entering a project prefix or id plus an optional message, and
the same view now shows a "My access requests" panel with the caller's pending,
approved, and rejected requests, including any decision note an admin supplied.
Decision notifications now also appear in `tk user notifications` and in the
site2 Projects view's "Notifications" panel. Marking one as handled is
available through `tk user read-notification -id <notification-id>` or the web
UI's "Mark read" action.
The Projects view also shows recent project history for the selected project,
including access-request audit events, so the same audit trail is visible in the
web UI without dropping to the CLI.
Access-request creation and approval/rejection decisions are also recorded in
project history, so `tk history` without a ticket id shows the audit trail for
the active project.

Project admins can manage multiple git repositories on a project:

```bash
tk project repo ls -project_id CUS
tk project repo add -project_id CUS github.com/acme/customer-portal.git
tk project repo rm -project_id CUS github.com/acme/customer-portal.git
```

Snapshot export/import are removed from client mode. Run these as server-side
maintenance operations instead:

```bash
tk export -o ./ticket-snapshot.json   # server-side maintenance
tk import -i ./ticket-snapshot.json   # server-side maintenance
```

Snapshot files are JSON and include a `schema_version`; imports replace existing
database contents and preserve entity ids.

If the CLI reports that a database schema is older than the binary, port it
into a fresh database without modifying the source database:

```bash
tk -f old_ticket/ticket.db upgrade-database -o new_database/ticket.db
```

Older databases without explicit schema metadata are treated as legacy and must be upgraded before normal local commands will open them.

Start the server:

```bash
tk server
```

If `-f` is omitted, `tk server` uses the local database resolved from the
current configuration, which defaults to `$TICKET_HOME/ticket.db`.
If `-f` is provided, `tk server` uses that exact database file for that run.
`tk server` serves `site2` by default. Use `-site default` to serve the original embedded website.

If `-v` is supplied, `tk server` prints verbose request and response logs to stdout.
In `-v` mode, chat sessions also print prompt/output activity, heartbeat status with active connection/process counts, and per-process running/completed/error activity telemetry. The chat process starts when the first prompt is sent.

On startup, `tk server` also prints a colored ASCII-art `TICKET` banner before the listen message.

To run the server in Docker with a persistent SQLite volume from a repository
checkout:

```bash
docker compose -f deploy/compose.yaml up -d
docker compose -f deploy/compose.yaml logs -f
```

The container stores its database in the bind-mounted `./data` directory at
`/data/ticket.db` and initialises on first boot. Set `TICKET_ADMIN_PASSWORD`
before the first boot; the container refuses to initialise a new database
without it.

On a deployed host, copy `deploy/README.md` and `deploy/compose.yaml` to the
deployment directory as `README.md` and `compose.yaml`, then follow the minimal
commands in that README. If you need the compose YAML directly from the Ticket
binary, use `tk docker-compose > compose.yaml` and review it before deploying.

Immediately below the banner it prints:

- the embedded version
- the resolved task database path

By default the web app is available at `http://localhost:8080`.

Show the current CLI version:

```bash
tk version
```

`tk version` prints the semantic version embedded into the binary at build time. Each `make build` increments that semantic version before compiling the binary.

Running `ticket` with no arguments prints a colored ASCII-art `TICKET` banner above the main usage output.

If you are using the CLI against a running server on another host:

```bash
export TICKET_URL=http://your-server:8080
export TICKET_USERNAME=alice
export TICKET_PASSWORD=secret12
tk status
tk whoami
```

Set `TICKET_PROJECT` (or pass `-project_id`) when you want an explicit project
for commands like `tk ls`, `tk add`, or `tk summary`.
You can tune remote API timeout with `TICKET_TIMEOUT` (seconds, default `5`,
minimum `1`, maximum `30`).

As an admin create users:

```bash
tk user create -username XXXX -email user@example.com
created user xxxxx
password: generated-password
```

Server-side user creation and self-registration both provision users through the
active plan policy. By default that means:

1. the user is assigned to the `free` plan
2. the user is added to the shared public team
3. the user receives a private project alias named `private`

Admins can change the default plan, registration approval settings, and the
per-plan onboarding policy from the Projects view. Each plan can now control:

1. the default project alias handed to new users (`public` or `private`)
2. whether registration auto-assigns the shared public team
3. whether registration auto-creates a private project
4. whether registration auto-creates a private team

As an admin enable/disable users:

```bash
tk user enable -username XXXX
tk user disable -username XXXX
tk user ls|list
tk user rm|delete -username XXXX
```

These commands are admin-only. If a logged-in non-admin user runs them, the server returns `403` and the CLI prints `user is not an admin`.

Manage autonomous agents:

**Agent Commands:**
```bash
tk agent request [flags]
tk agent run -id <uuid>
```

**Admin Commands:**
```bash
tk agent create [-password <p>]  # UUID auto-generated
tk agent ls
tk agent update -id <uuid> -password <p>
tk agent disable -id <uuid>
tk agent enable -id <uuid>
tk agent delete -id <uuid>
tk agent reset-password -id <uuid> [-password <p>]
tk agent config-set -id <uuid> <key> <value>
tk agent config-ls -id <uuid>
tk agent config-rm -id <uuid> <key>
```

Run an agent worker process:

```bash
tk agent run -id <uuid>
```

`ticket agent run` resolves required settings from flags and env vars:

- `AGENT_ID` (flag: `-id`)
- `AGENT_PASSWORD` (no flag; read from env or prompted with `*` masking)
- selected remote from repo/global config
- `TICKET_AGENT_LLM` (optional, default: `claude`)

If any required values are missing, the command exits with an explicit missing-fields error.

The `-llm` flag selects the LLM: `claude` (default, uses Sonnet 4.5), `codex`, or a path to any binary. Use `-v` to stream all LLM input/output to the terminal with `>` / `<` prefixes.

## Accounts And Login

Create an account:

```bash
tk register -username name -email name@example.com
```

If `-password` is omitted during registration, the server generates one and
prints it in the response/output.

If self-registration is configured with auto-approval disabled, the account is
created in a disabled state. The CLI reports that registration was submitted
and tells the user to wait for approval or check email for next steps.

Set environment credentials for authenticated CLI use:

```bash
export TICKET_URL=https://ticket.localhost
export TICKET_USERNAME=name
export TICKET_PASSWORD='*******'
```

`tk login` stores a bearer token in `$TICKET_HOME/credentials.json`. Normal
remote CLI commands reuse that stored token until the server rejects or expires
it. If no stored token is available, remote commands fail fast and tell the
user to run `tk login` or set `TICKET_URL` plus `TICKET_USERNAME` /
`TICKET_PASSWORD` (or `TICKET_TOKEN`).

Check the current mode and connection state:

```bash
tk status
```

`tk status` prints current effective configuration and connection/auth state.

Inspect server-backed registration settings:

```bash
tk config ls
tk config get registration_enabled
```

`tk status` prints `TICKET_URL`, `TICKET_USERNAME`, and whether a password or
token is available, then checks the remote status endpoint.

- `connection: success` in green if the server responds successfully
- `connection: failure` in red if the server cannot be contacted or returns an error

Server connectivity and authentication status are shown in the same status view.

If `-nocolor` is set, the same output is printed without ANSI colors.

Show aggregate counts:

```bash
tk count
15
tk count -project_id 1
11
```

`ticket count` prints totals for users and work items by type. Without `-project_id` it also prints the total project count.

Log out:

```bash
tk logout
```

`ticket logout` removes the matching entry from `$TICKET_HOME/credentials.json`.

The web app uses the same account system. Once logged in, your session is shared across normal browser workflows.

Top banner behavior in the web UI:

- the left logo is rendered as animated 8x8 pixel glyphs and morphs continuously across `ticket`, `tkt`, and `tket`
- the `t` glyph does not light bottom-left or bottom-right pixels
- logo hue/luminance transitions use perlin-style noise and never hard-switch between words
- the center banner area renders an animated 8-bit activity stream using websocket event activity
- login/register pages use the same animated logo renderer in place of a static `ticket` heading and do not open websocket activity streams

## Typical Workflow

Most teams use `ticket` in this order:

1. Create or select a project.
2. Capture epics, tasks, and bugs.
3. Review and search what has been collected.
4. Assign, claim, and organize work.
5. Inspect dependencies and revision history.

## Projects

Create a project:

```bash
tk project create -prefix CUS -title "Customer Portal" \
  -wow "Portal ways of working" \
  -dor "Default ready criteria" \
  -dod "Default done criteria" \
  -ac "Default acceptance criteria" \
  -dor-map "develop=Implementation ready,qa=QA ready" \
  -ac-map "develop=Code reviewed"
```

The project is now the default project.

List projects:

```bash
tk project list
tk project ls
```

`tk project list` prints the project id, prefix, title, and status.

Select the project per command with `-project_id` or by exporting `TICKET_PROJECT`:

```bash
export TICKET_PROJECT=CUS
```

Rename a project's prefix (re-keys all tickets, updates all references):

```bash
tk project rename-prefix NEW
```

This changes every ticket key in the active project (e.g. `CUS-1` → `NEW-T-1`),
including parent references, dependencies, comments, history, and time entries.

### Repository-aware project routing

`tk` walks up from the current working directory to find the nearest `.git`
directory, sends that repository's `origin` URL to the server, and lets the
server resolve the matching project. If there is no matching repository and no
explicit project override, the server falls back to the caller's default/private
project.

Show project usage:

```bash
tk project
```

`ticket project` shows the current active project, or `no active project` if none is selected.

Get details on a project:

```bash
tk project get <prefix-or-id>
tk project CUS
```

Update a project:

```bash
tk project CUS update -title "New project title"
tk project CUS update -description "The new description"
tk project CUS update -wow "Updated ways of working"
tk project CUS update -dor "Updated default definition of ready"
tk project CUS update -dod "Updated default definition of done"
tk project CUS update -ac "Updated default acceptance criteria"
tk project CUS update -dor-map "qa=QA ready"
tk project CUS update -ac-map "develop=Reviewed and approved"
```

Enable or disable a project:

```bash
tk project CUS enable
tk project CUS disable
```

The active project is remembered by the CLI so you do not need to pass a project prefix for every command.

## Capture Work

Capture is intentionally lightweight. You can add project work as soon as it appears, then organize it later.

Add a task (type defaults to task)

```bash
tk add "Customers can reset their password."
tk add -dor "Ready for design" -ac "Password reset works end to end" "Customers can reset their password."
```

These are equivalent:

```bash
tk add "I am a new task"
tk create "I am a new task"
tk new "I am a new task"
tk add -title "I am a new task"
```

Add a bug:

```bash
tk bug "This is a bug"
```

Add an epic:

```bash
tk epic "This is an Epic"
```

```bash
tk create -t task -p 1 -a alice -d "This is a Task" -ac "Has a title and description" -estimate_effort 5 -estimate_complete 2026-04-30T17:00:00Z "This is a Task"
```

Creation defaults:

- `-t` / `-type`: defaults to `task`
- `-p` / `-priority`: defaults to `1`
- `-a` / `-assignee`: defaults to blank
- `-d` / `-description`: defaults to blank
- `-ac`: defaults to blank
- `-estimate_effort`: defaults to `0`
- `-estimate_complete`: defaults to blank and should use RFC3339 when set
- `-parent`: defaults to blank
- `-project`: defaults to the current project

Command aliases:

- `ticket add`, `ticket create`, and `ticket new` are the same command
- `ticket list` and `ticket ls` are the same command
- `ticket list -n <limit>` applies a server-side limit, where `0` means all results

Each captured item records:

- its project
- its author
- its creation time
- its current status
- its revision history

In the web app, use the capture panel at the top of the project page to create the same item types. Newly created items appear immediately for other connected users.

## Review And Search

List open items in the active project:

```bash
tk list
tk ls
tk list -n 20
tk ls -a              # include closed and archived tickets
```

Filter by item kind:

```bash
tk list --type task
tk list --type bug
tk list --type epic
```

Filter by lifecycle:

```bash
tk list --stage design
tk list --state active
tk list --status develop/idle
tk list --status done/complete
```

Filter by assignee:

```bash
tk list -u alice
tk ls -u alice
```

`ticket list` prints a table with ID (key with status icon), type, title, stage, state, ready, parent, assignee, and priority.

Search within the active project:

```bash
tk search "password reset"
tk search password reset -status develop/idle -owner alice
```

Search across all projects:

```bash
tk search password reset -allprojects
```

Show a single item:

```bash
tk get CUS-42
tk get -json CUS-42
```

`ticket get` accepts a ticket ID (key such as `CUS-42`). It prints the ticket fields directly, including `DependsOn`, the acceptance criteria, `EstimateEffort`, `EstimateComplete`, `CloneOf` when the ticket is a clone, and comments ordered most recent first.

Show orphaned items with no parent:

```bash
tk orphans
```

Assignment commands:

```bash
tk assign CUS-42 alice
tk unassign CUS-42 alice
tk dependency add 4 1,2,3
tk dependency remove 4 2
tk claim
tk claim -id CUS-42
tk claim -dry-run
tk unclaim CUS-42
tk request
tk request CUS-42
tk attach CUS-17 CUS-E-9
tk detach CUS-17
tk delete CUS-17
```

`ticket assign` and `ticket unassign` are admin-only.

They also fail if the named user does not exist or is disabled.

`ticket claim` is server-mediated. If the current user already has an active claimed ticket, that ticket is returned. Otherwise the server assigns the highest-priority oldest eligible ready `develop/idle` leaf ticket in the active project. `ticket claim -dry-run` shows the candidate without changing server state. `ticket unclaim` is retained as a compatibility alias for clearing your own assignment.

New tickets default to not-ready. Mark a ticket as ready before it can be picked up by `claim` or `request`:

```bash
tk undraft CUS-42      # mark ready for work
tk draft CUS-42        # mark as draft (not ready)
```

Only ready tickets are eligible for automatic assignment. You can still explicitly request a specific not-ready ticket by ID.

`ticket rm` and `ticket delete` soft-delete a ticket. Deleted tickets are hidden from normal reads and listings, and they still fail if the ticket has child tickets.

`ticket request` is the lower-level form of `ticket claim`. It accepts a ticket ID (key such as `CUS-42`). If no work can be assigned, the JSON response status is `NO-WORK`. If a specific ticket is requested and cannot be assigned, the JSON response status is `REJECTED`.

Lifecycle commands:

```bash
tk design CUS-42
tk develop CUS-42
tk test CUS-42
tk done CUS-42
tk idle CUS-42
tk active CUS-42
tk complete CUS-42
```

`ticket complete` keeps the current stage and marks the ticket state as `complete`. Use `ticket done` to move a ticket into terminal `done/complete`.

Most client-facing commands also support `-json` to pretty-print the JSON response.

Document management commands:

```bash
tk document new -title "Architecture notes" -content "Initial design"
tk document ls
tk document get 1
tk document update 1 -notes "Reviewed with team"
tk document file-add 1 -path ./notes.md
tk document file-ls 1
tk document file-get 1 1 -o ./downloaded-notes.md
tk document rm 1
```

Show the history of any item:

```bash
tk history CUS-42
```

`ticket history` prints the stored history events for that item.

Each ticket now also tracks first-class execution work items in the API. Use
`GET /api/tickets/{ticket_ref}/work-items` to inspect active/completed work-item
records linked to lifecycle transitions. Access is restricted to project
members and admins. Optional query filters: `status=active|success|fail|stopped`
and `assignee_type=human|agent`.

Project documents are available via:

- `GET|POST /api/projects/{project_ref}/documents`
- `GET|PUT|DELETE /api/documents/{document_id}`
- `GET|POST|DELETE /api/documents/{document_id}/labels` (DELETE requires `label_id` query param)
- `GET|POST /api/documents/{document_id}/files`
- `GET|DELETE /api/documents/{document_id}/files/{file_id}`

Work-item lifecycle operations are also available for members/admins:

- `POST /api/tickets/{ticket_ref}/work-items/{work_item_id}/reassign` (body: `assignee`, optional `message`)
- `POST /api/tickets/{ticket_ref}/work-items/{work_item_id}/cancel` (optional `message`)
- `POST /api/tickets/{ticket_ref}/work-items/{work_item_id}/retry` (optional `assignee`)
- `POST /api/tickets/{ticket_ref}/work-items/{work_item_id}/feedback` (optional `message`, optional `commit_ref`, at least one required)

Intervention decisions (`POST /api/tickets/{ticket_ref}/intervene`) are restricted
to project admins.

Intervention mailbox state is available at
`GET /api/tickets/{ticket_ref}/intervention-state` and
`POST /api/tickets/{ticket_ref}/intervention-state` (body: `state`) for
admin/member triage workflows.

Failure escalation inbox entries are available at:

- `GET /api/tickets/{ticket_ref}/inbox` (optional `?status=open|resolved`)
- `POST /api/tickets/{ticket_ref}/inbox/escalate` (admin decision queue entry; optional `message`)
- `POST /api/tickets/{ticket_ref}/inbox/{inbox_id}/decide` (body: `decision=clarify_goal|start_again|refine_requirements`, optional `message`)

The intervention queue endpoint (`GET /api/projects/{project_ref}/interventions`)
is restricted to project members and admins.

Intervention SLA reporting is available at
`GET /api/projects/{project_ref}/interventions/report` (optional
`?escalation_hours=<n>`), returning state counts, oldest active age, and
ticket-level escalation markers. Add `?format=csv` to export a CSV view.

Intervention trend points are available at
`GET /api/projects/{project_ref}/interventions/trends?days=7`.

Intervention drilldown slices for command-center triage are available at
`GET /api/projects/{project_ref}/interventions/drilldown?escalation_hours=24`.

Server-backed next-work forecasting is available at
`GET /api/projects/{project_ref}/forecast` (optional `?limit=<n>`). Site2 uses
this endpoint for the `Predicted next work` panel and shows confidence scores.

Forecast calibration metrics are available at
`GET /api/projects/{project_ref}/forecast/calibration?lookback_hours=1`.

Forecast backtesting metrics are available at
`GET /api/projects/{project_ref}/forecast/backtest?window_hours=24`.

Policy-ranked queue candidates are available at
`GET /api/projects/{project_ref}/work-items/queue?strategy=priority|order|aging`.

Global automation policy controls are available at
`GET|PUT /api/config/automation_policy`, and ticket-level diagnostics are
available at `GET /api/tickets/policy/{ticket_ref}`.

Agent model configuration supports inheritance from system → project → goal:

- `GET|PUT /api/config/agent-model` (system default + provider catalog)
- `GET|PUT /api/projects/{project_ref}/agent-model` (project override)
- `GET|PUT /api/goals/{goal_id}/agent-model` (goal override)
- `GET /api/goals/{goal_id}/agent-model/resolved` (effective resolved config)

Admin plan management is available via:

- `GET|POST /api/plans`
- `GET|PUT /api/plans/{plan_ref}`
- `GET|POST /api/plans/default`
- `POST /api/users/{username}/plan` with `plan_id` or `plan_slug`

Execution packet visibility is available at
`GET /api/tickets/{ticket_ref}/execution-packet`, returning the resolved
goal + role + phase + project guidance layers and the effective merged rules
used for agent execution.

Phase sign-off tracking is available at:

- `GET /api/tickets/{ticket_ref}/phase-signoffs` to view planning/implementation/verification approvals
- `POST /api/tickets/{ticket_ref}/phase-signoffs/{phase}` with body `{ "approved": true|false, "note": "..." }`

Workflow governance versioning is available at:

- `POST|GET /api/workflows/{id}/versions`
- `POST /api/workflows/{id}/versions/{version_id}/approve`
- `POST /api/workflows/{id}/versions/{version_id}/activate`

In the web app, the item detail pane shows:

1. the current item
2. dependencies
3. comments
4. revision history

## Terminal UI (TUI)

Launch the full-screen terminal UI:

```bash
tk -g
```

The TUI provides a keyboard-driven interface to your tickets without starting
a web server.

### Panels

The TUI has six top-level panels, navigated with **Tab** (forward) or
**Shift-Tab** / **left arrow** (back):

| Tab       | Contents                                              |
|-----------|-------------------------------------------------------|
| Home      | Project summary — counts by type, in-progress, recent history |
| Projects  | All projects; select one to make it active            |
| Ideas     | Captured requirements/ideas (raw, unprocessed tickets) |
| Tickets   | Ticket tree — epics with nested tasks, bugs, etc.     |
| Workflows | Workflow definitions and stages                       |
| Config    | Theme picker and TUI settings                         |

### Navigation

- **Tab / Shift-Tab** or **← →** — cycle panels
- **↑ ↓** — move cursor within a panel
- **Enter** — open/confirm selected item
- **←** / **→** on the ticket tree — collapse / expand an epic
- **q** or **Ctrl-C** — quit

### Ticket editor (TUI)

Open a ticket in the TUI editor directly from the CLI:

```bash
tk edit TK-42
```

The editor opens the full-screen TUI pre-focused on the selected ticket,
allowing you to update all fields inline without leaving the terminal.

Recent TUI workflow improvements:

- ticket detail now shows lifecycle context such as draft/archive/delete flags,
  lineage, effective Workflow source, current role, and resolved project / role /
  ticket guidance
- ticket create/edit forms expose draft and Workflow controls
- project edit exposes visibility, default draft, default Workflow, default
  guidance, and git repository fields

### Themes

The Config panel lists available themes. Arrowing up/down immediately applies
the highlighted theme so you can preview before leaving the panel.

### Persistence

The TUI saves the last active panel, cursor position, expanded epics, and
selected theme on exit. These are restored on next launch. Set
`tui_disable_persist: true` in your config to opt out.

## CLI Kanban Board (`tk board`)

`tk board` renders a stage-based kanban view of all tickets in the current
project directly in the terminal — no browser or server required.

```bash
tk board          # Current project, open tickets
tk board -a       # Include archived tickets
```

Tickets are grouped into columns by their **stage** (design → develop → test
→ done). Within each column, tickets are sorted by priority and last-modified
time.

Example output:

```
DESIGN (4)           DEVELOP (7)          TEST (2)             DONE (12)
──────────────────   ──────────────────   ──────────────────   ──────────────────
TK-42  Fix login     TK-38  Add labels    TK-51  Regression     TK-29  Auth
TK-44  Add search    TK-39  Pagination    TK-52  Perf test       TK-30  Register
TK-45  Dark mode     TK-40  Export CSV                           ...
...
```

`tk board -json` returns the same data as a JSON array for scripting.



The embedded web app is the easiest way to work visually across many related items.

Use it for:

1. capturing work during discovery and delivery
2. reviewing related items side by side
3. browsing task details and dependencies without switching commands
4. using the top-right header for project selection and profile actions only (there is no panel-dependent perspective button)
5. switching perspectives with `V`:
   - `stories`: high-level requirements for the current project
   - `board`: stage lanes for the current project
   - `agents`: opens agent management
   - `roles`: opens role management
   - `teams`: opens team management
   - `settings`: opens settings
   - `chat`: LLM chat panel
   - `tv : ticketvision`: a left-to-right project → epics → stories graph view
6. in `board`, cards are ordered by last modified timestamp (newest first)
   and each lane includes a quick `+ New` action plus an empty-lane prompt for
   creating a ticket directly into that stage
7. opening the `sections` left panel to jump directly to:
   - `stories`
   - `board`
   - `agents`
   - `roles`
   - `teams`
   - `settings`
   - `chat`
   - unavailable on the login page; it appears only after authentication
   - panel is visible by default and can be collapsed/expanded only via the `sections` minimise/grow control
   - when the viewport is short, the `sections` panel can scroll vertically so all selector items remain reachable
8. scrolling project content vertically in the main panel while the top banner and section selector controls stay visible
9. panels do not use an `Esc` close hint/binding; close behavior is controlled by in-panel navigation/actions
10. editing tickets in a dialog that shows a `Ticket Form` section with `Field` and `Value` table headers for all ticket inputs

Because the CLI and web app use the same server API, edits made in one interface appear in the other without any import or sync step.

Keyboard shortcuts in the board view:

- `D` on a focused ticket prompts `Archive this ticket?`; choose `OK` to archive
- `U` undoes the most recent ticket action you initiated in the current browser session
- `P` opens project edit for the currently selected project (swimlanes view)
- `R` opens the Roles dialog for role persona management
- `S` opens the Story dialog for creating a high-level requirement
- a fixed bottom-right version overlay shows the current server version reported by `/api/status`
- board updates are live via websocket; ticket changes from other users should appear without browser refresh
- websocket change messages include `entity_type`, `entity_id`, and `change_type` indicators (legacy `type` remains present for compatibility)
- the web client disables HTTP cache for API reads and keeps websocket health checks with frequent fallback sync so board state self-heals if websocket delivery is interrupted
- if websocket activity is quiet for 10+ seconds, the banner animator shows an idle waveform/pixel sweep until new events arrive
- profile menu includes `Agents`, `Roles`, and `Teams` browser panels
- each management panel can switch between `card` and `list` layouts
- clicking an agent, role, or team item opens a popup editor for create/update work
- agents support create/update/enable/disable/delete using the same API
- roles support create/update/delete role personas (`title`, `description`, `acceptance_criteria`)
- the ticket board supports drag-and-drop stage moves and shows card badges for
  draft, archived, and ticket-level Workflow overrides
- the new backlog perspective groups tickets by effective Workflow, lays out the
  ordered stage lanes for each workflow, and exposes quick Workflow/stage/role/status
  filters without needing any new server endpoints
- each ticket now has a History view that replays its journey across the Workflow as
  a stage track, with step-through controls and a switch between ticket-only and
  project-stream history using the existing history APIs
- the Workflow editor uses draggable stage cards so admins can reorder stages,
  edit ways-of-working / DoR / DoD inline, and add or remove stage roles from
  the same popup
- stage-role chips are draggable as well, so role order within a stage can be
  adjusted visually instead of only through CLI commands
- the Workflow editor now keeps an explicit stage/role selection and supports
  keyboard shortcuts for common authoring flows:
  - `N` focuses the new-stage composer
  - `E` focuses the selected stage title field
  - `Delete` / `Backspace` removes the selected role or stage after confirmation
  - `Left` / `Right` moves between stages
  - `Up` / `Down` moves between roles in the selected stage
- workflow create/edit now exposes `approval_policy` (`single_role` / `all_roles`)
  and `progression_mode` (`linear` / `stage_only`) so lifecycle progression can be
  managed from Site2 without CLI-only configuration
- workflow stages now support explicit DAG-style transitions (`next_stage_ids`) so
  progression can branch beyond simple linear ordering
- workflow editor now includes graph governance controls: validate graph,
  auto-chain transitions, and save-all stage bulk updates
- board now includes a `Predicted next work` panel that forecasts next
  phase/role transitions from workflow policy + explicit stage transitions, now
  sourced from server-side `/api/projects/{id}/forecast` with confidence scores
  and dependency-aware blocking
- board now supports quick filtering (`board-search`) and hide-done toggling for
  denser high-volume workflows
- the Interventions board includes built-in filter (`all`, `unassigned`, `agent`,
  `human`) and sort (`priority`, `order`, `most recent update`) controls for triage
- each intervention card now shows a compact conversation thread (latest comments)
  and supports adding intervention comments inline for accountable handoff context
- intervention cards expose quick actions for `retry-role` and `cancel` in addition
  to the existing decision dropdown submit path
- intervention cards now expose mailbox state (`open`, `triaged`, `in_progress`,
  `resolved`, `wont_fix`) with owner visibility for triage governance
- interventions view now includes an SLA summary bar from
  `/api/projects/{id}/interventions/report` (state counts + oldest active age)
- interventions view now shows drilldown highlights from
  `/api/projects/{id}/interventions/drilldown` (escalated count + top owner)
- predicted work summary now includes forecast backtesting signals from
  `/api/projects/{id}/forecast/backtest` alongside calibration data
- roles include `description` and `acceptance_criteria` fields for defining role personas
- `chat` opens an LLM conversation view with a bottom composer and upward-scrolling message history
- chat websocket traffic runs prompt-scoped external processes (default `codex exec`) and streams process stdout/stderr back to the browser; set `TICKET_CHAT_CMD` to override the command
- admin `settings` includes global chat limits:
  - max concurrent chat agents (default `2`)
  - max chat duration in minutes (default `3`)
- when chat capacity is full, new chat input is disabled until the server reports a free slot
- `/api/status` includes `chat_max_connections`, `chat_max_duration_minutes`, and `chat_running_processes`
- Story dialog includes `Analyse` which decomposes a story into epics and tasks using the `StoryReview` role
- story analyse spawns an external Codex process with Ticket server context and instructs Codex to run `tk create` commands for epics/tasks in the selected project
- Epic ticket dialog includes `Analyse` which decomposes an epic into tickets using the `EpicReview` role

Security notes:
- `tk agent run -llm` uses an explicit executable allow-list (`claude`, `codex` by default). Add extra names with `TICKET_AGENT_ALLOWED_LLM_BINARIES`.
- `TICKET_CHAT_CMD` and `TICKET_ANALYSE_CMD` run server-side subprocesses; keep these values operator-controlled and trusted.

## Command Reference

```bash
tk export -o ./ticket-snapshot.json
tk import -i ./ticket-snapshot.json
tk upgrade-database -o ./new_database/ticket.db
tk server -v
tk version

tk register -username <name> -email <email> [-password <password>]
tk status
tk config ls
tk config rm server
tk logout

tk user create -username <name> [-email <email>] [-password <password>]
tk user ls
tk user delete -username <name>
tk user enable -username <name>
tk user disable -username <name>
tk user reset-password -username <name> [-password <password>]
# Agent Commands
tk agent request [flags]
tk agent run -id <uuid>                     # remote from repo/global config; password from AGENT_PASSWORD env or prompt

# Agent Admin Commands
tk agent create [-password <password>]  # UUID auto-generated
tk agent list
tk agent update -id <uuid> -password <password>
tk agent delete -id <uuid>
tk agent enable -id <uuid>
tk agent disable -id <uuid>
tk agent reset-password -id <uuid> [-password <password>]
tk agent config-set -id <uuid> <key> <value>
tk agent config-ls -id <uuid>
tk agent config-rm -id <uuid> <key>

tk project create -prefix ABC -title "..."
tk project create -prefix ABC -title "..." -wow "Ways of working" -dor "Default definition of ready" -dod "Default definition of done" -ac "Default acceptance criteria"
tk project create -prefix ABC -title "..." -dor-map "develop=Implementation ready" -ac-map "qa=Sign-off complete"
tk project list
tk project ls
export TICKET_PROJECT=<prefix-or-id>
tk project
tk project get <prefix-or-id>
tk project repo ls -project_id <prefix-or-id>
tk project repo add -project_id <prefix-or-id> <git-repository>
tk project repo rm -project_id <prefix-or-id> <git-repository>
tk project <prefix-or-id>
tk project <prefix-or-id> update -title "..."
tk project <prefix-or-id> update -description "..."
tk project <prefix-or-id> update -wow "..."
tk project <prefix-or-id> update -dor "..."
tk project <prefix-or-id> update -dod "..."
tk project <prefix-or-id> update -ac "..."
tk project <prefix-or-id> update -dor-map "stage=text"
tk project <prefix-or-id> update -dod-map "stage=text"
tk project <prefix-or-id> update -ac-map "stage=text"
tk project <prefix-or-id> update -git-repository "https://github.com/org/repo.git"
tk project <prefix-or-id> enable
tk project <prefix-or-id> disable
tk project rename-prefix <new-prefix>
tk project rm [-id] <prefix-or-id> [--confirm <token>]

tk ticket <verb> [flags]                              # namespace for all ticket verbs

tk add "..."
tk bug "..."
tk epic "..."

tk list
tk ls
tk ls -a                                              # include closed/archived
tk list --type task
tk list --status develop/idle
tk list -u <name>
tk search "..."
tk search "..." -allprojects
tk get -id <key-or-id>
tk prompt <key-or-id>
tk edit [-id] <key-or-id>
tk history <key-or-id>
tk health <key-or-id>
tk comment add <key-or-id> "..."
tk orphans

tk dependency add <key-or-id> <key-or-id[,key-or-id...]>
tk dependency remove <key-or-id> <key-or-id[,key-or-id...]>
tk assign <key-or-id> <name>
tk unassign <key-or-id> <name>
tk claim -id <key-or-id>
tk unclaim <key-or-id>
tk request [<key-or-id>]
tk attach -id <key-or-id> <parent-key-or-id>
tk detach -id <key-or-id>
tk undraft <key-or-id>
tk draft <key-or-id>
tk complete <key-or-id>
tk reopen <key-or-id>
tk archive -id <key-or-id>
tk unarchive -id <key-or-id>
tk rm <key-or-id>
tk delete <key-or-id>
tk idle -id <key-or-id>
tk active -id <key-or-id>
tk complete -id <key-or-id>
tk state -id <key-or-id> <state>
tk update -id <key-or-id> -stage <stage> -state <state>
tk reject -id <key-or-id>
tk count
tk count -project_id <prefix-or-id>
tk whoami
tk summary

tk update -id <key-or-id> -stage develop -state idle
tk update -id <key-or-id> -title "new title"
tk update -id <key-or-id> -description "new description"
tk update -id <key-or-id> -dor "new default definition of ready"
tk update -id <key-or-id> -dod "new default definition of done"
tk update -id <key-or-id> -ac "new acceptance criteria"
tk update -id <key-or-id> -dor-map "develop=implementation ready"
tk update -id <key-or-id> -ac-map "qa=QA sign-off complete"
tk update -id <key-or-id> -git-repository "https://github.com/org/repo.git"
tk update -id <key-or-id> -git-branch "feature/x"
tk update -id <key-or-id> -priority 4
tk update -id <key-or-id> -order 7
tk update -id <key-or-id> -parent_id 12
tk update -id <key-or-id> -estimate_effort 5
tk update -id <key-or-id> -estimate_complete 2026-04-30T17:00:00Z
tk update -id <key-or-id> -stage develop -state active -priority 2 -title "new title"
tk new -f tickets.txt                    # preview file intent only (no writes)
tk new -f tickets.txt -commit            # create/update from file and write back ids
tk update -f tickets.txt                 # preview file update intent only
tk update -f tickets.txt -commit         # apply file updates (id: required per entry)

When you pass `-stage`, the value must be one of the stages in the ticket's current
workflow. If it is invalid, `tk update` prints the valid stages for that ticket.

File-driven ticket format (`tk new -f`, `tk update -f`) uses blocks:

```text
# Ticket title              # level 1 (root)
## Child ticket title       # level 2 (child of nearest level 1)
### Grandchild ticket title # level 3 (child of nearest level 2)
id: CUS-12              # optional for tk new; required for tk update
type: bug               # optional
labels: api, urgent     # optional (comma-separated)

Description line 1
Description line 2
```

The parser enforces valid hierarchy transitions. For example, `###` without a
preceding `##` parent is a parse error and the whole operation fails.

`tk new -f` without `-commit` prints an intent preview in `tk ls`-style rows plus:
`Tip: \`use -commit\` to write back to tk`; it does not call the server.
With `-commit`, new entries are created, entries with `id:` are updated, and the file
is rewritten with `id:` filled in for created tickets.

`tk update -f` requires `id:` for every entry. Without `-commit` it previews only
with the same tip message;
with `-commit` it updates title/description/type/labels from the file.

`tk reject -id <key-or-id>` sends a ticket back to the first stage in its current
workflow, sets the state to `idle`, and marks it as draft.

tk workflow list
tk workflow create -name <name> [-d <description>]
tk workflow get -id <id>
tk workflow delete -id <id>
tk workflow add-stage -id <workflow-id> -name <name> [-wow <text>] [-dor <text>] [-dod <text>] [-d <desc>] [-order <n>]
tk workflow stage-update -stage-id <id> -name <name> [-wow <text>] [-dor <text>] [-dod <text>] [-d <desc>] [-ac <criteria>]
tk workflow remove-stage -stage-id <id>
tk workflow reorder-stages -id <workflow-id> <stage_id,stage_id,...>
tk workflow export -id <id> [-o <file>]
tk workflow set -ticket <ticket-id> -workflow <workflow-id>
tk workflow stage-role-add -workflow_id <id> -stage_id <id> -role_id <id>
tk workflow stage-role-rm -workflow_id <id> -stage_id <id> -role_id <id>
tk workflow stage-role-order -workflow_id <id> -stage_id <id> -roles <id,id,...>

tk work-item list -id <ticket-id> [-status <active|success|fail|stopped>] [-assignee_type <human|agent>] [-limit <n>]
tk work-item queue [-project_id <id>] [-id <ticket-id>] [-dry-run] [-explain] [-strategy <priority|order|aging>] [-preview]
tk work-item start -id <ticket-id> [-m <message>]
tk work-item create [-project_id <id>] -title <title> [-type <task|bug|story|chore>] [-description <text>] [-start]
tk work-item reassign -id <ticket-id> -work-item <id> -assignee <username> [-m <message>]
tk work-item cancel -id <ticket-id> -work-item <id> [-m <message>]
tk work-item retry -id <ticket-id> -work-item <id> [-assignee <username>]
tk work-item feedback -id <ticket-id> -work-item <id> [-m <message>] [-commit_ref <sha>]
tk work-item state-get -id <ticket-id>
tk work-item state-set -id <ticket-id> -state <open|triaged|in_progress|resolved|wont_fix>

tk role list
tk role create -title <title> [-description <desc>] [-dor <text>] [-dod <text>] [-ac <criteria>] [-dor-map stage=text] [-dod-map stage=text] [-ac-map stage=text]
tk role get -id <id>
tk role update -id <id> [-title <title>] [-description <desc>] [-dor <text>] [-dod <text>] [-ac <criteria>] [-dor-map stage=text] [-dod-map stage=text] [-ac-map stage=text]
tk role delete -id <id>

tk decision add "text"
tk decision list

tk conversation show <key-or-id>

```


## Running a server

You can run your ticket system as a server.  First you need to convert the database so that
it can be used remotely

### 1. an admin user

```bash
tk user create -username admin -email admin@example.com
password: xxxx-xxxx-xxxx-xxxxx
```

### 2. a human user to interact with

```bash
tk user create -username my-username -email me@example.com
password: xxxx-xxxx-xxxx-xxxxx
```

### 3. Associate the user with the project you have been working on locally

```bash
tk project add-user -username username -role admin,member,commenter,observer
```

OR make the project public to any logged in user

```bash
tk project public ID
```

### 4. any agents you want to do the work

```bash
tk agent create
agent_id: xxxx-xxxx-xxxx
password: xxxx-xxxx-xxxx
```

### 5. Run the server

```bash
tk server
```

You can now run as the user

```bash
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=user-username
export TICKET_PASSWORD=user-password
export TICKET_PROJECT=DEMO
tk ls
```

You could run as an agent to do work automatically

```bash
export TICKET_PROJECT=DEMO
export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=agent-password
tk agent run                  # default LLM: claude (Sonnet 4.5)
tk agent run -llm codex       # use codex instead
tk agent run -v               # stream LLM I/O to terminal
```

Only non-draft tickets are eligible for automatic assignment (`tk undraft <id>`).
Agents are stored in the users table with `user_type=agent`.
