# Testing

## Test suites

| Target                | What it covers                                      | Duration |
|-----------------------|-----------------------------------------------------|----------|
| `make test-unit`      | Config, password hashing, web package                | ~1s      |
| `make test-integration` | CLI, client, server, store, libticket, libtickethttp | ~25s     |
| `make test-go-cover`  | All Go tests with per-package coverage thresholds    | ~30s     |
| `make test-playwright`| Browser tests against the web UI (11 spec files)     | ~20s     |
| `make test-tk-test`   | Executable documentation tests (see below)           | ~15s     |
| `make test`           | Unit + integration + playwright                      | ~50s     |

Run a single Go test:

```bash
go test ./internal/store/ -run TestTicketLifecycle
```

## tk-test: executable documentation

`cmd/tk-test` is a tool that turns markdown documentation into tests. It parses
fenced ` ```bash ` code blocks from markdown files and executes them sequentially,
verifying each block exits 0.

### How it works

1. Reads one or more markdown files passed as arguments
2. Extracts every fenced `bash` code block, tracking its file and line number
3. Creates an isolated temp directory with `TICKET_HOME`, a fresh `git init`, and
   the built `ticket` binary on `PATH`
4. Runs each block in order using `bash -e`, carrying environment state between
   blocks (simulating a user following a tutorial step by step)
5. Reports pass/fail/skip per block with `file:line` references

### Usage

```bash
# Run against the quickstart guides (requires make build first)
make test-tk-test

# Run directly with verbose output
go run ./cmd/tk-test -v QUICKSTART_CLIENT.md QUICKSTART_SERVER.md

# Point at a different binary
go run ./cmd/tk-test -ticket ./bin/tk QUICKSTART_CLIENT.md
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

1. Runs `tk initdb` non-interactively (since `tk init` is interactive)
2. Picks a random free port to avoid conflicts with running services
3. Starts the server in the background on that port
4. Waits for `/api/healthz` to respond (up to 10 seconds)
5. Rewrites `localhost:8080` references in subsequent blocks to the dynamic port
6. Updates `config.json` so the CLI detects remote mode
7. Kills the server when the file finishes

### TICKET_URL bridging

The quickstart docs use `export TICKET_URL=http://localhost:8080` to switch the
CLI to remote mode. The actual CLI reads mode from the `location` field in
`config.json`, not from an environment variable. tk-test bridges this gap by
updating `config.json` whenever it encounters a `TICKET_URL` export.

## Contract tests

`libtickettest/contract.go` defines a `Factory` pattern and a `RunServiceContractTests`
function that exercises the full `libticket.Service` interface. Both implementations
run the same suite:

- `libticket/local_test.go` — tests `LocalService` (direct SQLite)
- `libtickethttp/http_test.go` — tests `libtickethttp.Service` (HTTP client against a test server)

## Coverage thresholds

Enforced via `make test-go-cover`:

| Package              | Minimum |
|----------------------|---------|
| `cmd/ticket`         | 55%     |
| `libticket`          | 65%     |
| `libtickethttp`      | 75%     |
| `internal/client`    | 55%     |
| `internal/store`     | 70%     |
| `internal/config`    | 70%     |

## Playwright browser tests

Located in `tests/playwright/` with 11 spec files covering auth, navigation,
ticket management, projects, stories, sdlcs, labels, time tracking,
dependencies, hierarchy, and chat. Run with:

```bash
make test-playwright
```

Requires Node and Chromium (`make setup-playwright` installs both).
