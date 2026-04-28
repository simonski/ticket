# Tech Writer

**Score: 70/100** (was 73)

## Mission
Ensure the project can be understood, operated, and changed by someone who was not present for implementation.

## Review objective
Assess docs inventory, drift, setup clarity, examples, and operational guidance.

## Inputs reviewed
- `README.md`
- `USER_GUIDE.md`
- `TESTING.md`
- `docs/ONBOARDING.md`
- `docs/RUNBOOKS.md`
- `docs/SLO.md`
- `SPEC.md`
- `openapi.yaml`

## Findings

### Passing checks
- README points to the authoritative contract, guide, and design docs (`README.md:25-35`).
- Testing docs are detailed and executable-doc oriented (`TESTING.md:22-79`).
- Onboarding gives a clear reading order and daily loop (`docs/ONBOARDING.md:36-48`, `docs/ONBOARDING.md:89-109`).
- Runbooks and SLO docs exist (`docs/RUNBOOKS.md:1-19`, `docs/SLO.md:1-24`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| OpenAPI and version metadata drift from current version. | High | Documentation and generated references are not trustworthy. | `openapi.yaml:1-5`, `SPEC.md:1-4`, `cmd/tk/VERSION:1` | Repair version sync and update docs after release decision. |
| Deploy docs need bootstrap-password rewrite. | High | Users may follow insecure guidance. | `deploy/compose.yaml:6-10`, `docs/RUNBOOKS.md:40-50` | Update deploy/runbook/server quickstart after secure bootstrap change. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| release-manager | Docs cannot be finalized until blockers fixed. | Release decision. |
| product-manager | User-facing command changes need product wording. | CLI noun model. |

## Verdict
Documentation breadth is strong, but current contract/version/deploy drift is too important to ignore. Docs should be updated after the underlying fixes, not paper over them.

## Changes since last assessment
- SLO and runbook docs are present; OpenAPI regressed.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Contract/version drift | High | Add doc/version validation and refresh docs. | tech-writer |
