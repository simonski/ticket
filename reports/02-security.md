# Security

**Score: 65/100** (was 65)

## What is being assessed

Authentication, access control, cookie security, CSRF protection, rate limiting, WebSocket security, container security, and data protection.

## Methodology

Read `internal/password/hash.go`, `internal/server/api_auth.go`, `ratelimit.go`, `server.go`, `api_system.go`, `realtime.go`, `chat_ws.go`, `api_helpers.go`, `internal/store/auth.go`, `encrypt.go`, `compose.yaml`, `Dockerfile`.

## Findings

### Passing checks
- Argon2ID password hashing (64 MB, 4 iterations, 2 parallelism, 16-byte salt, constant-time compare) — `internal/password/hash.go`
- 32-byte CSPRNG session tokens with 30-day expiry and server-side revocation — `internal/store/auth.go:135-149`
- Cookie flags: `HttpOnly: true`, `SameSite: Lax`, `Secure: r.TLS != nil` — `api_auth.go:144-152`
- Session invalidation on password reset — `auth.go:334`
- Rate limiting on login/register: 10 req/min/IP — `api.go:14`
- Security headers: `X-Content-Type-Options`, `X-Frame-Options`, CSP — `server.go:146-153`
- All SQL parameterised throughout `internal/store/`
- Role-based access control: `requireUser`/`requireAdmin`, per-project roles — `api_helpers.go:120-195`
- WebSocket origin validation — `realtime.go:136-156`
- WS auth checked before upgrade — `api_auth.go:25-80`
- Non-root container, multi-stage build — `Dockerfile:20-22`
- AES-256-GCM email encryption — `encrypt.go`
- `ReadHeaderTimeout: 30s` — `server.go:37`

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No CSRF protection on state-changing endpoints | High | All `api_*.go` | Add `gorilla/csrf` or double-submit cookie |
| No account lockout after failed logins | High | `api_auth.go:113-137` | Add `failed_attempts`/`locked_until` columns |
| `/metrics` unauthenticated — exposes counts and heap stats | Medium | `api_system.go:32-74` | Add `requireAdmin` guard |
| Cross-project WebSocket broadcast | Medium | `realtime.go:66-80` | Filter by `projectIDs` on `liveClient` |
| `context.Background()` in WS handler | Low-Med | `chat_ws.go:168,177` | Derive context from WS lifecycle |
| Missing `WriteTimeout`/`ReadTimeout`/`IdleTimeout` | Medium | `server.go:34-38` | Add timeouts; exempt WS via hijack |
| No request body size limit | Medium | Various handlers | Use `http.MaxBytesReader` |
| `X-Forwarded-For` spoofing bypasses rate limiter | Medium | `ratelimit.go:49-51` | Only trust with configured trusted-proxy CIDR |
| Container hardening absent (cap_drop, pids_limit) | Medium | `compose.yaml` | Add security options |
| CSP allows `unsafe-inline` | Low | `server.go:150` | Replace with nonces |

## Verdict

Core auth primitives are strong. The Origin-check on WebSocket closes cross-origin hijacking. Score unchanged at 65: new cross-project leak and persistent CSRF/lockout/metrics gaps offset the WS origin fix.

## Changes since last assessment
- WebSocket Origin validation fixed — `realtime.go:136-156`
- All other previous findings unchanged

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| CSRF protection | High | Add middleware to all state-changing routes |
| Account lockout | High | Lock 15 min after 10 failures |
| `/metrics` unauthenticated | Medium | Gate with `requireAdmin` |
| Cross-project WS broadcast | Medium | Filter by project |
| HTTP timeouts | Medium | Set Write/Read/IdleTimeout |
| Body size limit | Medium | `http.MaxBytesReader` in JSON handlers |
| X-Forwarded-For trust | Medium | Trusted-proxy CIDR |
| Container hardening | Medium | `cap_drop: [ALL]`, `pids_limit: 200` |
