# Maintainer

**Score: 68/100** (was 73)

## Mission
Protect long-term ownership cost and handoff safety.

## Review objective
Assess upgrade, debug, extension, docs, and release maintenance risks.

## Inputs reviewed
- `Makefile`
- `.github/workflows/makefile.yaml`
- `docs/ONBOARDING.md`
- `docs/DESIGN.md`
- `cmd/tk`

## Findings

### Passing checks
- Onboarding gives a practical reading order and daily loop (`docs/ONBOARDING.md:23-48`, `docs/ONBOARDING.md:89-109`).
- Makefile exposes setup, test, coverage, lint, docker, release, and deploy targets (`Makefile:14-57`).
- Contributor docs warn that `make build` bumps version and recommend `make build-dev` for dev builds (`CONTRIBUTING.md:37-39`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Version and contract files drift. | High | Maintainers cannot trust release artifacts without manual inspection. | `cmd/tk/VERSION:1`, `SPEC.md:1-4`, `openapi.yaml:1-5` | Add a single version consistency check. |
| Large CLI/test files remain ownership hotspots. | Medium | Future contributors face high context load. | `cmd/tk/cmd_ticket.go`, `cmd/tk/main_test.go` | Continue modularizing CLI command namespaces. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | Version/docs drift needs cleanup. | Version source-of-truth note. |
| tech-lead | Hotspot refactor should be planned. | CLI split plan. |

## Verdict
The repo is navigable, but maintainer confidence is hurt by artifact drift and large hotspots. Fixing version/OpenAPI consistency should be treated as maintenance debt, not a one-off typo.

## Changes since last assessment
- Onboarding and test docs remain strong.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Artifact drift | High | Validate version consistency across binary/spec/OpenAPI/docs. | maintainer |
