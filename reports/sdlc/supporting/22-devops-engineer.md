# DevOps Engineer

**Score: 62/100** (was 72)

## Mission
Protect repeatable builds, safe deployment, and correct runtime packaging.

## Review objective
Review CI/CD, Dockerfiles, compose, env handling, secrets, and release automation.

## Inputs reviewed
- `.github/workflows/makefile.yaml`
- `Makefile`
- `Dockerfile`
- `deploy/compose.yaml`
- `deploy/entrypoint.sh`
- `deploy/README.md`

## Findings

### Passing checks
- CI runs setup, OpenAPI validation, coverage, build, lint, gosec, govulncheck, and Playwright (`.github/workflows/makefile.yaml:22-48`).
- Dockerfile pins base images by digest and uses non-root user (`Dockerfile:1-44`).
- Compose drops capabilities and enables no-new-privileges for the ticket container (`deploy/compose.yaml:15-24`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Compose hard-codes admin password and mutable latest image. | Critical | Production deployments are both insecure and non-reproducible. | `deploy/compose.yaml:3-10` | Require `.env`/secret input and versioned image tags. |
| Watchtower auto-updates every 30 seconds by default. | Medium | Production can change continuously without review or rollback staging. | `deploy/compose.yaml:27-33` | Move watchtower to an opt-in demo profile or document operational risk. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| supply-chain | Image pinning and provenance overlap. | Pin/sign strategy. |
| sre | Auto-update behavior affects operations. | Rollback and alerting plan. |

## Verdict
Build automation is strong, but deployment defaults are not production-safe. DevOps score falls because the compose path is the most direct operator path.

## Changes since last assessment
- Container hardening remains good; unsafe compose defaults remain open.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Insecure compose defaults | Critical | Remove default password and mutable production tag. | devops-engineer |
