# DevOps

**Score: 88/100** (was 87)

## What is being assessed
Build, test, release, and deployment readiness: version alignment, supply-chain pinning, CI scope, container/runtime hardening, and whether the deployment defaults match the project’s documented expectations.

## Methodology
Reviewed `Dockerfile`, `.github/workflows/makefile.yaml`, `Makefile`, `.dockerignore`, and both compose files, then refreshed the previous DevOps report against the current repository state.

## Findings

### Passing checks
- **Supply-chain pinning is much stronger now** — the Docker images are pinned to digests and the main GitHub Actions references are pinned to SHAs (`Dockerfile:2`, `Dockerfile:15`, `.github/workflows/makefile.yaml:15-18`, `.github/workflows/makefile.yaml:43`, `.github/workflows/makefile.yaml:56-87`).
- **CI now covers both Go and browser paths** — the workflow runs `make test-go-cover` and a dedicated Playwright job before publish (`.github/workflows/makefile.yaml:25-45`).
- **The deployment/runtime posture is improved around secrets and build context** — `.dockerignore` excludes `.env*`, `.ticket/`, and local databases (`.dockerignore:22-29`).
- **The runtime image stays minimal and non-root** — the container runs as `ticket` and includes a healthcheck against `/api/healthz` (`Dockerfile:19-33`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The production/deploy compose file still gives the main `ticket` service no explicit CPU/memory limits | Medium | `deploy/compose.yaml:1-13` | Add resource limits for the `ticket` service so the shipped deploy profile matches the hardened dev compose profile. |
| The deploy compose file still does not define an explicit application network boundary | Low | `deploy/compose.yaml:1-31` | Add a named network and attach services explicitly to reduce accidental service coupling. |

## Verdict
DevOps improved because the biggest supply-chain and CI gaps from older reports are now closed: pinned Docker images, pinned actions, and Playwright in CI are all present. The remaining work is mostly about tightening the deploy compose defaults rather than repairing the build pipeline itself.

## Changes since last assessment
- Reclassified Docker image and GitHub Action pinning as complete (`Dockerfile:2`, `Dockerfile:15`, `.github/workflows/makefile.yaml:15-18`, `43`, `56-87`).
- Reclassified Playwright-in-CI as complete (`.github/workflows/makefile.yaml:40-45`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Missing production service limits | Medium | Add CPU/memory limits for the main `ticket` service in `deploy/compose.yaml`. |
| No explicit deploy network boundary | Low | Define a named network and attach deployment services explicitly. |
