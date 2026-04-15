# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Installs both `ticket` and the alias `tk`.

Or download a binary for your platform from the [releases page](https://github.com/simonski/ticket/releases).

---

## Choose your mode

`tk` works in two modes - *local* and *remote*

### [Local mode](QUICKSTART_CLIENT.md)

Everything runs on your machine using a SQLite file. No server needed.  No env vars, almost no setup.
Best for solo use, small projects, or getting started quickly.

```bash
# initialise a ticket database and config file under ${CWD}/.ticket
# choose Local mode when prompted
tk init
tk add "First ticket"
tk list
```

### [Server mode](QUICKSTART_SERVER.md)

Run an HTTP server with multi-user auth, a web Kanban board, WebSocket live
updates, and AI agent support. Best for teams, shared backlogs, and CI/CD.

```bash
# initialise a ticket database and config file under ${CWD}/.ticket
# choose Local mode when prompted so the server has a local database
tk init

# run a server
tk server
```

### Access the server in another terminal and register a user

```bash
export TICKET_URL=http://localhost:8080

### Register a user
tk register -username alice -password secret12
```

### Open a new terminal and interact as alice

```bash
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=alice
export TICKET_PASSWORD=secret12
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

## Daily sdlc

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
Tabs: **Home · Projects · Ideas · Tickets · SDLCs · Config**

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
| `TICKET_URL` | Override the effective location: bare paths and `file:///...` are local, `http(s)://...` is remote |
| `TICKET_USERNAME` | Default username for login/register |
| `TICKET_PASSWORD` | Default password for login/register |
| `TICKET_TIMEOUT` | Remote HTTP timeout in seconds for CLI API calls (default `5`, clamped to `1..30`) |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

When `TICKET_URL` is set, it overrides the configured `location`. Bare paths
and `file:///...` keep the CLI in local mode; `http(s)://...` switches to
client/server mode.

When `TICKET_URL`, `TICKET_USERNAME`, and `TICKET_PASSWORD` are all set, those
values take precedence over local `.ticket/config.json` and credentials.

In that env-trio mode, client commands do not require `tk init`.
`tk login` is optional in that mode because remote calls auto-authenticate.
