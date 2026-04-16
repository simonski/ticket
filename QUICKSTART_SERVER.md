# Quickstart: Server Mode

Server mode runs an HTTP server with multi-user authentication, a web Kanban
board, WebSocket live updates, and AI agent support.

## 1. Initialise and start the server

```bash
tk init
tk server
```

In the first terminal, choose **Local mode** during `tk init` so the server has
its local SQLite database. Save the generated `admin` password.

The web UI is available at `http://localhost:8080`. Leave the server running
and open a second terminal for the steps below.

To try the fresh replacement frontend without removing the original one, start
the server with:

```bash
tk server -site site2
```

## 2. Register and log in

Point the CLI at the running server and create your account:

```bash
export TICKET_URL=http://localhost:8080

tk register -username alice -password secret12
tk login    -username alice -password secret12
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
| `TICKET_URL` | Override the effective location: bare paths and `file:///...` are local, `http(s)://...` are remote |
| `TICKET_USERNAME` | Default username for login/register |
| `TICKET_PASSWORD` | Default password for login/register |
| `AGENT_ID` | Agent UUID for `tk agent run` |
| `AGENT_PASSWORD` | Agent password for `tk agent run` |
| `TICKET_AGENT_LLM` | Override default LLM command (default: `claude`) |

---

Previous: [Local mode quickstart](QUICKSTART_CLIENT.md) — single-user, no server required.
