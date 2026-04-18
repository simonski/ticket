# SRE

**Score: 76/100** (was 74)

## Mission
Protect operability in production through telemetry, alerts, runbooks, and credible recovery procedures.

## Review objective
Assess whether the current server can be observed, diagnosed, and recovered by an on-call operator.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/api_system.go`
- `docs/SLO.md`
- `docs/RUNBOOKS.md`
- `compose.yaml`

## Findings

### Passing checks
- The server emits structured request logs with `request_id`, method, path, status, and duration, and it propagates or creates `X-Request-ID` consistently (`internal/server/server.go:402-445`).
- The product exposes both `/api/healthz` and authenticated `/metrics`, and the SLO doc ties those surfaces to concrete alert examples (`internal/server/api_system.go:19-85`, `docs/SLO.md:25-74`).
- The repo includes real runbooks for deployment, crash recovery, DB repair, backups, lockouts, latency, and disk exhaustion (`docs/RUNBOOKS.md:22-217`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The SLO document explicitly relies on structured logs for request latency and 5xx monitoring because the metrics surface does not expose them directly. | Medium | On-call diagnosis and alerting remain weaker and more tool-dependent than they should be. | `internal/server/api_system.go:33-85`, `docs/SLO.md:27-29`, `docs/SLO.md:72-74` | Add first-class request counters/histograms and error metrics. |
| There is no tracing pipeline, and the docs state that plainly. | Low | Multi-step failures outside the single-process happy path will be harder to reconstruct. | `docs/SLO.md:76-86` | Decide whether tracing is intentionally out of scope or add a minimal path for future expansion. |
| Backup and restore guidance exists, but restore validation is still operationally manual. | Low | Recovery confidence can decay until the first real incident forces a restore. | `docs/RUNBOOKS.md:126-184` | Add periodic restore drills or a scripted recovery validation. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| performance-engineer | Latency instrumentation is shared observability/performance work. | Metrics expansion requirements. |
| resilience-engineer | Recovery drills and long-lived connection controls overlap heavily. | Incident scenario checklist. |
| support-readiness | Better telemetry improves support triage directly. | Which user-visible failures need clearer operator signals? |

## Verdict
This is an operable system, not a black box: logs, health, metrics, SLOs, and runbooks all exist. The biggest SRE gap is depth rather than absence—the observability surface is useful, but still too shallow for high-confidence latency and recovery management.

## Changes since last assessment
- Operability scores slightly higher because direct code review confirmed structured request IDs/logging and an authenticated metrics surface rather than only doc claims (`internal/server/server.go:402-445`, `internal/server/api_system.go:33-85`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing request/error metrics | Medium | Add HTTP counters and latency histograms. | sre |
| No tracing stance beyond docs | Low | Make tracing scope explicit or add a minimal implementation path. | systems-architect |
| Restore drills not automated | Low | Add periodic recovery verification. | sre |
