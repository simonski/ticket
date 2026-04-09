# New Starter

**Score: 86/100** (was 83)

## What is being assessed
Onboarding effectiveness from the perspective of a day-one engineer: reading order, way of working, setup speed, ticket sdlc, testing expectations, collaboration, common pitfalls, and documentation completeness.

## Methodology
Read `README.md`, `CLAUDE.md`, `QUICKSTART.md`, `CONTRIBUTING.md`, `docs/ONBOARDING.md` (if exists), `TESTING.md`. Searched for `ticket binary`, `ticket server`, `./ticket`, `bin/ticket` in docs.

## Findings

### Passing checks
- `docs/ONBOARDING.md` exists and is well structured: reading order, dev setup, ticket sdlc, test conventions, collaboration, common pitfalls table (`docs/ONBOARDING.md`)
- Reading order is explicit: README → CLAUDE.md → QUICKSTART → CONTRIBUTING → ONBOARDING → USER_GUIDE (`ONBOARDING.md:18-30`)
- `make setup` → `make test` flow works from clean clone; documented in ONBOARDING.md pitfalls table
- Binary rename `ticket → tk` reflected in CLAUDE.md, README, QUICKSTART, CONTRIBUTING, USER_GUIDE
- Common pitfalls table is comprehensive: 8 gotchas documented (`ONBOARDING.md:101-115`)
- `tk` ticket sdlc documented: `tk ls`, `tk start`, `tk state`, `tk close` (`ONBOARDING.md:58-80`)
- Test conventions documented in `TESTING.md` with example patterns
- Commit message style, PR conventions, branching in `CONTRIBUTING.md`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `ONBOARDING.md` still mentions "ticket binary" (pre-rename) | Low | `docs/ONBOARDING.md:108` | Change "ticket binary" → "tk binary" |
| `ONBOARDING.md` still says "ticket server" | Low | `docs/ONBOARDING.md:161` | Change "ticket server" → "tk server" |
| `QUICKSTART.md` may have `./ticket` references | Low | `QUICKSTART.md` | Audit for pre-rename binary references |
| No explicit SLA on PR review response time documented | Low | `CONTRIBUTING.md` | Add "PRs are reviewed within N business days" expectation |

## Verdict
Strong score (+3) and approaching best-in-class. The ONBOARDING.md provides a clear, sequenced path for new starters. The main remaining gap is two stale binary name references in ONBOARDING.md — these cause confusion for anyone following the docs literally.

## Changes since last assessment
- Binary rename `ticket → tk` propagated to key docs (README, CLAUDE.md, QUICKSTART, CONTRIBUTING, USER_GUIDE)
- `docs/ONBOARDING.md` added (was absent in prior assessment) — major improvement
- `tk init` now reports sdlc/role status post-setup — reduces "why isn't anything working" confusion
- Two stale `ticket` references remain in ONBOARDING.md

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix "ticket binary" → "tk binary" in ONBOARDING.md | Low | `sed -i 's/ticket binary/tk binary/g' docs/ONBOARDING.md` |
| Fix "ticket server" → "tk server" in ONBOARDING.md | Low | `sed -i 's/ticket server/tk server/g' docs/ONBOARDING.md` |
| Audit QUICKSTART.md for pre-rename binary refs | Low | `grep -n 'ticket' QUICKSTART.md` |
