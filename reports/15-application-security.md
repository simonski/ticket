# Application Security

**Score: 80/100** (was 80)

## Mission
Think like an attacker and verify whether realistic entry points can be abused through injection, traversal, XSS, CSRF, or command execution.

## Review objective
Assess the main browser, API, WebSocket, and subprocess attack surfaces.

## Inputs reviewed
- `internal/server/server.go`
- `internal/server/realtime.go`
- `internal/server/analyse.go`
- `internal/server/chat_ws.go`
- `web/static/index.html`

## Findings

### Passing checks
- The CSRF model is explicit and compares tokens in constant time for cookie-authenticated mutation requests (`internal/server/server.go:454-549`).
- WebSocket upgrades validate `Origin` against `Host` before accepting the connection (`internal/server/realtime.go:153-177`).
- The SPA escapes HTML-significant characters centrally before many template insertions (`web/static/index.html:5957-5964`, `web/static/index.html:7113-7119`).
- Analyse/chat command resolution uses argument splitting plus `exec.Command`, not shell interpolation (`internal/server/analyse.go:38-58`, `internal/server/analyse.go:192-198`, `internal/server/chat_ws.go:324-335`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The server-side chat/analyse execution model still trusts operator-provided commands and inherited environment completely. | Medium | A misconfigured deployment can turn the AI/operator path into a much wider execution surface than intended. | `internal/server/analyse.go:60-79`, `internal/server/analyse.go:139-176`, `internal/server/chat_ws.go:227-237` | Treat subprocess config as privileged-only, filter env, and document the threat model in operator docs. |
| A very large number of DOM fragments still rely on one escape helper plus `innerHTML`. | Low | The current code is mostly safe, but the surface is fragile: one missed escape in future edits could introduce XSS. | `web/static/index.html:3722-3737`, `web/static/index.html:3955-3972`, `web/static/index.html:5957-5964`, `web/static/index.html:7113-7119` | Prefer DOM construction APIs for new work and shrink the raw `innerHTML` footprint over time. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| security-engineer | Proxy/process controls need to become concrete hardening steps. | Child-process hardening checklist. |
| frontend-engineer | Reducing raw `innerHTML` use is a client-implementation task. | DOM-construction migration targets. |
| release-manager | Operator-trusted command execution belongs in release risk framing. | Residual-risk statement for AI/chat features. |

## Verdict
The repo has no obvious high-probability injection or CSRF catastrophe in the current paths I reviewed. The bigger risk is fragility: trusted operator command execution and a large `innerHTML` surface are both manageable today, but they deserve tighter guardrails before the next wave of feature work.

## Changes since last assessment
- The practical attack surface is narrower than older shell-execution fears implied because the current code uses `exec.Command` with parsed args rather than shell evaluation (`internal/server/analyse.go:45-58`, `internal/server/chat_ws.go:324-335`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Trusted subprocess surface | Medium | Filter env and document operator-only command trust. | security-engineer |
| Large `innerHTML` footprint | Low | Prefer DOM APIs in new work and reduce template-string rendering over time. | frontend-engineer |
