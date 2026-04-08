# Idiomatic Go

**Score: 81/100** (was 72)

## What is being assessed
Go code quality against idiomatic patterns: error handling, context propagation, concurrency safety, package organisation, interface design, naming conventions, resource cleanup, test patterns, linting configuration, and use of modern stdlib.

## Methodology
Reviewed all Go source files across `cmd/ticket/`, `internal/`, `libticket/`, `libtickethttp/`, and `libtickettest/`. Inspected `go.mod`, `Makefile`, `.golangci.yml`, and all test files. Searched for context usage, mutex patterns, defer patterns, deprecated stdlib calls, linting configuration, and nosec annotations. Version 0.1.737.

## Findings

### Passing checks
- Error wrapping with `%w` used consistently — 127 instances (`internal/store/store.go`, `internal/store/ticket.go`, etc.)
- Sentinel errors defined with `errors.New` and checked with `errors.Is()`: `internal/store/ticket.go:13-16`, `internal/store/auth.go:17-20`, `internal/store/project.go:12`
- Clean package DAG — no circular imports; dependencies flow `cmd → libticket → internal/store`
- `sync.Mutex` used correctly on `liveHub` (`internal/server/realtime.go`), rate limiter (`internal/server/ratelimit.go:11`), and chat runtime (`internal/server/chat_ws.go:38,58`)
- Buffered channels with proper select patterns (`internal/server/realtime.go`, `internal/server/chat_ws.go`)
- `defer rows.Close()` on all SQL queries; `defer resp.Body.Close()` on all HTTP responses
- Idiomatic acronym casing: `ID`, `UserID`, `ProjectID`, `WorkflowID` (not `Id`, `UserId`)
- No deprecated `ioutil.*` — uses `os.ReadFile()`, `os.WriteFile()`, `io.NopCloser()`
- No unnecessary dependencies in `go.mod`; minimal, well-chosen set
- Store layer now accepts `context.Context` as first param on all 40+ store functions (`internal/store/ticket.go:112`, `internal/store/store.go:108`)
- HTTP API handlers pass `r.Context()` to all store calls (`internal/server/api_tickets.go:35,44,66`)
- `.golangci.yml` exists with solid linter set: errcheck, govet (shadow), staticcheck, unused, gosimple, ineffassign, gocritic, misspell, gofmt, noctx
- `make lint` target present and runs `golangci-lint run ./...`
- 75 `#nosec` annotations all carry explanatory comments; zero unadorned suppressions
- `t.TempDir()` used in 97+ test sites — automatic cleanup without `defer os.RemoveAll`
- Table-driven tests used in `cmd/ticket/main_test.go:688`, `cmd/ticket/main_test.go:2191`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `libticket/local.go` calls `context.Background()` 125 times; `Service` interface has no ctx parameter so request context never reaches the store via the library path | High | `libticket/local.go:54,57,65,71,89` (and ~120 more) | Add `ctx context.Context` as first arg to all `Service` interface methods; `LocalService` can then forward the caller's context to store functions |
| `analyse.go` calls `store.GetRoleByTitle(context.Background(), ...)` inside an HTTP request handler | Medium | `internal/server/analyse.go:134,176` | Use `r.Context()` instead so the request context (deadline/cancellation) is honoured |
| `http.Get` called without a context-aware client (caught by `noctx` linter) | Low | `cmd/ticket/cmd_setup.go:298`, `cmd/tk-test/main.go:524` | Use `http.NewRequestWithContext` + `http.DefaultClient.Do()` |
| `go.mod` declares `go 1.26.0` (unreleased as of this writing) | Low | `go.mod:3` | Pin to latest stable release (1.23.x) until 1.26 ships |
| `r.PathValue("{id}")` not used despite go.mod requiring ≥1.22 | Low | `internal/server/api_tickets.go:338`, `api_projects.go:73`, all `*_handlers.go` | Replace 11× `strings.TrimPrefix(r.URL.Path, prefix)` with named route params in `mux.HandleFunc("/api/tickets/{id}", ...)` + `r.PathValue("id")` |
| `_ = json.NewEncoder(w).Encode(payload)` silently swallows encode errors | Low | `internal/server/api_helpers.go:204` | Log or at least `slog.Warn` the encode error so write failures are visible |

## Verdict
A substantial jump from 72 → 81. Both previously-critical gaps are closed: the store layer is now context-aware throughout, and a credible linting pipeline (`golangci.yml` + `make lint` + `gosec` in CI) is in place. The remaining score ceiling is the `Service` interface abstraction, which still discards the caller's context at the library boundary — fixing that single issue would push the score to ≥87 and is the highest-leverage remaining change.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `internal/store` functions now accept `ctx context.Context` throughout | +6 — critical context propagation gap at store layer closed |
| HTTP handlers (`internal/server/api_*.go`) now pass `r.Context()` to all store calls | +4 — server-side context propagation correct |
| `.golangci.yml` added with 10 linters including `noctx`, `govet shadow`, `gocritic` | +3 — quality gate now enforced in CI |
| `make lint` target added | +1 |
| All gosec findings resolved — 75 `#nosec` annotations, every one with a justification comment | +2 |
| `publish` CI job added to `makefile.yaml` (build → test → gosec → govulncheck → release) | +1 |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `Service` interface takes no context; `LocalService` uses `context.Background()` 125× | High | Add `ctx context.Context` as first parameter to all 108 `Service` methods |
| `analyse.go` HTTP handler uses `context.Background()` for store calls | Medium | Use `r.Context()` in all handler-triggered calls |
| `http.Get` without context in 2 places | Low | Use `http.NewRequestWithContext` |
| `go.mod` requires `go 1.26.0` (pre-release) | Low | Pin to latest stable Go |
| `r.PathValue` not used despite Go 1.22+ required | Low | Adopt named route variables; eliminates 11× `strings.TrimPrefix` path parsing |
| Encode error silenced in `writeJSON` | Low | Log write failures |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Context propagation in store layer | High | Add `ctx context.Context` to all store function signatures; use `*Context` SQL variants |
| Add `.golangci.yml` | Medium | Enable at minimum: `errcheck`, `govet`, `staticcheck`, `contextcheck` |
| Add `make lint` to Makefile and CI | Medium | Run `golangci-lint run ./...` as a gate before tests |
| Add `t.Helper()` to test helpers | Low | `testDB()`, `testHandler()`, `assertTableExists()` etc. |
| Pin Go version in `go.mod` to stable | Low | Use `go 1.23.x` not a preview version |
