# User Guide

`ticket` is a ticket management tool.

This guide describes a single Go binary that provides a server, a CLI, and an embedded web application backed by SQLite.

## How `ticket` Works

`ticket` has three interfaces:

1. The server, which owns persistence, authentication, and collaboration.
2. The CLI, which provides fast and explicit terminal workflows.
3. The web app, which is embedded in the same binary and uses the same API.

All project data follows the server data model and API semantics, whether you are working against a remote server or a local workspace.

Client-side files live under `$TICKET_HOME`, which defaults to `~/.config/ticket`.

- `$TICKET_HOME/config.json` stores non-sensitive client defaults such as the current username, server URL, and active project
- `$TICKET_HOME/credentials.json` stores the current session token

## Getting Started

Write the local agent instructions template into the current repository:

```bash
ticket onboard
```

`ticket onboard` appends the embedded onboarding template into `${CWD}/AGENTS.md`. If the file does not exist yet, it is created.

Initialize a task sqlite database:

```bash
ticket initdb
```

If `-f` is omitted, `ticket initdb` creates the SQLite database at `$TICKET_HOME/ticket.db`.

`ticket initdb` creates:

1. an `admin` account
2. the default project, `Default Project`, with project id `1` and prefix `TK`

Bootstrap resolution works like this:

- admin username: always `admin`
- admin password: `-password` if provided, otherwise a generated random password printed to stdout
- existing database file: overwritten only when `--force` is supplied

Start the server:

```bash
ticket server
```

If `-f` is omitted, `ticket server` uses `$TICKET_HOME/ticket.db`.

If `-v` is supplied, `ticket server` prints verbose request and response logs to stdout.

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

If you are using the CLI against a running server on another host, configure TICKET_SERVER first:

```bash
export TICKET_SERVER=http://your-server:8080
```

As an admin create users:

```bash
ticket user create --username XXXX --password YYYY
created user xxxxx
```

As an admin enable/disable users:

```bash
ticket user enable --username XXXX
ticket user disable --username XXXX
ticket user ls|list
ticket user rm|delete --username XXXX
```

These commands are admin-only. If a logged-in non-admin user runs them, the server returns `403` and the CLI prints `user is not an admin`.

## Accounts And Login

Create an account:

```bash
ticket register --username name --password '*******'
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

In REMOTE mode it prints:

- `mode: remote`
- `server: <TICKET_SERVER>`
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

- `hint: run ticket initdb`

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
ticket project create -prefix CUS -description "Portal backlog" -ac "Portal launch criteria" "Customer Portal"
```

The project is now the default project.

List projects:

```bash
ticket project list
ticket project ls
```

`ticket project list` prints the project id, prefix, title, and status, and marks the active project as `(current)`.

Select the active project for subsequent commands:

```bash
ticket project use CUS
```

Show the current project:

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

List all items in the active project:

```bash
ticket list
ticket ls
ticket list -n 20
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

`ticket list` prints a table with the ticket key, type, rendered `status` (`stage/state`), assignee, priority, and title.

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

`ticket get` accepts either a ticket key or an internal numeric id. It prints the ticket fields directly, including `DependsOn`, the acceptance criteria, `EstimateEffort`, `EstimateComplete`, `CloneOf` when the ticket is a clone, and comments ordered most recent first.

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

`ticket claim` is server-mediated. If the current user already has an active claimed ticket, that ticket is returned. Otherwise the server assigns the highest-priority oldest eligible `develop/idle` leaf ticket in the active project. `ticket claim -dry-run` shows the candidate without changing server state. `ticket unclaim` is retained as a compatibility alias for clearing your own assignment.

`ticket rm` and `ticket delete` remove a ticket permanently. They fail if the ticket still has child tickets.

`ticket request` is the lower-level form of `ticket claim`. It accepts either a ticket key or an internal numeric id. If no work can be assigned, the JSON response status is `NO-WORK`. If a specific ticket is requested and cannot be assigned, the JSON response status is `REJECTED`.

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

## Web Interface

The embedded web app is the easiest way to work visually across many related items.

Use it for:

1. capturing work during discovery and delivery
2. reviewing related items side by side
3. browsing task details and dependencies without switching commands

Because the CLI and web app use the same server API, edits made in one interface appear in the other without any import or sync step.

## Command Reference

```bash
ticket initdb
ticket server -v
ticket version

ticket register --username <name> --password <password>
ticket login --username <name> --password <password>
ticket status
ticket logout

ticket user create --username <name> --password <password>
ticket user ls
ticket user delete --username <name>
ticket user enable --username <name>
ticket user disable --username <name>

ticket project create -prefix ABC "..."
ticket project list
ticket project ls
ticket project use <prefix-or-id>
ticket project
ticket project get <prefix-or-id>
ticket project <prefix-or-id>
ticket project <prefix-or-id> update -title "..."
ticket project <prefix-or-id> update -description "..."
ticket project <prefix-or-id> update -ac "..."
ticket project <prefix-or-id> enable
ticket project <prefix-or-id> disable

ticket add "..."
ticket bug "..."
ticket epic "..."

ticket list
ticket ls
ticket list --type task
ticket list --status develop/idle
ticket list -u <name>
ticket search "..."
ticket search "..." -allprojects
ticket get <key-or-id>
ticket history <key-or-id>
ticket health <key-or-id>
ticket comment add <key-or-id> "..."
ticket orphans

ticket dependency add <key-or-id> <key-or-id[,key-or-id...]>
ticket dependency remove <key-or-id> <key-or-id[,key-or-id...]>
ticket assign <key-or-id> <name>
ticket unassign <key-or-id> <name>
ticket claim <key-or-id>
ticket unclaim <key-or-id>
ticket request [<key-or-id>]
ticket attach <key-or-id> <parent-key-or-id>
ticket detach <key-or-id>
ticket rm <key-or-id>
ticket delete <key-or-id>
ticket design <key-or-id>
ticket develop <key-or-id>
ticket test <key-or-id>
ticket done <key-or-id>
ticket idle <key-or-id>
ticket active <key-or-id>
ticket complete <key-or-id>
ticket update <key-or-id> -stage <stage> -state <state>
ticket count
ticket count -project_id <prefix-or-id>

ticket update <key-or-id> -stage develop -state idle
ticket update <key-or-id> -title "new title"
ticket update <key-or-id> -description "new description"
ticket update <key-or-id> -ac "new acceptance criteria"
ticket update <key-or-id> -priority 4
ticket update <key-or-id> -order 7
ticket update <key-or-id> -parent_id 12
ticket update <key-or-id> -estimate_effort 5
ticket update <key-or-id> -estimate_complete 2026-04-30T17:00:00Z
ticket update <key-or-id> -stage develop -state active -priority 2 -title "new title"

```
