# Tech Writer

**Score: 77/100** (was 81)

## Mission
Ensure the project can be understood, operated, and changed by someone who did not build it.

## Review objective
Assess documentation completeness, reading order, drift, and operational clarity.

## Inputs reviewed
- `README.md`
- `CLAUDE.md`
- `TESTING.md`
- `docs/ONBOARDING.md`
- `docs/RUNBOOKS.md`
- `docs/PRIVACY.md`

## Findings

### Passing checks
- The repo has a credible documentation spine: README, onboarding, testing, lifecycle, runbooks, and privacy docs are all present and linked (`README.md:31-40`, `docs/ONBOARDING.md:23-48`, `docs/RUNBOOKS.md:1-19`).
- Onboarding provides a strong reading order and a clear day-to-day development loop for new contributors (`docs/ONBOARDING.md:36-48`, `docs/ONBOARDING.md:89-117`).
- The runbooks are concrete and operational rather than aspirational (`docs/RUNBOOKS.md:22-217`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| README build-from-source guidance still tells readers to use `make build`, conflicting with onboarding guidance that says this should be release-only. | Medium | The top-level doc steers newcomers into accidental version bumps. | `README.md:64-71`, `docs/ONBOARDING.md:111-117` | Align README with the safer `go build -o ./bin/tk ./cmd/tk` development path. |
| Multiple docs still report 11 Playwright specs even though the repo now has 12. | Low | Readers cannot fully trust the test inventory without checking the tree themselves. | `CLAUDE.md:27`, `TESTING.md:124-126` | Update the count or generate it automatically. |
| The privacy document is visibly stale in versioning/examples. | Medium | One of the most policy-sensitive docs undermines confidence by looking outdated. | `docs/PRIVACY.md:4-5`, `docs/PRIVACY.md:113-114` | Refresh the privacy doc to current version, commands, and date. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| new-starter | Doc drift is a day-one productivity problem. | Updated newcomer reading path. |
| support-readiness | README/help entry points shape support outcomes. | Troubleshooting/help section proposal. |
| privacy-and-compliance | Privacy doc repair must reflect actual policy intent. | Updated privacy content checklist. |

## Verdict
The documentation set is broad and unusually useful for a project of this size. The main problem is trust drift: a few visible inconsistencies in high-traffic docs now matter more because the rest of the documentation is otherwise strong.

## Changes since last assessment
- Documentation breadth remains strong, but the score slips because the most visible drift items are still unresolved and easy for newcomers to hit (`README.md:64-71`, `CLAUDE.md:27`, `docs/PRIVACY.md:4-5`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| README build guidance conflict | Medium | Align README development build instructions with onboarding. | tech-writer |
| Stale Playwright counts | Low | Keep test inventory facts synchronized. | tech-writer |
| Stale privacy doc | Medium | Refresh version/examples and validate policy language. | privacy-and-compliance |
