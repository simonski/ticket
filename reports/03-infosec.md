# InfoSec / Cyber Threat Model

**Score: 52/100**

## What is being assessed
Full attack surface threat model covering: SQL injection, XSS, CSRF, path traversal, command injection, SSRF, credential stuffing, session hijacking, container escape, and privilege escalation. Adopts a paranoid posture examining all trust boundaries.

## Methodology
Systematic review of all API handlers, template rendering, subprocess execution, SQL queries, and infrastructure configuration. Cross-referenced OWASP Top 10 against codebase. Checked every user input path and external process invocation.

## Findings

### Passing checks
- **SQL injection**: All user-input queries use `?` placeholders — no direct string interpolation of user data
- **XSS**: Single-page app uses `escape()` function before all `innerHTML` insertions; error messages use `.textContent`; CSP headers set
- **Path traversal**: Static files served via `fs.FS` wrapper — OS-level path constraints prevent `../` breakout
- **File upload**: No file upload endpoints exist
- **Session hijacking**: 32-byte cryptographically random tokens; `HttpOnly`, `Secure`, `SameSite=Lax` cookies
- **Parameterized queries**: Consistent `?` placeholder use throughout `internal/store/`
- **Dependency vulnerabilities**: No known CVEs in current dependency set
- **Credential enumeration**: Same error message for non-existent user and wrong password

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Command injection via `exec.Command("sh", "-c", llmCmd)` | Critical | `cmd/ticket/cmd_agent.go:701` | Replace with `exec.Command("claude", "-p", "--model", ..., promptFile)` — no shell |
| SSRF in analyse feature — URL from config not validated | High | `internal/server/analyse.go:61` | Validate URL is localhost-only (`127.0.0.1` or `::1`) before use |
| Credentials passed via env vars to analysis subprocess | High | `internal/server/analyse.go` | Use API token bearer auth; avoid passing username/password to subprocesses |
| Internal errors leaked to clients via `err.Error()` in responses | High | `internal/server/api.go:139`, multiple | Catch specific errors (UNIQUE constraint etc.) and return generic messages; log details server-side |
| `/api/status` returns server version + chat config to unauthenticated users | Medium | `api.go:233,242` | Require authentication for `/api/status`; disable version disclosure |
| SQL `PRAGMA table_info()` uses string concatenation for table name | Medium | `internal/store/store.go:1595,1565` | Validate `tableName` against strict allowlist (alphanumeric + underscore only) |
| No CSRF protection on state-changing endpoints | High | All POST/PUT/DELETE in `api.go` | Implement CSRF middleware (see security report) |
| WebSocket token exposed in query parameters (logged in URLs/history) | Medium | `api.go:44,70` | Require `Authorization: Bearer` header only |
| `chat_ws.go` spawns arbitrary shell commands | High | `internal/server/chat_ws.go:~150-200` | Audit all `exec.Command` calls; use arg arrays not shell strings; sandbox with seccomp |

## Verdict
The most critical issue is command injection in the LLM agent feature — passing a shell string to `sh -c` with a file path derived from ticket analysis could allow an attacker to escape the intended command boundary. SSRF in the analyse feature is a secondary concern. The core web application (tickets, users, projects) has a solid security posture but should not be Internet-exposed without fixing the agent command injection and adding CSRF protection.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Command injection in agent execution | Critical | Use `exec.Command` with explicit arg array; never pass to `sh -c` |
| SSRF in analyse feature | High | Allowlist: only `127.0.0.1` and `::1` URLs accepted |
| Internal error message leakage | High | Catch typed errors; log detail; return generic user-facing message |
| CSRF middleware | High | Protect all state-changing endpoints |
| Version disclosure on `/api/status` | Medium | Require auth or remove version from unauthenticated response |
| PRAGMA string concatenation | Medium | Validate table name against alphanumeric allowlist |
| Subprocess credential handling | High | Pass API token not username/password to subprocesses |
