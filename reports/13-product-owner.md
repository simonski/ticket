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
- **`tk ready` / `tk notready` now match their names** — the root routing now passes the correct draft booleans and the lifecycle helper/help text matches the draft model
- **SDLC-scoped role CRUD is now available under `tk sdlc`** — `role-add`, `role-get`, `role-update`, and `role-rm` work under the SDLC namespace and are regression-tested
- **`tk sdlc get` now includes stage acceptance criteria** — detail output renders AC alongside stage metadata
- **`tk status` now shows project SDLC and default draft context** — both text and JSON output expose the active project’s workflow and default-draft setting
- **`tk ticket tree` no longer pretends to be a distinct feature** — the placeholder alias is removed and replaced with an explicit error directing users to supported views
- **The SPA’s modal accessibility is materially stronger** — modal dialogs now expose labels and ARIA metadata, trap focus, support Escape/Tab keyboard handling, and keep the ticket parent link keyboard-activatable

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

The remaining workflow gaps from the prior review are now closed. The highest-impact semantic bug (`tk ready` / `tk notready`) is fixed, the SDLC surface is more complete, status output reflects the active workflow, and the web UI’s modal interactions are materially more usable from the keyboard.

## Changes since last assessment
- Fixed the inverted ready/notready lifecycle routing and added CLI regression coverage
- Completed the `tk sdlc role-*` command family and stage acceptance-criteria output
- Added project SDLC/default-draft context to `tk status`
- Removed the placeholder `tk ticket tree` alias
- Improved modal labels, focus management, Escape handling, and keyboard affordances in the SPA

## Remaining recommendations

None. Re-audited on **2026-04-12** under **TK-131** after commit **`619ed5a`**.
