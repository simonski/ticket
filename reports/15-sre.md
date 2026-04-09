# SRE / Observability

**Score: 42/100** (was 44)

## What is being assessed
Production operational readiness: graceful shutdown (SIGTERM handling, connection draining), health/readiness endpoint quality, metrics (Prometheus), structured logging, alerting rules, backup/restore procedures, runbooks, SLOs, capacity planning, and security hardening of observability endpoints.

## Methodology
Reviewed `internal/server/server.go`, `cmd/ticket/cmd_setup.go` (`runServer`), `internal/server/api_system.go`, `compose.yaml`, `Dockerfile`, and `docs/RUNBOOKS.md`. Searched the entire codebase for `prometheus`, `slog`, `logrus`, `zap`, `signal`, `Shutdown`, and `SIGTERM`. Cross-checked findings against previous assessment recommendations.

## Findings

### Passing checks
- `/api/healthz` executes `SELECT 1` and returns 200+version — confirms DB is reachable before reporting healthy — `api_system.go:14-24`
- `/metrics` endpoint returns Prometheus text format (`text/plain; version=0.0.4`) — `api_system.go:27-78`
- `/metrics` exposes: `ticket_up`, `ticket_open_tickets_total`, `ticket_projects_total`, `ticket_users_total`, `go_goroutines`, `go_memstats_alloc_bytes`, `go_memstats_sys_bytes` — `api_system.go:39-78`
- Structured logging with `log/slog`: request method, path, status, duration_ms, query, bodies — `server.go:161-196`
- `slog.Error` / `slog.Info` used in background goroutines (agent reaper, purge) — `server.go:50-55`
- `ReadHeaderTimeout: 30 * time.Second` set on `http.Server` — mitigates Slowloris — `server.go:24-29`
- Security headers middleware: `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy` — `server.go:115-122`
- `docs/RUNBOOKS.md` covers: cold start, crash restart, DB recovery, backup/restore, user lockout, agent reaper, high latency, WebSocket disconnections, disk full — `docs/RUNBOOKS.md`
- Automated backup example provided (daily cron using `ticket export | gzip`) — `docs/RUNBOOKS.md:backup`
- `ticket export` / `ticket import` provide consistent backup/restore — `cmd/ticket/main.go`
- `TICKET_HISTORY_RETENTION_DAYS` env var for bounded history growth — `server.go:75-84`
- `restart: unless-stopped` in `compose.yaml` — container auto-restarts on crash
- Resource limits in compose: 512m memory, 1.0 CPU — prevents OOM runaway
- Agent reaper runs on startup and every 60s — prevents stuck-agent accumulation — `server.go:36-68`
- Retention purge runs on startup and daily — `server.go:70-93`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No graceful shutdown — `runServer` calls `srv.ListenAndServe()` with no SIGTERM handler | Critical | `cmd/ticket/cmd_setup.go:1002` | Wrap with `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)` + `srv.httpServer.Shutdown(ctx)` + close `stopReaper` channel |
| Structured logging disabled by default — requires `-v` flag | High | `server.go:135-143` | Log at WARN/ERROR level unconditionally; gate request-body logging behind `-v`; startup and errors should always be structured |
| `/metrics` is unauthenticated — exposes ticket/user/project counts to anonymous callers | High | `api_system.go:27` | Require bearer token or restrict to loopback/internal network; add `requireAdmin` check or IP allowlist |
| No HTTP request rate or latency metrics — `/metrics` has only gauge snapshots | Medium | `api_system.go` | Add `http_requests_total{method,path,status}` counter and `http_request_duration_seconds` histogram using `prometheus/client_golang` |
| `fmt.Fprintf` used for startup banner and DB path — not captured in structured log | Medium | `cmd/ticket/cmd_setup.go:999-1001` | Emit structured startup event via `slog.Info` with addr, db_path, version |
| No alerting rules — no Prometheus alert rules file | High | repo-wide | Define: `TicketDown` (ticket_up==0), `High5xxRate` (5xx >5% of requests), `SlowAPI` (p99 latency >1s) |
| No SLOs defined | High | repo-wide | Document: 99.5% availability, p99 latency <500ms, 5xx rate <0.1% |
| No liveness vs readiness split — single `/api/healthz` serves both purposes | Low | `api_system.go:14` | Add `GET /healthz` (liveness, no DB check) separate from `/api/healthz` (readiness, DB ping) |
| No distributed tracing | Low | repo-wide | Add OpenTelemetry SDK; instrument HTTP handlers and SQLite queries |
| `/metrics` uses hand-rolled Prometheus text format instead of `prometheus/client_golang` | Low | `api_system.go:27-78` | Use official library for correctness (type/help lines, label escaping, histogram buckets) |

## Verdict
Score drops 44 → 42 on fresh re-assessment. The `/metrics`, `slog`, and `RUNBOOKS.md` improvements from the previous cycle are real and still present. However two new findings push the score down: `/metrics` is **completely unauthenticated** — any anonymous caller can discover org size (user count, project count, ticket count); and `log/slog` structured output requires `-v` flag — production deployments without `-v` get unstructured prints. The most critical single gap remains: no SIGTERM handler. A `docker compose stop` or Kubernetes pod eviction kills all in-flight requests abruptly.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `ReadHeaderTimeout: 30s` added to `http.Server` | Closes Slowloris (slow-header) vulnerability |
| `/metrics` Prometheus endpoint present (7 metrics: up, tickets, projects, users, goroutines, alloc, sys) | Closes previous High finding on missing metrics — but endpoint is unauthenticated (-2) |
| `log/slog` structured logging with method/path/status/duration_ms fields | Closes previous High finding on unstructured logging — but requires `-v` flag (-1) |
| Comprehensive `docs/RUNBOOKS.md` (9 scenarios including DB recovery and backup) | Closes previous High finding on missing runbooks |
| `TICKET_HISTORY_RETENTION_DAYS` env var for history pruning | Reduces unbounded storage growth risk |
| **New finding:** `/metrics` unauthenticated — exposes org size (user/project/ticket counts) | **-2** — security regression; no auth on operational endpoint |
| **New finding:** structured logging gated behind `-v` flag | **-1** — production logs unstructured by default |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Graceful shutdown | Critical | `signal.NotifyContext` + `srv.httpServer.Shutdown(ctx)` + close `stopReaper` on SIGTERM/SIGINT |
| Unauthenticated `/metrics` | High | Add `requireAdmin` check or IP allowlist; metrics reveal org/team size |
| Alert rules | High | Define Prometheus rules for: server down, high 5xx rate, high latency, low disk |
| Define SLOs | High | Document availability, latency, and error-rate targets |
| Log unconditionally at ERROR/WARN | High | Remove `-v` requirement for error and startup logging |
| HTTP request metrics (rate + latency histogram) | Medium | Replace gauge-only `/metrics` with full RED metrics via `prometheus/client_golang` |
| Structured startup log | Medium | Replace `fmt.Fprintf` banner with `slog.Info("server starting", "addr", listenAddr, "db", dbPath)` |
| Liveness vs readiness split | Low | `GET /healthz` (no DB) for liveness; keep `/api/healthz` (DB ping) for readiness |
| OpenTelemetry tracing | Low | Span HTTP handlers and SQLite queries; export to OTEL collector |
