# SRE

**Score: 74/100** (was 73)

## What is being assessed
Operational readiness: health endpoints, metrics, alertability, runbook accuracy, capacity guidance, and whether operators can trust the documented observability story.

## Methodology
Reviewed `internal/server/api_system.go`, `internal/server/server.go`, `cmd/tk/cmd_ticket_health.go`, `docs/SLO.md`, and the current runbook baseline, then refreshed the previous SRE findings against current evidence.

## Findings

### Passing checks
- **The runtime exposes concrete liveness/capacity signals** — `/api/healthz` and authenticated `/metrics` are implemented in the server, not just described in docs (`internal/server/api_system.go:19-80`).
- **The SLO document now matches the real signal set** — it explicitly describes `/metrics`, log-derived alerts, no tracing pipeline, and SQLite capacity ceilings (`docs/SLO.md:25-96`).
- **Ticket health checks now cover more operational context** — the CLI health command expanded to 10 checks spanning ticket, project, SDLC, and stage context (`cmd/tk/cmd_ticket_health.go:54-76`, `cmd/tk/cmd_ticket_health.go:146-157`, `cmd/tk/cmd_ticket_health.go:175-245`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The metrics surface still lacks request-rate, error-rate, and latency counters/histograms | Medium | `internal/server/api_system.go:57-80` | Add request/response counters and latency histograms so the SLOs do not depend on log-derived signals for core API behavior. |
| Alerting guidance exists only as documentation; no alerting config ships with the repository | Medium | `docs/SLO.md:25-74` | Add a checked-in Prometheus/Alertmanager example or equivalent operational config alongside the narrative SLO guidance. |

## Verdict
SRE improved because the docs now tell the truth and the health command is materially richer than before. The next operational step is to turn the current liveness/capacity gauges into a fuller request-observability story and ship actual alert wiring rather than only prose.

## Changes since last assessment
- Reclassified the old docs-vs-runtime observability drift as fixed (`docs/SLO.md:25-96`, `internal/server/api_system.go:19-80`).
- Credited the expanded 10-check health model as an operational readiness improvement (`cmd/tk/cmd_ticket_health.go:175-245`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Limited metrics surface | Medium | Add API request/error/latency metrics to `/metrics`. |
| Alerting only documented, not shipped | Medium | Check in runnable alerting examples or starter configs instead of prose-only guidance. |
