# Quickstart: Local Mode

Local mode runs everything on your machine with a SQLite database. No server required.

## 1. Initialise a workspace

```bash
tk init
```

Choose **Local mode** when prompted. If the suggested project prefix contains
characters other than `A-Z`, enter a short uppercase prefix such as `TK`.

This creates `.ticket/ticket.db` in the current directory (or the nearest
`.ticket/` found by walking up the tree). Save the generated `admin` password.

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
tk idea shape -id CUS-4 # refine the new requirement
tk idea accept requirement CUS-4
```

## 4. Inspect and organise

```bash
tk list
tk get   -id CUS-1
tk summary
tk attach -id CUS-1 CUS-3
tk dep add -id CUS-2 CUS-1
```

## 5. Move work through the lifecycle

Tickets progress through stages: **design -> develop -> test -> done**.

```bash
tk active   -id CUS-1
tk success  -id CUS-1
```

## 6. Log time and add comments

```bash
tk time log -id CUS-1 -m 90 -note "Initial implementation"
tk time ls  -id CUS-1
```

## 7. Labels and decisions

```bash
tk label create backend
tk label add -id CUS-1 1

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
tk doctor ticket -id CUS-1
```

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `TICKET_URL` | Override the effective location: bare paths and `file:///...` are local, `http(s)://...` are remote |

---

Next: [Server mode quickstart](QUICKSTART_SERVER.md) — multi-user, web UI, and AI agents.
