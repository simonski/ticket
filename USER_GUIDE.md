# User Guide

`ticket` is a ticket management tool.

This guide describes a single Go binary that provides a server, a CLI, and an embedded web application backed by SQLite.

## How `ticket` Works

`ticket` has three interfaces:

1. The server, which owns persistence, authentication, and collaboration.
2. The CLI, which provides fast and explicit terminal workflows.
3. The web app, which is embedded in the same binary and uses the same API.

All project data follows the server data model and API semantics, whether you are working against a remote server or a local workspace.

Client-side files live under `$TICKET_HOME`.

If `$TICKET_HOME` is not set, `tk` walks up the directory tree from the current
working directory looking for an existing `.ticket` directory. If none is found,
`.ticket` in the current directory is used as the default.

- `$TICKET_HOME/config.json` stores non-sensitive client defaults such as the current username, server URL, and active project
- `$TICKET_HOME/credentials.json` stores the current session token

## Getting Started

Write the local agent instructions template into the current repository:

```bash
ticket onboard
```

`ticket onboard` prints the embedded onboarding template to stdout.

Initialize a task sqlite database:

```bash
ticket init
```

If `-f` is omitted, `ticket init` creates the SQLite database at `$TICKET_HOME/ticket.db` (defaults to `.ticket/ticket.db` in the current or nearest parent directory).

`ticket init` creates:

1. an `admin` account
2. the default project, `Default Project`, with project id `1` and prefix `TK`

Bootstrap resolution works like this:

- admin username: always `admin`
- admin password: `-password` if provided, otherwise a generated random password printed to stdout
- existing database file: overwritten only when `--force` is supplied
- optional seed data: include `--populate` to create 3 example projects (with stories, epics, tasks, bugs, chores) and example users across 3 teams

Create or restore database snapshots (LOCAL mode):

```bash
ticket export -o ./ticket-snapshot.json
ticket import -i ./ticket-snapshot.json
```

Snapshot files are JSON and include a `schema_version`; imports replace existing database contents and preserve entity ids.

Start the server:

```bash
ticket server
```

If `-f` is omitted, `ticket server` uses the database at `$TICKET_HOME/ticket.db` (same resolution as `ticket init`).

If `-v` is supplied, `ticket server` prints verbose request and response logs to stdout.
In `-v` mode, chat sessions also print prompt/output activity, heartbeat status with active connection/process counts, and per-process running/completed/error activity telemetry. The chat process starts when the first prompt is sent.

On startup, `ticket server` also prints a colored ASCII-art `TICKET` banner before the listen message.

Immediately below the banner it prints:

- the embedded version
- the resolved task database path

By default the web app is available at `http://localhost:8080`.

Show the current CLI version:

```bash
ticket version
```

`ticket version` prints the semantic version embedded into the binary at build time. Each `make build` increments that semantic version before compiling the binary.

Running `ticket` with no arguments prints a colored ASCII-art `TICKET` banner above the main usage output.

If you are using the CLI against a running server on another host, configure TICKET_URL first:

```bash
export TICKET_URL=http://your-server:8080
```

As an admin create users:

```bash
ticket user create -username XXXX -password YYYY
created user xxxxx
```

As an admin enable/disable users:

```bash
ticket user enable -username XXXX
ticket user disable -username XXXX
ticket user ls|list
ticket user rm|delete -username XXXX
```

These commands are admin-only. If a logged-in non-admin user runs them, the server returns `403` and the CLI prints `user is not an admin`.

Manage autonomous agents:

**Agent Commands:**
```bash
ticket agent request [flags]
ticket agent run -id <uuid> -url http://localhost:8080
```

**Admin Commands:**
```bash
ticket agent create [-password <p>]  # UUID auto-generated
ticket agent ls
ticket agent update -id <uuid> -password <p>
ticket agent disable -id <uuid>
ticket agent enable -id <uuid>
ticket agent delete -id <uuid>
ticket agent reset-password -id <uuid> [-password <p>]
ticket agent config-set -id <uuid> <key> <value>
ticket agent config-ls -id <uuid>
ticket agent config-rm -id <uuid> <key>
```

Run an agent worker process:

```bash
ticket agent run -id <uuid> -url http://localhost:8080
```

`ticket agent run` resolves required settings from flags first, then env vars:

- `AGENT_ID` (flag: `-id`)
- `AGENT_PASSWORD` (no flag; read from env or prompted with `*` masking)
- `TICKET_URL` (flag: `-url`)
- `TICKET_AGENT_LLM` (optional, default: `claude`)

If any required values are missing, the command exits with an explicit missing-fields error.

The `-llm` flag selects the LLM: `claude` (default, uses Sonnet 4.5), `codex`, or a path to any binary. Use `-v` to stream all LLM input/output to the terminal with `>` / `<` prefixes.

## Accounts And Login

Create an account:

```bash
ticket register -username name -password '*******'
```

Log in:

```bash
ticket login -username name -password '*******'
```

For `ticket register`, you can omit the flags and let the CLI resolve them from `TICKET_USERNAME` and `TICKET_PASSWORD`. If those are not set, `ticket register` falls back to `whoami` and `password`.

`ticket login` resolves values in this order:

1. a valid session already stored in `$TICKET_HOME/credentials.json`
2. the `username` already stored in `$TICKET_HOME/config.json`
3. `-username` and `-password`
4. `TICKET_USERNAME` and `TICKET_PASSWORD`
5. interactive prompts for anything still missing

If login fails with `invalid credentials`, the CLI prints that message, prompts for username and password, and retries once.

When prompts are shown, any discovered values are presented as defaults that you can keep or replace.

When `ticket login` prompts for a password in an interactive terminal, typed characters are masked with `*`.

On successful login:

- the session token is stored in `$TICKET_HOME/credentials.json`
- the `username` and `server_url` fields in `$TICKET_HOME/config.json` are updated

Registering a user does not log that user in or create local session credentials.

Check the current mode and connection state:

```bash
ticket status
```

`ticket status` always prints the current effective configuration first, then performs a mode-appropriate connectivity check.

Inspect or clear local CLI config keys:

```bash
ticket config ls
ticket config rm server
ticket config delete current_project
```

Supported local keys are:

- `server`
- `username`
- `current_project`
- `current_epic_id`

In REMOTE mode it prints:

- `mode: remote`
- `server: <TICKET_URL>`
- `username: <configured username or blank>`
- `authenticated: true|false`

Then it calls the remote status endpoint and prints:

- `connection: success` in green if the server responds successfully
- `connection: failure` in red if the server cannot be contacted or returns an error

In LOCAL mode it prints:

- `mode: local`
- `db_path: <resolved database path>`
- `db_exists: true|false`

In LOCAL mode, commands act as the bootstrap `admin` user by default. No login or password prompt is required.

Then it opens the database if present and verifies the schema is usable. It prints:

- `connection: success` in green if the database can be opened and queried
- `connection: failure` in red if the database is missing, cannot be opened, or the schema is invalid

If the database does not exist in LOCAL mode, it also prints:

- `hint: run tk init`

If `-nocolor` is set, the same output is printed without ANSI colors.

Show aggregate counts:

```bash
ticket count
15
ticket count -project_id 1
11
```

`ticket count` prints totals for users and work items by type. Without `-project_id` it also prints the total project count.

Log out:

```bash
ticket logout
```

`ticket logout` removes `$TICKET_HOME/credentials.json`.

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
ticket project create -prefix CUS -title "Customer Portal" -description "Portal backlog" -ac "Portal launch criteria"
```

The project is now the default project.

List projects:

```bash
ticket project list
ticket project ls
```

`ticket project list` prints the project id, prefix, title, and status, and marks the active project with `*`.

Select the active project for subsequent commands:

```bash
ticket project use CUS
```

Rename a project's prefix (re-keys all tickets, updates all references):

```bash
ticket project rename-prefix NEW
```

This changes every ticket key in the active project (e.g. `CUS-T-1` → `NEW-T-1`),
including parent references, dependencies, comments, history, and time entries.
The config is updated automatically. Local mode only.

### Per-directory project binding

`tk` automatically locates the right workspace by walking up the directory tree
from the current working directory looking for an existing `.ticket` directory.
The first one found is used as `$TICKET_HOME`. This means different directories
can have separate databases and configs:

```bash
cd ~/code/project-1/
tk init                     # creates ~/code/project-1/.ticket/
tk add "A new ticket"       # uses project-1's database

cd ~/code/project-2/
tk init                     # creates ~/code/project-2/.ticket/
tk add "A new ticket"       # uses project-2's database
```

The lookup order is:
1. `$TICKET_HOME` env var if set
2. Walk up from CWD looking for an existing `.ticket` directory
3. `.ticket` in the current directory (default if none found)

Show project usage:

```bash
ticket project
```

`ticket project` shows the current active project, or `no active project` if none is selected.

Get details on a project:

```bash
ticket project get <prefix-or-id>
ticket project CUS
```

Update a project:

```bash
ticket project CUS update -title "New project title"
ticket project CUS update -description "The new description"
ticket project CUS update -ac "The acceptance criteria"
```

Enable or disable a project:

```bash
ticket project CUS enable
ticket project CUS disable
```

The active project is remembered by the CLI so you do not need to pass a project prefix for every command.

## Capture Work

Capture is intentionally lightweight. You can add project work as soon as it appears, then organize it later.

Add a task (type defaults to task)

```bash
ticket add "Customers can reset their password."
```

These are equivalent:

```bash
ticket add "I am a new task"
ticket create "I am a new task"
ticket new "I am a new task"
ticket add -title "I am a new task"
```

Add a bug:

```bash
ticket bug "This is a bug"
```

Add an epic:

```bash
ticket epic "This is an Epic"
```

```bash
ticket create -t task -p 1 -a alice -d "This is a Task" -ac "Has a title and description" -estimate_effort 5 -estimate_complete 2026-04-30T17:00:00Z "This is a Task"
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
ticket list
ticket ls
ticket list -n 20
ticket ls -a              # include closed and archived tickets
```

Filter by item kind:

```bash
ticket list --type task
ticket list --type bug
ticket list --type epic
```

Filter by lifecycle:

```bash
ticket list --stage design
ticket list --state active
ticket list --status develop/idle
ticket list --status done/complete
```

Filter by assignee:

```bash
ticket list -u alice
ticket ls -u alice
```

`ticket list` prints a table with ID (key with status icon), type, title, stage, state, ready, parent, assignee, and priority.

Search within the active project:

```bash
ticket search "password reset"
ticket search password reset -status develop/idle -owner alice
```

Search across all projects:

```bash
ticket search password reset -allprojects
```

Show a single item:

```bash
ticket get CUS-T-42
ticket get -json CUS-T-42
```

`ticket get` accepts a ticket ID (key such as `CUS-T-42`). It prints the ticket fields directly, including `DependsOn`, the acceptance criteria, `EstimateEffort`, `EstimateComplete`, `CloneOf` when the ticket is a clone, and comments ordered most recent first.

Show orphaned items with no parent:

```bash
ticket orphans
```

Assignment commands:

```bash
ticket assign CUS-T-42 alice
ticket unassign CUS-T-42 alice
ticket dependency add 4 1,2,3
ticket dependency remove 4 2
ticket claim
ticket claim -id CUS-T-42
ticket claim -dry-run
ticket unclaim CUS-T-42
ticket request
ticket request CUS-T-42
ticket attach CUS-T-17 CUS-E-9
ticket detach CUS-T-17
ticket delete CUS-T-17
```

`ticket assign` and `ticket unassign` are admin-only.

They also fail if the named user does not exist or is disabled.

`ticket claim` is server-mediated. If the current user already has an active claimed ticket, that ticket is returned. Otherwise the server assigns the highest-priority oldest eligible ready `develop/idle` leaf ticket in the active project. `ticket claim -dry-run` shows the candidate without changing server state. `ticket unclaim` is retained as a compatibility alias for clearing your own assignment.

New tickets default to not-ready. Mark a ticket as ready before it can be picked up by `claim` or `request`:

```bash
ticket ready CUS-T-42       # mark ready for work
ticket notready CUS-T-42    # mark not ready
```

Only ready tickets are eligible for automatic assignment. You can still explicitly request a specific not-ready ticket by ID.

`ticket rm` and `ticket delete` remove a ticket permanently. They fail if the ticket still has child tickets.

`ticket request` is the lower-level form of `ticket claim`. It accepts a ticket ID (key such as `CUS-T-42`). If no work can be assigned, the JSON response status is `NO-WORK`. If a specific ticket is requested and cannot be assigned, the JSON response status is `REJECTED`.

Lifecycle commands:

```bash
ticket design CUS-T-42
ticket develop CUS-T-42
ticket test CUS-T-42
ticket done CUS-T-42
ticket idle CUS-T-42
ticket active CUS-T-42
ticket complete CUS-T-42
```

`ticket complete` keeps the current stage and marks the ticket state as `complete`. Use `ticket done` to move a ticket into terminal `done/complete`.

Most client-facing commands also support `-json` to pretty-print the JSON response.

Show the history of any item:

```bash
ticket history CUS-T-42
```

`ticket history` prints the stored history events for that item.

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

### Themes

The Config panel lists available themes. Arrowing up/down immediately applies
the highlighted theme so you can preview before leaving the panel.

### Persistence

The TUI saves the last active panel, cursor position, expanded epics, and
selected theme on exit. These are restored on next launch. Set
`tui_disable_persist: true` in your config to opt out.

## Web Interface

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
- roles support create/update/delete role personas (`title`, `motivation`, `goals`)
- seeded roles include richer multi-paragraph `motivation` and `goals` text for classical delivery personas
- `chat` opens an LLM conversation view with a bottom composer and upward-scrolling message history
- chat websocket traffic runs prompt-scoped external processes (default `codex exec`) and streams process stdout/stderr back to the browser; set `TICKET_CHAT_CMD` to override the command
- admin `settings` includes global chat limits:
  - max concurrent chat agents (default `2`)
  - max chat duration in minutes (default `3`)
- when chat capacity is full, new chat input is disabled until the server reports a free slot
- `/api/status` includes `chat_max_connections`, `chat_max_duration_minutes`, and `chat_running_processes`
- Story dialog includes `Analyse` which decomposes a story into epics and tasks using the `StoryReview` role
- story analyse spawns an external Codex process with remote `ticket` environment (`TICKET_URL`, `TICKET_USERNAME`, `TICKET_PASSWORD`) and instructs Codex to run `ticket login` plus `ticket create` commands for epics/tasks in the selected project
- Epic ticket dialog includes `Analyse` which decomposes an epic into tickets using the `EpicReview` role

## Command Reference

```bash
ticket init
ticket export -o ./ticket-snapshot.json
ticket import -i ./ticket-snapshot.json
ticket server -v
ticket version

ticket register -username <name> -password <password>
ticket login -username <name> -password <password>
ticket status
ticket config ls
ticket config rm server
ticket logout

ticket user create -username <name> -password <password>
ticket user ls
ticket user delete -username <name>
ticket user enable -username <name>
ticket user disable -username <name>
ticket user reset-password -username <name> [-password <password>]
# Agent Commands
ticket agent request [flags]
ticket agent run -id <uuid> -url <server-url>  # password from AGENT_PASSWORD env or prompt

# Agent Admin Commands
ticket agent create [-password <password>]  # UUID auto-generated
ticket agent list
ticket agent update -id <uuid> -password <password>
ticket agent delete -id <uuid>
ticket agent enable -id <uuid>
ticket agent disable -id <uuid>
ticket agent reset-password -id <uuid> [-password <password>]
ticket agent config-set -id <uuid> <key> <value>
ticket agent config-ls -id <uuid>
ticket agent config-rm -id <uuid> <key>

ticket project create -prefix ABC -title "..."
ticket project init
ticket project list
ticket project ls
ticket project use <prefix-or-id>
ticket project
ticket project get <prefix-or-id>
ticket project <prefix-or-id>
ticket project <prefix-or-id> update -title "..."
ticket project <prefix-or-id> update -description "..."
ticket project <prefix-or-id> update -ac "..."
ticket project <prefix-or-id> update -git-repository "https://github.com/org/repo.git"
ticket project <prefix-or-id> update -git-branch "main"
ticket project <prefix-or-id> enable
ticket project <prefix-or-id> disable
ticket project rename-prefix <new-prefix>
ticket project rm [-id] <prefix-or-id> [--confirm <token>]

ticket ticket <verb> [flags]                              # namespace for all ticket verbs

ticket add "..."
ticket bug "..."
ticket epic "..."

ticket list
ticket ls
ticket ls -a                                              # include closed/archived
ticket list --type task
ticket list --status develop/idle
ticket list -u <name>
ticket search "..."
ticket search "..." -allprojects
ticket get -id <key-or-id>
ticket edit [-id] <key-or-id>
ticket history <key-or-id>
ticket health <key-or-id>
ticket comment add <key-or-id> "..."
ticket orphans

ticket dependency add <key-or-id> <key-or-id[,key-or-id...]>
ticket dependency remove <key-or-id> <key-or-id[,key-or-id...]>
ticket assign <key-or-id> <name>
ticket unassign <key-or-id> <name>
ticket claim -id <key-or-id>
ticket unclaim <key-or-id>
ticket request [<key-or-id>]
ticket attach -id <key-or-id> <parent-key-or-id>
ticket detach -id <key-or-id>
ticket ready <key-or-id>
ticket notready <key-or-id>
ticket close -id <key-or-id>
ticket open -id <key-or-id>
ticket archive -id <key-or-id>
ticket unarchive -id <key-or-id>
ticket rm <key-or-id>
ticket delete <key-or-id>
ticket idle -id <key-or-id>
ticket active -id <key-or-id>
ticket complete -id <key-or-id>
ticket state -id <key-or-id> <state>
ticket update -id <key-or-id> -stage <stage> -state <state>
ticket count
ticket count -project_id <prefix-or-id>
ticket whoami
ticket summary

ticket update -id <key-or-id> -stage develop -state idle
ticket update -id <key-or-id> -title "new title"
ticket update -id <key-or-id> -description "new description"
ticket update -id <key-or-id> -ac "new acceptance criteria"
ticket update -id <key-or-id> -git-repository "https://github.com/org/repo.git"
ticket update -id <key-or-id> -git-branch "feature/x"
ticket update -id <key-or-id> -priority 4
ticket update -id <key-or-id> -order 7
ticket update -id <key-or-id> -parent_id 12
ticket update -id <key-or-id> -estimate_effort 5
ticket update -id <key-or-id> -estimate_complete 2026-04-30T17:00:00Z
ticket update -id <key-or-id> -stage develop -state active -priority 2 -title "new title"

ticket workflow list
ticket workflow create -name <name> [-d <description>]
ticket workflow get -id <id>
ticket workflow delete -id <id>
ticket workflow add-stage -id <wf-id> -name <name>
ticket workflow remove-stage -stage-id <id>
ticket workflow reorder-stages -id <wf-id> <ids>

ticket decision add "text"
ticket decision list

ticket conversation show <key-or-id>

```


## Running a server

You can run your ticket system as a server.  First you need to convert the database so that
it can be used remotely

### 1. an admin user

```bash
tk user create -username admin -role admin
password: xxxx-xxxx-xxxx-xxxxx
```

### 2. a human user to interact with

```bash
tk user create -username my-username -role user
password: xxxx-xxxx-xxxx-xxxxx
```

### 3. Associate the user with the project you have been working on locally

```bash
tk project add-user -username username -role owner,editor,viewer
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
tk ls
```

You could run as an agent to do work automatically

```bash
export TICKET_URL=http://localhost:8080
export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=agent-password
tk agent run                  # default LLM: claude (Sonnet 4.5)
tk agent run -llm codex       # use codex instead
tk agent run -v               # stream LLM I/O to terminal
```

Only tickets marked as ready are eligible for automatic assignment (`tk ready <id>`).
Agents are stored in the users table with `user_type=agent`.

