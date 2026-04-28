# Backend Engineer

**Score: 76/100** (was 73)

## Mission
Ensure server-side code is correct, idiomatic, explicit, and safe under load.

## Review objective
Check error handling, context propagation, auth paths, concurrency, and resource control.

## Inputs reviewed
- `internal/server`
- `internal/store`
- `internal/client`
- `libticket`
- `Makefile`

## Findings

### Passing checks
- HTTP server has read/header/idle timeouts and deliberately omits write timeout for websockets (`internal/server/server.go:41-49`).
- Store opens SQLite with single connection, foreign keys, and busy timeout (`internal/store/schema_version.go:40-56`).
- Auth helpers centralize user/admin requirements and error mapping (`internal/server/api_helpers.go:159-172`, `internal/server/api_helpers.go:313-323`).
- Go coverage gates pass for backend packages (`Makefile:135-157`; coverage command output).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Chat child process inherits entire environment. | High | Secrets available to server can leak to external child commands. | `internal/server/chat_ws.go:232-237` | Whitelist only required environment variables. |
| Health endpoint does not reflect subsystem readiness. | Medium | API may report healthy while chat or maintenance subsystems are degraded. | `internal/server/api_system.go:19-31` | Add readiness details or separate `/api/readyz`. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| application-security | Env inheritance is a trust-boundary issue. | Env whitelist design. |
| sre | Readiness semantics affect monitoring. | Health payload proposal. |

## Verdict
Backend implementation quality is generally strong. The main backend risk is unsafe environment propagation into child processes.

## Changes since last assessment
- Server package coverage now meets its raised 70% gate at 70.0%.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Child env leakage | High | Add filtered environment builder and tests. | backend-engineer |
