# Database Engineer

**Score: 82/100** (was 80)

## Mission
Protect data correctness, durability, and query behavior in the SQLite-backed core.

## Review objective
Evaluate schema design, indexing, retention/migration behavior, and DB-level integrity guarantees.

## Inputs reviewed
- `internal/store/store.go`
- `internal/store/activity.go`
- `internal/store/ticket.go`
- `internal/server/api_system.go`

## Findings

### Passing checks
- SQLite is opened with WAL mode, foreign keys, busy timeout, and single-connection settings appropriate for the chosen operational model (`internal/store/store.go:21-53`).
- The schema carries a broad index set across tickets, sessions, history, labels, dependencies, roles, stages, and memberships (`internal/store/store.go:507-560`, `internal/store/store.go:660-783`).
- Ticket history reads are paginated instead of unbounded (`internal/store/activity.go:46-57`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Important domain rules are enforced in application code but not as DB constraints. | Medium | Out-of-band writers or future code paths can store invalid values the store layer would normally reject. | `internal/store/store.go:226-289`, `internal/store/ticket.go:1620-1627` | Add CHECK constraints for ticket type/state/stage where feasible. |
| Project prefix format is documented as constrained, but the schema does not enforce it. | Medium | The database can contain invalid prefixes if an alternate write path bypasses current validation. | `internal/store/store.go:226-247`, `docs/LIFECYCLE.md:14-15` | Add a DB-level CHECK or a migration-time validation for project prefixes. |
| The legacy `open` column still exists beside the newer lifecycle fields and is used by metrics. | Low | The schema still carries two overlapping notions of ticket openness, which complicates reasoning and reporting. | `internal/store/store.go:730-740`, `internal/server/api_system.go:46-49` | Decide whether `open` remains authoritative for any purpose; remove or derive it if not. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| domain-designer | DB constraints should reflect true domain invariants. | Which invariants are mandatory at DB level? |
| performance-engineer | The `open` field affects reporting/query semantics. | Should metrics derive from lifecycle fields instead? |
| backend-engineer | DB constraints will require corresponding error-shaping changes. | Constraint-error handling plan. |

## Verdict
The SQLite layer is one of the stronger parts of the system: setup, indexing, and pagination are thoughtful. The main improvement area is integrity hardening—today, too many important rules still live only in Go code.

## Changes since last assessment
- Confidence improved because the current schema/index review shows explicit pagination and broad index coverage, but the DB-constraint story is still softer than the domain would justify (`internal/store/activity.go:46-57`, `internal/store/store.go:507-560`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing DB CHECK constraints | Medium | Push key domain invariants into the schema. | database-engineer |
| Unenforced prefix rule | Medium | Add DB-level prefix validation. | database-engineer |
| Legacy `open` overlap | Low | Simplify or derive the field from the main lifecycle model. | backend-engineer |
