# Idiomatic JavaScript

**Score: 78/100**

## What is being assessed
Quality of the frontend JavaScript in `web/static/index.html` (6077 lines, 247KB): modern syntax, fetch error handling, CSRF token inclusion, innerHTML XSS safety, DOM manipulation patterns, event listener lifecycle, dependency hygiene, and console log discipline.

## Methodology
Reviewed `web/static/index.html` in full. Searched for `var`, `innerHTML`, `.catch`, `console.log`, `eval`, XSS patterns, CSRF token handling, event listener cleanup, and timer management. Reviewed `package.json` for dependencies.

## Findings

### Passing checks
- `const`/`let` used for 90%+ of declarations — modern scoping
- Centralized `call()` function wraps all `fetch()` calls with 401 handling and JSON parsing
- All state-changing requests include `Authorization: Bearer` token via `headers()` helper (`index.html:3111-3115`)
- Custom `escape()` function (`index.html:5596-5602`) escapes `&`, `<`, `>`, `"`, `'` — applied consistently before all `innerHTML` assignments
- Error messages displayed via `.textContent` — never `innerHTML` — preventing secondary XSS
- WebSocket and timer cleanup: `socket.close()` and `clearTimeout`/`clearInterval` called on teardown
- No `console.log`, `console.error`, or `console.warn` in production code
- Only 1 dev dependency (`@playwright/test`) — zero runtime dependencies
- `encodeURIComponent()` used for WebSocket URL token parameter (`index.html:5291`)
- Arrow functions and template literals used throughout — modern JS throughout
- No `eval()` or `Function()` constructor usage

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 9 `var` declarations remain (function-scoped, hoisting risk) | Medium | `index.html:2877-2884, 3092-3095, 3109` | Replace with `let` or `const` |
| Several `.catch(() => {})` silently swallow errors | Medium | `index.html:2582, 3755, 4636, 4675` | Log error or display to user; never swallow silently |
| No loading/disabled state on async buttons | Medium | `index.html:3288, 5667` | Disable button + show spinner during `await call()`; re-enable in `finally` |
| No success toast notifications — operations complete silently | Low | Web UI generally | Add transient success feedback (e.g., `"Saved"` banner for 2s) |
| THREE.js loaded from unpinned CDN (`three@0.161.0` via unpkg) | Low | `index.html:1551` | Pin CDN URL with SRI hash (`integrity="sha384-..."`) |

## Verdict
The JavaScript is clean, well-structured, and demonstrates good security hygiene — particularly the consistent use of `escape()` before `innerHTML`, Bearer token auth on all mutations, and zero console.log pollution. The few remaining `var` declarations and silent `.catch({})` swallowers are the main quality gaps.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Replace 9 `var` with `let`/`const` | Medium | `index.html:2877-2884, 3092-3095, 3109` |
| Fix silent `.catch(() => {})` — show or log error | Medium | `index.html:2582, 3755, 4636, 4675` |
| Add button loading states during async ops | Medium | Disable + spinner pattern on all `await call()` buttons |
| Add SRI hash to CDN-loaded THREE.js | Low | `index.html:1551` — add `integrity` attribute |
| Consider bundling THREE.js locally | Low | Removes CDN dependency and latency |
