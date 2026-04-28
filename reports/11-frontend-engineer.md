# Frontend Engineer

**Score: 74/100** (was 73)

## Mission
Ensure browser behavior is robust, safe, and maintainable.

## Review objective
Review web UI structure, validation, CSP integration, and browser test proof.

## Inputs reviewed
- `web/static/index.html`
- `web/site2/index.html`
- `internal/server/server.go`
- `tests/playwright`
- `TESTING.md`

## Findings

### Passing checks
- Browser suites cover 12 Playwright specs (`TESTING.md:138-153`).
- Server injects CSP nonces into style/script tags for the SPA root (`internal/server/server.go:425-437`).
- CSP headers include script/style nonce directives (`internal/server/server.go:255-265`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Browser tests emphasize functionality more than failure modes. | Medium | Network/server failure regressions can escape. | `TESTING.md:138-153` | Add Playwright cases for 500, timeout, and disconnected websocket states. |
| Inline-heavy SPA remains CSP-sensitive. | Medium | New inline script/style tags can break or force CSP weakening. | `web/static/index.html:1-70`, `internal/server/server.go:425-437` | Add tests that assert nonce injection after web edits. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| qa-architect | Needs browser negative-path coverage. | Playwright failure matrix. |
| security-engineer | CSP regression tests support XSS controls. | CSP test expectations. |

## Verdict
The web surface has useful tests and a real CSP strategy. The next maturity step is negative-path browser coverage.

## Changes since last assessment
- Main and site2 prefix validation are aligned.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing negative-path browser tests | Medium | Add Playwright 500/timeout/websocket failure cases. | frontend-engineer |
