# Cyber

**Score:** 73/100 **(was 73)**

## Standard
The project is resilient against realistic attack paths and insecure operating conditions.

## Assessment scope
Request security, session/cookie behavior, deploy defaults, proxy trust, and subprocess execution surfaces.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/api_auth.go`
- `internal/server/chat_ws.go`
- `deploy/compose.yaml`
- `deploy/entrypoint.sh`
- `.github/workflows/makefile.yaml`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Auth/authz boundaries explicit and tested | MUST | partial | `internal/server/api_auth.go:138-168`; `internal/server/server_test.go:115-146` | Authentication handling is visible; this run did not audit every authorization path. |
| Untrusted input validated/encoded/rejected | MUST | partial | `internal/server/server.go:255-266`; `internal/store/keys.go:13-24` | Core surfaces validate/encode, but trust-boundary gaps remain. |
| Common attack classes assessed | MUST | partial | reviewed headers/cookies/proxy/env surfaces | Residual risk is still implicit rather than explicitly threat-modeled. |
| Privileged actions fail safely | MUST | partial | `internal/server/server.go:668-681`; `internal/server/api_auth.go:138-168` | Proxy/header trust remains too permissive. |
| Security-relevant configuration defaults safer | MUST | partial | `deploy/compose.yaml:6-10`; `deploy/entrypoint.sh:12-16` | The default deploy path is still not safe enough. |
| Known critical vulnerabilities block high score | MUST | pass | `.github/workflows/makefile.yaml:37-40` | CI continues to run security scanning. |
| Threat modeling exists for major trust boundaries | SHOULD | fail | no explicit trust-boundary artifact reviewed | Still implicit. |
| Abuse cases covered in tests | SHOULD | partial | `internal/server/server_test.go:22-39`; `internal/server/server_test.go:115-146` | Some security behavior is tested, but not as a broad abuse suite. |
| Security tooling runs in CI | SHOULD | pass | `.github/workflows/makefile.yaml:37-40` | gosec and govulncheck are real pipeline steps. |
| Residual risks recorded | SHOULD | partial | `reports/workflow/10-RECOMMENDATIONS.md` | Better than baseline, but still mostly report-driven. |

## Findings

### Strengths
- Security headers, CSP, and HSTS-on-secure requests are present in the server response path (`internal/server/server.go:255-266`).
- Session and CSRF cookies use explicit `HttpOnly`/`SameSite`/`Secure` behavior tied to request security (`internal/server/api_auth.go:138-168`, `internal/server/server_test.go:115-146`).
- CI still runs both `gosec` and `govulncheck` (`.github/workflows/makefile.yaml:37-40`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| `X-Forwarded-Proto` is trusted without trusted-proxy validation | medium | Misconfigured or untrusted proxy chains can influence secure-cookie and HSTS behavior | `internal/server/server.go:668-681` | Validate trusted proxies before honoring forwarded headers. |
| Chat subprocess inherits the full server environment | medium | Sensitive env values can leak into child processes configured for chat backends | `internal/server/chat_ws.go:232-237` | Whitelist child-process env instead of appending full `os.Environ()`. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| infosec | Env inheritance and proxy trust affect information-security posture | Carry forward env-leak concerns |
| devops | Proxy and release hardening require deployment/release changes | Decide trusted-proxy and immutable-image UX |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R5 | medium | Validate trusted proxies before honoring `X-Forwarded-Proto` | cyber | proxy config shape | Secure-cookie/HSTS decisions ignore untrusted forwarded headers |
| R6 | medium | Whitelist child-process env for chat/analyse subprocesses | infosec | subprocess config | Child processes receive only required env vars |

## Changes since last run
- The surrounding platform is healthier because OpenAPI validation, coverage gates, and browser runner stability improved elsewhere in the stack.
- The known-default deploy password blocker is closed; proxy trust and child-process env breadth are still the highest-leverage cyber issues.

## Open questions
- Should deploy docs move from mutable `latest` images to pinned production image references?

## Verdict
This repo still has real cyber posture - headers, cookies, and CI security scanning are not aspirational. The score stays flat because the default deploy path is still insecure, forwarded-proto trust is too implicit, and child-process env handling remains broader than it should be.
