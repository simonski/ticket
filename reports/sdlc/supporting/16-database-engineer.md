# Database Engineer

**Score: 78/100** (was 74)

## Mission
Protect data correctness, durability, query behavior, and recovery safety.

## Review objective
Assess schema controls, SQLite configuration, migration/versioning, backups, and query risks.

## Inputs reviewed
- `internal/store`
- `docs/RUNBOOKS.md`
- `Makefile`
- `TESTING.md`

## Findings

### Passing checks
- SQLite connections enable foreign keys and a busy timeout (`internal/store/schema_version.go:40-56`).
- WAL mode support exists (`internal/store/schema_version.go:59-61`).
- Schema version errors explicitly block stale/newer binary mismatches (`internal/store/schema_version.go:22-38`).
- Backup and restore runbooks are documented (`docs/RUNBOOKS.md:126-180`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| SQLite concurrency ceiling is documented but not proven by load tests. | Medium | Operators may exceed single-writer assumptions and see lock contention. | `docs/SLO.md:88-97`, `internal/store/schema_version.go:46-56` | Add a simple write-concurrency/load test and scaling trigger. |
| Restore flow can be unsafe if run against active server. | Medium | Operator may import while server is using the DB. | `docs/RUNBOOKS.md:163-180` | Make restore instructions explicitly require stopped server and isolated target DB. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| performance-engineer | Need concurrency baseline. | SQLite write-load result. |
| sre | Restore safety affects incident response. | Restore runbook update. |

## Verdict
Database fundamentals are strong for the intended SQLite deployment. The next maturity step is operational proof under concurrent write/load and safer restore guidance.

## Changes since last assessment
- Coverage for store remains above its 70% gate at 71.1%.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Unproven concurrency ceiling | Medium | Add load tests and document thresholds. | database-engineer |
