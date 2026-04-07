# Security

**Score: 58/100**

## What is being assessed
Authentication security (password hashing, session management, lockout), access control (RBAC, team-based), data protection, CSRF protection, cookie security, container security, rate limiting, and vulnerability management.

## Methodology
Reviewed `internal/password/hash.go`, `internal/store/auth.go`, `internal/server/api.go`, `internal/server/ratelimit.go`, `Dockerfile`, `compose.yaml`, `go.mod`. Checked for CSRF tokens, cookie flags, SQL parameterisation, and Origin validation.

## Findings

### Passing checks
- Password hashing: Argon2id with 64MB memory, 4 iterations, 16-byte crypto/rand salt, `subtle.ConstantTimeCompare` (`internal/password/hash.go`)
- Session tokens: 32-byte cryptographically random, base64url-encoded (`internal/store/auth.go:135-139`)
- Cookies: `HttpOnly=true`, `Secure` conditional on TLS, `SameSite=Lax` (`api.go:180-182`)
- Rate limiting on `/api/login` and `/api/register`: 10 req/min per IP (`api.go:18`)
- SQL parameterisation: 95%+ of queries use `?` placeholders throughout store layer
- Security headers: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, CSP (`server.go:98-105`)
- No hardcoded secrets anywhere in codebase
- All Go dependencies up-to-date with no known vulnerabilities
- Non-root Docker user (`adduser ticket` + `USER ticket`)
- RBAC: project roles (viewer/editor/owner) + admin checks on all sensitive endpoints
- Foreign key constraints enforced via PRAGMA

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No CSRF protection on any state-changing endpoint | High | All POST/PUT/DELETE in `api.go` | Implement synchronizer token pattern or double-submit cookie; add CSRF middleware |
| WebSocket upgrade does not validate `Origin` header | High | `internal/server/realtime.go:132-158` | Validate `Origin` header against allowlist before upgrade |
| Session `expires_at` column exists but never checked | High | `internal/store/auth.go:155-177` | Add `AND expires_at > CURRENT_TIMESTAMP` to token lookup query |
| `X-Forwarded-For` trusted without proxy allowlist | Medium | `internal/server/ratelimit.go:49-50` | Only trust `X-Forwarded-For` from known proxy IPs via config |
| WebSocket token accepted as query parameter (logged in URLs) | Medium | `api.go:44,70` | Require `Authorization: Bearer` header only; remove `?token=` support |
| No account-level lockout (only IP rate limit) | Medium | `internal/server/api.go` | Add username-based failed-attempt counter with exponential backoff |
| Docker compose missing `cap_drop`, `pids_limit`, resource limits | Medium | `compose.yaml` | Add `cap_drop: [ALL]`, `pids_limit: 50`, memory/CPU limits |
| Session duration 30 days with no rotation | Low | `api.go:183` | Reduce to 24h; implement token rotation on sensitive operations |
| CSP includes `'unsafe-inline'` for scripts | Low | `server.go:102` | Move inline JS to external file; remove `unsafe-inline` |
| SQL string concatenation in migration PRAGMA calls | Low | `internal/store/store.go:1595,1565` | Validate `tableName` against alphanumeric+underscore allowlist |

## Verdict
Solid cryptographic foundation (Argon2id, secure random tokens, correct cookie flags) but critical gaps in CSRF protection and WebSocket origin validation. These two issues could allow cross-site attacks against authenticated users. The rate limiter is easy to bypass via X-Forwarded-For spoofing. Should not be exposed to untrusted networks without fixing CSRF and WebSocket origin checks first.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| CSRF middleware | High | Add double-submit cookie or synchronizer token to all state-changing endpoints |
| WebSocket `Origin` validation | High | Validate `Origin` against configured allowlist in `realtime.go:132` |
| Session expiry enforcement | High | Check `expires_at` in `GetUserByToken()` |
| Trusted proxy config for `X-Forwarded-For` | Medium | Add `TRUSTED_PROXIES` env var; only trust forwarded IPs from that list |
| Remove `?token=` from WebSocket auth | Medium | Header-only auth prevents token exposure in logs |
| Add `cap_drop: [ALL]` to compose | Medium | Reduce container attack surface |
