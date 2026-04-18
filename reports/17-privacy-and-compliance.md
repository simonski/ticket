# Privacy and Compliance

**Score: 74/100** (was 79)

## Mission
Protect accountable handling of personal data and ensure the documented privacy posture matches the implemented system.

## Review objective
Check retention, deletion, encryption, and policy/document alignment for user-related data.

## Inputs reviewed
- `docs/PRIVACY.md`
- `internal/store/auth.go`
- `internal/store/encrypt.go`
- `internal/server/server.go`
- `internal/server/api_auth.go`

## Findings

### Passing checks
- The repo has an explicit privacy document covering data categories, retention, and subject-right concepts (`docs/PRIVACY.md:1-129`).
- User deletion actively anonymizes audit trails and removes user-linked operational data in one transaction (`internal/store/auth.go:321-380`).
- History and session retention behavior is implemented with configurable purge logic on the server side (`internal/server/server.go:86-120`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The privacy document is visibly stale and still references an older product version and old CLI examples. | Medium | A compliance-facing document no longer matches the product users actually run. | `docs/PRIVACY.md:4-5`, `docs/PRIVACY.md:113-114` | Update the version/date and replace old command names with current `tk` examples. |
| Email encryption is optional rather than a default operational requirement. | Medium | A production deployment can store personal data unencrypted at rest if operators skip the env var. | `internal/store/encrypt.go:17-31`, `docs/PRIVACY.md:82-84` | Make the production recommendation stronger and add deployment-time checks or warnings. |
| Session cookie security still depends on proxy-header trust that is not fully validated. | Low | A misconfigured reverse proxy weakens the documented cookie/TLS posture. | `internal/server/api_auth.go:138-147`, `internal/server/server.go:551-565` | Couple secure-cookie decisions to trusted proxy validation and document the requirement in operator docs. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | The main privacy doc needs immediate drift cleanup. | Privacy doc update list. |
| security-engineer | Proxy/cookie trust affects privacy posture as well as security posture. | Trusted-proxy checklist. |
| devops-engineer | Encryption posture depends on deployment choices. | Production env validation/warning path. |

## Verdict
The codebase has meaningful privacy-aware behavior, especially around user deletion and retention. The weak point is policy fidelity: the repo says the right kinds of things, but the privacy document and deployment defaults do not yet support a high-confidence compliance story.

## Changes since last assessment
- The underlying code posture is stronger than the privacy document suggests, but the documentation lag is now large enough to drag the score down by itself (`internal/store/auth.go:321-380`, `docs/PRIVACY.md:4-5`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Stale privacy policy | Medium | Refresh the document to current version, commands, and dates. | tech-writer |
| Optional email encryption | Medium | Strengthen production enforcement or warnings. | devops-engineer |
| Proxy-trust cookie gap | Low | Validate reverse-proxy trust before treating requests as secure. | security-engineer |
