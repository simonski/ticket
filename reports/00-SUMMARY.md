# SDLC Assessment Summary

**Date:** 2026-04-10
**Project:** ticket / tk
**Overall Score: 71/100** (was 71, +0)

## Score Table

| # | Category | Previous | Current | Delta |
|---|----------|----------|---------|-------|
| 01 | OpenAPI | 64 | 68 | +4 |
| 02 | Security | 65 | 65 | 0 |
| 03 | InfoSec | 63 | 72 | +9 |
| 04 | Idiomatic Go | 78 | 76 | -2 |
| 05 | Idiomatic JS | 82 | 81 | -1 |
| 06 | DevOps | 76 | 82 | +6 |
| 07 | QA | 78 | 76 | -2 |
| 08 | Tech Lead | 60 | 56 | -4 |
| 09 | Architect | 68 | 72 | +4 |
| 10 | Performance | 58 | 61 | +3 |
| 11 | Database | 60 | 72 | +12 |
| 12 | Tech Writer | 77 | 79 | +2 |
| 13 | Product Owner | 85 | 78 | -7 |
| 14 | Compliance | 74 | 72 | -2 |
| 15 | SRE | 50 | 52 | +2 |
| 16 | UX | 74 | 76 | +2 |
| 17 | New Starter | 86 | 84 | -2 |
| **Overall** | | **71** | **71** | **0** |

## Score Distribution

| Band | Categories |
|------|-----------|
| 80-89 | DevOps (82), New Starter (84), Idiomatic JS (81) |
| 70-79 | Idiomatic Go (76), QA (76), Tech Writer (79), UX (76), Product Owner (78), InfoSec (72), Architect (72), Database (72), Compliance (72) |
| 60-69 | OpenAPI (68), Security (65), Performance (61) |
| 50-59 | Tech Lead (56), SRE (52) |

## What Changed Since Last Assessment

The `refactor/sdlc-lifecycle` branch landed Phase 2 of the SDLC lifecycle refactor: 81 files changed, +4,469 / -2,883 lines. The refactor renamed `workflow` to `sdlc` throughout, added SDLC/Stage/Role/StageRole entities across all four layers (store, service, API, CLI), introduced configurable lifecycle processes, and added `tk next`/`tk previous` for ticket advancement.

### Improvements (+9 categories improved)
- **Database +12**: Recursive CTE for ancestor walk, batch comment fetch, new SDLC tables with proper FK/indexes, zombie `agents` table migrated and dropped
- **InfoSec +9**: Argon2id password hashing confirmed, rate limiting on auth endpoints, WebSocket origin validation, comprehensive `escape()` XSS coverage in SPA
- **DevOps +6**: `gosec` + `govulncheck` in CI, SBOM generation in release pipeline, `.dockerignore` added, compose resource limits added
- **OpenAPI +4**: 30+ new SDLC operations with typed schemas, lifecycle fields on Ticket/Project schemas, consistent error responses
- **Architect +4**: SDLC four-layer chain complete (store->service->API->CLI), Service interface decomposed into 7 sub-interfaces, clean package DAG
- **Performance +3**: New SDLC indexes, transaction on role reordering, bounded WebSocket channels
- **Tech Writer +2**: SPEC.md/USER_GUIDE.md/QUICKSTART.md updated for new lifecycle model, docs/LIFECYCLE.md is comprehensive design spec
- **UX +2**: SDLC management view added to SPA, undo stack with keyboard shortcut
- **SRE +2**: `stopReaper` channel correctly plumbed to background goroutines

### Regressions (-7 categories regressed)
- **Product Owner -7**: LIFECYCLE.md spec defines commands that don't exist (`tk fail`, `stage-get`, `stage-update`, SDLC-scoped roles); `project set-draft` is a stub; CLI command names don't match spec
- **Tech Lead -4**: Mega-files grew (cmd_ticket.go +138 to 2,826; store/ticket.go +228 to 1,988); TUI `ticketStates` bug (`"open"` is not a valid state); Service interface at 119 methods with naming inconsistency
- **Idiomatic Go -2**: New gofmt failures in SDLC files; missing transactions in `ReorderSdlcStages`/`DeleteSdlc`/`ImportSdlc`; unused symbols introduced
- **QA -2**: Stage-role feature untested at contract/API/CLI levels; URL mismatch between client and server means stage-role is broken in remote mode
- **Compliance -2**: `TICKET_SESSION_EXPIRY_DAYS` documented but not implemented; verbose logging can expose plaintext passwords
- **Idiomatic JS -1**: `allTickets` ReferenceError bug confirmed; unescaped `s.sort_order` in innerHTML
- **New Starter -2**: docs/LIFECYCLE.md not in reading order; ONBOARDING.md has no SDLC concepts section; DESIGN.md references non-existent file

## Key Metrics

| Metric | Previous | Current |
|--------|----------|---------|
| Go test functions | 312+ | 411 |
| Playwright specs | 11 | 11 |
| SQLite indexes | 44+ | 50+ |
| API endpoints | 60+ | 83 |
| CLI commands | 60+ | 70+ |
| Service interface methods | 108 | 119 |
| Lines in cmd_ticket.go | 2,626 | 2,826 |
| Lines in store/ticket.go | 1,680 | 1,988 |
| Lines in tui/model.go | — | 3,151 |
| Lines in index.html | 5,700+ | 6,080 |
| RUNBOOK scenarios | 9 | 9 |

## Top Priority Action Items

### Critical (blocks feature correctness)
1. **Fix SDLC API URL mismatch** — SPA calls `/api/sdlc` (singular), server registers `/api/sdlcs` (plural). Every SDLC UI operation returns 404. Fix: 8 find-replace in `web/static/index.html`
2. **Fix stage-role remote mode routing** — Client sends `POST /api/sdlcs/{id}/stages/{sid}/roles` but server registers flat `/api/sdlcs/stages/roles/`. Stage-role CRUD is broken in HTTP mode. Fix: align client URLs in `internal/client/client.go:1689-1713` with server routes
3. **Fix TUI `ticketStates` bug** — `"open"` is not a valid store state (should be `"idle"`); `ticketStages` uses wrong names. Fix: `internal/tui/model.go:281-282`
4. **Add `tk fail` shortcut** — Specified in LIFECYCLE.md but missing from `main.go` switch. Fix: add `case "fail"` at `cmd/tk/main.go:185-195`

### High (security + data integrity)
5. **Add CSRF protection** — No CSRF middleware on any state-changing endpoint (`internal/server/api.go`)
6. **Add account lockout** — No protection after repeated failed logins (`internal/store/auth.go`)
7. **Wrap `ReorderSdlcStages`/`DeleteSdlc`/`ImportSdlc` in transactions** — Partial failures leave inconsistent state (`internal/store/sdlc.go`)
8. **Add HTTP WriteTimeout/ReadTimeout/IdleTimeout** — Only `ReadHeaderTimeout` is set (`internal/server/server.go:34-38`)
9. **Add SIGTERM graceful shutdown** — Process cannot drain connections on deploy (`cmd/tk/cmd_setup.go:1143`)

### Medium (spec compliance + quality)
10. **Implement missing LIFECYCLE.md commands** — `stage-get`, `stage-update`, SDLC-scoped role commands, `project set-draft`
11. **Add stage-role contract tests** — `AddSdlcStageRole`/`RemoveSdlcStageRole`/`ReorderSdlcStageRoles` have zero test coverage
12. **Fix openapi.yaml paths** — `/api/sdlc` → `/api/sdlcs` throughout; align stage-role URL structure
13. **Fix docs/DESIGN.md** — Broken reference to `TICKET_LIFECYCLE_SPEC.md`; stale `open` field in ticket model
14. **Authenticate `/metrics` endpoint** — Exposes goroutine counts, memory stats, user/ticket counts to anyone
15. **Fix N+1 queries** — `listSdlcStages` per-stage role fetch; `recalculateParentLifecycle` per-child stage-order lookup

### Low
16. **Run `gofmt -w`** on new SDLC files so `make lint` passes
17. **Clear `tickets.assignee` on user delete** — GDPR gap (`internal/store/auth.go`)
18. **Add SDLC concepts to docs/ONBOARDING.md** and add LIFECYCLE.md to reading order
19. **Fix `allTickets` ReferenceError** in `web/static/index.html:5074,5173`
20. **Make `#app-status` visible** in SPA — undo/error messages are written to a hidden element

## Cumulative Progress

| Assessment | Date | Score | Delta |
|-----------|------|-------|-------|
| v1 (original) | 2026-04-09 | 70 | baseline |
| v2 | 2026-04-09 | 71 | +1 |
| v3 (current) | 2026-04-10 | 71 | 0 |

## Notes
- The SDLC lifecycle refactor is structurally sound — the four-layer chain (store->service->API->CLI) is complete for core operations and the interface decomposition is well factored
- The flat overall score masks significant movement: 9 categories improved while 7 regressed, reflecting that the refactor added substantial new infrastructure but also introduced new bugs and spec-implementation gaps
- The three critical bugs (API URL mismatch, stage-role routing, TUI invalid state) are all low-effort fixes that would unblock the feature
- Test count grew from 312 to 411 (+32%) — good coverage expansion, but the gap on stage-role testing allowed a routing bug to ship
