# Architect

**Score: 72/100** (was 68)

## What is being assessed

Package layering, circular dependencies, resource bounding, interface quality, WebSocket/event system, and the new SDLC lifecycle refactor architecture.

## Methodology

Mapped all internal import edges. Read bounded resource declarations. Reviewed `libticket/service.go` composition. Read `realtime.go`, `chat_ws.go`, `live_event.go`. Read all new SDLC store/service/API/CLI code.

## Findings

### Passing checks
- Package DAG is acyclic — no circular imports
- SQLite: `SetMaxOpenConns(1)` + WAL + `busy_timeout=5000` — correct for SQLite single-writer (`store.go:26-27`)
- WebSocket send channels bounded: live=32, chat=64 — slow clients dropped via `default:` (`realtime.go:50,73-76`)
- Rate limiter with sliding window, pruning on each call (`api.go:14`)
- Two bounded background goroutines stopped via `stopReaper` channel (`server.go:25,39`)
- Service interface decomposes into 7 sub-interfaces: `AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `SdlcService`, `TicketService`
- SDLC four-layer chain complete: store -> service -> API -> CLI
- HTTP client parity: `libtickethttp` delegates all 11 `SdlcService` methods to client

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Server handlers call `store.*` directly, bypassing `libticket.Service` — parallel service layers with no shared validation | Medium | `api_sdlc.go` (all handlers) | Route through `LocalService` or document bypass |
| N+1 in `listSdlcStages`: separate `ListSdlcStageRoles` per stage | Medium | `sdlc.go:281-282` | Single JOIN query |
| No contract tests for `AddSdlcStageRole`, `RemoveSdlcStageRole`, `ReorderSdlcStageRoles` | Medium | `libtickettest/contract.go` | Add `stage-role-crud` section |
| `AddSdlcStage` doesn't accept/persist `acceptance_criteria` despite column existing | Low | `sdlc.go:127-137` | Add field to `SdlcStageRequest` and INSERT |
| No `UpdateSdlc` or `UpdateSdlcStage` — stages immutable after creation | Low | `sdlc.go` | Implement across all four layers |
| Rate limiter map grows unboundedly | Low | `ratelimit.go:12` | Add periodic eviction of stale IPs |
| `project set-draft` is a stub | Low | `cmd_project.go:194` | Implement or remove |
| `sharedChatRuntime` is a package-level global | Low | `chat_ws.go:55` | Inject as parameter |
| `libtickethttp` imports `internal/store` for types — structural coupling | Info | `libtickethttp/http.go:11` | Move types to `libticket` |

## Verdict

The SDLC refactor is structurally sound with a complete four-layer chain. Interface decomposition into 7 sub-interfaces is a genuine improvement. Score improves +4 from 68 to 72, held back by the service-layer bypass and new N+1 query.

## Changes since last assessment
- SDLC model: new `Sdlc`, `SdlcStage`, `SdlcWithStages` types; 3 new tables with FK integrity
- Service interface: `SdlcService` (11 methods) added; total now 119 methods
- API: `api_sdlc.go` with 7 route groups; `api_roles.go` updated
- CLI: `cmd_sdlc.go` covers full SDLC management
- Ticket model: `SdlcID`, `SdlcStageID`, `RoleID`, lifecycle auto-advance

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add stage-role contract tests | High | `libtickettest/contract.go` |
| Fix `AddSdlcStage` acceptance_criteria gap | High | Add field to request and INSERT |
| Fix N+1 in `listSdlcStages` | Medium | Single JOIN query |
| Implement `UpdateSdlc`/`UpdateSdlcStage` | Medium | All four layers |
| Implement or remove `project set-draft` | Medium | `cmd_project.go:178` |
| Fix rate limiter memory leak | Medium | Periodic eviction |
