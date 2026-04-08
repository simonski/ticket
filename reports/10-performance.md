# Performance

**Score: 60/100** (was 60)

## What is being assessed
N+1 query patterns, unbounded list queries, connection pool configuration, database indexing, SSE/WebSocket scalability, pagination on list endpoints, goroutine lifecycle management, and slow-query visibility.

## Methodology
Reviewed `internal/store/store.go` for connection pool and index definitions. Inspected all `List*` functions in `internal/store/ticket.go`, `activity.go`, `agent.go`, `team.go`, `project.go` for LIMIT clauses. Checked `internal/server/api_tickets.go` and `api_projects.go` for pagination query param handling. Reviewed `internal/server/realtime.go` and `live_event.go` for SSE/WebSocket subscriber management. Checked `internal/server/chat_ws.go` for goroutine lifecycle patterns.

## Findings

### Passing checks
- SQLite WAL mode: `PRAGMA journal_mode=WAL` — concurrent readers do not block writer (internal/store/store.go)
- `busy_timeout=5000ms` — prevents immediate failure under write lock contention (internal/store/store.go)
- `MaxOpenConns(1)`, `MaxIdleConns(1)` — correct for SQLite; prevents "database is locked" SQLITE_BUSY errors (internal/store/store.go)
- 26+ indexes covering all primary query patterns: `project_id`, `parent_id`, `assignee`, `stage`, `state`, `ticket_id`, `item_id`, `depends_on` (internal/store/store.go:415-443)
- `liveHub.broadcast()` uses non-blocking `select/default` — slow WebSocket clients cannot stall broadcasts (internal/server/realtime.go:77)
- WebSocket clients have buffered send channels (`make(chan []byte, 32)` and `64`) — absorb burst events (internal/server/realtime.go:35, chat_ws.go)
- `liveHub` uses `sync.RWMutex` — concurrent broadcast reads do not block each other (internal/server/realtime.go:30)
- `sync.Once` for client cleanup — no goroutine leak on disconnect (internal/server/realtime.go:38)
- `GET /api/projects/:id/tickets` accepts `?limit=` query param — unbounded fetch can be capped by callers (internal/server/api_projects.go:100)
- `GET /api/projects/:id/history` defaults to `limit=10`, configurable via `?limit=` — history is bounded by default (internal/server/api_projects.go:153)
- Chat process bridge reaper uses bounded goroutine with stop channel — no goroutine accumulation (internal/server/chat_ws.go)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `ListHistoryEvents()` has no LIMIT — fetches all history rows per ticket with no bound | High | internal/store/activity.go:46-75 | Add `LIMIT` parameter (default 100); callers rarely need full unbounded history |
| `ListComments()` has no LIMIT — unbounded per-ticket comment fetch | High | internal/store/activity.go:170-193 | Add `LIMIT` parameter (default 50) with optional `offset` |
| `GET /api/projects/:id/tickets` defaults to `limit=0` (unlimited) — full project scan on every list request | High | internal/server/api_projects.go:100, internal/store/ticket.go:682 | Change API default to `limit=200`; require explicit `all=true` for unbounded fetch |
| No OFFSET support on ticket list — pagination requires full re-scan to page 2+ | Medium | internal/store/ticket.go:634-700 | Add `Offset int` to `TicketListParams`; expose via `?offset=` query param |
| Missing indexes on `tickets.open` and `tickets.archived` — every active-ticket query does full table scan on soft-delete filter | High | internal/store/store.go (schema section) | `CREATE INDEX IF NOT EXISTS idx_tickets_open ON tickets(open, archived)` |
| `ListTicketsByProject()` fetches comments separately in `hydrateTicket()` — N+1 query per list call | High | internal/store/ticket.go:886, activity.go:170 | Batch-load comments via `WHERE ticket_id IN (...)` after fetching ticket list |
| No slow query detection or query timing instrumentation | Medium | internal/store/ | Wrap `db.QueryContext` with timing; log queries exceeding 100ms via `slog` |
| No HTTP response caching headers on static assets | Medium | internal/server/server.go | Add `Cache-Control: public, max-age=86400` and `ETag` support for embedded static files |
| No gzip/brotli compression on API or static responses | Low | internal/server/server.go | Wrap mux with `compress/gzip` middleware; ~60% reduction in JSON response size |
| `MaxOpenConns=1` undocumented — not visible to operators; concurrency ceiling invisible | Low | internal/store/store.go | Add comment explaining SQLite WAL single-writer design choice |

## Verdict
The SQLite foundation is well-configured (WAL, busy_timeout, correct pool settings) and indexed for the primary query patterns. The SSE/WebSocket layer is scalable with non-blocking broadcasts and buffered channels. The main performance gap is the unbounded `ListHistoryEvents` and `ListComments` per ticket (both have no LIMIT), the missing composite index on `(open, archived)` causing full scans on the most common query, and the N+1 comments fetch pattern on ticket hydration. No improvements to these issues were made since the last assessment — score holds at 60.

## Changes since last assessment
- No performance-related changes between 0.1.730 and 0.1.737; all seven version bumps were feature/fix work in chat bridge and CLI commands

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add LIMIT to `ListHistoryEvents()` and `ListComments()` | High | Default 100/50; expose via params |
| Default ticket list API to `limit=200` | High | Unbounded fetch is default path today |
| Add `idx_tickets_open_archived` composite index | High | Eliminates full table scan on active-ticket filter |
| Fix N+1 comments in `hydrateTicket()` | High | Batch `WHERE ticket_id IN (...)` after ticket list fetch |
| Add query timing instrumentation | Medium | Log queries >100ms; prerequisite for production diagnosis |
| Add OFFSET to ticket list params | Medium | Enables cursor-free pagination for simple clients |
| Add HTTP caching headers to static assets | Medium | Reduces repeat-visitor bandwidth |
| Document `MaxOpenConns=1` rationale | Low | Add inline comment in store.go |
