# Architecture

**Score: 70/100**

## What is being assessed
Package dependency DAG (no circular imports), resource bounding, plugin/provider patterns, event/notification system design, reconciliation loops, interface abstraction quality, and scalability ceiling.

## Methodology
Ran `go build ./...` to verify no circular imports. Reviewed all package boundaries, import chains, `libticket/service.go` interface, `internal/server/realtime.go` event system, `internal/store/store.go` connection config, and background goroutines. Traced end-to-end data flows.

## Findings

### Passing checks
- Clean package DAG: `cmd → libticket → internal/store` — no circular imports (verified by build)
- Dependency flows: `store` (foundation) → `server`/`client` (mid-tier) → `libticket`/`libtickethttp` (abstraction) → `cmd/ticket` (top)
- Dual-mode abstraction: `Service` interface hides whether backend is local SQLite or remote HTTP
- WebSocket event system: 28 event types broadcast to connected clients (`internal/server/realtime.go`)
- Buffered channels (32/64 items) with non-blocking send — prevents slow subscriber backpressure
- `sync.Once` for safe resource cleanup; `sync.RWMutex` on hub client map
- Agent reaper goroutine (`server.go:37`) — 1-minute tick, bounded, stopped via channel
- WAL mode enabled: `PRAGMA journal_mode=WAL` — allows concurrent readers with one writer
- `busy_timeout=5000ms` — prevents immediate lock failure under contention
- `MaxOpenConns=1` — correct for SQLite write serialisation
- Config resolution clean: `file://` → local, `http(s)://` → remote (`internal/config/config.go`)
- No circular dependencies between any packages

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| SQLite `MaxOpenConns=1` is a hard concurrency ceiling (~50-100 concurrent users) | High | `internal/store/store.go:24-25` | Document limit explicitly; provide PostgreSQL migration path for scaling |
| `libticket.Service` has 104 methods — violates Interface Segregation Principle | High | `libticket/service.go` | Split into domain sub-interfaces: `TicketService`, `UserService`, `WorkflowService` etc. |
| `internal/client/client.go` has ~80 mode-branching `if c.mode == local` checks | High | `internal/client/client.go` | Strategy pattern: `LocalClient` and `RemoteClient` implement same interface independently |
| History events grow unbounded — no TTL or archival policy | High | `internal/store/activity.go` | Add `TICKET_HISTORY_RETENTION_DAYS` config; periodic cleanup goroutine |
| Rate limiter map grows unbounded with unique IP addresses | Medium | `internal/server/ratelimit.go` | Add periodic cleanup of stale entries (e.g., entries older than window) |
| Event system: no event persistence — reconnecting clients miss events | Medium | `internal/server/realtime.go` | Add event log table; replay on reconnect (cursor-based) |
| Event broadcast is O(n) fanout to all clients — no topic filtering | Medium | `internal/server/realtime.go` | Add project-scoped subscriptions to avoid cross-project event leakage |
| Storage backend tightly coupled to SQLite — no abstraction layer | Medium | `internal/store/` | Define `Store` interface; implement `SQLiteStore`; enables future backends |
| No schema migration versioning — migrations run on every boot | Medium | `internal/store/store.go:105-546` | Add `schema_version` table; run only unapplied migrations |
| Comments table has no archival policy — grows with deleted projects | Low | `internal/store/` | Add `ON DELETE CASCADE` or explicit cleanup in `DeleteProject` |

## Verdict
Clean layered architecture with no circular imports and a well-designed dual-mode abstraction. The main architectural risks are the SQLite concurrency ceiling (hard limit of ~50-100 concurrent users without migration), the 104-method God interface making mocking and versioning difficult, and unbounded data growth in history/rate-limiter tables. These are manageable at current scale but require attention before production growth.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Document SQLite concurrency ceiling | High | Add capacity note to README and deploy docs |
| Split `Service` interface | High | Domain sub-interfaces: `TicketService`, `UserService`, `WorkflowService` etc. |
| History TTL policy | High | Config var + cleanup goroutine for old history events |
| Strategy pattern for `client.go` | Medium | Eliminate 80 mode-check branches |
| Add `Store` interface | Medium | Enables alternative backends and proper mocking |
| Schema migration versioning | Medium | `schema_version` table; skip already-applied migrations |
| Rate limiter map cleanup | Medium | Periodic eviction of expired entries |
| Event persistence for reconnect | Medium | Replay cursor-based event log on WS reconnect |
