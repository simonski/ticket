# SDLC Assessment Summary

**Date:** 2026-04-18  
**Project:** ticket / tk  
**Overall Score:** 75/100 **(was 80, -5)**  
**Assessment scope:** README, user/admin docs, Go services, SQLite store, web UI, Playwright/Go test setup, CI/CD, container/deploy config, and the prior `reports/` baseline.

## Score Table

| # | Role | Previous | Current | Delta |
|---|------|----------|---------|-------|
| 01 | product-manager | 85 | 83 | -2 |
| 02 | user-researcher | 79 | 70 | -9 |
| 03 | ux-review | 79 | 76 | -3 |
| 04 | accessibility | 79 | 72 | -7 |
| 05 | support-readiness | 74 | 74 | 0 |
| 06 | systems-architect | 79 | 78 | -1 |
| 07 | api-architect | 74 | 68 | -6 |
| 08 | domain-designer | 79 | 75 | -4 |
| 09 | tech-lead | 56 | 58 | +2 |
| 10 | backend-engineer | 79 | 78 | -1 |
| 11 | frontend-engineer | 88 | 75 | -13 |
| 12 | code-reviewer | 77 | 74 | -3 |
| 13 | maintainer | 56 | 64 | +8 |
| 14 | security-engineer | 82 | 81 | -1 |
| 15 | application-security | 80 | 80 | 0 |
| 16 | database-engineer | 80 | 82 | +2 |
| 17 | privacy-and-compliance | 79 | 74 | -5 |
| 18 | supply-chain | 80 | 73 | -7 |
| 19 | qa-architect | 77 | 78 | +1 |
| 20 | performance-engineer | 75 | 72 | -3 |
| 21 | resilience-engineer | 74 | 74 | 0 |
| 22 | devops-engineer | 88 | 84 | -4 |
| 23 | sre | 74 | 76 | +2 |
| 24 | release-manager | 80 | 70 | -10 |
| 25 | tech-writer | 81 | 77 | -4 |
| 26 | new-starter | 88 | 84 | -4 |
| **Overall** |  | **80** | **75** | **-5** |

## Score Distribution

| Band | Roles |
|------|-------|
| 90+ | None |
| 80-89 | product-manager (83), security-engineer (81), application-security (80), database-engineer (82), devops-engineer (84), new-starter (84) |
| 70-79 | user-researcher (70), ux-review (76), accessibility (72), support-readiness (74), systems-architect (78), domain-designer (75), backend-engineer (78), frontend-engineer (75), code-reviewer (74), privacy-and-compliance (74), supply-chain (73), qa-architect (78), performance-engineer (72), resilience-engineer (74), sre (76), tech-writer (77), release-manager (70) |
| Below 70 | api-architect (68), tech-lead (58), maintainer (64) |

## Top Systemic Risks

1. **Contract drift is now material** — `openapi.yaml` is malformed at the top-level version field, which breaks standard tooling before deeper contract checks can even run (`openapi.yaml:1-10`).
2. **The web UI still diverges from backend and docs in important workflows** — the project prefix field allows 8 characters while the documented rule is 1-5 uppercase characters, and the ticket modal still exposes a generic manual health control while the CLI uses a 10-check health model (`web/static/index.html:1610-1612`, `docs/LIFECYCLE.md:14-15`, `USER_GUIDE.md:64-67`, `web/static/index.html:1897-1905`, `cmd/tk/cmd_ticket_health.go:218-246`).
3. **Release integrity is weaker than the runtime security posture** — CI runs linting, `gosec`, and `govulncheck`, but release artifacts and images are not signed or attested (`.github/workflows/makefile.yaml:22-38`, `.github/workflows/makefile.yaml:86-100`, `Makefile:171-215`).
4. **Docs drift is visible to both contributors and operators** — docs still say there are 11 Playwright specs even though the repo now has 12, the privacy doc still carries an older version, and README still recommends `make build` even though onboarding warns against it for day-to-day work (`CLAUDE.md:27`, `TESTING.md:124-126`, `docs/PRIVACY.md:4-5`, `README.md:64-71`, `docs/ONBOARDING.md:111-117`).
5. **Local/browser quality gates are brittle** — Playwright uses fixed local ports in both configs, which creates avoidable false negatives when those ports are already in use (`playwright.config.js:7-15`, `playwright.site2.config.js:7-15`).

## Cross-role Contradiction Log

| Contradiction | Resolution |
|---------------|------------|
| A QA/resilience pass claimed request correlation IDs were missing. | Rejected after direct review: `loggingHandler` generates or forwards `X-Request-ID` and logs it with method, path, status, and duration (`internal/server/server.go:402-445`). |
| An architecture pass suggested TUI lifecycle constants were wrong. | Narrowed: lifecycle states and stages use `store` constants, but the TUI ticket type picker is incomplete and omits valid backend ticket types such as `story`, `feature`, and `idea` (`internal/tui/model_forms.go:265-268`, `internal/store/ticket.go:1620-1627`). |
| A security/data pass suggested short encryption keys were padded. | Rejected after direct review: the encryption key helper now enforces a minimum of 32 bytes and errors otherwise (`internal/store/encrypt.go:17-31`). |
| A QA/performance pass described ticket history reads as unbounded. | Rejected after direct review: `ListHistoryEvents` normalizes limit/offset and applies `LIMIT ? OFFSET ?` (`internal/store/activity.go:46-57`). |

## What Changed Since Last Assessment

- The positive CLI changes from the prior pass remain in place: multi-ID draft/undraft support and richer ticket detail rendering are still present (`cmd/tk/cmd_ticket_lifecycle.go:262-323`, `cmd/tk/printer.go:256-290`).
- The contract surface is still degraded by the malformed OpenAPI version header, so the most visible previous blocker remains unresolved (`openapi.yaml:1-10`).
- The doc drift surface is now larger: specialist docs still report 11 Playwright specs while the repo contains 12, and the privacy policy still carries a stale product version (`CLAUDE.md:27`, `TESTING.md:124-126`, `docs/PRIVACY.md:4-5`).
- Observability confidence improved because the current code clearly includes request-scoped correlation IDs and structured request logging (`internal/server/server.go:402-445`).

## Cumulative Improvement

| Assessment | Date | Score | Delta |
|-----------|------|-------|-------|
| v1 | 2026-04-09 | 70 | baseline |
| v2 | 2026-04-09 | 71 | +1 |
| v3 | 2026-04-10 | 71 | 0 |
| v4 | 2026-04-12 | 80 | +9 |
| v5 | 2026-04-12 | 79 | -1 |
| v6 | 2026-04-13 | 80 | +1 |
| v7 | 2026-04-18 | 75 | -5 |

## Key Delivery Metrics

| Metric | Current | Evidence |
|--------|---------|----------|
| Go test files | 40 | repository count during assessment |
| Playwright spec files | 12 | `tests/playwright/*.spec.js` repository count during assessment |
| Go coverage gate packages | 6 | `Makefile:105-127` |
| Go coverage gates | passing | assessment run against `Makefile:105-127` |
| OpenAPI YAML parses | no | `openapi.yaml:1-10` |
| Request correlation IDs | present | `internal/server/server.go:402-445` |
| Health/metrics endpoints | 2 | `internal/server/api_system.go:19-85` |
| SHA-pinned workflow actions | present | `.github/workflows/makefile.yaml:15-17`, `.github/workflows/makefile.yaml:43`, `.github/workflows/makefile.yaml:56`, `.github/workflows/makefile.yaml:63`, `.github/workflows/makefile.yaml:87` |
| Browser test fixed ports | 2 configs | `playwright.config.js:7-15`, `playwright.site2.config.js:7-15` |
| Artifact/image signatures | none | `Makefile:171-215`, `.github/workflows/makefile.yaml:86-100` |

## Prioritized Action Register

| Priority | Finding | Owner role | Dependency notes |
|----------|---------|------------|------------------|
| P1 | Repair `openapi.yaml` and add spec validation to CI | api-architect | Depends on devops-engineer to add a validation step in workflow |
| P2 | Add release artifact signing and container/image attestations | supply-chain | Depends on devops-engineer and release-manager agreeing the signing path |
| P3 | Fix web prefix validation and align the web health control with the 10-check backend model | frontend-engineer | Depends on product-manager and tech-writer to settle expected wording |
| P4 | Replace click-only `div` menus and non-announced status regions with accessible components | accessibility | Depends on ux-review and frontend-engineer for implementation details |
| P5 | Remove fixed Playwright ports or randomize them in local CI-style runs | qa-architect | Depends on devops-engineer because the configs are part of the build/test surface |
| P6 | Reduce ownership hotspots in `web/static/index.html`, `cmd/tk/main.go`, and `internal/client/client.go` | tech-lead | Depends on maintainer to sequence refactors without destabilizing behavior |
| P7 | Publish a README-level help/troubleshooting entry point | support-readiness | Depends on tech-writer and sre |
| P8 | Tighten trusted-proxy handling for `X-Forwarded-Proto` and secure cookie decisions | security-engineer | Depends on devops-engineer to define expected reverse proxy topology |
