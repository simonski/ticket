# Performance

**Score: 58/100** (was 62)

## What is being assessed
N+1 query patterns, unbounded resource usage, connection pool configuration, goroutine leak potential, SSE/WebSocket scalability, pagination on list endpoints, and algorithmic complexity of display functions.

## Methodology
Read `internal/store/ticket.go`, `internal/store/store.go` (indexes), `internal/server/api_tickets.go`. Ran grep for `CREATE INDEX`, `SELECT.*FROM messages`, `LIMIT`, `SetMaxOpenConns`. Analysed `buildTreeDisplay` algorithm in `printer.go`.

## Findings

### Passing checks
- 44 indexes defined covering most foreign key columns (`internal/store/store.go:427+`)
- `idx_history_events_project_id`, `idx_history_events_ticket_id` present (`store.go:427-428`)
- Batch comment loading: `batchFetchComments()` bulk-loads comments for multiple tickets in one query (`ticket.go:903`)
- Recursive CTE for ancestor traversal avoids per-row parent lookups (`ticket.go:767`)
- `buildTreeDisplay` is O(n): single-pass map build + single recursive descent (`printer.go:316-369`)
- WebSocket goroutines cleaned up: `defer hub.remove(client)` and `defer client.close()` (`realtime.go:97`, `chat_ws.go:111`)
- Timers properly stopped on goroutine exit (`chat_ws.go:304, 492`)
- Pagination implemented with LIMIT/OFFSET on ticket list (`ticket.go:694`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `db.SetMaxOpenConns(1)` — single connection for all concurrent requests | Critical | `internal/store/store.go:26-27` | Set to 25 for server mode: `db.SetMaxOpenConns(25); db.SetMaxIdleConns(5)` |
| `messages` table: no index on `from_user_id` or `to_user_id` | High | `internal/store/store.go:867` | `CREATE INDEX idx_messages_from_user_id ON messages(from_user_id)` |
| `goals` table: no index on `project_id` | High | `internal/store/store.go:887` | `CREATE INDEX idx_goals_project_id ON goals(project_id)` |
| `tickets.clone_of` FK column has no index | Medium | `internal/store/store.go:169` | `CREATE INDEX idx_tickets_clone_of ON tickets(clone_of)` |
| Several list endpoints default to LIMIT 1000 — unbounded for large datasets | Low | `internal/store/auth.go:211`, `team.go:110` | Expose pagination params; reduce default LIMIT to 100 |

## Verdict
The connection pool `SetMaxOpenConns(1)` is the dominant performance bottleneck — under any concurrent load it serialises all database access, causing cascading latency. This is likely intentional for SQLite WAL mode but is undocumented and will cause severe throughput degradation. Score drops from 62 to 58 as this finding is now confirmed.

## Changes since last assessment
- `buildTreeDisplay` now called from `runBoard` — confirmed O(n), no performance regression
- Connection pool limit confirmed at 1 (unchanged, newly flagged as critical)
- `messages`/`goals` index gaps carried forward

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Connection pool | Critical | Increase `MaxOpenConns` to 25 for server mode; add comment explaining WAL trade-off |
| `messages` indexes | High | Add FK indexes for `from_user_id`, `to_user_id` |
| `goals` index | High | Add FK index for `project_id` |
| `clone_of` index | Medium | Add index on `tickets.clone_of` |
| Pagination defaults | Low | Reduce default LIMIT from 1000 to 100; document max-page-size |
