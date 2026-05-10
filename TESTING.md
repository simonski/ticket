# Testing

## Test suites

| Target                | What it covers                                      | Duration |
|-----------------------|-----------------------------------------------------|----------|
| `make test`           | Fast default (unit tests only)                      | ~1s      |
| `make test-unit`      | Config, password hashing, web package                | ~1s      |
| `make test-api-js`    | JavaScript API client-library tests (`web/site2/api.test.js`) | ~2s |
| `make test-api-cli`   | CLI/API contract path (`cmd/tk`, client, server, libticket) | ~25s |
| `make test-api`       | Both API suites (`test-api-js` + `test-api-cli`)    | ~35s     |
| `make test-browser`   | Browser E2E Playwright suite                         | ~20s     |
| `make test-integration` | CLI, internal/client, server, store, libticket | ~25s     |
| `make test-go-cover`  | All Go tests with per-package coverage thresholds    | ~30s     |
| `make test-playwright`| Browser tests against the web UI (12 spec files)     | ~20s     |
| `make test-quickstart`| Executable QUICKSTART/TUTORIAL tests (see below)     | ~15s     |
| `make test-todo-example` | Reproducible todo tutorial seed + verification     | ~5s      |
| `make testscripts`    | Shell-based CLI harness scenarios                    | ~5s      |
| `make test-all`       | Unit + api + browser + quickstart + shell harnesses + todo example | ~80s |

Run a single Go test:

```bash
go test ./internal/store/ -run TestTicketLifecycle
```

## Recommended staged workflow

1. Fast local loop: `make test` + targeted tests.
2. If API contracts/surface changed: `make test-api`.
3. If web UI behavior changed: `make test-browser`.
4. Before completion/PR: `make test-all` and `make lint`.

## tk-test: executable documentation

`cmd/tk-test` is a tool that turns markdown documentation into tests. It parses
fenced ` ```bash ` code blocks from markdown files and executes them sequentially,
verifying each block exits 0.

### How it works

1. Reads one or more markdown files passed as arguments
2. Extracts every fenced `bash` code block, tracking its file and line number
3. Creates an isolated temp environment with a fresh Git repo as the working
   directory, a separate `$TICKET_HOME`, and the built `ticket` binary on `PATH`
4. Runs each block in order using `bash -e`, carrying environment state between
   blocks (simulating a user following a tutorial step by step)
5. Reports pass/fail/skip per block with `file:line` references

### Usage

```bash
# Run against QUICKSTART + TUTORIAL (requires make build-dev first)
make test-quickstart

# Run directly with verbose output
go run ./cmd/tk-test -v QUICKSTART.md TUTORIAL.md

# Point at a different binary
go run ./cmd/tk-test -ticket ./bin/tk QUICKSTART.md
```

### What gets skipped

The tool automatically skips blocks that cannot be executed in an automated test:

- **Interactive commands** — `tk -g`, `tk gui`
- **Placeholder values** — blocks containing `<agent-uuid>`, `<YOUR_TOKEN>`, etc.
- **Install commands** — `brew install`, `go install`, `docker`, `ssh`, `scp`
- **Output examples** — blocks that look like sample output rather than commands
- **Empty blocks and comments**

### Server mode handling

When a block contains `tk server`, tk-test:

1. Runs `tk initdb` when the documentation block still embeds first-run local setup
2. Picks a random free port to avoid conflicts with running services
3. Starts the server in the background on that port
4. Waits for `/api/healthz` to respond and prints captured server logs on failure
5. Rewrites `localhost:8080` references in subsequent blocks to the dynamic port
6. Updates repo-local `.ticket/config.json` so the CLI targets the test server
7. Kills the server when the file finishes

### Remote binding in docs

The executable docs are `QUICKSTART.md` and `TUTORIAL.md`. `cmd/tk-test`
rewrites `localhost:8080` references to the dynamic test server port so those
blocks stay executable.

## Script harness

`scripts/testharness.sh` is a growing shell-based regression harness for direct
CLI scripting flows. It creates an isolated temp repo plus `$TICKET_HOME`,
bootstraps a fresh local database with `tk initdb`, binds the repo
non-interactively with `tk project init`, and executes end-to-end scenarios
that assert behavior with CLI exit codes and `tk ls -count` checks.

Current harness scenarios cover:

1. scriptable count/assertion flows
2. Workflow progression, regression, and terminal-stage behavior
3. comment / idea / decision CRUD-adjacent operator flows plus snapshot export/import restore
4. remote server login, multi-project switching, and agent work request behavior
5. agent admin controls: config round-trip, password rotation, and project-targeted queue selection

Run it with:

```bash
make testscripts
```

## Todo example scenario verification

`scripts/populate_todo_example.sh` seeds a reproducible todo-app planning
workspace (project, Workflow, epic/tasks, labels, dependencies, time entries,
story/decision/idea). `scripts/verify_todo_example.sh` runs that seed flow in
an isolated temp repo/home pair, then asserts key expected outputs for project
`DEMO`.

Run it with:

```bash
make test-todo-example
```

## Contract tests

`libticket/contract_test.go` defines a `Factory` pattern and a `RunServiceContractTests`
function that exercises the full `libticket.Service` interface. Both implementations
run the same suite:

- `libticket/local_test.go` — tests `LocalService` (direct SQLite)
- `libticket/http_test.go` — tests the remote `libticket.Service` implementation against a test server

## Coverage thresholds

Enforced via `make test-go-cover`:

| Package              | Minimum |
|----------------------|---------|
| `cmd/tk`         | 55%     |
| `libticket`          | 65%     |
| `internal/client`    | 55%     |
| `internal/store`     | 70%     |
| `internal/server`    | 70%     |
| `internal/config`    | 70%     |

## Playwright browser tests

Located in `tests/playwright/` with 12 spec files covering auth, navigation,
ticket management, projects, stories, workflows, labels, time tracking,
dependencies, hierarchy, and chat. Run with:

```bash
make test-playwright
```

Requires Node and Chromium (`make setup-playwright` installs both).

The Playwright configs now auto-select a free localhost port by default. If you
need a fixed port for debugging, set `PLAYWRIGHT_PORT` or
`PLAYWRIGHT_SITE2_PORT` before running the tests. The main suite defaults to two
workers for stability; override that with `PLAYWRIGHT_WORKERS` if needed.
