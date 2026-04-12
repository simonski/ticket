# Architect

**Score: 79/100** (was 79)

## What is being assessed

Package dependency DAG, circular dependencies, resource bounding, plugin/provider patterns, event/notification system, interface abstraction quality, the libticket + libtickethttp merge, four-layer chain (store->service->API->CLI), and static SDLC/role seed files via embed.

## Methodology

Mapped all internal import edges using `go list`. Verified acyclicity via `go vet`. Read all bounded resource declarations across store, server, and chat subsystems. Reviewed `libticket/service.go` interface composition, `internal/static/embed.go` seed system, `realtime.go`/`chat_ws.go`/`live_event.go` event system, `ratelimit.go` sliding window, and the four-layer chain from store through CLI.

## Findings

### Passing checks
- **Package DAG is acyclic** -- `go vet ./...` clean, no circular imports
- **Clean dependency layering**: `store` -> `password` (only downward); `libticket` -> `store`, `client`, `config`; `server` -> `store`, `config`, `web`; `cmd/tk` -> everything except `client` (correct: CLI resolves via `libticket`)
- **config and password are leaf packages** -- zero internal imports, ideal for unit testing
- **SQLite correctly bounded**: `SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`, WAL mode, `busy_timeout=5000` (`store.go:26-27,32-36`)
- **HTTP server timeouts configured**: `ReadHeaderTimeout: 30s`, `ReadTimeout: 60s`, `IdleTimeout: 120s`; `WriteTimeout` intentionally omitted for WebSocket support with inline documentation (`server.go:38-45`)
- **Request body size limit**: 1 MB via `http.MaxBytesReader` on all non-GET/HEAD requests (`server.go:178-186`)
- **WebSocket send channels bounded**: live=32 (`realtime.go:50`), chat=64; slow clients dropped via `default:` select
- **Rate limiter with sliding window and stale-key eviction** on every call -- no unbounded growth (`ratelimit.go:42-49`)
- **Chat connections bounded**: configurable `MaxConnections` (default 2) and `MaxDurationMin` (default 3) via `app_settings` table (`settings.go:10-11`, `chat_ws.go:190,602`)
- **Two background goroutines** (agent reaper, retention purge) stopped cleanly via `stopReaper` channel and `Shutdown()` (`server.go:49-50,193-196`)
- **Service interface decomposed into 7 sub-interfaces**: `AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `SdlcService`, `TicketService` -- 116 methods total (`service.go`)
- **libticket + libtickethttp successfully merged**: single `libticket` package with `LocalService` (SQLite) and `HTTPService` (HTTP client) both satisfying `Service` interface; `libtickettest` contract tests also merged into `libticket_test` package
- **Contract test Factory pattern** covers both implementations via `RunServiceContractTests(t, factory, opts)` (`contract_test.go:21`)
- **Stage-role contract tests added**: `stage-role-crud` section covers `AddSdlcStageRole`, `RemoveSdlcStageRole`, `ReorderSdlcStageRoles` (`contract_test.go:853`)
- **N+1 in listSdlcStages fixed**: batch-loads all roles via `listSdlcStageRolesBatch` (`sdlc.go:354-356`)
- **SDLC four-layer chain complete**: store (`sdlc.go`) -> service (`local.go`, `http.go`) -> API (`api_sdlc.go`) -> CLI (`cmd_sdlc.go`)
- **Static SDLC/role seed files via embed**: `internal/static/embed.go` with `//go:embed roles/*.md` and `//go:embed sdlc/*.md`; 20 role files + 2 SDLC files parsed from markdown frontmatter; `SeedDatabase` function passed as `SeedFunc` to `store.Init` (`static/embed.go:80-116`)
- **`UpdateSdlcStage` implemented** with `acceptance_criteria` support (`sdlc.go:148`)
- **`project set-draft` implemented** -- no longer a stub (`cmd_project.go:178-204`)
- **Event/notification system**: `liveHub` broadcasts typed events (`ticket_created`, `ticket_updated`, `ticket_deleted`, `project_created`, `project_updated`, `project_users_updated`) to WebSocket clients with optional project-scoped filtering (`realtime.go`, `live_event.go`)
- **Retention purge**: expired sessions and old history events purged periodically via `runRetentionPurge`, configurable via `TICKET_HISTORY_RETENTION_DAYS` env var (`server.go:84-115`)

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Server API handlers call `store.*` directly, bypassing `libticket.Service` -- two parallel code paths with no shared validation | Medium | `api_tickets.go`, `api_sdlc.go`, all `api_*.go` | Route server handlers through `LocalService` or explicitly document that the API layer is a separate implementation |
| No `UpdateSdlc` (rename/update SDLC metadata) -- only `UpdateSdlcStage` exists | Medium | `store/sdlc.go`, `service.go` | Add `UpdateSdlc` across all four layers |
| `sharedChatRuntime` is a package-level global singleton | Low | `chat_ws.go:55` | Inject as parameter into `router` for testability |
| `libticket.HTTPService` still imports `internal/store` for types (structural coupling across package boundary) | Low | `libticket/http.go:11`, `libticket/types.go:3` | Migrate shared types to `libticket` package; use type aliases to maintain backward compatibility |
| 116-method `Service` interface is large -- implementors must satisfy all 7 sub-interfaces | Low | `libticket/service.go:155-163` | Consider accepting sub-interfaces at call sites where full Service is not needed |
| `getSdlcStageRow` still calls `ListSdlcStageRoles` per-row (N+1 for single-stage lookup path) | Low | `store/sdlc.go:327` | Acceptable for single-row path but inconsistent with batch pattern |
| No plugin/provider/registry pattern -- all capabilities are compile-time | Info | Entire codebase | Acceptable for current scope; consider if extensibility becomes a requirement |

## Verdict

The architecture posture remains broadly stable in this pass. The earlier libticket/libtickethttp consolidation, N+1 fixes, rate-limiter eviction, stage-role contract coverage, acceptance-criteria support, and embedded static seeds all remain in place, and the main remaining concern is still the intentional dual-path design where API handlers talk directly to store code.

The primary remaining architectural concern is the server API layer calling `store` directly rather than routing through `libticket.Service`, creating two parallel validation paths. This is a deliberate design choice (the server has direct DB access and doesn't need the service abstraction layer) but it means validation logic must be maintained in two places.

No material architectural regression or improvement was identified in this pass.

## Changes since last assessment
- No material architectural changes landed in this review window
- The earlier service/package consolidation and SDLC improvements remain intact
- The server-vs-service dual-path design remains the main architectural tradeoff

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Server API bypasses `libticket.Service` | Medium | Route handlers through `LocalService` or document the dual-path design |
| No `UpdateSdlc` for SDLC metadata | Medium | Implement across store, service, API, CLI |
| `sharedChatRuntime` global | Low | Inject into router as constructor parameter |
| `libticket` imports `internal/store` types | Low | Migrate shared domain types to `libticket`; type-alias in store |
| 116-method Service interface | Low | Accept sub-interfaces at call sites where appropriate |
