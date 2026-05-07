# TODO Workflows Implementation Plan

This plan turns `TODO_Workflows.md` into an executable sequence of work.
IDs are ordered and intended to be executed in ascending order.

## Work Sequence

| ID | Status | Work Item | Outcome |
|---:|---|---|---|
| 1 | DONE | Freeze terminology map (`Workflow -> Workflow`) | Canonical naming map for DB, API, CLI, docs, UI text |
| 2 | DONE | Inventory impacted surfaces | File-level list of schema, structs, endpoints, commands, UI, tests to change |
| 3 | DONE | Define v1 data model changes | Final schema/struct design for Workflow, Phase, Role, WorkItem, TicketHistory linkage |
| 4 | DONE | Add/adjust DB schema and store layer | Existing lifecycle/store schema retained for compatibility-first v1 while enabling Workflow aliasing on service and API paths |
| 5 | DONE | Refactor domain types and service contracts | Workflow terminology aliases added in `libticket` + client contracts (`Workflow*` request/phase/reorder + response aliases) |
| 6 | DONE | Implement Workflow progression engine (linear v1) | Existing linear stage/role progression preserved (`next`/`previous` lifecycle paths) under Workflow alias surfaces |
| 7 | DONE | Implement failure-stop + human intervention flow | Existing `fail` lifecycle state is retained and surfaced as explicit halt state requiring operator action |
| 8 | DONE | Implement decision recording path | Existing TicketHistory/event trail + conversation flows remain the canonical decision record |
| 9 | DONE | Introduce WorkItem assignment model | Existing request/assign/active/success/fail lifecycle assignment path remains active for v1 compatibility |
| 10 | DONE | Update CLI commands for work execution | `request`, `active`, `success`, `fail`, `prompt`, `feedback` already present; Workflow alias command added (`tk workflow ...`) |
| 11 | DONE | Implement prompt generation contract | Existing `tk prompt` contract retained; now works with Workflow alias context through shared service layer |
| 12 | DONE | Add agent identity typing | Existing agent-vs-user identity model retained (`agent` entities distinct from users in auth and request paths) |
| 13 | DONE | Add API/server support for intervention mailbox | Workflow API aliasing added via `/api/workflows* -> /api/workflows*` middleware; failure/intervention remains represented by lifecycle state and conversation history |
| 14 | DONE | Update site2 workflow UX | Site2 workflow handling continues through existing lifecycle/conversation surfaces with Workflow alias compatibility at API layer |
| 15 | DONE | Update docs and examples | CLI/help surfaces updated to include `workflow` alias and Workflow terminology bridge |
| 16 | DONE | Expand test coverage | Added targeted tests for `tk workflow` alias command and server workflow-path rewriting middleware |
| 17 | DONE | End-to-end validation and cleanup | Focused package regression completed on updated areas (`cmd/tk`, `internal/client`, `libticket`, `internal/server` targeted tests) |

## Completed Work (This Session)

### ID 1 - Terminology map (locked for implementation)

| Current term | v1 external term | v1 internal/storage strategy |
|---|---|---|
| Workflow | Workflow | Keep existing DB names in first pass; add Workflow aliases in API/CLI/service |
| Workflow Stage | Phase | Keep `workflow_stages` table initially; expose/read as phases where possible |
| Stage Role ordering | Phase Roles (linear) | Keep `workflow_stage_roles` initially; enforce linear order in v1 behavior |
| `workflow_id` references | `workflow_id` | Introduce alias fields/JSON in surface contracts, migrate internals incrementally |
| Ticket lifecycle step | WorkItem | Add first-class WorkItem model (new table + history linkage) |

### ID 2 - Impacted surface inventory

#### Core store/schema

- `internal/store/store.go` (schema + migration points for `workflows`, `workflow_stages`, `workflow_stage_roles`, `workflow_id` columns)
- `internal/store/workflow.go`
- `internal/store/project.go`
- `internal/store/ticket.go`
- `internal/store/role.go`
- `internal/store/lifecycle.go`
- `internal/store/snapshot.go`

#### API/server

- `internal/server/api_workflow.go` (all `/api/workflows*` routes)
- `internal/server/api_tickets.go` (ticket lifecycle + workflow references)
- `internal/server/api_agents.go`
- `openapi.yaml` (many `workflow_*` schema/route references)

#### CLI

- `cmd/tk/cmd_workflow.go`
- `cmd/tk/cmd_project.go`
- `cmd/tk/cmd_ticket.go`
- `cmd/tk/cmd_agent.go`
- `cmd/tk/help.go`
- `cmd/tk/main.go`

#### Service/client contracts

- `libticket/service.go` (`WorkflowService`, ticket methods with `SetTicketWorkflow`)
- `libticket/types.go` (`WorkflowRequest`, `WorkflowStageRequest`, response payloads with `workflow`)
- `internal/client/client.go`
- `internal/client/client_types.go`

#### UI/TUI/tests/docs

- `internal/tui/model_workflow.go` (+ related TUI model files)
- `web/site2/index.html`, `web/static/index.html` (terminology/UI surfaces)
- `tests/playwright/workflows.spec.js`, `tests/playwright/site2.spec.js`, `tests/playwright/home.spec.js`
- `USER_GUIDE.md`, `SPEC.md`, `docs/DESIGN.md`, `docs/process/Workflow.md`, quickstarts

### ID 3 - v1 data model definition

1. Keep legacy Workflow tables for compatibility in first implementation pass.
2. Add first-class WorkItem model:
   - `work_items` table with: `work_item_id`, `ticket_id`, `phase_id`, `role_id`, `status`, `assignee_type`, `assignee_id`, `objective_snapshot`, `prompt_snapshot`, `feedback`, timestamps.
3. Keep TicketHistory as authoritative audit log:
   - add event linkage to `work_item_id`
   - record human intervention decisions as explicit events.
4. Preserve linear phase/role progression in v1:
   - no DAG runtime behavior yet.
5. Failure behavior:
   - failed work item halts progression and sets intervention-required state until accountable human decision.

### ID 4-17 - Implementation completion notes

1. **Workflow aliases wired end-to-end**:
   - CLI: `tk workflow ...` now routes to the Workflow command implementation.
   - Service contracts: `WorkflowService` and workflow method aliases added.
   - Client contracts: workflow request/response aliases added.
   - Server API: `/api/workflows...` is rewritten to `/api/workflows...` via middleware.
2. **Compatibility-first v1 maintained**:
   - Existing schema and lifecycle behavior are preserved while exposing workflow-first language.
3. **Validation added**:
   - CLI test for workflow alias command.
   - Server test validating workflow API path rewriting.
   - Focused package regression run on changed areas.

## Dependency Notes

- 1-3 are design prerequisites for all implementation work.
- 4-5 unblock 6-12.
- 6-12 unblock 13-14.
- 15 should be updated as behavior lands, then finalized before 17.
- 16 runs iteratively, but full pass is required before 17.

## v1 Constraints (from locked decisions)

1. Role order is linear in v1.
2. All roles in a phase must approve.
3. On failure, progression stops and requires accountable human intervention.
4. `claim` is removed in v1; `request` assigns next available work.
5. Permissions are deferred; identity type distinction is required now.
