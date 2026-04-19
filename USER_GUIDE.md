# User Guide

`tk` is a ticket management tool.

This guide describes a single Go binary that provides a server, a CLI, and an embedded web application backed by SQLite.

## How `ticket` Works

`tk` has three interfaces:

1. The server, which owns persistence, authentication, and collaboration.
2. The CLI, which provides fast and explicit terminal sdlcs.
3. The web app, which is embedded in the same binary and uses the same API.

All project data follows the server data model and API semantics, whether you are working against a remote server or a local workspace.

Client-side files live under `.ticket/` in the active workspace.

`tk` walks up the directory tree from the current working directory looking for
a `.git` directory. When it finds one, it uses `.ticket/` at that repository
root. If no Git root is found, `.ticket/` in the current directory is used as
the default.

- `.ticket/config.json` stores non-sensitive client defaults such as the current username, server URL, and active project
- `.ticket/credentials.json` stores the current session token

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

Initialize a task sqlite database:

```bash
tk init
```

If `-f` is omitted, `tk init` creates the SQLite database at `.ticket/ticket.db` (at the nearest Git root, or in the current directory when no Git root exists). `TICKET_URL` can also override the active local or remote location.

`tk init` creates:

1. an `admin` account
2. the default project, `Default Project`, with project id `1` and prefix `TK`

Bootstrap resolution works like this:

- admin username: always `admin`
- admin password: `-password` if provided, otherwise `password`
- existing database file: overwritten only when `--force` is supplied
- optional seed data: include `--populate` to create 3 example projects (with stories, epics, tasks, bugs, chores) and example users across 3 teams
- non-interactive project setup: use `-prefix`, `-name`, and `-git` to rename the default project after bootstrap
- initial workflow selection: use `-sdlc <name>` to assign one of the built-in SDLCs during init
- project prefixes must be 1-5 uppercase ASCII letters

For example:

```bash
tk init -prefix CUS -name "Customer Portal" -git https://github.com/acme/customer-portal.git -sdlc agile
```

Create or restore database snapshots (LOCAL mode):

```bash
tk export -o ./ticket-snapshot.json
tk import -i ./ticket-snapshot.json
```

Snapshot files are JSON and include a `schema_version`; imports replace existing database contents and preserve entity ids.

If the CLI reports that a local database schema is older than the binary, port it into a fresh database without modifying the source database:

```bash
tk -f old_ticket/ticket.db upgrade-database -o new_database/ticket.db
```

Older databases without explicit schema metadata are treated as legacy and must be upgraded before normal local commands will open them.

Start the server:

```bash
tk server
```

If `-f` is omitted, `tk server` uses the database at `.ticket/ticket.db` (same resolution as `tk init`).
If `-f` is provided, `tk server` uses that exact database file and does not infer DB location from env vars or the default `.ticket/` workspace.
Use `-site default` to serve the existing embedded website, or `-site site2` to serve the fresh replacement frontend while keeping the old one available.

If `-v` is supplied, `tk server` prints verbose request and response logs to stdout.
In `-v` mode, chat sessions also print prompt/output activity, heartbeat status with active connection/process counts, and per-process running/completed/error activity telemetry. The chat process starts when the first prompt is sent.

On startup, `tk server` also prints a colored ASCII-art `TICKET` banner before the listen message.

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

If you are using the CLI against a running server on another host, configure TICKET_URL first:

```bash
export TICKET_URL=http://your-server:8080
```

For env-first remote mode (no local init/config required), set all three:

```bash
export TICKET_URL=http://your-server:8080
export TICKET_USERNAME=alice
export TICKET_PASSWORD=secret12
tk status
tk whoami
```

When all three are set, remote mode takes precedence over local `.ticket/config.json`.
In this mode, `tk login` is optional for normal commands because the client
auto-authenticates when issuing remote API calls.
You can tune remote API timeout with `TICKET_TIMEOUT` (seconds, default `5`,
minimum `1`, maximum `30`).

As an admin create users:

```bash
tk user create -username XXXX -password YYYY
created user xxxxx
```

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
- `TICKET_URL` (env/config)
- `TICKET_AGENT_LLM` (optional, default: `claude`)

If any required values are missing, the command exits with an explicit missing-fields error.

The `-llm` flag selects the LLM: `claude` (default, uses Sonnet 4.5), `codex`, or a path to any binary. Use `-v` to stream all LLM input/output to the terminal with `>` / `<` prefixes.

## Accounts And Login

Create an account:

```bash
tk register -username name -password '*******'
```

Log in:

```bash
tk login -username name -password '*******'
```

For `tk register`, you can omit the flags and let the CLI resolve them from `TICKET_USERNAME` and `TICKET_PASSWORD`. If those are not set, `tk register` falls back to `whoami` and `password`.

If `TICKET_URL` is set, it overrides the `location` in `.ticket/config.json`.
Bare paths and `file:///...` stay in local mode; `http(s)://...` switches to
remote mode. If that remote location also has `TICKET_USERNAME` and
`TICKET_PASSWORD` set, those credentials override stored remote credentials.

`tk login` resolves values in this order:

1. a valid session already stored in `.ticket/credentials.json`
2. the `username` already stored in `.ticket/config.json`
3. `-username` and `-password`
4. `TICKET_USERNAME` and `TICKET_PASSWORD`
5. interactive prompts for anything still missing

If login fails with `invalid credentials`, the CLI prints that message, prompts for username and password, and retries once.

When prompts are shown, any discovered values are presented as defaults that you can keep or replace.

When `tk login` prompts for a password in an interactive terminal, typed characters are masked with `*`.

On successful login:

- the session token is stored in `.ticket/credentials.json`
- the `username` and `location` fields in `.ticket/config.json` are updated

Registering a user does not log that user in or create local session credentials.

Check the current mode and connection state:

```bash
tk status
```

`tk status` always prints the current effective configuration first, then performs a mode-appropriate connectivity check.

Inspect or clear local CLI config keys:

```bash
tk config ls
tk config rm location
tk config delete project_id
```

Supported local keys are:

- `location`
- `username`
- `project_id`
- `current_epic_id`

In REMOTE mode it prints:

- `mode: remote`
- `location: <http(s)://server>`
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

`ticket logout` removes `.ticket/credentials.json`.

The web app uses the same account system. Once logged in, your session is shared across normal browser sdlcs.

Top banner behavior in the web UI:

- the left logo is rendered as animated 8x8 pixel glyphs and morphs continuously across `ticket`, `tkt`, and `tket`
- the `t` glyph does not light bottom-left or bottom-right pixels
- logo hue/luminance transitions use perlin-style noise and never hard-switch between words
- the center banner area renders an animated 8-bit activity stream using websocket event activity
- login/register pages use the same animated logo renderer in place of a static `ticket` heading and do not open websocket activity streams

## Typical SDLC

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

`tk project list` prints the project id, prefix, title, and status, and marks the active project with `*`.

Select the active project for subsequent commands:

```bash
tk project use CUS
```

Rename a project's prefix (re-keys all tickets, updates all references):

```bash
tk project rename-prefix NEW
```

This changes every ticket key in the active project (e.g. `CUS-1` → `NEW-T-1`),
including parent references, dependencies, comments, history, and time entries.
The config is updated automatically. Local mode only.

### Per-directory project binding

`tk` automatically locates the right workspace by walking up the directory tree
from the current working directory looking for a `.git` directory. The first Git
root found gets a `.ticket/` workspace. If there is no Git root, `tk` falls back
to `.ticket/` in the current directory. This means different repositories can
have separate databases and configs:

```bash
cd ~/code/project-1/
tk init                     # creates ~/code/project-1/.ticket/
tk add "A new ticket"       # uses project-1's database

cd ~/code/project-2/
tk init                     # creates ~/code/project-2/.ticket/
tk add "A new ticket"       # uses project-2's database
```

The workspace lookup order is:
1. Walk up from CWD looking for a `.git` directory and use `.ticket/` at that repository root
2. `.ticket/` in the current directory (default if no Git root is found)

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

Show the history of any item:

```bash
tk history CUS-42
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
| SDLCs | SDLC definitions and stages                       |
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
  lineage, effective SDLC source, current role, and resolved project / role /
  ticket guidance
- ticket create/edit forms expose draft and SDLC controls
- project edit exposes visibility, default draft, default SDLC, default
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
  draft, archived, and ticket-level SDLC overrides
- the new backlog perspective groups tickets by effective SDLC, lays out the
  ordered stage lanes for each workflow, and exposes quick SDLC/stage/role/status
  filters without needing any new server endpoints
- each ticket now has a History view that replays its journey across the SDLC as
  a stage track, with step-through controls and a switch between ticket-only and
  project-stream history using the existing history APIs
- the SDLC editor uses draggable stage cards so admins can reorder stages,
  edit ways-of-working / DoR / DoD inline, and add or remove stage roles from
  the same popup
- stage-role chips are draggable as well, so role order within a stage can be
  adjusted visually instead of only through CLI commands
- the SDLC editor now keeps an explicit stage/role selection and supports
  keyboard shortcuts for common authoring flows:
  - `N` focuses the new-stage composer
  - `E` focuses the selected stage title field
  - `Delete` / `Backspace` removes the selected role or stage after confirmation
  - `Left` / `Right` moves between stages
  - `Up` / `Down` moves between roles in the selected stage
- roles include `description` and `acceptance_criteria` fields for defining role personas
- `chat` opens an LLM conversation view with a bottom composer and upward-scrolling message history
- chat websocket traffic runs prompt-scoped external processes (default `codex exec`) and streams process stdout/stderr back to the browser; set `TICKET_CHAT_CMD` to override the command
- admin `settings` includes global chat limits:
  - max concurrent chat agents (default `2`)
  - max chat duration in minutes (default `3`)
- when chat capacity is full, new chat input is disabled until the server reports a free slot
- `/api/status` includes `chat_max_connections`, `chat_max_duration_minutes`, and `chat_running_processes`
- Story dialog includes `Analyse` which decomposes a story into epics and tasks using the `StoryReview` role
- story analyse spawns an external Codex process with remote `ticket` environment (`TICKET_URL`, `TICKET_USERNAME`, `TICKET_PASSWORD`) and instructs Codex to run `tk login` plus `tk create` commands for epics/tasks in the selected project
- Epic ticket dialog includes `Analyse` which decomposes an epic into tickets using the `EpicReview` role

Security notes:
- `tk agent run -llm` uses an explicit executable allow-list (`claude`, `codex` by default). Add extra names with `TICKET_AGENT_ALLOWED_LLM_BINARIES`.
- `TICKET_CHAT_CMD` and `TICKET_ANALYSE_CMD` run server-side subprocesses; keep these values operator-controlled and trusted.

## Command Reference

```bash
tk init
tk export -o ./ticket-snapshot.json
tk import -i ./ticket-snapshot.json
tk upgrade-database -o ./new_database/ticket.db
tk server -v
tk version

tk register -username <name> -password <password>
tk login -username <name> -password <password>
tk status
tk config ls
tk config rm server
tk logout

tk user create -username <name> -password <password>
tk user ls
tk user delete -username <name>
tk user enable -username <name>
tk user disable -username <name>
tk user reset-password -username <name> [-password <password>]
# Agent Commands
tk agent request [flags]
tk agent run -id <uuid>                     # TICKET_URL from env/config; password from AGENT_PASSWORD env or prompt

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
tk project init
tk project list
tk project ls
tk project use <prefix-or-id>
tk project
tk project get <prefix-or-id>
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

When you pass `-stage`, the value must be one of the stages in the ticket's current
workflow. If it is invalid, `tk update` prints the valid stages for that ticket.

`tk reject -id <key-or-id>` sends a ticket back to the first stage in its current
workflow, sets the state to `idle`, and marks it as draft.

tk sdlc list
tk sdlc create -name <name> [-d <description>]
tk sdlc get -id <id>
tk sdlc delete -id <id>
tk sdlc add-stage -id <sdlc-id> -name <name> [-wow <text>] [-dor <text>] [-dod <text>] [-d <desc>] [-order <n>]
tk sdlc stage-update -stage-id <id> -name <name> [-wow <text>] [-dor <text>] [-dod <text>] [-d <desc>] [-ac <criteria>]
tk sdlc remove-stage -stage-id <id>
tk sdlc reorder-stages -id <sdlc-id> <stage_id,stage_id,...>
tk sdlc export -id <id> [-o <file>]
tk sdlc set -ticket <ticket-id> -sdlc <sdlc-id>
tk sdlc stage-role-add -sdlc_id <id> -stage_id <id> -role_id <id>
tk sdlc stage-role-rm -sdlc_id <id> -stage_id <id> -role_id <id>
tk sdlc stage-role-order -sdlc_id <id> -stage_id <id> -roles <id,id,...>

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

Only non-draft tickets are eligible for automatic assignment (`tk undraft <id>`).
Agents are stored in the users table with `user_type=agent`.
