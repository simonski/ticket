# DevOps

**Score: 82/100** (was 76)

## What is being assessed

Build tooling, containerisation, compose configuration, CI/CD pipeline, Go version alignment, linting, vulnerability scanning, coverage enforcement, secrets handling, and version management.

## Methodology

Read `Makefile`, `Dockerfile`, `compose.yaml`, `.github/workflows/makefile.yaml`, `.golangci.yml`, `go.mod`, `.gitignore`, `.dockerignore`, `deploy/entrypoint.sh`, `cmd/tk/VERSION`.

## Findings

### Passing checks
- All required Makefile targets present: `build`, `test`, `lint`, `setup`, `docker-build`, `docker-up`, `docker-down`
- Multi-stage Dockerfile: `golang:1.26-alpine` builder, `alpine:3.21` runtime — `Dockerfile:2,15`
- Non-root USER — `Dockerfile:9-11`
- Dockerfile HEALTHCHECK — `Dockerfile:22-24`
- Compose resource limits: `memory: 512m`, `cpus: 1.0` — `compose.yaml:13-18`
- Compose healthcheck mirrors Dockerfile — `compose.yaml:19-24`
- CI/CD pipeline triggers on push/PR to `main` and `develop` — `.github/workflows/makefile.yaml`
- Go version alignment: both go.mod and Dockerfile use Go 1.26
- `govulncheck` in CI — `makefile.yaml:30`
- `gosec` in CI — `makefile.yaml:28`
- Coverage thresholds enforced per-package in CI — `makefile.yaml:25`
- Secrets via `secrets.*` not hardcoded — `makefile.yaml:49,87`
- `.gitignore` covers secrets; `.dockerignore` present and well-scoped
- SBOM generation via `cyclonedx-gomod` in release pipeline
- Version management: `cmd/tk/VERSION` auto-bumped

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `.golangci.yml` `go: "1.23"` is stale — should be `"1.26"` | Medium | `.golangci.yml:3` | Update to `"1.26"` |
| `make lint` not invoked in CI pipeline | Medium | `.github/workflows/makefile.yaml` | Add `make lint` step |
| GitHub Actions not pinned to SHA digests | Medium | `makefile.yaml:15,17,45,52,75` | Pin to `@sha256:...` |
| Single service, no network segmentation in compose | Low | `compose.yaml` | Add named network |
| Docker base images not pinned to digests | Low | `Dockerfile:2,15` | Add `@sha256:...` |
| No Go module/build cache in CI | Low | `makefile.yaml` | Enable `cache: true` on `setup-go` |
| `docker-push` re-tags from local rather than GHCR name | Low | `Makefile:225-228` | Use GHCR name throughout |
| No guard for duplicate VERSION release | Info | `makefile.yaml:40-88` | Check if release already exists |

## Verdict

Improved from 76 to 82. Dockerfile, compose, release pipeline, and security scanning are solid. Top items: add `make lint` to CI, fix stale `.golangci.yml` Go version, pin Actions to SHA digests.

## Changes since last assessment
- Docker image tag fixed — `Makefile:222`
- `gosec` + `govulncheck` added to CI
- SBOM generation integrated in release pipeline
- `.dockerignore` added
- Compose resource limits added

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add `make lint` to CI | Medium | `.github/workflows/makefile.yaml` |
| Update `.golangci.yml` Go version | Medium | `.golangci.yml:3` |
| Pin GitHub Actions to SHA | Medium | `makefile.yaml` |
| Enable Go cache in CI | Low | `setup-go` `cache: true` |
| Pin Docker base images | Low | `Dockerfile:2,15` |
