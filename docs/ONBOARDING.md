# Onboarding Guide

Welcome to the **ticket** project. This guide gets you from a fresh clone to
fully productive in under 30 minutes.

---

## Contents

1. [Start here](#start-here)
2. [Reading order](#reading-order)
3. [Prerequisites](#prerequisites)
4. [Clone and setup](#clone-and-setup)
5. [Daily development loop](#daily-development-loop)
6. [Ticket sdlc](#ticket-sdlc)
7. [Workflow expectations](#workflow-expectations)
8. [Running tests](#running-tests)
9. [Common pitfalls](#common-pitfalls)
10. [Getting help](#getting-help)

---

## Start here

If you only need the shortest path to productivity, use this order:

1. Read `README.md` for the product shape and local/server modes.
2. Run `make setup` and `make test` from a clean clone.
3. Read `CLAUDE.md` for the package map, architecture, and build/test commands.
4. Skim `CONTRIBUTING.md` before opening a branch or PR.

Then come back to this guide for the deeper reading order and day-to-day workflow.

---

## Reading order

Read these documents in order — each one builds on the last:

1. `README.md` — what the project is and why it exists
2. `CLAUDE.md` — architecture, package table, build commands, special commands
3. `QUICKSTART.md` — choose local or server mode, then follow the linked guide
4. `TESTING.md` — test strategy, how to run each suite, coverage thresholds
5. `docs/DESIGN.md` — deeper architecture, data model, design decisions
6. `docs/LIFECYCLE.md` — SDLC lifecycle, stages, roles, and stage-role assignments
7. `USER_GUIDE.md` — full CLI and web UI reference
8. `CONTRIBUTING.md` — branching, commits, PR process

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | https://go.dev/dl/ |
| Node.js | 18+ | https://nodejs.org/ |
| Git | any | https://git-scm.com/ |
| `gh` (optional) | any | https://cli.github.com/ |

On Linux, Playwright also needs system Chromium libraries:
```bash
sudo apt-get install -y libx11-dev libxcomposite-dev libxdamage-dev \
  libxext-dev libxfixes-dev libxrandr-dev libgbm-dev libpango1.0-dev \
  libasound2-dev libatk1.0-dev libcups2-dev
```

---

## Clone and setup

```bash
git clone https://github.com/simonski/ticket.git
cd ticket

make setup    # Downloads Go modules, Node deps, Playwright Chromium, dev tools
make test     # All tests should pass on a clean clone
```

The `make setup` command installs:
- Go module dependencies
- `govulncheck` — vulnerability scanner
- `cyclonedx-gomod` — SBOM generator
- `golangci-lint` — linter
- Node/npm packages
- Playwright Chromium browser

---

## Daily development loop

```bash
# Build for local use (does NOT bump version)
go build -o ./bin/tk ./cmd/tk

# Run unit + integration tests
make test-go

# Run a single test
go test ./internal/store/ -run TestTicketLifecycle

# Run with verbose output
go test ./internal/store/ -run TestTicketLifecycle -v

# Lint
make lint

# Run all tests (Go + Playwright)
make test
```

> **⚠️ Critical pitfall: `make build` increments the version**
>
> `make build` auto-increments the patch version in `cmd/tk/VERSION` on
> every run. This creates an unwanted commit and pollutes git history.
>
> **Always use `go build -o ./bin/tk ./cmd/tk` for development.**
> Only use `make build` when cutting a release.

---

## Ticket sdlc

The project tracks its own work using the `tk` CLI tool (an alias for the
`ticket` binary):

```bash
# See what's open
tk ls

# Pick up the next ticket
tk ls -status develop/idle -type task

# Mark a ticket active when you start
tk active -id TK-XXX

# Mark it complete when done
tk complete -id TK-XXX

# Create a new bug
tk bug -title "Fix login redirect loop"

# View a ticket with full history
tk get TK-XXX
```

See `docs/LIFECYCLE.md` for the lifecycle reference and `USER_GUIDE.md` for the
full CLI and web workflow reference.

### SDLC entities

The ticket system supports custom SDLC (Software Development Life Cycle)
definitions. An SDLC defines a sequence of stages that tickets move through.
Roles describe the responsibilities at each stage, and stage-role assignments
connect roles to specific stages within an SDLC.

```bash
# Create an SDLC definition
tk sdlc create -name "Agile" -d "Standard agile process"

# Add stages to the SDLC
tk sdlc add-stage -id 1 -name develop
tk sdlc add-stage -id 1 -name test

# Create a role
tk role create -title Engineer -d "Software engineer"

# Assign a role to a stage
tk sdlc stage-role-add -sdlc_id 1 -stage_id 1 -role_id 1

# List all roles
tk role ls
```

See `docs/LIFECYCLE.md` for the full SDLC reference.

---

## Workflow expectations

Use this as the short newcomer checklist before you open a PR:

1. **Create a focused branch** - use `feature/TK-123-short-name`, `fix/TK-123-short-name`, `docs/short-name`, or `chore/short-name`.
2. **Keep commits traceable** - prefer subjects like `TK-123: fix lifecycle reopen path`.
3. **Run the local gates before review** - for code changes, run `make lint` and `make test-go-cover`; run `make test` when browser behavior changed.
4. **Update the matching docs** - user-visible behavior belongs in `USER_GUIDE.md`; architecture or storage changes belong in `docs/DESIGN.md`; API/spec changes should update `SPEC.md` and `openapi.yaml` in the same change.
5. **Keep the PR narrow** - one concern per PR, rebase or squash before merge, and include the `TK-XXX` reference in the PR title or description.

`CONTRIBUTING.md` has the full version of these rules; this section is the quick start for new contributors.

---

## Running tests

```bash
make test-unit          # Fast unit tests (config, password, web)
make test-integration   # Integration tests (store, server, libticket, client)
make test-go-cover      # Go tests with enforced coverage thresholds
make test-playwright    # End-to-end browser tests (requires Chromium)
make test               # All of the above
```

The contract test suite (`libtickettest/contract.go`) runs the same 28
operations against both `LocalService` (SQLite) and `HTTPService` (HTTP
client). If you add a `Service` method, add a contract test.

Coverage thresholds are enforced per-package. A build that drops below
threshold will fail both locally (`make test-go-cover`) and in CI.

---

## Common pitfalls

| Pitfall | Fix |
|---------|-----|
| `make build` bumps the version unexpectedly | Use `go build -o ./bin/tk ./cmd/tk` for dev builds |
| Playwright tests fail because Chromium is missing | Run `make setup` once, or `make setup-playwright` if only the browser install is missing |
| `tk` command not found | Run `go build -o ./bin/tk ./cmd/tk` and add `./bin` to your PATH, or copy `./bin/tk` to a directory in your PATH |
| `tk` is talking to the wrong backend | Run `tk status` first. Local mode uses `.ticket/config.json` + `.ticket/ticket.db`; remote mode uses the `location` entry in `.ticket/config.json` |
| API or lifecycle behavior changed but the docs/spec now disagree | Update `SPEC.md`, `openapi.yaml`, and the relevant guide in the same PR; if you are using the repo SDLC commands, run the `spec` workflow |
| `.ticket/ticket.db` blocks a rebase or pull | The repo tracks `.ticket/ticket.db`. If you do not need your local ticket state, restore that file before rebasing; otherwise copy it aside first and restore it after the rebase |
| `make test` times out | Playwright tests require a local server; the Makefile starts one automatically, but if port 8080 is already in use the tests will hang — kill any running `ticket` server first |
| Local and remote mode behave differently | Read `tk status` before debugging. The same CLI can talk directly to SQLite or to a server depending on `.ticket/config.json` |
| Import cycle errors | The dependency flow must be `cmd -> libticket -> internal/store`. Nothing in `internal/` may import `cmd/` |
| Coverage threshold failure | Run `make test-go-cover` to see which package is below threshold; add tests or adjust the threshold in the Makefile with a comment explaining why |

---

## Getting help

- Read `docs/RUNBOOKS.md` for production operations, backup/restore, and
  common incident playbooks.
- Open a ticket: `tk bug -title "I'm stuck on..."` — even for onboarding
  friction. This helps us improve.
- Check `docs/DESIGN.md` for architecture decisions and rationale.
