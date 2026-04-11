# Idiomatic Go

**Score: 76/100** (was 78)

## What is being assessed

Idiomatic Go patterns across the full codebase, with focus on the SDLC lifecycle refactor in `internal/store/sdlc.go`, `role.go`, `lifecycle.go`, `internal/server/api_sdlc.go`, `api_roles.go`, `api_types.go`, `libticket/local.go`, `libtickethttp/http.go`, and `libtickettest/contract.go`.

## Methodology

Static analysis via `gofmt -l` and `golangci-lint run ./...`. Manual review of error handling, context propagation, concurrency, naming, test patterns, transaction safety, and struct alignment in all new SDLC files.

## Findings

### Passing checks
- Well-defined sentinel errors (`ErrTicketNotFound`, `ErrForbidden`, etc.) used consistently with `errors.Is()`
- Error wrapping with `%w` in new code (`sdlc.go:217`)
- Contract tests cover new SDLC operations — `libtickettest/contract.go` has `sdlc-crud-and-stages` and `role-crud`
- `t.Helper()` and `t.Cleanup()` used in new tests
- Table-driven tests for lifecycle logic — `lifecycle_test.go`
- `sync.Once` for WebSocket close idempotency — `realtime.go`
- `sync.RWMutex` in liveHub — read-lock on broadcast, write-lock on add/remove
- `defer tx.Rollback()` after `BeginTx` — `role.go:ReorderSdlcStageRoles`
- Service interface composition via sub-interfaces (`SdlcService`, `RoleService`, etc.)

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| New SDLC files introduce `gofmt` failures (misaligned struct tags) | High | `sdlc.go:46`, `api_types.go:24,69`, `contract.go:780` | Run `gofmt -w` |
| Unused symbols: `getRoleByTitle`, `wfOffset`/`wfStageCursor`/`wfInStages` | High | `role_test.go:113`, `tui/model.go:400-403` | Remove or use |
| Missing transactions in `ReorderSdlcStages`, `DeleteSdlc`, `ImportSdlc` | Medium | `sdlc.go` (lines ~103, ~145, ~200) | Wrap in `BeginTx`/`Commit` like `ReorderSdlcStageRoles` |
| `context.Background()` in 130 `LocalService` calls | Medium | `libticket/local.go` | Long-term: add `ctx` to `Service` interface |
| `context.Background()` in WS message handler | Medium | `chat_ws.go:168,177` | Derive from WS session lifecycle |
| SDLC mutation sub-routes use `requireUser` not `requireAdmin` | Medium | `api_sdlc.go:171-221` | Require admin for SDLC mutations |
| `err == sql.ErrNoRows` instead of `errors.Is` (5 instances) | Medium | `ticket.go:741,799`, `settings.go:22,45`, `lifecycle.go:99` | Use `errors.Is` |
| Silently discarded errors in new SDLC code | Low | `sdlc.go:253,282`, `role.go:159` | Log or propagate errors |
| Store errors returned as 400 instead of 500 | Low | `api_sdlc.go`, `api_roles.go` | Return 500 for DB errors |
| Non-standard flat URL routing for stage-roles | Low | `api_sdlc.go:68` | Document or refactor to RESTful nesting |

### Pre-existing issues (unchanged)
- `noctx` violations in `client.go:1523`, `client_util.go:99,142`
- `paramTypeCombine` gocritic flags throughout `client.go`

## Verdict

The SDLC refactor is structurally sound — interface decomposition, contract tests, and store patterns are largely correct. Score drops 2 points due to new gofmt failures, missing transactions (inconsistent with `ReorderSdlcStageRoles` in the same codebase), and unused symbols.

## Changes since last assessment
- New SDLC service interface methods with contract test coverage (+1)
- `r.Context()` correctly used in all new HTTP handlers (+0.5)
- gofmt failures in new files (-1.5)
- Missing transactions in ReorderSdlcStages/DeleteSdlc/ImportSdlc (-1)
- Unused symbols introduced (-1)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| gofmt failures | High | Run `gofmt -w` on affected files |
| Missing transactions | High | Wrap in `BeginTx`/`Commit` |
| Unused symbols | High | Remove `getRoleByTitle`, `wfOffset` etc. |
| SDLC mutation auth | Medium | Require admin |
| `err == sql.ErrNoRows` | Medium | Use `errors.Is` |
| Store errors as 400 | Low | Return 500 for DB errors |
