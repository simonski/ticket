# UX Review

**Score: 76/100** (was 79)

## Mission
Protect interaction quality so common tasks stay legible, efficient, and recoverable across both the happy path and error states.

## Review objective
Assess the coherence of the current web and CLI interactions, with emphasis on discoverability, feedback, and workflow friction.

## Inputs reviewed
- `web/static/index.html`
- `cmd/tk/printer.go`
- `docs/ONBOARDING.md`

## Findings

### Passing checks
- The web app has proper modal focus trapping and Escape/Tab handling, which reduces a large class of interaction dead-ends (`web/static/index.html:6129-6164`).
- The main modals and ticket detail surfaces expose meaningful structure and feedback instead of hidden side effects only (`web/static/index.html:1601-1634`, `web/static/index.html:1733-1752`, `cmd/tk/printer.go:256-290`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The project picker and profile menu are custom `div` menus driven by click handlers. | Medium | Navigation and selection are less discoverable and less robust than native/button-based controls. | `web/static/index.html:1271-1284`, `web/static/index.html:3394-3474` | Replace these with semantic buttons/menuitems or a small shared menu component. |
| Project membership management asks for raw numeric user/team IDs. | Medium | A high-value admin workflow is awkward and error-prone even for experienced users. | `web/static/index.html:1491-1513` | Replace numeric ID entry with searchable user/team selectors. |
| The ticket editor still says “No save button. Changes are persisted as you type.” | Low | Users can be unsure when data is committed and when a half-edit becomes live. | `web/static/index.html:1897-1905` | Add clearer inline save state or move to explicit save for the highest-risk fields. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| accessibility | Several UX weaknesses are also semantic/input weaknesses. | Menu semantics and status messaging audit. |
| frontend-engineer | The awkward interactions live in the SPA. | Searchable member picker + shared menu component. |
| product-manager | Health/autosave semantics need a product decision, not just a UI patch. | Should editing be explicit or ambient? |

## Verdict
The app has solid modal and board foundations, but some admin workflows still feel like internal tools rather than polished product surfaces. The next UX win is reducing avoidable precision work: clicks on custom menus and numeric-ID entry should not gate key tasks.

## Changes since last assessment
- Keyboard/focus handling remains a strength, but the surrounding admin workflows still carry friction-heavy controls instead of product-grade selectors (`web/static/index.html:1491-1513`, `web/static/index.html:6129-6164`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Click-only custom menus | Medium | Replace with shared semantic menu components. | frontend-engineer |
| Numeric-ID membership flow | Medium | Introduce searchable selectors for users and teams. | frontend-engineer |
| Ambiguous autosave | Low | Add explicit save-state feedback or switch to explicit save. | product-manager |
