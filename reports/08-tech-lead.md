# Tech Lead

**Score: 56/100** (was 59)

## What is being assessed

File size, duplication, maintainability, dead code, complexity, error-message consistency, helper reuse, interface size, and whether the codebase is moving toward simpler operational ownership rather than larger monoliths.

## Methodology

Reviewed the large CLI, client, TUI, server, and store files; checked linter/static-analysis findings mentioned in the prior report; inspected `cmd/tk/main.go`, `cmd/tk/cmd_ticket.go`, `cmd/tk/printer.go`, `internal/tui/model_forms.go`, `internal/client/client.go`, and other oversized files; compared current hotspots against the previous assessment.

## Findings

### Passing checks
- **The earlier file-splitting work still holds** — `cmd/tk/cmd_ticket.go` remains reduced from the old 2.8k-line shape, even though it is still large (`cmd/tk/cmd_ticket.go`)
- **`internal/tui/model.go` remains split** — major extraction work from the previous pass is still in place (`internal/tui/model.go`, `internal/tui/model_forms.go`)
- **The CLI now has small, reusable helpers for recent behavior** — `ticketIsOpenForList` and `childTicketCounts` centralize list/detail rules instead of repeating ad hoc logic (`cmd/tk/cmd_ticket.go:39-55`)
- **Child rendering behavior is now isolated to a dedicated helper path** — `printTicketChildren` and `childTicketColor` keep the state-emphasis logic out of `runGet` (`cmd/tk/printer.go:262-307`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| TUI `ticketStates` still contains invalid `"open"` value | Critical | `internal/tui/model_forms.go:217` | Replace with store constants and use `idle` instead of `open` |
| TUI `ticketStages` still uses names that do not match store lifecycle constants | Critical | `internal/tui/model_forms.go:218` | Replace with `StageDesign`, `StageDevelop`, `StageTest`, `StageDone` from the store package |
| SA4010-style append bugs remain unresolved | High | `cmd/tk/cmd_requirement.go:32`, `internal/store/store.go:1168` | Fix discarded append results immediately to avoid silent data loss |
| `internal/client/client.go` still repeats the local DB open/close pattern at scale | High | `internal/client/client.go` | Extract a `withLocalDB` helper and collapse the repetition |
| `web/static/index.html` is still a monolith | High | `web/static/index.html` | Split CSS/JS from the HTML shell to reduce coupling and review burden |
| Multiple files still exceed the 700-line threshold by a wide margin | High | `cmd/tk/cmd_ticket.go`, `internal/client/client.go`, `internal/store/ticket.go`, `internal/tui/model.go`, others | Continue decomposition in the biggest hotspots rather than adding more behavior in place |
| `main.go` still relies on a large command switch | Medium | `cmd/tk/main.go` | Move toward a registry/dispatch table to reduce branching complexity |
| Magic numbers and raw ANSI escapes remain scattered | Low | `cmd/tk/*`, `cmd/tk/status.go`, `cmd/tk/banner.go` | Consolidate common values/constants |

## Verdict

The codebase still shows the benefits of earlier structural cleanups, but the unresolved TUI lifecycle bugs and static-analysis issues now outweigh those wins in this category. The repo is not getting harder to work in everywhere, but the biggest maintainability risks are still concentrated in the same files and the same correctness hotspots.

## Changes since last assessment
- The recent `cmd/tk` CLI work added focused helper extraction instead of more inline branching (`cmd/tk/cmd_ticket.go:39-55`, `cmd/tk/printer.go:262-307`)
- The previously flagged critical TUI lifecycle constant bugs remain unresolved
- The previous report’s static-analysis concerns are still active, with the append bugs still unfixed

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Invalid TUI lifecycle constants | Critical | Replace ad hoc stage/state strings with store constants |
| Unused append results | High | Fix the SA4010-style append bugs immediately |
| Repeated local DB boilerplate | High | Extract a helper in `internal/client/client.go` |
| Large monolithic files | High | Continue splitting `client.go`, `ticket.go`, `index.html`, and other >700-line files |
| Command switch complexity | Medium | Introduce a command registry/dispatch table |
| Scattered constants/ANSI codes | Low | Centralize reusable values in constants/helpers |
