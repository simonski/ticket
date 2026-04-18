# Resilience Engineer

**Score: 74/100** (was 74)

## Mission
Ensure the system degrades predictably and recovers cleanly instead of failing in confusing or cascading ways.

## Review objective
Evaluate timeouts, panic recovery, shutdown behavior, periodic maintenance, and failure isolation.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/analyse.go`
- `docs/RUNBOOKS.md`
- `compose.yaml`

## Findings

### Passing checks
- The server sets read/header/idle timeouts and adds request-level timeout middleware for normal HTTP paths (`internal/server/server.go:40-48`, `internal/server/server.go:225-239`).
- Panic recovery is centralized and graceful shutdown stops background maintenance loops before shutting the HTTP server down (`internal/server/server.go:241-250`, `internal/server/server.go:301-305`).
- Runbooks cover crash recovery, DB recovery, backup/restore, lockout, latency, and disk-full scenarios (`docs/RUNBOOKS.md:22-217`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| WebSocket and chat paths are explicitly exempted from request timeout middleware. | Medium | Long-lived sessions have weaker boundedness and can tie up resources unless their own controls remain sufficient. | `internal/server/server.go:229-237` | Add or document explicit per-connection timeout/idle policies for WebSocket/chat flows. |
| The deployment compose file has a healthcheck but no startup dependency or automated recovery validation beyond container restart. | Low | Some failure modes are documented, but the packaged deployment still offers limited built-in recovery orchestration. | `compose.yaml:8-29`, `docs/RUNBOOKS.md:61-84` | Document expected supervisor/restart behavior and add deployment smoke verification guidance. |
| Backup/restore is documented, but there is no evidence in the normal CI path that restore drills are exercised automatically. | Low | A recovery plan can drift from reality between incidents. | `docs/RUNBOOKS.md:126-184`, `.github/workflows/makefile.yaml:22-45` | Add a scheduled restore verification or a dedicated recovery smoke test. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| sre | Recovery and timeout gaps become on-call problems first. | WebSocket boundedness and restore-drill checklist. |
| qa-architect | Restore confidence needs executable proof, not only docs. | Recovery test candidate list. |
| devops-engineer | Deployment recovery behavior is partly packaging, not just code. | Container health/restart expectations. |

## Verdict
The server has the right basic resilience habits—timeouts, panic recovery, shutdown, and runbooks are all present. The weaker area is long-lived connection boundedness and recovery proof: the system explains how to recover, but it does not yet prove enough of that path automatically.

## Changes since last assessment
- Resilience confidence improved because the current code clearly shows graceful shutdown and scheduled purge/reaper behavior, not just doc intent (`internal/server/server.go:51-53`, `internal/server/server.go:301-305`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Unbounded WebSocket/chat duration | Medium | Define explicit idle/max-duration enforcement for long-lived connections. | resilience-engineer |
| Limited packaged recovery automation | Low | Document and test deployment recovery expectations. | devops-engineer |
| Restore drills not proven | Low | Add automated recovery verification. | qa-architect |
