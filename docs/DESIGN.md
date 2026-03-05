# Design

The authoritative lifecycle remodel specification is
`docs/TICKET_LIFECYCLE_SPEC.md`. Where this file still describes the older
single-status model, the lifecycle spec takes precedence until the rest of this
document is rewritten.

## Product Summary

`ticket` is a lightweight ticket and project management system delivered as a single Go binary.

It is designed for small teams that want low-friction ticket tracking without separate infrastructure for the API, database, and web UI. The product combines a server, a terminal-first CLI, and an embedded web application around one shared data model.

The system has three interfaces:

1. A server that owns persistence, authentication, and collaboration.
2. A CLI for fast, explicit terminal workflows.
3. An embedded web application for browsing, editing, and status management.

The repository also contains a static `VERSION` file. `make build` increments the patch version before compiling the binary and copies that value into the embedded build asset used by `ticket version`.

Client-side files are stored under `$TICKET_HOME`, which defaults to `~/.config/ticket`.

## Product Principles

1. The server defines the single system of record and the shared data model used by both remote and local workflows.
2. The CLI and web app use the same API semantics and data model.
3. Common operations should be fast and predictable from the terminal.
4. Projects should support lightweight hierarchy through epics and child tickets.
5. Every meaningful change should be traceable through history and comments.

## Primary Users And Workflows

The primary user is a small software team managing projects, epics, tasks, bugs.

The first release must support these workflows end to end:

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

- `user_id`
- `username`
- `password_hash`
- `role`
- `display_name`
- `enabled`
- `created_at`

Roles in the first release:

- `admin`
- `user`

Notes:

- administrators can create, enable, and disable users
- regular users can log in and manage project work according to API permissions

### Project

- `project_id`
- `title`
- `description`
- `created_at`
- `created_by`
- `status`

Projects are the top-level container for work items.

### Ticket

`Ticket` is the main work artifact. All item types share one core model.

- `ticket_id`
- `key`
- `project_id`
- `parent_id`
- `type`
- `title`
- `description`
- `acceptance_criteria`
- `stage`
- `state`
- `priority`
- `estimate_effort`
- `estimate_complete`
- `assignee`
- `comments`
- `created_at`
- `created_by`
- `updated_at`
- `archived`

Supported `type` values in the first release:

- `epic`
- `task`
- `bug`
- `spike`
- `chore`

Model notes:

- `parent_id` is nullable and supports hierarchical work
- tickets are orphaned when `parent_id` is null
- ticket creation accepts either a positional title or `-title`
- `acceptance_criteria` is captured directly on the task record
- `estimate_effort` is an integer assessment of task effort
- `estimate_complete` is the estimated delivery datetime and should use RFC3339 format
- `comments` are exposed on task detail reads as an array of `{author, date, text}` ordered most recent first

CLI creation defaults:

- `ticket add`, `ticket create`, and `ticket new` are the same command
- `ticket list` and `ticket ls` are the same command
- if `-type` / `-t` is omitted, the type defaults to `task`
- if `-priority` / `-p` is omitted, the priority defaults to `1`
- if `-assignee` / `-a` is omitted, the assignee is blank
- if `-description` / `-d` is omitted, the description is blank
- if `-ac` is omitted, the acceptance criteria is blank
- if `-estimate_effort` is omitted, it defaults to `0`
- if `-estimate_complete` is omitted, it is blank
- if `-parent` is omitted, the ticket is created without a parent
- if `-project` is omitted, the active project is used

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

The bootstrap command is `ticket initdb`.

`ticket initdb` must:

1. create the schema in a new SQLite database
2. create an `admin` account
3. create a default project

Representative flow:

```bash
ticket initdb -f ticket.db --force -password secret
```

Bootstrap defaults:

- admin username is always `admin`
- if `-f` is omitted, the SQLite database is created at `$TICKET_HOME/ticket.db`
- admin password comes from `-password` when supplied
- if `-password` is omitted, the CLI generates a random password and prints it to stdout
- if `--force` is supplied, any existing SQLite database file is overwritten
- the default project is created automatically during initialization with prefix `TK`

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

If `ticket server` is run without `-f`, it must open the SQLite database at `$TICKET_HOME/ticket.db`.

If `ticket server` is run with `-v`, it must print verbose request and response details to stdout.

### Authentication And User Management

The first release must support:

1. administrator bootstrap during initialization
2. user creation by administrators
3. user listing by administrators
4. user deletion by administrators
5. enable and disable user accounts
6. login and logout from CLI and web
7. user/session status inspection from the CLI

Representative commands:

```bash
ticket onboard
ticket version
ticket user create --username alice --password secret
ticket user ls
ticket user delete --username alice
ticket user enable --username alice
ticket user disable --username alice

ticket register
ticket login
ticket status
ticket logout
```

`ticket onboard` must append the embedded `cmd/ticket/AGENTS.md` template into `${CWD}/AGENTS.md`, creating that file if it does not exist.

`ticket status` must always print the current effective configuration first, then perform a mode-appropriate connectivity check.

In REMOTE mode it must print at least:

- `mode: remote`
- `server: <TICKET_SERVER>`
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

If the database does not exist in LOCAL mode, `ticket status` must also print:

- `hint: run ticket initdb`

If `-nocolor` is set, the same output must be printed without ANSI colors.

`ticket count` must query the server and print aggregate counts for users and work item types. Without a project filter it must also print the project count. With `-project_id <id>` it must scope work item counts to that project.

The CLI must resolve credentials from `-username` and `-password` first, then `TICKET_USERNAME` and `TICKET_PASSWORD`, and finally default to OS `whoami` and `password`.

The CLI must resolve the server URL from `-url` first, then `TICKET_SERVER`, then saved config, and finally default to `http://localhost:8080`.

The CLI must expose `ticket version`, which prints the semantic version embedded into the binary at build time.

`ticket initdb` is separate from the login and registration flows: it only creates `admin`, does not consume `TICKET_USERNAME`, and does not read `TICKET_PASSWORD`.

Admin-only user-management requests must be rejected by the server when the caller is authenticated but not an admin. Those requests must return HTTP 403 with an error explaining that the user is not an admin.

When `ticket` is run without arguments, the CLI should print a colored ASCII-art `TICKET` banner above the main usage text.

When `ticket server` starts, it should print the same colored ASCII-art `TICKET` banner before the startup message.

Below that banner, `ticket server` must print the embedded version and the resolved task database path.

The CLI stores non-sensitive client defaults in `$TICKET_HOME/config.json` and session credentials in `$TICKET_HOME/credentials.json`.

`ticket login` must:

1. check `$TICKET_HOME/credentials.json` first and reuse that session if it is still valid
2. check the `username` in `$TICKET_HOME/config.json`
3. check `-username` and `-password`, then `TICKET_USERNAME` and `TICKET_PASSWORD`
4. prompt for any missing values
5. when prompting, use the discovered values as editable defaults
6. print `invalid credentials` on an invalid-login response before prompting for a retry
7. when prompting for a password in an interactive terminal, echo `*` characters instead of the raw password
8. on success, write the session token to `$TICKET_HOME/credentials.json`
9. on success, update the `username` and `server_url` keys in `$TICKET_HOME/config.json`

`ticket register` must create the account but must not create or persist a logged-in session.

`ticket logout` must remove `$TICKET_HOME/credentials.json`.

### Project Management

Users must be able to:

1. create projects
2. list projects
3. inspect project details
4. select an active project for CLI defaults

Representative commands:

```bash
ticket project create -prefix CUS -description "Portal backlog" -ac "Launch criteria" "Customer Portal"
ticket project list
ticket project ls
ticket project use CUS
ticket project get CUS
ticket project
ticket project CUS update -title "Customer Portal"
ticket project CUS update -description "Portal backlog"
ticket project CUS update -ac "Launch criteria"
ticket project CUS enable
ticket project CUS disable
```

`ticket project list` should show at least the project id, prefix, title, and status, and indicate which project is current in the local CLI context.

All `ticket <command> create` commands must return to STDOUT the newly created ID, if they succeed.

The selected project should be remembered locally by the CLI.

### Work Item Capture

Creating work should be low-friction.

Users must be able to create tasks, bugs, and epics.

Representative commands:

```bash
ticket add "Customers can reset their password."
ticket create "Customers can reset their password."
ticket new "Customers can reset their password."
ticket bug "Reset token fails after first use."
ticket epic "Authentication"
ticket create -t task -p 1 -a alice -d "Add audit event" "Add password reset audit event"
```

Behavior notes:

- `ticket add`, `ticket create`, and `ticket new` are aliases
- `ticket list` and `ticket ls` are aliases
- `ticket list -n <limit>` applies a server-side limit, with `0` meaning no limit
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
ticket list
ticket ls
ticket list --type bug
ticket list --status develop/idle
ticket search "password reset"
ticket search "password reset" -allprojects
ticket get CUS-T-42
ticket orphans
```

`ticket search` should search the active project by default. If `-allprojects` is supplied, it should search across all projects.

The CLI should support `-json` on client-facing commands and pretty-print the response JSON.

`ticket get <key-or-id>` should print a flat detail view with the fields `ID`, `Type`, `Description`, `ParentID`, `CloneOf` when present, `ProjectID`, `Title`, `Assignee`, `Order`, `EstimateEffort`, `EstimateComplete`, `DependsOn`, `Status`, `Priority`, `Created`, `LastModified`, `Acceptance Criteria`, and a `Comments` section ordered most recent first.

`ticket list` should render a readable table that includes at least the id, type, status, assignee, priority, and title.

### Workflow And Lifecycle Management

The system should support ticket progression through explicit stage/state
changes.

The lifecycle model is:

- stages: `design`, `develop`, `test`, `done`
- states: `idle`, `active`, `complete`
- rendered status: `<stage>/<state>`

The CLI and web app must both support easy lifecycle changes.

Assignment workflows must support:

- `ticket assign <key-or-id> <name>` for admins
- `ticket unassign <key-or-id> <name>` for admins
- `ticket dependency add <key-or-id> <dependency-id[,dependency-id...]>`
- `ticket dependency remove <key-or-id> <dependency-id[,dependency-id...]>`
- `ticket request [<key-or-id>]` for the caller
- `ticket claim` or `ticket claim -id <key-or-id>` for the caller
- `ticket claim -dry-run` for preview without mutation
- `ticket unclaim <key-or-id>` for the caller
- `ticket attach <key-or-id> <parent-key-or-id>`
- `ticket detach <key-or-id>`
- `ticket rm <key-or-id>`
- `ticket delete <key-or-id>`
- `ticket list -u <name>` / `ticket ls -u <name>` for assignee filtering
- `ticket design <key-or-id>`
- `ticket develop <key-or-id>`
- `ticket test <key-or-id>`
- `ticket done <key-or-id>`
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
ticket project create -prefix CUS "Customer Portal"
ticket project use CUS

ticket epic "Authentication"
ticket add "Customers can reset their password."
ticket bug "Reset token expires immediately."
ticket list
ticket get CUS-T-42
ticket search "password reset"
ticket history CUS-T-42
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

The web UI should make these activities easy:

- switch between projects
- add and edit items
- view hierarchy
- manage status on a board
- inspect history and comments

## Persistence And Architecture

### Storage

- SQLite is the only database in the first release.
- SQLite remains the persistence layer behind the server data model; local mode uses the same data model and validation rules as the server-backed flow.

Suggested storage areas:

1. users
2. sessions
3. projects
4. tasks
5. ticket_history
6. comments

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
- heavyweight enterprise workflow configuration
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

The lifecycle of a ticket is:
    stage: design | develop | test | done
    state: idle | active | complete
    status: <stage>/<state>

This is set using
    `ticket design N`
    `ticket develop N`
    `ticket test N`
    `ticket done N`
    `ticket idle N`
    `ticket active N`
    `ticket complete N`
or
    `ticket update N -status <stage/state>`
    `ticket update N -title <title>`
    `ticket update N -description <description>`
    `ticket update N -ac <acceptance-criteria>`
    `ticket update N -priority <priority>`
    `ticket update N -order <order>`
    `ticket update N -parent_id <parent-id>`
    `ticket update N -estimate_effort <effort>`
    `ticket update N -estimate_complete <rfc3339-datetime>`

## Requesting Tickets

A user can makes a request to work on a specific task

    `ticket request N`

It is either assigned the task it requested, or it is rejected. If assigned, the task is updated to have this user name and the response is `{"status":"ASSIGNED","task":...}`. If not, the response is `{"status":"REJECTED"}`.

Or a user may request ANY task

    ticket request

It is either assigned a task, or no work is available. If assigned, the task is updated to have this user name and the response is `{"status":"ASSIGNED","task":...}`. If not, the response is `{"status":"NO-WORK"}`.

If the user has already been assigned a `develop/active` ticket, that ticket is returned. If the user has been assigned a `develop/idle` ticket, then the oldest assigned `develop/idle` ticket is returned.

    
