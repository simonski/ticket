# Quickstart

## Install

```bash
brew install simonski/tap/ticket
```

Or download a binary from the [releases page](https://github.com/simonski/ticket/releases).

## Choose your mode

`ticket` works in two modes:

### [Local mode](QUICKSTART_CLIENT.md)

Everything runs on your machine with a SQLite database. No server needed.
Best for solo use, small projects, or getting started quickly.

```bash
tk init
tk project create -prefix MY -title "My Project"
tk add "First ticket"
tk list
```

### [Server mode](QUICKSTART_SERVER.md)

Run an HTTP server with multi-user authentication, a web UI, and AI agent support.
Best for teams, shared backlogs, and CI/CD integration.

```bash
tk init
tk server                              # start on :8080
export TICKET_URL=http://localhost:8080
tk register -username alice -password secret
tk login -username alice -password secret
```

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
