# New Starter

**Score: 88/100** (was 87)

## What is being assessed
How quickly a new engineer can orient themselves: reading order, setup speed, workflow discoverability, test expectations, and whether the docs surface the latest commands and gotchas without tribal knowledge.

## Methodology
Reviewed `README.md`, `CLAUDE.md`, `QUICKSTART.md`, `USER_GUIDE.md`, and `docs/ONBOARDING.md`, then compared newcomer-facing guidance with the latest CLI/docs changes.

## Findings

### Passing checks
- **The repo now has a visible reading order** — README and ONBOARDING both start with explicit “Start here” sections (`README.md:33`, `docs/ONBOARDING.md:23-47`).
- **Build, test, and architecture expectations are explicit** — CLAUDE documents setup/test commands, the version-bump caveat on `make build`, and the main package architecture (`CLAUDE.md:3-27`, `CLAUDE.md:29-45`, `CLAUDE.md:76-82`).
- **The ticket workflow is discoverable in-repo** — ONBOARDING documents branch/PR expectations and common `.ticket/` pitfalls for contributors (`docs/ONBOARDING.md:182-221`).
- **`tk skill` is now visible to newcomers in the main user docs** — the command is documented in USER_GUIDE, QUICKSTART, and README (`USER_GUIDE.md:37-43`, `QUICKSTART.md:114`, `README.md:205`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `docs/ONBOARDING.md` still does not mention `tk skill`, even though the command now exists specifically to help agent/newcomer workflows | Medium | `docs/ONBOARDING.md:23-221`, `USER_GUIDE.md:37-43` | Add `tk skill` to the onboarding reading/first-steps section and explain when a newcomer should use it. |
| Runbook recovery examples still lean on `/tmp` scratch paths, which is avoidable noise for new contributors learning the repo workflow | Low | `docs/RUNBOOKS.md:95-106`, `docs/RUNBOOKS.md:118-121`, `docs/RUNBOOKS.md:155-158` | Rewrite the examples to use project-local or clearly user-chosen paths instead of `/tmp`-style scratch files. |

## Verdict
Onboarding is still one of the project’s strongest areas, and it got better with `tk skill` plus the refreshed agent-run docs. The remaining newcomer gaps are now small and concrete: one missing onboarding mention and a few runbook examples that still feel more ops-centric than first-day friendly.

## Changes since last assessment
- Credited `tk skill` as a new newcomer-facing command and documentation surface (`cmd/tk/cmd_setup.go:48-66`, `README.md:205`, `USER_GUIDE.md:37-43`).
- Credited the newer `tk get` and bulk draft/undraft behavior as lower-friction day-two/day-three workflows (`cmd/tk/printer.go:253-290`, `cmd/tk/cmd_ticket_lifecycle.go:262-323`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `tk skill` missing from ONBOARDING | Medium | Add the command to the onboarding path so new contributors discover it without reading the full user guide first. |
| `/tmp`-style runbook examples | Low | Use project-local or clearly user-selected paths in newcomer-facing recovery examples. |
