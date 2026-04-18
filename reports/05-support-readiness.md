# Support Readiness

**Score: 74/100** (was 74)

## Mission
Protect diagnosability and recovery so operators and maintainers can help users quickly when workflows fail.

## Review objective
Assess whether current docs, logs, and recovery surfaces provide enough information to triage user-facing failures.

## Inputs reviewed
- `docs/RUNBOOKS.md`
- `docs/SLO.md`
- `README.md`
- `internal/server/server.go`
- `playwright.config.js`
- `playwright.site2.config.js`

## Findings

### Passing checks
- The server emits request-scoped logs with request ID, method, path, status, and duration for API calls (`internal/server/server.go:402-445`).
- The repo has real runbook and SLO surfaces for backup/restore, health checks, and alerting guidance (`docs/RUNBOOKS.md:22-183`, `docs/SLO.md:1-74`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The main README still lacks a visible help/reporting path. | Medium | The most obvious recovery surface for new users does not tell them how to get unstuck. | `README.md:1-237` | Add a short “Getting help” section linking to issue filing and operator docs. |
| Recovery guidance is spread across runbooks and onboarding, but there is no dedicated troubleshooting surface for common day-to-day failures. | Medium | Operators must search multiple docs or inspect code to resolve basic errors. | `docs/RUNBOOKS.md:1-260`, `docs/ONBOARDING.md:211-235` | Publish a compact troubleshooting guide for auth, DB, and browser-test failures. |
| Browser test supportability is brittle because both Playwright configs use fixed local ports. | Low | A busy developer machine produces opaque false failures instead of actionable product regressions. | `playwright.config.js:7-15`, `playwright.site2.config.js:7-15` | Move to dynamic/free ports or document an explicit conflict check. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | Missing support/troubleshooting entry points are documentation work first. | README help section + troubleshooting doc. |
| sre | Operational recovery guidance should match observed telemetry. | Which failures deserve first-class runbook entries? |
| qa-architect | Port-conflict failures should not look like product failures. | Test-env hardening plan. |

## Verdict
The codebase is supportable once you know where to look, mainly because the request logging and runbooks are solid. The weakness is discoverability: first-line support paths are not obvious from the repo front door, and local test failures can still look more mysterious than they should.

## Changes since last assessment
- Supportability on the server side is stronger than some prior reports suggested because request-scoped correlation IDs are clearly present in the current code (`internal/server/server.go:402-445`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| No README help path | Medium | Add a repo-front-door support section. | tech-writer |
| No focused troubleshooting doc | Medium | Publish targeted triage steps for common failures. | support-readiness |
| Fixed Playwright ports | Low | Make browser-test setup self-healing or self-diagnosing. | qa-architect |
