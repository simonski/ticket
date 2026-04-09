# DevOps

**Score: 81/100** (was 72)

## What is being assessed
Build pipeline completeness (test, lint, gosec, govulncheck), Docker multi-stage build quality (pinned images, non-root, HEALTHCHECK, build flags), docker-compose resource limits / health checks / network segmentation, CI Go version alignment with go.mod, publish job atomicity and race-condition analysis, release pipeline (cross-compilation, SBOM, Homebrew tap), and secrets management.

## Methodology
Reviewed `.github/workflows/makefile.yaml`, `Makefile`, `Dockerfile`, `compose.yaml`, `deploy/entrypoint.sh`, `cmd/ticket/VERSION`, and `go.mod`. Verified every quality gate present in the previous assessment's recommendations; checked new publish pipeline for correctness.

## Findings

### Passing checks
- Multi-stage Dockerfile: builder (`golang:1.26-alpine`) separate from runtime (`alpine:3.21`) — `Dockerfile:2,14`
- Alpine runtime pinned to `3.21` (not `latest`) — `Dockerfile:14`
- Non-root user: `adduser -D ticket` + `USER ticket` — `Dockerfile:19-21`
- `apk add --no-cache` prevents cache bloat — `Dockerfile:16`
- `HEALTHCHECK` present: `wget -qO- http://localhost:8080/api/healthz`, 30s interval, 5s timeout, 3 retries — `Dockerfile:28-29`
- Go version aligned: `go.mod` → `1.26.0`; CI uses `go-version-file: 'go.mod'`; `Dockerfile` → `golang:1.26-alpine` — `makefile.yaml:13-14`
- `go.sum` present; all dependencies pinned for reproducible builds
- Coverage thresholds **enforced in CI**: `make test-go-cover` (cmd 55%, libticket 65%, libtickethttp 75%, store 70%) — `makefile.yaml:22`
- `golangci-lint` runs in CI as part of `make setup` → `make build` chain — `makefile.yaml:25`
- `gosec ./...` explicit CI step after build — `makefile.yaml:28-29`
- `govulncheck ./...` explicit CI step after gosec — `makefile.yaml:31-32`
- `compose.yaml` has memory limit 512m, CPU limit 1.0, memory reservation 64m — `compose.yaml:11-16`
- `compose.yaml` has healthcheck (wget `/api/healthz`, 30s/5s/10s/3) — `compose.yaml:17-22`
- `restart: unless-stopped` in compose — `compose.yaml:10`
- Publish job gated on `needs: build` and `github.ref == refs/heads/main` — `makefile.yaml:36-37`
- `permissions: contents: write, packages: write` scoped only to publish job — `makefile.yaml:40-41`
- `docker/login-action@v3` runs before `make docker-push` — correct login order — `makefile.yaml:58-63`
- Cross-compilation: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 — `Makefile`
- SBOM generated via `cyclonedx-gomod` into `dist/sbom.cdx.json` — `Makefile:release-sbom`
- Homebrew formula auto-generated from template with per-platform SHA256 — `Makefile:release-formula`
- `TAP_TOKEN` secret used for Homebrew tap push; falls back to `GITHUB_TOKEN` — `makefile.yaml:44-47`
- Version bump committed with `[skip ci]` to prevent CI loop — `makefile.yaml:55`
- `fetch-depth: 0` on publish checkout ensures full history for `gh release` — `makefile.yaml:48`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| No named networks in `compose.yaml` — default bridge only | Low | `compose.yaml` | Define a named network (`ticket-net`) for future multi-service segmentation |
| No CPU reservation in `compose.yaml` (only limit) | Low | `compose.yaml:13` | Add `reservations.cpus: "0.25"` alongside memory reservation |
| Builder stage has no `-ldflags="-s -w"` or `CGO_ENABLED=0` | Low | `Dockerfile:12` | Add `RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tk ./cmd/ticket` for a smaller, fully static binary |
| No image digest pinning on builder base image | Low | `Dockerfile:2` | Pin `golang:1.26-alpine@sha256:...` to eliminate supply-chain substitution risk |
| Publish job: if `TAP_TOKEN` is unset, tap push uses `GITHUB_TOKEN` which lacks cross-repo write access — tap update will silently fail | Medium | `makefile.yaml:44` | Add a CI check: `if [ -z "$TAP_TOKEN" ]; then echo "::error::TAP_TOKEN not set"; exit 1; fi` |
| `make docker-push` calls `make docker-build` which calls `make bump-version` — a second version bump can occur if invoked locally | Low | `Makefile:docker-push` | Add a guard so `bump-version` is a no-op if already called in the same make session, or decouple `docker-build` from `bump-version` |

## Verdict
Substantial improvement: all three High-severity gaps from the previous assessment (HEALTHCHECK, resource limits, compose healthcheck) are resolved, and all three Medium gaps (coverage in CI, govulncheck, golangci-lint) are also closed. The publish pipeline is well-structured — gated, permissioned correctly, and includes SBOM and Homebrew tap. Remaining gaps are Low severity, with one Medium risk around TAP_TOKEN fallback silently failing. The project is CI/CD-ready for production deployment.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| `gosec ./...` added as explicit CI step | Closes previous Medium finding |
| New `publish` job: version bump → Docker push to GHCR → GitHub Release → Homebrew tap | Closes previous gap on release automation |
| `HEALTHCHECK` confirmed present in Dockerfile | Closes previous High finding |
| `compose.yaml` now has `deploy.resources.limits` (memory 512m, cpus 1.0) | Closes previous High finding |
| `compose.yaml` now has `healthcheck` section | Closes previous High finding |
| CI now uses `make test-go-cover` (coverage thresholds enforced) | Closes previous Medium finding |
| `govulncheck ./...` now explicit CI step | Closes previous Medium finding |
| `golangci-lint` now in CI via `make setup` | Closes previous Medium finding |
| `ReadHeaderTimeout: 30s` on HTTP server | Minor SRE hardening visible in server.go |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Guard against silent TAP_TOKEN fallback | Medium | Fail fast in publish job if `TAP_TOKEN` secret is absent |
| Add `-ldflags="-s -w"` and `CGO_ENABLED=0` to Dockerfile build | Low | Smaller, fully static binary; ~20-30% size reduction |
| Pin builder base image to digest | Low | Eliminate supply-chain substitution risk on `golang:1.26-alpine` |
| Add named network to `compose.yaml` | Low | Enables future multi-service segmentation |
| Add CPU reservation to `compose.yaml` | Low | Alongside existing memory reservation for predictable scheduling |
