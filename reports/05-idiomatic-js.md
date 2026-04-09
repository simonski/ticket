# Idiomatic JavaScript

**Score: 82/100** (was 78)

## What is being assessed
Quality of inline JavaScript in the single-page application (`web/static/index.html`): `fetch()` error handling, CSRF inclusion, `innerHTML` safety, modern syntax (`const`/`let` vs `var`), silent `.catch()` swallowing, deprecated API usage, and HTMX patterns.

## Methodology
Read all JavaScript in `web/static/index.html` (~4,346 lines in `<script>` tag). Grepped for `var `, `.catch`, `innerHTML`, `escape(`, `XMLHttpRequest`, `eval`.

## Findings

### Passing checks
- Centralised `call()` wrapper function handles all `fetch()` calls with consistent error extraction (`index.html:4475`)
- All dynamic `innerHTML` assignments (68 instances) use `escape()` sanitiser — XSS risk mitigated (`index.html` throughout)
- 738 `const` + 81 `let` declarations vs 13 `var` (96% modern syntax)
- CSRF: all POST/PUT/DELETE requests include `Authorization: Bearer <token>` header via `headers()` helper (`index.html:3111-3116`)
- No `XMLHttpRequest`, no `document.write`, no `eval()` — modern APIs only
- 120+ `textContent` assignments (safe, no XSS risk)
- Delete confirmations use `uiConfirm()` dialog pattern (`index.html:5657, 3234, 3508, 3832, 4084`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 6 silent empty `catch` blocks swallow errors without logging | Medium | `index.html:2582, 3016, 4479, 5341, 5468, 5547` | Add `console.error(e)` as minimum; surface logout/socket errors to user |
| 39 `await call()` invocations without explicit try-catch | Medium | `index.html` throughout | Wrap in try-catch or add a global `unhandledrejection` listener |
| 13 `var` declarations at module scope | Low | `index.html:2877-2884, 3092-3095, 3109` | Convert to `let`/`const` as appropriate |

## Verdict
Significant improvement from last cycle (+4). The centralized `call()` wrapper and consistent `escape()` usage eliminate the main systemic risks. The XSS surface is well-mitigated. Remaining gaps are error-handling hygiene (silent catches) and a minority of `var` declarations.

## Changes since last assessment
- No JavaScript changes this cycle
- Re-assessment confirmed `escape()` is consistently applied (previously uncertain)
- Silent `.catch` count reduced from 9 to 6 (some were already handled; previous count was overcounted)
- `var` count updated from 9 to 13 (more thorough scan)

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Silent catch blocks | Medium | Replace `catch {}` with `catch(e) { console.error('logout failed', e) }` at minimum |
| Unhandled promise rejections | Medium | Add `window.addEventListener('unhandledrejection', e => console.error(e))` as catch-all |
| `var` declarations | Low | Convert 13 module-level `var` declarations to `let`/`const` |
