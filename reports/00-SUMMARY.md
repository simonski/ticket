# SDLC Assessment Summary

**Date:** 2026-04-13
**Project:** ticket / tk
**Overall Score: 80/100** (was 79, +1)

## Score Table

| # | Category | Previous | Current | Delta |
|---|----------|----------|---------|-------|
| 01 | OpenAPI | 78 | 74 | -4 |
| 02 | Security | 82 | 82 | 0 |
| 03 | InfoSec | 78 | 80 | +2 |
| 04 | Idiomatic Go | 79 | 79 | 0 |
| 05 | Idiomatic JS | 88 | 88 | 0 |
| 06 | DevOps | 87 | 88 | +1 |
| 07 | QA | 75 | 77 | +2 |
| 08 | Tech Lead | 56 | 56 | 0 |
| 09 | Architect | 79 | 79 | 0 |
| 10 | Performance | 75 | 75 | 0 |
| 11 | Database | 80 | 80 | 0 |
| 12 | Tech Writer | 81 | 81 | 0 |
| 13 | Product Owner | 82 | 85 | +3 |
| 14 | Compliance | 79 | 79 | 0 |
| 15 | SRE | 73 | 74 | +1 |
| 16 | UX | 78 | 79 | +1 |
| 17 | New Starter | 87 | 88 | +1 |
| **Overall** |  | **79** | **80** | **+1** |

## Score Distribution

| Band | Categories |
|------|------------|
| 90+ | None |
| 80-89 | Security (82), InfoSec (80), Idiomatic JS (88), DevOps (88), Database (80), Tech Writer (81), Product Owner (85), New Starter (88) |
| 70-79 | OpenAPI (74), Idiomatic Go (79), QA (77), Architect (79), Performance (75), Compliance (79), SRE (74), UX (79) |
| 60-69 | None |
| 50-59 | Tech Lead (56) |

## What Changed Since Last Assessment

Recent user-facing work materially improved the product surface: `tk draft` / `tk undraft` now support multiple IDs (`cmd/tk/cmd_ticket_lifecycle.go:262-323`), `tk get` detail columns and child counts are aligned and regression-tested (`cmd/tk/printer.go:253-290`, `cmd/tk/main_test.go:2167-2201`), `tk skill` is now a documented first-class command (`cmd/tk/cmd_setup.go:48-66`, `README.md:205`, `USER_GUIDE.md:37-43`), and ticket health expanded from 4 to 10 checks with project/SDLC/stage context (`cmd/tk/cmd_ticket_health.go:54-76`, `cmd/tk/cmd_ticket_health.go:175-245`).

The main regression uncovered in this refresh is `openapi.yaml`: the new endpoints are documented, but the top-level `info.version` entry is malformed as a bare `.1.775`, so the file is no longer valid YAML (`openapi.yaml:1-10`). Smaller doc/UI drift remains in `docs/PRIVACY.md`, `docs/ONBOARDING.md`, and the SPA health/prefix controls.

## Key Metrics

| Metric | Previous | Current | Evidence |
|--------|----------|---------|----------|
| Go test functions | 414 | 486 | repository-wide `*_test.go` count on 2026-04-13 |
| Playwright specs | 11 | 11 | `tests/playwright/*.spec.*` |
| OpenAPI operationIds | 113 | 119 | `grep -c '^\s*operationId:' openapi.yaml` |
| OpenAPI YAML parses | yes | no | `openapi.yaml:1-10` |
| Health check criteria | 4 | 10 | `cmd/tk/cmd_ticket_health.go:218-245` |
| Draft/undraft target count | 1 | many | `cmd/tk/cmd_ticket_lifecycle.go:275-323` |
| `cmd/tk/cmd_ticket.go` lines | 2,060 | 2,187 | current wc |
| `cmd/tk/cmd_ticket_health.go` lines | 440 | 541 | current wc |
| `web/static/index.html` lines | 6,183 | 6,287 | current wc |
| Binary version | 0.1.774 | 0.1.790 | `cmd/tk/VERSION` |

## Cumulative Improvement

| Assessment | Date | Score | Delta |
|-----------|------|-------|-------|
| v1 (original) | 2026-04-09 | 70 | baseline |
| v2 | 2026-04-09 | 71 | +1 |
| v3 | 2026-04-10 | 71 | 0 |
| v4 | 2026-04-12 | 80 | +9 |
| v5 | 2026-04-12 | 79 | -1 |
| v6 (current) | 2026-04-13 | 80 | +1 |

## Prioritized Actions

| Priority | Action | Evidence | Why it matters |
|----------|--------|----------|----------------|
| 1 | Repair the malformed OpenAPI header and add a spec validation check to CI | `openapi.yaml:1-10` | The spec currently cannot be parsed by standard tooling, which undermines docs, SDK generation, and drift checking. |
| 2 | Replace full-project child scans in CLI/store paths with targeted child queries | `cmd/tk/cmd_ticket.go:728-737`, `internal/store/ticket.go:1864-1870`, `internal/store/ticket.go:1885-1892` | The same scalability smell appears in interactive read, clone, and delete flows. |
| 3 | Finish the remaining doc drift cleanup in privacy/onboarding materials | `docs/PRIVACY.md:4-5`, `docs/PRIVACY.md:113-114`, `docs/ONBOARDING.md:23-47` | New commands are documented in README/USER_GUIDE, but the specialist docs still lag. |
| 4 | Align the web UI with the expanded health model and backend prefix validation | `web/static/index.html:1274-1275`, `web/static/index.html:1495-1503`, `cmd/tk/cmd_ticket_health.go:218-245`, `docs/LIFECYCLE.md:14-15` | The main UX now trails the CLI in two visible workflow areas. |
