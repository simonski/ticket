# ticket

`ticket` is an issue tracking toolkit for software engineering.  It is a single Go binary that provides a CLI, a terminal UI, a web UI, and a REST API — all backed by SQLite.

```
brew install simonski/tap/ticket
```

---

## Introduction

`ticket` tracks engineering work through a lightweight lifecycle:

| Concept | Example |
|---------|---------|
| Project | `CUS` — Customer Portal |
| Ticket key | `CUS-T-42` |
| Ticket types | `epic`, `task`, `bug`, `story`, `spike`, `chore`, `note`, `question`, `requirement`, `decision` |
| Lifecycle | `stage/state` — e.g. `develop/active` |
| Stages | `design → develop → test → done` |
| States | `idle`, `active`, `success`, `fail` |

Setting a ticket's state to `success` automatically advances it to the next stage.

It works in two modes:

- **Local** — CLI and TUI operate directly on a SQLite file. No server required.
- **Server** — HTTP server adds multi-user auth, a web Kanban board, WebSocket live updates, and AI agent support.

The authoritative system contract is in [SPEC.md](./SPEC.md). Full user-facing documentation is in [USER_GUIDE.md](./USER_GUIDE.md). Architecture and design notes are in [docs/DESIGN.md](./docs/DESIGN.md).

## Start here

If you're new to the repo, read these first:

1. [QUICKSTART.md](./QUICKSTART.md) - choose local or server mode
2. [docs/ONBOARDING.md](./docs/ONBOARDING.md) - setup, reading order, and common pitfalls
3. [CLAUDE.md](./CLAUDE.md) - build/test commands, architecture, and package map
4. [CONTRIBUTING.md](./CONTRIBUTING.md) - branch naming, commit style, and PR expectations

## Installation

### Homebrew (macOS / Linux)

```bash
brew install simonski/tap/ticket
```

Installs as `tk`.

### Go install

```bash
go install github.com/simonski/ticket/cmd/tk@latest
```

### Download a binary

Download a tarball for your platform from the [releases page](https://github.com/simonski/ticket/releases), extract it, and put `ticket` on your `PATH`.

---

## Build from source

```bash
git clone https://github.com/simonski/ticket
cd ticket
make setup        # install Go tools, Node, Playwright
make build
```

New to the codebase? See [docs/ONBOARDING.md](docs/ONBOARDING.md) for the guided setup, reading order, workflow expectations, and newcomer gotchas.

Run the tests:

```bash
make test
```

---

## Usage

### Quick start (local)

```bash
tk init                                          # create a repo-local .ticket/ workspace
tk add "First ticket"
tk list
```

See [QUICKSTART.md](./QUICKSTART.md) for a full walkthrough.

### Command structure

```
tk <noun> <verb> [flags]
```

| Noun | Common verbs |
|------|-------------|
| `ticket` | `ls`, `new`, `get`, `update`, `rm`, `state`, `assign`, `close` |
| `idea` | `ls`, `new`, `get`, `shape`, `accept`, `reject` |
| `project` | `ls`, `new`, `get`, `use`, `rm`, `init` |
| `dep` | `add`, `remove` |
| `label` | `ls`, `new`, `rm`, `add`, `remove` |
| `time` | `log`, `ls`, `total`, `rm` |
| `story` | `ls`, `new`, `get`, `update`, `rm` |
| `decision` | `ls`, `new` |
| `role` | `ls`, `new`, `get`, `update`, `rm` |
| `sdlc` | `ls`, `new`, `get`, `rm`, `set`, `unset` |
| `team` | `ls`, `new`, `update`, `rm` |
| `agent` | `ls`, `new`, `update`, `rm`, `run` |
| `user` | `ls`, `new`, `rm`, `enable`, `disable` |

**Shortcuts:**

```bash
tk add "Fix login bug"                    # create a task
tk bug "Token expires too early"          # create a bug
tk epic "Authentication"                  # create an epic
tk idea new "Add dark mode"               # capture a requirement
tk ls                                     # list open tickets
tk summary                                # daily starting-point overview
```

### Ticket lifecycle

```bash
tk active -id MY-T-1      # begin work  (develop/active)
tk complete -id MY-T-1    # finish stage, auto-advance
tk idle -id MY-T-1        # pause
tk complete -id MY-T-1    # mark ticket complete
```

### TUI

```bash
tk -g
```

Launches a full-screen terminal UI. Navigate with Tab / arrow keys.  
Tabs: **Home · Projects · Ideas · Tickets · SDLCs · Config**

### Web server

```bash
tk server                  # start on :8080
```

Opens a Kanban board with live WebSocket updates at `http://localhost:8080`.

See [QUICKSTART_SERVER.md](./QUICKSTART_SERVER.md) for multi-user server setup.

---

## AI agent support

`ticket` can run an AI coding agent that picks up ready tickets and works on them autonomously.

```bash
tk agent create                        # prints agent UUID and password
export AGENT_ID=<uuid>
export AGENT_PASSWORD=<password>
export TICKET_URL=http://localhost:8080
tk agent run                           # default LLM: claude (Sonnet)
tk agent run -llm codex                # use codex
tk agent run -v                        # stream LLM I/O to terminal
```

Custom `-llm` binaries are only allowed when explicitly added to
`TICKET_AGENT_ALLOWED_LLM_BINARIES` (comma-separated names).

Only non-draft tickets are eligible. Use `tk undraft -id <id>` to make a ticket available.

### Claude Code skill

A Claude Code skill ships in `.claude/skills/tk/`. Copy it into your project's
`.claude/skills/` directory (or `~/.claude/skills/` for all projects) and Claude
will query and update tickets automatically during coding sessions.

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the config/database directory. Otherwise `tk` uses `.ticket/` at the nearest Git root, or `.ticket/` in the current directory if no Git root exists |
| `TICKET_URL` | Connect to a remote server (`http(s)://host:port`) |
| `TICKET_USERNAME` | Default username for remote login |
| `TICKET_PASSWORD` | Default password for remote login |
| `TICKET_TRUSTED_PROXY_CIDRS` | Comma-separated CIDRs trusted as reverse proxies for `X-Forwarded-For`/`X-Forwarded-Proto` handling |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override the LLM command (default: `claude`) |
| `TICKET_AGENT_ALLOWED_LLM_BINARIES` | Additional allow-listed binary names for `tk agent run -llm` (default allow-list: `claude`, `codex`) |

`TICKET_CHAT_CMD` and `TICKET_ANALYSE_CMD` execute server-side processes. Treat
them as trusted operator-only configuration and never source their values from
untrusted input.

---

## Architecture

A single Go binary provides four interfaces to the same data:

```mermaid
graph TB
    subgraph Interfaces
        CLI["CLI<br/>70+ commands"]
        TUI["TUI<br/>BubbleTea terminal UI"]
        WEB["Web UI<br/>Embedded SPA"]
        API["HTTP API<br/>REST + WebSocket"]
    end

    subgraph Core
        SVC["libticket.Service<br/>119-method interface"]
        LOCAL["LocalService<br/>Direct DB access"]
        REMOTE["HTTP Client<br/>Remote access"]
    end

    subgraph Storage
        DB[(SQLite<br/>ticket.db)]
    end

    CLI --> LOCAL
    CLI --> REMOTE
    TUI --> REMOTE
    WEB -->|"/api/*"| API
    API --> LOCAL
    REMOTE -->|"HTTP"| API
    LOCAL --> DB
```

### Package Dependencies

```mermaid
graph LR
    CMD["cmd/tk<br/>CLI entry point"] --> CLIENT["internal/client"]
    CMD --> CONFIG["internal/config"]
    CMD --> STORE["internal/store"]
    CMD --> LIB["libticket"]

    SERVER["internal/server<br/>HTTP API"] --> STORE
    SERVER --> STATIC["web/static<br/>Embedded SPA"]

    TUI_PKG["internal/tui"] --> CLIENT
    TUI_PKG --> CONFIG
    TUI_PKG --> LIB

    CLIENT --> STORE
    CLIENT --> CONFIG
    CLIENT --> LIB

    LIBHTTP["libtickethttp"] --> LIB
    LIB --> STORE

    LIBTEST["libtickettest<br/>Contract tests"] --> LIB
```

### Data Model

```mermaid
erDiagram
    Project ||--o{ Ticket : contains
    Project ||--o{ ProjectMember : has
    Project }o--o| SDLC : uses
    Ticket ||--o{ Ticket : "parent-child"
    Ticket ||--o{ Dependency : blocks
    Ticket ||--o{ Label : tagged
    Ticket ||--o{ TimeEntry : tracks
    Ticket ||--o{ Comment : has
    Ticket ||--o{ HistoryEvent : logs
    Team ||--o{ User : members
    Team ||--o{ Agent : agents
    SDLC ||--o{ SdlcStage : stages
    User ||--o{ Ticket : assigned
    Agent ||--o{ Ticket : works
```

### Ticket Lifecycle

```mermaid
stateDiagram-v2
    direction LR

    state "design" as D {
        [*] --> d_idle
        d_idle --> d_active
        d_active --> d_success
        d_active --> d_fail
        d_fail --> d_active
    }

    state "develop" as DEV {
        [*] --> dev_idle
        dev_idle --> dev_active
        dev_active --> dev_success
        dev_active --> dev_fail
        dev_fail --> dev_active
    }

    state "test" as T {
        [*] --> t_idle
        t_idle --> t_active
        t_active --> t_success
        t_active --> t_fail
        t_fail --> t_active
    }

    state "done" as DONE {
        [*] --> done_idle
        done_idle --> done_success
    }

    D --> DEV : success auto-advances
    DEV --> T : success auto-advances
    T --> DONE : success auto-advances
```

## Use Cases

### Developer

```mermaid
graph LR
    DEV((Developer))
    DEV --> CREATE["Create tickets<br/>add, bug, epic, story"]
    DEV --> TRACK["Track work<br/>active, complete, claim"]
    DEV --> VIEW["View progress<br/>list, board, summary"]
    DEV --> RELATE["Manage relations<br/>set-parent, add-dependency"]
    DEV --> TIME["Log time"]
    DEV --> COMMENT["Add comments"]
    DEV --> SEARCH["Search tickets"]
```

### Agent

```mermaid
graph LR
    AGENT((Agent))
    AGENT --> REG["Register & authenticate"]
    AGENT --> HB["Send heartbeat"]
    AGENT --> REQ["Request work"]
    AGENT --> UPD["Update ticket state"]
    AGENT --> CFG["Read configuration"]
```

### Admin

```mermaid
graph LR
    ADMIN((Admin))
    ADMIN --> USERS["Manage users<br/>create, enable, disable"]
    ADMIN --> AGENTS["Manage agents<br/>create, configure"]
    ADMIN --> TEAMS["Manage teams<br/>create, add members"]
    ADMIN --> PROJECTS["Manage projects<br/>create, set sdlc"]
    ADMIN --> ROLES["Manage roles"]
    ADMIN --> WF["Define sdlcs<br/>stages, ordering"]
    ADMIN --> SYS["System config<br/>registration, features"]
```

### Web User

```mermaid
graph LR
    USER((Web User))
    USER --> LOGIN["Register & login"]
    USER --> BOARD["View Kanban board"]
    USER --> EDIT["Edit tickets inline"]
    USER --> LIVE["Live updates<br/>via WebSocket"]
    USER --> CHAT["Chat / comments"]
```

### Deployment Modes

```mermaid
graph TB
    subgraph "Local Mode (default)"
        CLI_L["CLI / TUI"] -->|direct| DB_L[(SQLite)]
    end

    subgraph "Remote Mode (TICKET_URL set)"
        CLI_R["CLI / TUI"] -->|HTTP| SRV["Server"]
        SPA["Web UI"] -->|HTTP + WS| SRV
        AGENT_R["Agents"] -->|HTTP| SRV
        SRV --> DB_R[(SQLite)]
    end
```
