# API Architect

**Score: 68/100** (was 74)

## Mission
Protect contract clarity so API consumers can integrate safely without reverse-engineering implementation details.

## Review objective
Check whether the OpenAPI contract is valid, current, and specific enough to describe real request/response rules.

## Inputs reviewed
- `openapi.yaml`
- `internal/server/api_helpers.go`
- `internal/server/api_tickets.go`
- `internal/store/ticket.go`

## Findings

### Passing checks
- The API contract attempts to cover the main auth schemes and core entities rather than leaving consumers with only handler code (`openapi.yaml:53-76`, `openapi.yaml:78-186`).
- Lifecycle parsing from rendered `status` is centralized instead of scattered across handlers (`internal/server/api_helpers.go:25-33`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The OpenAPI document is malformed at the top-level version field. | Critical | Standard validators, viewers, and SDK generators fail before any deeper contract checking can happen. | `openapi.yaml:1-10` | Repair the YAML and add OpenAPI validation to CI. |
| The ticket type enum in OpenAPI omits valid backend ticket types such as `feature` and `idea`. | High | API consumers can reject valid values or miss supported domain concepts. | `openapi.yaml:169-186`, `internal/store/ticket.go:1620-1627` | Make the spec enum match the backend’s authoritative ticket-type set. |
| Explicit `stage`/`state` inputs bypass HTTP-layer parsing and are left for deeper store validation. | Medium | Clients get later, less contract-shaped failures and the API contract stays underspecified. | `internal/server/api_helpers.go:25-33`, `internal/store/ticket.go:180-188` | Validate explicit lifecycle fields at the HTTP boundary and document those rules in the spec. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| devops-engineer | Broken API specs should be gated before merge/release. | CI validation step for `openapi.yaml`. |
| tech-writer | Spec and user-facing docs must agree on supported types and lifecycle semantics. | Contract drift cleanup list. |
| backend-engineer | HTTP validation should match store validation. | Where should lifecycle validation live? |

## Verdict
The codebase has a serious API-contract problem even though the handlers themselves are fairly disciplined. Until `openapi.yaml` parses and the enum drift is fixed, the contract is not reliable enough to be treated as an authoritative integration surface.

## Changes since last assessment
- The core blocker from the previous assessment remains unresolved: the spec is still malformed at the version field (`openapi.yaml:1-10`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Invalid OpenAPI document | Critical | Repair the YAML and validate it in CI. | api-architect |
| Ticket type drift | High | Align spec enums with backend-valid types. | api-architect |
| Late lifecycle validation | Medium | Validate and document explicit stage/state rules at the API boundary. | backend-engineer |
