# InfoSec / Cyber

**Score: 72/100** (was 63)

## What is being assessed

Threat modeling across all attack surfaces: HTTP API, WebSocket, CLI, SQLite, Docker. Covers SQL injection, XSS, CSRF, path traversal, command injection, credential stuffing, session hijacking, and container security.

## Methodology

Static code review across all Go packages and the SPA. SQL audit of every `db.ExecContext`/`db.QueryContext` call. XSS trace of every `innerHTML` assignment. Command execution audit of every `exec.Command` call. Session/cookie/CSRF inspection. Container review.

## Findings

### Passing checks
- All SQL uses `?` placeholders; dynamic `IN` clauses build placeholder strings from lengths — `internal/store/`
- `escape()` function covers `&<>"'` and applied to all server-sourced innerHTML — `index.html:5596`
- 256-bit CSPRNG session tokens with Argon2id hashing — `auth.go:135`
- Cookie: HttpOnly, SameSite:Lax, Secure conditional on TLS — `api_auth.go:144-152`
- Rate limiting 10/min on login and register — `api_auth.go:87,118`
- WebSocket origin check — `realtime.go:132-157`
- Security headers on every response — `server.go:148-150`
- Project prefix validated: `^[A-Z]{2,5}$` — `keys.go:13`
- Non-root container, multi-stage build — `Dockerfile:20-22`
- File reads use `TICKET_HOME`-based paths only — no traversal risk
- Ticket type validated through allow-list switch — `keys.go:101-124`

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Command injection via `--llm` flag — `sh -c` with user string | High | `cmd_agent.go:698` | Replace with `exec.Command(binary, args...)` |
| `X-Forwarded-For` spoofing bypasses rate limiter | Medium | `ratelimit.go:49` | Trust only with configured proxy CIDR |
| CSS injection via label `color` field — `escape()` doesn't sanitise CSS | Medium | `index.html:4739` | Validate color to `^#[0-9a-fA-F]{3,6}$` server-side |
| Missing `ReadTimeout`/`WriteTimeout` — Slowloris variant | Medium | `server.go:34-38` | Add timeouts |
| Hardcoded fallback password `"password"` | Medium | `resolve.go:96-98`, `cmd_setup.go:1087` | Emit warning; prompt on `--populate` |
| No minimum password length | Low | `auth.go:76` | Enforce >= 8 chars |
| Watchtower mounts Docker socket without constraints | Low | `deploy/compose.yaml:10-14` | Add `no-new-privileges`, `read_only` |
| CSP allows `unsafe-inline` | Low | `server.go:150` | Replace with nonces |
| No username character restriction | Low | `auth.go:75` | Restrict to alphanumeric + `_` + `.` |

## Verdict

Major improvements: Argon2id confirmed, rate limiting on auth, WebSocket origin validation, comprehensive XSS coverage. The command injection via `--llm` is the highest-risk finding. Score improves +9 from 63 to 72.

## Changes since last assessment
- Password hashing confirmed as Argon2id with correct params
- Login rate limiting added
- XSS mitigated via comprehensive `escape()` on all innerHTML
- WebSocket origin validation added
- Session tokens confirmed as 256-bit CSPRNG

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Command injection via `--llm` | High | Allow-list agent binaries; no `sh -c` |
| X-Forwarded-For spoofing | Medium | Trusted-proxy CIDR |
| CSS injection via label color | Medium | Server-side hex validation |
| HTTP timeouts | Medium | Add Read/Write/IdleTimeout |
| Minimum password length | Low | Enforce >= 8 chars |
