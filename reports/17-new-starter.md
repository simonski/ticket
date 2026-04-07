# New Starter Onboarding

**Score: 62/100**

## What is being assessed
How effectively a new engineer can go from zero to productive — including reading order, development setup, workflow clarity, branching conventions, commit style, PR process, testing expectations, and common pitfalls. Good onboarding means a day-1 developer can contribute within hours, not days.

## Methodology
Reviewed all documentation files (README, CLAUDE.md, QUICKSTART*, TESTING.md, AGENTS.md, TICKETS.md, docs/RULES.md), inspected Makefile for setup/test targets, analysed git log for branching and commit conventions, and checked for CONTRIBUTING.md, PR templates, and a dedicated onboarding guide.

## Findings

### Passing checks
- Reading order is logical: README → CLAUDE.md → QUICKSTART → TESTING.md
- `make setup` installs all dependencies (Go modules + Node + Playwright) in one command
- `make test` runs all test suites (unit + integration + Playwright)
- Architecture is excellently documented: README Mermaid diagrams, `docs/DESIGN.md`, `CLAUDE.md` package table
- `TESTING.md` covers all 6 test suites with coverage thresholds
- `cmd/ticket/TICKETS.md` provides a complete ticket workflow reference with stage lifecycle diagram
- `CLAUDE.md` lists special commands (spec, sdlc, drift, next, review, continue, pr)
- Contract tests (`libtickettest/contract.go`) ensure consistent behavior across implementations
- `make dev` prints env vars needed for local development mode

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `make build` auto-increments patch version — undocumented pitfall | High | `Makefile:44-70` | Document in CLAUDE.md: use `go build -o ./bin/ticket ./cmd/ticket` for dev, reserve `make build` for releases |
| No `CONTRIBUTING.md` | High | repo root | Document branching, commit format, PR process, review expectations, coding style |
| No `docs/ONBOARDING.md` | High | docs/ | Create single entry-point guide for new starters |
| `AGENTS.md` is 4 lines (stub) | High | `AGENTS.md` | Expand with full agent workflow or merge into `TICKETS.md` |
| Branching convention not documented | Medium | repo root | Document `feature/TK-XXX-description` pattern (visible in git log but not written down) |
| Commit style inconsistent and undocumented | Medium | git history | Document: `TK-XXX: imperative verb description`; add pre-commit hook or template |
| `make test` fails if `setup-playwright` skipped — error message not obvious | Medium | `Makefile` | Add dependency check or clear error message pointing to `make setup` |
| No PR template (`.github/PULL_REQUEST_TEMPLATE.md`) | Medium | `.github/` | Add checklist: tests pass, docs updated, ticket linked, coverage thresholds met |
| Workflow info scattered across 3 files | Low | TICKETS.md, AGENTS.md, CLAUDE.md | Consolidate into single reference or add clear cross-links |
| Linux Chromium dependencies for Playwright not documented | Low | `TESTING.md` | Add note on required system libs (libx11-dev etc.) |
| Network dependency in `make setup` can fail offline | Low | `Makefile` | Document prerequisite of internet access; add offline setup notes |

## Verdict
The project has excellent architectural documentation and a working setup/test pipeline, but is missing the human-workflow documentation that day-1 developers need: no `CONTRIBUTING.md`, no onboarding guide, undocumented `make build` version-bump behaviour, and no PR template. A developer could spend hours puzzling over why their branch has version commits, or what branching convention to follow.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Create `docs/ONBOARDING.md` | High | Reading order, dev loop (don't use `make build`!), ticket workflow, pitfalls |
| Create `CONTRIBUTING.md` | High | Branching, commit style, PR process, review SLA, coding conventions |
| Document `make build` version-bump behaviour | High | Add prominent note to CLAUDE.md warning developers to use `go build` directly |
| Add PR template | Medium | `.github/PULL_REQUEST_TEMPLATE.md` with tests/docs/ticket checklist |
| Document branching convention | Medium | `feature/TK-XXX-description` pattern in CONTRIBUTING.md |
| Add `.git/commit_template` | Low | Enforce `TK-XXX: verb description` commit format |
