# UX Review

**Score: 76/100** (was 74)

## Mission
Ensure interactions are coherent, legible, and efficient under normal and error conditions.

## Review objective
Check consistency of CLI/web interaction patterns, feedback, validation, and empty states.

## Inputs reviewed
- `web/static/index.html`
- `web/site2/index.html`
- `cmd/tk/*`
- `USER_GUIDE.md`

## Findings

### Passing checks
- Project-prefix validation is now aligned in both web UIs (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`).
- CLI empty-list behavior has been normalized in the current worktree (`cmd/tk/cmd_ticket.go:575-584`, `cmd/tk/namespace_helpers.go:14-20`).
- Web UI uses labels for project modal fields (`web/static/index.html:1618-1625`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| CLI behavior changed without user guide updates. | Medium | Users cannot discover new empty-get and bare-ID behavior from docs. | `cmd/tk/namespace_helpers.go`, `USER_GUIDE.md` | Add a short "namespace command rules" section. |
| Deployment UX defaults are unsafe. | High | A convenience default creates a bad first impression and high operational risk. | `deploy/compose.yaml:6-10` | Require explicit password input and show clear failure text. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | New command rules need documentation. | CLI noun UX summary. |
| security-engineer | First-run UX must not trade safety for convenience. | Bootstrap secret design. |

## Verdict
Core interaction consistency is improving, especially in CLI nouns and project-prefix validation. The deployment entrypoint remains the largest UX/safety conflict.

## Changes since last assessment
- Site2 prefix validation is aligned with backend limits.
- `tk ls`/namespace empty output is simpler and more consistent in the current worktree.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Undocumented command consistency model | Medium | Update user docs after CLI changes are committed. | ux-review |
