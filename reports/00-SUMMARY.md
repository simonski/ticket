# SDLC Assessment Summary

**Date:** 2026-04-12
**Project:** ticket / tk
**Overall Score: 79/100** (was 80, -1)

## Score Table

| # | Category | Previous | Current | Delta |
|---|----------|----------|---------|-------|
| 01 | OpenAPI | 78 | 78 | 0 |
| 02 | Security | 82 | 82 | 0 |
| 03 | InfoSec | 78 | 78 | 0 |
| 04 | Idiomatic Go | 79 | 79 | 0 |
| 05 | Idiomatic JS | 88 | 88 | 0 |
| 06 | DevOps | 87 | 87 | 0 |
| 07 | QA | 79 | 75 | -4 |
| 08 | Tech Lead | 59 | 56 | -3 |
| 09 | Architect | 79 | 79 | 0 |
| 10 | Performance | 76 | 75 | -1 |
| 11 | Database | 80 | 80 | 0 |
| 12 | Tech Writer | 82 | 81 | -1 |
| 13 | Product Owner | 84 | 82 | -2 |
| 14 | Compliance | 80 | 79 | -1 |
| 15 | SRE | 74 | 73 | -1 |
| 16 | UX | 80 | 78 | -2 |
| 17 | New Starter | 89 | 87 | -2 |
| **Overall** |  | **80** | **79** | **-1** |

## Score Distribution

| Band | Categories |
|------|-----------|
| 85-89 | Idiomatic JS (88), DevOps (87), New Starter (87) |
| 80-84 | Security (82), Database (80), Tech Writer (81), Product Owner (82) |
| 75-79 | OpenAPI (78), InfoSec (78), Idiomatic Go (79), QA (75), Architect (79), Performance (75), Compliance (79), UX (78) |
| 70-74 | SRE (73) |
| 55-59 | Tech Lead (56) |

## What Changed Since Last Assessment

Since the previous report set, the codebase picked up useful CLI workflow improvements in `cmd/tk`: `tk ls` now filters on genuinely open tickets instead of raw `complete` state, `tk get` now reports child totals/open/closed counts, and epic child rendering keeps closed children visible while visually de-emphasising them by state (`cmd/tk/cmd_ticket.go:39-55`, `cmd/tk/cmd_ticket.go:397-402`, `cmd/tk/cmd_ticket.go:644-658`, `cmd/tk/printer.go:262-307`). Those changes improved day-to-day ergonomics, but the fresh SDLC pass also uncovered real regressions and stale findings that the previous summary understated.

### Key movements

- **QA -4**: the review confirmed the coverage gate is still broken and still lacks API/CLI coverage for several SDLC flows, despite the recent CLI regression tests added for `tk ls`/`tk get` (`cmd/tk/main_test.go:1441-1495`, `cmd/tk/main_test.go:1855-1880`, `cmd/tk/main_test.go:4193-4203`)
- **Tech Lead -3**: two previously flagged critical TUI state/stage mismatches remain unresolved, and new SA4010-style append bugs were identified
- **Performance -1**: unbounded history queries and full-project scans in clone/delete paths remain unresolved
- **SRE -1**: `docs/SLO.md` still references request counters/histograms that `/metrics` does not expose
- **Tech Writer / Product Owner / Compliance / UX / New Starter** all slipped slightly as the updated review caught stale docs, unresolved workflow correctness bugs, verbose response logging gaps, and web UX/accessibility debt
- **Architect / Database / OpenAPI / Security / InfoSec / Idiomatic Go / Idiomatic JS / DevOps** remained broadly stable with no fresh regressions of consequence

## Key Metrics

| Metric | Previous | Current |
|--------|----------|---------|
| Go test functions | 410 | 414 |
| Playwright specs | 11 | 11 |
| OpenAPI operationIds | 113 | 113 |
| Lines in cmd_ticket.go | 2,041 | 2,060 |
| Lines in tui/model.go | 1,846 | 1,846 |
| Lines in index.html | 6,183 | 6,183 |
| Files failing gofmt | 34 | 32 |
| Binary version | 0.1.769 | 0.1.774 |

## Top Priority Action Items

### Critical
1. **Fix `tk ready` / `tk notready` inversion** — the CLI still maps `ready` to `draft=true`, which is backwards for a core workflow (`cmd/tk/main.go:253-256`)
2. **Fix TUI state/stage constants** — `ticketStates` still includes `"open"` and `ticketStages` still uses non-store names, causing invalid lifecycle values (`internal/tui/model_forms.go:217-218`)
3. **Fix coverage gate failures** — the updated QA review still flags threshold failures in core packages and missing API/CLI coverage for SDLC stage-role flows

### High
4. **Implement real request counters/histograms or remove the non-working alerting rules** — `docs/SLO.md` references metrics `/metrics` does not export
5. **Repair doc drift** — prefix length, `tk init` flags, `TICKET_HOME`, and default DB path documentation are stale across the main docs set
6. **Run `gofmt -w ./...` and fix the remaining static-analysis issues** — 32 files still fail formatting and critical SA4010 findings remain

### Medium
7. **Finish SDLC-scoped role CRUD under `tk sdlc`** — `role-list` is scoped, but add/get/update/rm still are not
8. **Complete dark-mode and modal keyboard handling in the SPA** — hardcoded light colors and missing Escape handling remain the top UX issues
9. **Suppress verbose response-body logging for auth and agent endpoints** — current behavior can still leak tokens or generated passwords in verbose mode

## Cumulative Progress

| Assessment | Date | Score | Delta |
|-----------|------|-------|-------|
| v1 (original) | 2026-04-09 | 70 | baseline |
| v2 | 2026-04-09 | 71 | +1 |
| v3 | 2026-04-10 | 71 | 0 |
| v4 | 2026-04-12 | 80 | +9 |
| v5 (current) | 2026-04-12 | 79 | -1 |

## Notes

- The project is still materially stronger than the v1-v3 baseline, especially in security, DevOps, OpenAPI alignment, and architectural coherence.
- This refresh is the first report set to surface a **mixed picture**: CLI ergonomics improved, but quality gates, stale documentation, and some long-standing correctness issues are still dragging down the overall readiness score.
- The biggest disconnect remains between documented intent and implemented behavior: `tk ready`, TUI lifecycle constants, and SLO alerting rules all advertise workflows the current code does not fully honor.
