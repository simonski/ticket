# Accessibility

**Score: 72/100** (was 79)

## Mission
Protect usability for keyboard users, screen-reader users, and anyone who depends on semantic feedback rather than visual inference alone.

## Review objective
Verify that the current web UI preserves semantics, labels, focus order, and dynamic announcements for key tasks.

## Inputs reviewed
- `web/static/index.html`

## Findings

### Passing checks
- Login form labels are correctly associated with their inputs, and the main modal surfaces are marked as dialogs (`web/static/index.html:1292-1306`, `web/static/index.html:1601-1634`, `web/static/index.html:1733-1752`, `web/static/index.html:1909-1999`).
- The modal focus trap restores focus and handles Escape/Tab navigation, which is a strong baseline for assistive-technology support (`web/static/index.html:6129-6164`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The profile avatar and profile menu items are `div` elements with click handlers instead of semantic buttons/menuitems. | High | Screen-reader and keyboard users get weaker semantics and less predictable interaction behavior. | `web/static/index.html:1275-1284`, `web/static/index.html:3471-3508` | Replace interactive `div`s with buttons and labelled menu items. |
| The project picker menu is rendered as clickable `div`s rather than native options or semantic menu items. | High | Project switching is harder to understand and operate without pointer-first interaction. | `web/static/index.html:1271-1273`, `web/static/index.html:3397-3456` | Rebuild the picker as a semantic listbox/menu or use a native `<select>` where possible. |
| Agent, role, and team status messages only change text color and are not announced as alerts. | Medium | Dynamic success/failure feedback can be missed entirely by assistive technologies and colorblind users. | `web/static/index.html:3675-3677`, `web/static/index.html:3884-3886`, `web/static/index.html:4085-4087` | Add `role="status"` or `role="alert"` plus visible text prefixes for error/success states. |
| Three management dialogs use generic `aria-label` naming instead of heading-linked labeling. | Medium | Dialog announcements are less descriptive and less consistent than the better-labelled modals elsewhere in the app. | `web/static/index.html:1643`, `web/static/index.html:1666`, `web/static/index.html:1687` | Add heading IDs and switch these dialogs to `aria-labelledby`. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| frontend-engineer | The main blockers are in component semantics. | Semantic menu/dialog/status refactor list. |
| ux-review | Interaction changes must remain coherent, not only compliant. | Menu and feedback redesign review. |
| product-manager | Accessibility fixes affect visible workflow shape. | Prioritize semantic menus vs. other SPA work. |

## Verdict
The repo has a workable accessibility foundation, especially around labels and focus trapping. The main gaps are interactive semantics and dynamic feedback, both of which are fixable but currently reduce confidence in important admin and navigation workflows.

## Changes since last assessment
- Focus handling remains good, but custom-menu semantics and non-announced status changes are still notable blockers (`web/static/index.html:3397-3474`, `web/static/index.html:6129-6164`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Non-semantic project/profile menus | High | Replace interactive `div`s with semantic controls. | frontend-engineer |
| Non-announced status feedback | Medium | Add live regions and text-based success/error cues. | frontend-engineer |
| Generic management-dialog labels | Medium | Use `aria-labelledby` consistently. | frontend-engineer |
