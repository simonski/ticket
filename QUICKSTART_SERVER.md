# Quickstart: Server Mode

Server mode runs an HTTP server with multi-user authentication, a web Kanban
board, WebSocket live updates, and AI agent support.

## 1. Initialise and start the server

```bash
tk initdb
tk server
```

In the first terminal, `tk initdb` creates the shared local database at
`$TICKET_HOME/ticket.db` and bootstraps `admin` / `password`.

The web UI is available at `http://localhost:8080`. Leave the server running
and open a second terminal for the steps below.

### Docker deployment

The repository also includes a container entrypoint and compose file that run
Ticket as a persistent server backed by a bind-mounted `./data` directory:

```bash
docker compose -f deploy/compose.yaml up -d
docker compose -f deploy/compose.yaml logs -f
tk docker-compose > compose.yaml
```

On first boot the container:

1. creates `/data/ticket.db` if it does not exist
2. bootstraps `admin` / `password` unless `TICKET_ADMIN_PASSWORD` is already set
3. prints `admin password: ...` to stdout once
4. starts `tk server -f /data/ticket.db -addr 0.0.0.0:8080`

The SQLite database lives in `./data/ticket.db`, so it survives container
restarts and image upgrades.

If you want the compose YAML generated directly from the Ticket binary, run
`tk docker-compose`.

To try the fresh replacement frontend without removing the original one, start
the server with:

```bash
tk server -site site2
```

## 2. Configure the CLI for the running server

Register a named remote for the server, bind this repo to it, and use the
bootstrap admin credentials:

```bash
tk remote add local-server http://localhost:8080
tk login -username admin -password password
tk project remote local-server
tk whoami
```

## 3. Create a project

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
```

## 4. Capture and organise work

```bash
tk add  "Customers can reset their password"
tk bug  "Reset token expires immediately"
tk epic "Authentication"
tk list
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design -> develop -> test -> done**.

```bash
tk complete -id CUS-1
```

## 6. Claim and request work

```bash
tk request
```

Use `tk claim -id <ticket>` when you want a specific claimable ticket.
Admins can assign to other users with `tk assign <id> <username>`.

## 7. Run an AI agent

The agent picks up tickets marked `ready` and works on them autonomously.

```bash
tk ready -id CUS-3

tk agent create
export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=<generated-password>

tk agent run
tk agent run -llm codex
tk agent run -v
```

## 8. Web UI

Open `http://localhost:8080` in a browser:

- Kanban board grouped by SDLC stage
- Ticket creation and inline editing
- Drag-and-drop stage transitions
- Team and user management
- Live updates via WebSocket

## 9. Using with Claude Code

Write the bundled skill so Claude can query and update tickets during coding:

```bash
mkdir -p .claude/skills/tk
tk skill > .claude/skills/tk/SKILL.md
```

Claude will then read live ticket state, log time, create bugs, and record
decisions automatically during your sessions.

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the global Ticket home directory (default `~/.ticket`) |
| `TICKET_TIMEOUT` | Remote HTTP timeout in seconds for CLI API calls (default `5`, clamped to `1..30`) |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

---

Previous: [Local mode quickstart](QUICKSTART_CLIENT.md) — single-user, no server required.
