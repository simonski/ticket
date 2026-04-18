# Security Engineer

**Score: 81/100** (was 82)

## Mission
Protect confidentiality, integrity, and access boundaries through strong defaults and explicit controls.

## Review objective
Verify that auth, sessions, CSRF, cookies, and deployment-facing security controls remain sound.

## Inputs reviewed
- `internal/password/hash.go`
- `internal/store/auth.go`
- `internal/server/api_auth.go`
- `internal/server/server.go`
- `internal/server/ratelimit.go`

## Findings

### Passing checks
- Passwords are hashed with Argon2id and verified with constant-time comparison (`internal/password/hash.go:16-55`).
- Session creation, expiry, lockout, and cookie settings are explicit rather than ambient (`internal/store/auth.go:116-213`, `internal/server/api_auth.go:138-147`, `internal/server/api_auth.go:161-172`).
- The server applies CSP, X-Frame-Options, CSRF protection, and request body limits at middleware level (`internal/server/server.go:253-265`, `internal/server/server.go:287-295`, `internal/server/server.go:454-549`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| `requestIsSecure()` trusts `X-Forwarded-Proto` without the trusted-proxy CIDR checks used elsewhere. | Medium | A spoofed proxy header can affect secure-cookie and HSTS behavior assumptions. | `internal/server/server.go:551-565`, `internal/server/ratelimit.go:61-115` | Reuse trusted-proxy validation for `X-Forwarded-Proto` handling. |
| Analyse/chat subprocesses inherit the full server environment. | Low | Secrets or unrelated runtime config can leak into child process execution contexts. | `internal/server/analyse.go:60-79`, `internal/server/chat_ws.go:232-237` | Pass only the minimum required environment to child commands. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| devops-engineer | Proxy trust rules must match deployed topology. | Trusted-proxy configuration plan. |
| application-security | Child-process behavior is part of the realistic attack surface. | Threat model for server-side command execution. |
| privacy-and-compliance | Cookie and proxy decisions affect data-protection posture. | TLS/proxy deployment checklist. |

## Verdict
The current security posture is respectable: password, session, CSRF, and header controls are all present and implemented centrally. The main remaining weakness is trust-boundary consistency around proxies and subprocess environment scope.

## Changes since last assessment
- Security remains one of the stronger areas, and a previously reported short-key concern is now closed by explicit minimum-length enforcement in the encryption helper (`internal/store/encrypt.go:17-31`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Untrusted `X-Forwarded-Proto` path | Medium | Gate secure-request decisions behind trusted proxy checks. | security-engineer |
| Broad subprocess env | Low | Filter child-process environment variables. | devops-engineer |
