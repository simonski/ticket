# Performance Engineer

**Score: 67/100** (was 72)

## Mission
Protect latency, throughput, and bounded resource usage.

## Review objective
Assess hot paths, connection usage, metrics, benchmarks, and capacity assumptions.

## Inputs reviewed
- `internal/store/schema_version.go`
- `internal/server/api_system.go`
- `internal/server/server.go`
- `docs/SLO.md`
- `Makefile`

## Findings

### Passing checks
- SQLite uses a single connection and busy timeout, making the concurrency ceiling explicit (`internal/store/schema_version.go:46-56`).
- HTTP server timeouts protect slow clients (`internal/server/server.go:41-49`).
- SLOs define concrete latency targets (`docs/SLO.md:10-24`).
- `/metrics` exposes liveness, entity counts, goroutines, and memory (`internal/server/api_system.go:33-85`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| No request latency/error counters despite SLOs. | Medium | Operators cannot measure p50/p95/p99 or 5xx targets from metrics. | `docs/SLO.md:72-74`, `internal/server/api_system.go:33-85` | Add request counters and histograms. |
| No benchmark/load target in Makefile. | Medium | Performance regressions lack a repeatable gate. | `Makefile:118-171` | Add benchmark or smoke-load target for ticket list/create and websocket capacity. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| sre | Metrics must support alerting. | Metrics design. |
| database-engineer | SQLite ceiling needs proof. | Concurrent write test. |

## Verdict
Resource choices are bounded and documented, but performance proof remains immature. SLOs are aspirational until request metrics exist.

## Changes since last assessment
- `docs/SLO.md` exists; metrics remain shallow.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing latency metrics | Medium | Add Prometheus request counters/histograms. | performance-engineer |
