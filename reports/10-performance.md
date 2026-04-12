# Performance

**Score: 75/100** (was 76)

## What is being assessed

Query efficiency, connection usage, pagination, bounded resources, buffer growth, websocket/chat scaling, and whether the current implementation avoids obvious full-scan or unbounded-result traps.

## Methodology

Reviewed `internal/store/`, `internal/server/`, `internal/client/`, and the current performance report baseline. Checked query paths, list endpoints, timeout configuration, resource caps, and the known hotspots in ticket clone/delete and history retrieval.

## Findings

### Passing checks
- **The earlier SDLC N+1 fixes are still in place** — batch stage-role loading and batch stage-order lookup remain the default paths (`internal/store/sdlc.go`, `internal/store/lifecycle.go`)
- **HTTP timeouts and body limits remain configured** — read/header/idle timeouts and the 1MB body cap are still enforced (`internal/server/server.go:41-45`, `internal/server/server.go:178-186`)
- **Realtime buffers are bounded** — websocket send channels and chat session limits still constrain in-memory fan-out (`internal/server/realtime.go`, `internal/server/chat_ws.go`)
- **Project ticket listing still supports a caller-provided limit** — `TicketListParams` keeps a limit path for normal ticket lists (`internal/store/ticket.go`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| SQLite is still forced through `SetMaxOpenConns(1)` | Medium | `internal/store/store.go:26-27` | Re-evaluate read concurrency under WAL mode or explicitly document this as a hard scaling constraint |
| `ListHistoryEvents` still has no limit parameter | Medium | `internal/store/activity.go:46-67` | Add a default limit and explicit pagination |
| `cloneTicketRecursive` still loads the whole project to find children | Medium | `internal/store/ticket.go:1806` | Replace the full-project fetch with a targeted `WHERE parent_id = ?` query |
| `DeleteTicket` still loads the whole project to check for children | Medium | `internal/store/ticket.go:1827` | Replace the full-project fetch with an existence check on `parent_id` |
| Several list paths remain unbounded | Low | `internal/store/sdlc.go`, `internal/store/story.go`, `internal/store/label.go` | Add default caps/offsets to SDLC, story, and label list operations |
| `loggingResponseWriter` still buffers full bodies | Low | `internal/server/server.go:215-229` | Cap captured response-body size instead of buffering whole payloads |

## Verdict

Performance slipped slightly because the same medium-severity hotspots are still present and now stand out more clearly against the already-fixed SDLC query work. The system is not broadly slow by design, but it still carries a few avoidable full-scan and unbounded-result paths that will hurt first as data volume grows.

## Changes since last assessment
- 2026-04-12 — TK-203 — commit `67b0af3` documented the single-node SQLite scaling ceiling and capacity assumptions in `docs/SLO.md`
- 2026-04-12 — TK-205 — commit `108ee1f` verified clone-path child loading already uses `WHERE parent_id = ?` instead of a whole-project scan
- 2026-04-12 — TK-206 — commit `108ee1f` changed child existence checks to `SELECT ... WHERE parent_id = ? LIMIT 1`
- 2026-04-12 — TK-208 — commit `67b0af3` capped buffered request/response body logging in `internal/server/server.go`
- The previously fixed SDLC batch-loading improvements remain intact
- The older full-project scan and unbounded-history issues remain unresolved

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Unbounded ticket history | Medium | Add limit/pagination to `ListHistoryEvents` |
| Unbounded SDLC/story/label lists | Low | Add consistent default caps and optional offsets |
