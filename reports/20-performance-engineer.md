# Performance Engineer

**Score: 72/100** (was 75)

## Mission
Protect latency, throughput, and bounded resource usage as the system and user base grow.

## Review objective
Check whether the current architecture, DB access, and telemetry can support predictable performance.

## Inputs reviewed
- `internal/store/store.go`
- `internal/store/activity.go`
- `internal/server/api_system.go`
- `internal/server/server.go`
- `docs/SLO.md`

## Findings

### Passing checks
- SQLite is configured for WAL mode with a busy timeout, which is the right starting point for a single-writer local/server model (`internal/store/store.go:21-53`).
- History listing is paginated instead of loading unbounded event streams by default (`internal/store/activity.go:46-57`).
- The repo has at least a documented latency and capacity stance rather than leaving performance expectations entirely implicit (`docs/SLO.md:10-17`, `docs/SLO.md:88-97`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The DB connection pool is intentionally capped at one open and one idle connection. | Medium | The single-server model has a hard concurrency ceiling, and that limit is easy to hit before the product outgrows the docs. | `internal/store/store.go:27-28`, `docs/SLO.md:95-97` | Make this scaling limit explicit in operator docs and watch backlog/latency metrics against it. |
| `/metrics` exposes only coarse gauges, while the SLO doc admits latency and 5xx alerting still depend on logs. | Medium | Capacity and latency problems will be harder to detect and quantify than they should be. | `internal/server/api_system.go:33-85`, `docs/SLO.md:27-29`, `docs/SLO.md:72-74` | Add request counters/histograms and explicit DB timing instrumentation. |
| Metrics still count open tickets using the legacy `open` column instead of the richer lifecycle fields. | Low | Performance and product dashboards can drift from the true workflow state model. | `internal/server/api_system.go:46-49`, `internal/store/store.go:730-740` | Revisit whether this aggregate should derive from the modern lifecycle state instead. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| database-engineer | The main scale ceiling is still the SQLite operational model. | Query/connection risk list. |
| sre | Telemetry gaps are operational performance gaps. | Metrics expansion plan. |
| systems-architect | The single-connection ceiling is an architecture decision, not just a tuning issue. | When does the current model stop fitting? |

## Verdict
The current performance posture is acceptable for the documented single-server target, but it is close-coupled to that assumption. The largest gap is observability: the repo knows some of its limits, but it does not yet measure enough to manage them confidently.

## Changes since last assessment
- The performance story is clearer because `docs/SLO.md` now states the SQLite scaling ceiling explicitly, but the operational instrumentation still lags behind that clarity (`docs/SLO.md:95-97`, `internal/server/api_system.go:33-85`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Single-connection ceiling | Medium | Document and monitor the SQLite concurrency limit explicitly. | performance-engineer |
| Sparse request telemetry | Medium | Add request counters/histograms and DB timing metrics. | sre |
| Legacy `open` metric source | Low | Align reporting metrics with the primary lifecycle model. | database-engineer |
