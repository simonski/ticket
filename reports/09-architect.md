# Architecture

**Score: 68/100** (was 72)

## What is being assessed
Package dependency graph (circular imports), resource bounding (channels, goroutines, connections), WebSocket event scoping, interface abstraction quality, CLI→local vs CLI→remote abstraction, extensibility patterns, and architectural regressions.

## Methodology
Read `libticket/service.go`, `internal/server/api.go`, `internal/server/realtime.go`, `internal/server/chat_ws.go`, `go.mod`. Counted Service interface methods. Checked `liveHub.broadcast()` for project filtering. Reviewed `cmd/ticket/resolve.go`.

## Findings

### Passing checks
- No circular imports; package DAG is clean (`go build ./...` clean)
- CLI→local/remote abstraction is excellent: `cmd/ticket/resolve.go:56-72` uses `libticket.Service` interface, mode determined by `config.ResolveLocation()`, transparent to CLI handlers
- All channels bounded: chat `send: make(chan []byte, 64)`, realtime `send: make(chan []byte, 32)` (`chat_ws.go:96`, `realtime.go:50`)
- Chat process count bounded by `maxConnections` config (`chat_ws.go:190-195`)
- Goroutine lifecycle scoped to connection: `defer client.close()` on all WS handlers (`chat_ws.go:111`)
- Service interface decomposed into 7 sub-interfaces (Auth, User, Agent, Project, Team, Workflow, Ticket) (`libticket/service.go`)
- Rate limiting added at API boundary for auth endpoints (`api.go:14`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `liveHub.broadcast()` sends all events to all clients — no project-level filtering | Critical | `internal/server/realtime.go:66-80` | Create per-project hub instances or filter `broadcast()` by `event.ProjectID` vs user's project list |
| `TicketService` sub-interface has 44 methods — too broad for interface segregation | High | `libticket/service.go:93-137` | Split into `TicketLifecycle`, `TicketComments`, `TicketLabels`, `TicketTime` |
| `LocalService` uses `context.Background()` ~125 times — context doesn't propagate | Medium | `libticket/local.go` | Add `ctx context.Context` to `LocalService` method signatures |
| No plugin/provider abstraction for Chat, Auth, or Notifications | Medium | `internal/server/` | Define `ChatProvider` interface; allows future backends without core changes |

## Verdict
The core CLI abstraction layer is excellent, channels are bounded, and imports are clean. The critical issue is the WebSocket broadcast hub sending all events to all connected clients without project-scoping — a multi-tenant isolation failure that worsens as the system scales. Score drops from 72 to 68 as this issue is now confirmed rather than theoretical.

## Changes since last assessment
- `runBoard` now calls `buildTreeDisplay` — better use of existing architecture (positive)
- `runInitCheckDefaults` added cleanly using existing `libticket.Service` interface (positive)
- WebSocket broadcast gap carries forward from v0.1.737
- `TicketService` interface continues to grow without segregation

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| WebSocket project scoping | Critical | Map `projectID → []*liveClient` in hub; `broadcast(event)` only sends to clients subscribed to `event.ProjectID` |
| Split `TicketService` | High | Extract 4 focused sub-interfaces; reduces mock surface and compile coupling |
| Context propagation | Medium | Pass `ctx` through `LocalService`; enables request-scoped cancellation for slow queries |
| Provider pattern | Medium | Define `ChatProvider` interface (`Send`, `Receive`, `Close`) for the chat sub-system |
