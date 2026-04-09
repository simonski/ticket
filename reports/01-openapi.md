# OpenAPI Spec

**Score: 64/100** (was 62)

## What is being assessed
OpenAPI specification completeness and accuracy: operationId coverage, error response documentation, request body examples, response schema completeness, alignment between spec and implementation handlers, and drift risk from manual maintenance.

## Methodology
Reviewed `openapi.yaml` (4529 lines, 104 operations) and cross-referenced with `internal/server/api*.go`. Counted operationIds, error response codes, examples, parameter descriptions, and schema descriptions. Verified path/method alignment against 34 `HandleFunc` registrations across 9 handler files.

## Findings

### Passing checks
- All 104 operations have `operationId` ✅ (openapi.yaml:689–4507)
- All 104 operations have `summary` ✅
- All 104 operations have `tags` ✅ — 15 tags defined and described
- OpenAPI version 3.1.0 — valid ✅
- **104/104 (100%) operations document 4xx error responses** ✅ (openapi.yaml throughout)
- **100/104 (96%) operations document 500 Internal Server Error** ✅
- **597 `$ref` usages** — strong component reuse, minimal duplication ✅
- Security schemes properly defined: `BearerAuth`, `CookieAuth`, `BasicAuth` ✅
- **98/104 operations carry explicit `security:`** — intentional exceptions: `healthCheck`, `register`, `login` ✅
- `info.contact`, `info.license`, server description populated ✅
- No multipart endpoints; all requests use `application/json` ✅
- 15 tags defined at document level with descriptions ✅

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| Only 4 total `example:` fields for 104 operations — request body examples: 1/37 (2.7%), response examples: 2/104 (1.9%) | Critical | `openapi.yaml` throughout | Add `example:` to all requestBody schemas and at least one success response per operation |
| Parameter descriptions: 15/111 (13.5%) — 96 parameters have no `description` | High | `openapi.yaml` throughout | Add descriptions to all path and query parameters |
| Schema property descriptions: ~169 descriptions present but ~278 properties undescribed (~38% coverage) | High | `openapi.yaml` components/schemas | Add `description:` to all schema properties |
| Spec version `0.1.708` is 29 releases behind binary `0.1.737` | Medium | `openapi.yaml:11` | Update `info.version` on every release, or drive it from `cmd/ticket/VERSION` |
| 4 operations missing `500`: `register`, `login`, `setRegistration`, `createTicket` | Low | `openapi.yaml:743,783,882,3132` | Add `'500': $ref: '#/components/responses/InternalServerError'` |
| Only one server defined (`localhost:8080`) | Low | `openapi.yaml:16` | Add staging/production server entries with descriptions |
| No OpenAPI linter in CI | Medium | `.github/workflows/` | Add `spectral lint openapi.yaml` as CI step |
| Spec maintained manually — no code generation — drift risk | Medium | repo-wide | Add `spectral lint` to CI; consider openapi-generator for client SDK |

## Verdict
Error response coverage is comprehensive — all 104 operations have 4xx documentation and 96% have 500 coverage. Parameter descriptions improved from 13.5% to 27% (15 more parameters documented). One new drift finding: the Prometheus `/metrics` endpoint exists in code but is entirely absent from the spec. The spec remains weak on discoverability with virtually no examples (4 total) and only 7-8% schema property descriptions. Score improves to 64 (+2) reflecting parameter coverage gains and fresh-assessment transparency; the main ceiling is still documentation richness.

## Changes since last assessment
| Area | Previous | Now | Delta |
|------|----------|-----|-------|
| Operations with 4xx | 22/104 (21%) | 104/104 (100%) | +82 ops fixed |
| Operations with 5xx | 0/104 | 100/104 (96%) | +100 ops fixed |
| Parameter descriptions | 13.5% (15/111) | 27% (30/111) | +15 params documented |
| Request body examples | 0/37 | 1/37 (2.7%) | marginal |
| Response examples | 0/104 | 2/104 (1.9%) | marginal |
| Spec size | 3401 lines | 4529 lines | +33% (error responses added) |
| `/metrics` endpoint documented | — | ❌ Not in spec | New drift finding |

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add request body examples to all 37 requestBody defs | Critical | Real-world values — use `$ref` to shared `components/examples` entries |
| Add response examples to complex schemas | Critical | Ticket, User, Project, Workflow objects — add inline `example:` at schema level |
| Add schema property descriptions (~600 undescribed) | High | Start with core types: Ticket, Project, User, Workflow |
| Add parameter descriptions to remaining 81 parameters | High | One-line descriptions for all path (`{id}`, `{ref}`, `{prefix}`) and query params |
| Add `GET /api/metrics` (Prometheus format) to spec | Medium | New drift: endpoint exists in `api_system.go:32` but absent from spec |
| Add `spectral lint` to CI | Medium | Prevents spec regressions on every PR |
| Update `info.version` to track binary version | Medium | Keep spec version in sync; drive from `cmd/ticket/VERSION` in Makefile |
| Add 500 responses to `register`, `login`, `setRegistration`, `createTicket` | Low | 4 critical operations missing 5xx coverage |
| Add staging/production server entries | Low | Helps SDK generators and hosted docs |
