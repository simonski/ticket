# OpenAPI Spec

**Score: 64/100** (was 64)

## What is being assessed
Accuracy and completeness of the OpenAPI specification (`openapi.yaml`) relative to the actual HTTP routes registered in code. Good means: every route documented, all parameters described, request/response schemas present, examples provided for each operation, and no spec-code drift.

## Methodology
Counted routes in `internal/server/api.go` vs operationIds in `openapi.yaml`. Grepped for parameter descriptions, response schemas, examples. Checked for endpoints in code not in spec (especially `/metrics`, agent config routes).

## Findings

### Passing checks
- 104 operationIds in `openapi.yaml` covering core CRUD operations
- 2xx response schemas present for ~100% of documented operations (`openapi.yaml` throughout)
- No multipart upload endpoints — code and spec consistently have zero (`internal/server/api.go`)
- Contact/license info correct in `openapi.yaml` header
- Version field in spec kept reasonably in sync

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `/metrics` endpoint exists in code but absent from spec | High | `internal/server/api_system.go:32` | Add `/metrics` with Prometheus text format response schema |
| `/api/agents/{id}/config` GET/POST/DELETE not in spec | High | `internal/server/api_agents.go:409-445` | Add 3 agent config operations to `openapi.yaml` |
| Parameter descriptions: only ~9% of 96 parameters have descriptions | Medium | `openapi.yaml` throughout | Add `description:` to all path/query params |
| Only 4 examples across 104 operations (3.8% coverage) | Medium | `openapi.yaml` | Add at least one request/response example per tag |

## Verdict
The spec covers the main CRUD surface well with good response schemas, but three routes are undocumented and parameter descriptions are almost entirely absent. No changes were made to `openapi.yaml` this cycle; score held steady.

## Changes since last assessment
- No changes to `openapi.yaml` this cycle
- `/metrics` and agent config gaps carried forward from v0.1.737

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Add `/metrics` to spec | High | Document Prometheus metrics endpoint with response schema |
| Add agent config routes | High | Document GET/POST/DELETE `/api/agents/{id}/config` and `/api/agents/{id}/config/{key}` |
| Parameter descriptions | Medium | Batch-add `description:` fields to all 96 parameters (one-time effort) |
| Add examples | Medium | Add at least one request/response example per operation tag |
