# Product Manager

**Score: 78/100** (was 77)

## Mission
Protect product focus: `ticket` should solve lightweight project and ticket management for small teams and agentic engineering workflows without expanding into an unfocused platform.

## Review objective
Verify that the delivered surfaces match the stated product promise, critical journeys, and boundaries.

## Inputs reviewed
- `README.md`
- `docs/DESIGN.md`
- `USER_GUIDE.md`
- `cmd/tk/help.go`
- `web/static/index.html`

## Findings

### Passing checks
- README clearly states the product, modes, interfaces, and authoritative docs (`README.md:1-25`).
- Design docs identify primary users and core journeys from local init through web board use (`docs/DESIGN.md:42-59`).
- The lifecycle vocabulary is explicit in README and matches the domain model (`README.md:9-18`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Production deployment journey is unsafe by default. | High | First-run users can deploy an internet-facing instance with the known admin password. | `deploy/compose.yaml:6-10`, `deploy/entrypoint.sh:12-16` | Make production bootstrap require an explicit secret; keep demo defaults only in a clearly named demo profile. |
| Product contract is broken for integrators. | High | API users cannot rely on the shipped OpenAPI artifact. | `openapi.yaml:1-5`, `Makefile:109-113` | Repair and gate OpenAPI before product/release communication. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| release-manager | Production readiness is blocked. | P0 deploy-default and OpenAPI decisions. |
| tech-writer | User-facing deployment docs must change with bootstrap behavior. | Updated quickstart/deploy copy. |

## Verdict
The product intent is coherent and well explained. The main product risk is that the most important server deployment journey currently teaches insecure behavior.

## Changes since last assessment
- CLI noun consistency work improves the terminal journey but is still uncommitted.
- Site prefix validation drift appears closed in both web UIs.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Unsafe first-run server path | High | Redesign first boot around explicit secrets. | product-manager |
