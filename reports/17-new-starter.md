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
- **README and ONBOARDING now provide a clear front door** — both docs now include an explicit “start here” path for new contributors (`README.md`, `docs/ONBOARDING.md`)
- **Newcomer workflow expectations are now consolidated** — `docs/ONBOARDING.md` now captures branch naming, commit style, quality gates, and doc-update expectations in one section
- **Common onboarding pitfalls are now documented where newcomers will see them** — `docs/ONBOARDING.md` now calls out Playwright setup, local-vs-remote mode confusion, spec drift, and `.ticket/ticket.db` rebase conflicts
- **Core reference docs were refreshed to current behavior** — `docs/DESIGN.md`, `SPEC.md`, and `docs/RUNBOOKS.md` now reflect the current `tk` command surface, config-home rules, project prefix rule, and story model

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| None | - | - | Previous onboarding recommendations were completed on 2026-04-12 via TK-135 in commit `6f840bf` |

## Verdict

Onboarding is still comparatively strong: a new engineer can get productive without much guesswork, and the repo includes far more internal guidance than most projects of this size. The small regression reflects documentation drift and the fact that the ideal reading sequence is still implied more than explicitly curated.

## Changes since last assessment
- 2026-04-12 — TK-135 — commit `6f840bf` added explicit “start here” sections to `README.md` and `docs/ONBOARDING.md`
- 2026-04-12 — TK-135 — commit `6f840bf` consolidated branch, PR, and quality-gate expectations into a newcomer workflow section in `docs/ONBOARDING.md`
- 2026-04-12 — TK-135 — commit `6f840bf` added a focused newcomer pitfalls section covering Playwright setup, local-vs-remote mode, spec drift, and `.ticket/ticket.db` workflow friction
- 2026-04-12 — TK-135 — commit `6f840bf` refreshed `docs/DESIGN.md`, `SPEC.md`, and `docs/RUNBOOKS.md` to match the current `tk` command surface and config behavior

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| None | - | Completed on 2026-04-12 via TK-135 in commit `6f840bf` |
