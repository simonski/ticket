# Security

**Score: 65/100** (was 68)

## What is being assessed
Authentication (session tokens, password hashing), account protection (lockout, rate limiting), data protection (cookie flags, HTTPS enforcement), CSRF on all state-changing endpoints, container hardening (CapDrop, PidsLimit, non-root), HTTP server timeouts, and input validation.

## Methodology
Read `internal/server/api_auth.go`, `internal/password/hash.go`, `internal/server/ratelimit.go`, `internal/server/server.go`, `compose.yaml`, `Dockerfile`. Searched for CSRF middleware, cookie configuration, timeout settings, body size limits.

## Findings

### Passing checks
- Argon2ID password hashing with 64 MB memory, 4 iterations, 16-byte salt, constant-time comparison (`internal/password/hash.go`)
- Session tokens: 32-byte random, stored in SQLite, 30-day expiry with automatic purge (`internal/server/api_auth.go:139-152`)
- Cookie flags: `HttpOnly: true`, `SameSite: Lax`, `Secure` conditional on TLS (`api_auth.go:144-152`)
- Rate limiting on `/api/login` and `/api/register`: 10 req/min/IP (`internal/server/ratelimit.go`, `api.go:14`)
- Security headers set: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Content-Security-Policy` (`server.go:146-153`)
- All SQL queries parameterized; no string-concatenated queries found
- Non-root container user (`Dockerfile:20-22`)
- Multi-stage Docker build on Alpine 3.21

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No CSRF tokens on POST/PUT/DELETE endpoints | High | All `api_*.go` files | Add CSRF middleware (e.g. `gorilla/csrf`) to all state-changing routes |
| No account lockout after failed login attempts | High | `internal/server/api_auth.go:113-137` | Track failed attempts per username; lock after 10 failures for 15 min |
| `WriteTimeout`, `ReadTimeout`, `IdleTimeout` not set | Medium | `internal/server/server.go:34-38` | Add all three timeouts; only `ReadHeaderTimeout: 30s` currently set |
| No request body size limit on JSON endpoints | Medium | `internal/server/api_tickets.go`, `api_projects.go` | Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, 1<<20)` |
| `cap_drop`/`pids_limit` absent from `compose.yaml` | Medium | `compose.yaml` | Add `cap_drop: [ALL]`, `pids_limit: 100` to service |
| Rate limiter trusts `X-Forwarded-For` without validation | Medium | `internal/server/ratelimit.go:48-56` | Only trust forwarded IP if a known proxy prefix is configured |
| CSP allows `unsafe-inline` for scripts and styles | Low | `internal/server/server.go:150` | Use CSP nonces; remove `unsafe-inline` |

## Verdict
Core auth primitives are strong (Argon2ID, session management, security headers, rate limiting on auth). The main gaps are CSRF protection and account lockout — both were present in the previous assessment and remain unfixed, driving a slight score decrease as they are now confirmed absent rather than merely flagged.

## Changes since last assessment
- No security-relevant code changes this cycle
- `tk-test` default binary path still points to `./bin/ticket` (not a security issue)
- All previously identified gaps persist

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| CSRF protection | High | Add `gorilla/csrf` to all state-changing API routes |
| Account lockout | High | Add `failed_attempts` + `locked_until` columns to `users` table |
| HTTP timeouts | Medium | Set `WriteTimeout: 30s`, `ReadTimeout: 60s`, `IdleTimeout: 120s` |
| Body size limit | Medium | Apply `http.MaxBytesReader` to all POST/PUT handlers |
| Container hardening | Medium | Add `cap_drop`, `pids_limit`, `security_opt: no-new-privileges` |
