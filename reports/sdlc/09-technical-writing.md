# Technical Writing

**Score:** 73/100 **(was 70)**

## Standard
Documentation helps the next user, operator, and contributor succeed without tribal knowledge.

## Assessment scope
Repository entry docs, quickstarts, user/operator docs, privacy doc, and version-bearing public contract artifacts.

## Inputs reviewed
- `README.md`
- `QUICKSTART_SERVER.md`
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
| Core setup/usage docs match implementation | MUST | partial | `docs/PRIVACY.md:3-5`; `TESTING.md:150-153`; `docs/ONBOARDING.md:219-221`; `QUICKSTART_SERVER.md:13-35` | Several docs are now current, but bootstrap guidance still drifts. |
| Important workflows have a clear entry point | MUST | pass | `README.md:27-35`; `docs/RUNBOOKS.md:40-50` | Entry path remains good. |
| Commands/flags/examples/files accurate | MUST | partial | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Version-bearing artifacts are still inconsistent. |
| Assumptions/prereqs/recovery stated where relevant | MUST | pass | `docs/ONBOARDING.md:219-221`; `docs/RUNBOOKS.md:40-50` | Recovery guidance remains concrete. |
| Release-significant changes reflected in docs | MUST | partial | `docs/PRIVACY.md:3-5` fixed; quickstart/bootstrap wording still stale | Improvement is real, but not complete. |
| Operator-facing docs exist | MUST | pass | `docs/RUNBOOKS.md:40-50`; `deploy/README.md:1-13` | Operator-facing material exists. |
| Quickstarts executable | SHOULD | pass | `tests/quickstart_test.sh:1-8` | Strong repo trait. |
| Reference docs avoid duplication drift | SHOULD | partial | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Remaining version drift is the clearest duplication failure. |
| Beginner guidance vs deep reference separated | SHOULD | pass | README/quickstarts/onboarding/runbooks split remains clear | Good information architecture. |
| Examples use realistic current terminology | SHOULD | partial | `QUICKSTART_SERVER.md:13-35`; `USER_GUIDE.md:85-92`; `USER_GUIDE.md:159-162` | Bootstrap-password wording still reads as normalized behavior rather than explicitly demo-only. |

## Findings

### Strengths
- The documentation spine is stronger than the baseline: onboarding and testing docs now explicitly reflect the Playwright auto-port behavior and current development guidance (`TESTING.md:150-153`, `docs/ONBOARDING.md:219-221`).
- The privacy policy is current again (`docs/PRIVACY.md:3-5`).
- Runbooks use explicit `<secret>` placeholders for first-admin creation instead of normalizing a hardcoded password (`docs/RUNBOOKS.md:44-46`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Quickstart/user-guide bootstrap docs still normalize `admin/password` | high | Operator-facing docs still undercut a safer deploy posture | `QUICKSTART_SERVER.md:13-35`; `USER_GUIDE.md:85-92`; `USER_GUIDE.md:159-162` | Remove default-password guidance and require explicit bootstrap-secret instructions. |
| Version-bearing public artifacts still drift | medium | Readers still get conflicting answers about the current contract/spec version | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Sync version-bearing artifacts from one source of truth. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| cyber / infosec | Bootstrap-secret wording directly affects the security posture communicated to operators | Harden quickstart/user-guide bootstrap guidance |
| architecture | Version drift is also a contract/source-of-truth issue | Decide how spec/version artifacts should stay aligned |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R4 | high | Remove default bootstrap-password guidance from deploy docs and require explicit secret guidance | technical-writing | deploy UX | No default bootstrap password in quickstarts/user guide/deploy docs |
| R7 | medium | Sync `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION` from one source of truth | technical-writing | version/source-of-truth decision | Matching versions across spec, API contract, and binary |

## Changes since last run
- The stale privacy-doc finding is closed.
- The malformed OpenAPI finding is closed, but the version-coherence problem remains because `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION` still disagree.
- Testing/onboarding docs now correctly explain the Playwright auto-port behavior.

## Open questions
- Should `SPEC.md` and `openapi.yaml` be generated/synced automatically, or stay manual with a stricter update workflow?

## Verdict
Technical writing is materially better than the baseline because several high-traffic docs are current again and the OpenAPI file is no longer malformed. It still stops short of “excellent” because bootstrap-secret guidance remains too casual in quickstarts/user docs, and the project still does not present one authoritative current version across its public artifacts.
