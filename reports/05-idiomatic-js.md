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
- **No remaining `.onclick =` handlers**: interactive UI actions now use `addEventListener` consistently
- **`textContent` used for error messages**: all `appStatus.textContent = err.message` patterns are XSS-safe
- **Custom `uiConfirm`/`uiAlert` dialogs** -- no native `alert()`/`confirm()`
- **WebSocket reconnect** with fallback polling (lines 4572-4586)
- **Project modal saves now surface request failures inline**: the create/edit modal wraps project save requests in `try/catch` and writes failures to `projModalError`
- **Best-effort catches are now documented**: silent `catch {}` sites now explain intentional fallbacks or ignore paths

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 4,578-line single `<script>` block | Advisory | `index.html:1603-6181` | Consider splitting into ES modules when the codebase grows further |

## Verdict

The JavaScript surface remains strong in this pass. The earlier fixes for the `allTickets` ReferenceError, the unescaped `s.sort_order`, and the lingering `var` declarations are still intact, and the previously open operational recommendations are now closed. The only notable follow-up left from this assessment is the advisory-sized single-script architecture.

## Changes since last assessment

- 2026-04-12 — TK-123 — commit `23f436c` documented best-effort `catch {}` paths in `web/static/index.html` so intentional ignores are explicit
- 2026-04-12 — TK-123 — commit `23f436c` verified project modal saves already surface `call()` failures in `projModalError`
- 2026-04-12 — TK-123 — commit `23f436c` verified the remaining UI actions use `addEventListener` rather than `.onclick =`

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| None | - | Completed on 2026-04-12 via TK-123 in commit `23f436c` |
