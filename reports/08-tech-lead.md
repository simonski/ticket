# Tech Lead / Code Quality

**Score: 63/100** (was 55)

## What is being assessed
File size discipline (no file >700 lines), code duplication, error message consistency, magic numbers, cyclomatic complexity, dead code, naming conventions, interface sizes, helper reuse, CI pipeline quality, and overall refactoring debt.

## Methodology
Measured all Go file sizes (`wc -l **/*.go`), counted interface method counts, grepped for repeated path-parsing patterns, reviewed `internal/server/api_helpers.go`, `.github/workflows/makefile.yaml`, and inspected the largest switch statements. Version 0.1.737.

## Findings

### Passing checks
- Consistent CRUD naming: `Create*`, `Get*`, `List*`, `Update*`, `Delete*` across all store functions
- Sentinel errors centralised by domain: `ErrUnauthorized`, `ErrForbidden`, `ErrAdminRequired` (`store/auth.go:17-20`); `ErrTicketNotFound`, `ErrTicketHasChildren` (`store/ticket.go:13-16`); `ErrProjectNotFound`, `ErrTeamNotFound`, etc.
- `writeJSON()`, `writeError()`, `writeAuthError()` helpers extracted and used consistently (`internal/server/api_helpers.go:201,207,211`)
- `requireUser()`, `requireAdmin()`, `canReadProject()`, `canWriteProject()`, `canManageProjectUsers()` permission helpers in `api_helpers.go`
- CLI commands well-organised by namespace across 12 `cmd_*.go` files
- Stage/state constants externalised: `store.StateActive`, `store.StageDesign`, `store.ProjectRoleOwner` etc.
- No deprecated stdlib functions used
- **`internal/server/api.go` reduced from 3009 → 55 lines** — fully split into domain handlers
- **`libticket/service.go` `Service` interface now composes sub-interfaces**: `AuthService`, `UserService`, `AgentService`, `ProjectService`, `TeamService`, `WorkflowService`, `TicketService`
- **Publish CI job added** to `.github/workflows/makefile.yaml`: build → test → gosec → govulncheck → Docker push → GitHub release → Homebrew tap
- `writeAuthError()` centralises the `errors.Is` → HTTP status mapping (`api_helpers.go:211-222`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `cmd/ticket/main_test.go` — 4742 lines, single file | High | `cmd/ticket/main_test.go` | Split by feature area: `cmd_ticket_test.go`, `cmd_user_test.go`, `cmd_project_test.go` etc. |
| `internal/tui/model.go` — 2817 lines managing 12 view modes in one file | High | `internal/tui/model.go` | Extract each view mode to its own file: `view_list.go`, `view_detail.go`, `view_edit.go` etc. |
| `internal/server/api_test.go` — 2664 lines, single test file | High | `internal/server/api_test.go` | Split to match the new `api_*.go` file layout |
| `cmd/ticket/cmd_ticket.go` — 2610 lines, all ticket operations in one file | High | `cmd/ticket/cmd_ticket.go` | Split by operation group: lifecycle, assignment, search, output rendering |
| `internal/client/client.go` — 1894 lines with ~80 `if c.mode == local` branches | High | `internal/client/client.go` | Strategy pattern: separate `LocalClient` and `RemoteClient` structs sharing an interface |
| `internal/store/store.go` — 1680 lines (schema + all migration logic) | Medium | `internal/store/store.go` | Extract migrations to `internal/store/migrations.go` |
| `internal/store/ticket.go` — 1679 lines | Medium | `internal/store/ticket.go` | Split read (`ticket_query.go`) from write (`ticket_mutate.go`) from lifecycle (`ticket_lifecycle.go`) |
| `libtickettest/contract.go` — 1505 lines, single contract test file | Medium | `libtickettest/contract.go` | Split into `contract_tickets_test.go`, `contract_projects_test.go` etc. |
| `internal/server/api_tickets.go` — 1036 lines, the newly split file already exceeds threshold | Medium | `internal/server/api_tickets.go` | Further split ticket CRUD, lifecycle, and dependency handlers |
| 11× repeated `strings.TrimPrefix(r.URL.Path, "/api/<resource>/")` path parsing | Medium | `api_tickets.go:338`, `api_projects.go:73`, `api_teams.go:68`, `api_users.go:61`, `api_roles.go:57`, `api_workflows.go:46,106`, `api_agents.go:59` | Use named route variables: `mux.HandleFunc("/api/tickets/{id}", ...)` + `r.PathValue("id")` — `go.mod` already requires ≥1.22 |
| `cmd/ticket/main.go` has a 40-case top-level dispatch switch | Medium | `cmd/ticket/main.go:94` | Register commands in a `[]Command{Name, Aliases, Handler}` slice to eliminate the switch |
| `TicketService` sub-interface has 43 methods — still exceeds comfortable interface size | Low | `libticket/service.go:93` | Further decompose: `TicketReadService`, `TicketWriteService`, `TicketLifecycleService` |
| Magic numbers in `internal/tui/model.go`: `4`, `3`, `w - 4` repeated across layout code | Low | `internal/tui/model.go:80-230` | Define `const modelPadding = 4`, `const formHeight = 3` |

## Verdict
A meaningful structural improvement since the last assessment. The most impactful item — the 3009-line monolithic `api.go` — has been fully decomposed into domain handler files, and the `Service` God interface is now properly composed from sub-interfaces. These two changes alone account for the score increase. However, eight files still exceed 1000 lines (the threshold for meaningful structural concern), and the path-parsing pattern hasn't been modernised despite Go 1.22 route variables being available. The `internal/client/client.go` branching pattern (`~80 mode checks`) is now the single highest-leverage refactoring target remaining.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `internal/server/api.go` split: 3009 lines → 55 lines (bootstrap only); domain logic moved to `api_tickets.go` (1036), `api_projects.go` (571), `api_users.go` (124), `api_agents.go` (472), `api_workflows.go`, `api_teams.go` | +6 — critical structural debt resolved |
| `libticket/service.go` `Service` interface now composes 7 named sub-interfaces | +3 — ISP now respected; mocking is tractable |
| Publish CI job added: build → test → gosec → govulncheck → Docker push → GitHub release → Homebrew tap update | +2 — full release automation, no manual steps |
| `#nosec` suppression discipline: all 75 annotations carry explanatory comments | +1 |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Split `cmd_ticket.go` (2610 lines) by operation group | High | ~1 week; lifecycle, assignment, search, rendering each become their own file |
| Strategy pattern for `client.go` (1894 lines, ~80 mode branches) | High | ~1 week; eliminates structural duplication between local and remote paths |
| Split `main_test.go` (4742 lines) by feature | High | ~2 days; mirrors the cmd_*.go decomposition |
| Split `api_test.go` (2664 lines) to mirror `api_*.go` files | High | ~2 days |
| Use `r.PathValue("id")` instead of 11× `strings.TrimPrefix` | Medium | ~1 day; cleaner routing, enforced by Go stdlib |
| Extract migrations from `store.go` (1680 lines) | Medium | ~1 day; `migrations.go` + `schema.go` |
| Further decompose `TicketService` (43 methods) | Low | ~half day; read/write/lifecycle sub-interfaces |
| Replace 40-case dispatch switch in `main.go` | Low | ~1 day; `[]Command` slice is more extensible |
