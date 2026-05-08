# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Installs the binary as `tk`.

Or, download a binary for your platform from the [releases page](https://github.com/simonski/ticket/releases).

---

## Choose your mode

`tk` has two primary operating modes:

### [Local mode](docs/quickstarts/client.md)

In local mode, the client talks to SQLite directly. No server needed.
`tk initdb` creates the shared local database at `$TICKET_HOME/ticket.db`
(default `~/.ticket/ticket.db`), registers the default `local` remote, and
`tk init` requires a git repo and binds the current repo by writing
`.ticket/config.json`.
Best for solo use, small projects, or getting started quickly.

```bash
# inside an existing repo (or after `git init`)
tk initdb

# bind this repo to a project and choose Local mode when prompted
tk init
tk new "First ticket"
tk ls
```

### [Server mode](docs/quickstarts/server.md)

Run an HTTP server with multi-user auth, a web Kanban board, WebSocket live
updates, and AI agent support. Best for teams, shared backlogs, and CI/CD.

```bash
# create the shared local database once
tk initdb

# now run the server
tk server
```

### Access a remote server

```bash
tk remote add local-server http://localhost:8080
tk project remote local-server
tk register -username alice -password secret12
tk login -username alice -password secret12
```

---

## Key concepts

| Concept | Example |
|---------|---------|
| Project | `CUS` — Customer Portal |
| Ticket key | `CUS-42` |
| Ticket types | `task`, `bug`, `epic`, `story`, `spike`, `chore`, `note`, `question`, `requirement`, `decision` |
| Lifecycle | `stage/state` e.g. `develop/active` |
| Stages | `design → develop → test → done` |
| States | `idle`, `active`, `success`, `fail` |

Setting state to `success` auto-advances to the next stage.

---

## Daily workflow

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
tk summary                            # daily overview
tk ls                                 # list open tickets
tk add "Fix login timeout"            # create a task
tk bug "Token expires too early"      # create a bug
tk epic "Authentication"              # create an epic

tk complete -id CUS-1               # mark ticket complete
```

---

## Terminal UI

```bash
tk -g
```

Full-screen terminal UI. Navigate with Tab / arrow keys.  
Tabs: **Home · Projects · Ideas · Tickets · Workflows · Config**

---

## Reproducible example scenario (todo app planning)

Use the dedicated tutorial to seed a realistic project with Workflow, epics, tasks,
labels, dependencies, time entries, and decision/idea artifacts:

```bash
./scripts/populate_todo_example.sh
```

Full walkthrough: [docs/quickstarts/todo-example.md](./docs/quickstarts/todo-example.md).

Validation command (recommended before relying on the tutorial in CI/docs updates):

```bash
make build-dev && ./tests/quickstart_test.sh && ./scripts/verify_todo_example.sh
```

---

## Using with Claude Code

Write the bundled skill into your project:

```bash
mkdir -p .claude/skills/tk
tk skill > .claude/skills/tk/SKILL.md
```

Claude will then query and update tickets automatically during coding sessions:
reading live ticket state, logging time, creating bugs, and recording decisions.

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the global Ticket home directory (default `~/.ticket`) |
| `TICKET_TIMEOUT` | Remote HTTP timeout in seconds for CLI API calls (default `5`, clamped to `1..30`) |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

Use `tk remote add NAME URL` plus `tk project remote NAME` to select a server.
Remote credentials are stored per canonical URL in `$TICKET_HOME/credentials.json`.
