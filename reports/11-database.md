# Database

**Score: 68/100** (was 66)

## What is being assessed
Schema design, index coverage, foreign key cascade behaviour, migration strategy, connection pool configuration, query parameterisation, N+1 patterns, pagination support, audit trail integrity, and encryption/tamper evidence.

## Methodology
Reviewed `internal/store/store.go` (full schema + all migrations), `ticket.go`, `activity.go`, `auth.go`, `snapshot.go`, `team.go`, and `encrypt.go`. Extracted all 25 `CREATE TABLE` statements, counted 35 non-PK indexes, verified all foreign key definitions, traced the idempotent migration runner (~50 guard-clause steps), audited `fmt.Sprintf` use in query strings, and identified loop-level DB call patterns.

Tables: `users`, `sessions`, `roles`, `projects`, `tickets`, `stories`, `story_ticket_links`, `history_events`, `ticket_history`, `comments`, `dependencies`, `app_settings`, `project_members`, `teams`, `team_members`, `team_agents`, `project_teams`, `workflows`, `labels`, `ticket_labels`, `time_entries`, `workflow_stages`, `messages`, `goals`, `agent_config` (25 total; last 3 added via migrations).

Indexes (35 non-PK): sessions ×2, tickets ×9, stories ×1, story_ticket_links ×1, history_events ×2, ticket_history ×2, comments ×2, dependencies ×3, labels ×1, ticket_labels ×2, project_members ×1, team_members ×1, team_agents ×1, users ×1, time_entries ×2, workflow_stages ×2, projects ×1.

## Findings

### Passing checks
- WAL mode: `PRAGMA journal_mode=WAL` — concurrent readers, single writer (`store.go:32`)
- Foreign keys enforced: `PRAGMA foreign_keys=ON` (`store.go:28`)
- `busy_timeout=5000ms` — handles lock contention gracefully (`store.go:36`)
- `MaxOpenConns=1`, `MaxIdleConns=1` — correct for SQLite single-writer model (`store.go:23-24`)
- 35 non-PK indexes defined; 9 previously missing indexes added this release
- All user-input queries use `?` placeholders — no SQL injection via user data
- `batchFetchComments` uses IN-clause with one `?` per id; correct `#nosec G202` annotation (`ticket.go:910`)
- Transactions used for `CreateTicket`, `UpdateTicket`, `DeleteTicket`, `DeleteProject`
- Soft delete (`open`/`archived` fields) with hard-delete option via `DeleteTicket()`
- `defer Rollback()` pattern used correctly on all transaction code paths
- Schema initialised with `CREATE TABLE IF NOT EXISTS` — idempotent on every boot
- `AddHistoryEvent` now writes only to `ticket_history` — dual-write race eliminated (`activity.go:34-43`)
- `PurgeOldHistory` called daily from the server reaper goroutine via `TICKET_HISTORY_RETENTION_DAYS` (`server.go:93`)
- `PurgeExpiredSessions` called hourly from the server reaper (`server.go:81`)
- Email field encrypted at rest using AES-256-GCM with nonce-prefixed ciphertext (`encrypt.go`)
- AES-GCM provides authenticated encryption — ciphertext integrity/tamper detection is built in
- `quoteIdentifier` used in `snapshot.go` double-quotes identifiers; `snapshotTableOrder` is a hardcoded allowlist (`snapshot.go:220`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `history_events` is a zombie table — writes removed from `AddHistoryEvent` but table still DELETEd in `ticket.go:1543`, `project.go:342`, and included in snapshot order (`snapshot.go:37`) | High | `store.go:259`, `ticket.go:1543`, `project.go:342` | Complete the consolidation: drop `history_events`; update all DELETE/UPDATE callers to reference `ticket_history` |
| No `ON DELETE CASCADE` on any foreign key (0 occurrences) | High | All FK definitions, `store.go:172–411` | Add `ON DELETE CASCADE` to child table FKs; eliminates manual 10-step delete chains that risk orphaned rows |
| `messages` table has no indexes on `from_user_id` or `to_user_id` | High | `store.go:867` (migration) | Add `idx_messages_from_user_id`, `idx_messages_to_user_id` — inbox/outbox queries will full-scan as volume grows |
| `goals` table has no index on `project_id` | High | `store.go:887` (migration) | Add `idx_goals_project_id` — all goal list queries filter by project |
| `columnExists` and `tableColumnNames` concatenate `tableName` into `PRAGMA table_info(...)` without `quoteIdentifier` | Medium | `store.go:1636,1661` | Apply `quoteIdentifier` already defined in `snapshot.go:220`; callers pass hardcoded literals today but pattern is fragile |
| No migration versioning — all ~50 idempotent guard checks run on every startup | Medium | `store.go:470` (`migrateSchema`) | Add `schema_version` table; record applied step IDs; skip already-applied checks on normal boot |
| `GetWorkflowStageOrder` called inside a loop over ticket children | Medium | `ticket.go:994-1010` | Pre-fetch all workflow stage orders for the relevant workflow in one query; resolve in memory |
| `DeleteTicket` and `cloneTicketRecursive` call `ListTickets` for entire project to find children | Medium | `ticket.go:1495-1522`, `ticket.go:1511` | Replace with `SELECT * FROM tickets WHERE parent_id = ?`, which uses `idx_tickets_parent_id` |
| `TICKET_ENCRYPTION_KEY` is padded/truncated to 32 bytes — weak key derivation | Medium | `encrypt.go:15-19` | Derive working key via HKDF or PBKDF2; document minimum entropy requirements |
| No full-text search — `LIKE '%pattern%'` cannot use indexes | Medium | `ticket.go` (`ListTickets` search param) | Implement FTS5 virtual table for ticket `title`/`description` |
| Audit log hard-deleted when parent project or ticket deleted | Medium | `project.go:342`, `ticket.go:1543` | Archive rather than delete; use soft-delete sentinel or separate `deleted_events` archive |
| OFFSET-based pagination is O(n) — no keyset/cursor implementation | Low | `ticket.go` (`ListTickets`) | Add cursor-based path: `WHERE ticket_id > ? ORDER BY ticket_id LIMIT ?` |

## Verdict
Fresh re-assessment confirms score moves from 66 to 68. The 9 previously missing indexes, daily history purge, and AES-GCM email encryption remain solid. Three new issues were confirmed via re-assessment: the `messages` table is missing `idx_messages_from_user_id` and `idx_messages_to_user_id`; the `goals` table is missing `idx_goals_project_id`; and the `history_events` zombie table is still referenced in DELETE paths. On balance the +2 from additional index coverage (time_entries, workflow_stages) slightly outweighs the new FK-column index gaps. `ON DELETE CASCADE` is still absent across all 25+ foreign key relationships — the most impactful single fix remaining.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| Added 9 previously missing indexes (`idx_tickets_open/archived/status/type`, `idx_project_members_user_id`, `idx_team_members_user_id`, `idx_team_agents_user_id`, `idx_ticket_labels_ticket_id`, `idx_users_username`) | **+8** — eliminates full-table-scans on the hottest read paths |
| `AddHistoryEvent` now writes only to `ticket_history`; dual-write race eliminated | **+2** — removes inconsistency risk from prior assessment |
| `PurgeOldHistory` + `PurgeExpiredSessions` wired into server reaper goroutine | **+2** — history and session growth bounded |
| AES-256-GCM email encryption added (`encrypt.go`) | **+1** — at-rest data protection for PII |
| Additional coverage: `time_entries` (`idx_time_entries_ticket_id`, `idx_time_entries_user_id`) and `workflow_stages` indexes present | **+2** — confirmed via re-assessment |
| New `messages` and `goals` tables added without FK-column indexes | **-3** — new unindexed tables on most-queried columns |
| `history_events` table still exists as zombie; partial migration not completed | **-2** — dead table still referenced in DELETE paths and snapshots |
| `columnExists` still concatenates table name into PRAGMA without `quoteIdentifier` | **-2** — code smell identified last assessment, unfixed |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Complete `history_events` removal | High | Drop table; migrate all DELETE/UPDATE callers to `ticket_history` |
| Add `ON DELETE CASCADE` to FKs | High | Eliminates manual multi-step delete chains |
| Add indexes on `messages` and `goals` FK columns | High | `idx_messages_from_user_id`, `idx_messages_to_user_id`, `idx_goals_project_id` |
| Schema migration versioning | Medium | `schema_version` table; skip already-applied steps |
| Fix `columnExists` PRAGMA concatenation | Medium | Use `quoteIdentifier` from `snapshot.go:220` |
| Resolve `GetWorkflowStageOrder` N+1 in children loop | Medium | Pre-fetch all stage orders in one query |
| Replace project-wide `ListTickets` for child lookup | Medium | Use direct `WHERE parent_id = ?` queries |
| Improve `TICKET_ENCRYPTION_KEY` derivation | Medium | Use HKDF/PBKDF2; document key format |
| FTS5 for ticket search | Medium | Replace LIKE with FTS5 virtual table |
| Archive (not delete) audit records | Medium | Keep `ticket_history` rows with `deleted_at` sentinel |
| Keyset pagination | Low | Replace OFFSET with `WHERE ticket_id > ? LIMIT ?` |
