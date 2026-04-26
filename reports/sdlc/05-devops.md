# DevOps

**Score:** 72/100 **(was 70)**

## Standard
The project can be built, shipped, configured, observed, and recovered with disciplined operational practice.

## Assessment scope
GitHub Actions, release Makefile targets, deploy compose, health/metrics, and backup/restore docs.

## Inputs reviewed
- `.github/workflows/makefile.yaml`
- `Makefile`
- `deploy/compose.yaml`
- `deploy/entrypoint.sh`
- `docs/RUNBOOKS.md`
- `internal/server/api_system.go`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Builds reproducible from documented commands | MUST | pass | `Makefile:1-35`; `.github/workflows/makefile.yaml:22-41` | Build and test flows are explicit. |
| Deployment artifacts versioned and understandable | MUST | partial | `deploy/compose.yaml:1-45` | The deploy story is understandable, but still mutable. |
| CI/CD visible and repeatable | MUST | pass | `.github/workflows/makefile.yaml:1-103` | One workflow covers build, Playwright, and publish. |
| Runtime exposes enough health/logging for diagnosis | MUST | partial | `internal/server/api_system.go:19-85`; `internal/server/server.go:520-548` | Health and coarse metrics exist; deeper request telemetry does not. |
| Backup/restore documented for stateful system | MUST | pass | `docs/RUNBOOKS.md:126-184` | Backup/restore guidance is concrete. |
| Secrets not hardcoded in deploy artifacts | MUST | fail | `deploy/compose.yaml:6-10`; `deploy/entrypoint.sh:12-16` | The clearest current DevOps failure. |
| Signed or attestable release artifacts | SHOULD | fail | `Makefile:169-243`; `.github/workflows/makefile.yaml:50-103` | No signing/attestation path reviewed. |
| Environments configurable without source changes | SHOULD | pass | `deploy/compose.yaml:6-10` | Runtime env vars exist. |
| Infrastructure assumptions documented | SHOULD | partial | `docs/RUNBOOKS.md:40-50`; `deploy/compose.yaml:1-45` | Basic guidance exists, but not full proxy/secret/release posture. |
| Local workflows resemble production where practical | SHOULD | partial | `.github/workflows/makefile.yaml:22-48`; `TESTING.md:138-153` | CI is closer to reality than the deploy bundle is. |

## Findings

### Strengths
- CI is stronger than the baseline: OpenAPI validation, coverage gates, Playwright, lint, gosec, and govulncheck all sit in visible pipeline steps (`.github/workflows/makefile.yaml:25-48`).
- Release tooling still produces checksums and an SBOM (`Makefile:169-243`).
- Backup and restore expectations remain documented with concrete commands (`docs/RUNBOOKS.md:126-184`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Unsigned release artifacts and images | high | Consumers still cannot verify provenance of shipped binaries and images | `Makefile:169-243`; `.github/workflows/makefile.yaml:50-103` | Add signing/attestation to the release path. |
| Deploy compose still uses mutable `latest` tags | high | Reproducibility and rollback confidence remain weaker than they should be | `deploy/compose.yaml:3`; `deploy/compose.yaml:28-33` | Pin deploy images to release tags or digests. |
| Committed default admin password in deploy bundle | high | Operators can still deploy an insecure first-boot system by accident | `deploy/compose.yaml:6-10`; `deploy/entrypoint.sh:12-16` | Remove the default and require an explicit secret path. |
| Metrics endpoint lacks request-level counters/histograms | medium | Operational diagnosis and alerting remain shallow | `internal/server/api_system.go:33-85` | Add request-level counters/histograms to `/metrics`. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| infosec | Signing, secrets, and deploy mutability are also information-security issues | Carry forward release-trust and bootstrap-secret gaps |
| qa | Metrics depth and deployment reproducibility affect release confidence | Decide whether observability should become a stronger release gate |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R3 | high | Add release signing/attestation and stop publishing mutable deployment references | devops | CI/release changes | Signed artifacts/images and immutable deploy refs |
| R4 | high | Remove the committed deploy default password and require an explicit bootstrap secret path | cyber | deploy UX | No committed default password |
| R11 | medium | Add request-level counters/histograms to `/metrics` | devops | observability design | `/metrics` includes request-level telemetry |

## Changes since last run
- The pipeline is objectively healthier than the baseline because the OpenAPI gate, coverage gate, and Playwright runner are all green in the current tree (`.github/workflows/makefile.yaml:25-48`).
- The release/deploy hardening findings themselves did not move: signing, mutable refs, and bootstrap secrets remain open.

## Open questions
- Should the repo keep a demo-optimized compose bundle and add a second production profile, or harden the single shipped deploy path?

## Verdict
Operational maturity is real here: the CI workflow is visible, gates are meaningful, and runbooks exist. The score still trails because the default deploy path is too trusting, release provenance is not cryptographically anchored, and observability is still shallower than a production-grade standard should accept.
