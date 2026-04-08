# New Starter

**Score: 83/100** (was 62)

## What is being assessed
How effectively a new engineer can go from zero to productive — including reading order, development setup, workflow clarity, branching conventions, commit style, PR process, testing expectations, and common pitfalls. Good onboarding means a day-1 developer can contribute within hours, not days.

## Methodology
Reviewed all documentation files (README, CLAUDE.md, QUICKSTART, QUICKSTART_CLIENT, QUICKSTART_SERVER, CONTRIBUTING.md, TESTING.md, docs/ONBOARDING.md, docs/DESIGN.md, docs/RULES.md), inspected Makefile for setup/test targets, checked `.github/` for PR/issue templates, and verified cross-references between documents. Version 0.1.737.

## Findings

### Passing checks
- README.md is a strong entry point: project intro, install, build-from-source, usage, env vars, full architecture Mermaid diagrams, and data model (README.md)
- README.md warns clearly that `make build` increments the version with a code-block note (README.md)
- QUICKSTART.md presents local vs server mode choice with links to detailed per-mode guides (QUICKSTART.md)
- QUICKSTART_CLIENT.md covers local mode end-to-end in 9 numbered steps (QUICKSTART_CLIENT.md)
- QUICKSTART_SERVER.md covers server mode end-to-end including agents and web UI (QUICKSTART_SERVER.md)
- CONTRIBUTING.md (new) covers branching, commit style, PR process, PR checklist, testing expectations, coding conventions, and architecture decisions (CONTRIBUTING.md)
- docs/ONBOARDING.md exists with reading order, prerequisites, clone-and-setup, daily dev loop, ticket workflow, test commands, and common pitfalls table (docs/ONBOARDING.md)
- `make setup` installs all dependencies (Go modules + Node + Playwright + dev tools) in one command (Makefile)
- `make test` runs all test suites; individual targets (test-unit, test-integration, test-go-cover, test-playwright) documented (CLAUDE.md, TESTING.md)
- `make dev` prints env vars for local development mode (CLAUDE.md)
- CLAUDE.md has complete package table, `make build` pitfall warning, single-test invocation, coverage thresholds, Docker targets (CLAUDE.md)
- TESTING.md documents all 6 test targets with estimated durations and the contract test pattern (TESTING.md)
- `cmd/tk-test` turns Quickstart markdown bash blocks into executable tests — new starters' tutorials are CI-verified (TESTING.md)
- Contract tests (`libtickettest/contract.go`) ensure `LocalService` and `HTTPService` behave identically (TESTING.md)
- Linux Playwright system dependencies documented (docs/ONBOARDING.md)
- Common pitfalls table covers 7 failure modes with fixes (docs/ONBOARDING.md)
- Architecture and data model documented in README (Mermaid), CLAUDE.md, and docs/DESIGN.md

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No `.github/PULL_REQUEST_TEMPLATE.md` — PR checklist exists in CONTRIBUTING.md text but is not auto-populated in GitHub PRs | Medium | `.github/` | Add `.github/PULL_REQUEST_TEMPLATE.md` containing the checklist from CONTRIBUTING.md so it appears in every new PR |
| CONTRIBUTING.md says "104-method Service monolith" but CLAUDE.md says 108 methods | Low | `CONTRIBUTING.md:138`, `CLAUDE.md` | Sync the method count; use a comment in the interface file as the authoritative source |
| ONBOARDING.md reading order item 3 says "QUICKSTART.md — fastest path to a running server" but QUICKSTART.md is a mode-choice guide, not server-specific | Low | `docs/ONBOARDING.md` | Change description to "fastest path to a running local or server workspace" |
| `make test-go` is a valid Makefile target but is not listed in CLAUDE.md's build/test command table | Low | `CLAUDE.md` | Add `make test-go` to the command table so new starters know it exists |
| No issue templates in `.github/ISSUE_TEMPLATE/` — new contributors filing bugs have no guidance | Low | `.github/` | Add minimal bug-report and feature-request templates |

## Verdict
Onboarding quality has jumped sharply since the last assessment. The three previously missing high-severity items — `CONTRIBUTING.md`, `docs/ONBOARDING.md`, and the `make build` pitfall warning — are now fully addressed. README, QUICKSTART*, and CONTRIBUTING.md are all well-written and cross-linked. A day-1 engineer can now read a clear path from clone to first PR without guessing at conventions. Remaining gaps are cosmetic (no PR template, a stale method count, one misleading sentence) rather than blocking. The addition of `tk-test` as executable documentation is particularly strong: a new starter's quickstart tutorial is now CI-verified.

## Changes since last assessment
| Item | Change |
|------|--------|
| README.md | Rewritten — architecture diagrams added, install/build/usage all fleshed out, `make build` pitfall noted |
| QUICKSTART.md | Rewritten — now a clean mode-selection entry point with links to CLIENT and SERVER variants |
| QUICKSTART_CLIENT.md | Rewritten — 9-step walkthrough covering ideas, lifecycle, time, labels, decisions, TUI |
| QUICKSTART_SERVER.md | Rewritten — 9-step walkthrough covering auth, agents, web UI, Claude Code skill |
| CONTRIBUTING.md | Created — branching convention, commit format, PR checklist, testing table, coding conventions, architecture decision process |
| docs/ONBOARDING.md | Already existed; content covers all required areas |
| `make build` pitfall | Now documented prominently in README.md, CLAUDE.md, CONTRIBUTING.md, and docs/ONBOARDING.md |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| No `.github/PULL_REQUEST_TEMPLATE.md` | Medium | Create from the checklist already in CONTRIBUTING.md — one file, immediate GitHub UX improvement |
| Stale method count in CONTRIBUTING.md | Low | Change "104-method" to "108-method" (or make it a link to the interface file) |
| Misleading ONBOARDING.md description for QUICKSTART.md | Low | Update step 3 to say "mode-selection guide" rather than "running server" |
| `make test-go` missing from CLAUDE.md table | Low | Add one line to the command table |
| No issue templates | Low | Add `.github/ISSUE_TEMPLATE/bug_report.md` and `feature_request.md` |
