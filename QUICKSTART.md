# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Installs the binary as `tk`.

Or, download a binary for your platform from the [releases page](https://github.com/simonski/ticket/releases).

---

## Start server mode

`tk` runs as a client/server system.

### Server setup

Run an HTTP server with multi-user auth, a web Kanban board, WebSocket live
updates, and AI agent support. Best for teams, shared backlogs, and CI/CD.

```bash
# create the database once (server-side)
tk initdb --force

# now run the server
tk server
```

### Configure client access

```bash
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=admin
export TICKET_PASSWORD=password
export TICKET_PROJECT=1
tk ls

tk register -username alice -email alice@example.com -password secret12
tk project use 1
tk whoami
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
TASK_ID=$(tk add -printid "Fix login timeout")   # create a task
tk bug "Token expires too early"      # create a bug
tk epic "Authentication"              # create an epic

tk complete -id "$TASK_ID"            # mark the created task complete
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

Hands-on end-to-end workflow: [TUTORIAL.md](./TUTORIAL.md).

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
| `TICKET_URL` | Base URL for the running Ticket server (defaults to `https://ticket.localhost` when unset) |
| `TICKET_USERNAME` | Username for API authentication |
| `TICKET_PASSWORD` | Password for API authentication |
| `TICKET_TIMEOUT` | Remote HTTP timeout in seconds for CLI API calls (default `5`, clamped to `1..30`) |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

Set `TICKET_URL`, `TICKET_USERNAME`, and `TICKET_PASSWORD` to connect the CLI to a server.
