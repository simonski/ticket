# Performance

**Score: 61/100** (was 58)

## What is being assessed

N+1 queries, connection pooling, unbounded resources, goroutine leaks, pagination coverage, query instrumentation, and SDLC refactor performance impact.

## Methodology

Read all store files for N+1 patterns and missing pagination. Read `store.go` for connection settings. Read `realtime.go`, `chat_ws.go`, `server.go` for goroutine/buffer patterns. Read API handlers for pagination.

## Findings

### Passing checks
- SQLite WAL mode + `busy_timeout=5000` — `store.go:32,36`
- 30+ indexes covering hot columns — `store.go`
- Recursive CTE for ancestor chain — `ticket.go:1046`
- Batch comment fetch via `IN` clause — `ticket.go:1200`
- WebSocket send channels bounded (32/64) with drop-on-full — `realtime.go:50,74`
- Reaper goroutines stopped via channel — `server.go:70,108`
- New SDLC indexes added — `store.go:481-483,652`
- `ReorderSdlcStageRoles` wrapped in transaction — `role.go:183`

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| N+1: `GetSdlcStageOrder` per child in `recalculateParentLifecycle` | High | `ticket.go:1293,1312` | Batch fetch with `IN (...)` |
| N+1: `ListSdlcStageRoles` per stage in `listSdlcStages` | High | `sdlc.go:282` | Single JOIN query |
| N+1: `GetTicket` in nested loops in `resolveBySequenceNumber` | Medium | `ticket.go:1029-1041` | Single `IN (...)` query |
| `SetMaxOpenConns(1)` serialises all reads | Medium | `store.go:26-27` | Consider raising for WAL readers |
| `ReorderSdlcStages` issues UPDATEs without transaction | Medium | `sdlc.go:156-172` | Wrap in `BeginTx`/`Commit` |
| Missing `ReadTimeout`/`WriteTimeout`/`IdleTimeout` | Medium | `server.go:34-38` | Add timeouts |
| Two separate writes per WebSocket frame | Medium | `realtime.go:267-277` | Combine into single buffer write |
| Chat heartbeat goroutine never stopped | Low | `chat_ws.go:484-504` | Close `heartbeatStop` on shutdown |
| Unbounded list queries (users, agents, roles, teams, sdlcs) | Low | Various store files | Add pagination |
| `ListHistoryEvents` has no limit parameter | Low | `activity.go:46` | Add limit param |
| No query timing instrumentation | Info | — | Add driver hooks or middleware |

## Verdict

Score improves 58 to 61. SDLC refactor added correct indexes and transaction on role reordering but introduced two new N+1 patterns on hot paths. `SetMaxOpenConns(1)` remains.

## Changes since last assessment
- New SDLC indexes added (+)
- Transaction on role reordering (+)
- Two new N+1 patterns (stage-role per stage, stage-order per child) (-)
- `ReorderSdlcStages` without transaction (-)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix `listSdlcStages` N+1 | High | Single JOIN query |
| Fix `recalculateParentLifecycle` N+1 | High | Batch `IN (...)` |
| Wrap `ReorderSdlcStages` in transaction | Medium | `BeginTx`/`Commit` |
| Add HTTP timeouts | Medium | Write/Read/IdleTimeout |
| Fix WebSocket double-write | Medium | Single buffer write |
| Add pagination to list queries | Low | LIMIT/OFFSET support |
