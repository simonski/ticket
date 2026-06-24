# Copilot instructions for `simonski/ticket`

## Build, test, and lint commands

Use the Make targets in `Makefile` as the canonical entrypoints:

```bash
make setup              # Go modules + Node + Playwright
make build-dev          # build ./bin/tk (does NOT bump cmd/tk/VERSION)
make build              # build + bump patch version + sync openapi.yaml version
make lint               # golangci-lint + gosec
make test               # ultra-fast default: unit tests
make test-fast          # recommended developer loop: unit + Go API smoke
make test-api-smoke     # fast Go API smoke packages (internal/client + internal/server)
make test-cli           # heavier CLI package tests
make test-contract      # heavier libticket contract tests
make test-all           # full suite: unit + api + browser + quickstart + docs/harness
make test-api-cli       # CLI/API interface tests (cmd + client + server + contract)
make test-api           # alias for test-api-cli
make test-browser       # fast browser smoke suite (Playwright)
make test-browser-full  # full browser E2E suite (Playwright)
make test-quickstart    # executable QUICKSTART/TUTORIAL docs tests
make test-go            # all Go tests
make test-go-cover      # package coverage gates (cmd/tk, libticket, client, store, server, config)
make ci-bootstrap       # install deps for the same verify/browser flow used by GitHub Actions
make ci-verify          # validate-openapi + coverage + build-dev + lint + vulncheck
make ci-browser         # full browser E2E suite used by GitHub Actions
make ci                 # ci-verify + ci-browser
```

Targeted runs:

```bash
go test ./internal/store -run TestTicketLifecycle
go test ./internal/server -run TestWorkflowVersionGovernanceAPI
go test ./... -race
```

Executable docs/tutorial verification:

```bash
make test-quickstart      # runs cmd/tk-test against QUICKSTART.md and TUTORIAL.md
make test-todo-example    # verifies scripts/populate_todo_example.sh scenario
make testscripts          # shell harness regression
make validate-openapi     # structural OpenAPI check
```

## High-level architecture

- The product is a **single Go binary** (`cmd/tk/main.go`) exposing four interfaces over one domain model:
  1. CLI commands (`tk ...`)
  2. HTTP API (`internal/server`, routes registered via `internal/server/api.go`)
  3. Embedded web UIs (`web/default` + `web/shared`, served by server)
  4. BubbleTea TUI (`internal/tui`)

- Runtime is client/server behind one service contract:
  - **Server**: SQLite persistence via `internal/store`
  - **Client**: HTTP access via `internal/client`
  - Both align through `libticket.Service` and contract tests in `libticket/contract_test.go`.

- Configuration is split between global and repo-local state:
  - Global `$TICKET_HOME` (default `~/.config/ticket`) stores the SQLite DB, TUI preferences, and credentials.
  - Repo-local `.ticket/config.json` stores project binding for the current working tree.
  - Server selection comes from `TICKET_URL`; auth is reused from `credentials.json` or environment variables.

- Lifecycle/workflow model is central:
  - Projects bind to workflows (ordered stages + stage-role ordering + optional DAG transitions).
  - Ticket lifecycle uses `stage`, `role`, `state` (`idle|active|success|fail`), `draft`, `complete`, `archived`.
  - Parent tickets derive lifecycle from children; direct lifecycle mutation is for leaf tickets.

## Key repository conventions

- **Do not use `make build` for routine validation** unless version bump is intended; prefer `make build-dev` / `make build-bin`.

- **Keep API contract/docs synchronized with behavior changes**:
  - update `openapi.yaml` for endpoint/schema changes
  - update `USER_GUIDE.md` for user-visible CLI/API/Web changes
  - update `docs/DESIGN.md` for architecture-level changes

- **Server API tests use common helpers/patterns** in `internal/server/api_test.go`:
  - `testHandler(t)` + `doJSONRequest(...)` + `decodeResponse(...)`
  - follow this pattern for new endpoint coverage.

- **Service changes should be covered in contract tests**, not just one implementation:
  - extend `libticket/contract_test.go` and ensure both local + HTTP implementations pass.

- **Quickstart/tutorial docs are executable test assets**:
  - shell blocks in `QUICKSTART.md` and `TUTORIAL.md` are run by `cmd/tk-test`.
  - keep examples automation-safe and consistent with current CLI behavior.

- **Use staged test gates to keep iteration fast**:
  - always run `make test` + `make lint` for normal edits
  - prefer `make test-fast` for the normal developer loop before escalating to broader suites
  - run `make test-api` when API contract/surface changes (`openapi.yaml`, `internal/server`, `internal/client`, CLI API handlers)
  - run `make test-browser` while iterating on web UX, then `make test-browser-full`
  - run `make test-all` before completion/PR

- `tk server` serves the embedded web UI from `web/default` + `web/shared` (the only site).
