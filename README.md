# ticket

`ticket` is a ticket and project management system for software engineering work.

It models:

- projects with unique prefixes such as `CUS`
- tickets with human keys such as `CUS-42`
- ticket types `epic`, `task`, `bug`, `spike`, and `chore`
- lifecycle as `stage/state`, for example `develop/active`
- stages: `design → develop → test → done`
- states: `idle | active | success | fail`
  - `idle`: ready but not currently in progress
  - `active`: currently being worked on (requires an assignee)
  - `success`: stage complete, auto-advances to next stage
  - `fail`: stage did not succeed

The authoritative system contract is in [SPEC.md](./SPEC.md). User-facing
workflow details are in [USER_GUIDE.md](./USER_GUIDE.md). Implementation and
architecture notes are in [docs/DESIGN.md](./docs/DESIGN.md).

## Install

```bash
brew install simonski/tap/ticket
```

Or in one step (no prior tap needed):

```bash
brew install simonski/tap/ticket
```

Both `ticket` and `tk` commands are installed.

## Build from source

```bash
make build
make tools
```

`make build` writes the CLI binary to `./bin/ticket` and updates a `./tk` symlink for shorter invocation.

## Test

```bash
make test
make test-unit
make test-integration
make test-playwright
```

`make test` runs the unit suite, integration suite, and Playwright frontend
smoke test.

## Run

```bash
ticket init
ticket server
```

The web UI is then available at `http://localhost:8080`.

## CLI Quick Start

Create a project:

```bash
ticket project create -prefix CUS -title "Customer Portal"
ticket project use CUS
```

Create tickets:

```bash
ticket epic "Authentication"
ticket add "Customers can reset their password."
ticket bug "Reset token expires immediately."
```

Inspect and move work:

```bash
ticket list
ticket get CUS-T-42
ticket active -id CUS-T-42
ticket complete -id CUS-T-42
ticket claim
```

## Claude Code integration

`ticket` ships a Claude Code skill in `.claude/skills/tk/`. Copy it into your
project's `.claude/skills/` directory (or `~/.claude/skills/` globally) and Claude
will query and update tickets during coding sessions automatically.

See [QUICKSTART.md](./QUICKSTART.md#using-with-claude-code) for setup details.

## Notes

- The CLI and web app use the same HTTP API.
- Ticket refs accept human keys such as `CUS-T-42` and internal numeric ids
  where supported, but keys are preferred.
- The supported HTTP resource family is `/api/tickets`.
