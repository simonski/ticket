# Security Engineer

**Score: 66/100** (was 73)

## Mission
Protect confidentiality, integrity, authentication, authorization, and secure defaults.

## Review objective
Audit auth, secrets, cookies, deployment defaults, and trust boundaries.

## Inputs reviewed
- `internal/password/hash.go`
- `internal/store/auth.go`
- `internal/server`
- `deploy`
- `SECURITY.md`

## Findings

### Passing checks
- Passwords use Argon2id with strong default parameters and constant-time verification (`internal/password/hash.go:22-56`).
- Session tokens use 32 random bytes and configurable expiry (`internal/store/auth.go:191-212`).
- Security policy tells users not to file public vulnerability issues (`SECURITY.md:8-19`).
- CSRF and session cookie naming use secure host-prefixed names when HTTPS is detected (`internal/server/api_helpers.go:188-199`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Default admin password is committed and fallbacked. | Critical | Public deployments are vulnerable to immediate admin compromise. | `deploy/compose.yaml:6-10`, `deploy/entrypoint.sh:12-16` | Remove default; require explicit secret or generate one-time secret output. |
| HTTPS trust is inferred from unvalidated `X-Forwarded-Proto`. | High | A direct client can influence secure-cookie/HSTS behavior if headers are accepted outside a trusted proxy boundary. | `internal/server/server.go:668-681` | Only honor forwarded proto from configured trusted proxy CIDRs. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| devops-engineer | Compose defaults need secure replacement. | Deployment bootstrap design. |
| application-security | Header trust must be threat-modeled. | Proxy trust-boundary model. |

## Verdict
Core auth primitives are strong, but secure defaults are not. The default password is a production blocker.

## Changes since last assessment
- Trusted proxy handling exists for client IP extraction, but secure-proto trust still needs equivalent protection.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Known default admin password | Critical | Remove password fallback and committed value. | security-engineer |
