# Compliance

**Score: 74/100** (was 74)

## What is being assessed
GDPR compliance (right to erasure, data retention, data minimisation), audit trail integrity (HMAC), cookie consent, license compliance, and SBOM currency.

## Methodology
Read `docs/PRIVACY.md`, `LICENSE`, `internal/store/auth.go` (DeleteUser). Grepped for HMAC, `assignee` cleanup, purge functions. Checked for SBOM.

## Findings

### Passing checks
- `docs/PRIVACY.md` exists and is comprehensive (130 lines): documents right to erasure, session expiry, data retention config, cookie policy (`docs/PRIVACY.md`)
- `DeleteUser()` comprehensively cleans personal data: nullifies audit trail references, removes sessions, memberships, time entries, messages, comments in a transaction (`internal/store/auth.go:243-310`)
- Data retention configurable: `TICKET_SESSION_EXPIRY_DAYS`, `TICKET_HISTORY_RETENTION_DAYS` env vars (`docs/PRIVACY.md:57-72`)
- Background purge job runs daily for expired sessions and old history (`internal/server/server.go:82-96`)
- Cookie flags documented in `PRIVACY.md` and implemented: `HttpOnly`, `SameSite=Lax`, `Secure` when TLS active (`internal/server/api_auth.go:144-152`)
- MIT License present and clear (`LICENSE`)
- `go.mod` tracks all dependencies

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `tickets.assignee` is TEXT (username), not FK — not cleared on user deletion | High | `internal/store/auth.go:243-310`, `store.go:223` | Add `UPDATE tickets SET assignee = '' WHERE assignee = (SELECT username FROM users WHERE user_id = ?)` in `DeleteUser()` |
| No SBOM file committed to repository | Medium | Root dir | Generate `sbom.json` with `syft` or `cyclonedx-gomod`; commit and update in release pipeline |
| No HMAC on `history_events.payload` — audit records silently tamperable | Medium | `internal/store/store.go:264,277` | Add HMAC-SHA256 signature field to `history_events` |
| No explicit cookie consent banner or opt-in mechanism | Low | Web UI | Add cookie notice on first visit referencing `PRIVACY.md` |

## Verdict
Score holds at 74. The `PRIVACY.md` additions and `DeleteUser()` implementation are strong GDPR foundations. The key remaining gap is the `assignee` field not being cleared when a user is deleted — orphaned username strings remain visible in ticket views after the user account is removed.

## Changes since last assessment
- `docs/PRIVACY.md` confirmed comprehensive and well-structured
- `DeleteUser()` scope confirmed: sessions, memberships, messages, comments cleaned — but `assignee` missed
- SBOM still not generated or committed
- HMAC on audit trail still absent

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Clear `assignee` on user delete | High | Add UPDATE to `DeleteUser()` transaction in `auth.go` |
| Generate and commit SBOM | Medium | `cyclonedx-gomod mod -output sbom.json`; add to `make release` |
| Audit HMAC | Medium | Add optional HMAC signing to `history_events.payload`; verify on read in compliance-sensitive deployments |
