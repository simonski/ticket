# Supply Chain

**Score: 73/100** (was 80)

## Mission
Protect the project from dependency, build-pipeline, and artifact provenance compromise.

## Review objective
Assess whether dependencies, CI, container builds, and release artifacts are trustworthy and verifiable.

## Inputs reviewed
- `go.mod`
- `go.sum`
- `Dockerfile`
- `.github/workflows/makefile.yaml`
- `Makefile`
- `deploy/compose.yaml`

## Findings

### Passing checks
- Docker build stages are pinned to image digests, and GitHub Actions uses SHA-pinned action references (`Dockerfile:1-17`, `.github/workflows/makefile.yaml:15-17`, `.github/workflows/makefile.yaml:43`, `.github/workflows/makefile.yaml:56`, `.github/workflows/makefile.yaml:63`, `.github/workflows/makefile.yaml:87`).
- CI runs `gosec`, `govulncheck`, linting, and coverage thresholds as part of the normal workflow (`.github/workflows/makefile.yaml:22-38`, `Makefile:55-64`, `Makefile:105-127`).
- Release automation generates checksums and an SBOM instead of shipping opaque binaries only (`Makefile:171-182`, `Makefile:199-215`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Release artifacts and images are not signed or attested. | High | Consumers cannot verify that binaries and images genuinely came from the intended CI pipeline. | `Makefile:171-215`, `.github/workflows/makefile.yaml:86-100` | Add artifact signing plus container/image attestations and document verification steps. |
| The deploy compose file still uses `containrrr/watchtower:latest`. | Medium | A mutable deployment dependency weakens reproducibility and provenance. | `deploy/compose.yaml:15-24` | Pin Watchtower to a specific immutable digest. |
| The workflow has no provenance/SLSA-style attestation step. | Medium | Even with SBOMs and checksums, consumers still cannot verify build origin strongly. | `.github/workflows/makefile.yaml:47-100` | Add provenance generation/attachment to the release pipeline. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| devops-engineer | Most supply-chain controls land in CI and release automation. | Signing/attestation implementation plan. |
| release-manager | Unsigned artifacts are a ship/no-ship concern. | External-release acceptance criteria. |
| security-engineer | Provenance policy should match security expectations. | Verification policy and trust model. |

## Verdict
The repository is ahead of average on basic supply-chain hygiene—pinning and scanning are real. The big missing piece is provenance: without signatures or attestations, release trust still depends too heavily on hosting and convention.

## Changes since last assessment
- The fundamental gap remains the same theme but is now more important because the repo has stronger runtime controls than release-artifact controls (`Dockerfile:1-17`, `Makefile:171-215`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Unsigned artifacts/images | High | Add signing and verification docs. | devops-engineer |
| Mutable deploy dependency | Medium | Pin Watchtower by digest. | devops-engineer |
| No provenance step | Medium | Add build attestation to releases. | supply-chain |
