# Tutorial

This tutorial is an executable walkthrough for a clean local setup and a minimal end-to-end flow.

## 1. Bootstrap local data

```bash
tk initdb --force
```

## 2. Start server mode

```bash
tk server
```

## 3. Configure CLI against the server

```bash
export TICKET_URL=http://localhost:8080
export TICKET_USERNAME=alice
export TICKET_PASSWORD=secret12
tk register -username alice -email alice@example.com -password secret12
tk whoami
```

## 4. Create a project and seed work

```bash
tk project create -prefix DEMO -title "Tutorial Project"
export TICKET_PROJECT=DEMO
tk epic "Release tutorial flow"
tk add "Implement core command"
tk bug "Fix onboarding typo"
tk ls
```

## 5. Work items and lifecycle

```bash
tk summary
tk complete -id DEMO-1
tk ls
```
