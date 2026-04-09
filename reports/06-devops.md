# DevOps

**Score: 76/100** (was 81)

## What is being assessed
Build pipeline (Makefile targets, binary naming), Docker quality (multi-stage, Alpine version, non-root user, health check), Compose (resource limits, health checks, network segmentation), CI/CD (Go version alignment, linting, govulncheck, coverage thresholds), release pipeline (cross-platform, checksums), and secrets management.

## Methodology
Read `Makefile`, `Dockerfile`, `compose.yaml`, `.github/workflows/makefile.yaml`, `homebrew/ticket.rb.tmpl`. Searched for old `ticket` binary references, checked CI pipeline steps.

## Findings

### Passing checks
- Binary correctly builds to `./bin/tk` (`Makefile:47`: `go build -o ./bin/tk ./cmd/ticket`)
- Release tarballs named `tk_VERSION_*` for all 4 platforms (`Makefile:136-156`)
- Homebrew formula updated: URLs use `tk_VERSION_*`, `bin.install "tk"`, test calls `tk version` (`homebrew/ticket.rb.tmpl:11,16,23,28,34,38`)
- Dockerfile: multi-stage build, Alpine 3.21 pinned, non-root user `ticket`, HEALTHCHECK configured (`Dockerfile:2,15,20-22,30-31`)
- compose.yaml: resource limits set (`memory: 512m`, `cpus: "1.0"`), health check configured (`compose.yaml:12-24`)
- CI: Go version from `go.mod` (`go-version-file: 'go.mod'`), `govulncheck` step present (`.github/workflows/makefile.yaml:19,33-34`)
- Coverage thresholds enforced per package in `Makefile:100-117` (cmd 55%, libticket 65%, libtickethttp 75%, store 70%, config 70%)
- SHA256 checksums generated for release artifacts (`Makefile:158-164`)
- SBOM via CycloneDX in release pipeline (`Makefile:166-169`)
- Automated release on tag push (`.github/workflows/makefile.yaml:84-88`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `cmd/tk-test/main.go` defaults to `./bin/ticket`, not `./bin/tk` | High | `cmd/tk-test/main.go:37,42,49` | Change default from `bin/ticket` to `bin/tk`; update usage strings |
| `golangci-lint` not run in CI pipeline | Medium | `.github/workflows/makefile.yaml` | Add `make lint` step between govulncheck and test |
| Docker image tagged as `ticket:` not `tk:` | Medium | `Makefile:222,225-228` | Rename docker build/push targets to use `tk:` image tag |

## Verdict
The binary rename from `ticket` to `tk` is complete in all the primary surfaces (Makefile, Dockerfile, Homebrew, docs). However, `cmd/tk-test/main.go` was missed — its default flag value and usage strings still reference `./bin/ticket`, breaking executable documentation tests unless `-ticket ./bin/tk` is passed explicitly. The score regresses from 81 to 76 for this issue plus the missing golangci-lint CI step.

## Changes since last assessment
- Binary rename complete: `Makefile`, `Dockerfile`, `deploy/entrypoint.sh`, `homebrew/ticket.rb.tmpl` all updated (commit `c2c1353`)
- All documentation updated to reference `tk` binary
- **NEW ISSUE FOUND**: `cmd/tk-test/main.go` still defaults to `./bin/ticket`
- CI pipeline unchanged since v0.1.737

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Fix `tk-test` default path | High | `bin = "bin/tk"` at `cmd/tk-test/main.go:49`; update usage strings on lines 37, 42 |
| Add golangci-lint to CI | Medium | Add `- run: make lint` step after `govulncheck` in `makefile.yaml` |
| Rename docker image tag | Medium | Change `ticket:` to `tk:` in `Makefile:222,225-228` docker targets |
