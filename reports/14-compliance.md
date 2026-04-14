# Compliance

**Score: 79/100** (was 79)

## What is being assessed
Privacy, retention, erasure, auditability, and license/SBOM evidence, with attention to whether the project’s documented compliance story matches what the code can actually do.

## Methodology
Reviewed `docs/PRIVACY.md`, `LICENSE`, `sbom.json`, user-deletion logic, history storage, and snapshot signing to verify the current compliance posture against the previous baseline.

## Findings

### Passing checks
- **The project ships the expected legal/composition artifacts** — both `LICENSE` and `sbom.json` are present at the repository root.
- **Retention controls are documented with concrete knobs** — the privacy policy names both session and history retention environment variables (`docs/PRIVACY.md:57-72`).
- **User deletion still does real cleanup plus audit anonymisation** — deleting a user nulls history references and removes sessions, memberships, time entries, messages, comments, and related records (`internal/store/auth.go:310-366`).
- **Snapshot export/import is tamper-evident when configured** — snapshots are signed and verified with HMAC before import (`internal/store/snapshot.go:113-131`, `internal/store/snapshot.go:242-276`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The privacy policy metadata is stale and still uses old `ticket` CLI examples for rights workflows | Medium | `docs/PRIVACY.md:4-5`, `docs/PRIVACY.md:113-114` | Update the version/date and rewrite the rights examples to use the current `tk` command surface. |
| Ticket/history audit records are preserved but not individually signed or chained | Medium | `internal/store/activity.go:34-42` | Add record-level signing/chaining if tamper-evident audit history is a compliance requirement beyond snapshot integrity. |

## Verdict
The compliance posture is credible for a self-hosted project of this size because retention controls, erasure semantics, and SBOM/licensing artifacts are all concrete. The remaining weakness is specialist-document freshness and the fact that audit integrity stops at snapshot signing rather than per-record protection.

## Changes since last assessment
- Reconfirmed the privacy policy now documents retention knobs and self-hosted controller responsibilities (`docs/PRIVACY.md:57-72`, `docs/PRIVACY.md:117-129`).
- Reconfirmed user deletion still anonymises audit references while removing personal records (`internal/store/auth.go:327-366`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Stale privacy metadata/examples | Medium | Refresh `docs/PRIVACY.md` so its header and command examples match the current product. |
| Audit history integrity gap | Medium | Add record-level signing/chaining if compliance expectations require tamper-evident history, not just signed snapshots. |
