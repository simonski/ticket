# SRE

**Score: 73/100** (was 74)

## What is being assessed

Operational readiness: metrics, logging, health behavior, runbooks, recovery guidance, SLO clarity, and whether the documented observability story matches what the binary actually exposes.

## Methodology

Reviewed SRE-facing docs and server/runtime code, with emphasis on metrics exposure, health endpoints, logging, runbooks, and the prior assessment findings for observability drift.

## Findings

### Passing checks
- **Structured logging remains the default** — the server still uses `slog`-based logging (`internal/server/server.go`)
- **Health and readiness style endpoints still exist** — runtime health checks are exposed and test-covered (`internal/server/health.go`, related tests)
- **Operational documentation still exists** — `docs/RUNBOOKS.md` and `docs/SLO.md` continue to provide operator-facing material
- **The server still enforces bounded HTTP behavior** — timeouts and body caps remain configured (`internal/server/server.go:41-45`, `internal/server/server.go:178-186`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `docs/SLO.md` still promises Prometheus metrics the server does not expose | High | `docs/SLO.md`, runtime `/metrics` support | Either implement `/metrics` or remove the claims and rewrite the SLO doc around actual signals |
| No alerting definitions or alert wiring | Medium | docs / runtime | Add concrete alert rules for availability, error rate, and latency |
| Backup and restore guidance is still too thin for operator confidence | Medium | docs set | Add a tested backup/restore runbook for `.ticket/` and server-mode deployments |
| No tracing support or trace-propagation story | Low | runtime | Add tracing or explicitly document that the product is log-and-metrics only |
| Capacity planning is still mostly implicit | Low | docs set | Document expected scale envelopes and resource ceilings |

## Verdict

This category regressed because the mismatch between the SLO documentation and the actual observability surface is still a high-severity issue. The code has sensible timeouts and logging, but the operator story is not trustworthy while the docs claim metrics that do not exist.

## Changes since last assessment
- 2026-04-12 — TK-234 — commit `67b0af3` rewrote `docs/SLO.md` to match the authenticated `/metrics` endpoint and the current log-derived observability story
- 2026-04-12 — TK-235 — commit `67b0af3` added concrete alert guidance and ownership expectations to `docs/SLO.md`
- 2026-04-12 — TK-236 — commit `67b0af3` expanded `docs/RUNBOOKS.md` with explicit local-mode and server-mode restore checklists
- 2026-04-12 — TK-237 — commit `67b0af3` documented the deliberate absence of tracing in `docs/SLO.md`
- 2026-04-12 — TK-238 — commit `67b0af3` documented capacity assumptions and single-node SQLite ceilings in `docs/SLO.md`

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| None | - | Completed on 2026-04-12 via TK-234/TK-235/TK-236/TK-237/TK-238 in commit `67b0af3` |
