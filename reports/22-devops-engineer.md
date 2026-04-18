# DevOps Engineer

**Score: 84/100** (was 88)

## Mission
Protect repeatable builds, safe deployment, and correct runtime packaging.

## Review objective
Review CI/CD, container packaging, deployment manifests, and environment-sensitive operational defaults.

## Inputs reviewed
- `Dockerfile`
- `compose.yaml`
- `deploy/compose.yaml`
- `deploy/entrypoint.sh`
- `.github/workflows/makefile.yaml`
- `Makefile`

## Findings

### Passing checks
- The main container runs with dropped capabilities, `no-new-privileges`, healthchecks, and explicit resource limits in the local compose setup (`compose.yaml:11-29`).
- The production entrypoint initializes the DB only when needed and then execs the server directly (`deploy/entrypoint.sh:4-14`).
- CI covers build, Go quality gates, Playwright, and publish flow in one workflow (`.github/workflows/makefile.yaml:9-100`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The published deploy compose file pulls `ghcr.io/simonski/ticket:latest` and `containrrr/watchtower:latest`. | Medium | Deployment reproducibility is weaker than the build pipeline suggests. | `deploy/compose.yaml:2-28` | Pin deploy images to immutable digests or explicit release tags. |
| The release pipeline publishes artifacts and images without signing or provenance attachment. | Medium | DevOps can ship builds consistently, but not yet verifiably. | `.github/workflows/makefile.yaml:47-100`, `Makefile:171-215` | Add signing and provenance steps to the publish job. |
| The README still tells developers to use `make build`, even though that path increments the version and is intended for releases. | Low | Daily-development workflow remains easier to misuse than necessary. | `README.md:64-71`, `docs/ONBOARDING.md:111-117` | Align top-level build guidance with the safer developer workflow. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| supply-chain | Artifact trust work is shared DevOps/supply-chain work. | Signing and attestation plan. |
| tech-writer | Developer workflow guidance currently disagrees across docs. | Build-from-source doc correction. |
| release-manager | Mutable deploy references affect release approval quality. | Required immutability threshold for shipping. |

## Verdict
The delivery pipeline is functional and better controlled than many single-binary projects. The remaining DevOps work is mostly about immutability and trust: deployments and releases are convenient today, but not yet fully reproducible or verifiable.

## Changes since last assessment
- Container/runtime hardening is better substantiated than the earlier DevOps pass captured, especially in local compose, but deploy immutability is still behind the rest of the pipeline (`compose.yaml:11-29`, `deploy/compose.yaml:2-28`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Mutable deploy images | Medium | Pin deploy/runtime images immutably. | devops-engineer |
| Missing signing/provenance | Medium | Add signed and attested publish outputs. | supply-chain |
| Misleading build guidance | Low | Align top-level docs with the safe dev build path. | tech-writer |
