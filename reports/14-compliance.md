# Compliance

**Score: 42/100**

## What is being assessed
GDPR compliance (right to erasure, data retention, portability, transparency), audit trail completeness and integrity, cookie consent, data processing documentation, license compliance, and SBOM existence.

## Methodology
Reviewed `internal/store/auth.go` (user deletion), `internal/store/encrypt.go` (encryption), all history/activity tables, `internal/server/api.go` for cookie handling, `LICENSE`, `go.mod` dependency licenses, and documentation for privacy policy or DPA.

## Findings

### Passing checks
- MIT License — clear, permissive, safe for commercial use
- All 7 direct dependencies: MIT (charmbracelet), Apache 2.0 (google/uuid), BSD (golang.org/x), BSD (modernc.org/sqlite) — no GPL, no copyleft
- No analytics, no tracking cookies, no third-party telemetry
- `localStorage` used only for UI state (theme, panel width) — not personal data
- Cookies: `HttpOnly`, `Secure`, `SameSite=Lax`, 30-day `MaxAge` — secure session management
- `AES-256-GCM` encryption available for email field via `TICKET_ENCRYPTION_KEY` env var (`internal/store/encrypt.go`)
- Audit trail exists: `history_events` table captures ticket creation, updates, assignments, comments
- Password hashing: Argon2id — personal data (password) is irreversibly protected
- Explicit logout deletes session token from database

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `DeleteUser()` does not cascade: leaves comments, time entries, history events, project memberships | Critical | `internal/store/auth.go` | Implement full cascading delete or anonymisation (replace user references with `[deleted]`) for GDPR Art. 17 |
| No data retention policy — all data kept indefinitely | Critical | repo-wide | Add `TICKET_HISTORY_RETENTION_DAYS`, `TICKET_SESSION_EXPIRY_DAYS` config vars with enforcement |
| No privacy documentation (`PRIVACY.md`, DPA template) | Critical | repo-wide | Create `docs/PRIVACY.md` documenting: data categories, processing purposes, retention periods, subject rights |
| No SBOM (Software Bill of Materials) | High | repo-wide | Generate SBOM with `syft` or `cyclonedx-go` as part of release pipeline |
| SQLite database stored unencrypted at rest | High | `internal/store/store.go` | For production: require disk encryption (dm-crypt/FileVault) or implement SQLCipher; document requirement |
| Session `expires_at` column exists but never enforced — sessions permanent until logout | High | `internal/store/auth.go:155-177` | Enforce `AND expires_at > CURRENT_TIMESTAMP` in token lookup |
| `TICKET_ENCRYPTION_KEY` optional — email stored plaintext if not set | Medium | `internal/store/encrypt.go` | Require encryption key in production mode; warn loudly if unset |
| Audit log records deleted when parent ticket/project deleted — no tamper evidence | Medium | `internal/store/project.go:328-361` | Archive rather than delete audit records; add HMAC chain for integrity |
| No data export API for GDPR portability (Art. 20) | Medium | repo-wide | Add `GET /api/users/{id}/export` returning all user's data as JSON |
| No cookie consent mechanism | Low | web UI | For EU deployments: add cookie notice (functional cookies only — no consent required, but notice needed) |

## Verdict
Compliant for open-source/internal use but not ready for GDPR-regulated production deployment. The critical gaps are incomplete user deletion (violates right to erasure), no data retention policy (violates storage limitation), and no privacy documentation. The encryption infrastructure exists but is not enforced. License and dependency compliance are clean.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Implement cascading user deletion / anonymisation | Critical | GDPR Art. 17 right to erasure |
| Add data retention policy and enforcement | Critical | Configurable TTL for sessions, history, old tickets |
| Create `docs/PRIVACY.md` | Critical | Data categories, purposes, retention, subject rights process |
| Generate SBOM in release pipeline | High | `syft packages . -o cyclonedx-json > sbom.json` |
| Enforce `TICKET_ENCRYPTION_KEY` in server mode | High | Log warning; refuse start in strict mode if key absent |
| Enforce session expiry | High | Add `expires_at` check to `GetUserByToken()` |
| Add `GET /api/users/{id}/export` | Medium | GDPR Art. 20 portability |
| HMAC chain on audit records | Medium | Tamper-evident audit log |
