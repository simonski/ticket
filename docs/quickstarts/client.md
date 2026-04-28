# Quickstart: Local Mode

Local mode runs everything on your machine with a SQLite database. No server required.

## 1. Initialise the shared local database

```bash
tk initdb
```

This creates the shared local database at `$TICKET_HOME/ticket.db`
(default `~/.ticket/ticket.db`) and bootstraps the local admin account.

If you want this repo to use its own local database instead of the shared one,
run `tk initdb .` to create `./.ticket/ticket.db`, register a named local
remote for that path, and bind this repo to it.

## 2. Bind this git repo to a project

```bash
# inside an existing repo (or after `git init`)
tk init
```

Choose **Local mode** when prompted. Ticket will use the repo name, git origin,
and derived prefix as defaults, and then write `.ticket/config.json` at the git
repo root.

## 3. Capture work

```bash
tk new  "Customers can reset their password"
tk bug  "Reset token expires immediately"
tk epic "Authentication"
```

Capture lightweight ideas before turning them into tickets:

```bash
tk idea new "Add dark mode"
tk idea ls              # list all ideas
tk idea get RE-4        # inspect the idea; replace RE with your project prefix
tk idea update -id RE-4 -title "Add dark mode support"
```

## 4. Inspect and organise

```bash
tk ls
tk get   -id RE-1
tk summary
tk attach -id RE-1 RE-3
tk dep add -id RE-2 RE-1
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design -> develop -> test -> done**.

```bash
tk active   -id RE-1
tk success  -id RE-1
```

## 6. Log time and add comments

```bash
tk time log -id RE-1 -m 90 -note "Initial implementation"
tk time ls  -id RE-1
```

## 7. Labels and decisions

```bash
tk label create backend
tk label add -id RE-1 1

tk decision add "Use JWT for auth"
tk decision list
```

## 8. Terminal UI

```bash
tk -g
```

Full-screen terminal UI. Navigate with Tab / arrow keys.  
Tabs: **Home · Projects · Ideas · Tickets · SDLCs · Config**

## 9. Health check

```bash
tk doctor project
tk doctor ticket -id RE-1
```

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the global Ticket home directory (default `~/.ticket`) |

---

Next: [Server mode quickstart](server.md) — multi-user, web UI, and AI agents.
