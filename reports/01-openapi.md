# OpenAPI Spec

**Score: 55/100**

## What is being assessed
OpenAPI specification completeness and accuracy: operationId coverage, error response documentation, request body examples, response schema completeness, alignment between spec and implementation handlers, and drift risk from manual maintenance.

## Methodology
Reviewed `openapi.yaml` (3401 lines, 104 operations across 78 paths) and cross-referenced with `internal/server/api.go` (3009 lines). Counted operationIds, documented response codes, examples, and verified path/method alignment.

## Findings

### Passing checks
- All 104 operations have `operationId` ✅
- All 104 operations have `summary` ✅
- All 104 operations have `tags` ✅
- OpenAPI version 3.1.0 — valid ✅
- Path parameters correctly typed and described (int64, string, uuid as appropriate)
- Security schemes properly defined: `BearerAuth`, `CookieAuth`, `BasicAuth`
- 141 `required` field definitions across schemas — properly constrained
- All `$ref` pointers resolve correctly; no broken schema references
- Route/method alignment between spec and `api.go` handlers verified (GET, POST, PUT, DELETE all correctly routed)
- No multipart endpoints; all requests use `application/json`

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| 82/104 operations (79%) document only success response — no error codes | Critical | `openapi.yaml` throughout | Add 400, 401, 403, 404, 500 responses to all CRUD operations |
| 0/104 operations document `500 Internal Server Error` | High | `openapi.yaml` | Add standard 500 response to all operations via `$ref` to shared component |
| 0/37 request body definitions include examples | High | `openapi.yaml` throughout | Add `example:` to all requestBody schemas (register, login, createTicket, etc.) |
| 0/104 response definitions include examples (only 2 trivial `example: ok`) | High | `openapi.yaml` | Add realistic response examples to all schemas |
| Spec maintained manually — no code generation — high drift risk | Medium | repo-wide | Add `openapi-generator` to CI; regenerate SDKs from spec |
| No OpenAPI linter in CI | Medium | `.github/workflows/` | Add `spectral lint openapi.yaml` as CI step |
| `updateTicket` spec shows 200 only; implementation returns 400, 403, 404 | High | `openapi.yaml:2498`, `api.go:2818,2825,2857` | Document all actual error responses |

## Verdict
The spec has excellent structural coverage — all operations named, tagged, and security-scoped. However 79% of operations are missing error response documentation, making the spec misleading for API consumers and code generation. The complete absence of request/response examples severely limits the spec's usefulness as developer documentation.

## Changes since last assessment
First assessment.

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add 400/401/403/404/500 to all CRUD operations | Critical | Use shared `$ref: '#/components/responses/Unauthorized'` etc. |
| Add request body examples to all 37 requestBody defs | High | Real-world values, not placeholder strings |
| Add response examples to complex schemas | High | Ticket, User, Project, Workflow objects |
| Add `spectral lint` to CI | Medium | Prevents spec regressions |
| Evaluate openapi-generator for SDK generation | Medium | Eliminates manual client/server drift |
