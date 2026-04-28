# Supply Chain

**Score: 64/100** (was 72)

## Mission
Protect the project from dependency, build, and release artifact compromise.

## Review objective
Assess dependency hygiene, CI pinning, scanning, container provenance, release signing, and deploy references.

## Inputs reviewed
- `go.mod`
- `package.json`
- `Dockerfile`
- `.github/workflows/makefile.yaml`
- `Makefile`
- `deploy/compose.yaml`

## Findings

### Passing checks
- Go dependencies are module-pinned in `go.mod` and `go.sum`; direct set is small (`go.mod:1-13`).
- Node dependency surface is limited to Playwright dev tooling (`package.json:1-7`).
- GitHub Actions are pinned by full commit SHA (`.github/workflows/makefile.yaml:15-18`, `.github/workflows/makefile.yaml:59-67`).
- Dockerfile pins base images by digest and runs non-root (`Dockerfile:1-44`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Production compose uses mutable latest tags. | High | Deployments can change without explicit operator approval. | `deploy/compose.yaml:3`, `deploy/compose.yaml:27-33` | Use versioned tags or document digest pinning for production. |
| Release workflow lacks signing/attestation. | High | Users cannot verify binary/image provenance beyond checksums. | `.github/workflows/makefile.yaml:96-103`, `Makefile:173-180` | Add cosign/SLSA attestation and publish verification instructions. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| devops-engineer | Compose/runtime defaults need change. | Pinning strategy. |
| release-manager | Provenance affects ship decision. | Signing plan. |

## Verdict
Dependency hygiene is strong, but release provenance and mutable deploy references are not good enough for production confidence.

## Changes since last assessment
- Dockerfile remains hardened; compose defaults remain risky.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Mutable deploy tags | High | Pin production deploy references. | supply-chain |
