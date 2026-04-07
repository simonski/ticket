# DevOps

**Score: 72/100**

## What is being assessed
Build pipeline quality (Makefile targets, versioning), Docker configuration (multi-stage, Alpine pinning, non-root user, health checks), docker-compose resource limits and health checks, CI/CD pipeline (Go version alignment, linting, coverage enforcement, vulnerability scanning), secrets management, and release pipeline.

## Methodology
Reviewed `Makefile`, `Dockerfile`, `compose.yaml`, `.github/workflows/makefile.yaml`, `go.mod`, `go.sum`, `deploy/README.md`, and `.gitignore`. Checked for pinned versions, non-root users, resource limits, and secrets handling.

## Findings

### Passing checks
- Multi-stage Dockerfile: builder (`golang:1.26-alpine`) separate from runtime (`alpine:3.21`) — `Dockerfile:2,15`
- Alpine version **pinned** to `3.21` (not `latest`) — `Dockerfile:15`
- Non-root user configured: `adduser -D -h /home/ticket ticket` + `USER ticket` — `Dockerfile:20-22`
- `COPY` used exclusively (not `ADD`) — `Dockerfile:7,11,25,26`
- `apk add --no-cache` prevents package cache bloat — `Dockerfile:17`
- Go version aligned: `go.mod` `1.26.0`, `Dockerfile` `golang:1.26-alpine`, CI `ubuntu-latest` — all match
- All Go dependencies pinned with exact versions in `go.mod`/`go.sum`
- `go.sum` present for reproducible builds
- `.gitignore` excludes `.ticket/credentials.json` and `config.json` — no secrets in repo
- Patch version auto-incremented on `make build`, embedded via `//go:embed VERSION`
- Full cross-platform release pipeline: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
- Homebrew formula auto-generated on release (`homebrew/ticket.rb.tmpl`)
- `gosec` security scanning runs in CI (`.github/workflows/makefile.yaml:29-32`)
- Per-package coverage thresholds enforced locally: cmd 55%, libticket 65%, libtickethttp 75%, store 70%

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No `HEALTHCHECK` directive in Dockerfile | High | `Dockerfile` | Add `HEALTHCHECK --interval=30s CMD curl -f http://localhost:8080/api/healthz \|\| exit 1` |
| `compose.yaml` has no resource limits (`mem_limit`, `cpus`) | High | `compose.yaml` | Add `deploy.resources.limits` with `memory: 512m` and `cpus: "1.0"` |
| `compose.yaml` has no `healthcheck` section | High | `compose.yaml` | Add healthcheck pointing to `/api/healthz` |
| Coverage thresholds not enforced in CI | Medium | `.github/workflows/makefile.yaml` | Change `make test` to `make test-go-cover` in CI workflow |
| `govulncheck` installed but never invoked in CI | Medium | `Makefile:52`, CI workflow | Add `govulncheck ./...` step to CI workflow |
| No `golangci-lint` in CI | Medium | CI workflow | Add `golangci-lint run ./...` step |
| No network segmentation in `compose.yaml` | Low | `compose.yaml` | Define named networks if multi-service deployment needed |
| Release process not linked from main README | Low | `README.md` | Add link to `deploy/README.md` |

## Verdict
Strong DevOps foundation: multi-stage Docker build, pinned versions, non-root container user, automated release pipeline, and gosec in CI. The gaps are operational — missing Docker healthcheck, no resource limits in compose, and coverage thresholds not enforced in CI (only locally). These should be resolved before production deployment.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add `HEALTHCHECK` to Dockerfile | High | Enables container orchestrators to detect unhealthy containers |
| Add resource limits to `compose.yaml` | High | Prevent container from consuming unbounded memory/CPU |
| Add `healthcheck` to `compose.yaml` | High | Enables `depends_on: condition: service_healthy` |
| Enforce coverage in CI | Medium | Replace `make test` with `make test-go-cover` in workflow |
| Run `govulncheck` in CI | Medium | Add `govulncheck ./...` as a CI step |
| Add `golangci-lint` to CI | Medium | Add step with `.golangci.yml` config |
