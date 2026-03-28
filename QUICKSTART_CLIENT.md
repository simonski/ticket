# Quickstart: Local Mode

Local mode runs everything on your machine with a SQLite database. No server required.

## 1. Initialise a workspace

```bash
tk init
```

Creates a SQLite database at `<TICKET_HOME>/ticket.db` (defaults to `.ticket/ticket.db`
in the current directory, or the nearest `.ticket/` directory found by walking up the tree).

Prints the generated `admin` password on first run.

## 2. Create a project

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
```

## 3. Capture work

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

## 4. Inspect and organise

```bash
tk list
tk get -id CUS-T-1
tk attach -id CUS-T-1 CUS-E-1   # set parent epic
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design** -> **develop** -> **test** -> **done**.

```bash
tk active -id CUS-T-1       # start work (sets state=active)
tk complete -id CUS-T-1     # finish stage, auto-advance to next
tk idle -id CUS-T-1         # pause
```

## 6. Use the TUI

```bash
tk -g
```

Launches the full-screen terminal UI. Navigate panels with Tab / arrow keys.
Tabs: **Home** . **Projects** . **Ideas** . **Tickets** . **Workflows** . **Config**.

---

## Environment variables

| Variable        | Purpose                                |
|-----------------|----------------------------------------|
| `TICKET_HOME`   | Override the config/database directory |

---

Next: [Server mode quickstart](QUICKSTART_SERVER.md) for multi-user, web UI, and agents.
