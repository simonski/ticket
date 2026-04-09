# Database

**Score: 60/100** (was 68)

## What is being assessed
Schema evolution strategy (migrations), index coverage on all foreign keys, cascade delete correctness, query parameterisation, N+1 detection, pagination support, and audit trail integrity (HMAC).

## Methodology
Read `internal/store/store.go` schema section. Grepped for `CREATE INDEX`, `FOREIGN KEY`, `ALTER TABLE`, `history_events`. Searched for INSERT into `messages` and `goals` tables.

## Findings

### Passing checks
- All queries use `?` parameterised placeholders; no SQL string concatenation (`store.go`, `ticket.go`, `auth.go` throughout)
- `fmt.Sprintf` uses marked `#nosec G201` with justified explanations (`store.go:108, 139, 1386`)
- `history_events` table is active: populated in `auth.go:262-266`, `project.go`, read in `store_test.go` (not a zombie)
- `idx_history_events_project_id`, `idx_history_events_ticket_id` present (`store.go:427-428`)
- Migration strategy: disable FK checks, recreate table, re-enable — safe for SQLite (`store.go:472-475`)
- App-level cascade deletes in `DeleteUser()` handle transactional cleanup (`auth.go:243-310`)
- Session expiry enforced at query time: `WHERE expires_at > datetime('now')` (`auth.go:169`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `messages` table: FKs to `users(user_id)` but no indexes; no INSERT statements in codebase | High | `internal/store/store.go:867` | Either add indexes + usage, or remove zombie table in next migration |
| `goals` table: FK to projects but no index, no INSERT in codebase | High | `internal/store/store.go:887` | Remove zombie table or implement with index |
| No `ON DELETE CASCADE` on any FK — relies on app-level logic | Medium | `store.go` all FK definitions | Add `ON DELETE CASCADE` to child FK constraints (tickets→projects, comments→tickets, etc.) |
| `tickets.assignee` is TEXT (username), not FK to `users` — not nulled on user delete | Medium | `store.go:223-224`, `auth.go:243-310` | Add `UPDATE tickets SET assignee = '' WHERE assignee = ?` to `DeleteUser()` |
| No HMAC on `history_events.payload` — audit records can be tampered silently | Medium | `internal/store/store.go:264,277` | Compute HMAC-SHA256 of payload on insert; verify on read |
| `workflow_stages.role_id` FK has no index | Low | `store.go:367` | `CREATE INDEX idx_workflow_stages_role_id ON workflow_stages(role_id)` |

## Verdict
Score drops from 68 to 60. The `messages` and `goals` tables are confirmed zombie — defined in schema, never written to in production code — creating schema bloat and confusion. The `assignee` orphaning-on-delete is a real data integrity gap. DB cascade deletes are entirely absent; the app-level workarounds work but are fragile.

## Changes since last assessment
- Schema unchanged this cycle
- `messages`/`goals` zombie status confirmed (no new INSERT statements added)
- `assignee` cleanup on user delete confirmed absent
- Zombie `history_events` finding from v0.1.730 corrected: it IS active

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Remove `messages`/`goals` zombie tables | High | Drop tables in next migration or implement them properly |
| Fix `assignee` orphaning | High | Add `UPDATE tickets SET assignee = '' WHERE assignee = ?` inside `DeleteUser()` transaction |
| Add HMAC to audit payloads | Medium | Sign `history_events.payload` with HMAC-SHA256 keyed on a server secret |
| Add `ON DELETE CASCADE` | Medium | At minimum: `comments→tickets`, `ticket_labels→tickets`, `time_entries→tickets` |
| Add missing FK indexes | Low | `workflow_stages.role_id`, `tickets.clone_of`, `messages.from/to_user_id` |
