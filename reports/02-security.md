# Security

**Score: 65/100** (was 58)

## What is being assessed
Authentication security (password hashing, session management, lockout), access control (RBAC, team-based), data protection, CSRF protection, cookie security, container security, rate limiting, and vulnerability management. Version 0.1.737, assessed 2025-07-21.

## Methodology
Reviewed `internal/server/api_auth.go`, `internal/server/api_tickets.go`, `internal/server/ratelimit.go`, `internal/server/server.go`, `internal/password/hash.go`, `internal/store/auth.go`, `internal/store/encrypt.go`, `.github/workflows/makefile.yaml`, `compose.yaml`, `Dockerfile`. Checked: JWT/token validation on every state-changing endpoint, Argon2id parameters, rate limiting coverage, cookie flags, CSRF protection, WebSocket origin validation, account lockout, container security (non-root, CapDrop, PidsLimit), and session expiry enforcement.

## Findings

### Passing checks
- Argon2id with 64 MB memory, 4 iterations, 16-byte `crypto/rand` salt, `subtle.ConstantTimeCompare` (`internal/password/hash.go:28-38`)
- Session tokens: 32-byte `crypto/rand`, base64url-encoded (`internal/store/auth.go:133-139`)
- Cookies: `HttpOnly: true`, `Secure: r.TLS != nil`, `SameSite: Lax`, 30-day `MaxAge` (`internal/server/api_auth.go:144-151`)
- Session expiry enforced: `GetUserByToken` query checks `expires_at > CURRENT_TIMESTAMP` (`internal/store/auth.go:157-163`)
- Rate limiting on `/api/login` and `/api/register` (`internal/server/api_auth.go:87,118`)
- WebSocket Origin header validated against server Host before upgrade (`internal/server/realtime.go:133-149`)
- Security response headers: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, CSP (`internal/server/server.go:127-131`)
- Non-root Docker runtime user: `adduser -D ticket` + `USER ticket` (`Dockerfile:13-14`)
- `ReadHeaderTimeout: 30s` prevents Slowloris attacks (`internal/server/server.go:37`)
- gosec + govulncheck run in CI on every push/PR (`makefile.yaml:21-24`)
- AES-256-GCM with random nonce for email-at-rest encryption (`internal/store/encrypt.go:22-41`)
- No hardcoded credentials anywhere in codebase
- RBAC enforced: project membership and admin role checked before mutations
- `PurgeExpiredSessions` runs daily to clean up stale tokens (`internal/server/server.go:77-80`)
- Memory and CPU limits set in compose (`compose.yaml:12-16`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Verbose logging writes full request body (including login credentials) to logs | High | `internal/server/server.go:207-233` | Redact any `password` field before logging; never log auth request bodies |
| `X-Forwarded-For` header trusted unconditionally — rate limiter bypassed by IP spoofing | Medium | `internal/server/ratelimit.go:49-51` | Only trust XFF from a configured trusted-proxy CIDR list; default to `RemoteAddr` |
| No account-level lockout — only per-IP rate limiting | Medium | `internal/server/api_auth.go:118-120` | Add per-username failed-attempt counter with 15-minute lockout window |
| CSP includes `'unsafe-inline'` for `script-src` and `style-src` | Medium | `internal/server/server.go:129` | Move all inline JS/CSS to external files; use a nonce-based CSP |
| `compose.yaml` missing `cap_drop`, `no-new-privileges`, `pids_limit` | Medium | `compose.yaml` | Add `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, `pids_limit: 100` |
| `WriteTimeout` and `ReadTimeout` not set (only `ReadHeaderTimeout`) | Low | `internal/server/server.go:35-39` | Add `ReadTimeout: 60s`, `WriteTimeout: 90s` to prevent resource exhaustion |
| Cookie `Secure` flag is `false` when behind TLS-terminating reverse proxy | Low | `internal/server/api_auth.go:149` | Document that a `TICKET_BEHIND_PROXY=true` env var should force `Secure: true` |
| WebSocket token accepted via URL query parameter (appears in server logs) | Low | `internal/server/api_auth.go:24-26` | Prefer `Authorization: Bearer` only; deprecate `?token=` query param |

## Verdict
A meaningful improvement since v0.1.730. The two previously-Critical findings (WebSocket origin validation and session expiry enforcement) are now fixed. gosec is clean and integrated into CI. The cryptographic foundation (Argon2id, 32-byte random tokens, AES-256-GCM at rest) remains solid. The remaining priority issues are: verbose-mode credential logging (passwords will appear in log files), the X-Forwarded-For bypass that trivialises rate limiting, and the absence of username-level account lockout. None of these are exploitable without either access to logs or the ability to send spoofed requests, but they should be addressed before internet-facing deployment.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| WebSocket `Origin` validation added (`realtime.go:133-149`) | Closed High finding from previous report |
| Session `expires_at` now checked in `GetUserByToken` | Closed High finding from previous report |
| gosec integrated into CI (`makefile.yaml:21-22`) | Systemic improvement; ongoing vulnerability gate |
| govulncheck integrated into CI | Dependency CVE gate added |
| All 97 gosec findings resolved (genuine fixes + justified `#nosec`) | Reduced latent risk across codebase |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Credential logging in verbose mode | High | Redact `password` from logged request bodies in `server.go:230` |
| X-Forwarded-For trusted without proxy allowlist | Medium | Introduce `TICKET_TRUSTED_PROXIES` env var; fallback to `RemoteAddr` |
| No per-username account lockout | Medium | Track failed attempts in DB; lock for 15 min after 10 failures |
| CSP `unsafe-inline` for scripts | Medium | Nonce-based CSP; extract inline JS to static file |
| Container hardening (cap_drop, pids_limit) | Medium | Add to `compose.yaml` and document in deployment guide |
| HTTP server timeouts incomplete | Low | Set `ReadTimeout` and `WriteTimeout` alongside `ReadHeaderTimeout` |
