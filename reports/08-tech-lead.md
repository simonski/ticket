# Tech Lead

**Score: 56/100** (was 60)

## What is being assessed

Code quality: file sizes, duplication, error consistency, magic values, dead code, naming conventions, interface sizes, helper reuse, and impact of the SDLC lifecycle refactor.

## Methodology

`wc -l` on all Go and web files. `git diff main...HEAD` for size deltas. Grep and spot-reads across `cmd/tk/`, `internal/store/`, `internal/client/`, `libticket/`, `libtickethttp/`, `internal/tui/`. Diffed `service.go` between main and refactor branch.

## Findings

### Passing checks
- `doJSON` HTTP helper centralised — `client_util.go:87,130`
- `writeJSON`/`writeError` server helpers consistent — `api.go`
- Store sentinel errors defined and used via `errors.Is()` — `ticket.go:13-16`
- `lifecycle.go` constants file well-structured — `internal/store/lifecycle.go`
- Workflow->Sdlc rename complete — no residual references
- New SDLC files appropriately sized: `cmd_sdlc.go` (342), `api_sdlc.go` (268), `store/sdlc.go` (285)

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| TUI `ticketStates` has `"open"` — not a valid store state (should be `idle`) | Critical | `tui/model.go:281` | Replace with `"idle"` |
| TUI `ticketStages` has `"planning"`, `"development"`, `"review"` — none match store constants | High | `tui/model.go:282` | Use `design/develop/test/done` |
| `cmd_ticket.go` grew to 2,826 lines (+138 from refactor) | High | `cmd/tk/cmd_ticket.go` | Split into sub-files |
| `tui/model.go` at 3,151 lines — largest file in repo | High | `internal/tui/model.go` | Extract form, picker, board, detail views |
| `store/ticket.go` grew to 1,988 lines (+228 from refactor) | High | `internal/store/ticket.go` | Split CRUD from lifecycle |
| `client.go` has 99 `c.openLocalDB()` repetitions | High | `internal/client/client.go` (1,976 lines) | Extract `withLocalDB(func)` helper |
| Service interface at 119 methods with naming inconsistency | High | `libticket/service.go:107-131` | Normalise VerbNoun pattern |
| `index.html` at 6,080 lines | Medium | `web/static/index.html` | Split CSS/JS into separate files |
| Magic number `* 10` in sort bucket | Medium | `cmd_ticket.go:36` | Add `const stageBucketSize = 10` |
| 18 repeated `writeError(w, 404, "ticket not found")` without shared helper | Medium | `api_tickets.go` | Extract `handleTicketNotFound` |
| `ReadyTicket`/`NotReadyTicket`/`DraftTicket`/`UndraftTicket` — 4 methods for 2 booleans | Medium | `service.go:117-120` | Collapse to `SetTicketDraft(bool)` |
| Hardcoded stage strings not using constants | Low | `store.go:1055-1056`, `main.go:184,188,194` | Use `StageDevelop`/`StageDone` constants |

## Verdict

The refactor's new files are appropriately sized, but it worsened the three pre-existing mega-file problems without splitting them. The TUI `ticketStates` bug (`"open"`) is the most critical new regression. Score drops 4 points.

## Changes since last assessment
- `lifecycle.go` constants file added (+)
- Workflow->Sdlc rename complete (+)
- New SDLC files well-scoped (+)
- `cmd_ticket.go` +138 lines, `store/ticket.go` +228 lines, `client.go` +82 lines (-)
- TUI `ticketStates`/`ticketStages` bug introduced (-)
- Service interface naming inconsistency grew (-)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix TUI `ticketStates`/`ticketStages` | Critical | `tui/model.go:281-282` |
| Split `tui/model.go` (3,151 lines) | High | Extract into sub-files |
| Split `cmd_ticket.go` (2,826 lines) | High | Extract state/lifecycle commands |
| Refactor `client.go` local/remote branching | Medium | `withLocalDB(func)` helper |
| Normalise Service interface naming | Medium | Consistent VerbNoun pattern |
| Use lifecycle constants everywhere | Low | `store.go:1055`, `main.go:184` |
