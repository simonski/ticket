# QA Architect

**Score: 78/100** (was 77)

## Mission
Prove the important paths work and that dangerous failures are exercised before users discover them.

## Review objective
Assess the current test strategy, proof coverage, flakiness profile, and unverified risk areas.

## Inputs reviewed
- `TESTING.md`
- `Makefile`
- `.github/workflows/makefile.yaml`
- `libtickettest/contract.go`
- `playwright.config.js`
- `playwright.site2.config.js`

## Findings

### Passing checks
- The repo has multiple meaningful test layers: unit, integration, contract, shell-harness, executable-doc, and Playwright browser coverage (`TESTING.md:3-13`, `TESTING.md:21-78`, `TESTING.md:79-108`).
- CI enforces package-level coverage thresholds instead of treating coverage as a vanity metric (`Makefile:105-127`, `.github/workflows/makefile.yaml:25-38`).
- The `tk-test` runner deliberately randomizes the server port for doc-driven tests, which is a good anti-flake pattern (`TESTING.md:60-78`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The repository does not appear to gate OpenAPI/spec validity in CI. | High | A broken API contract can ship even when the main workflow is green. | `openapi.yaml:1-10`, `.github/workflows/makefile.yaml:22-38` | Add YAML/OpenAPI validation to CI and fail fast on malformed spec output. |
| Playwright still uses fixed local ports even though another repo test surface already solved this problem with dynamic ports. | Medium | Browser tests remain more fragile than necessary on developer machines and shared environments. | `playwright.config.js:7-15`, `playwright.site2.config.js:7-15`, `TESTING.md:60-78` | Apply the `tk-test` dynamic-port approach to Playwright web-server setup. |
| Testing docs still claim there are 11 Playwright specs when the repo contains 12. | Low | QA guidance is slightly stale, which erodes trust in the test inventory over time. | `TESTING.md:124-126` | Keep the test inventory current or generate it automatically. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| api-architect | Spec validation is both a QA and contract concern. | OpenAPI CI gate requirement. |
| devops-engineer | Dynamic-port browser test setup belongs in build/test plumbing. | Playwright server boot redesign. |
| release-manager | Current proof gaps affect ship confidence directly. | What remains unproven for release? |

## Verdict
The project has a stronger test strategy than many repos of similar size, and the layered approach is real. The remaining QA concern is proof completeness: spec validity and browser-test determinism still are not guarded as tightly as the rest of the system.

## Changes since last assessment
- Confidence rose slightly because the contract/doc-test surfaces are broader and more intentional than the earlier QA report captured (`TESTING.md:21-78`, `libtickettest/contract.go:1-281`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Missing spec-validation gate | High | Add YAML/OpenAPI validation to CI. | qa-architect |
| Fixed Playwright ports | Medium | Switch browser tests to dynamic free ports. | qa-architect |
| Stale test inventory docs | Low | Keep Playwright spec counts in sync automatically. | tech-writer |
