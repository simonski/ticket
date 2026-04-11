# OpenAPI

**Score: 68/100** (was 64)

## What is being assessed

The OpenAPI spec at `openapi.yaml` against actual HTTP route registrations in `internal/server/`. Completeness of operationIds, request/response schemas, examples, and accuracy of URL paths — with focus on the SDLC lifecycle refactor additions.

## Methodology

Counted all `operationId` entries in `openapi.yaml` (109 total across 83 paths). Enumerated all `mux.HandleFunc` registrations and traced sub-route dispatch. Compared spec paths against code paths, checked schema completeness and example coverage.

## Findings

### Passing checks
- 109 `operationId` values present, no duplicates
- All 16 tag groups defined with descriptions (`openapi.yaml:21-49`)
- All SDLC/stage/stage-role endpoints have typed request and response schemas
- `Ticket` schema includes new lifecycle fields: `sdlc_stage_id`, `role_id`, `previous_sdlc_stage_id`, `previous_role_id`, `draft`
- `Project` schema includes `sdlc_id`
- Standard error responses consistently applied on all SDLC operations
- Component-level examples defined for User, Project, Ticket, Login

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| All SDLC paths use `/api/sdlc/` in spec but `/api/sdlcs/` (plural) in code — every SDLC call will 404 | Critical | `openapi.yaml:1673-2143` vs `api_sdlc.go:19-171` | Rename all `/api/sdlc/` to `/api/sdlcs/` |
| Stage-role URL structure differs: spec uses nested REST paths, code uses flat `/api/sdlcs/stages/roles/{sdlcId}/{stageId}` | Critical | `openapi.yaml:1997-2143` vs `api_sdlc.go:68-135` | Align spec to flat route structure |
| Four ticket action routes missing: `POST /api/tickets/{ref}/close`, `/open`, `/ready`, `/notready` | High | `api_tickets.go:579,603,673,696` | Add operations to spec |
| `GET /metrics` undocumented | High | `api_system.go:32` | Document endpoint |
| `addSdlcStage` request body has ghost `role_id` field not accepted by code | High | `openapi.yaml:1858` vs `api_types.go:39-43` | Remove from spec or implement |
| `{project_id}` typed as `integer/int64` but code accepts string prefix | Medium | `openapi.yaml:2638` vs `store/project.go:182-204` | Change type to `string` or document |
| Optional `message` body on action endpoints undocumented | Medium | `api_tickets.go:562-736` | Add optional requestBody |
| `ListSdlcs` returns `[]Sdlc` without stages but spec schema implies stages array | Medium | `openapi.yaml:265-268` vs `api_sdlc.go:144` | Split into Summary/Detail schemas |
| `importSdlc` request body is bare `type: object` | Medium | `openapi.yaml:1746-1748` | Define `SdlcExport` component schema |
| Dead `Conflict` (409) and `MethodNotAllowed` (405) response components | Low | `openapi.yaml:613,619` | Remove or use |
| `TicketExample` uses `open: true` — field doesn't exist | Low | `openapi.yaml:672` | Change to `complete: false` |

## Verdict

The spec improved with 30+ new SDLC operations and typed schemas. However, every SDLC path has the wrong URL prefix (`/api/sdlc` vs `/api/sdlcs`), making spec-generated clients unable to call any SDLC endpoint.

## Changes since last assessment
- Added 10 new SDLC paths, 3 stage-role operations, 6 ticket lifecycle operations
- Added `Sdlc`, `SdlcStage` schemas and lifecycle fields on Ticket/Project
- New URL mismatch discovered (singular vs plural)

## Remaining recommendations

| Finding | Severity | Recommendation |
|---------|----------|----------------|
| `/api/sdlc` vs `/api/sdlcs` mismatch | Critical | Find-replace in `openapi.yaml` |
| Stage-role URL structure | Critical | Align spec to flat route |
| 4 undocumented ticket actions | High | Add to spec |
| Ghost `role_id` on `addSdlcStage` | High | Remove from spec |
| `SdlcExport` schema missing | High | Define as named component |
