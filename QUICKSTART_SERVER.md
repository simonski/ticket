# Quickstart: Server Mode

Server mode runs an HTTP server with multi-user authentication, a web Kanban
board, WebSocket live updates, and AI agent support.

## 1. Initialise and start the server

```bash
tk init
tk server
```

The web UI is available at `http://localhost:8080`.  
Leave the server running and open a second terminal for the steps below.

## 2. Register and log in

Point the CLI at the running server and create your account:

```bash
export TICKET_URL=http://localhost:8080

tk register -username alice -password secret
tk login    -username alice -password secret
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

Tickets progress through stages: **design → develop → test → done**.

```bash
tk active   -id CUS-T-1      # begin work  (sets state=active)
tk complete -id CUS-T-1      # finish stage, auto-advance to next
tk idle     -id CUS-T-1      # pause
tk complete -id CUS-T-1      # mark ticket complete
```

## 6. Claim and request work

```bash
tk claim   -id CUS-T-2       # assign to yourself
tk request                    # get the next ready ticket
```

Admins can assign to other users with `tk assign <id> <username>`.

## 7. Run an AI agent

The agent picks up tickets marked `ready` and works on them autonomously.

```bash
tk undraft -id CUS-T-3         # make ticket available for agents

tk agent create               # prints agent_id (UUID) and password
export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=<generated-password>

tk agent run                  # default LLM: claude (Sonnet)
tk agent run -llm codex       # use codex
tk agent run -v               # stream LLM I/O to terminal
```

## 8. Web UI

Open `http://localhost:8080` in a browser:

- Kanban board grouped by sdlc stage
- Ticket creation and inline editing
- Drag-and-drop stage transitions
- Team and user management
- Live updates via WebSocket

## 9. Using with Claude Code

Copy the bundled skill so Claude can query and update tickets during coding:

```bash
cp -r .claude/skills/tk ~/.claude/skills/   # global (all projects)
# or
cp -r .claude/skills/tk .claude/skills/     # this project only
```

Claude will then read live ticket state, log time, create bugs, and record
decisions automatically during your sessions.

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

---

Previous: [Local mode quickstart](QUICKSTART_CLIENT.md) — singleplayer, no server required.

