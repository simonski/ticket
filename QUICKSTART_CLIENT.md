# Quickstart: Local Mode

Local mode runs everything on your machine with a SQLite database. No server required.

## 1. Initialise a workspace

```bash
tk init
```

Creates `.ticket/ticket.db` in the current directory (or the nearest `.ticket/`
found by walking up the directory tree). Prints the generated `admin` password
on first run. Save it — you'll need it for admin operations.

## 2. Create a project

```bash
tk project create -prefix CUS -title "Customer Portal"
tk project use CUS
```

## 3. Capture work

```bash
tk add  "Customers can reset their password"
tk bug  "Reset token expires immediately"
tk epic "Authentication"
```

Capture lightweight ideas before turning them into tickets:

```bash
tk idea new "Add dark mode"
tk idea ls              # list all ideas
tk idea shape -id 1     # refine into a proper ticket
tk idea accept -id 1    # promote to ticket
```

## 4. Inspect and organise

```bash
tk list
tk get   -id CUS-T-1
tk summary                              # daily project overview
tk attach -id CUS-T-1 -parent CUS-E-3  # set parent epic
tk dep add -from CUS-T-2 -to CUS-T-1   # add dependency
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design → develop → test → done**.

```bash
tk active   -id CUS-T-1    # begin work  (sets state=active)
tk complete -id CUS-T-1    # finish stage, auto-advance to next
tk idle     -id CUS-T-1    # pause
tk close    -id CUS-T-1    # close ticket
```

## 6. Log time and add comments

```bash
tk time log -id CUS-T-1 -minutes 90 -note "Initial implementation"
tk time ls  -id CUS-T-1
```

## 7. Labels and decisions

```bash
tk label new "backend"
tk label add -id CUS-T-1 -label backend

tk decision new "Use JWT for auth"
tk decision ls
```

## 8. Terminal UI

```bash
tk -g
```

Full-screen terminal UI. Navigate with Tab / arrow keys.  
Tabs: **Home · Projects · Ideas · Tickets · SDLCs · Config**

## 9. Health check

```bash
tk doctor project   # review project health
tk doctor ticket    # review ticket health
```

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the config/database directory |

---

Next: [Server mode quickstart](QUICKSTART_SERVER.md) — multi-user, web UI, and AI agents.

