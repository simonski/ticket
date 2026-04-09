# Idiomatic Go

**Score: 78/100** (was 80)

## What is being assessed
Error handling patterns (`%w` wrapping, no swallowed errors), context propagation through handlers, concurrency safety (goroutine lifecycle, channel bounds, mutex discipline), package organisation, interface design, naming conventions, and Go version currency.

## Methodology
Read `cmd/ticket/main.go`, `internal/server/chat_ws.go`, `internal/client/client.go` (first 100 lines), `libticket/service.go`, `go.mod`. Grepped for `context.Background` in server handlers, `go func`, goroutine spawn patterns.

## Findings

### Passing checks
- Error wrapping with `%w` used consistently in cmd handlers (`cmd/ticket/cmd_setup.go:296-303`)
- No swallowed errors found; strategic `_ = err` in cleanup defers only (`chat_ws.go:144, 655`)
- Goroutine lifecycle properly scoped: write loops use `defer client.close()`, timeout goroutines exit on completion (`chat_ws.go:110-122, 301-315`)
- All channels are bounded: `send: make(chan []byte, 64)` in chat, `send: make(chan []byte, 32)` in realtime (`chat_ws.go:96`, `realtime.go:50`)
- Mutex discipline correct: `sync.Mutex` with `defer mu.Unlock()` pattern (`chat_ws.go:38-40`)
- Service interface properly decomposed into 7 sub-interfaces: Auth, User, Agent, Project, Team, Workflow, Ticket (`libticket/service.go`)
- Exported types follow Go conventions throughout
- No circular imports; `go build ./...` clean
- `go.mod`: Go 1.26.0 (current stable); dependencies up to date
- `cmd/ticket/cmd_setup.go:16`: `strconv` import correctly added in recent commit

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `context.Background()` used in WebSocket request handler instead of `r.Context()` | High | `internal/server/chat_ws.go:168, 177` | Replace with request context to enable cancellation propagation |
| `libticket/local.go` uses `context.Background()` ~125 times | Medium | `libticket/local.go` throughout | Thread `ctx` parameter through `LocalService` methods |
| Non-wrapped `fmt.Errorf` for HTTP status error | Low | `cmd/ticket/cmd_setup.go:307` | Use `fmt.Errorf("server returned status %d: %w", resp.StatusCode, someErr)` |

## Verdict
The codebase has strong concurrency patterns and good error wrapping discipline. The confirmed use of `context.Background()` inside a live HTTP request handler (`chat_ws.go:168,177`) breaks request-scoped deadline propagation and is the primary regression from the previous assessment. The `LocalService` context gap is architectural but lower urgency.

## Changes since last assessment
- `strconv` import cleanly added to `cmd_setup.go` (TK-222 / init checks work)
- `runInitCheckDefaults` adds new error-wrapped paths — error handling correct
- `context.Background()` in `chat_ws.go` confirmed present (known since v0.1.737)
- No new concurrency patterns introduced

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `context.Background()` in WS handler | High | `store.ChatEnabled(r.Context(), db)` and `store.ChatLimitsConfig(r.Context(), db)` |
| `LocalService` context propagation | Medium | Add `ctx context.Context` parameter to `LocalService` methods; use it in store calls |
