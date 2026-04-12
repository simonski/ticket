# Compliance

**Score: 79/100** (was 80)

## What is being assessed

Privacy, retention, auditability, licensing, and evidence that the system can support ordinary operational and regulatory expectations without hidden data-handling surprises.

## Methodology

Reviewed audit, retention, and data-handling documentation plus the current server/store implementation and compared them with the previous compliance report baseline.

## Findings

### Passing checks
- **The project still carries explicit licensing and SBOM artifacts** — `LICENSE` and `sbom.json` remain present at the repo root
- **Audit-oriented activity history remains part of the product model** — ticket activity and event retrieval are still implemented in `internal/store/activity.go`
- **The docs set still includes security and operational guidance** — the repository continues to maintain security- and ops-adjacent docs in `docs/`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No documented retention or deletion policy for ticket/user data | High | docs set | Add an explicit retention-and-erasure policy covering `.ticket/`, server data, backups, and logs |
| No user-facing data export / erasure workflow | High | CLI/API surface | Add documented admin workflows for export and purge operations |
| Audit integrity is eventful but not tamper-evident | Medium | `internal/store/activity.go` | Add signed or chained audit records if compliance-grade integrity is required |
| Cookie/privacy guidance is incomplete for hosted deployments | Medium | docs / web deployment guidance | Document cookie usage, auth data handling, and operator responsibilities |
| Third-party dependency review is present via SBOM but not operationalised | Low | `sbom.json` | Add a documented dependency review cadence and ownership |

## Verdict

The project remains better than average on basic artifact hygiene, but it still lacks the documented privacy and retention controls that make a compliance story credible beyond engineering intent. The score dipped because the remaining gaps are still material and mostly procedural rather than purely technical.

## Changes since last assessment
- The same core compliance gaps remain open: retention, erasure, and stronger audit integrity
- The repo still has licensing/SBOM basics, but the operational compliance story has not materially advanced

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Missing retention/erasure policy | High | Publish a concrete data retention and deletion policy |
| Missing export/purge workflows | High | Add admin-facing export and purge commands/endpoints |
| Audit trail not tamper-evident | Medium | Add chained or signed audit records |
| Incomplete cookie/privacy guidance | Medium | Document operator responsibilities for hosted deployments |
| No documented dependency review cadence | Low | Add ownership and review frequency for SBOM/vulnerability review |
