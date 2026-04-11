# Product Owner

**Score: 78/100** (was 85)

## What is being assessed

Feature completeness of the SDLC lifecycle refactor against the specification in `docs/LIFECYCLE.md`. User journey from project creation through lifecycle management.

## Methodology

Read `docs/LIFECYCLE.md` (authoritative spec) and `CLAUDE.md`. Verified CLI commands in `cmd/tk/main.go`, `cmd_sdlc.go`, `cmd_project.go`. Verified API endpoints in `api_sdlc.go`, `api_roles.go`. Verified service interface in `service.go`. Searched for stubs.

## Findings

### Passing checks
- SDLC CRUD (create, list, get, delete, export, import) — `cmd_sdlc.go:45-213`, `api_sdlc.go:137-267`
- Stage management (add, remove, reorder) — `cmd_sdlc.go:99-163`
- Stage-role assignments (add, rm, order) — `cmd_sdlc.go:235-293`, `api_sdlc.go:67-135`
- Role CRUD — `cmd_team.go:283-418` under `tk role` namespace
- Project attach-SDLC as `tk project sdlc <id>` — `cmd_project.go:335-378`
- Ticket next/previous — `cmd_ticket.go:2756/2794`, backed by `findNextStep`/`findPrevStep`
- Ticket complete/reopen, draft/undraft, idle/active/success shortcuts
- State transitions enforced in store — `lifecycle.go`
- Export/import round-trip tested — `sdlc_test.go:145`
- Contract tests run against both implementations

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `tk fail` shortcut missing from `main.go` switch — spec requires it | High | `main.go:185-195` | Add `case "fail"` |
| LIFECYCLE.md defines `stage-list/get/update/rm/order` but CLI uses different names (`add-stage/remove-stage/reorder-stages`); `stage-list/get/update` don't exist | High | `cmd_sdlc.go:99-163` | Rename or alias to match spec |
| LIFECYCLE.md defines SDLC-scoped role commands but roles are global under `tk role` | High | `cmd_sdlc.go` (absent) | Expose `ListRolesBySdlc` via CLI |
| `tk project set-draft` is a stub — prints "not yet implemented" | High | `cmd_project.go:194` | Implement |
| `tk project set-sdlc` syntax differs from spec (positional vs flag) | Medium | `cmd_project.go:196` vs `LIFECYCLE.md:25` | Add flag-style alias |
| `tk sdlc stage-update` absent from CLI, service, and API | Medium | `service.go:80-93` | Implement across all layers |
| `tk sdlc stage-get` absent — no per-stage detail command | Medium | `cmd_sdlc.go` | Add using existing store function |
| API for stage update (`PUT /api/sdlcs/stages/{id}`) missing | Medium | `api_sdlc.go` | Add endpoint |
| `tk status` doesn't display SDLC name or draft default | Low | `main.go:296-427` | Add to summary output |
| `tk sdlc get` doesn't surface stage acceptance criteria | Low | `cmd_sdlc.go:327-342` | Add to detail output |

## Verdict

The SDLC refactor delivered the core data model and lifecycle mechanics, but CLI surface area falls short of the LIFECYCLE.md specification. Missing `tk fail`, SDLC-scoped roles, and stub `project set-draft` are the key gaps. Score drops from 85 to 78 against the new spec bar.

## Changes since last assessment
- SDLC tables, stage-role junction, next/previous orchestration, export/import all delivered (+)
- Assessment now applies LIFECYCLE.md as precise checklist (-)
- Several spec-required commands missing or stubbed (-)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add `tk fail` shortcut | Critical | Add `case "fail"` to `main.go` |
| Implement `project set-draft` | High | Store + service + API + CLI |
| Add `stage-update` across all layers | High | Store + service + API + CLI |
| Add `stage-list` and `stage-get` | High | CLI commands using existing store |
| Add SDLC-scoped role commands | High | Use `store.ListRolesBySdlc` |
| Rename CLI commands to match spec | Medium | `add-stage` -> `stage-add` etc. |
| Add stage update API endpoint | Medium | `PUT /api/sdlcs/stages/{id}` |
