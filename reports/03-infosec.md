# InfoSec / Cyber

**Score: 63/100** (was 60)

## What is being assessed
Threat model covering SQL injection, XSS, CSRF, path traversal, command injection, SSRF, credential stuffing, session hijacking, container escape, and privilege escalation. Each surface reviewed with paranoid posture, including non-alphanumeric input handling.

## Methodology
Read `internal/store/ticket.go`, `internal/server/api_tickets.go`, `internal/server/api.go`, `internal/server/chat_ws.go`, `internal/server/realtime.go`. Searched for `fmt.Sprintf.*SELECT`, `os.Open`, `exec.Command`, `http.Get`, websocket upgrades, X-Forwarded-For handling.

## Findings

### Passing checks
- **SQL injection**: All queries use `?` parameterized placeholders; no `fmt.Sprintf` in SQL strings (`internal/store/ticket.go` throughout)
- **XSS**: Project is a JSON REST API with no server-side template rendering; innerHTML in frontend always uses `escape()` sanitizer (`web/static/index.html`)
- **Path traversal**: No user-controlled file operations found
- **Command injection**: `exec.Command` at `internal/server/chat_ws.go:227` uses array args, marked `#nosec G204` with valid justification (args from server config, not user input)
- **SSRF**: HTTP client base URL comes from `config.ResolveLocation()`, not user input (`internal/client/client.go:34`)
- **WebSocket auth**: Token validated before upgrade; both `/api/ws` and `/api/chat/ws` require valid session (`internal/server/api_auth.go:20-45`)
- **Session management**: Opaque 32-byte tokens, constant-time comparison, 30-day expiry, purged on password reset
- **Password timing**: `subtle.ConstantTimeCompare` used (`internal/password/hash.go`)

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| WebSocket `liveHub.broadcast()` sends all events to all clients regardless of project membership | High | `internal/server/realtime.go:66-80` | Implement per-project broadcast groups; filter by `projectID` before send |
| Rate limiter IP extraction trusts `X-Forwarded-For` unconditionally | Medium | `internal/server/ratelimit.go:48-56` | Validate against a trusted-proxy allowlist; use rightmost non-trusted IP |
| CSP `script-src 'unsafe-inline'` weakens XSS defence | Low | `internal/server/server.go:150` | Replace with CSP nonces for any inline scripts |
| No CSRF tokens on state-changing endpoints (see Security report) | High | All `api_*.go` | Add CSRF middleware |

## Verdict
The project is clean against the classic injection trilogy (SQL, XSS, path traversal) and has no SSRF or command injection surface. The critical open issue is WebSocket event broadcast leaking cross-project metadata to all connected sessions — an information-disclosure flaw in multi-user deployments. Score improves slightly (+3) as SSRF and command injection are now confirmed safe.

## Changes since last assessment
- SSRF vector confirmed safe (baseURL from config, not user input) — addressed concern from v0.1.730
- WebSocket auth confirmed present
- Cross-project WebSocket broadcast gap carries forward unchanged

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| WebSocket cross-project leakage | High | Create per-project `liveHub` instances or filter in `broadcast()` by user's project list |
| X-Forwarded-For spoofing | Medium | Add `trustedProxies` config; strip untrusted forwarded headers |
| CSP unsafe-inline | Low | Use nonces for `<script>` blocks; tighten CSP to `script-src 'nonce-{n}'` |
