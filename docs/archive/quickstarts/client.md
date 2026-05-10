# Quickstart: CLI Client

The CLI now runs as a client against a running Ticket server.

## 1. Initialise and start a local server

```bash
tk initdb
tk server
```

This creates `$TICKET_HOME/ticket.db` (default `~/.ticket/ticket.db`) and starts
the HTTP API + web UI on `http://localhost:8080`.

## 2. Configure the CLI for that server

```bash
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=admin
export TICKET_PASSWORD=password
tk project use 1
tk whoami
```

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
tk idea get TK-4        # inspect the idea
tk idea update -id TK-4 -title "Add dark mode support"
```

## 4. Inspect and organise

```bash
tk ls
tk get   -id TK-1
tk summary
tk attach -id TK-1 TK-3
tk dep add -id TK-2 TK-1
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design -> develop -> test -> done**.

```bash
tk active   -id TK-1
tk success  -id TK-1
```

## 6. Log time and add comments

```bash
tk time log -id TK-1 -m 90 -note "Initial implementation"
tk time ls  -id TK-1
```

## 7. Labels and decisions

```bash
tk label create backend
tk label add -id TK-1 1

tk decision add "Use JWT for auth"
tk decision list
```

## 8. Terminal UI

```bash
tk -g
```

Full-screen terminal UI. Navigate with Tab / arrow keys.  
Tabs: **Home · Projects · Ideas · Tickets · Workflows · Config**

## 9. Health check

```bash
tk doctor project
tk doctor ticket -id TK-1
```

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_HOME` | Override the global Ticket home directory (default `~/.ticket`) |
| `TICKET_URL` | Base URL for the running Ticket server (defaults to `https://ticket.localhost` when unset) |
| `TICKET_USERNAME` | Username for API authentication |
| `TICKET_PASSWORD` | Password for API authentication |

---

Next: [Server quickstart](server.md) — multi-user, web UI, and AI agents.
