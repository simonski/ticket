# Release Manager

**Score: 70/100** (was 80)

## Mission
Decide whether the current repository state should be allowed to ship based on evidence, dependencies, and rollback confidence.

## Review objective
Reconcile the role findings into a practical ship/no-ship decision.

## Inputs reviewed
- `reports/00-SUMMARY.md`
- `openapi.yaml`
- `.github/workflows/makefile.yaml`
- `Makefile`
- `playwright.config.js`
- `playwright.site2.config.js`
- `docs/RUNBOOKS.md`

## Findings

### Passing checks
- Core Go quality gates, vulnerability scanning, and browser tests are wired into CI before publish (`.github/workflows/makefile.yaml:22-49`).
- The repo has a real rollback/recovery reference set through runbooks and backup guidance (`docs/RUNBOOKS.md:87-184`).
- Release packaging generates tarballs, checksums, and an SBOM, which is a useful baseline even though it is not the full provenance story (`Makefile:154-215`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The OpenAPI file is malformed and is not currently blocked by CI. | High | A broken public contract can ship in a nominally releasable state. | `openapi.yaml:1-10`, `.github/workflows/makefile.yaml:22-38` | Block releases on spec validation and repair the file immediately. |
| Release artifacts and images are not signed or attested. | High | External consumers cannot verify release provenance strongly enough. | `.github/workflows/makefile.yaml:86-100`, `Makefile:171-215` | Add signing and provenance before treating releases as high-trust outputs. |
| Browser-test execution remains vulnerable to fixed-port collisions in local/shared environments. | Medium | Release confidence for UI changes is lower than the test-suite breadth implies. | `playwright.config.js:7-15`, `playwright.site2.config.js:7-15` | Move Playwright to dynamic-port server setup. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| api-architect | Contract correctness is a release gate. | Spec repair and CI validation plan. |
| supply-chain | Provenance is the other primary release blocker. | Signing/attestation implementation. |
| qa-architect | Browser-test determinism affects ship confidence. | Dynamic-port browser gate plan. |

## Verdict
**Conditional no-go for high-confidence release.** The runtime itself is not the main blocker; the blockers are contract integrity and release trust. I would not approve a release that markets the API or security posture aggressively until the spec gate and provenance controls are in place.

## Changes since last assessment
- Release confidence fell because the contract/provenance gaps are now clearer and more material than the previous summary reflected (`openapi.yaml:1-10`, `Makefile:171-215`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Malformed OpenAPI not gated | High | Repair the spec and fail CI on invalid output. | api-architect |
| Unsigned release outputs | High | Add signing and attestation to publish flow. | release-manager |
| Flaky browser gate setup | Medium | Remove fixed-port coupling from Playwright runs. | qa-architect |
