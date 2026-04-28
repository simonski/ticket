# User Researcher

**Score: 76/100** (was 73)

## Mission
Protect task success, recoverability, and user trust for real users operating `tk` through CLI, web, and server modes.

## Review objective
Determine whether users can understand the workflow, recover from common mistakes, and complete high-value tasks.

## Inputs reviewed
- `README.md`
- `QUICKSTART.md`
- `TESTING.md`
- `docs/ONBOARDING.md`
- recent CLI noun UX changes

## Findings

### Passing checks
- New users get an explicit reading order and setup loop (`docs/ONBOARDING.md:23-48`, `docs/ONBOARDING.md:69-109`).
- Executable docs reduce stale quickstart risk by running fenced bash blocks (`TESTING.md:22-49`).
- README distinguishes local and server modes early (`README.md:20-25`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Deploy/runbook journey conflicts with bootstrap reality. | High | Users may both rely on the default password and be told to register a first admin, creating confusion and risk. | `deploy/entrypoint.sh:12-16`, `docs/RUNBOOKS.md:40-50` | Rewrite first-run server flow once bootstrap secret handling is fixed. |
| Current CLI UX improvements are not yet persistent. | Medium | Users can see better behavior locally, but it is not committed or released. | `cmd/tk/namespace_helpers.go`, `cmd/tk/main_test.go` | Commit, rerun full gates, and update `USER_GUIDE.md` for new `get`/`ls` rules. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| ux-review | CLI noun consistency should align with web terminology. | Current/most-recent entity behavior. |
| support-readiness | Conflicting bootstrap guidance affects support scripts. | First-run recovery path. |

## Verdict
User guidance is much stronger than the baseline, especially around onboarding and executable docs. Trust suffers where deployment instructions and bootstrap defaults conflict.

## Changes since last assessment
- Empty-list and empty-get CLI behavior is being standardized.
- OpenAPI validation regressed, which harms API-user trust.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Confusing first-run server story | High | Make one secure bootstrap path and document only that path for production. | user-researcher |
