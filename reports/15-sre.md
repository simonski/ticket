# SRE

**Score: 52/100** (was 50)

## What is being assessed

Operational readiness: observability, health checks, graceful shutdown, HTTP timeouts, alerting, runbooks, backup/restore, capacity planning, SLA/SLO definitions, graceful degradation.

## Methodology

Read `internal/server/server.go`, `api_system.go`, `cmd/tk/cmd_setup.go`. Read `deploy/compose.yaml`, `docs/RUNBOOKS.md`. Grepped for SIGTERM, Shutdown, timeouts, prometheus, recover(), request IDs, SLO/SLA.

## Findings

### Passing checks
- `/api/healthz` with live SQLite `SELECT 1` — `api_system.go:18-30`
- Structured logging via `log/slog` — `server.go:11,54,57,82,84,94,96`
- Request logging: method, path, status, `duration_ms` — `server.go:201-241`
- `/metrics` in Prometheus format — `api_system.go:32-74`
- `ReadHeaderTimeout: 30s` — `server.go:37`
- `stopReaper` channel plumbed to background goroutines — `server.go:26,66,73,108`
- `docs/RUNBOOKS.md` with 9 scenarios — `docs/RUNBOOKS.md`
- Backup procedure documented with cron and retention — `RUNBOOKS.md:121-162`
- Rate limiting on auth endpoints — `ratelimit.go`
- Docker `restart: unless-stopped` — `deploy/compose.yaml:8`
- DB migrations auto-run on startup

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No SIGTERM/graceful shutdown — `ListenAndServe` never calls `Shutdown` | High | `cmd_setup.go:1143` | Add `signal.Notify` + `srv.Shutdown(ctx)` |
| Missing `WriteTimeout`/`ReadTimeout`/`IdleTimeout` | High | `server.go:34-38` | Add timeouts |
| `/metrics` unauthenticated | High | `api_system.go:32` | Add `requireAdmin` or restrict to loopback |
| Background goroutines not stopped — `stopReaper` never closed | Medium | `server.go:41-42` | Close from SIGTERM handler |
| No panic recovery middleware | Medium | `server.go:133` | Add `recover()` middleware returning 500 |
| No request correlation IDs | Low | `server.go:201-241` | Generate UUID per request |
| No distributed tracing | Low | — | Document gap |
| No SLA/SLO definitions | Medium | `docs/` | Define in `docs/SLO.md` |
| No alerting rules | Medium | — | Add `alerts/ticket.yml` |
| No capacity planning docs | Low | `docs/` | Add capacity section |
| No Docker resource limits in deploy compose | Low | `deploy/compose.yaml` | Add limits |
| No health-check in deploy compose | Medium | `deploy/compose.yaml` | Add healthcheck |

## Verdict

Score improves marginally +2 to 52. All three high-severity items from previous report remain: no graceful shutdown, no HTTP timeouts, unauthenticated `/metrics`. These block production readiness.

## Changes since last assessment
- `stopReaper` confirmed correctly plumbed at goroutine level (+)
- No SRE gaps remediated — SDLC refactor didn't touch server lifecycle code

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Graceful SIGTERM shutdown | Critical | `cmd_setup.go:1134-1144` |
| HTTP timeouts | Critical | `server.go:34-38` |
| Authenticate `/metrics` | High | `api_system.go:32` |
| Compose health check | Medium | `deploy/compose.yaml` |
| Panic recovery middleware | Medium | `server.go` handler chain |
| SLO/alerting docs | Medium | `docs/SLO.md` |
| Request correlation IDs | Low | `server.go:201` |
