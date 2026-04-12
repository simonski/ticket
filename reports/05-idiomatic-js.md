# Idiomatic JavaScript

**Score: 88/100** (was 88)

## What is being assessed

All inline JavaScript in `web/static/index.html` -- the only JS in the project (~4,578 lines inside a single `<script>` block, lines 1603-6181). Assessed for modern syntax, XSS safety, fetch error handling, CSRF discipline, and DOM manipulation patterns.

## Methodology

Full read of the script block with targeted searches for: `var` declarations, `innerHTML` without `escape()`, raw `fetch()` outside the `call()` wrapper, `catch {}` patterns, `.onclick` vs `addEventListener`, `==` vs `===`, `eval`/`Function`/`document.write`, CSRF header inclusion, and the specific bugs flagged in the previous assessment.

## Findings

### Passing checks

- **`allTickets` ReferenceError fixed**: no occurrences of `allTickets` remain in the file (was a bug at lines 5074, 5173)
- **`s.sort_order` now escaped**: line 4113 uses `escape(String(s.sort_order))` inside innerHTML
- **Zero `var` declarations**: 752 `const`, 94 `let`, 0 `var` (was 13 `var` previously)
- **`escape()` function** at line 5699 covers `& < > " '` -- applied consistently to all server-sourced innerHTML interpolations
- **Single `fetch()` call site**: only at line 4558 inside the `call()` wrapper -- no raw fetch elsewhere
- **`call()` wrapper** throws on non-OK responses (line 4568) and handles 401 session expiry (line 4562)
- **CSRF token included**: `getCsrfToken()` at line 3159 reads `_csrf` cookie; `headers()` at line 3164 attaches `X-CSRF-Token` to every request
- **Strict equality only**: 192 `===` comparisons, zero loose `==` comparisons
- **No `eval`, `new Function`, or `document.write`**
- **`addEventListener` dominant**: 113 uses vs 9 `.onclick` assignments
- **`textContent` used for error messages**: all `appStatus.textContent = err.message` patterns are XSS-safe
- **Custom `uiConfirm`/`uiAlert` dialogs** -- no native `alert()`/`confirm()`
- **WebSocket reconnect** with fallback polling (lines 4572-4586)

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `projModalCreate` click handler has no `try/catch` around `call()` -- thrown errors (network, 401, 5xx) go unhandled | Medium | `index.html:5042-5072` | Wrap lines 5067-5071 in try/catch; show error in `projModalError` |
| `.onclick =` used in 9 places instead of `addEventListener` | Low | `index.html:4118,4287,4311,5176,5182,5213,5246,5275,5283` | Migrate to `addEventListener` for consistency; `.onclick = null` can be replaced with `AbortController` |
| 8 silent `catch {}` blocks with no logging or comment | Low | `index.html:2630,3064,3813,4561,4719,5444,5534,5571,5650` | Add `// best-effort` comments for intentional ignores (logout, socket.close); add `console.warn` for the capacity polling catch at 5534 and the team fetch catch at 3813 |
| 4,578-line single `<script>` block | Advisory | `index.html:1603-6181` | Consider splitting into ES modules when the codebase grows further |

## Verdict

The JavaScript surface remains strong in this pass. The earlier fixes for the `allTickets` ReferenceError, the unescaped `s.sort_order`, and the lingering `var` declarations are still intact, and the remaining issues stay limited to one missing `try/catch`, a few `.onclick` uses, and silent `catch {}` blocks.

## Changes since last assessment

- No material JavaScript regressions were found in this pass
- The earlier escaping and modern-syntax fixes remain intact
- The same small error-handling and consistency issues remain open

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `projModalCreate` missing try/catch | Medium | Wrap `call()` and subsequent awaits in try/catch; display error in modal |
| `.onclick` pattern (9 occurrences) | Low | Migrate to `addEventListener` for consistency |
| Silent `catch {}` blocks (8 occurrences) | Low | Add comments or `console.warn` to aid debugging |
