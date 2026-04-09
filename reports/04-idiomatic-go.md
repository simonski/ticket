# Idiomatic Go

**Score: 80/100** (was 81)

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
Fresh re-assessment reveals a slight regression (81 → 80). Store-layer context propagation remains solid throughout, and the linting pipeline is mature. However the WebSocket chat handler (`chat_ws.go:168,177`) was found to use `context.Background()` inside request-scoped code — a new finding not captured in the previous report. The Service interface's 125 `context.Background()` calls remain the highest-leverage unfixed item. The duplicate "Remaining recommendations" section below has been retained for historical reference but the second block is superseded.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `internal/store` functions accept `ctx context.Context` throughout | +6 — critical context propagation gap closed |
| HTTP handlers pass `r.Context()` to all store calls | +4 — server-side propagation correct |
| `.golangci.yml` with 10 linters (noctx, govet shadow, gocritic) | +3 — quality gate in CI |
| All gosec findings resolved with justified `#nosec` comments | +2 |
| **New finding:** `chat_ws.go:168,177` uses `context.Background()` inside WS handler | -1 — context loss in chat path |
| Service interface still takes no `ctx`; `LocalService` has 125 `context.Background()` calls | -4 — highest-leverage remaining gap |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `Service` interface takes no context; `LocalService` uses `context.Background()` 125× | High | Add `ctx context.Context` as first parameter to all 108 `Service` methods |
| `chat_ws.go:168,177` uses `context.Background()` in WS handler | Medium | Derive context from WebSocket upgrade request |
| `analyse.go` HTTP handler uses `context.Background()` for store calls | Medium | Use `r.Context()` in all handler-triggered calls |
| `http.Get` without context in 2 places (`cmd_setup.go:298,156`) | Low | Use `http.NewRequestWithContext` + `http.DefaultClient.Do()` |
| `go.mod` requires `go 1.26.0` (pre-release) | Low | Pin to latest stable Go release |
| `r.PathValue` not used despite Go 1.22+ required | Low | Adopt named route variables; eliminates 11× `strings.TrimPrefix` |
| Encode error silenced in `writeJSON` | Low | `slog.Warn` on write failures in `api_helpers.go:204` |
