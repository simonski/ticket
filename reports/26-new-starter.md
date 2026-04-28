# New Starter

**Score: 74/100** (was 73)

## Mission
Represent the engineer who joins tomorrow and must become productive without private context.

## Review objective
Evaluate setup speed, reading order, local workflow clarity, testing expectations, and common traps.

## Inputs reviewed
- `README.md`
- `docs/ONBOARDING.md`
- `CONTRIBUTING.md`
- `TESTING.md`
- `Makefile`
- `CLAUDE.md`

## Findings

### Passing checks
- README provides concise project shape and start-here links (`README.md:1-35`).
- Onboarding explains prerequisites, reading order, setup, and daily workflow (`docs/ONBOARDING.md:23-109`).
- Contributing docs warn about `make build` version bump and recommend `make build-dev` (`CONTRIBUTING.md:37-39`).
- Makefile help exposes expected targets (`Makefile:14-57`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| OpenAPI validation fails during standard gates. | High | A new contributor hits a red documented quality gate without context. | `openapi.yaml:1-5`, `Makefile:109-113` | Fix immediately or document temporary known failure. |
| Worktree contains unrelated local `.ticket` and version changes. | Medium | New contributors may struggle to tell repo state from local runtime artifacts. | `.ticket/config.json`, `cmd/tk/VERSION:1` | Add cleanup guidance and avoid committing runtime artifacts. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| maintainer | New-starter friction is ownership debt. | Clean worktree/release checklist. |
| qa-architect | Red gate needs explanation or fix. | Validation status. |

## Verdict
A new engineer can navigate the repository better than before. The biggest day-one issue is hitting a broken OpenAPI validation gate in an otherwise well-documented workflow.

## Changes since last assessment
- Onboarding remains strong and current.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Red standard gate | High | Repair OpenAPI validation before onboarding claims "make test/gates pass". | new-starter |
