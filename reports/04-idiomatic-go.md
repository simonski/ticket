# Idiomatic Go

**Score: 72/100**

## What is being assessed
Go code quality against idiomatic patterns: error handling, context propagation, concurrency safety, package organisation, interface design, naming conventions, resource cleanup, test patterns, linting configuration, and use of modern stdlib.

## Methodology
Reviewed all Go source files across `cmd/ticket/`, `internal/`, `libticket/`, `libtickethttp/`, and `libtickettest/`. Inspected `go.mod`, `Makefile`, and all test files. Searched for context usage, mutex patterns, defer patterns, deprecated stdlib calls, and linting configuration.

## Findings

### Passing checks
- Error wrapping with `%w` used consistently (`internal/store/store.go:1478`, `internal/store/auth.go:157`)
- Sentinel errors defined and checked with `errors.Is()`: `internal/store/auth.go:15-19`, `internal/store/project.go:11`
- Clean package DAG — no circular imports; dependencies flow `cmd → lib → internal/store`
- `sync.RWMutex` used correctly on `liveHub` (`internal/server/realtime.go:30`), rate limiter (`internal/server/ratelimit.go:25`), and chat runtime (`internal/server/chat_ws.go:37`)
- Buffered channels with proper select patterns (`internal/server/realtime.go:50`, `internal/server/chat_ws.go:95`)
- `sync.Once` for resource close (`internal/server/realtime.go:83`, `internal/server/chat_ws.go:38`)
- `defer rows.Close()` on all SQL queries; `defer resp.Body.Close()` on all HTTP responses
- Idiomatic acronym casing: `ID`, `UserID`, `ProjectID`, `WorkflowID` (not `Id`, `UserId`)
- No deprecated `ioutil.*` — uses `os.ReadAll()`, `os.WriteFile()`, `io.NopCloser()`
- No unnecessary dependencies in `go.mod`; minimal, well-chosen set

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No `context.Context` passed to any DB calls | High | `internal/store/activity.go:38,45,53`, `internal/store/ticket.go` (all queries) | Refactor store functions to accept `ctx context.Context` as first param; use `QueryContext()`, `ExecContext()` |
| HTTP handlers never pass request context to store | High | `internal/server/api.go:100+` | Pass `r.Context()` through to all store calls |
| No `.golangci.yml` linting config | Medium | repo root | Add config enabling `errcheck`, `govet`, `staticcheck`, `unused`, `contextcheck` |
| No `make lint` target | Medium | `Makefile` | Add lint target running `golangci-lint run ./...` |
| Test helper functions lack `t.Helper()` | Low | `internal/store/store_test.go`, `internal/server/api_test.go` | Add `t.Helper()` as first line of each helper function |
| Most tests use `defer` instead of `t.Cleanup()` | Low | `internal/store/auth_test.go`, `internal/store/workflow_test.go` | Prefer `t.Cleanup()` for parallel test safety |
| `go.mod` declares `go 1.26.0` (pre-release) | Low | `go.mod:3` | Pin to latest stable release (1.23.x) for reproducible builds |

## Verdict
Solid Go codebase with excellent naming, resource management, and concurrency patterns. The critical gap is complete absence of `context.Context` through the storage layer, which prevents request cancellation, deadline propagation, and observability hooks. Linting infrastructure is missing entirely, which will allow regressions in code quality over time.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Context propagation in store layer | High | Add `ctx context.Context` to all store function signatures; use `*Context` SQL variants |
| Add `.golangci.yml` | Medium | Enable at minimum: `errcheck`, `govet`, `staticcheck`, `contextcheck` |
| Add `make lint` to Makefile and CI | Medium | Run `golangci-lint run ./...` as a gate before tests |
| Add `t.Helper()` to test helpers | Low | `testDB()`, `testHandler()`, `assertTableExists()` etc. |
| Pin Go version in `go.mod` to stable | Low | Use `go 1.23.x` not a preview version |
