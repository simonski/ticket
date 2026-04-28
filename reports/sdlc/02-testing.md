# Testing

**Score:** 76/100 **(was 74)**

## Standard
The project proves behavior with reliable automated tests rather than confidence by inspection.

## Assessment scope
Go coverage gates, Playwright execution, executable docs, and CI test wiring.

## Inputs reviewed
- `TESTING.md`
- `Makefile`
- `.github/workflows/makefile.yaml`
- `playwright.config.js`
- `playwright.site2.config.js`
- `tests/quickstart_test.sh`
- assessment runs: `TICKET_FAST_HASH=1 make test-go-cover`, `npx playwright test -c playwright.config.js`, `npx playwright test -c playwright.site2.config.js`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Critical paths covered by automation | MUST | pass | `TESTING.md:138-153`; `tests/quickstart_test.sh:1-8`; assessment runs on 2026-04-26 | Breadth across Go, docs, and browser tests is real. |
| Bug fixes add/update regression tests | MUST | partial | recent Playwright and config-test fixes are visible in the current tree | This run did not audit every historical fix. |
| Tests deterministic and isolated | MUST | pass | `playwright.config.js:1-20`; `playwright.site2.config.js:1-19` | Fixed-port brittleness is gone. |
| CI executes relevant suites | MUST | pass | `.github/workflows/makefile.yaml:25-48` | OpenAPI, coverage, and Playwright run in CI. |
| Test failures fail the quality gate | MUST | pass | `Makefile:131-158`; `.github/workflows/makefile.yaml:25-48` | The documented quality gates are real and enforceable. |
| Harnesses reflect supported behavior | MUST | pass | `tests/quickstart_test.sh:1-8`; `TESTING.md:21-98` | Quickstarts and harnesses still mirror supported workflows. |
| Coverage thresholds for key layers | SHOULD | pass | `Makefile:131-153` | Thresholds are explicit and currently green. |
| Success/failure/edge behavior covered | SHOULD | partial | assessment run `TICKET_FAST_HASH=1 make test-go-cover` | Coverage breadth is improved, but server proof is still shallow. |
| Documentation examples executable/validated | SHOULD | pass | `tests/quickstart_test.sh:1-8` | Strong repo trait. |
| Local suites fast enough to run regularly | SHOULD | pass | `TESTING.md:138-153`; Playwright run passed in 36.4s / 7.4s on 2026-04-26 | Better than the baseline now that port conflicts are removed. |

## Findings

### Strengths
- The repo now meets its own enforced coverage policy again (`Makefile:131-153`, assessment run `TICKET_FAST_HASH=1 make test-go-cover` on 2026-04-26).
- Both Playwright entry points passed in this rerun, and the configs no longer depend on fixed local ports (`playwright.config.js:1-20`, `playwright.site2.config.js:1-19`).
- Executable quickstarts remain a meaningful source of confidence rather than dead documentation (`tests/quickstart_test.sh:1-8`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| `internal/server` proof should continue deepening beyond the gate | medium | Auth, websocket, and agent paths deserve ongoing targeted proof even though the gate now passes at 70% | assessment run `TICKET_FAST_HASH=1 make test-go-cover` (`internal/server` 70.0%) | Keep adding targeted proof around the riskiest server paths. |
| Race testing exists but is not part of CI | medium | Concurrency issues can still hide behind green default gates | `Makefile:122-123`; `.github/workflows/makefile.yaml:25-48` | Decide whether `go test -race ./...` or a narrower race job belongs in CI. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| qa | The proof boundary has improved, but server-risk proof is still limited | Carry forward server-coverage and race-policy questions |
| devops | CI scope determines whether race coverage becomes a standard gate | Decide what is affordable in the pipeline |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R15 | medium | Increase proof around auth, websocket, and agent/server paths, and decide whether race testing belongs in CI | testing | targeted test additions and CI budget | Stronger `internal/server` proof and explicit race-test policy |

## Changes since last run
- The previous red coverage-gate finding is closed; `internal/config` is now above its enforced threshold (`Makefile:131-153`).
- The previous fixed-port Playwright finding is closed; both configs now resolve a free localhost port (`playwright.config.js:1-20`, `playwright.site2.config.js:1-19`).

## Open questions
- Should the repo treat race coverage as a routine CI gate, or keep it as an explicit-but-manual validation step?

## Verdict
Testing is stronger than the baseline because the repo’s two biggest trust breaks - the red coverage gate and brittle fixed browser ports - are gone. The next ceiling is not breadth; it is depth in the server layer and an explicit policy for concurrency proof.
