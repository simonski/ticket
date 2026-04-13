# InfoSec / Cyber

**Score: 78/100** (was 78)

## What is being assessed

Comprehensive threat modeling across all attack surfaces of the ticket/tk project: HTTP REST API, WebSocket endpoints, embedded SPA, CLI, SQLite data store, container deployment. Assessed against OWASP Top 10 categories including SQL injection, XSS, CSRF, path traversal, command injection, SSRF, credential stuffing, session hijacking, container escape, and privilege escalation. Paranoid posture with special attention to non-alphanumeric character handling.

## Methodology

1. Static code review of all Go source in `internal/server/`, `internal/store/`, `internal/password/`, `internal/client/`, `cmd/tk/`.
2. SQL audit of every `db.ExecContext`/`db.QueryContext` call; trace of all `fmt.Sprintf` SQL string construction.
3. XSS trace of every `innerHTML` assignment in `web/static/index.html` against the `escape()` function.
4. Command execution audit of every `exec.Command` / `exec.CommandContext` call.
5. Session, cookie, and CSRF flow analysis.
6. WebSocket upgrade and origin validation review.
7. Container image and compose configuration review.
8. Authentication flow audit: password hashing, lockout, rate limiting.
9. Encryption-at-rest review for PII fields.
10. Input validation audit for non-alphanumeric characters in usernames, ticket fields, search queries.

## Attack Surface Table

| Surface | Entry Points | Trust Boundary | Assets at Risk |
|---------|-------------|----------------|----------------|
| HTTP REST API | 60+ endpoints under `/api/` | Network / Internet | All ticket data, user credentials, project config |
| WebSocket | `/api/ws`, `/api/chat/ws` | Network / Internet | Real-time data stream, chat subprocess I/O |
| Embedded SPA | `web/static/index.html` | Browser | Session tokens, CSRF tokens, rendered data |
| CLI | `cmd/tk` binary, 60+ subcommands | Local user | SQLite database, credentials.json, local filesystem |
| SQLite DB | `$TICKET_HOME/ticket.db` | Filesystem | All persistent state, password hashes, sessions |
| Chat subprocess | `TICKET_CHAT_CMD` / `TICKET_ANALYSE_CMD` | Server-side process | Server OS, any accessible resources |
| Container | Docker image, compose.yaml | Container runtime | Host Docker socket (watchtower), data volume |
| LLM agent | `--llm` flag on `tk agent` | CLI user input | Server OS via command execution |

## Threat Model

| Threat | Vector | Likelihood | Impact | Mitigation Status |
|--------|--------|-----------|--------|-------------------|
| SQL Injection | API params to SQL queries | Low | Critical | MITIGATED - all queries use `?` placeholders |
| Stored XSS | Ticket title/description rendered in SPA | Low | High | MITIGATED - `escape()` covers `& < > " '` on all innerHTML |
| CSRF | Forged POST/PUT/DELETE from malicious site | Low | High | MITIGATED - double-submit cookie with constant-time compare |
| Path Traversal | Static file serving, `TICKET_HOME` | Low | High | MITIGATED - `http.FileServer` + `fs.FS`, `spaHandler` validates against staticFS |
| Command Injection (server) | `TICKET_CHAT_CMD`, `TICKET_ANALYSE_CMD` env vars | Low | Critical | PARTIAL - env-var sourced, not user input; but no allow-list |
| Command Injection (CLI) | `--llm` flag on `tk agent` | Medium | High | PARTIAL - default case avoids `sh -c` but binary name is user-controlled |
| SSRF | No outbound HTTP from server | N/A | N/A | NOT APPLICABLE - server makes no outbound requests |
| Credential Stuffing | `/api/login` endpoint | Medium | High | MITIGATED - rate limit 10/min + account lockout after 10 failures |
| Session Hijacking | Token theft via XSS or network sniffing | Low | High | MITIGATED - HttpOnly cookies, SameSite:Lax, Secure on TLS |
| Container Escape | Docker runtime | Low | Critical | PARTIAL - non-root user, but watchtower has Docker socket |
| Privilege Escalation | Horizontal: access other users' data | Low | High | MITIGATED - project role checks, admin-only endpoints |
| WebSocket Hijacking | Cross-origin WebSocket connection | Low | Medium | MITIGATED - Origin header validation in `upgradeWebSocket` |

## Findings

### Passing checks

- **SQL Injection**: All SQL uses `?` placeholders throughout `internal/store/`. Dynamic `IN` clauses in `lifecycle.go:137` build `?` placeholder strings from slice lengths, not user data. The `fmt.Sprintf` calls in `store.go:1328-1456` use hardcoded internal table names from migration structs, not user input. `#nosec G202` annotations are correctly applied with justification.
- **XSS Protection**: `escape()` function at `index.html:5699-5706` covers all five HTML-significant characters (`& < > " '`). Traced 80+ `innerHTML` assignments; all server-sourced values pass through `escape()`. No use of `document.write`, `insertAdjacentHTML` with unescaped content, or `eval()`.
- **CSRF**: Double-submit cookie pattern implemented in `server.go:297-379`. Uses `crypto/rand` for 32-byte token generation. Validates with `subtle.ConstantTimeCompare`. Correctly exempts login/register (no cookie yet), Bearer/Basic auth (not browser-initiated), and requests without session cookie.
- **Password Hashing**: Argon2id with 64MB memory, 4 iterations, parallelism 2, 32-byte key, 16-byte salt via `crypto/rand` -- `internal/password/hash.go`. Verification uses `subtle.ConstantTimeCompare`.
- **Session Tokens**: 256-bit CSPRNG tokens via `crypto/rand`, base64url-encoded -- `auth.go:177-181`. Configurable expiry via `TICKET_SESSION_EXPIRY_DAYS`, default 30 days.
- **Account Lockout**: 10 failed attempts triggers 15-minute lockout -- `auth.go:27-28, 109-173`. Failed attempts reset on successful login.
- **Rate Limiting**: IP-based, 10 requests per minute on `/api/login` and `/api/register` -- `ratelimit.go`, `api.go:14`.
- **Cookie Security**: HttpOnly, SameSite:Lax, Secure conditional on TLS -- `api_auth.go:144-172`. CSRF cookie is non-HttpOnly (required for JS read) but SameSite:Strict.
- **Security Headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, CSP header -- `server.go:169-176`.
- **WebSocket Origin Validation**: `upgradeWebSocket` at `realtime.go:145-170` validates Origin header against Host, rejects cross-origin connections.
- **HTTP Timeouts**: `ReadHeaderTimeout: 30s`, `ReadTimeout: 60s`, `IdleTimeout: 120s` -- `server.go:39-45`. WriteTimeout intentionally omitted for WebSocket compatibility (documented).
- **Body Size Limits**: 1MB max on non-GET/HEAD requests via `MaxBytesReader` -- `server.go:178-186`.
- **Graceful Shutdown**: `Shutdown()` method closes reaper goroutine, chat heartbeat, and HTTP server -- `server.go:193-197`.
- **Container Hardening**: Non-root `ticket` user, multi-stage build, healthcheck, `ca-certificates` only -- `Dockerfile`.
- **Path Traversal**: `spaHandler` validates paths against `fs.Stat` on the static filesystem. `http.FileServer` with `http.FS` prevents directory traversal. `staticPath` uses `os.DirFS` which scopes to the given directory.
- **Input Validation**: Ticket types validated against allow-list. Project prefixes validated as `^[A-Z]{1,5}$`. Password minimum 8 characters enforced -- `auth.go:88`.
- **Sensitive Endpoint Logging**: Login, register, and reset-password request bodies are excluded from verbose logging -- `server.go:256-258`.
- **Session Cleanup**: `PurgeExpiredSessions` runs daily. `DeleteUser` cascades session deletion -- `auth.go:295-366`.
- **Panic Recovery**: `recoverMiddleware` catches panics and returns 500 without leaking stack traces -- `server.go:157-167`.
- **Encryption at Rest**: Optional AES-256-GCM encryption for email fields via `TICKET_ENCRYPTION_KEY` -- `encrypt.go`.
- **Credential Storage**: `.ticket/credentials.json` in `.gitignore`.
- **SEARCH/LIKE**: Search query in `ticket.go:887-890` uses parameterized `LIKE ?` with `%` wrapping -- no injection vector. SQLite LIKE does not interpret special chars beyond `%` and `_` which are harmless here.
- **Metrics Authenticated**: `/metrics` endpoint requires authenticated user -- `api_system.go:32-33`.

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| CLI `--llm` flag passes user-supplied binary name to `exec.Command` | High | `cmd/tk/cmd_agent.go:708` | Validate against allow-list of known agent binaries; reject paths containing `/`, `..`, or shell metacharacters |
| `TICKET_CHAT_CMD` and `TICKET_ANALYSE_CMD` parsed via `strings.Fields` and passed to `exec.Command` | Medium | `chat_ws.go:227`, `analyse.go:145,194` | Document that these env vars must be trusted; consider allow-listing binaries |
| Encryption key pad/truncate to 32 bytes weakens short keys | Medium | `store/encrypt.go:21-24` | Reject keys shorter than 32 bytes or derive via HKDF/scrypt; warn on startup if key is weak |
| CSP allows `'unsafe-inline'` for both script-src and style-src | Medium | `server.go:173` | Migrate to nonce-based CSP; extract inline styles to stylesheet |
| WebSocket auth token passed in query string (visible in server logs, proxy logs, browser history) | Medium | `api_auth.go:25,51` | Prefer cookie-based auth for WebSocket; or document risk and ensure tokens are short-lived |
| `clientIP()` uses `r.RemoteAddr` directly -- behind reverse proxy, all requests appear from proxy IP, rate limiting is per-proxy not per-client | Medium | `ratelimit.go:59-65` | Add configurable trusted-proxy support; parse `X-Forwarded-For` only when proxy CIDR is set |
| `http.DefaultClient` used in `internal/client/client.go:37` with no timeout | Low | `internal/client/client.go:37` | Create client with explicit `Timeout: 30 * time.Second` |
| No username character restriction beyond empty check | Low | `store/auth.go:83-84` | Restrict to `^[a-zA-Z0-9._-]+$` to prevent homoglyph attacks and log injection |
| Watchtower service mounts Docker socket without constraints | Low | `deploy/compose.yaml:18-19` | Add `security_opt: [no-new-privileges:true]`, `read_only: true`; consider socket proxy |
| Docker base images not pinned to digest | Low | `Dockerfile:3,16` | Pin `golang:1.26-alpine@sha256:...` and `alpine:3.21@sha256:...` for reproducible builds |
| `TICKET_FAST_HASH=1` env var reduces Argon2id to 1MB/1 iteration -- exploitable if set in production | Low | `internal/password/hash.go:23-24` | Log warning when fast hash is active; reject in production builds or require explicit opt-in flag |
| CSRF cookie not set with `__Host-` prefix | Informational | `server.go:329` | Use `__Host-_csrf` prefix when Secure is true, for stronger cookie integrity |
| No HSTS header | Informational | `server.go:169-176` | Add `Strict-Transport-Security` when TLS is active |
| Session cookie name `ticket_token` leaks application identity | Informational | `api_auth.go:145` | Use generic name like `__Host-session` |

## Verdict

The infoSec posture remains broadly stable in this pass. The earlier CSRF, lockout, timeout, body-limit, graceful-shutdown, and authenticated-metrics improvements are still in place, and the codebase continues to show strong baseline defenses through parameterized SQL, XSS escaping, Argon2id password hashing, and layered session management.

The remaining highest-risk item is the CLI `--llm` flag command injection vector, though its exploitability requires local CLI access (not remotely triggerable). The server-side command execution via env vars (`TICKET_CHAT_CMD`, `TICKET_ANALYSE_CMD`) is intentional functionality but should be documented as requiring trusted configuration. The encryption key handling in `encrypt.go` has a subtle weakness in accepting short keys.

The next clear gains would come from nonce-based CSP, tighter username validation, stronger encryption-key handling, trusted-proxy-aware rate limiting, and HSTS.

## TK-121 implementation commits

| Ticket | Recommendation | Commit | Status |
|--------|----------------|--------|--------|
| TK-156 | Allow-list known agent binaries; reject shell metacharacters in binary name | `0c33ada` | Done |
| TK-157 | Use HKDF to derive key; reject raw keys shorter than 32 bytes | `bf9062b` | Done |
| TK-158 | Migrate to nonce-based CSP for scripts and styles | `f64d009` (audit confirmation) | Done |
| TK-159 | Use cookie auth for WebSocket or document and mitigate log exposure | `ca92b5f` | Done |
| TK-160 | Add trusted-proxy CIDR config; parse X-Forwarded-For conditionally | `1b398f4` (audit confirmation) | Done |
| TK-161 | Document security requirements; consider allow-listing executables | `(this commit)` | Done |
| TK-162 | Set explicit 30s timeout on `internal/client` HTTP client | `8778bc8` | Done |
| TK-163 | Restrict to `^[a-zA-Z0-9._-]+$` | `25ca883` | Done |
| TK-164 | Pin base images to `@sha256:` digests | `69d171f` (audit confirmation) | Done |
| TK-165 | Add `no-new-privileges`, `read_only`, or use socket proxy | `4430649` | Done |
| TK-166 | Add `Strict-Transport-Security` when serving over TLS | `ab75c48` | Done |
| TK-167 | Use `__Host-` prefix for CSRF and session cookies when Secure | `b432ff2` | Done |
