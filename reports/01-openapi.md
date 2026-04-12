# OpenAPI

**Score: 78/100** (was 78)

## What is being assessed

The `openapi.yaml` specification file for the Ticket API. This assessment checks completeness (all server routes documented), correctness (URL patterns match implementation), schema quality (response schemas, examples), and consistency (client, server, and spec all agree).

## Methodology

1. Read `openapi.yaml` and counted operationIds.
2. Extracted every `mux.HandleFunc` route from `internal/server/api_*.go` (auth, system, users, agents, roles, sdlc, teams, projects, tickets).
3. Cross-referenced server routes against openapi.yaml paths.
4. Verified SDLC endpoint URLs match between server, client (`internal/client/client.go`), and openapi.yaml.
5. Checked stage-role URL structure alignment between client and server.
6. Reviewed schema definitions, `$ref` usage, and examples.
7. Checked for new endpoints related to recent changes (libticket merge, tk init flags).

## Findings

### Passing checks

- **113 operationIds** defined, covering a comprehensive API surface.
- **SDLC URL mismatch resolved**: All SDLC endpoints now consistently use `/api/sdlcs` (plural) across server, client, SPA, and openapi.yaml. The previous `/api/sdlc` vs `/api/sdlcs` mismatch from v3 is fixed.
- **Stage-role URLs aligned**: Server registers `/api/sdlcs/stages/roles/{sdlcId}/{stageId}[/{roleId}]`. Client uses the same pattern (`/api/sdlcs/stages/roles/%d/%d` and `.../%d/%d/%d`). OpenAPI documents `/api/sdlcs/stages/roles/{sdlc_id}/{stage_id}` and `.../{role_id}`. All three agree.
- **23 component schemas** defined (User, Agent, Project, Ticket, Sdlc, SdlcExport, SdlcStage, Team, TeamMember, Role, Label, Comment, Dependency, TimeEntry, Story, HistoryEvent, ProjectMember, ProjectTeamMember, CountSummary, StatusResponse, AuthResponse, AgentWorkResponse, TicketRequestResponse).
- **101 `$ref` usages** for schema reuse across the spec.
- **8 named examples** provided in the components section.
- **Error responses** consistently documented using `$ref` to shared response components (BadRequest, Unauthorized, Forbidden, NotFound, InternalServerError).
- **Security schemes** correctly documented: BearerAuth, CookieAuth, BasicAuth.
- **Tags** well-organized across 16 categories.
- **No new API drift**: Recent codebase changes (libticket merge, `tk init` flags) did not introduce new API endpoints.

### Issues found

| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| `/metrics` endpoint missing from spec | Medium | `api_system.go:32` registers `GET /metrics` (Prometheus format) | Add `/metrics` path with `text/plain` response content type |
| `/api/users/{username}/reset-password` missing from spec | Medium | `api_users.go:91` handles `POST /api/users/{username}/reset-password` | Add path with password request body and User response schema |
| `/api/projects/{project_id}/set-draft` missing from spec | Medium | `api_projects.go:429` handles `PUT /api/projects/{project_id}/set-draft` | Add path with `{draft: boolean}` request body |
| `/api/agents/{agent_id}/config` (GET/POST) missing from spec | Medium | `api_agents.go:409` handles agent config CRUD | Add path for listing and setting agent config entries |
| `/api/agents/{agent_id}/config/{key}` (DELETE) missing from spec | Medium | `api_agents.go:432` handles config key deletion | Add path for deleting individual agent config keys |
| Only 4 inline examples across 113 operations | Low | Throughout openapi.yaml | Add response examples to high-traffic endpoints (ticket CRUD, project list, login) |
| `healthz` endpoint documents 401/403/404 error responses it cannot produce | Low | openapi.yaml line 741-750 | Remove inapplicable error responses from unauthenticated endpoint |
| Version in spec (`0.1.708`) likely stale | Low | openapi.yaml line 9 | Automate version sync with `cmd/tk/VERSION` |

## Verdict

The OpenAPI spec remains broadly stable in this pass. The earlier SDLC URL and stage-role alignment fixes are still intact across server, client, SPA, and spec, schema coverage remains solid with 23 component schemas and strong `$ref` reuse, and the main remaining gap is still the five undocumented server endpoints plus thin example coverage.

## Changes since last assessment

- 2026-04-12 — TK-145 — commit `67b0af3` documented `/metrics`, `/api/users/{username}/reset-password`, `/api/projects/{project_id}/set-draft`, `/api/agents/{agent_id}/config`, and `/api/agents/{agent_id}/config/{key}` in `openapi.yaml`
- 2026-04-12 — TK-147 — commit `67b0af3` removed impossible 401/403/404 responses from `/api/healthz`
- 2026-04-12 — TK-148 — commit `108ee1f` verified the existing `sync-openapi-version` Makefile target already keeps `openapi.yaml` in sync with `cmd/tk/VERSION`
- 2026-04-12 — TK-146 — commit `b46cf07` added inline request/response examples for login, projects, ticket CRUD/history/claim flows, and fixed nearby OpenAPI path/response drift

## Remaining recommendations

None. The current OpenAPI backlog items from this assessment are now resolved.
