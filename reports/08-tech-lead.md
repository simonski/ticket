# Tech Lead / Code Quality

**Score: 55/100**

## What is being assessed
File size discipline (no file >700 lines), code duplication, error message consistency, magic numbers, cyclomatic complexity, dead code, naming conventions, interface sizes, helper reuse, and overall refactoring debt.

## Methodology
Measured all Go file sizes. Analysed `internal/server/api.go`, `cmd/ticket/cmd_ticket.go`, `internal/tui/model.go`, `cmd/ticket/main.go`. Checked for repeated patterns, magic numbers, named constants, and helper extraction in `internal/server/api_helpers.go`.

## Findings

### Passing checks
- Consistent CRUD naming: `Create*`, `Get*`, `List*`, `Update*`, `Delete*` across all store functions
- Sentinel errors defined and centralised: `ErrUnauthorized`, `ErrForbidden`, `ErrAdminRequired` (`store/auth.go:15-19`)
- `writeJSON()`, `writeError()`, `writeAuthError()` helpers extracted and used consistently
- `requireUser()`, `requireAdmin()`, `canReadProject()`, `canWriteProject()` permission helpers in `api_helpers.go`
- CLI commands well-organised by namespace: `ticket`, `user`, `project`, `team`, `role`, `story`, `workflow`, `label`
- 12 `cmd_*.go` files split by domain — good initial decomposition
- Stage/state constants externalised: `store.StateActive`, `store.StageDesign`, `store.ProjectRoleOwner` etc.
- No deprecated stdlib functions used

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `internal/server/api.go` is 3009 lines — monolithic handler | High | `internal/server/api.go` | Split into `api_tickets.go`, `api_users.go`, `api_projects.go` etc. (6-8 files ~400 lines each) |
| `internal/tui/model.go` is 2817 lines managing 12 view modes | High | `internal/tui/model.go` | Extract each view mode to its own file: `view_list.go`, `view_detail.go`, `view_edit.go` etc. |
| `cmd/ticket/cmd_ticket.go` is 2584 lines — all ticket ops in one file | High | `cmd/ticket/cmd_ticket.go` | Split by operation group (lifecycle, assignment, search, output) |
| `libticket/service.go` defines 104-method God interface | High | `libticket/service.go` | Split into domain sub-interfaces: `TicketService`, `UserService`, `ProjectService` etc. |
| 55+ repetitions of `len(parts) == N && parts[1] == "resource"` path parsing | High | `internal/server/api.go` | Extract `parseResourcePath(r)` helper returning `(resource, id, action)` |
| `cmd/ticket/main.go` has 77-case switch statement | Medium | `cmd/ticket/main.go:94-266` | Register commands in a `[]CommandHandler` slice with `Name`, `Aliases`, `Handler` |
| `cmd/ticket/main_test.go` is 4718 lines | Medium | `cmd/ticket/main_test.go` | Split by feature area into multiple test files |
| Magic numbers in `internal/tui/model.go`: heights `4`, `3`, `w - 4` repeated | Medium | `internal/tui/model.go:80-230` | Define `const modelPadding = 4`, `const formHeight = 3` |
| Hardcoded error strings `"invalid json body"`, `"invalid ticket id"` scattered | Medium | `internal/server/api.go` | Externalise to `constants.go` |
| `internal/client/client.go` has ~80 `if c.mode == local` branches | Medium | `internal/client/client.go` | Strategy pattern: separate `LocalClient` and `RemoteClient` structs |
| `internal/store/store.go` is 1615 lines including all migration logic | Medium | `internal/store/store.go` | Extract migrations to `internal/store/migrations.go` |
| No `pathParam()` helper despite 55+ path-parsing repetitions | Medium | `internal/server/api.go` | Extract `pathParam(r, n)` returning nth path segment |

## Verdict
The codebase is functional and well-named, but carrying significant structural debt from organic growth. Seven files exceed 1000 lines (the threshold for serious concern), and the 104-method Service interface and 3009-line API handler are the most impactful refactoring targets. These don't affect correctness today but will slow feature development and increase bug risk as the codebase grows.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Split `api.go` into resource handlers | High | ~1-2 week effort; highest impact on maintainability |
| Split `Service` interface into sub-interfaces | High | ~1 week; enables proper mocking and ISP compliance |
| Extract `parseResourcePath()` helper | High | ~1 day; eliminates 55+ repeated patterns |
| Split `model.go` view modes | Medium | ~1-2 weeks; improves TUI maintainability |
| Register commands via `[]CommandHandler` | Medium | ~2 days; replaces 77-case switch |
| Externalise error string constants | Low | ~half day; reduces magic strings |
