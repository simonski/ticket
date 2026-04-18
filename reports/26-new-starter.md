# New Starter

**Score: 84/100** (was 88)

## Mission
Represent the engineer who joins tomorrow and needs to become productive quickly without private context.

## Review objective
Evaluate setup clarity, reading order, workflow expectations, and common newcomer traps.

## Inputs reviewed
- `README.md`
- `docs/ONBOARDING.md`
- `CLAUDE.md`
- `CONTRIBUTING.md`
- `TESTING.md`
- `Makefile`

## Findings

### Passing checks
- The repo gives newcomers a strong reading path across README, onboarding, CLAUDE, contributing, and testing docs (`README.md:33-40`, `docs/ONBOARDING.md:23-48`).
- Onboarding includes practical day-to-day commands, warnings about the release-only build path, and common pitfalls instead of assuming tribal knowledge (`docs/ONBOARDING.md:89-117`).
- The Makefile exposes the main workflows clearly enough that a new contributor can discover build/test targets without spelunking scripts (`Makefile:1-31`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The first build instructions a newcomer sees in README still recommend `make build`, which conflicts with the onboarding warning that it bumps the version. | Medium | A new engineer can create accidental version churn before understanding the release workflow. | `README.md:64-71`, `docs/ONBOARDING.md:111-117` | Make the README’s first developer build path match onboarding. |
| The reported Playwright spec count is stale in contributor-facing docs. | Low | New contributors get a small but unnecessary trust dent when the docs disagree with the repo tree. | `CLAUDE.md:27`, `TESTING.md:124-126` | Keep generated/test inventory facts synchronized. |
| The repo layout still funnels several everyday change paths through oversized files. | Low | Newcomers can understand the system quickly, but contributing safely still gets harder than necessary in a few hotspots. | `web/static/index.html:1`, `cmd/tk/main.go:52-118`, `internal/client/client.go:1-260` | Continue decomposing the highest-churn files. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | The main day-one trap is documentation drift. | Newcomer-first doc cleanup list. |
| maintainer | Large hotspots affect both onboarding and long-term ownership. | Which file splits reduce newcomer risk fastest? |
| tech-lead | New contributors benefit from narrower change seams. | Refactor order for high-churn files. |

## Verdict
This is still a relatively good repo for a new engineer to enter: the reading order and workflow guidance are better than average. The score falls only because the first impression still contains a preventable build trap and a few visible doc drifts.

## Changes since last assessment
- Newcomer experience remains strong because onboarding is substantially better than a bare README workflow, but the highest-traffic top-level docs still need cleanup (`docs/ONBOARDING.md:23-48`, `README.md:64-71`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| README build trap | Medium | Put the safe dev build command in the first-start flow. | tech-writer |
| Stale test inventory facts | Low | Keep newcomer-visible counts synchronized automatically. | tech-writer |
| Large-file hotspots | Low | Reduce complexity in the main everyday change surfaces. | tech-lead |
