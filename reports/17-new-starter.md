# New Starter

**Score: 87/100** (was 88)

## What is being assessed

How effectively a new engineer can clone the repo, understand the architecture, run the product and tests, learn the workflow, and avoid common pitfalls without relying on tribal knowledge.

## Methodology

Reviewed onboarding-facing docs (`README.md`, `CLAUDE.md`, `QUICKSTART*.md`, `TESTING.md`, `docs/ONBOARDING.md`), build/test commands, and the current repository workflow guidance.

## Findings

### Passing checks
- **The repo still has a strong top-level doc set** — `README.md`, `CLAUDE.md`, quickstarts, testing docs, and `docs/ONBOARDING.md` give a newcomer several clear entry points
- **Build and test workflows are explicit** — setup, build, lint, and test commands are documented in `CLAUDE.md` and supported by the Makefile
- **Architecture and lifecycle concepts are documented** — package roles and lifecycle behavior are explained in `CLAUDE.md`, `SPEC.md`, and `docs/LIFECYCLE.md`
- **Ticket workflow is discoverable in-repo** — the project uses `tk`, `.ticket/`, and a documented local workflow rather than hiding work tracking elsewhere

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Reading order is still distributed across several docs instead of one crisp front door | Medium | `README.md`, `docs/ONBOARDING.md` | Add an explicit “read these in order” section in README and onboarding |
| Some docs still contain stale operational details | Medium | `docs/DESIGN.md`, `SPEC.md`, `docs/RUNBOOKS.md` | Resolve the known doc drift so newcomers do not learn obsolete behavior |
| The repo workflow expectations are still spread across multiple files | Low | `CLAUDE.md`, `CONTRIBUTING.md`, `docs/ONBOARDING.md` | Consolidate branching/PR/quality-gate expectations into one short section |
| Hidden gotchas remain around generated artifacts and large mixed-mode surfaces | Low | various docs | Add a concise “common pitfalls” section covering OpenAPI, Playwright, and local-vs-remote mode |

## Verdict

Onboarding is still comparatively strong: a new engineer can get productive without much guesswork, and the repo includes far more internal guidance than most projects of this size. The small regression reflects documentation drift and the fact that the ideal reading sequence is still implied more than explicitly curated.

## Changes since last assessment
- The onboarding doc set remains broad and useful
- The main regression comes from stale details in a few core docs rather than missing onboarding material

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| No single canonical reading order | Medium | Add a short “start here” sequence in README and onboarding docs |
| Doc drift in core references | Medium | Refresh DESIGN, SPEC, and RUNBOOKS to match current behavior |
| Workflow expectations are scattered | Low | Consolidate branch/PR/test expectations into one newcomer section |
| Common pitfalls are under-surfaced | Low | Add a concise gotchas section for setup, generated files, and test modes |
