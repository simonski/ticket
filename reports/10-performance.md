# Performance

**Score: 60/100**

## What is being assessed
N+1 query patterns, unbounded resource queries, connection pooling, goroutine leak prevention, SSE/WebSocket scalability, pagination on list endpoints, keepalive/heartbeat patterns, and static file serving efficiency.

## Methodology
Reviewed all store query functions for LIMIT clauses and JOIN opportunities. Traced `hydrateTicket()` call chain. Checked all `List*` functions in `internal/store/`. Reviewed static file serving in `internal/server/server.go`. Inspected WebSocket goroutine management and heartbeat patterns.

## Findings

### Passing checks
- SQLite WAL mode: `PRAGMA journal_mode=WAL` — concurrent readers with single writer (`store.go`)
- `busy_timeout=5000ms` — prevents immediate failure under lock contention
- `MaxOpenConns=1`, `MaxIdleConns=1` — correct for SQLite's write serialisation model
- 26 indexes defined covering primary query patterns (project_id, assignee, stage, state, ticket_id)
- WebSocket clients use buffered channels (32/64 items) with non-blocking send — no slow-subscriber backpressure
- `sync.Once` for WebSocket cleanup — no goroutine leaks
- Agent reaper goroutine bounded: single goroutine, 1-minute tick, stopped via channel
- List tickets endpoint supports `limit` and `offset` parameters
- WebSocket heartbeat: `ensureRealtimeSync()` polls if WS unhealthy for >45s

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| N+1 query: `hydrateTicket()` fetches comments in a separate query for every `GetTicket()` call | High | `internal/store/ticket.go:837-843` | Use JOIN or batch fetch comments for all tickets in a list query |
| `ListProjects()`, `ListUsers()`, `ListAgents()`, `ListTeams()`, `ListStories()`, `ListLabels()`, `ListHistoryEvents()` have no LIMIT | High | `internal/store/project.go:120-180`, `auth.go:200-220`, `agent.go:70-90`, `team.go:104-123` | Add default LIMIT (e.g., 100) with configurable page size to all list functions |
| `ListTicketParents()` loops calling `GetTicket()` per ancestor — O(depth) queries | Medium | `internal/store/ticket.go:754-770` | Load parent chain in single recursive CTE query |
| No HTTP caching headers on static file responses | Medium | `internal/server/server.go:87-124` | Add `Cache-Control: public, max-age=86400` and ETag support |
| No gzip compression on HTTP responses | Medium | `internal/server/server.go` | Wrap handler with `gzip.Handler` or use `compress/gzip` middleware |
| `fs.Stat()` called on every static file request — no in-memory cache | Medium | `internal/server/server.go:111-125` | Use `http.FileServerFS` with `io/fs` caching layer |
| Missing indexes on `tickets.open`, `tickets.archived`, `tickets.status` — full table scan on soft-delete filter | High | `internal/store/store.go` | Add `CREATE INDEX IF NOT EXISTS idx_tickets_open ON tickets(open)` (and archived, status) |
| Missing `idx_project_members_user_id`, `idx_team_members_user_id` — O(n) user-centric lookups | High | `internal/store/store.go` | Add indexes on `user_id` columns in all membership tables |
| OFFSET-based pagination is O(n) for late pages | Low | `internal/store/ticket.go` | Switch to cursor-based pagination using `ticket_id > cursor` |
| No query timing metrics — impossible to identify slow queries in production | Medium | `internal/store/` | Add query timing via wrapper or `slog` structured logging |

## Verdict
Good foundation with WAL mode, proper connection pooling, and indexed primary query paths. The N+1 comments hydration is the most impactful issue — a list of 50 tickets generates 51 queries. Unbounded list queries are a latency time-bomb as data grows. The missing indexes on soft-delete fields (`open`, `archived`) cause full table scans on the most common query pattern.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix N+1 in `hydrateTicket()` | High | Batch-fetch comments with `WHERE ticket_id IN (...)` |
| Add LIMIT to all list functions | High | Default 100, configurable via params |
| Add missing indexes on `open`, `archived`, `status`, `user_id` membership tables | High | 9 missing indexes identified in db review |
| Add HTTP caching headers | Medium | `Cache-Control`, `ETag`, `Last-Modified` on static assets |
| Add gzip middleware | Medium | ~60-70% reduction in static asset transfer size |
| Cursor-based pagination | Low | Replace `OFFSET` with `WHERE ticket_id > ?` for large datasets |
