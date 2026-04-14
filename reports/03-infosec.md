# InfoSec / Cyber

**Score: 80/100** (was 78)

## What is being assessed
Threat-model coverage across the CLI, API, SPA, database, WebSocket, and container surfaces: SQL injection, XSS, CSRF, command injection, session abuse, privilege escalation, and blast-radius containment.

## Methodology
Reviewed the same high-risk attack surfaces as the prior InfoSec report, but refreshed the evidence against current code paths for input handling, rendering, cookie/session controls, and command execution.

## Findings

### Passing checks
- **SQL-injection posture remains strong** — write/read paths continue to rely on placeholders rather than user-built SQL, including history storage and snapshot import (`internal/store/activity.go:34-57`, `internal/store/snapshot.go:167-183`).
- **Stored-XSS defenses are still visible at the render edge** — the central `escape()` helper encodes all five HTML-significant characters and is used in `innerHTML` templating for agent rows/cards (`web/static/index.html:3271-3285`, `web/static/index.html:5803-5809`).
- **Session and browser request forgery defenses layer together cleanly** — secure cookie names are defined centrally and the security middleware adds CSP/HSTS/body caps on top (`internal/server/api_helpers.go:18-23`, `internal/server/server.go:253-294`).
- **The CLI command-injection surface is narrower than before** — only allow-listed LLM binary names are accepted and path-like values are rejected (`cmd/tk/cmd_agent.go:769-806`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The server still exposes two direct process-launch surfaces controlled by environment variables | Medium | `internal/server/chat_ws.go:227-233`, `internal/server/analyse.go:139-146`, `internal/server/analyse.go:192-199` | Enforce a server-side executable allow-list and reject dangerous command forms before the process can start. |
| The deployment topology remains mostly flat, so a compromised runtime still has a broad local blast radius | Low | `deploy/compose.yaml:1-31` | Introduce explicit network boundaries and service-level runtime limits in the deployment compose file. |

## Verdict
The current cyber posture is sturdier than the baseline reports suggested because the highest-value browser and CLI controls are now concrete code, not TODOs. The remaining paranoia case is server-side command execution, which is intentional functionality but still deserves stricter containment.

## Changes since last assessment
- Reclassified the old CLI `--llm` concern as fixed because the command now validates against an allow-list (`cmd/tk/cmd_agent.go:769-806`).
- Verified the browser surface now benefits from nonce-based CSP and secure cookie naming (`internal/server/api_helpers.go:18-23`, `internal/server/server.go:253-263`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Trusted-env command execution | Medium | Apply allow-listing and startup validation to `TICKET_CHAT_CMD` and `TICKET_ANALYSE_CMD`. |
| Flat deployment blast radius | Low | Add explicit runtime isolation in `deploy/compose.yaml` so a container compromise has less reach by default. |
