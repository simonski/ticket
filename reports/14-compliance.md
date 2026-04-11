# Compliance

**Score: 72/100** (was 74)

## What is being assessed

GDPR right to erasure, data retention controls, audit trail completeness, cookie consent, license compliance, SBOM existence, data processing documentation, and PII handling in logs.

## Methodology

Read `internal/store/auth.go` in full. Searched `store.go` for `assignee` column. Read `server.go` logging handler. Read `docs/PRIVACY.md`. Checked for `TICKET_SESSION_EXPIRY_DAYS` implementation. Checked LICENSE and dependencies.

## Findings

### Passing checks
- MIT License present — `LICENSE`
- All dependencies permissively licensed (MIT/BSD/Apache) — `go.mod`
- `docs/PRIVACY.md` comprehensive — covers GDPR Arts 15-21, retention table, legal basis
- `DeleteUser()` transactional and broad: nullifies history `created_by`, deletes sessions/memberships/time entries — `auth.go:247-310`
- Audit trail preserved with PII removed (nullified, not deleted)
- Expired session purge via `PurgeExpiredSessions()` — `activity.go:202-211`
- `TICKET_HISTORY_RETENTION_DAYS` implemented — `server.go:88-96`
- Cookie: HttpOnly, Secure conditional, SameSite:Lax, MaxAge=30d — `api_auth.go:144-152`
- Cookie logout clears token (MaxAge=-1) — `api_auth.go:166-175`
- Request body logging opt-in only (`--verbose` flag) — `server.go:135-142`
- No analytics, telemetry, or tracking pixels
- Security headers on every response

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `tickets.assignee` not cleared on user delete — orphaned PII | High | `store.go:229`, `auth.go:247-310` | Add `UPDATE tickets SET assignee=''` before DELETE |
| `TICKET_SESSION_EXPIRY_DAYS` documented but not implemented — hardcoded 30 days | Medium | `PRIVACY.md:61,70` vs `auth.go:143-144` | Read env var and parameterise, or remove from docs |
| No SBOM file | Medium | Root dir | `cyclonedx-gomod mod -output sbom.json` |
| Verbose logging will log plaintext passwords on `/api/login` | Medium | `server.go:229-231` | Redact body for auth paths |
| No HMAC on `history_events.payload` — audit records tamperable | Low | `store.go:268-279` | Add optional HMAC-SHA256 column |
| No cookie consent banner | Low | `index.html` | Add notice (functional cookie exemption likely applies) |

## Verdict

Score drops 2 points to 72. The `tickets.assignee` GDPR gap and the `TICKET_SESSION_EXPIRY_DAYS` documentation-vs-implementation mismatch are the key issues. All other compliance foundations remain solid.

## Changes since last assessment
- `TICKET_SESSION_EXPIRY_DAYS` gap newly identified
- Verbose body-logging PII risk newly identified
- `tickets.assignee` still not cleared (unchanged)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Clear `assignee` on user delete | High | Add UPDATE to `DeleteUser()` transaction |
| Implement `TICKET_SESSION_EXPIRY_DAYS` | Medium | Read env var in `CreateSession()` |
| Redact passwords from verbose logging | Medium | Scrub body for auth paths |
| Generate SBOM | Medium | `cyclonedx-gomod` |
| Audit HMAC for history | Low | Optional signing column |
