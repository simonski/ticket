# Idiomatic JavaScript

**Score: 81/100** (was 82)

## What is being assessed

All inline JavaScript in `web/static/index.html` — the only JS in the project (~4,500 lines inside a single `<script>` block, lines 1555-6078).

## Methodology

Read the entire script block with targeted searches for: `var` declarations, `.catch` and empty catch patterns, `innerHTML` vs `escape()`, `fetch()` vs `call()` wrapper, CSRF headers, `.onclick` vs `addEventListener`.

## Findings

### Passing checks
- `escape()` function defined at line 5596, covers `&<>"'`
- `escape()` applied to all server-sourced innerHTML assignments
- Single `fetch()` wrapper (`call()`) — raw `fetch` only at line 4476 inside `call`
- `call()` throws on non-OK responses (lines 4480-4486)
- `const`/`let` dominant: 703 `const`, 59 `let` vs 13 `var`
- Event delegation via `addEventListener` mostly used
- Custom `uiConfirm`/`uiAlert` dialogs — no native `alert`/`confirm`
- WebSocket reconnect logic present
- Token auth in every request via `headers()` helper at line 3111

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `allTickets` used but never declared — ReferenceError at runtime | Bug | `index.html:5074,5173` | Replace with `tickets` |
| `s.sort_order` interpolated in innerHTML without `escape()` | Medium | `index.html:4039` | Use `${escape(String(s.sort_order))}` |
| `projModalCreate` click handler has no `try/catch` on `call()` | Medium | `index.html:4964` | Wrap in try/catch; show `projModalError` |
| 13 `var` declarations remain | Low | `index.html:2877-3109` | Change to `let` |
| Silent `catch (_) {}` in `startChatCapacityPolling` | Low | `index.html:5431` | Add `console.warn` |
| `.onclick =` used in 8 places instead of `addEventListener` | Low | `index.html:4044,4210,4234,5073,5110,5143,5172,5180` | Migrate to `addEventListener` |
| Silent `catch {}` on logout call | Low | `index.html:3016` | Add `// best-effort` comment |
| Silent `catch {}` on team fetch in loop | Low | `index.html:3755` | At minimum log error |

## Verdict

Well-structured with clean `call()` wrapper and consistent `escape()` usage. The SDLC refactor made only minor naming changes (workflow->sdlc) with no new JS quality issues. Score drops 1 point due to confirmed `allTickets` bug.

## Changes since last assessment
- `workflow`/`workflowListEl` renamed to `sdlc`/`sdlcListEl` (commit `f5c243f`)
- No new JS patterns or issues introduced by the refactor

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `allTickets` ReferenceError | Bug | Replace with `tickets` at lines 5074, 5173 |
| Unescaped `s.sort_order` | Medium | Escape in innerHTML |
| `projModalCreate` missing try/catch | Medium | Wrap async call |
| 13 `var` declarations | Low | Change to `let` |
| `.onclick` pattern | Low | Migrate to `addEventListener` |
