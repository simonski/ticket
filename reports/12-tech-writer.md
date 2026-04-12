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
- **Prefix, lifecycle-command, and runbook syntax drift is now corrected** — `docs/LIFECYCLE.md` and `docs/RUNBOOKS.md` match the current `tk` CLI verbs and flag names
- **The operational restore/export examples now reflect the real snapshot interface** — runbooks use `tk export -o ...` and `tk import -i ...` instead of stale pipeline/overwrite examples
- **Published API version metadata is back in sync** — `openapi.yaml` now matches `cmd/tk/VERSION`

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

The documentation set is back in sync on the high-visibility surfaces that were drifting. The main remaining risk in this category was stale examples rather than missing concepts, and that gap is now closed across lifecycle docs, runbooks, and published API metadata.

## Changes since last assessment
- Corrected the lifecycle prefix rule and current `tk` command examples in `docs/LIFECYCLE.md`
- Refreshed `docs/RUNBOOKS.md` to use the real export/import workflow and current CLI flag spellings
- Synced `openapi.yaml` version metadata with `cmd/tk/VERSION`

## Remaining recommendations

None. Re-audited on **2026-04-12** under **TK-130** after commit **`619ed5a`**.
