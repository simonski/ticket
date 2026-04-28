# Privacy And Compliance

**Score: 74/100** (was 71)

## Mission
Protect lawful, accountable data handling and clear operator responsibilities.

## Review objective
Review data categories, retention, deletion, minimisation, security docs, and self-hosting obligations.

## Inputs reviewed
- `docs/PRIVACY.md`
- `SECURITY.md`
- `internal/store/auth.go`
- `internal/server/server.go`

## Findings

### Passing checks
- Privacy doc names data categories and storage locations (`docs/PRIVACY.md:29-43`).
- Retention variables are documented, including history retention (`docs/PRIVACY.md:57-72`).
- TLS and disk-encryption expectations are documented for production (`docs/PRIVACY.md:76-91`).
- Vulnerability reporting channel is private-first (`SECURITY.md:8-19`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| History retention defaults to indefinite. | Medium | Operators may violate minimisation/storage-limitation expectations if they never configure retention. | `docs/PRIVACY.md:57-64`, `internal/server/server.go:87-105` | Make deployment checklist require a retention decision. |
| Privacy doc version lags binary version. | Low | Published policy can look stale. | `docs/PRIVACY.md:3-5`, `cmd/tk/VERSION:1` | Sync policy metadata during release prep. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| sre | Retention is operational configuration. | Default retention checklist. |
| tech-writer | Metadata drift needs docs cleanup. | Version sync task. |

## Verdict
Privacy documentation is unusually explicit for a self-hosted tool. It still relies on operators making retention/encryption decisions correctly.

## Changes since last assessment
- Privacy/SLO/runbook coverage is stronger than the original baseline.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Indefinite history by default | Medium | Require documented retention decision in deploy guide. | privacy-and-compliance |
