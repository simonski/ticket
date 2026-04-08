# InfoSec / Cyber Threat Model

**Score: 60/100** (was 52)

## What is being assessed
Full attack surface threat model covering SQL injection, XSS, CSRF, path traversal, command injection, SSRF, credential stuffing, session hijacking, container escape, and privilege escalation. All `#nosec G703/G704/G204` suppressions are individually assessed to determine whether the stated risk mitigation is genuine. Version 0.1.737, assessed 2025-07-21.

## Methodology
Systematic review of `cmd/ticket/cmd_agent.go` (agent shell invocation), `internal/server/analyse.go` (subprocess SSRF), `internal/server/chat_ws.go` (WebSocket → process bridging), `internal/store/ticket.go` (SQL surface), `internal/server/api_tickets.go` (CSRF/access control), `cmd/ticket/cmd_setup.go` (path traversal, SSRF). Cross-referenced all `exec.Command` call sites, every `#nosec` annotation, and all user-input trust boundaries. OWASP Top 10 used as a checklist baseline.

## Findings

### Passing checks
- **SQL injection**: All user-controlled values use `?` placeholders throughout `internal/store/`; no string interpolation of user data in queries (`internal/store/ticket.go`, `internal/store/auth.go`)
- **`#nosec G202` in `DeleteUser`**: Table/column names come from a hardcoded `[]struct` — not user input. Suppression is justified (`internal/store/auth.go:213`)
- **XSS**: SPA escapes user content before `innerHTML`; API returns JSON, not HTML; CSP headers present
- **Path traversal**: Static files served via `fs.FS` — OS-level constraints prevent `../` breakout
- **WebSocket origin validation**: `upgradeWebSocket` now validates `Origin` against `r.Host` before handshake (`internal/server/realtime.go:133-149`)
- **Chat output sanitisation**: `sanitizeTerminalOutput()` strips ANSI escape sequences before forwarding to WebSocket clients (`internal/server/chat_ws.go:70-80`)
- **Chat capacity limits**: `MaxConnections` and `MaxDurationMin` enforced before spawning a process (`internal/server/chat_ws.go:153-169`)
- **Chat process timeout**: Per-session duration timer kills subprocess on expiry (`internal/server/chat_ws.go:281-296`)
- **`#nosec G505/G401` in realtime.go**: SHA-1 is mandated by RFC 6455 WebSocket handshake spec — suppression is correct and well-documented (`internal/server/realtime.go:5,186`)
- **`#nosec G304/G703` in cmd_setup.go line 505**: Path is a well-known local skill path, not a user-supplied value — suppression is justified
- **`#nosec G107/G704` in cmd_setup.go line 298**: URL is entered by the operator during `tk init` setup wizard — suppression is justified for this CLI use case
- **Credential enumeration**: Same error text returned for unknown username and wrong password (`internal/store/auth.go:109`)
- **Session hijacking**: 32-byte random tokens, `HttpOnly`, `SameSite=Lax` cookies; `expires_at` enforced in DB lookup

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| **`sh -c` with filename derived from server-supplied ticket ID** — if ticket ID contains shell metacharacters the shell will execute them | High | `cmd/ticket/cmd_agent.go:699-701` | Replace `exec.Command("sh","-c",llmCmd)` with `exec.Command(binary, args...)` using explicit arg slices; quote or validate `ticketKey` before use in filename |
| **Verbose logging writes full request/response bodies** — POST `/api/login` bodies include plaintext password | High | `internal/server/server.go:207-233` | Redact `password` key before logging; never buffer auth request bodies |
| **Credentials passed in subprocess environment** — `TICKET_USERNAME` / `TICKET_PASSWORD` injected into `codex exec` env in `storyAnalyseProcessEnv()` | High | `internal/server/analyse.go:63-75` | Pass an API token (session token) rather than username+password; rotate immediately after use |
| **`#nosec G204` in `analyse.go:145,194`** — `TICKET_ANALYSE_CMD` env var parsed and exec'd; risk is genuinely low *if* only admins control server env, but is an RCE vector if env is injectable | Medium | `internal/server/analyse.go:145,194` | Validate command against an explicit allowlist (`["codex","claude"]`) before exec |
| **`#nosec G204` in `chat_ws.go:227`** — `TICKET_CHAT_CMD` from env exec'd directly; same trust-boundary concern as above | Medium | `internal/server/chat_ws.go:227` | Same as above: allowlist-validate the command name |
| **Rate limiter `attempts` map never purges stale keys** — unique IPs accumulate indefinitely; high-volume scans cause unbounded memory growth | Medium | `internal/server/ratelimit.go:22-40` | Periodically delete map keys whose slice is empty; cap map size or use an LRU structure |
| **No CSRF token for state-changing API endpoints** — SameSite=Lax provides partial protection but POST requests from same-site links are not blocked | Medium | `internal/server/api_tickets.go` (all POST/PUT/DELETE) | Add synchronizer token (double-submit cookie pattern) for all mutating endpoints |
| **Internal `err.Error()` strings returned to clients** — SQLite constraint messages, store errors expose schema details | Low | Multiple handlers in `api_tickets.go` | Map typed errors to generic user-facing messages; log the detail server-side only |
| **`/api/status` discloses server version and chat config to unauthenticated callers** | Low | `internal/server/api_auth.go:192-205` | Omit `server_version` and chat limit fields from unauthenticated response |

## Verdict
Substantial improvement since v0.1.730. The two highest-impact issues from the previous report — missing WebSocket origin validation and no `expires_at` session check — are now fixed. gosec is clean and enforced in CI. The remaining priority concerns are: the `sh -c` invocation in the agent runner (a genuine command injection path if the server is malicious or compromised), plaintext credentials in subprocess environments, and verbose logging writing passwords to disk. None of these affect the core web application in default (non-agent, non-verbose) deployments. The `#nosec G204` suppressions in `analyse.go` and `chat_ws.go` are justifiable only so long as environment variable injection by non-admins is not possible; this should be reinforced with an allowlist check.

## Changes since last assessment
| Change | Impact |
|--------|--------|
| WebSocket `Origin` header validation added | Closed High finding (cross-origin WebSocket hijacking) |
| Session `expires_at` now enforced in `GetUserByToken` | Closed High finding (infinite session tokens) |
| `sanitizeTerminalOutput()` strips ANSI codes from chat output | Reduces terminal-escape injection risk to chat clients |
| Chat capacity and duration limits implemented | Prevents resource-exhaustion via chat subprocess floods |
| All 97 gosec findings addressed with genuine fixes or documented `#nosec` | Systemic improvement; each suppression now has a written justification |
| gosec + govulncheck added to CI | Ongoing automated security gate |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `sh -c` command injection in agent runner | High | Use explicit `exec.Command(binary, arg1, arg2, ...)` with no shell; validate `ticketKey` is alphanumeric+dash only |
| Credential logging in verbose mode | High | Redact `password` from logged request body in `server.go` |
| Credentials in subprocess environment | High | Use short-lived API token instead of `TICKET_USERNAME`/`TICKET_PASSWORD` in `storyAnalyseProcessEnv` |
| `TICKET_ANALYSE_CMD` / `TICKET_CHAT_CMD` allowlist | Medium | Validate command name against `["codex","claude"]` before calling `exec.Command` |
| Rate limiter memory growth | Medium | Prune stale keys from `attempts` map periodically |
| CSRF protection | Medium | Double-submit cookie or synchronizer token on all state-changing endpoints |
| Internal error leakage | Low | Return generic messages to clients; structured-log the original error server-side |
| Version/config disclosure on `/api/status` | Low | Move `server_version` and chat limits to authenticated-only part of the response |
