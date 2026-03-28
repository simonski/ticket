# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test

```bash
make build                # Build binary to ./bin/ticket, increments patch version
make test                 # Run all tests (unit + integration + playwright)
make test-unit            # Unit tests only (config, password, web)
make test-integration     # Integration tests (cmd, client, server, store, libticket, libtickethttp)
make test-go-cover        # Tests with per-package coverage thresholds
go test ./internal/store/ -run TestTicketLifecycle  # Run a single test
```

Coverage thresholds enforced: cmd/ticket 55%, libticket 65%, libtickethttp 75%, internal/client 55%, internal/store 70%, internal/config 70%.

Docker: `make docker-build`, `make docker-up`, `make docker-down`.

## Architecture

Single Go binary (`cmd/ticket/main.go`) providing four interfaces to the same data:

1. **CLI** — 60+ commands routed via a switch statement in `run()`
2. **HTTP API** — REST endpoints under `/api/`, registered in `internal/server/api.go`
3. **Web UI** — Embedded SPA served from `web/static/`
4. **TUI** — BubbleTea terminal UI in `internal/tui/`

### Two Modes

- **Local mode** (default) — Direct SQLite via `internal/store`. No server needed.
- **Remote mode** (`TICKET_URL` set) — HTTP client via `internal/client` to a running server.

Mode is resolved by `internal/config.ResolveURL()`. The CLI, `libticket.LocalService`, and `libtickethttp.Service` all implement the same `libticket.Service` interface (108 methods).

### Key Packages

| Package | Role |
|---------|------|
| `cmd/ticket` | CLI entry point, all command handlers |
| `internal/server` | HTTP server, API handlers, WebSocket, chat |
| `internal/store` | SQLite schema, CRUD, lifecycle rules (20+ files) |
| `internal/client` | HTTP client for remote mode |
| `internal/config` | Config resolution (`$TICKET_HOME`, mode detection) |
| `internal/password` | Argon2id hashing |
| `internal/tui` | BubbleTea terminal UI |
| `libticket` | `Service` interface + `LocalService` implementation |
| `libtickethttp` | `Service` implementation wrapping HTTP client |
| `libtickettest` | Shared contract tests run against both implementations |

### Data Model

- **Projects** have prefixes (e.g. `CUS`). **Tickets** have human keys (e.g. `CUS-T-42`).
- Ticket types: epic, task, bug, spike, chore, story, note, question, requirement, decision.
- Lifecycle is `stage/state`: stages `design → develop → test → done`, states `idle | active | success | fail`.
- Setting `state=success` auto-advances to the next workflow stage.
- Parent tickets derive their lifecycle from descendants — only leaf tickets can be directly mutated.

### Test Strategy

Contract tests in `libtickettest/contract.go` define a `Factory` pattern and verify the `Service` interface. Both `libticket/local_test.go` and `libtickethttp/http_test.go` run the same contract suite. API endpoint tests are in `internal/server/api_test.go` using `testHandler()` + `doJSONRequest()` helpers.

## Development Rules

- Red/green testing. `make test` must always pass.
- Unit tests, integration tests, and Playwright tests required for all code.
- Keep documentation in sync: update DESIGN.md and USER_GUIDE.md when code changes.
- Externalise strings to `constants.go` where possible.
- The authoritative specification is `SPEC.md`; the OpenAPI spec is `openapi.yaml`.

## Workflow (from AGENTS.md)

1. File issues for remaining work
2. Run quality gates if code changed (`make test`)
3. Update issue status
4. **Push to remote** — this is mandatory. Work is NOT complete until `git push` succeeds.
5. Clean up and hand off context

## Special Commands

These words as user input trigger specific workflows defined in `docs/RULES.md`:

- `spec` — Rebuild SPEC.md and openapi.yaml from the codebase
- `sdlc` — SDLC review (versions, SAST, quality, documentation)
- `drift` — Check documentation vs implementation drift
- `next` — Pick up next ticket or continue current
- `review` — Read TODO/DESIGN/USER_GUIDE and propose next steps
- `continue` — Read TODO/DESIGN/USER_GUIDE and continue implementation
- `pr` — File a PR containing the ticket ID
