# SRE

**Score: 50/100** (was 42)

## What is being assessed
Observability (metrics, structured logging, tracing), alerting readiness, runbook quality, incident response, backup/restore, graceful shutdown, HTTP timeout configuration, and health check endpoints.

## Methodology
Read `internal/server/api.go`, `internal/server/server.go`, `docs/RUNBOOKS.md`, `deploy/entrypoint.sh`. Grepped for `prometheus`, `slog`, `SIGTERM`, `signal`, `IdleTimeout`, `WriteTimeout`, `http.Server{`.

## Findings

### Passing checks
- `/api/healthz` endpoint returns `{"status":"ok","version":"<ver>"}` with SQLite `SELECT 1` health check (`internal/server/api_system.go:18-30`)
- Structured logging: `log/slog` used for agent reaper, session purge, history purge (`internal/server/server.go:12,54,57,82,84,94,96`)
- Full request logging middleware logs method, path, status, duration_ms for all `/api/` requests (`server.go:201-241`)
- Prometheus-style `/metrics` endpoint exists with runtime stats (`api_system.go:32`)
- `docs/RUNBOOKS.md` is comprehensive: cold start, crash recovery, DB recovery, backup/restore, user lockout, agent reaper, high latency, WebSocket disconnections, disk full — 9 scenarios with step-by-step commands
- Backup procedure documented with cron example: `docker exec ticket tk export | gzip` (`RUNBOOKS.md:121-162`)
- Restore procedure with `--overwrite` flag documented (`RUNBOOKS.md:131-162`)
- `ReadHeaderTimeout: 30s` set on HTTP server (`server.go:37`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `/metrics` endpoint has no authentication — exposes goroutine counts, user counts, memory stats | High | `internal/server/api_system.go:32` | Add `requireUser()` middleware or serve on a separate internal port |
| No `SIGTERM` / graceful shutdown handler | High | `cmd/ticket/cmd_setup.go` (`runServer`) | Add `signal.Notify` + `http.Server.Shutdown(ctx)` with 30s drain timeout |
| `WriteTimeout`, `ReadTimeout`, `IdleTimeout` not configured | High | `internal/server/server.go:34-38` | Set `WriteTimeout: 30s`, `ReadTimeout: 60s`, `IdleTimeout: 120s` |
| Background reaper/purge goroutines not stopped on shutdown | Medium | `internal/server/server.go:41-42` | Close `stopReaper` channel from SIGTERM handler |
| No distributed tracing or request correlation IDs | Low | `internal/server/server.go` | Add `X-Request-ID` header; pass to slog fields |
| No panic recovery middleware | Low | `internal/server/api.go` | Add `http.HandlerFunc` wrapper with `recover()` to prevent server crashes |

## Verdict
Meaningful improvement (+8) from confirming the runbooks are comprehensive and structured logging is in place. However, three high-severity issues remain: the metrics endpoint is unauthenticated, there is no graceful shutdown on SIGTERM, and HTTP Write/Read/Idle timeouts are missing — these are blocking for production hardening.

## Changes since last assessment
- `deploy/entrypoint.sh` updated for `tk` binary name (cosmetic)
- No new observability or SRE improvements implemented
- Same three High-severity gaps persist from v0.1.737

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Protect `/metrics` | High | Add `requireUser()` check or bind to `127.0.0.1:9090` separately |
| Add SIGTERM handler | High | `signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)`; `srv.Shutdown(ctx)` |
| HTTP timeouts | High | Set `WriteTimeout`, `ReadTimeout`, `IdleTimeout` on `http.Server` struct |
| Stop background jobs on shutdown | Medium | Close `stopReaper` channel from SIGTERM handler to drain goroutines |
| Request correlation IDs | Low | Generate UUID in middleware; attach to `slog.With("request_id", id)` |
