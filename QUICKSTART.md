# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

## 1. Initialise a local workspace

```bash
tk init
```

Creates a SQLite database at `<TICKET_HOME>/ticket.db` (defaults to `.ticket/ticket.db`
in the current directory, or the nearest `.ticket/` directory found by walking up the tree).

Prints the generated `admin` password on first run.

## 2. Start the server (optional)

```bash
tk server
```

The web UI is available at `http://localhost:8080`. The CLI works against the
same database whether the server is running or not.

## 3. Create a project

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
```

## 4. Capture work

```bash
tk add "Customers can reset their password"
tk bug "Reset token expires immediately"
tk epic "Authentication"
```

Or capture lightweight ideas first:

```bash
tk idea new "Add dark mode"
tk idea ls          # list all ideas
```

## 5. Inspect and organise

```bash
tk list
tk get -id CUS-T-1
tk attach -id CUS-T-1 CUS-E-1   # set parent epic
```

## 6. Move work through the lifecycle

```bash
tk active -id CUS-T-1       # start work (sets state=active)
tk complete -id CUS-T-1     # finish stage, auto-advance
tk idle -id CUS-T-1         # pause
```

## 7. Assign and claim

```bash
tk assign CUS-T-1 alice
tk claim -id CUS-T-1        # assign to yourself
tk request                  # get the next available ticket
```

## 8. Run an agent (optional)

Create an agent and run it against a server:

```bash
tk agent create
# prints agent_id (UUID) and password

export TICKET_URL=http://localhost:8080
export AGENT_ID=<agent-uuid>
export AGENT_PASSWORD=<generated-password>
tk agent run                  # default LLM: claude (Sonnet 4.5)
tk agent run -llm codex       # use codex instead
tk agent run -v               # stream LLM I/O to terminal
```

Only tickets marked as `ready` are eligible for automatic assignment. Use
`ticket ready <id>` to make a ticket available to agents.

## 9. Use the TUI

```bash
tk -g
```

Launches the full-screen terminal UI. Navigate panels with Tab / arrow keys.
Tabs: **Home** · **Projects** · **Ideas** · **Tickets** · **Workflows** · **Config**.

---

## Using with Claude Code

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
