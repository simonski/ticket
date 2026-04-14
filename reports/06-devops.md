# DevOps

**Score: 87/100** (was 87)

## What is being assessed

Build tooling, containerisation, compose configuration, CI/CD pipeline, Go version alignment, linting, vulnerability scanning, coverage enforcement, secrets handling, version management, and release pipeline.

## Methodology

Read `Makefile`, `Dockerfile`, `compose.yaml`, `deploy/compose.yaml`, `.github/workflows/makefile.yaml`, `.golangci.yml`, `go.mod`, `.gitignore`, `.dockerignore`, `deploy/entrypoint.sh`, `cmd/tk/VERSION`, `homebrew/ticket.rb.tmpl`.

## Findings

### Passing checks
- All required Makefile targets present: `build`, `test`, `lint`, `setup`, `docker-build`, `docker-up`, `docker-down`, `release`, `clean` -- `Makefile`
- Multi-stage Dockerfile: `golang:1.26-alpine` builder, `alpine:3.21` runtime -- `Dockerfile:3,17`
- Non-root USER in Dockerfile -- `Dockerfile:22-24`
- Dockerfile HEALTHCHECK with sensible intervals -- `Dockerfile:32-33`
- Dev compose (`compose.yaml`) has resource limits (`memory: 512m`, `cpus: 1.0`), `cap_drop: ALL`, `pids_limit`, `security_opt: no-new-privileges` -- `compose.yaml:13-22`
- Dev compose healthcheck with `start_period` -- `compose.yaml:25-30`
- CI triggers on push/PR to `main` and `develop` -- `.github/workflows/makefile.yaml:3-7`
- Go version alignment: `go.mod`, Dockerfile, and `.golangci.yml` all use Go 1.26 -- `go.mod:3`, `Dockerfile:3`, `.golangci.yml:3`
- CI uses `go-version-file: 'go.mod'` with `cache: true` -- `makefile.yaml:18-19`
- `govulncheck` in CI -- `makefile.yaml:38`
- `gosec` in CI (both standalone step and via `make lint`) -- `makefile.yaml:33-36`
- Coverage thresholds enforced per-package in CI via `make test-go-cover` -- `makefile.yaml:26`
- Linting in CI: `make lint` runs both `golangci-lint` and `gosec` -- `makefile.yaml:31-32`, `Makefile:58-59`
- Secrets via `secrets.*` context, not hardcoded -- `makefile.yaml:53,84,92`
- `.gitignore` excludes credentials; `.dockerignore` well-scoped (excludes `.git`, `node_modules`, `*.db`, `.ticket/`, etc.)
- SBOM generation via `cyclonedx-gomod` in release pipeline -- `Makefile:167-169`
- Version management: `cmd/tk/VERSION` auto-bumped on build and in CI publish -- `Makefile:68-79`, `makefile.yaml:69-77`
- Full release pipeline: cross-compile, checksums, SBOM, Homebrew formula, GitHub release, tap push -- `Makefile:136-216`
- Publish job conditional on main push with correct permissions -- `makefile.yaml:41-47`
- Entrypoint script handles first-run DB init gracefully -- `deploy/entrypoint.sh`

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| GitHub Actions not pinned to SHA digests | Medium | `makefile.yaml:15,17,49,56,80` | Pin `actions/checkout`, `actions/setup-go`, `docker/login-action` to `@sha256:...` to prevent supply-chain attacks |
| Docker base images not pinned to digests | Medium | `Dockerfile:3,17` | Pin `golang:1.26-alpine` and `alpine:3.21` to `@sha256:...` for reproducible builds (TODO already noted in Dockerfile) |
| Deploy compose: watchtower has no resource limits | Medium | `deploy/compose.yaml:16-19` | Add `deploy.resources.limits` to the watchtower service |
| Deploy compose: watchtower mounts Docker socket without read-only | Medium | `deploy/compose.yaml:18` | Add `:ro` to the Docker socket volume mount |
| Deploy compose: no network segmentation | Low | `deploy/compose.yaml` | Add named network to isolate services |
| Deploy compose: ticket service has no resource limits | Low | `deploy/compose.yaml:2-13` | Add `deploy.resources.limits` matching dev compose |
| No guard for duplicate VERSION release | Low | `makefile.yaml:86-93` | Check if `gh release view v$VERSION` already exists before creating |
| `docker-push` re-tags from local name rather than building with GHCR name | Low | `Makefile:224-228` | Build directly with `--tag $(GHCR_IMAGE):$(VERSION)` |
| `.dockerignore` excludes all `*.md` including `cmd/tk/VERSION`-adjacent files but not `.env*` | Low | `.dockerignore:13-14` | Add `.env*` pattern to prevent accidental secret inclusion |
| No Playwright tests in CI | Low | `.github/workflows/makefile.yaml` | CI runs `test-go-cover` but not `test-playwright`; add a browser test job |

## Verdict

The DevOps posture remains strong in this pass. Go version alignment, CI linting, module caching, SBOM generation, coverage gates, and the release pipeline all remain intact, while the primary remaining gaps are still supply-chain hardening and tightening the deploy compose setup.

## Changes since last assessment
- No material DevOps regressions were found in this pass
- The earlier CI alignment and lint/cache improvements remain intact
- The same supply-chain and deploy-compose hardening work remains open

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| None | - | No new unresolved DevOps actions in this assessment pass. |
