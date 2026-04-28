# Application Security

**Score: 67/100** (was 71)

## Mission
Think like an attacker and prove whether realistic attack paths are blocked.

## Review objective
Review injection, XSS/CSRF, SSRF, child-process, header spoofing, and abuse flows.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/chat_ws.go`
- `internal/server/api_helpers.go`
- `internal/server/ratelimit.go`
- `deploy`

## Findings

### Passing checks
- CSP headers and nonce injection exist for the SPA root (`internal/server/server.go:255-265`, `internal/server/server.go:425-437`).
- Rate limiting bounds per-key request bursts and evicts stale keys (`internal/server/ratelimit.go:12-59`).
- Auth errors are centrally mapped (`internal/server/api_helpers.go:313-323`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Chat child process inherits secrets. | High | If chat command or prompt path is compromised, inherited env can expose credentials and tokens. | `internal/server/chat_ws.go:232-237` | Build a minimal env whitelist and test it. |
| Header spoofing can affect secure-request decisions. | High | Direct attackers may manipulate cookie/security behavior in non-proxy deployments. | `internal/server/server.go:668-681` | Require trusted proxy validation before honoring `X-Forwarded-Proto`. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| security-engineer | Controls overlap with secure defaults. | Trust boundary model. |
| backend-engineer | Child env filtering is code work. | `cleanChatEnv` implementation. |

## Verdict
XSS and CSRF controls are materially present. The most realistic attack paths now center on deploy/default credentials, header spoofing, and child-process environment exposure.

## Changes since last assessment
- CSP concern is reduced by nonce tests and injection code.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Child process env exposure | High | Whitelist env vars and add a unit test. | application-security |
