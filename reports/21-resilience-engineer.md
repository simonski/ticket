# Resilience Engineer

**Score: 66/100** (was 72)

## Mission
Ensure the system degrades safely instead of collapsing messily.

## Review objective
Review timeouts, shutdown, backpressure, health, retries, and failure isolation.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/chat_ws.go`
- `internal/server/api_system.go`
- `internal/store/schema_version.go`
- `docs/RUNBOOKS.md`

## Findings

### Passing checks
- HTTP server has read/header/idle timeouts and preserves WebSocket behavior (`internal/server/server.go:41-49`).
- Request timeout middleware exempts long-lived websocket paths deliberately (`internal/server/server.go:227-240`).
- Chat enforces session duration and capacity limits (`internal/server/chat_ws.go:187-200`).
- Runbooks cover common failure scenarios (`docs/RUNBOOKS.md:8-19`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Readiness does not expose degraded chat capacity. | Medium | Operators cannot automatically route around degraded chat runtime. | `internal/server/api_system.go:19-31`, `internal/server/chat_ws.go:187-200` | Add readiness signal for chat capacity and recent errors. |
| Deployment starts with unsafe credentials. | Critical | Resilience to compromise is poor; incident begins on first boot. | `deploy/entrypoint.sh:12-16` | Remove password fallback and document recovery if bootstrap secret is lost. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| sre | Readiness and incident signals overlap. | `/api/readyz` or health payload. |
| security-engineer | Default password is a resilience/security incident starter. | Bootstrap redesign. |

## Verdict
Runtime resilience has sensible timeouts and capacity checks. Production resilience is undermined by insecure deployment defaults and shallow readiness.

## Changes since last assessment
- Runbooks and SLO docs are present; readiness remains limited.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Shallow readiness | Medium | Add subsystem readiness. | resilience-engineer |
