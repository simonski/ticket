# Tech Lead

**Score: 60/100** (was 63)

## What is being assessed
File sizes (target: no file >700 lines), code duplication, cyclomatic complexity, magic numbers, dead code, naming consistency, and helper reuse. Good means: files are focused and reviewable, functions have single responsibilities, errors are consistent.

## Methodology
Ran `wc -l` on all `.go` files. Grepped for TODO/FIXME, magic numbers, duplicated patterns. Inspected `cmd_ticket.go`, `client.go`, `cmd_setup.go` and `printer.go` for complexity.

## Findings

### Passing checks
- Only 1 TODO marker: `cmd/ticket/cmd_ticket.go:70` ("dedicated tree view") — clean codebase
- Error wrapping with `%w` used consistently; 68 `fmt.Errorf` instances
- `promptYN` and `prompt` helpers reused throughout `cmd_setup.go:47-73` (no inline re-implementation)
- No obvious copy-paste duplication detected
- Magic numbers minimal: mostly named constants or HTTP status codes
- Dead code: none found
- Error messages consistent: `"usage: "` prefix, lowercase descriptions

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `cmd/ticket/cmd_ticket.go`: 2,626 lines, 32 `run*` handler functions | Critical | `cmd/ticket/cmd_ticket.go` | Split into `cmd_ticket_list.go`, `cmd_ticket_get.go`, `cmd_ticket_update.go`, `cmd_ticket_state.go` |
| `internal/client/client.go`: 1,894 lines, 102 methods on one struct | Critical | `internal/client/client.go` | Split into `client_tickets.go`, `client_projects.go`, `client_auth.go`, `client_teams.go` |
| `internal/tui/model.go`: 2,817 lines — monolithic TUI model | Critical | `internal/tui/model.go` | Extract view components into separate files |
| `cmd/ticket/cmd_setup.go`: now 1,144 lines (grew +128 lines this cycle) | High | `cmd/ticket/cmd_setup.go` | Extract to `cmd_setup_init.go`, `cmd_setup_seed.go`, `cmd_setup_validate.go` |
| `cmd/ticket/main_test.go`: 4,744 lines — test file larger than production code | High | `cmd/ticket/main_test.go` | Split by command domain |
| `internal/store/ticket.go`: 1,688 lines; `internal/store/store.go`: 1,680 lines | High | `internal/store/` | Extract schema DDL to `schema.go`; split ticket CRUD by operation type |
| `cmd/ticket/printer.go`: 993 lines | Medium | `cmd/ticket/printer.go` | Extract colour helpers and table formatters into sub-files |
| 13 files total exceed 700 lines | High | Multiple files | Address in priority order above |

## Verdict
Score regresses from 63 to 60 because `cmd_setup.go` grew 128 lines this cycle with no corresponding refactor, and the 13 oversized files remain unaddressed. The codebase is functionally sound but architecturally strained — `cmd_ticket.go` at 2,626 lines with 32 handler functions is the highest-priority refactoring target.

## Changes since last assessment
- `cmd_setup.go` grew from ~1,016 to 1,144 lines (+128) due to `runInitCheckDefaults` and `addDefaultWorkflowStages`
- No file splits or extractions performed this cycle
- `runBoard` changes added ~46 lines to `cmd_ticket.go`
- All 13 oversized files unchanged in structure

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Split `cmd_ticket.go` | Critical | Minimum: extract `runList`, `runBoard`, `runGet` into dedicated files |
| Split `client.go` | Critical | Group methods by domain: auth, tickets, projects, teams |
| Split `cmd_setup.go` | High | Extract `runInitCheckDefaults` + seeds to `cmd_setup_validate.go` |
| Split `tui/model.go` | High | Extract view rendering functions into `tui/views/` sub-package |
| Add `gocyclo` lint rule | Medium | Enforce max cyclomatic complexity of 15 per function in golangci-lint config |
