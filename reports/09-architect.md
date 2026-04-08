# Architecture

**Score: 73/100** (was 70)

## What is being assessed
Package dependency DAG (circular import detection), resource bounding, plugin/provider patterns, event/notification system design, interface abstraction quality, SQLite concurrency ceiling, background goroutine lifecycle, and unbounded data growth risks.

## Methodology
Ran `go list -f '{{.ImportPath}} -> {{join .Imports " "}}' ./...` to extract the full import graph. Reviewed `go.mod`, `cmd/ticket/main.go`, `libticket/service.go`, `libticket/local.go`, `libtickethttp/http.go`, `internal/config/config.go`, `internal/server/api.go`, `internal/server/realtime.go`, `internal/server/ratelimit.go`, `internal/server/server.go`, and all files in `internal/store/` (30 files). Counted Service interface methods, traced goroutine creation points, and verified bounding mechanisms.

**Package dependency DAG (confirmed, no cycles):**
```
cmd/ticket ─┬─► libticket ──────────────────────► internal/store
            │                                          ▲
            ├─► libtickethttp ──► internal/client ─────┤
            │                         ▲                │
            ├─► internal/server ───────────────────────┤
            │                                          │
            ├─► internal/tui ──► libticket ────────────┤
            └─► internal/config ◄── (most packages)

libtickettest ──► libticket, libtickethttp (test only)
```

**go.mod direct dependencies (7):** `charmbracelet/bubbles`, `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `google/uuid`, `golang.org/x/crypto`, `golang.org/x/term`, `modernc.org/sqlite`. No HTTP framework, no ORM — intentionally lean.

**Service interface (103 methods total):** `AuthService` (5) + `UserService` (10) + `AgentService` (13) + `ProjectService` (13) + `TeamService` (10) + `WorkflowService` (9) + `TicketService` (43).

## Findings

### Passing checks
- Clean package DAG — no circular imports (verified via `go list`) 
- Dependency flows: `store` (foundation) → `server`/`client` (mid-tier) → `libticket`/`libtickethttp` (abstraction) → `cmd/ticket` (top)
- Dual-mode abstraction: `Service` interface hides whether backend is local SQLite or remote HTTP (`libticket/service.go`)
- **Service interface now decomposed into 7 sub-interfaces** (`AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `WorkflowService`, `TicketService`) — ISP partially satisfied
- WebSocket event system: custom RFC 6455 implementation with origin validation (`realtime.go`)
- Buffered send channels (32 items) with non-blocking fallback — prevents slow-subscriber backpressure (`realtime.go:50`)
- `sync.Once` for safe WS resource cleanup; `sync.RWMutex` on hub client map (`realtime.go:30,84`)
- Agent reaper goroutine — 1-minute tick, stopped via channel (`server.go:41,64`)
- Daily history purge + hourly session purge goroutines wired to same stop channel (`server.go:81,93`)
- WAL mode + `busy_timeout=5000ms` + `MaxOpenConns=1` — correct SQLite concurrency config
- Auth rate limiter: 10 requests/1 min per IP (`api.go:14`); expired entries pruned on each `allow()` call
- Config resolution clean: `file://` or path → local; `http(s)://` → remote (`internal/config/config.go`)
- No external framework for HTTP routing — standard library `net/http.ServeMux` only
- `modernc.org/sqlite` (CGO-free pure-Go SQLite) — no CGO, simplifies cross-compilation and Docker

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| SQLite `MaxOpenConns=1` is a hard concurrency ceiling (~50–100 concurrent users at typical write rates) | High | `store.go:23` | Document limit explicitly in `README` and `deploy/`; provide PostgreSQL migration path |
| `internal/client/client.go` has grown to **98** `c.mode == local` branch points (up from ~80) | High | `internal/client/client.go` | Strategy pattern: separate `LocalClient` and `RemoteClient` structs implementing the same interface; eliminate branching entirely |
| `Service` composite interface is 103 methods — implementing mocks/stubs requires satisfying all 103 | High | `libticket/service.go` | Provide sub-interface type aliases consumers can depend on directly; contract tests already pass sub-interfaces implicitly |
| SSE/WebSocket hub has no subscriber count limit — a single server can be DoS'd with connections | Medium | `realtime.go:47,54` | Add configurable `max_live_connections`; return 503 when exceeded |
| Event system has no persistence — reconnecting clients miss events that fired while disconnected | Medium | `realtime.go` | Add an `events` table (ring buffer, last N events per project); replay on reconnect using a cursor |
| Event broadcast is O(n) fanout to **all** connected clients regardless of project — cross-project event leakage possible | Medium | `realtime.go:66-78` | Add project-scoped subscription filtering; only broadcast to clients watching the event's `project_id` |
| Rate limiter `attempts` map grows indefinitely for unique IP addresses that never hit the limit | Medium | `ratelimit.go:12` | Add periodic map sweep (e.g., every `window` duration) to evict IPs with no entries in the current window |
| `messages` and `time_entries` tables have no TTL/archival policy | Medium | `store.go:867`, `store.go:390` | Add `MESSAGE_RETENTION_DAYS`; `time_entries` are user-generated and will grow unbounded |
| No `Store` interface — `*sql.DB` is passed directly throughout; prevents alternative backends and proper unit mocking | Medium | `internal/store/` (all files) | Define a `Store` interface; implement `SQLiteStore`; enables PostgreSQL backend and test doubles |
| No schema migration versioning — ~50 guard-clause checks run on every startup | Medium | `store.go:470` | Add `schema_version` table; this is both a DB and architecture issue |
| `cmd/ticket/main.go` is a 457-line switch statement with 60+ cases — no sub-command routing | Low | `cmd/ticket/main.go:94` | Extract each command group into a `cmd/<group>/` package with its own `Run(args)` entry point |

## Verdict
The decomposition of `Service` into 7 named sub-interfaces is a meaningful structural improvement over the prior monolithic 104-method blob. History TTL and session purge goroutines close two previously open data-growth risks. The clean 7-dependency `go.mod` and zero-cycle package graph remain strong foundations. The main unresolved risks are the SQLite concurrency ceiling (architectural, not a code defect), the `client.go` mode-branching that grew from ~80 to 98 cases, the unbounded WebSocket hub, and the absence of a `Store` interface that would unlock both real mocking and future backend migration. Score moves from 70 to 73.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `Service` interface split into 7 named sub-interfaces (`AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `WorkflowService`, `TicketService`) | **+5** — ISP violation substantially addressed; sub-interfaces can be depended on independently |
| Daily `PurgeOldHistory` + hourly `PurgeExpiredSessions` goroutines added to server reaper | **+3** — two previously unbounded growth vectors now bounded |
| Auth rate limiter expired-entry pruning runs on each `allow()` call | **+1** — partial mitigation of rate limiter map growth |
| `client.go` mode-branch count grew from ~80 to 98 | **-3** — regression; mode-branching is expanding rather than being refactored away |
| New `messages` table has no TTL/archival policy | **-1** — new unbounded growth vector introduced |
| SSE hub still has no subscriber limit | **-2** — connection-based DoS vector remains open |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Document and plan for SQLite concurrency ceiling | High | Explicit capacity note in README; PostgreSQL migration path |
| Refactor `client.go` mode-branching | High | Strategy pattern: `LocalClient` + `RemoteClient` implementing shared interface |
| Expose sub-interfaces for external consumers | High | Allow callers to declare `TicketService` or `UserService` dependency, not full `Service` |
| Add `max_live_connections` cap to WebSocket hub | Medium | 503 when exceeded; configurable via `app_settings` |
| Add `Store` interface | Medium | Enables alternative backends and proper unit mocking |
| Schema migration versioning | Medium | `schema_version` table; skip already-applied migrations |
| Rate limiter map periodic eviction | Medium | Sweep entries older than window on a ticker |
| Event persistence for reconnect | Medium | Ring-buffer `events` table; replay on WS reconnect with cursor |
| Project-scoped event subscriptions | Medium | Filter hub broadcast by `project_id`; prevent cross-project leakage |
| Add TTL policy for `messages` and `time_entries` | Medium | Config var + purge goroutine |
| Extract command groups from `main.go` switch | Low | `cmd/<group>/` packages with own `Run()` entry points |
