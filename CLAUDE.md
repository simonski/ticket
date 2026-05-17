# CLAUDE.md

Design
- red/green testing - on all work

Compile
- `make test` on every run (ultra-fast default: unit tests).
- `make test-fast` for the normal developer loop (unit + JS API + Go API smoke).
- `make test-all` before completion/PR - full suite must pass.
- `make lint` - on every turn - no lint failures.

## Build and Test

```bash
make setup                # Install all dev dependencies (Go modules + Node + Playwright)
make build                # Build binary to ./bin/tk and increment the patch version
make build-dev            # Build binary to ./bin/tk without changing the version
make test                 # Ultra-fast default: unit tests only
make test-fast            # Recommended developer loop: unit + JS API + Go API smoke
make test-api-smoke       # Fast Go API smoke packages (internal/client + internal/server)
make test-cli             # Heavier CLI package tests
make test-contract        # Heavier libticket contract tests
make test-api-js          # JavaScript API client-library tests (web/site2/api.test.js)
make test-api-cli         # CLI/API interface tests (cmd + client + server + contract)
make test-api             # API interface suites: test-api-js + test-api-cli
make test-browser         # Fast browser smoke Playwright suite
make test-browser-full    # Full browser end-to-end Playwright suite
make test-quickstart      # Executable QUICKSTART/TUTORIAL docs tests
make test-all             # Full suite: unit + api + browser + docs/harness
make test-go              # Run all Go tests (unit + integration)
make test-unit            # Unit tests only (config, password, web)
make test-integration     # Integration tests (cmd, internal/client, server, store, libticket)
make test-go-cover        # Tests with per-package coverage thresholds
make ci-bootstrap         # Install deps for the same verify/browser flow used by GitHub Actions
make ci-verify            # Validate OpenAPI + coverage + JS API + build-dev + lint + vulncheck
make ci-browser           # Full Playwright browser job used by GitHub Actions
make ci                   # ci-verify + ci-browser
make lint                 # Run golangci-lint on all packages
make dev                  # Print env vars for local development mode
```

> `make build` increments the patch version in `cmd/tk/VERSION` on every
> invocation. Use `make build-dev` when you want a development build without
> changing the version.

Run a single test: `go test ./internal/store/ -run TestTicketLifecycle`

Coverage thresholds enforced: cmd/tk 55%, libticket 65%, internal/client 55%, internal/store 69%, internal/server 63%, internal/config 70%.

Docker: `make docker-build`, `make docker-up`, `make docker-down`.

Playwright browser tests are in `tests/playwright/` (12 spec files). Run with `make test-playwright`.

### Staged test policy

- Default inner loop: `make test` + `make test-fast` + targeted package tests.
- If API contract/surface changes (`openapi.yaml`, `internal/server`, `internal/client`, `cmd/tk` handlers), run `make test-api`.
- If web/site2 UX changes, run `make test-browser` while iterating, then `make test-browser-full`.
- Before finishing a feature or opening a PR, run `make test-all` and `make lint`.

## Architecture

Single Go binary (`cmd/tk/main.go`) providing four interfaces to the same data:

1. **CLI** — 60+ commands routed via a switch statement in `run()`
2. **HTTP API** — REST endpoints under `/api/`, registered in `internal/server/api.go`
3. **Web UI** — Embedded SPA served from `web/static/`
4. **TUI** — BubbleTea terminal UI in `internal/tui/`

### Runtime model

- **Server** — SQLite-backed HTTP API and web UI.
- **Client** — CLI/TUI connect via `internal/client` to a configured server remote.

Routing is determined from the environment plus repo-local project binding: `TICKET_URL` selects the server, repo-local `.ticket/config.json` stores the active `project_id`, and `~/.ticket/credentials.json` stores reusable remote session credentials.

`$TICKET_HOME` controls the data directory. If unset, the CLI walks up from `cwd` looking for a `.git` directory, then uses `.ticket/` as a sibling. `~/.ticket/preferences.json` stores TUI preferences, `~/.ticket/credentials.json` stores per-remote credentials keyed by canonical URL, and `-f /path` is a per-command local database override.

### Key Packages

| Package | Role |
|---------|------|
| `cmd/tk` | CLI entry point, all command handlers |
| `internal/server` | HTTP server, API handlers, WebSocket, chat |
| `internal/store` | SQLite schema, CRUD, lifecycle rules (20+ files) |
| `internal/client` | HTTP client for server access |
| `internal/config` | Config resolution (`$TICKET_HOME`, mode detection) |
| `internal/password` | Argon2id hashing |
| `internal/tui` | BubbleTea terminal UI |
| `libticket` | `Service` interface, local/remote implementations, and shared contract tests |
| `internal/client` | HTTP client used by the remote-mode service implementation |

### Data Model

- **Projects** have prefixes (e.g. `CUS`). **Tickets** have human keys (e.g. `CUS-42`).
- Ticket types: epic, task, bug, spike, chore, story, note, question, requirement, decision.
- **Workflows** define configurable lifecycle processes attached to projects. An Workflow has ordered **stages** (e.g. `design → develop → test → done`) and **roles** (e.g. architect, engineer, QA). Roles are assigned to stages via a stage-role junction table with ordering. Workflows can be exported/imported as JSON.
- Ticket lifecycle fields: `stage` (from Workflow), `role` (current role within stage), `state` (`idle | active | success | fail`), `draft` (bool), `complete` (bool), `archived` (bool).
- `tk next` advances a ticket to the next role or stage (on success); `tk previous` moves it back (on fail).
- Parent tickets derive their lifecycle from descendants — only leaf tickets can be directly mutated.
- The authoritative lifecycle specification is `docs/LIFECYCLE.md`.

### Test Strategy

Contract tests in `libticket/contract_test.go` define a `Factory` pattern and verify the `Service` interface. Both `libticket/local_test.go` and `libticket/http_test.go` run the same contract suite. API endpoint tests are in `internal/server/api_test.go` using `testHandler()` + `doJSONRequest()` helpers.

## Development Rules

- Red/green testing. `make test` must always pass.
- Unit tests, integration tests, and Playwright tests required for all code.
- Keep documentation in sync: update DESIGN.md and USER_GUIDE.md when code changes.
- Externalise strings to `constants.go` where possible.
- The authoritative specification is `SPEC.md`; the OpenAPI spec is `openapi.yaml`.

## Special Commands

These words as user input trigger specific workflows.

- `spec` — Rebuild SPEC.md and openapi.yaml from the codebase
- `drift` — Check documentation vs implementation drift
- `next` — Pick up next ticket or continue current
- `review` — Read DESIGN/USER_GUIDE, tickets and propose next steps
- `continue` — Read TODO/DESIGN/USER_GUIDE and tickets and continue implementation
- `pr` — File a PR containing the ticket ID
- `linear` / `walkthrough` — Generate a code walkthrough using showboat

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
