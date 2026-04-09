# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Installs both `ticket` and the alias `tk`.

Or download a binary for your platform from the [releases page](https://github.com/simonski/ticket/releases).

---

## Choose your mode

`ticket` works in two modes:

### [Local mode](QUICKSTART_CLIENT.md)

Everything runs on your machine using a SQLite file. No server needed.  
Best for solo use, small projects, or getting started quickly.

```bash
tk init
tk project create -prefix MY -title "My Project"
tk project use MY
tk add "First ticket"
tk list
```

### [Server mode](QUICKSTART_SERVER.md)

Run an HTTP server with multi-user auth, a web Kanban board, WebSocket live
updates, and AI agent support. Best for teams, shared backlogs, and CI/CD.

```bash
tk init
tk server                              # start on :8080 (leave this running)
# in another terminal:
export TICKET_URL=http://localhost:8080
tk register -username alice -password secret
tk login    -username alice -password secret
```

---

## Key concepts

| Concept | Example |
|---------|---------|
| Project | `CUS` — Customer Portal |
| Ticket key | `CUS-T-42` |
| Ticket types | `task`, `bug`, `epic`, `story`, `spike`, `chore`, `note`, `question`, `requirement`, `decision` |
| Lifecycle | `stage/state` e.g. `develop/active` |
| Stages | `design → develop → test → done` |
| States | `idle`, `active`, `success`, `fail` |

Setting state to `success` auto-advances to the next stage.

---

## Daily sdlc

```bash
tk summary                            # daily overview
tk ls                                 # list open tickets
tk add "Fix login timeout"            # create a task
tk bug "Token expires too early"      # create a bug
tk epic "Authentication"              # create an epic

tk active   -id CUS-T-1              # begin work
tk complete -id CUS-T-1              # finish stage, auto-advance
tk close    -id CUS-T-1              # close ticket

tk claim    -id CUS-T-1              # assign to yourself
tk request                            # get the next available ticket
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

Copy the bundled skill into your project (or globally):

```bash
cp -r .claude/skills/tk ~/.claude/skills/   # global
# or
cp -r .claude/skills/tk .claude/skills/     # project-only
```

Claude will then query and update tickets automatically during coding sessions:
reading live ticket state, logging time, creating bugs, and recording decisions.

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the config/database directory |
| `TICKET_URL` | Connect to a remote server (`http(s)://host:port`) |
| `TICKET_USERNAME` | Default username for login/register |
| `TICKET_PASSWORD` | Default password for login/register |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

When `TICKET_URL` is set the CLI communicates with a running `tk server`
rather than opening the local database directly.

