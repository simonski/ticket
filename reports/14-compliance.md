# Compliance

**Score: 62/100** (was 42)

## What is being assessed
GDPR compliance (right to erasure, data retention, portability, transparency), audit trail completeness and integrity, cookie consent implications, data processing documentation, license compliance, and SBOM existence/freshness.

## Methodology
Reviewed `internal/store/auth.go` (user deletion cascade), `internal/store/encrypt.go` (AES-256-GCM email encryption), `internal/store/activity.go` (retention purge), `internal/server/server.go` (purge scheduler), `internal/store/store.go` (schema/FKs), `LICENSE`, `go.mod` (dependency licenses), `dist/sbom.cdx.json`, and `Makefile` (release-sbom target). Version under review: 0.1.737; SBOM in dist/ is for 0.1.733.

## Findings

### Passing checks
- MIT License — permissive, safe for commercial use (LICENSE)
- All direct dependencies use permissive licenses: charmbracelet suite (MIT), google/uuid (Apache 2.0), golang.org/x (BSD), modernc.org/sqlite (BSD/MIT) — no GPL, no copyleft (go.mod)
- No analytics, no tracking cookies, no third-party telemetry
- `localStorage` used only for UI state (theme, panel width) — not personal data
- Session cookie set `HttpOnly`, 30-day expiry; session expiry enforced in `GetUserByToken()` via `AND (s.expires_at IS NULL OR s.expires_at > CURRENT_TIMESTAMP)` (internal/store/auth.go:169)
- AES-256-GCM encryption available for email field via `TICKET_ENCRYPTION_KEY` env var (internal/store/encrypt.go)
- Password hashing: Argon2id — irreversibly protected personal data (internal/password/)
- Explicit logout deletes session token from database (internal/store/auth.go:DeleteSession)
- `DeleteUser()` runs in a transaction and cascades: deletes sessions, project_members, team_members, team_agents, time_entries, agent_config, messages, comments; anonymises `history_events.created_by` and `ticket_history.created_by` (NULL); nullifies `tickets.created_by` (internal/store/auth.go:247–302)
- `PurgeOldHistory()` deletes `ticket_history` records older than `TICKET_HISTORY_RETENTION_DAYS` days (internal/store/activity.go:214)
- `runRetentionPurge()` called at server startup and on a periodic ticker — purges expired sessions AND old history events (internal/server/server.go:76–107)
- SBOM generated via `cyclonedx-gomod` as part of release pipeline (`make release-sbom`) and uploaded to GitHub release (Makefile:166–169, 197)
- `dist/sbom.cdx.json` present: CycloneDX 1.6 format, full module dependency list with hashes (dist/sbom.cdx.json)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `tickets.assignee` is a plain TEXT username, not a FK — not cleared when user is deleted; deleted user's name lingers on assigned tickets | High | `internal/store/store.go:224`, `internal/store/auth.go:DeleteUser` | Add `UPDATE tickets SET assignee = '' WHERE assignee = ?` with the deleted username inside the `DeleteUser` transaction |
| No privacy documentation (`PRIVACY.md` or DPA template) | High | repo-wide | Create `docs/PRIVACY.md` documenting: data categories, processing purposes, retention periods, subject rights process (access, erasure, portability) |
| No data export API (GDPR Art. 20 portability) | High | repo-wide | Add `GET /api/users/{id}/export` returning all the user's tickets, comments, time entries, and activity as JSON |
| SBOM in `dist/` is stale — generated for v0.1.733, current release is v0.1.737 | Medium | `dist/sbom.cdx.json` | Regenerate SBOM as part of every release; CI should fail if `dist/sbom.cdx.json` version does not match `cmd/ticket/VERSION` |
| `TICKET_ENCRYPTION_KEY` optional — email stored plaintext by default | Medium | `internal/store/encrypt.go` | Log a `WARN` at server startup if key is absent; add a `--strict` / `TICKET_STRICT=1` mode that refuses to start without it |
| SQLite database stored unencrypted at rest — no documentation of this requirement | Medium | `internal/store/store.go` | Add a note to deployment docs requiring OS-level disk encryption (dm-crypt / FileVault) for any multi-user server |
| History events deleted when parent ticket is deleted — no tamper-evident audit trail | Medium | `internal/store/store.go:1405` | Archive rather than delete audit records on ticket deletion; consider HMAC chaining for integrity |
| No cookie consent notice/banner for EU deployments | Low | `web/static/index.html` | Add a dismissible notice explaining the single functional session cookie (no consent required for strictly necessary cookies, but notice is best practice) |
| `TICKET_HISTORY_RETENTION_DAYS` defaults to 0 (disabled) — no guidance on recommended value | Low | `internal/server/server.go:87` | Document the env var in README/USER_GUIDE; recommend a default (e.g. 365 days) for production |

## Verdict
Significant improvement from 42 → 62 since the last assessment. The critical gaps around user deletion cascade and session expiry are resolved; an SBOM is now generated and published with each release; and a configurable history retention mechanism exists and runs automatically. The remaining blockers for GDPR-regulated production deployment are: clearing the `assignee` field on user deletion, providing a data-export endpoint (Art. 20), and publishing a privacy notice. Encryption and retention infrastructure is present but needs hardening for default-on behaviour.

## Changes since last assessment
- **SBOM generated and published** (`make release-sbom` + `cyclonedx-gomod`): resolves the previous High "No SBOM" finding. CycloneDX 1.6 JSON uploaded to GitHub releases (Makefile:166–197)
- **`DeleteUser()` properly cascades** (internal/store/auth.go): deletes sessions, memberships, time entries, messages, comments; anonymises audit trail `created_by` — substantially addresses the previous Critical "incomplete user deletion" finding
- **Session expiry enforced** (internal/store/auth.go:169): `AND (s.expires_at IS NULL OR s.expires_at > CURRENT_TIMESTAMP)` present in `GetUserByToken` — resolves previous High finding (this was already fixed prior to 0.1.730 but mis-stated in last report)
- **`PurgeOldHistory()` integrated**: called in `runRetentionPurge()` on server startup and ticker (internal/server/server.go:93–96) — partially resolves previous Critical "no data retention" finding; still opt-in via env var

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Clear `tickets.assignee` on user deletion | High | Add `UPDATE tickets SET assignee = ''` in `DeleteUser` transaction |
| Create `docs/PRIVACY.md` | High | Data categories, purposes, retention periods, subject rights |
| Add `GET /api/users/{id}/export` | High | GDPR Art. 20 portability endpoint |
| Regenerate SBOM on every release; enforce version match | Medium | CI check: sbom version == VERSION file |
| Warn / refuse start if `TICKET_ENCRYPTION_KEY` absent | Medium | `slog.Warn` at startup; optional strict mode |
| Document disk encryption requirement for production | Medium | Add note to QUICKSTART_SERVER.md |
| Archive (not delete) audit records on ticket deletion | Medium | Tamper-evident audit trail |
| Add cookie consent notice | Low | Dismissible banner for EU deployments |
| Document and default `TICKET_HISTORY_RETENTION_DAYS` | Low | Recommend 365 days in deployment docs |
