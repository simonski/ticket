# Copilot instructions for `simonski/ticket`

## Build, test, and lint commands

Use the Make targets in `Makefile` as the canonical entrypoints:

```bash
make setup              # Go modules + Node + Playwright
make build-dev          # build ./bin/tk (does NOT bump cmd/tk/VERSION)
make build              # build + bump patch version + sync openapi.yaml version
make lint               # golangci-lint + gosec
make test-go            # all Go tests
make test-go-cover      # package coverage gates (cmd/tk, libticket, client, store, server, config)
make test               # full suite: unit + integration + docs tests + harness + playwright
```

Targeted runs:

```bash
go test ./internal/store -run TestTicketLifecycle
go test ./internal/server -run TestWorkflowVersionGovernanceAPI
go test ./... -race
```

Executable docs/tutorial verification:

```bash
make test-tk-test         # runs cmd/tk-test against docs/quickstarts/*.md
make test-todo-example    # verifies scripts/populate_todo_example.sh scenario
make testscripts          # shell harness regression
make validate-openapi     # structural OpenAPI check
```

## High-level architecture

- The product is a **single Go binary** (`cmd/tk/main.go`) exposing four interfaces over one domain model:
  1. CLI commands (`tk ...`)
  2. HTTP API (`internal/server`, routes registered via `internal/server/api.go`)
  3. Embedded web UIs (`web/static` + `web/site2`, served by server)
  4. BubbleTea TUI (`internal/tui`)

- There are two execution modes behind one service contract:
  - **Local mode**: direct SQLite via `internal/store`
  - **Remote mode**: HTTP client via `internal/client`
  - Both are aligned through `libticket.Service` and contract tests in `libticket/contract_test.go`.

- Configuration is split between global and repo-local state:
  - Global `$TICKET_HOME` (default `~/.ticket`) stores DB, remotes registry, and credentials.
  - Repo-local `.ticket/config.json` stores active remote + project binding.
  - CLI resolves mode/binding by walking up from CWD.

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

- **Docs quickstarts are executable test assets**:
  - shell blocks in `docs/quickstarts/client.md` and `docs/quickstarts/server.md` are run by `cmd/tk-test`.
  - keep examples automation-safe and consistent with current CLI behavior.

- `tk server` serves **site2 by default**; use `tk server -site default` only when explicitly testing the legacy site.
