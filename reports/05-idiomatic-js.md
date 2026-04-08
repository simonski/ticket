# Idiomatic JavaScript

**Score: 78/100** (was 78)

## What is being assessed
Quality of the frontend JavaScript in `web/static/index.html` (6080 lines, ~247KB): modern syntax, fetch error handling, auth token inclusion, innerHTML XSS safety, DOM manipulation patterns, event listener lifecycle, dependency hygiene, and console log discipline.

## Methodology
Reviewed `web/static/index.html` in full at v0.1.737. Searched for `var`, `innerHTML`, `.catch`, `console.log`, `eval`, XSS patterns, auth token handling, event listener cleanup, timer management, and empty catch blocks. Reviewed `package.json` for dependencies.

## Findings

### Passing checks
- `const`/`let` used for 90%+ of declarations — modern scoping throughout (`index.html`)
- Centralized `call()` function wraps all `fetch()` calls with 401 handling and JSON parsing (`index.html:4476`)
- All state-changing requests include `Authorization: Bearer` token via `headers()` helper (`index.html:3111-3115`)
- Custom `escape()` function escapes `&`, `<`, `>`, `"`, `'` — applied consistently before all `innerHTML` assignments (`index.html:5596-5602`)
- Error messages displayed via `.textContent` — never raw `innerHTML` — preventing secondary XSS
- WebSocket and timer cleanup: `socket.close()` and `clearTimeout`/`clearInterval` called on teardown (`index.html:5341, 5468, 5547`)
- No `console.log`, `console.error`, or `console.warn` in production code
- Only 1 dev dependency (`@playwright/test`) — zero runtime browser dependencies
- `encodeURIComponent()` used for WebSocket URL token parameter (`index.html:5291`)
- Arrow functions and template literals used throughout — modern ES2020+ idioms
- No `eval()` or `Function()` constructor usage
- Justified empty catch blocks use WebGL fallback (`index.html:1967, 2693`), socket close, JSON parse, and focus fallback — all well-reasoned
- Ticket delete now guarded with `uiConfirm()` before issuing DELETE call (`index.html:5657`) — fixes previously reported gap

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 9 `var` declarations remain (function-scoped, hoisting risk) | Medium | `index.html:2877-2884, 3092-3095, 3109` | Replace with `let` or `const` |
| 2 silent `catch {}` blocks swallow real errors | Medium | `index.html:3755` (team-linked-projects), `4636` (ticket reload) | Log or surface these errors; never swallow silently |
| `.catch(() => {})` on `loadTeams().then(...)` — error silently discarded | Medium | `index.html:2582` | Add `.catch(err => setTeamStatus(err.message, true))` |
| No loading/disabled state on async submit buttons | Medium | `index.html:3288, 5659` | Disable button + label "Deleting…" during `await call()`; re-enable in `finally` |
| THREE.js loaded from CDN without SRI hash | Low | `index.html:1550` | Add `integrity="sha384-..."` attribute to the import |

## Verdict
The JavaScript is clean, secure, and largely idiomatic. The `escape()` pattern before every `innerHTML`, Bearer-token on all mutations, and zero `console.log` pollution are all strong. The ticket delete confirmation gap (previously High) has been fixed. The remaining issues are the 9 legacy `var` declarations, two genuine silent-swallow catch blocks, and the absence of button loading states — none of which are security issues, but all reduce robustness. Score holds at 78/100.

## Changes since last assessment
- **Fixed (High):** Ticket delete in web UI now requires `uiConfirm()` confirmation before issuing DELETE (`index.html:5657`) — previously no confirmation existed.
- No other JavaScript-relevant changes between v0.1.730 and v0.1.737; all other commits were Go backend, CI, config, and documentation.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Replace 9 `var` with `let`/`const` | Medium | `index.html:2877-2884, 3092-3095, 3109` |
| Fix 2 silent `catch {}` swallowers | Medium | `index.html:3755, 4636` — log or display error |
| Fix `.catch(() => {})` on team load chain | Medium | `index.html:2582` — surface error to status element |
| Add button loading states during async ops | Medium | Disable + "Saving…" label pattern on all `await call()` buttons |
| Add SRI hash to CDN-loaded THREE.js | Low | `index.html:1550` — add `integrity` attribute |
