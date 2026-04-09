# SDLC Assessment Summary

**Date:** 2026-04-09  
**Project:** ticket / tk  
**Overall Score: 71/100** (was 70, +1)

## Score Table

| # | Category | Previous | Current | Delta |
|---|----------|----------|---------|-------|
| 01 | OpenAPI | 64 | 64 | 0 |
| 02 | Security | 68 | 65 | -3 |
| 03 | InfoSec | 60 | 63 | +3 |
| 04 | Idiomatic Go | 80 | 78 | -2 |
| 05 | Idiomatic JS | 78 | 82 | +4 |
| 06 | DevOps | 81 | 76 | -5 |
| 07 | QA | 76 | 78 | +2 |
| 08 | Tech Lead | 63 | 60 | -3 |
| 09 | Architect | 72 | 68 | -4 |
| 10 | Performance | 62 | 58 | -4 |
| 11 | Database | 68 | 60 | -8 |
| 12 | Tech Writer | 74 | 77 | +3 |
| 13 | Product Owner | 78 | 85 | +7 |
| 14 | Compliance | 74 | 74 | 0 |
| 15 | SRE | 42 | 50 | +8 |
| 16 | UX | 68 | 74 | +6 |
| 17 | New Starter | 83 | 86 | +3 |
| **Overall** | | **70** | **71** | **+1** |

## Score Distribution

| Band | Categories |
|------|-----------|
| 80–89 | Idiomatic JS (82), New Starter (86), Product Owner (85) |
| 70–79 | Idiomatic Go (78), QA (78), Tech Writer (77), DevOps (76), UX (74), Compliance (74), Architect (68→was), Infosec (63→was) |
| 60–69 | OpenAPI (64), Security (65), InfoSec (63), Architect (68), Database (60), Tech Lead (60) |
| 50–59 | Performance (58), SRE (50) |

## What Changed Since Last Assessment

### Improvements
- **Product Owner +7**: TK-222 (`tk board` tree view), `tk init` workflow/role checks implemented; full SPEC feature coverage confirmed; tree display consistent between `tk ls` and `tk board`
- **SRE +8**: Confirmed comprehensive runbooks in `docs/RUNBOOKS.md` (9 scenarios with step-by-step commands); structured slog logging verified; health check at `/api/healthz` with DB connectivity check
- **UX +6**: TK-222 board tree hierarchy; `tk init` feedback reports workflow name, stage count, role count; `buildTreeDisplay` shared between `runList` and `runBoard`
- **Idiomatic JS +4**: `escape()` sanitiser confirmed applied to all 68 `innerHTML` assignments (XSS risk mitigated); `call()` wrapper centralises fetch
- **New Starter +3**: `docs/ONBOARDING.md` added (was absent); binary rename `ticket → tk` propagated to key docs
- **Tech Writer +3**: ONBOARDING.md comprehensive; CLAUDE.md up to date; USER_GUIDE reflects current features
- **QA +2**: Test isolation patterns verified; `libtickettest` contract suite confirmed running against both local and HTTP implementations
- **InfoSec +3**: XSS mitigated via `escape()` sanitiser in all innerHTML assignments

### Regressions
- **Database -8**: `messages` and `goals` tables confirmed as zombie tables (defined but never written to); `tickets.assignee` not cleared on user delete (GDPR gap); connection pool `SetMaxOpenConns(1)` confirmed
- **DevOps -5**: `cmd/tk-test/main.go:49` was still defaulting to `bin/ticket` (fixed this session); Docker image tagged `ticket:` not `tk:`; `golangci-lint` not in CI pipeline
- **Performance -4**: `db.SetMaxOpenConns(1)` at `internal/store/store.go:26-27` serialises ALL concurrent requests — critical for multi-user server deployment
- **Architect -4**: WebSocket broadcast confirmed cross-project (`realtime.go:66-80` sends ALL events to ALL clients regardless of project); multi-tenant data leakage
- **Security -3**: WebSocket cross-project broadcast confirmed; no account lockout on login brute force; `context.Background()` in WS handler breaks cancellation
- **Idiomatic Go -2**: `context.Background()` in `chat_ws.go:168,177` instead of `r.Context()`; 6 silent catch blocks in JS
- **Tech Lead -3**: File size regression (`cmd_ticket.go` at 2,626 lines); `runBoard` and `runList` duplication partially resolved by `buildTreeDisplay` but more remains

## Key Metrics

| Metric | Value |
|--------|-------|
| Go test functions | 312+ |
| Playwright specs | 11 |
| SQLite indexes | 44+ |
| API endpoints | 60+ |
| CLI commands | 60+ |
| Lines in cmd_ticket.go | 2,626 |
| Lines in store.go | 1,680 |
| Lines in index.html | 5,700+ |
| Service interface methods | 108 |
| RUNBOOK scenarios | 9 |

## Top Priority Action Items

### Critical (block production use)
1. **Fix `SetMaxOpenConns(1)`** → set to 25, add `SetMaxIdleConns(5)` (`internal/store/store.go:26-27`)
2. **Add SIGTERM graceful shutdown** → `signal.Notify` + `srv.Shutdown(ctx)` (`cmd/ticket/cmd_setup.go`)
3. **Add HTTP `WriteTimeout`/`ReadTimeout`/`IdleTimeout`** (`internal/server/server.go:34-38`)
4. **Filter WebSocket events by project** → `realtime.go:66-80` cross-project broadcast is a data leak
5. **Protect `/metrics` endpoint** → add auth or bind to loopback only

### High
6. **Clear `assignee` on user delete** → GDPR gap (`internal/store/auth.go`)
7. **Add account lockout on failed login** → brute-force protection (`internal/store/auth.go`)
8. **Fix `context.Background()` in WS handler** → use `r.Context()` (`internal/server/chat_ws.go:168,177`)

### Medium
9. **Remove zombie `messages`/`goals` tables** from schema (`internal/store/store.go:867,887`)
10. **Generate and commit SBOM** → `cyclonedx-gomod mod -output sbom.json`
11. **Add `golangci-lint` to CI pipeline** (`.github/workflows/`)
12. **Add loading spinners** to web UI async operations
13. **Add mobile breakpoints** (640px, 480px) to `web/static/index.html`

### Low
14. **OpenAPI drift**: 7 undocumented endpoints; add `requestBody` schemas for multipart uploads
15. **Authenticate chat WebSocket** before upgrade (currently upgrades before token check)
16. **Fix Docker image name** `ticket:` → `tk:` in `Makefile` and `compose.yaml`

## Cumulative Progress

| Assessment | Score | Delta |
|-----------|-------|-------|
| v1 (original) | 70 | baseline |
| v2 (current) | 71 | +1 |

## Notes
- Binary rename `ticket → tk` is complete in all source files, docs, and Makefile
- `tk init` now checks workflow/stages/roles and prompts to create defaults
- `tk board` now renders epic/story tree hierarchy consistent with `tk ls`
- `cmd/tk-test/main.go` fixed this session (default binary path was `bin/ticket`, now `bin/tk`)
