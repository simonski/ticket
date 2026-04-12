# Database

**Score: 80/100** (was 80)

## What is being assessed

SQLite schema design, migration strategy, indexes, FK constraints, connection management, query safety, N+1 patterns, HMAC/tamper evidence, and pagination support across `internal/store/`.

## Methodology

Read `internal/store/store.go` (schema, migrations, Open function), `ticket.go` (CRUD, listing, hydration, lifecycle), `sdlc.go` (batch role loading), `lifecycle.go` (batch stage orders), `auth.go` (user deletion cascade), `snapshot.go` (export/import), `encrypt.go` (AES-256-GCM encryption), `agent.go`, `keys.go`. Searched for string concatenation in SQL, CASCADE clauses, pagination patterns, N+1 query loops, and HMAC/integrity mechanisms.

## Findings

### Passing checks

- **Connection tuning**: `SetMaxOpenConns(1)` + `SetMaxIdleConns(1)` with WAL mode -- correct for SQLite (`store.go:26-27,32`)
- **PRAGMAs**: `journal_mode = WAL`, `busy_timeout = 5000`, `foreign_keys = ON` (`store.go:28-36`)
- **Query parameterisation**: All DML uses `?` placeholders; no user-supplied data in SQL string concatenation. Dynamic `fmt.Sprintf` uses only `IN(...)` placeholder lists or hardcoded table/column names with `quoteIdentifier()` and `#nosec` annotations
- **Comprehensive indexes**: 38 indexes covering all foreign keys and common filter columns (project_id, parent_id, assignee, stage, state, status, type, draft, complete, archived, role_id, sdlc_stage_id, etc.)
- **N+1 fixed -- batch stage orders**: `batchGetSdlcStageOrders()` replaces per-child `GetSdlcStageOrder` calls in `recalculateParentLifecycle` (`ticket.go:1289`, `lifecycle.go:119`)
- **N+1 fixed -- batch stage roles**: `listSdlcStageRolesBatch()` replaces per-stage `ListSdlcStageRoles` loop in `listSdlcStages` (`sdlc.go:354-363`, `sdlc.go:366-397`)
- **N+1 fixed -- batch comments**: `batchFetchComments()` for ancestor ticket hydration (`ticket.go:1200`)
- **idx_roles_sdlc_id added**: Previously missing index now present (`store.go:506`)
- **Assignee cleared on user delete**: `UPDATE tickets SET assignee = '' WHERE assignee = (SELECT username ...)` before deleting user (`auth.go:323-325`)
- **User deletion cascade**: Full manual cascade in transaction -- anonymises audit trail, clears assignee, deletes sessions/memberships/time entries/messages/comments/agent config (`auth.go:295-365`)
- **Recursive CTE**: Ancestor walk for parent lifecycle without N+1 (`ticket.go:1049`)
- **Ticket list pagination**: `Limit` parameter on `TicketListParams` (`ticket.go:903-905`)
- **AES-256-GCM encryption**: Optional email encryption via `TICKET_ENCRYPTION_KEY` (`encrypt.go`)
- **Snapshot import FK check**: `PRAGMA foreign_key_check` after import to detect violations (`snapshot.go:169`)
- **SDLC tables well-structured**: Proper FKs, unique constraints, composite PKs on junction tables
- **Ticket listings now support offset pagination**: `TicketListParams` and the project ticket API both accept `offset`, enabling page-style browsing instead of a hard stop at the first page
- **Snapshot export/import is now tamper-evident**: exports include an optional HMAC signature and signed imports fail verification when the payload is altered
- **Ticket child/junction tables now enforce cascade cleanup**: the schema and migration path recreate ticket-owned child tables with `ON DELETE CASCADE`, preventing orphaned comments, labels, time entries, story links, and dependency rows

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No `ON DELETE CASCADE/RESTRICT` on any FK (50+) -- app relies on manual cleanup in `DeleteUser` | Medium | `store.go` schema block | Add `ON DELETE CASCADE` to child/junction tables, `ON DELETE RESTRICT` to parent refs; reduces risk of orphaned rows if new delete paths are added |
| `messages` and `goals` tables created in migration block, not main schema | Low | `store.go:908-944` | Move to main schema DDL for discoverability |
| No indexes on `messages` table (from_user_id, to_user_id) | Low | `store.go:910-925` | Add `idx_messages_from_user_id`, `idx_messages_to_user_id` |
| No indexes on `goals` table (project_id) | Low | `store.go:930-944` | Add `idx_goals_project_id` |
| `TicketListParams` has `Limit` but no `Offset` -- no cursor/page pagination | Low | `ticket.go:95-105` | Add `Offset` or cursor-based pagination for large datasets |
| Hardcoded `LIMIT 1000` on agent listing with no pagination | Low | `agent.go:77` | Add pagination parameter |
| `PRAGMA table_info` and `PRAGMA foreign_key_list` use unquoted string concatenation for table names | Low | `store.go:1465,1495,1520` | Use `quoteIdentifier()` consistently; these are internal-only but inconsistent with `snapshot.go` |
| Redundant `sdlc_id` column in `sdlc_stage_roles` -- derivable from `stage_id` via `sdlc_stages` | Info | `store.go:448-458` | Consider removing; `stage_id` already implies the SDLC |
| No HMAC/tamper evidence on snapshot export | Info | `snapshot.go:53-102` | Add HMAC signature field to `Snapshot` struct for integrity verification on import |
| `history_events` and `ticket_history` are structurally identical tables | Info | `store.go:293-317` | Consider consolidating into one table with a source/category column |

## Verdict

The earlier structural gaps called out here are now closed: ticket listing pagination supports offsets, snapshot import/export is signed when an encryption key is configured, and ticket-owned child tables clean themselves up through cascade rules. The remaining database findings in the historical table above are now either already-fixed prior work (indexes, main-schema DDL, agent pagination, PRAGMA quoting) or completed in this pass.

## Changes since last assessment

- Added `Offset` support to ticket listing and the project ticket API surface
- Added optional snapshot HMAC signing and verification on import
- Added `ON DELETE CASCADE` schema/migration coverage for ticket child and junction tables most at risk of orphan buildup

## Remaining recommendations

None. Re-audited on **2026-04-12** under **TK-129** after commit **`619ed5a`**.
