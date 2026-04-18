# Frontend Engineer

**Score: 75/100** (was 88)

## Mission
Protect robust browser behavior, safe DOM rendering, and maintainable client-side workflows.

## Review objective
Assess whether the current SPA remains safe, coherent, and resilient under normal user interaction.

## Inputs reviewed
- `web/static/index.html`
- `playwright.config.js`
- `playwright.site2.config.js`
- `tests/playwright/auth.spec.js`

## Findings

### Passing checks
- The frontend centralizes HTML escaping before many `innerHTML` renders instead of interpolating raw values directly (`web/static/index.html:5957-5964`, `web/static/index.html:7113-7119`).
- Modal focus trapping and keyboard handling are implemented in one place and reused across dialogs (`web/static/index.html:6129-6164`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The project picker and profile menu still rely on custom `div`-based controls with click handlers. | Medium | Navigation and state changes remain more brittle and harder to maintain than semantic button/listbox patterns. | `web/static/index.html:1271-1284`, `web/static/index.html:3394-3474` | Replace these with shared semantic components. |
| Project membership editing depends on raw numeric user/team IDs. | Medium | An important management flow is harder than necessary and likely to drive avoidable errors. | `web/static/index.html:1491-1513` | Replace ID entry with searchable selectors and clearer result states. |
| Frontend browser tests use fixed local ports in both Playwright configs. | Low | Frontend regressions are harder to distinguish from environment collisions on developer machines and shared CI-like environments. | `playwright.config.js:7-15`, `playwright.site2.config.js:7-15` | Use random/free ports or a port-probing helper. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| ux-review | Several client-side issues are workflow-shape problems, not only implementation problems. | Menu/member-flow redesign. |
| accessibility | Semantic component changes must improve assistive behavior too. | Semantic menu/status requirements. |
| qa-architect | The browser-test configuration currently inflates frontend noise. | Dynamic-port test plan. |

## Verdict
The client has decent safety basics and good modal plumbing, but too many high-value interactions still depend on custom one-off controls. The next frontend step should improve both semantics and maintainability at the same time.

## Changes since last assessment
- The frontend now carries more product/admin surface than older JS-focused reports covered, and that has increased the cost of keeping one-file SPA controls coherent (`web/static/index.html:1`, `web/static/index.html:1491-1513`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Custom menu controls | Medium | Move to shared semantic menu/listbox components. | frontend-engineer |
| Numeric-ID admin flow | Medium | Implement searchable member selectors. | frontend-engineer |
| Fixed Playwright ports | Low | Make browser tests choose available ports automatically. | qa-architect |
