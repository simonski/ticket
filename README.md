# ticket

`ticket` is a ticket and project management system for software engineering work.

It models:

- projects with unique prefixes such as `CUS`
- tickets with human keys such as `CUS-42`
- ticket types `epic`, `task`, `bug`, `story`, `requirement`, `decision`, `question`, and `note`
- lifecycle as `stage/state`, for example `develop/active`
- stages: `design → develop → test → done`
- states: `idle | active | success | fail`
  - `idle`: ready but not currently in progress
  - `active`: currently being worked on (requires an assignee)
  - `success`: stage complete, auto-advances to next stage
  - `fail`: stage did not succeed

The authoritative system contract is in [SPEC.md](./SPEC.md). User-facing
workflow details are in [USER_GUIDE.md](./USER_GUIDE.md). Implementation and
architecture notes are in [docs/DESIGN.md](./docs/DESIGN.md).

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
        SVC["libticket.Service<br/>107-method interface"]
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
    CMD["cmd/ticket<br/>CLI entry point"] --> CLIENT["internal/client"]
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
    Project }o--o| Workflow : uses
    Ticket ||--o{ Ticket : "parent-child"
    Ticket ||--o{ Dependency : blocks
    Ticket ||--o{ Label : tagged
    Ticket ||--o{ TimeEntry : tracks
    Ticket ||--o{ Comment : has
    Ticket ||--o{ HistoryEvent : logs
    Team ||--o{ User : members
    Team ||--o{ Agent : agents
    Workflow ||--o{ WorkflowStage : stages
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
    ADMIN --> PROJECTS["Manage projects<br/>create, set workflow"]
    ADMIN --> ROLES["Manage roles"]
    ADMIN --> WF["Define workflows<br/>stages, ordering"]
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

## Install

```bash
brew install simonski/tap/ticket
```

Both `ticket` and the alias `tk` are installed.

or

```bash
go install github.com/simonski/ticket/cmd/ticket@latest
alias tk=ticket
```

## Build from source

```bash
cd $CODE
git clone github.com/simonski/ticket
cd ticket
make install
```

## Test

```bash
make test
```

## Usage

In your project, run

```bash
tk init
```

You can now create tickets

```bash
tk add "Create a skeleton project in go."
```

```bash
claude -p "work on next ticket"
```


## Web Server

Start the server and web UI:

```bash
tk server
```

The web UI is then available at `http://localhost:8080`.

## CLI Quick Start

Create a project:

```bash
ticket project create -prefix CUS -title "Customer Portal"
ticket project use CUS
```

Create tickets:

```bash
ticket epic "Authentication"
ticket add "Customers can reset their password."
ticket bug "Reset token expires immediately."
```

Inspect and move work:

```bash
ticket list
ticket get -id CUS-T-42
ticket active -id CUS-T-42
ticket complete -id CUS-T-42
ticket claim -id CUS-T-42
```

## Running an agent

Create an agent (requires a running server):

```bash
tk agent create
```

This prints the agent UUID and a generated password.

Run the agent worker:

```bash
export AGENT_ID=<uuid>
export AGENT_PASSWORD=<generated-password>
export TICKET_URL=http://localhost:8080
tk agent run
```

or with flags:

```bash
tk agent run -id <uuid> -url http://localhost:8080
```

The password is read from the `AGENT_PASSWORD` environment variable, or prompted interactively (input masked with `*`).

Options: `-llm claude` (default, uses Sonnet 4.5), `-llm codex`, or `-llm /path/to/binary`.
Use `-v` to stream LLM input/output to the terminal.

## Claude Code integration

`ticket` ships a Claude Code skill in `.claude/skills/tk/`. Copy it into your
project's `.claude/skills/` directory (or `~/.claude/skills/` globally) and Claude
will query and update tickets during coding sessions automatically.

See [QUICKSTART.md](./QUICKSTART.md#using-with-claude-code) for setup details.

## Notes

- The CLI and web app use the same HTTP API.
- Ticket IDs are human-readable keys such as `CUS-T-42`.
- `tk ls` hides closed and archived tickets by default; use `-a` to include closed, `-d` to also include archived.
- The HTTP API exposes resource families under `/api/` including tickets, projects,
  users, agents, teams, roles, workflows, and more.
