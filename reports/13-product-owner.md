# Product Owner

**Score: 82/100** (was 84)

## What is being assessed

Feature completeness and workflow quality for end users across CLI, API, and web UI, with emphasis on whether the implemented commands match their intended semantics and whether common ticket-management journeys feel correct.

## Methodology

Reviewed CLI routing and ticket workflows in `cmd/tk`, compared the current behavior to the previous product-owner report, and inspected the recent `tk ls` / `tk get` improvements and the still-open workflow correctness gaps.

## Findings

### Passing checks
- **`tk ls` now behaves more like “all open tickets”** — it uses a dedicated open-ticket predicate instead of relying only on `complete` state (`cmd/tk/cmd_ticket.go:39-44`, `cmd/tk/cmd_ticket.go:397-402`)
- **Open child tickets remain visible under open epics in list output** — covered by CLI regression tests in `cmd/tk/main_test.go:1441-1459`
- **`tk get` now exposes child totals/open/closed counts** — the command reports child counts directly before the child list (`cmd/tk/cmd_ticket.go:644-658`)
- **`tk get` keeps attached children visible and visually prioritised by state** — child rendering is explicit and state-aware (`cmd/tk/printer.go:262-307`)
- **The recent child-count and child-visibility behavior is regression-tested** — `cmd/tk/main_test.go:1855-1880`, `cmd/tk/main_test.go:4193-4203`
- **The broader SDLC and ticket-management feature set remains in place** — stage CRUD, stage-role assignment, `tk fail`, `project set-draft`, and the command help surface still exist and are wired end-to-end

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `tk ready` / `tk notready` semantics are still inverted | High | `cmd/tk/main.go:253-256` | Swap the boolean arguments so `ready` clears draft and `notready` sets it |
| SDLC-scoped role CRUD is still incomplete under `tk sdlc` | Medium | `cmd/tk/cmd_sdlc.go` | Add `role-add`, `role-get`, `role-update`, `role-rm` under the SDLC namespace |
| Web UI accessibility remains too thin for the feature surface | Medium | `web/static/index.html` | Add semantic labels, focus trapping, and stronger keyboard affordances |
| `tk sdlc get` still omits stage acceptance criteria | Low | `cmd/tk/cmd_sdlc.go` | Include acceptance criteria in detail output |
| `tk status` still omits SDLC and draft-default context | Low | status command path | Include current SDLC name and project draft default |
| `tk ticket tree` remains an alias for `get` | Low | `cmd/tk/cmd_ticket.go:77-78` | Implement a dedicated tree view or remove the alias |

## Verdict

The recent CLI work improved day-to-day usability for ticket inspection and listing, but the score still drops because the highest-impact correctness bug remains unresolved: `tk ready` still does the opposite of what users expect. This category is now less about missing features and more about semantic correctness and finishing the last SDLC workflow gaps.

## Changes since last assessment
- `tk ls` now filters on genuinely open tickets instead of only `complete` state
- `tk get` now reports child counts and keeps attached children visible with state-based emphasis
- The highest-impact workflow bug (`tk ready` / `tk notready`) still remains unresolved

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Inverted `ready` / `notready` semantics | High | Swap the booleans in `main.go` so the commands match their names |
| Incomplete SDLC-scoped role CRUD | Medium | Finish the `tk sdlc role-*` command family |
| Web accessibility debt | Medium | Add labels, ARIA, and focus trapping in the SPA |
| Missing AC in `tk sdlc get` | Low | Render stage acceptance criteria |
| `tk status` missing SDLC context | Low | Show SDLC name and draft-default setting |
| `tk ticket tree` placeholder | Low | Build it or remove the alias |
