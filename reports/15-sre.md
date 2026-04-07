# SRE / Observability

**Score: 28/100**

## What is being assessed
Production operational readiness: structured logging, metrics, distributed tracing, health checks, graceful shutdown, backup/restore, runbooks, incident response, alerting, SLOs, and capacity planning. Good SRE posture means the system can be understood, diagnosed, and recovered from in production.

## Methodology
Reviewed all server-side code (`internal/server/`, `cmd/ticket/cmd_setup.go`), deployment files (`compose.yaml`, `Dockerfile`, `deploy/`), and documentation. Checked for signal handling, metrics endpoints, structured logging libraries, and runbook existence.

## Findings

### Passing checks
- Health check endpoint `GET /api/healthz` executes `SELECT 1` and returns version (`internal/server/api.go:100-112`)
- Manual snapshot export/import available (`ticket export`, `ticket import`) with schema version validation
- SQLite WAL mode enabled for concurrent read access (`internal/store/store.go`)
- `busy_timeout: 5000ms` configured to handle lock contention
- Server flags cover port, address, verbose mode, static path
- Watchtower auto-update configured in `deploy/compose.yaml`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No graceful shutdown (no SIGTERM/SIGINT handling) | Critical | `cmd/ticket/cmd_setup.go:1002` | Add `signal.NotifyContext` + `srv.Shutdown(ctx)` with 30s timeout |
| No structured logging — only `fmt.Fprintf` | High | `internal/server/server.go:153-183` | Adopt `slog` (stdlib Go 1.21+) with fields: request_id, method, path, status, duration_ms, user_id |
| No metrics endpoint | High | repo-wide | Add `GET /metrics` returning Prometheus format; track http_requests_total, latency histograms |
| No tracing (OpenTelemetry) | Medium | repo-wide | Add OTEL SDK; create spans for HTTP handlers and DB queries |
| No alerting rules defined | High | repo-wide | Define Prometheus alert rules: 5xx rate >5%, latency p99 >1s, disk <10% |
| No SLOs defined | High | repo-wide | Define: 99.5% availability, p99 <500ms latency, <0.1% error rate |
| No incident response process | High | repo-wide | Create `docs/RUNBOOKS.md` with severity levels, escalation paths, playbooks |
| `MaxOpenConns: 1` — all requests serialize | High | `internal/store/store.go` | Increase to 5-10; document as SQLite concurrency limitation |
| Session `expires_at` column exists but never checked | Medium | `internal/store/store.go:168` | Enforce TTL check in token validation code path |
| No automated backup scheduling | Medium | repo-wide | Add cron/systemd timer for daily `ticket export` with compression |
| No runbooks | Medium | `deploy/README.md` | Create runbooks for: cold start, DB recovery, agent reaper issues, user lockout |
| No liveness vs readiness distinction | Low | `/api/healthz` | Add `/health` (liveness, no DB) separate from `/api/healthz` (readiness) |
| Startup timing not measured; migrations not logged | Low | `internal/store/store.go:105-546` | Log each migration step with duration; alert if startup >30s |

## Verdict
The project is not production-ready from an SRE perspective. The most critical gap is the absence of graceful shutdown — a SIGTERM will abruptly kill all WebSocket connections and in-flight requests. Structured logging and metrics are also entirely absent, making production diagnosis nearly impossible. The snapshot mechanism provides a foundation for backup, but requires operational wrapping.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Graceful shutdown | Critical | `signal.NotifyContext` + `srv.Shutdown(ctx)` + `stopReaper` channel close |
| Structured logging with `slog` | High | Add request_id, user_id, duration_ms to all HTTP log lines |
| Prometheus `/metrics` endpoint | High | Track requests, latency histograms, error counts, WS connections |
| Define and publish SLOs | High | 99.5% availability, p99 <500ms, <0.1% 5xx rate |
| `docs/RUNBOOKS.md` | High | Cover: restart, DB recovery, agent issues, user lockout, high latency |
| Session TTL enforcement | Medium | Check `expires_at` on every token lookup |
| Automated daily backup | Medium | Cron: `ticket export | gzip > backup-$(date +%Y%m%d).json.gz` |
| OpenTelemetry tracing | Medium | Span HTTP handlers and DB queries; export to OTEL collector |
| Increase `MaxOpenConns` | Medium | Set to 5-10; document SQLite concurrency ceiling |
