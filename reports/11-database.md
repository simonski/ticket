# Database

**Score: 60/100**

## What is being assessed
Schema design, index coverage, foreign key cascade behaviour, migration strategy, connection pool configuration, query parameterisation, N+1 patterns, pagination support, audit trail integrity, and full-text search capability.

## Methodology
Reviewed all files in `internal/store/` including `store.go` (schema + migrations), `ticket.go`, `project.go`, `auth.go`, `activity.go`. Listed all 20 tables, 26 indexes, and all foreign key relationships. Analysed migration approach and identified unbounded queries.

## Findings

### Passing checks
- SQLite WAL mode enabled: `PRAGMA journal_mode=WAL` — concurrent readers, single writer
- Foreign keys enforced: `PRAGMA foreign_keys=ON`
- `busy_timeout=5000ms` — handles lock contention gracefully
- `MaxOpenConns=1`, `MaxIdleConns=1` — correct for SQLite single-writer model
- 26 indexes defined covering: sessions, tickets (project_id, parent_id, assignee, stage, state), comments, dependencies, time entries, workflow stages
- All user-input queries use `?` placeholders — no SQL injection via user data
- Transactions used for `CreateTicket`, `UpdateTicket`, `DeleteTicket`, `DeleteProject`
- Soft delete (`open`/`archived` fields) with hard-delete option via `DeleteTicket()`
- `DELETE` operations wrap dependent table cleanup in transactions
- `defer Rollback()` pattern used correctly on all transaction code paths
- Schema initialised with `CREATE TABLE IF NOT EXISTS` — idempotent on every boot
- Audit trail: `history_events` and `ticket_history` tables capture create/update/archive events

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Missing indexes on `tickets.open`, `tickets.archived`, `tickets.status`, `tickets.type` | Critical | `internal/store/store.go` schema | Add 4 indexes — these fields appear in WHERE clauses of most list queries; currently full table scans |
| Missing `user_id` indexes on `project_members`, `team_members`, `team_agents` | High | `internal/store/store.go` | Add `idx_project_members_user_id`, `idx_team_members_user_id`, `idx_team_agents_user_id` |
| Missing `idx_ticket_labels_ticket_id` | High | `internal/store/store.go` | Add index — label lookups per ticket do full table scan |
| Missing `idx_users_username` | High | `internal/store/store.go` | Authentication path lookups by username do full scan |
| No `ON DELETE CASCADE` on any foreign key | High | All FK definitions | Add `ON DELETE CASCADE` to child table FKs; eliminates 10-step manual delete chains |
| `history_events` and `ticket_history` tables are duplicates | High | `internal/store/store.go`, `activity.go` | Consolidate into one table; `AddHistoryEvent` writes to both without a transaction — inconsistency risk |
| SQL string concatenation in `PRAGMA table_info()` calls | Medium | `internal/store/store.go:1595,1565` | Validate `tableName` against alphanumeric+underscore allowlist before use |
| No migration versioning — all migrations run on every boot | Medium | `internal/store/store.go:105-546` | Add `schema_version` table; skip already-applied migrations |
| `AddHistoryEvent()` writes to 2 tables without a transaction | Medium | `internal/store/activity.go` | Wrap both INSERTs in a single transaction |
| No full-text search — LIKE `%pattern%` cannot use indexes | Medium | `internal/store/ticket.go` | Implement FTS5 virtual table for ticket title/description search |
| OFFSET-based pagination is O(n) for large datasets | Low | `internal/store/ticket.go` | Switch to cursor-based: `WHERE ticket_id > ? ORDER BY ticket_id` |
| Audit log records deleted when parent ticket/project deleted | Medium | `internal/store/project.go:328-361` | Archive rather than delete audit records; use soft-delete on history |

## Verdict
Functional schema with good WAL configuration and parameterised queries. The critical gaps are missing indexes on the most-queried columns (`open`, `archived`, `status`) causing full table scans on every ticket list, duplicate audit tables creating inconsistency risk, and the absence of ON DELETE CASCADE requiring fragile manual delete chains. Adding the 9 missing indexes is the highest-leverage fix available.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add 9 missing indexes | Critical | `idx_tickets_open`, `idx_tickets_archived`, `idx_tickets_status`, `idx_tickets_type`, `idx_project_members_user_id`, `idx_team_members_user_id`, `idx_team_agents_user_id`, `idx_ticket_labels_ticket_id`, `idx_users_username` |
| Consolidate `history_events` + `ticket_history` | High | Single table; wrap writes in transaction |
| Add `ON DELETE CASCADE` to FKs | High | Eliminates manual multi-step delete chains |
| Schema migration versioning | Medium | `schema_version` table; idempotent migration runner |
| FTS5 for ticket search | Medium | Replace LIKE with FTS5 virtual table |
| Fix `PRAGMA table_info` concatenation | Medium | Validate against alphanumeric allowlist |
| Wrap `AddHistoryEvent` in transaction | Medium | Prevents partial audit log writes |
