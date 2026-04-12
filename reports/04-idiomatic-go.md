# Idiomatic Go

**Score: 79/100** (was 79)

## What is being assessed

Idiomatic Go patterns across the full codebase: error handling, context propagation, concurrency, package organisation, interface design, naming, gofmt compliance, transaction safety, and unused symbols. Special attention to changes since last review: libticket/libtickethttp merge, tk init refactor with -sdlc/-prefix/-name/-git flags, static SDLC/role seeds via embed, and 1-letter project prefix support.

## Methodology

Static analysis via `gofmt -l`, `go vet ./...`, and manual code review. Checked all 34 packages for formatting, error wrapping, context usage, interface size, DB connection patterns, concurrency primitives, and dead code.

## Findings

### Passing checks

- `go vet ./...` passes cleanly (0 warnings)
- No panics in production code
- All `fmt.Errorf` calls use `%w` for error wrapping (0 instances of `%v` for errors)
- `errors.Is()` used consistently; the previous `err == sql.ErrNoRows` pattern (5 instances) has been eliminated
- `context.Context` is always the first parameter in store functions
- Sentinel errors properly defined: `ErrTicketHasChildren`, `ErrProjectNotFound`, `ErrLabelNotFound`, etc.
- Previously missing transactions in `ReorderSdlcStages`, `DeleteSdlc`, `ImportSdlc` now all use `BeginTx`/`defer Rollback()`/`Commit()` correctly
- `defer tx.Rollback()` used consistently after `BeginTx` across all transactional code
- Service interface properly decomposed into 7 sub-interfaces (`AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `SdlcService`, `TicketService`)
- `sync.RWMutex` in liveHub, `sync.Once` for WebSocket close, `sync.Mutex` for rate limiter -- all correct
- Goroutines (8 total) properly scoped with context cancellation or cleanup
- `main()` delegates to `run()` returning error -- clean testable pattern
- `os.Exit` only in `main()` -- not scattered through library code
- Embedded static SDLC/role seeds via `//go:embed` with clean parsing in `internal/static/embed.go`
- Package-level compiled regexps (`stageHeadingRe`, `orderRe`, `roleRefRe`, `projectPrefixPattern`, `colorRegexp`)
- Contract tests via `libtickettest` pattern verifying both LocalService and HTTPService against same suite
- SDLC mutation endpoints now correctly use `requireAdmin` (previously flagged as `requireUser`)
- `libtickethttp` successfully merged into `libticket` package -- cleaner package tree
- `getRoleByTitle` unused symbol from previous review has been removed

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 34 files fail `gofmt` (637 changed lines) | High | cmd/tk, internal/client, internal/store, internal/tui, libticket, internal/server, internal/config | Run `gofmt -w ./...` -- primarily struct tag alignment and extra blank lines |
| DB opened and closed per method call (105 times in `libticket/local.go`, 104 in `internal/client/client.go`) | Medium | `libticket/local.go`, `internal/client/client.go` | Hold a `*sql.DB` on the struct; open once at construction, close on shutdown |
| `context.Background()` used ~288 times in non-test code | Medium | `cmd/tk/`, `libticket/local.go`, `internal/server/` | Service interface methods lack `context.Context` parameter; long-term: add ctx to interface |
| 73 silently discarded errors (`, _ :=` pattern) | Medium | `cmd/tk/cmd_setup.go` (20+), `cmd/tk/cmd_user.go`, `cmd/tk/cmd_ticket_health.go` | At minimum log discarded errors; propagate where possible |
| `http.NewRequest` instead of `http.NewRequestWithContext` (3 instances) | Medium | `internal/client/client_util.go:99,142`, `internal/client/client.go:1534` | Use `http.NewRequestWithContext` for cancellation support |
| `http.Get` without context (2 instances) | Medium | `cmd/tk/cmd_setup.go:218,383` | Use `http.NewRequestWithContext` + `client.Do()` |
| Unused struct fields: `wfOffset`, `wfStageCursor`, `wfInStages` | Low | `internal/tui/model.go:123-126` | Remove dead fields |
| `_ = cfg` assigned but immediately discarded | Low | `cmd/tk/main.go:314` | Refactor to not request cfg, or use it |
| `_ = *retry` unused deref | Low | `cmd/tk/cmd_requirement.go:357` | Remove dead code |
| `regexp.MustCompile` compiled inside loop body | Low | `internal/static/embed.go:212` | Hoist to package-level var |
| Composite Service interface has 123 methods | Low | `libticket/service.go` | Sub-interface decomposition helps, but consumers should accept narrowest interface needed |
| Store DB errors returned as HTTP 400 in some handlers | Low | `internal/server/api_sdlc.go:262,282` | Return 500 for non-validation DB errors |
| `SeedDatabase` silently swallows `CreateRole`/`CreateSdlc` errors with `continue` | Low | `internal/static/embed.go:89,101` | Log skipped seeds or check for duplicate-key specifically |

## Verdict

The idiomatic Go posture remains broadly stable in this pass. The earlier transaction fixes, `errors.Is` cleanup, service/package consolidation, and embedded-seed work all remain in place, and the remaining drag is still structural rather than semantic: gofmt drift, per-call DB open/close patterns, missing context propagation, and silent discarded errors.

The score is held back by pervasive gofmt violations (34 files, worsened since last review), the open-close-per-call DB pattern in both `LocalService` and `Client`, missing context propagation through the Service interface, and 73 silently discarded errors. These are structural issues that require coordinated refactoring.

## Changes since last assessment

- No material idiomatic-Go improvements landed in this review window
- The earlier transaction, error-handling, and packaging improvements remain intact
- gofmt drift and silent-error debt still dominate the remaining recommendations

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| gofmt failures (34 files) | High | Run `gofmt -w ./...` immediately |
| DB opened/closed per method call | Medium | Refactor LocalService and Client to hold a persistent `*sql.DB` |
| Service interface lacks context.Context | Medium | Add `ctx context.Context` as first param to all Service methods |
| 73 silently discarded errors | Medium | Audit and handle; at minimum wrap in `log.Printf` |
| `http.NewRequest` without context | Medium | Switch to `http.NewRequestWithContext` |
| Unused TUI fields | Low | Remove `wfOffset`, `wfStageCursor`, `wfInStages` |
| Regexp compiled in loop | Low | Hoist `regexp.MustCompile` at `embed.go:212` to package level |
| Store errors as HTTP 400 | Low | Distinguish validation errors (400) from DB errors (500) |
