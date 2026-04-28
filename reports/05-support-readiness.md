# Support Readiness

**Score: 73/100** (was 73)

## Mission
Ensure users and operators can diagnose failures quickly and recover safely.

## Review objective
Assess runbooks, logs, health checks, support docs, and recovery paths.

## Inputs reviewed
- `SUPPORT.md`
- `docs/RUNBOOKS.md`
- `docs/SLO.md`
- `internal/server/api_system.go`
- `internal/server/server.go`

## Findings

### Passing checks
- Runbooks cover cold start, crash restart, recovery, backups, lockouts, latency, websocket, and disk-full scenarios (`docs/RUNBOOKS.md:8-19`).
- SLO docs define availability, latency, and error-rate targets (`docs/SLO.md:3-24`).
- Request logs carry path/status/duration/request ID (`internal/server/server.go:548-552`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Health check is too shallow for production triage. | Medium | Load balancers and support cannot see chat capacity or degraded subsystems. | `internal/server/api_system.go:19-31` | Add optional readiness details for chat capacity and recent subsystem errors. |
| Runbook first-run guidance conflicts with bootstrap defaults. | High | Support may advise insecure or contradictory setup steps. | `docs/RUNBOOKS.md:40-50`, `deploy/entrypoint.sh:12-16` | Rewrite after secure bootstrap behavior lands. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| sre | Health/readiness design affects alerting. | Readiness payload proposal. |
| tech-writer | Runbooks need bootstrap refresh. | Updated first-run runbook. |

## Verdict
Support documentation is broad, but production diagnostics are still shallow. First-run support guidance must be corrected with the deploy-default fix.

## Changes since last assessment
- SLO documentation is present and explicit.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Shallow readiness | Medium | Extend health/readiness checks. | support-readiness |
