---
name: redteam
description: Use this skill when simulating a red team attack on the project. Applies when the user asks for a "red team assessment" or "red team report".
---

# Red Team Assessment Skill

Simulate an adversary attacking this project. Think like an attacker, not an auditor. The goal is to find chains of exploitation, not just checklists of missing headers.

## Core Principle

**Find what a real attacker would use.** Prioritise exploitable chains over theoretical issues. A CSRF token missing on an endpoint that already requires an API key is noise; a CSRF token missing on a session-cookie-authenticated state-changing endpoint is a real finding.

## Phase 1: Reconnaissance

Before probing anything, map the attack surface:

```bash
# All HTTP routes registered
grep -r 'router\.\|Handle\|HandleFunc\|GET\|POST\|PUT\|DELETE\|PATCH' internal/server/api_router.go

# All exec.Command / os.Exec calls — command injection candidates
grep -rn 'exec\.Command\|exec\.CommandContext\|os\.StartProcess\|sh -c' --include='*.go' .

# All file read/write operations — path traversal candidates
grep -rn 'os\.Open\|os\.ReadFile\|os\.WriteFile\|ioutil\.' --include='*.go' .

# All SQL query construction — injection candidates
grep -rn 'fmt\.Sprintf.*SELECT\|fmt\.Sprintf.*INSERT\|fmt\.Sprintf.*UPDATE\|fmt\.Sprintf.*DELETE' --include='*.go' .

# All innerHTML assignments — XSS candidates
grep -n 'innerHTML' web/static/index.html

# All cookie/session operations
grep -rn 'SetCookie\|Cookie\|session' --include='*.go' internal/server/

# All places user input flows to dangerous sinks
grep -rn 'Sprintf\|fmt\..*%s' --include='*.go' internal/server/
```

## Phase 2: Attack Chains to Investigate

Work through each chain fully before moving on. Document: **entry point → sink → impact → exploitability**.

### Chain 1: Authentication & Session
- Can session tokens be guessed or brute-forced? (length, entropy source)
- Is there account lockout? Try 100 failed logins — does the account lock?
- Can a valid session from user A be replayed as user B?
- Does logout actually invalidate the server-side token or just clear the cookie?
- Can session tokens appear in URLs, logs, or `Referer` headers?
- Password reset flow: is the token single-use? Time-limited? Bound to the account?

### Chain 2: Authorisation & Access Control
- Can a regular user access admin endpoints? Try every `/api/` route without `requireAdmin`.
- Can a member of project A read/write tickets in project B? Test by creating two projects and two users.
- Can a read-only role perform write operations by crafting a direct API request?
- IDOR: do IDs increment predictably? Can you access resource 42 if you only own resource 43?
- ForwardAuth: what happens if the `X-Forwarded-User` header is spoofed directly?

### Chain 3: Injection
- **Command injection** — `cmd_agent.go` uses `sh -c` with the `--llm` flag value. This is exploitable. Trace the full path from CLI flag to `exec`.
- **SQL injection** — check every `fmt.Sprintf` near a SQL statement. Dynamic `IN` clauses: are placeholders built from length or from content?
- **CSS injection** — label `color` field renders into `style=` attributes. Does the server validate it as `^#[0-9a-fA-F]{3,6}$`?
- **XSS** — trace every field that flows to `innerHTML`. Does `escape()` cover all paths, including error messages and ticket descriptions?
- **Path traversal** — any endpoint that takes a filename or path parameter. Does it resolve relative to `TICKET_HOME` or can `../` escape it?

### Chain 4: CSRF
- Identify every state-changing endpoint authenticated by session cookie.
- For each: is there a CSRF token, `SameSite=Strict`, or other mitigation?
- `SameSite=Lax` protects top-level navigations but not cross-origin `fetch()` with credentials.
- Can an attacker host a page that causes a logged-in user to create/delete tickets?

### Chain 5: WebSocket Abuse
- Can an unauthenticated client connect to the WebSocket endpoint?
- Does the origin check use an allow-list or just compare the request origin to the host header (which can be spoofed in some configurations)?
- Cross-project broadcast: can client in project A receive events from project B?
- Message flooding: is there a rate limit on WS messages? Can one client starve others?

### Chain 6: Information Disclosure
- `/metrics` — is this endpoint gated? What does it expose? (heap stats, goroutine counts, request rates)
- Error messages — do stack traces or internal paths leak in 500 responses?
- API responses — do they include fields that should be server-only (password hashes, internal IDs, tokens)?
- Logs — are credentials or tokens logged at any level?
- HTTP headers — does `Server:` or `X-Powered-By:` reveal version info?

### Chain 7: Denial of Service
- No `ReadTimeout`/`WriteTimeout` → Slowloris: open many connections, send headers slowly.
- No request body size limit → send a multi-GB JSON body to any handler.
- No WS rate limit → flood the event bus.
- SQLite locking: concurrent writes will block — can an attacker hold a write lock indefinitely?
- Unbounded list endpoints: request all tickets with no pagination → full table scan and response.

### Chain 8: Container & Infrastructure
- Docker socket mounted by Watchtower without constraints → if Watchtower is compromised, full host access.
- Missing `cap_drop: [ALL]` → container runs with default Linux capabilities.
- Missing `pids_limit` → fork bomb possible inside container.
- Volumes: are secrets mounted read-write when read-only would suffice?
- Environment variables: are credentials visible in `docker inspect`?

### Chain 9: Supply Chain
```bash
# Check for known vulnerabilities in dependencies
govulncheck ./...

# Review go.sum for unexpected changes
git log --oneline go.sum | head -20

# Check for dependencies with broad permissions
grep -E 'exec|net|os' go.mod
```

## Phase 3: Output Format

Produce a single report. Structure each finding as an **attack chain**, not a checklist item:

```markdown
## Finding: <Title>

**Severity:** Critical | High | Medium | Low
**CVSS-like score:** (optional)
**Status:** Open | Mitigated | Accepted

### Attack chain
1. Attacker does X (entry point: `file.go:line`)
2. Input flows to Y (sink: `file.go:line`)
3. Result: attacker achieves Z

### Proof of concept
Minimal reproduction steps or curl command.

### Impact
What can the attacker do? Data exfiltration, account takeover, RCE, DoS?

### Remediation
Specific code change with file and line reference.
```

## Phase 4: Severity Guide

| Severity | Criteria |
|----------|----------|
| **Critical** | Unauthenticated RCE, authentication bypass, full data exfiltration |
| **High** | Authenticated RCE, IDOR across users, persistent XSS, CSRF on sensitive actions |
| **Medium** | Information disclosure, self-XSS, missing rate limit on non-auth endpoint |
| **Low** | Defence-in-depth gaps, hardening improvements, theoretical issues without realistic exploit |

## Phase 5: Known Prior Findings

Do not re-report these as new — reference them and note if status changed:

| Finding | Severity | Location | Status |
|---------|----------|----------|--------|
| Command injection via `--llm` (`sh -c`) | High | `cmd_agent.go:698` | Open |
| No CSRF protection on state-changing endpoints | High | All `api_*.go` | Open |
| No account lockout after failed logins | High | `api_auth.go:113-137` | Open |
| `X-Forwarded-For` spoofing bypasses rate limiter | Medium | `ratelimit.go:49` | Open |
| CSS injection via label `color` field | Medium | `index.html:4739` | Open |
| `/metrics` unauthenticated | Medium | `api_system.go:32` | Open |
| Cross-project WebSocket broadcast | Medium | `realtime.go:66-80` | Open |
| Missing `ReadTimeout`/`WriteTimeout` | Medium | `server.go:34-38` | Open |
| No request body size limit | Medium | Various handlers | Open |
| Container missing `cap_drop`, `pids_limit` | Medium | `compose.yaml` | Open |
| Hardcoded fallback password `"password"` | Medium | `resolve.go:96` | Open |
| CSP allows `unsafe-inline` | Low | `server.go:150` | Open |

Focus your effort on finding **new** findings or **exploitable chains** that combine multiple known issues.
