# Technical Writing

**Score:** 80/100 **(was 73)**

## Standard
Documentation helps the next user, operator, and contributor succeed without tribal knowledge.

## Assessment scope
Repository entry docs, quickstarts, user/operator docs, privacy doc, and version-bearing public contract artifacts.

## Inputs reviewed
- `README.md`
- `docs/quickstarts/server.md`
- `USER_GUIDE.md`
- `TESTING.md`
- `docs/ONBOARDING.md`
- `docs/PRIVACY.md`
- `docs/RUNBOOKS.md`
- `SPEC.md`
- `openapi.yaml`
- `cmd/tk/VERSION`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Core setup/usage docs match implementation | MUST | pass | `docs/PRIVACY.md:3-5`; `TESTING.md:40-49`; `docs/ONBOARDING.md:207-220`; `docs/quickstarts/server.md:13-45` | Core setup docs now reflect current layout and remote/bootstrap behavior. |
| Important workflows have a clear entry point | MUST | pass | `README.md:27-35`; `docs/RUNBOOKS.md:40-50` | Entry path remains good. |
| Commands/flags/examples/files accurate | MUST | pass | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Version-bearing artifacts are aligned. |
| Assumptions/prereqs/recovery stated where relevant | MUST | pass | `docs/ONBOARDING.md:219-221`; `docs/RUNBOOKS.md:40-50` | Recovery guidance remains concrete. |
| Release-significant changes reflected in docs | MUST | pass | `docs/PRIVACY.md:3-5`; `docs/quickstarts/server.md:13-45`; `USER_GUIDE.md:83-165` | Bootstrap and version docs are refreshed. |
| Operator-facing docs exist | MUST | pass | `docs/RUNBOOKS.md:40-50`; `deploy/README.md:1-13` | Operator-facing material exists. |
| Quickstarts executable | SHOULD | pass | `tests/quickstart_test.sh:1-8` | Strong repo trait. |
| Reference docs avoid duplication drift | SHOULD | partial | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Remaining version drift is the clearest duplication failure. |
| Beginner guidance vs deep reference separated | SHOULD | pass | `README.md:27-42`; `docs/quickstarts/`; `docs/archive/` | The root now points to active docs while historical planning notes are archived. |
| Examples use realistic current terminology | SHOULD | pass | `docs/quickstarts/server.md:13-45`; `USER_GUIDE.md:83-165` | Bootstrap examples now use explicit credentials/placeholders for shared deployments. |

## Findings

### Strengths
- The documentation spine is stronger than the baseline: onboarding and testing docs now explicitly reflect the Playwright auto-port behavior and current development guidance (`TESTING.md:150-153`, `docs/ONBOARDING.md:219-221`).
- The privacy policy is current again (`docs/PRIVACY.md:3-5`).
- Quickstarts now live under `docs/quickstarts/`, historical plans live under `docs/archive/`, and root documentation has a clearer active-doc spine (`README.md:27-42`).
- Runbooks and server quickstarts use explicit secret guidance instead of normalizing a hardcoded password (`docs/RUNBOOKS.md:40-57`; `docs/quickstarts/server.md:13-45`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Keep moved-doc references fresh | low | Future doc moves can reintroduce broken paths | `README.md:27-42`; `Makefile:164-165`; `TESTING.md:40-49` | Continue running doc tests when moving user-facing docs. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| cyber / infosec | Bootstrap-secret wording directly affects the security posture communicated to operators | Keep docs aligned with future deploy hardening |
| architecture | Version drift is also a contract/source-of-truth issue | Keep `sync-openapi-version` and spec updates paired |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R7 | medium | Keep `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION` synced from one source of truth | technical-writing | version/source-of-truth decision | Matching versions across spec, API contract, and binary |

## Changes since last run
- The stale privacy-doc finding is closed.
- The malformed OpenAPI and version-coherence findings are closed for the current docs set.
- Testing/onboarding docs now correctly explain the moved quickstart paths and Playwright auto-port behavior.

## Open questions
- Should `SPEC.md` and `openapi.yaml` be generated/synced automatically, or stay manual with a stricter update workflow?

## Verdict
Technical writing is materially better than the baseline because several high-traffic docs are current again and the OpenAPI file is no longer malformed. It still stops short of “excellent” because bootstrap-secret guidance remains too casual in quickstarts/user docs, and the project still does not present one authoritative current version across its public artifacts.
