# Design

The authoritative lifecycle remodel specification is
`docs/LIFECYCLE.md`. The authoritative phase 1 entity definition is
`docs/ENTITY_MODEL.md`. Where this file still describes older entity or
single-status assumptions, those dedicated documents take precedence until the
rest of this document is rewritten.

## Product Summary

`ticket` is a lightweight ticket and project management system delivered as a single Go binary.

It is designed for small teams that want low-friction ticket tracking without separate infrastructure for the API, database, and web UI. The product combines a server, a terminal-first CLI, and an embedded web application around one shared data model.

The system has four interfaces:

1. A server that owns persistence, authentication, and collaboration.
2. A CLI for fast, explicit terminal sdlcs.
3. A TUI for interactive terminal navigation and editing.
4. An embedded web application for browsing, editing, and status management.

The repository also contains a static `VERSION` file. `make build` increments the patch version before compiling the binary and copies that value into the embedded build asset used by `tk version`.

`tk` now splits state between a global home and per-project routing. `$TICKET_HOME`
(default `~/.ticket`) stores the shared local database, global config, the
named remote registry, and remote credentials. `tk` walks up from the current
working directory looking for `.ticket/config.json` first, then `.git`, and
uses that root for repo-local routing. The steady-state model is:

- global `~/.ticket/config.json` stores `default_remote` plus `remotes[]`
- repo-local `.ticket/config.json` stores `remote` plus `project_id`
- `~/.ticket/credentials.json` stores credentials per canonical remote URL

## Product Principles

1. The server defines the single system of record and the shared data model used by both remote and local sdlcs.
2. The CLI and web app use the same API semantics and data model.
3. Common operations should be fast and predictable from the terminal.
4. Projects should support lightweight hierarchy through epics and child tickets.
5. Every meaningful change should be traceable through history and comments.

## Primary Users And SDLCs

The primary user is a small software team managing projects, epics, tasks, bugs.

The first release must support these sdlcs end to end:

1. Initialize a local SQLite-backed workspace.
2. Store passwords as Argon2id hashes in SQLite.
3. Start the server and embedded web app from the same binary.
4. Create and manage users.
5. Authenticate from the CLI and the web app.
6. Create and select projects.
7. Add work items such as tasks, bugs, and epics.
8. List, filter, search, and inspect items.
9. Optionally organize work beneath a parent task or epic.
10. Review item history and comments.
11. Manage work visually in the web app, including status-based board views.

## Domain Model

### User

- `user_id` (UUID string, primary key)
- `username`
- `email`
- `email_confirmed_at`
- `password_hash`
- `role`
- `display_name`
- `enabled`
- `user_type` (`user` or `agent`)
- `description`
- `status`
- `last_seen`
- `created_at`
- `updated_at`

Roles in the first release:

- `admin`
- `user`

Notes:

- administrators can create, enable, and disable users
- regular users can log in and manage project work according to API permissions

### Agent

Agents are stored in the `users` table with `user_type='agent'`. They share the User schema:

- `user_id` (UUID string, also used as the agent's username and display name)
- `password_hash`
- `role` (always `agent`)
- `enabled`
- `status`
- `last_seen`
- `created_at`
- `updated_at`

Notes:

- agents represent autonomous worker processes that authenticate to the API
- agents are identified solely by their UUID — there is no separate name or description
- agent credentials are stored as hashes and never persisted in plaintext
- lifecycle status is tracked as `idle`, `working`, or `disabled`

### Role

- `role_id`
- `sdlc_id` (FK → sdlcs; roles are scoped to an SDLC)
- `title`
- `description`
- `acceptance_criteria`
- `created_at`
- `updated_at`

Notes:

- roles represent reusable agent personas for ticket work
- roles are scoped to an SDLC and assigned to stages via the `sdlc_stage_roles` junction table
- each stage can have multiple ordered roles; a ticket progresses through each role within a stage before advancing

### Project

- `project_id`
- `prefix`
- `title`
- `description`
- `acceptance_criteria`
- `git_repository`
- `notes`
- `visibility`
- `default_draft`
- `sdlc_id`
- `created_at`
- `created_by`
- `status`

Projects are the top-level container for work items.

### Ticket

`Ticket` is the main work artifact. All item types share one core model.

- `ticket_id` (TEXT PRIMARY KEY — the human-readable key, e.g. `TK-1`, `CUS-T-42`)
- `project_id`
- `parent_id`
- `clone_of`
- `sdlc_stage_id`
- `type`
- `title`
- `description`
- `acceptance_criteria`
- `git_repository`
- `git_branch`
- `stage`
- `state`
- `status` (rendered as `stage/state`)
- `priority`
- `order`
- `estimate_effort`
- `estimate_complete`
- `assignee`
- `health_score`
- `draft`
- `complete`
- `role_id`
- `sdlc_stage_id`
- `comments`
- `created_at`
- `created_by`
- `updated_at`
- `archived`

Stages are defined by the project's sdlc (default: design → develop → test → done).

States: `idle`, `active`, `success`, `fail`

- idle: not active in the stage
- active: currently being worked on in the stage
- success: completed this stage successfully (auto-advances to next sdlc stage)
- fail: completed this stage and deemed a failure


Supported `type` values:

- `epic`
- `task`
- `bug`
- `spike`
- `chore`
- `note`
- `question`
- `requirement`
- `decision`

Model notes:

- `parent_id` is nullable and supports hierarchical work
- tickets are orphaned when `parent_id` is null
- stories are separate first-class entities stored in `stories` and linked to tickets through `story_ticket_links`
- ticket creation accepts either a positional title or `-title`
- `acceptance_criteria` is captured directly on the task record
- `estimate_effort` is an integer assessment of task effort
- `estimate_complete` is the estimated delivery datetime and should use RFC3339 format
- `comments` are exposed on task detail reads as an array of `{author, date, text}` ordered most recent first

CLI creation defaults:

- `tk add`, `tk create`, and `tk new` are the same command
- `tk list` and `tk ls` are the same command
- if `-type` / `-t` is omitted, the type defaults to `task`
- if `-priority` / `-p` is omitted, the priority defaults to `1`
- if `-assignee` / `-a` is omitted, the assignee is blank
- if `-description` / `-d` is omitted, the description is blank
- if `-ac` is omitted, the acceptance criteria is blank
- if `-estimate_effort` is omitted, it defaults to `0`
- if `-estimate_complete` is omitted, it is blank
- if `-parent` is omitted, the ticket is created without a parent
- if `-project` is omitted, the active project is used

### SDLC

SDLCs define the ordered sequence of stages a ticket progresses through.

- `sdlc_id`
- `name`
- `description`
- `created_at`
- `updated_at`

Each sdlc has an ordered set of stages:

- `sdlc_stage_id`
- `sdlc_id`
- `stage_name`
- `description`
- `role_id` (optional, links to a Role for agent persona context)
- `sort_order`
- `created_at`
- `updated_at`

Notes:

- a default sdlc is seeded on init with stages: design → develop → test → done
- each stage can be linked to a role, giving agents persona context when working that stage
- projects reference a sdlc; tickets inherit stages from their project's sdlc
- when a ticket's state is set to `success`, it auto-advances to the next sdlc stage with state `idle`
- on the final stage, `success` means the ticket is complete
- sdlcs can be exported/imported as JSON for portability between instances

### Label

Labels are project-scoped tags for categorising tickets.

- `label_id`
- `project_id`
- `name`
- `color`
- `created_at`

Notes:

- labels are unique per project (project_id + name)
- tickets can have multiple labels via the `ticket_labels` join table
- deleting a label cascades to remove all ticket associations

### Time Entry

Time entries track effort logged against tickets.

- `time_entry_id`
- `ticket_id`
- `user_id`
- `minutes`
- `note`
- `created_at`

Notes:

- minutes must be positive
- entries are per-user, allowing per-person effort tracking
- total time for a ticket is the sum of all its entries

### History

Append-only audit log for important changes.

- `id`
- `project_id`
- `ticket_id`
- `event_type`
- `payload`
- `created_at`
- `created_by`

Typical history events:

- ticket created
- ticket updated
- status changed
- assignee changed
- parent changed
- comment added

## Functional Scope

### Workspace Initialization

The product must support local initialization of a SQLite database from the CLI.

The bootstrap commands are `tk initdb` and `tk init`.

`tk initdb` must:

1. create the schema in a new SQLite database
2. create an `admin` account
3. create a default project
4. optionally seed example data when `--populate` is supplied

Representative flow:

```bash
tk initdb . --force -password secret --populate
```

Bootstrap defaults:

- admin username is always `admin`
- if `-f` is omitted, the SQLite database is created at `$TICKET_HOME/ticket.db` and registered as the global `local` remote
- admin password comes from `-password` when supplied
- if `-password` is omitted, the default is `password`
- if `--force` is supplied, any existing SQLite database file is overwritten
- the default project is created automatically during initialization with prefix `TK`
- if `--populate` is supplied, the CLI seeds:
  - 3 example projects
  - stories in each project with associated epic/task/bug/chore tickets
  - 3 example teams with sample users assigned across those teams

`tk init` must bind the current repo or directory by writing `.ticket/config.json`.
In local mode that file binds the location to a project in the selected local
remote database. In remote mode it binds the location to a named remote and
project, while credentials remain under `$TICKET_HOME`.

Snapshot portability:

- CLI-only snapshot commands:
  - `tk export -o <file>` writes a JSON representation of persisted entities
  - `tk import -i <file>` replaces database contents from that JSON file
- snapshot JSON includes a `schema_version` field and table payloads
- import must preserve entity ids (primary keys) so relationships remain stable after restore

### Server

The server is the system of record.

Responsibilities:

- manage SQLite persistence
- expose the HTTP API for CLI and web use
- enforce authentication and authorization
- serve the embedded web application
- support multi-user access
- provide near-real-time refresh for connected clients

The default local server should listen on `http://localhost:8080`.

If `tk server` is run without `-f`, it must open the SQLite database resolved by the active local configuration, defaulting to `$TICKET_HOME/ticket.db`.

If `tk server` is run with `-v`, it must print verbose request and response details to stdout.
When chat is active, `-v` must also print chat process telemetry, including:
- inbound client prompts
- outbound LLM process output chunks
- periodic heartbeat lines with active websocket connection count and running process count
- per-process status lines with running/error/completed state and recent prompt/output activity ages
- chat processes are spawned lazily on first prompt, not immediately on websocket connect

### Authentication And User Management

The first release must support:

1. administrator bootstrap during initialization
2. user creation by administrators
3. user listing by administrators
4. user deletion by administrators
5. enable and disable user accounts
6. login and logout from CLI and web
7. user/session status inspection from the CLI
8. admin-managed autonomous agents and agent worker lifecycle requests

Representative commands:

```bash
tk onboard
tk version
tk user new -username alice -password secret
tk user ls
tk user rm -username alice
tk user enable -username alice
tk user disable -username alice

# Agent Commands
tk agent request [flags]
tk agent run -id <uuid>  # password from AGENT_PASSWORD env or prompt

# Agent Admin Commands
tk agent new [-password <p>]  # UUID auto-generated
tk agent ls
tk agent update -id <uuid> -password <p>
tk agent enable -id <uuid>
tk agent disable -id <uuid>
tk agent rm -id <uuid>
tk agent reset-password -id <uuid> [-password <p>]
tk agent config-set -id <uuid> <key> <value>
tk agent config-ls -id <uuid>
tk agent config-rm -id <uuid> <key>

tk register
tk login
tk status
tk logout
```

`tk onboard` must print the embedded onboarding instructions to stdout.

`tk status` must always print the current effective configuration first, then perform a mode-appropriate connectivity check.

In REMOTE mode it must print at least:

- `mode: remote`
- `server: <configured http(s) location>`
- `username: <configured username or blank>`
- `authenticated: true|false`

The REMOTE connectivity check is:

- call the remote status endpoint

The REMOTE result must then print:

- `connection: success` in green if the server responds successfully
- `connection: failure` in red if the server cannot be contacted or returns an error

In LOCAL mode it must print at least:

- `mode: local`
- `db_path: <resolved database path>`
- `db_exists: true|false`

In LOCAL mode, commands should default to the bootstrap `admin` user and should not require a password prompt.

The LOCAL connectivity check is:

- if the database file exists, open it and verify the schema is usable

A usable schema means:

- the required application tables exist and can be queried

The LOCAL result must then print:

- `connection: success` in green if the database can be opened and the schema is valid
- `connection: failure` in red if the database is missing, cannot be opened, or the schema is invalid

If the database does not exist in LOCAL mode, `tk status` must also print:

- `hint: run tk init`

If `-nocolor` is set, the same output must be printed without ANSI colors.

`tk count` must query the server and print aggregate counts for users and work item types. Without a project filter it must also print the project count. With `-project_id <id>` it must scope work item counts to that project.

The CLI must resolve credentials from the selected remote's stored session and username first, then `-username` and `-password`, and finally default to OS `whoami` and `password` when a command permits defaults.

The CLI must resolve remote server selection from the repo-local `remote` binding first, then the global `default_remote`, with legacy raw `location` values supported only for compatibility.

`tk config` must support:

- `tk config ls|list` to print local config keys and values
- `tk config rm|delete <key>` to clear a local config key
- supported removable keys: `location`, `username`, `project_id`, `current_epic_id`

The CLI must expose `tk version`, which prints the semantic version embedded into the binary at build time.

`tk init` is separate from the login and registration flows: it only binds the repo to a remote/project pair and must never persist secrets into repo-local config.

Admin-only user-management requests must be rejected by the server when the caller is authenticated but not an admin. Those requests must return HTTP 403 with an error explaining that the user is not an admin.

When `tk` is run without arguments, the CLI should print a colored ASCII-art `TICKET` banner above the main usage text.

When `tk server` starts, it should print the same colored ASCII-art `TICKET` banner before the startup message.

Below that banner, `tk server` must print the embedded version and the resolved task database path.

The CLI stores repo-local routing in `.ticket/config.json`, global defaults in
`$TICKET_HOME/config.json`, and remote session credentials in
`$TICKET_HOME/credentials.json`.

`tk login` must:

1. check `$TICKET_HOME/credentials.json` first and reuse that session if it is still valid
2. check the stored username for the selected remote
3. check `-username` and `-password`
4. prompt for any missing values
5. when prompting, use the discovered values as editable defaults
6. print `invalid credentials` on an invalid-login response before prompting for a retry
7. when prompting for a password in an interactive terminal, echo `*` characters instead of the raw password
8. on success, write the session token to `$TICKET_HOME/credentials.json`
9. on success, update the selected remote credential entry with the latest username and token

`tk register` must create the account but must not create or persist a logged-in session.

`tk logout` must remove the matching remote credential entry from `$TICKET_HOME/credentials.json`.

### Project Management

Users must be able to:

1. create projects
2. list projects
3. inspect project details
4. select an active project for CLI defaults

Representative commands:

```bash
tk project new -prefix CUS -title "Customer Portal"
tk project init
tk project ls
tk project use CUS
tk project get CUS
tk project rename-prefix NEW
```

`tk project ls` should show at least the project id, prefix, title, and status, and indicate which project is current in the local CLI context.

All `tk <command> new` commands must return to STDOUT the newly created ID, if they succeed.

The selected project should be remembered locally by the CLI.

### Work Item Capture

Creating work should be low-friction.

Users must be able to create tasks, bugs, and epics.

Representative commands:

```bash
tk add "Customers can reset their password."
tk create "Customers can reset their password."
tk new "Customers can reset their password."
tk bug "Reset token fails after first use."
tk epic "Authentication"
tk create -t task -p 1 -a alice -d "Add audit event" "Add password reset audit event"
```

Behavior notes:

- `tk add`, `tk create`, and `tk new` are aliases
- `tk list` and `tk ls` are aliases
- `tk list -n <limit>` applies a server-side limit, with `0` meaning no limit
- task creation defaults are `type=task`, `priority=1`, blank assignee, blank description, blank parent, and current project
- `-ac` stores acceptance criteria on the task
- each item records project, creator, timestamps, status, and revision history

### Review And Search

Users must be able to:

1. list all items in the active project
2. filter by type
3. filter by status
4. search across titles and descriptions within the active project by default
5. inspect full item detail
6. list orphaned items with no parent

Representative commands:

```bash
tk list
tk ls
tk list -type bug
tk list -status develop/idle
tk search "password reset"
tk search "password reset" -allprojects
tk get CUS-T-42
tk orphans
```

`tk search` should search the active project by default. If `-allprojects` is supplied, it should search across all projects.

The CLI should support `-json` on client-facing commands and pretty-print the response JSON.

`tk get <key-or-id>` should print a flat detail view with the fields `ID`, `Type`, `Description`, `ParentID`, `CloneOf` when present, `ProjectID`, `Title`, `Assignee`, `Order`, `EstimateEffort`, `EstimateComplete`, `DependsOn`, `Status`, `Priority`, `Created`, `LastModified`, `Acceptance Criteria`, and a `Comments` section ordered most recent first.

`tk list` should render a readable table that includes at least the id, type, status, assignee, priority, and title.

### SDLC And Lifecycle Management

The system should support ticket progression through explicit stage/state
changes.

The lifecycle model is:

- stages: defined by the project's sdlc (default: `design`, `develop`, `test`, `done`)
- states: `idle`, `active`, `success`, `fail`
- rendered status: `<stage>/<state>`

The CLI and web app must both support easy lifecycle changes.

Assignment sdlcs must support:

- `ticket assign <key-or-id> <name>` for admins
- `ticket unassign <key-or-id> <name>` for admins
- `ticket dependency add <key-or-id> <dependency-id[,dependency-id...]>`
- `ticket dependency remove <key-or-id> <dependency-id[,dependency-id...]>`
- `tk request [<key-or-id>]` for the caller
- `tk claim` or `tk claim -id <key-or-id>` for the caller
- `tk claim -dry-run` for preview without mutation
- `tk unclaim <key-or-id>` for the caller
- `tk attach <key-or-id> <parent-key-or-id>`
- `tk detach <key-or-id>`
- `tk rm <key-or-id>`
- `tk delete <key-or-id>`
- `tk list -u <name>` / `tk ls -u <name>` for assignee filtering
- `tk design <key-or-id>`
- `tk develop <key-or-id>`
- `tk test <key-or-id>`
- `tk done <key-or-id>`
- `ticket idle <key-or-id>`
- `ticket active <key-or-id>`
- `ticket complete <key-or-id>`
- `ticket update <key-or-id> -status <stage/state>`
- `ticket update <key-or-id> -title <title>`
- `ticket update <key-or-id> -description <description>`
- `ticket update <key-or-id> -ac <acceptance-criteria>`
- `ticket update <key-or-id> -priority <priority>`
- `ticket update <key-or-id> -order <order>`
- `ticket update <key-or-id> -parent_id <parent-id>`
- `ticket update <key-or-id> -estimate_effort <effort>`
- `ticket update <key-or-id> -estimate_complete <rfc3339-datetime>`

Assignment rules:

- the server must reject admin-only assignment calls made by non-admin users
- `ticket assign` and `ticket unassign` must fail if the named target user does not exist
- `ticket assign` and `ticket unassign` must fail if the named target user is disabled
- `ticket request <key-or-id>` must return `{"status":"REJECTED"}` when the requested task cannot be assigned
- `ticket request` must return `{"status":"NO-WORK"}` when no assignable work exists
- successful request responses must return `{"status":"ASSIGNED","task":...}`
- if the caller already has an assigned `develop/active` ticket, that ticket is returned
- otherwise, if the caller has assigned `develop/idle` work, the oldest assigned `develop/idle` ticket is returned
- otherwise, `ticket request` assigns the oldest unassigned `develop/idle` ticket in the active project
- `ticket claim` must reject an explicitly requested ticket if it is already assigned to another user
- `ticket unclaim` must fail if the caller is not the current assignee
- a non-admin user must not be able to override another user assignment through the generic task update API
- `ticket rm` / `ticket delete` must remove a task permanently
- `ticket rm` / `ticket delete` must fail if the task still has child tasks

### Hierarchy

Projects must support lightweight hierarchy through parent-child relationships.

The first release should support:

1. creating epics
2. attaching tasks and bugs to an epic via `parent_id`
3. tracking the active epic in the CLI for faster entry
4. browsing hierarchy in the web UI

### History And Comments

Users must be able to inspect how an item changed over time.

The first release must include:

1. append-only history events for important changes
2. comments attached to items
3. `ticket history <key-or-id>` in the CLI for event output
4. item detail pages in the web app that surface history and comments

Representative commands:

```bash
ticket history CUS-T-42
ticket comment add CUS-T-42 "Waiting on API changes."
```

## CLI Design

The CLI is the fastest interface for expert users.

Requirements:

- use the same HTTP API as the web app
- never bypass the server or SQLite
- support explicit and scriptable commands
- maintain local defaults for current project, credentials, and active epic where useful

Representative command set:

```bash
tk project new -prefix CUS -title "Customer Portal"
tk project use CUS

tk epic "Authentication"
tk add "Customers can reset their password."
tk bug "Reset token expires immediately."
tk ls
tk get CUS-T-42
tk search "password reset"
tk history CUS-T-42
```

The CLI should support only the aliases that are part of the documented command surface.

## Web Application

The web application is embedded into the Go binary with `go:embed`.

Requirements:

- single-page application
- operationally lightweight
- collaborative and multi-user aware
- no manual page refresh required for normal use
- project switcher
- status-based board view
- item detail view with history and comments
- animated top banner:
  - left logo is an 8x8-per-character pixel morph between `ticket`, `tkt`, and `tket`
  - logged-in header logo is capped to the section-selector width instead of stretching full-banner
  - morphing is continuous (no hard word switch) and uses perlin-style noise for hue/luminance drift
  - renderer uses a Three.js full-rectangle pass with nearest-neighbor pixel sampling
  - top-mid status strip uses a Three.js full-width/full-height rectangle and animated 8-bit activity pixels driven by websocket events
  - status pixel colours come from event classifications (`edit`, `create`, `status`, `done`, `bug`) with bug-biased red tones

The web UI should make these activities easy:

- switch between projects
- use the top-right header for project selection and profile actions only (no panel-dependent perspective button)
- open a left-side slide panel (`sections`) to jump to:
  - `stories`
  - `board`
  - `agents`
  - `roles`
  - `teams`
  - `settings`
  - `chat`
  - hidden on the login screen and shown only after authentication
  - defaults to open and remains open unless the user explicitly toggles `sections` minimise/grow
  - the selector panel must support vertical scrolling when viewport height is constrained
- the main content area should support vertical scrolling while preserving the sticky top banner and fixed section selector controls
- panels should not advertise or bind `Escape` for close behavior
- manage agents, roles, and teams from dedicated browser panels with selectable `card` or `list` layouts
- clicking an agent/role/team browser item opens a popup editor for create/update actions
- add and edit items
- ticket dialog presents a labeled form table with explicit `Field` and `Value` headers
- view hierarchy
- manage status on a board
- inspect history and comments
- switch perspectives with `V` via a popup selector:
  - `stories`: high-level requirements panel for the active project
  - `board`: current lane board
    - cards are sorted by last-modified timestamp descending (newest first)
  - `agents`: opens agent management panel
  - `roles`: opens role management panel
  - `teams`: opens team management panel
  - `settings`: opens settings panel
  - `chat`: websocket-backed LLM chat pane with bottom composer and upward-animated conversation history
  - `tv : ticketvision`: Three.js project graph laid out left-to-right as project → epics → stories
- keyboard actions on focused tickets:
  - `D`: prompt `Archive this ticket?` and archive on confirmation
  - `U`: undo the most recent ticket action initiated in the current web session
  - `P`: open project edit modal for the current project (swimlanes view)
  - `R`: open role management modal
  - `S`: open story creation modal
- a fixed bottom-right overlay displays `server_version` from `/api/status`
- board state is refreshed by websocket events and should not require manual browser reload
- websocket change indicators on `/api/ws` carry:
  - `entity_type` (for example `ticket`, `project`)
  - `entity_id` (the changed entity id)
  - `change_type` (for example `created`, `updated`, `deleted`, `users_updated`)
  - legacy `type` is still emitted for backward compatibility
- chat websocket (`/api/chat/ws`) executes prompt-scoped external commands (default `codex exec`) and maps prompt input to process stdin with streamed stdout/stderr output back to the browser
- chat runtime limits are configurable in `app_settings`:
  - `chat_max_connections` (default `2`)
  - `chat_max_duration_minutes` (default `3`)
- chat process spawn is denied when `running_processes >= chat_max_connections`; client chat input is disabled until capacity is available again
- chat processes are force-stopped once `chat_max_duration_minutes` is exceeded
- `/api/status` returns `chat_max_connections`, `chat_max_duration_minutes`, and `chat_running_processes`
- admins update chat limits through `POST /api/config/chat_limits`
- stories are stored as first-class entities (`stories`) associated to one project; generated epics/tasks are linked via `story_ticket_links`
- story analysis uses the `StoryReview` role and an external Codex process with remote Ticket context to run `tk login` + `tk create` breakdown commands for epics/tasks; story is marked `ready_for_review`
- epic analysis uses the `EpicReview` role to generate child implementation tickets
- API reads for board state should bypass browser cache and include websocket health/fallback sync to recover from delivery gaps
- when no websocket activity is seen for 10 seconds, the status strip renders idle motion (waveform/sweep) until activity resumes

## Persistence And Architecture

### Storage

- SQLite is the only database in the first release.
- SQLite remains the persistence layer behind the server data model; local mode uses the same data model and validation rules as the server-backed flow.

Storage areas (22 tables):

1. users (includes agents with `user_type='agent'`)
2. sessions
3. projects
4. project_members
5. tickets
6. ticket_history
7. comments
8. sdlcs
9. sdlc_stages
10. labels
11. ticket_labels
12. time_entries
13. roles
14. teams
15. team_members
16. team_agents
17. project_teams
18. dependencies
19. stories
20. story_ticket_links
21. history_events
22. app_settings

### Application Shape

The implementation should be organized around shared domain concepts rather than separate one-off logic in each interface.

Suggested layers:

1. domain models and validation
2. application services for auth, projects, tasks, comments, and history
3. HTTP handlers and API contracts
4. SQLite repositories
5. CLI commands and web UI clients consuming the API

## Non-Goals For The First Release

Avoid overbuilding the initial product.

Non-goals:

- multiple database backends
- direct client access to SQLite
- heavyweight enterprise sdlc configuration
- advanced portfolio planning
- deeply nested issue taxonomies beyond simple parent-child hierarchy

## Quality Gates

The repository should provide at least these checks:

```bash
make build
make test
make test-go
make test-playwright
```

`make build` must increment the patch component of the semantic version stored in `VERSION` before running the Go build.

Changes are not complete until the relevant automated checks pass.

## Success Criteria

The product is successful if a user can:

1. initialize a local workspace and start the server
2. create users and authenticate successfully
3. create and switch projects quickly
4. add tasks, bugs, and epics with minimal friction
5. inspect work through list, search, detail, history, and comments
6. manage work visually through the web interface


## Ticket Lifecycle

Tickets have a two-part status: `stage/state` (e.g. `develop/active`, `done/success`).

### SDLC-Driven Stages

Stages are defined by the project's sdlc (an ordered sequence of stages). The default sdlc has: `design → develop → test → done`.

Stages advance automatically: when a ticket's state is set to `success`, it moves to the next sdlc stage with state `idle`. On the final stage, `success` means the ticket is complete.

You cannot set a ticket's stage directly — use state commands to drive progression.

### State Commands

States: `idle`, `active`, `success`, `fail`

```bash
ticket idle N            # Pause work
ticket complete N        # Mark success (auto-advances stage)
ticket state N active    # Set state directly
ticket state N success   # Completes current stage, advances to next
ticket state N fail
```

### Other Update Commands

```bash
ticket update N -title <title>
ticket update N -description <description>
ticket update N -ac <acceptance-criteria>
ticket update N -priority <priority>
ticket update N -order <order>
ticket update N -parent_id <parent-id>
ticket update N -estimate_effort <effort>
ticket update N -estimate_complete <rfc3339-datetime>
```

## Requesting Tickets

A user can makes a request to work on a specific task

    `ticket request N`

It is either assigned the task it requested, or it is rejected. If assigned, the task is updated to have this user name and the response is `{"status":"ASSIGNED","task":...}`. If not, the response is `{"status":"REJECTED"}`.

Or a user may request ANY task

    ticket request

It is either assigned a task, or no work is available. If assigned, the task is updated to have this user name and the response is `{"status":"ASSIGNED","task":...}`. If not, the response is `{"status":"NO-WORK"}`.

If the user has already been assigned a `develop/active` ticket, that ticket is returned. If the user has been assigned a `develop/idle` ticket, then the oldest assigned `develop/idle` ticket is returned.

    
## Version checking

```bash
ticket upgrade
```

Fetches the `VERSION` file from the GitHub repository
(`https://raw.githubusercontent.com/simonski/ticket/refs/heads/main/cmd/tk/VERSION`)
with a 3-second timeout and compares it to the version embedded at build time.

Outcomes:

- **network unavailable** — fails fast (3 s timeout) with a friendly message
- **same version** — `You are on the latest version (VERSION)`
- **newer available** — `A newer version of ticket is available, upgrade using: go install github.com/simonski/ticket@latest`
- **local is newer** — `Your local copy is newer than the repo`
