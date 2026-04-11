# New Starter

**Score: 84/100** (was 86)

## What is being assessed

Onboarding effectiveness — the full path from first clone to productive contributor, including documentation quality, accuracy, and coverage of the SDLC lifecycle refactor.

## Methodology

Read all seven documents in the reading order (README.md, CLAUDE.md, QUICKSTART.md, docs/ONBOARDING.md, TESTING.md, CONTRIBUTING.md, USER_GUIDE.md). Cross-checked claims against actual codebase. Reviewed Phase 2 commits for documentation gaps.

## Findings

### Passing checks
- Reading order documented and sequenced — `docs/ONBOARDING.md:25-31`
- README provides clear product summary — `README.md:1-35`
- QUICKSTART.md is concrete and runnable — `QUICKSTART.md:19-75`
- Branching conventions explicit — `CONTRIBUTING.md:46-57`
- Commit style documented — `CONTRIBUTING.md:60-76`
- PR process has checklist — `CONTRIBUTING.md:79-95`
- `make build` version-bump pitfall documented in 3 places
- Daily dev loop accurate — `ONBOARDING.md:75-92`
- Testing expectations comprehensive — `TESTING.md` + `CONTRIBUTING.md:99-125`
- Common pitfalls table solid — `ONBOARDING.md:156-165`
- `tk` tool usage covered end-to-end
- Lifecycle stage diagram in README (Mermaid)
- `make dev` exists and documented

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `docs/DESIGN.md` references non-existent `docs/TICKET_LIFECYCLE_SPEC.md` | High | `DESIGN.md:4` | Change to `docs/LIFECYCLE.md` |
| ONBOARDING.md has no SDLC concepts section — stages, roles, stage-roles unexplained | High | `ONBOARDING.md:105-132` | Add SDLC subsection with examples |
| USER_GUIDE.md SDLC commands use pre-refactor names | High | `USER_GUIDE.md:838-844` | Update to `stage-add`/`stage-rm`/`stage-order`; add role commands |
| DESIGN.md ticket model still lists old `open` field | Medium | `DESIGN.md:153` | Replace with `draft`/`complete` |
| `docs/LIFECYCLE.md` not in reading order | Medium | `ONBOARDING.md:25-31` | Add as item 5 or 6 |
| README.md doesn't link to `docs/ONBOARDING.md` | Medium | `README.md` | Add link under "Build from source" |
| ONBOARDING.md points to `cmd/tk/TICKETS.md` as reference — it's an agent template | Medium | `ONBOARDING.md:130-131` | Point to LIFECYCLE.md instead |
| Command syntax inconsistency between ONBOARDING.md and README.md | Low | `ONBOARDING.md:117-121` | Normalise to shortcut forms |
| No collaboration patterns section (review turnaround, escalation) | Low | `CONTRIBUTING.md` | Add section |
| `make dev` not mentioned in ONBOARDING.md daily loop | Low | `ONBOARDING.md:75-92` | Add mention |

## Verdict

Foundations remain strong. Score drops 2 points to 84 due to SDLC refactor documentation debt: no ONBOARDING.md section on new concepts, broken DESIGN.md reference, stale USER_GUIDE.md commands. A developer working with `tk sdlc` commands would have to piece together the picture from LIFECYCLE.md — which isn't in the reading order.

## Changes since last assessment
- Phase 2 SDLC refactor shipped across 6 commits
- README, QUICKSTART, SPEC, USER_GUIDE partially updated
- ONBOARDING.md and DESIGN.md not updated for new concepts

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix DESIGN.md broken reference | High | Point to `docs/LIFECYCLE.md` |
| Add SDLC concepts to ONBOARDING.md | High | Subsection with example commands |
| Update USER_GUIDE.md SDLC commands | High | Correct names + add role commands |
| Update DESIGN.md ticket model | Medium | Replace `open` with `draft`/`complete` |
| Add LIFECYCLE.md to reading order | Medium | `ONBOARDING.md:25-31` |
| Link ONBOARDING.md from README | Medium | Under "Build from source" |
