# Security

**Score: 82/100** (was 82)

## What is being assessed

Authentication, authorization, password hashing, account lockout, CSRF protection, cookie security, rate limiting, HTTP server hardening, container security, data protection (encryption at rest, env masking), vulnerability management, and graceful shutdown.

## Methodology

Static analysis of `internal/password/hash.go`, `internal/server/server.go`, `api.go`, `api_auth.go`, `api_system.go`, `api_router.go`, `api_teams.go`, `api_projects.go`, `ratelimit.go`, `realtime.go`, `internal/store/auth.go`, `internal/store/encrypt.go`, `compose.yaml`, `Dockerfile`, `Makefile`, and `.github/workflows/makefile.yaml`. Compared all findings against the previous assessment (score 65).

## Findings

### Passing checks

- **Argon2id password hashing** with production-grade parameters (64 MB memory, 4 iterations, 2 parallelism, 16-byte CSPRNG salt, 32-byte key, constant-time compare) -- `internal/password/hash.go`
- **Account lockout** implemented: 10 failed attempts triggers 15-minute lock; counters reset on success or expiry -- `internal/store/auth.go:109-173`
- **CSRF double-submit cookie** middleware on all `/api/` state-changing endpoints; constant-time token comparison; login/register/agent-register exempt; Bearer/Basic auth exempt -- `internal/server/server.go:297-379`
- **Session tokens**: 32-byte CSPRNG, base64url-encoded, 30-day expiry, server-side revocation, session invalidation on password reset -- `internal/store/auth.go:176-198, 369-394`
- **Cookie flags**: `HttpOnly: true`, `SameSite: Lax`, `Secure: r.TLS != nil` on `ticket_token` cookie; CSRF cookie is non-HttpOnly (by design, JS reads it) with `SameSite: Strict` -- `api_auth.go:144-152`, `server.go:329-336`
- **Rate limiting** on login and register: 10 requests per minute per IP, with stale-key eviction to prevent unbounded map growth -- `ratelimit.go`, `api_auth.go:87, 119`
- **HTTP timeouts**: `ReadHeaderTimeout: 30s`, `ReadTimeout: 60s`, `IdleTimeout: 120s`. `WriteTimeout` intentionally omitted to avoid killing long-lived WebSocket connections (documented in code) -- `server.go:38-45`
- **SIGTERM graceful shutdown**: `signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)` with 30-second drain; `Server.Shutdown()` stops reaper goroutines and chat heartbeat -- `cmd/tk/cmd_setup.go:1344-1360`, `server.go:193-197`
- **Security headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, Content-Security-Policy -- `server.go:169-176`
- **Body size limit**: 1 MB via `http.MaxBytesReader` on all non-GET/HEAD requests -- `server.go:178-186`
- **Panic recovery middleware** prevents stack traces leaking to clients -- `server.go:157-167`
- **`/metrics` endpoint authenticated**: requires `requireUser` -- `api_system.go:33-35`
- **WebSocket Origin validation** prevents cross-origin hijacking; auth checked before upgrade -- `realtime.go:146-170`, `api_auth.go:25-36`
- **WebSocket project-scoped broadcast**: clients subscribe to a `project_id`; broadcast filters by project -- `realtime.go:39, 77, 133-141`
- **Team-based access control**: team ownership checks via `TeamRoleForUser`; project visibility scoped to user via `ListProjectsVisibleToUser` -- `api_teams.go`, `api_projects.go:27`
- **Role-based access control**: `requireUser`/`requireAdmin` guards on all state-changing endpoints
- **All SQL parameterised** throughout `internal/store/`
- **AES-256-GCM encryption** for email addresses at rest via `TICKET_ENCRYPTION_KEY` env var -- `internal/store/encrypt.go`
- **Non-root container** with multi-stage build, dedicated `ticket` user -- `Dockerfile:22-25`
- **Container hardening**: `cap_drop: [ALL]`, `pids_limit: 200`, `no-new-privileges`, memory/CPU limits, health check -- `compose.yaml:12-29`
- **govulncheck** in CI pipeline and Makefile -- `.github/workflows/makefile.yaml:37-38`, `Makefile:53`
- **Sensitive endpoint logging suppression**: login/register/reset-password request bodies are not logged -- `server.go:256-258`
- **Expired session purge** runs daily -- `server.go:84-120`
- **Minimum password length** enforced (8 characters) -- `store/auth.go:88`

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `WriteTimeout` omitted on HTTP server | Low | `server.go:43-44` | Consider a reverse proxy (nginx/Caddy) with write timeouts in front; the omission is documented and justified for WebSocket support but leaves non-WS responses without a write deadline |
| Rate limiter uses `r.RemoteAddr` only, no `X-Forwarded-For` awareness | Medium | `ratelimit.go:59-65` | Behind a reverse proxy, all requests appear from one IP; add configurable trusted-proxy CIDR to extract real client IP |
| CSP allows `unsafe-inline` for scripts and styles | Low | `server.go:173` | Replace with nonce-based CSP to mitigate XSS via inline injection |
| CSRF cookie `Secure` flag depends on `r.TLS != nil` | Low | `server.go:335` | Behind a TLS-terminating proxy, `r.TLS` is nil so cookie is sent without Secure; consider a config flag or trust `X-Forwarded-Proto` |
| Encryption key padding/truncation to 32 bytes | Low | `internal/store/encrypt.go:21-24` | Reject keys that are not exactly 32 bytes rather than silently padding/truncating, which weakens entropy if a short key is provided |
| No general API rate limiting beyond auth endpoints | Low | `api.go:14` | Add per-user or global rate limiting for write-heavy endpoints to prevent abuse |
| Dockerfile base images not pinned to SHA256 digest | Low | `Dockerfile:3, 17` | Pin `golang:1.26-alpine` and `alpine:3.21` to `@sha256:...` for reproducible, tamper-resistant builds |

## Verdict

The security posture remains strong in this pass. CSRF, account lockout, authenticated metrics, timeouts, body limits, graceful shutdown, and container hardening all remain in place, while the remaining issues stay concentrated in deployment-layer hardening such as trusted-proxy awareness, CSP nonces, and image pinning.

## Changes since last assessment

- No material security regressions were found in this pass
- The earlier CSRF, lockout, timeout, and container-hardening improvements remain intact
- The same medium/low deployment-hardening recommendations remain open

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Rate limiter ignores `X-Forwarded-For` | Medium | Add trusted-proxy CIDR config; extract real IP only from trusted sources |
| CSP `unsafe-inline` | Low | Migrate to nonce-based CSP for scripts and styles |
| CSRF `Secure` flag behind TLS proxy | Low | Detect TLS termination via `X-Forwarded-Proto` or config flag |
| Encryption key padding | Low | Validate key is exactly 32 bytes; reject otherwise |
| No general API rate limiting | Low | Add per-user throttle on write endpoints |
| Dockerfile images not SHA-pinned | Low | Pin base images to digest for supply-chain integrity |
| Missing `WriteTimeout` | Low | Add a reverse proxy with write timeouts or implement per-handler deadlines for non-WS routes |
