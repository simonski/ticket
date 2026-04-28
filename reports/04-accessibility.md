# Accessibility

**Score: 75/100** (was 73)

## Mission
Ensure the system remains usable with keyboards, screen readers, and assistive technologies.

## Review objective
Verify semantic structure, labels, focusable controls, and error visibility in the web and CLI surfaces.

## Inputs reviewed
- `web/static/index.html`
- `web/site2/index.html`
- `internal/server/server.go`
- Playwright test surface

## Findings

### Passing checks
- Project modal fields have explicit labels (`web/static/index.html:1618-1625`).
- Prefix inputs constrain format at the browser level (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`).
- Server injects CSP nonces into root HTML, reducing pressure to weaken CSP for inline assets (`internal/server/server.go:425-437`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Accessibility regression proof is indirect. | Medium | UI changes can pass functional tests while breaking keyboard or screen-reader flows. | `tests/playwright`, `TESTING.md:138-153` | Add focused keyboard/focus assertions for modal, board, and forms. |
| CLI/web terminology drift remains possible. | Low | Assistive copy and help text can diverge from actual command behavior. | `cmd/tk/namespace_helpers.go`, `USER_GUIDE.md` | Update docs and help together with CLI noun changes. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| frontend-engineer | Needs keyboard/focus coverage. | Playwright a11y cases. |
| tech-writer | Command language must stay consistent. | Noun rules documentation. |

## Verdict
The project has a better semantic baseline than before. It still needs explicit a11y regression tests instead of relying on general browser smoke tests.

## Changes since last assessment
- Prefix validation and labels are improved.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing keyboard/focus proof | Medium | Add Playwright keyboard and focus assertions. | accessibility |
