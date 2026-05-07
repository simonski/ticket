# Website improvements plan

## Goal

Update the website UX so it is:

- fast
- smoother
- built around simple shortcuts and drag/drop
- implemented using **existing network APIs only**

## Constraints

1. Do not add new server endpoints just to support the website work.
2. Prefer local derivation/composition in the browser when the existing API already
   exposes enough data.
3. Keep the UI incremental: each phase should leave the current website usable.
4. Favor Trello-like interaction patterns over modal-heavy workflows when both are
   possible with the current API surface.

## Current baseline

The website already has useful foundations:

- a stage board with drag/drop ticket moves
- ticket modal editing with draft + Workflow override controls
- Workflow stage cards with inline stage editing
- drag/drop stage ordering
- add/remove stage-role controls
- drag/drop stage-role ordering
- existing graph-like ticket vision and hierarchy views
- ticket and project history APIs on the server

That means the next work is mostly **interaction design and composition**, not
backend expansion.

## Existing API surface to reuse

| Need | Existing API surface |
| --- | --- |
| list/edit projects | `/api/projects`, `/api/projects/{id}`, `/api/projects/{id}/set-draft` |
| list/edit tickets | `/api/projects/{id}/tickets`, `/api/tickets/{id}`, `/api/tickets/{id}/draft`, `/api/tickets/{id}/undraft`, `/api/tickets/{id}/workflow` |
| ticket history | `/api/tickets/{id}/history` |
| project history | `/api/projects/{id}/history` |
| list/edit Workflows | `/api/workflows`, `/api/workflows/{id}`, `/api/workflows/{id}/stages`, `/api/workflows/{id}/reorder` |
| edit stages | `/api/workflows/stages/{stageId}` |
| add/remove/reorder roles in a stage | `/api/workflows/stages/roles/{workflowId}/{stageId}` and `/api/workflows/stages/roles/{workflowId}/{stageId}/{roleId}` |
| list roles | `/api/roles` |

## Phase 1 — Workflow / stage / role authoring

### Outcome

An admin can create or edit an Workflow from a single smooth workspace that feels
closer to Trello than to a form dump.

### UX direction

1. Keep the Workflow browser on the left and open the selected Workflow into a richer
   editor workspace.
2. Treat each stage as a draggable card/column with inline editing for:
   - title
   - ways of working
   - DoR
   - DoD
3. Keep stage roles as draggable chips/cards inside the stage.
4. Add keyboard shortcuts for common actions:
   - `N` create stage
   - `E` focus first editable field on selected stage
   - `Backspace/Delete` remove selected stage or role after confirmation
   - arrow/tab navigation between stages and role chips
5. Reduce modal churn:
   - prefer inline stage editing
   - reserve popups for destructive confirmations and import/export

### Implementation notes

- Continue using the existing Workflow modal/editor surface rather than building a
  second admin flow.
- Maintain optimistic UI updates where safe, then reconcile with reloads from
  `/api/workflows/{id}`.
- Reuse the current stage-card and role-chip drag/drop patterns as the base.

### Definition of done

- full CRUD for Workflow + stage + stage-role assignment remains available
- stage and role ordering are both visual
- keyboard shortcuts exist for the highest-frequency actions
- Playwright covers create, edit, reorder, assign, remove

### Progress

Implemented in the current branch:

- Workflow stages now render in a more board-like workspace instead of a plain
  vertical list
- stage cards and role chips both support visual ordering
- stage/role selection is explicit and visible
- keyboard shortcuts now support:
  - `N` focus the new-stage composer
  - `E` focus the selected stage title field
  - `Delete` / `Backspace` remove the selected role or stage after confirmation
  - `←` / `→` move between stages
  - `↑` / `↓` move through roles in the selected stage

Still open inside phase 1:

- broader keyboard shortcuts for save/assign flows
- more discoverable hover/focus affordances
- fuller Playwright coverage for destructive shortcut paths

## Phase 2 — backlog view showing ticket position in the Workflow

### Outcome

An admin can see the backlog as work flowing through the effective Workflow, not
just as flat cards in a single board lane list.

### UX direction

Build a dedicated **backlog** perspective that complements the current board:

1. Group tickets by **effective Workflow** first.
2. Within each Workflow, show ordered stage lanes.
3. Within each stage, show the role sequence and the ticket’s current role
   position when available.
4. Make the view useful for both unstarted and active work:
   - draft
   - idle
   - active
   - success/fail
5. Support quick filters:
   - project
   - Workflow
   - stage
   - role
   - draft/archived/completed

### Data derivation

This phase should **not** need new endpoints.

- tickets come from `/api/projects/{id}/tickets`
- Workflow definitions come from `/api/workflows` + `/api/workflows/{id}`
- effective Workflow is derived in the client:
  1. ticket Workflow override
  2. nearest parent ticket Workflow override
  3. project default Workflow

### Definition of done

- backlog perspective exists beside the current board
- ticket position in Workflow is visible without opening the ticket modal
- filters are keyboard reachable and low-friction
- drag/drop or shortcut movement preserves current ticket update APIs

### Progress

Implemented in the current branch:

- added a dedicated **backlog** perspective beside the board and hierarchy views
- grouped tickets by effective Workflow in the browser using existing data only:
  - ticket Workflow override
  - nearest parent ticket Workflow override
  - project default Workflow
- rendered ordered stage lanes per Workflow with visible role tracks
- surfaced quick filters for Workflow, stage, role, and ticket status
- preserved drag/drop stage movement using the existing ticket update API

Still open inside phase 2:

- richer inline summaries for draft/archived/completed state at the lane level
- optional keyboard shortcuts for moving between backlog filters and lanes
- broader backlog regression coverage around drag/drop between lanes

## Phase 3 — history / ghost view

### Outcome

An admin can inspect a ticket’s journey through stages and roles in a
"mario-kart ghost" style replay/timeline.

### UX direction

1. Add a **history** view for a selected ticket.
2. Render the Workflow path as an ordered track:
   - stages laid out in sequence
   - roles shown within each stage
   - event markers placed where the ticket moved, failed, succeeded, commented,
     or changed
3. Support two interaction modes:
   - click an event in the timeline to inspect metadata
   - scrub/step through events chronologically
4. Allow switching between:
   - ticket-only history
   - project history filtered to a ticket

### Data source

- `/api/tickets/{id}/history`
- `/api/projects/{id}/history`
- current ticket + Workflow detail to map events onto stages/roles

### Rendering strategy

Start with a 2D timeline/track inside the existing web UI before considering any
heavier animation work. The priority is clarity and responsiveness, not visual
effects for their own sake.

### Definition of done

- a selected ticket can open a history replay view
- stage progression is visible spatially, not just as a text log
- users can inspect event metadata and comments/actions at each step
- the view works entirely from current APIs

### Progress

Implemented in the current branch:

- tickets now expose a **History** action in the web ticket modal
- the history modal renders the ticket journey as a staged track using the
  current Workflow layout when available
- users can step backward/forward through events or click any event marker
- event payload details are visible inline for inspection
- the view can switch between:
  - ticket-only history from `/api/tickets/{id}/history`
  - project history filtered to the current ticket from `/api/projects/{id}/history`

Still open inside phase 3:

- richer animation/polish for the replay path
- denser comment/action rendering for long project streams
- optional keyboard shortcuts for stepping through events

## Cross-cutting polish work

These apply across all phases:

1. Keep interaction latency low:
   - optimistic UI where practical
   - debounced persistence where appropriate
   - avoid redundant reload storms
2. Prefer direct manipulation:
   - drag/drop
   - inline editing
   - simple single-key shortcuts
3. Preserve discoverability:
   - visible buttons for mouse users
   - shortcut hints in headers/tooltips
4. Extend Playwright as each phase lands, especially around drag/drop and
   keyboard workflows.

## Execution order

1. finish phase-1 Workflow authoring polish around shortcuts, selection model, and
   reduced friction
2. add the backlog perspective using existing ticket + Workflow APIs
3. build the history/ghost view on top of ticket/project history APIs
4. refine performance and keyboard interactions after each slice, not only at
   the end

## Risks / watchouts

- the browser must derive effective Workflow correctly when parent lineage is
  involved
- drag/drop can become brittle without focused Playwright coverage
- history events may not always encode stage/role transitions directly, so the
  view may need to infer some positioning from event metadata and current Workflow
  structure
- phase 2 and phase 3 should avoid creating parallel concepts that duplicate the
  current board and hierarchy views without adding real clarity

## Site2 parity track

The work has now split into two website surfaces:

- `default` keeps the original embedded site
- `site2` is the fresh replacement surface selected with `tk server -site site2`

Current `site2` progress:

1. a new shell exists with separate navigation for tickets, projects, Workflows,
   roles, agents, and teams
2. ticket board interactions use the existing ticket APIs, including drag/drop
   stage movement, editing, draft toggles, history, lifecycle actions, and Workflow
   override
3. project CRUD uses `/api/projects`, `/api/projects/{id}`, and
   `/api/projects/{id}/set-draft`
4. Workflow CRUD and stage CRUD use `/api/workflows`, `/api/workflows/{id}`,
   `/api/workflows/{id}/stages`, and `/api/workflows/stages/{stageId}`
5. stage-role assignment in `site2` uses the existing
   `/api/workflows/stages/roles/...` endpoints
6. role, agent, and team CRUD now have dedicated editors in `site2`

Still open on the site2 track:

- richer keyboard shortcuts across the new shell
- stage-role drag/reorder polish beyond add/remove flows
- deeper relationship management for project/team membership surfaces
- broader browser coverage as more parity work lands

Latest regression coverage added for `site2`:

- project creation + default draft persistence
- ticket board drag/drop stage movement
- Workflow stage-role assignment

## Initial todos

1. Phase 1: smooth Workflow/stage/role authoring workspace
2. Phase 2: backlog perspective with Workflow-aware ticket position
3. Phase 3: ticket history ghost/timeline view
4. Shared: shortcuts, drag/drop polish, and Playwright coverage
