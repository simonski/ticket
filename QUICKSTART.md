# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Or build from source:

```bash
git clone https://github.com/simonski/ticket
cd ticket
make build
```

## 1. Initialise a local workspace

```bash
ticket init
```

Creates a SQLite database at `<TICKET_HOME>/ticket.db` (defaults to `.ticket/ticket.db`
in the current directory, or the nearest `.ticket/` directory found by walking up the tree).

Prints the generated `admin` password on first run.

## 2. Start the server (optional)

```bash
ticket server
```

The web UI is available at `http://localhost:8080`. The CLI works against the
same database whether the server is running or not.

## 3. Create a project

```bash
ticket project create -prefix CUS -title "Customer Portal"
ticket project use CUS
```

## 4. Capture work

```bash
ticket add "Customers can reset their password"
ticket bug "Reset token expires immediately"
ticket epic "Authentication"
```

Or capture lightweight ideas first:

```bash
ticket idea "Add dark mode"
ticket ideas          # list all ideas
```

## 5. Inspect and organise

```bash
ticket list
ticket get CUS-T-1
ticket attach CUS-T-1 CUS-E-1   # set parent epic
```

## 6. Move work through the lifecycle

```bash
ticket active -id CUS-T-1       # start work (sets state=active)
ticket complete -id CUS-T-1     # finish stage, auto-advance
ticket idle -id CUS-T-1         # pause
```

## 7. Assign and claim

```bash
ticket assign CUS-T-1 alice
ticket claim CUS-T-1            # assign to yourself
ticket request                  # get the next available ticket
```

## 8. Use the TUI

```bash
tk -g
```

Launches the full-screen terminal UI. Navigate panels with Tab / arrow keys.
Tabs: **Home** · **Projects** · **Ideas** · **Epics** · **Config**.

---

## Environment variables

| Variable      | Purpose                                              |
|---------------|------------------------------------------------------|
| `TICKET_HOME` | Override the config/database directory               |
| `TICKET_URL`  | Connect to a remote server (`http(s)://host:port`)   |

When `TICKET_URL` is set the CLI communicates with a running `ticket server`
rather than opening the local database directly.
