# SDLC Assessment Summary

**Date:** 2026-04-28  
**Project:** ticket / tk  
**Overall score:** 71/100 (was 74, -3)  
**Assessment scope:** full repository review across CLI, HTTP API, web UI, SQLite store, tests, CI/CD, deployment artifacts, security/privacy docs, runbooks, and existing SDLC reports.

## Score table

| Role | Previous | Current | Delta |
|------|----------|---------|-------|
| product-manager | 77 | 78 | +1 |
| user-researcher | 73 | 76 | +3 |
| ux-review | 74 | 76 | +2 |
| accessibility | 73 | 75 | +2 |
| support-readiness | 73 | 73 | 0 |
| systems-architect | 74 | 74 | 0 |
| api-architect | 74 | 74 | 0 |
| domain-designer | 74 | 76 | +2 |
| tech-lead | 74 | 70 | -4 |
| backend-engineer | 73 | 76 | +3 |
| frontend-engineer | 73 | 74 | +1 |
| code-reviewer | 73 | 68 | -5 |
| maintainer | 73 | 68 | -5 |
| security-engineer | 73 | 66 | -7 |
| application-security | 71 | 67 | -4 |
| database-engineer | 74 | 78 | +4 |
| privacy-and-compliance | 71 | 74 | +3 |
| supply-chain | 72 | 64 | -8 |
| qa-architect | 76 | 78 | +2 |
| performance-engineer | 72 | 67 | -5 |
| resilience-engineer | 72 | 66 | -6 |
| devops-engineer | 72 | 62 | -10 |
| sre | 72 | 68 | -4 |
| release-manager | 72 | 60 | -12 |
| tech-writer | 73 | 70 | -3 |
| new-starter | 73 | 74 | +1 |
| **Overall** | **74** | **71** | **-3** |

## Score distribution

| Band | Count | Roles |
|------|-------|-------|
| 90+ | 0 | None |
| 80-89 | 0 | None |
| 70-79 | 16 | product-manager, user-researcher, ux-review, accessibility, support-readiness, systems-architect, api-architect, domain-designer, tech-lead, backend-engineer, frontend-engineer, database-engineer, privacy-and-compliance, qa-architect, tech-writer, new-starter |
| Below 70 | 10 | code-reviewer, maintainer, security-engineer, application-security, supply-chain, performance-engineer, resilience-engineer, devops-engineer, sre, release-manager |

## Top systemic risks

1. **The deployment bundle still needs release hardening.** The known default admin password finding is closed, but the compose file still pins mutable `latest` images (`deploy/compose.yaml:3-10`, `deploy/compose.yaml:27-33`).
2. **Release provenance is incomplete.** CI builds and publishes images/releases, but the release path has no signing or attestation step (`.github/workflows/makefile.yaml:79-103`, `Makefile:173-180`).
3. **Trust-boundary hardening remains incomplete.** Request security decisions still honor `X-Forwarded-Proto` without tying it to trusted proxy CIDRs, and chat child processes inherit the full server environment (`internal/server/server.go:668-681`, `internal/server/chat_ws.go:232-237`).
4. **Operational telemetry is useful but shallow.** `/metrics` exposes liveness/count/memory gauges, while SLO docs explicitly defer request latency and 5xx alerting to logs until counters/histograms are added (`internal/server/api_system.go:33-85`, `docs/SLO.md:72-74`).
5. **Some version metadata remains stale outside the OpenAPI contract.** `openapi.yaml` now has a valid `info.version` and a regression guard, but `SPEC.md` and some docs still reference older version metadata (`openapi.yaml:1-5`, `cmd/tk/VERSION:1`, `SPEC.md:1-4`).

## Cross-role contradiction log

| Contradiction | Resolution |
|---------------|------------|
| Some role research treated CSP as missing. | Direct inspection shows CSP headers and nonce injection exist (`internal/server/server.go:255-265`, `internal/server/server.go:425-437`, `internal/server/server_test.go:368-380`). The remaining frontend/security concern is not missing CSP, but ensuring all inline surfaces remain nonce-compatible. |
| The prior report treated OpenAPI validation as closed, while this assessment initially found it broken. | The contract has now been repaired: `openapi.yaml` contains `info.version`, `make validate-openapi` passes, and `TestOpenAPIVersionMatchesBinaryVersion` guards version drift (`openapi.yaml:1-5`, `internal/server/api_test.go`). |
| Older reports flagged `site2` prefix validation drift. | Current inspection shows both main web and site2 enforce 1-5 uppercase prefixes (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`). That specific UI drift is closed. |
| Performance research claimed no SLO document. | `docs/SLO.md` exists and defines availability/latency/error targets (`docs/SLO.md:1-24`). The real gap is that request counters and latency histograms are not implemented yet (`docs/SLO.md:72-74`). |

## What changed since last assessment

- Go coverage gates pass, including `internal/server` at 70.0% against its raised 70% threshold (`Makefile:135-157`; assessment command `TICKET_FAST_HASH=1 make test-go-cover`).
- The main and site2 project-prefix inputs are now aligned with the store rule (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`).
- CLI noun UX has uncommitted improvements in progress, increasing usability but raising release hygiene concerns until committed and rerun through full gates (`cmd/tk/namespace_helpers.go`, `cmd/tk/main_test.go`).
- OpenAPI was repaired after the assessment: the contract is syntactically valid again, matches `cmd/tk/VERSION`, and is covered by a regression test (`openapi.yaml:1-5`, `internal/server/api_test.go`).

## Cumulative improvement table

| Area | Original | Prior | Current | Evidence |
|------|----------|-------|---------|----------|
| Overall SDLC score | 72 | 74 | 71 | `reports/sdlc/history.json:5-47`, this report |
| Go coverage gates | failing in early baseline | passing | passing | `Makefile:135-157`; `TICKET_FAST_HASH=1 make test-go-cover` |
| OpenAPI validity | malformed | valid | valid | `openapi.yaml:1-5`; `make validate-openapi` |
| Browser-test coverage | present | present | 12 specs | `TESTING.md:138-153`; metric command |
| Operational docs | partial | improved | SLO and runbooks present | `docs/SLO.md:1-24`, `docs/RUNBOOKS.md:1-19` |
| Deploy defaults | insecure | insecure | insecure | `deploy/compose.yaml:3-10`, `deploy/entrypoint.sh:12-16` |

## Key delivery metrics

| Metric | Current | Evidence |
|--------|---------|----------|
| Go test files | 41 | assessment command `find . -name '*_test.go' | wc -l` |
| Playwright spec files | 12 | assessment command `find tests/playwright -name '*.spec.js' | wc -l` |
| GitHub Actions workflows | 1 | assessment command `find .github/workflows ... | wc -l` |
| Tracked files | 267 | assessment command `git ls-files | wc -l` |
| Coverage gates | passing | `TICKET_FAST_HASH=1 make test-go-cover` |
| Raised server coverage gate | `internal/server` 70.0% / 70% | coverage command output |
| OpenAPI validation | passing | `make validate-openapi` |
| Release signing/attestation | absent | `.github/workflows/makefile.yaml:79-103` |
| Health endpoint | DB ping and version only | `internal/server/api_system.go:19-31` |
| Metrics endpoint | authenticated coarse gauges | `internal/server/api_system.go:33-85` |

## Prioritised action register

| Priority | Finding | Owner role | Dependency notes |
|----------|---------|------------|------------------|
| P0 | Pin/sign deploy artifacts and avoid mutable production refs | release-manager | Blocks production-grade release provenance |
| P1 | Pin deploy image tags or document explicit production pinning; add signing/attestation | supply-chain | Depends on release workflow changes |
| P1 | Gate `X-Forwarded-Proto` trust on configured proxy CIDRs and filter chat child-process env | application-security | Depends on deployment topology choices |
| P1 | Finish version metadata sync beyond the OpenAPI contract | api-architect | Depends on docs/spec refresh |
| P2 | Add request latency/error counters or histograms to match SLOs | sre | Depends on metrics design |
| P2 | Commit or isolate in-progress CLI noun UX changes before release | release-manager | Depends on owner decision for current worktree |
| P3 | Refresh docs that still reference old bootstrap/admin flows and current version | tech-writer | Depends on deploy-default decision |

## Verdict

The project remains functionally promising and has strong Go coverage, useful docs, hardened containers, and good local/remote architecture. The public API contract blocker is fixed, but the production compose path still ships insecure defaults and release provenance/trust-boundary controls are still incomplete. The release recommendation remains **no-go for production** until the remaining P0 deploy-default issue is fixed and rerun through CI-equivalent gates.
