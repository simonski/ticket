# Backend Engineer

**Score: 78/100** (was 79)

## Mission
Protect correctness, explicit validation, and safe state transitions in the server and store implementation.

## Review objective
Verify that important server-side paths remain explicit, validated, and safe under normal API use.

## Inputs reviewed
- `internal/store/ticket.go`
- `internal/server/api_tickets.go`
- `internal/server/api_helpers.go`
- `internal/server/server.go`
- `internal/server/analyse.go`
- `internal/server/chat_ws.go`

## Findings

### Passing checks
- Ticket creation enforces project, title, type, parent, state, and active-assignee rules in one place instead of leaving them implicit (`internal/store/ticket.go:153-188`).
- Ticket updates validate assignee existence/enabled state and prevent direct lifecycle edits on tickets whose state is derived from children (`internal/store/ticket.go:401-420`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Explicit `stage`/`state` inputs are passed through the HTTP helper without contract-shaped validation. | Medium | API callers get later store errors instead of early, HTTP-layer lifecycle feedback. | `internal/server/api_helpers.go:25-33`, `internal/store/ticket.go:180-188` | Validate explicit lifecycle fields at the request boundary before calling store logic. |
| Ticket creation can succeed even if the optional follow-up comment fails. | Low | The API can return success while silently dropping user intent attached to the same request. | `internal/server/api_tickets.go:73-75` | Either make comment attachment part of the same transactional outcome or surface the partial-failure state explicitly. |
| Server-side analyse/chat subprocesses inherit the full process environment. | Low | More configuration than necessary is exposed to child processes, which expands operational and debugging risk. | `internal/server/analyse.go:60-79`, `internal/server/analyse.go:139-176`, `internal/server/chat_ws.go:227-237` | Pass a filtered environment to child commands and document the contract explicitly. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| api-architect | Validation timing affects API contract quality. | Where should lifecycle validation fail? |
| security-engineer | Child-process environment scope is a security as well as backend concern. | Minimal env contract for subprocesses. |
| code-reviewer | Partial-success behavior needs an explicit stance. | Should create+comment be atomic? |

## Verdict
The store still carries the domain rules well, and the server generally defers to those rules instead of inventing its own. The remaining weaknesses are boundary polish: late validation and partial-success behavior make the backend feel less explicit than it should.

## Changes since last assessment
- The backend remains structurally solid, but the widened product surface makes boundary-level ambiguity more costly than it was in the earlier architect/backend passes (`internal/server/api_helpers.go:25-33`, `internal/server/api_tickets.go:73-75`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Late lifecycle validation | Medium | Validate explicit lifecycle fields at HTTP ingress. | backend-engineer |
| Partial-success comment path | Low | Make create+comment atomic or explicitly partial. | backend-engineer |
| Broad child-process env | Low | Filter subprocess env to the minimum needed. | security-engineer |
