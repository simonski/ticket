# Database

**Score: 72/100** (was 60)

## What is being assessed

SQLite schema design, migration strategy, indexes, FK constraints, connection management, query safety, N+1 patterns, and pagination — with focus on new SDLC tables.

## Methodology

Read `internal/store/store.go` in full. Read `ticket.go`, `sdlc.go`, `role.go`, `auth.go`, `goal.go`, `lifecycle.go`. Searched for string concatenation in SQL, pagination, ON DELETE clauses, N+1 patterns.

## Findings

### Passing checks
- `SetMaxOpenConns(1)` + `SetMaxIdleConns(1)` with WAL — correct for SQLite (`store.go:26-27`)
- `PRAGMA journal_mode = WAL` + `busy_timeout = 5000` + `foreign_keys = ON`
- All DML uses `?` placeholders
- Ticket list pagination supported — `store.go:903-905`
- Recursive CTE for ancestor walk — `ticket.go:1049`
- Batch comment fetch — `ticket.go:1200`
- SDLC tables have proper FKs and unique constraints
- `sdlc_stage_roles` PK is `(sdlc_id, stage_id, role_id)`
- Zombie `agents` table migrated and dropped

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| N+1 in `recalculateParentLifecycle`: `GetSdlcStageOrder` per child | Medium | `ticket.go:1293,1312`, `lifecycle.go:111` | Batch fetch stage orders |
| N+1 in `listSdlcStages`: `ListSdlcStageRoles` per stage | Medium | `sdlc.go:281-283` | Single JOIN query |
| `roles` table missing index on `sdlc_id` | Low | `store.go:175-185` | Add `idx_roles_sdlc_id` |
| No `ON DELETE` on any FK (50+) — app must manually clean up | Low | `store.go` schema | Add cascades/restricts |
| `tickets.assignee` not cleared on user delete — orphaned data | Low | `auth.go:247-310` | Add UPDATE before DELETE |
| `messages`/`goals` tables created in migration not main schema | Low | `store.go:888-925` | Move to main schema block |
| Hardcoded `LIMIT 1000` in list queries with no pagination | Low | `project.go:126`, `auth.go:211`, `team.go:110` | Add cursor/page pagination |
| Redundant `sdlc_id` column in `sdlc_stage_roles` | Info | `store.go:423` | Remove; `stage_id` implies sdlc |

## Verdict

Score improves +12 from 60 to 72. The SDLC refactor added well-structured tables with proper FK/indexes. Recursive CTE and batch comment fetch are genuine improvements. Remaining: two N+1 patterns, missing FK cascades, orphaned assignee on delete.

## Changes since last assessment
- Zombie `agents` table resolved — migrated and dropped
- Recursive CTE for ancestor walk (+)
- Batch comment hydration added (+)
- SDLC tables well-indexed and constrained (+)
- Two new N+1 patterns from SDLC code (-)
- `assignee` on user delete still not cleared

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Batch `GetSdlcStageOrder` calls | High | Single `IN (...)` query |
| Replace per-stage `ListSdlcStageRoles` loop | High | Single JOIN query |
| Add `idx_roles_sdlc_id` index | Medium | `store.go` schema |
| Add `ON DELETE CASCADE` to SDLC FKs | Medium | `store.go:418-433` |
| Clear `assignee` on user delete | Medium | `auth.go:243` |
| Add pagination to list functions | Low | Replace `LIMIT 1000` |
