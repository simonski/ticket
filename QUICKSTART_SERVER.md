# Quickstart: Server Mode

Server mode runs an HTTP server with multi-user authentication, a web UI, and agent support.

## 1. Initialise and start the server

```bash
tk init
tk server
```

The web UI is available at `http://localhost:8080`. The CLI works against the
same database whether the server is running or not.

## 2. Register and login

In another terminal, point the CLI at the running server:

```bash
export TICKET_URL=http://localhost:8080

tk register -username alice -password secret
tk login -username alice -password secret
```

## 3. Create a project

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
```

## 4. Capture and organise work

```bash
tk add "Customers can reset their password"
tk bug "Reset token expires immediately"
tk epic "Authentication"
tk list
```

## 5. Claim and request

```bash
tk claim -id CUS-T-1        # assign to yourself
tk request                   # get the next available ticket
```

Admins can also assign tickets to other users with `tk assign <id> <username>`.

## 6. Run an agent

Create an agent and run it against the server:

```bash
tk agent create
# prints agent_id (UUID) and password

export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=<generated-password>
tk agent run                  # default LLM: claude (Sonnet 4.5)
tk agent run -llm codex       # use codex instead
tk agent run -v               # stream LLM I/O to terminal
```

Only tickets marked as `ready` are eligible for automatic assignment. Use
`tk ready <id>` to make a ticket available to agents.

## 7. Web UI

Open `http://localhost:8080` in a browser. The web UI provides:

- Kanban board grouped by workflow stage
- Ticket creation and editing
- Drag-and-drop stage changes
- Workflow management
- Team and user management

## 8. Using with Claude Code

`ticket` ships a Claude Code skill that lets Claude work with your backlog directly
during coding sessions. To enable it:

1. Copy `.claude/skills/tk/` into your project's `.claude/skills/` directory (or the
   global `~/.claude/skills/` directory for all projects).
2. Claude will automatically read the skill when you mention tickets or use `/tk`.

Once active, Claude can:
- Query and update ticket state (`tk list`, `tk show`, `tk state`)
- Log time against tickets (`tk time log`)
- Create tickets for bugs or ideas discovered during work
- Record architectural decisions

The skill ensures Claude reads live ticket state on every action rather than relying
on conversation memory.

---

## Environment variables

| Variable             | Purpose                                              |
|----------------------|------------------------------------------------------|
| `TICKET_HOME`        | Override the config/database directory               |
| `TICKET_URL`         | Connect to a remote server (`http(s)://host:port`)   |
| `TICKET_USERNAME`    | Default username for login/register                  |
| `TICKET_PASSWORD`    | Default password for login/register                  |
| `AGENT_ID`           | Agent UUID for `tk agent run`                        |
| `AGENT_PASSWORD`     | Agent password for `tk agent run`                    |
| `TICKET_AGENT_LLM`  | Override default LLM command (default: `claude`)     |

When `TICKET_URL` is set the CLI communicates with a running `ticket server`
rather than opening the local database directly.

---

Previous: [Local mode quickstart](QUICKSTART_CLIENT.md) for singleplayer use.
