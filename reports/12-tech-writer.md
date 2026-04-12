# Tech Writer

**Score: 81/100** (was 82)

## What is being assessed

Accuracy and maintenance of the main documentation corpus: README, SPEC, USER_GUIDE, DESIGN, LIFECYCLE, runbooks, onboarding docs, and how well the docs track current CLI and runtime behavior.

## Methodology

Reviewed the current doc set alongside the existing report baseline and compared key user-facing claims against the code in `cmd/tk`, `internal/config`, and the current version file.

## Findings

### Passing checks
- **The doc corpus is still broad and discoverable** — top-level guides plus `docs/` still cover architecture, lifecycle, onboarding, testing, and runbooks
- **The lifecycle documentation remains the right conceptual center** — `docs/LIFECYCLE.md` still captures the authoritative stage/state model used by the codebase
- **The onboarding and testing docs still exist and are substantial** — `docs/ONBOARDING.md` and `TESTING.md` remain strong entry points for new contributors

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Prefix length docs are still stale | High | `SPEC.md`, `docs/LIFECYCLE.md` | Update all docs to match the current `^[A-Z]{1,5}$` rule |
| `tk init` non-interactive flags remain undocumented | High | `SPEC.md`, `USER_GUIDE.md` | Document `-sdlc`, `-prefix`, `-name`, and `-git` explicitly |
| `TICKET_CONFIG_DIR` references remain stale | High | `docs/DESIGN.md` | Replace with `TICKET_HOME` and the current walk-up discovery behavior |
| Default DB path documentation is outdated | High | `docs/DESIGN.md` | Replace `~/.config/ticket/ticket.db` with `.ticket/ticket.db` behavior |
| DESIGN ticket type list is incomplete | Medium | `docs/DESIGN.md` | Add `story`, `requirement`, and `decision` to match the current type set |
| Some operational docs still show old CLI syntax | Medium | `docs/RUNBOOKS.md` | Replace `ticket`/deprecated flag examples with current `tk` syntax |
| Version references are stale | Low | `SPEC.md`, `openapi.yaml` | Sync version text with `cmd/tk/VERSION` |

## Verdict

The documentation set is still one of the project’s strengths, but it lost a point because the code has moved faster than the docs in a few high-visibility places. The biggest problem is not missing documentation from scratch; it is stale documentation around environment naming, init flags, and prefix constraints.

## Changes since last assessment
- The main drift now centers on init flags, prefix length, and environment naming rather than missing SDLC concept coverage
- The newer CLI/runtime behavior continues to outpace updates in `DESIGN.md`, `SPEC.md`, and some operational docs

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Prefix length drift | High | Update docs to “1-5 uppercase ASCII letters” everywhere |
| Undocumented `tk init` flags | High | Document `-sdlc`, `-prefix`, `-name`, `-git` in SPEC and USER_GUIDE |
| Stale `TICKET_CONFIG_DIR` references | High | Replace with `TICKET_HOME` behavior |
| Stale DB path docs | High | Update DESIGN to `.ticket/ticket.db` |
| Incomplete ticket type list | Medium | Add the missing current types to DESIGN |
| Old CLI syntax in runbooks | Medium | Refresh commands to current `tk` forms |
| Version drift | Low | Sync doc versions with `cmd/tk/VERSION` |
