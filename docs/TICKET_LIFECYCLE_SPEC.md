# Ticket Lifecycle Spec

## Purpose

This document is the authoritative specification for remodeling ticket workflow
from the current single `status` field into an explicit `stage` + `state`
lifecycle model.

It defines:

- the ticket lifecycle domain model
- parent/child derivation rules
- CLI behavior
- API behavior
- database/storage changes
- service/backend recalculation rules
- migration requirements
- test requirements

This spec supersedes older documentation that described ticket progress using a
single status field. The supported external lifecycle vocabulary is now only
`stage`, `state`, and rendered `status` in `<stage>/<state>` form.

## Scope

This applies to:

- documentation
- CLI
- server/API
- service/backend logic
- data model
- database schema and migrations
- tests
- web UI behavior

## Terminology

### Ticket

A ticket is a piece of work. Supported ticket types:

- `epic`
- `task`
- `bug`

### Parenting

`parent_id` expresses containment.

Allowed parent/child relationships:

- `epic` may parent `epic | task | bug`
- `task` may parent `task | bug`
- `bug` may not parent anything

### Leaf Ticket

A leaf ticket is any ticket with no children.

Leaf tickets may be changed explicitly by lifecycle commands.

### Parent Ticket

A parent ticket is any ticket with one or more children.

Parent tickets do not accept direct lifecycle mutation. Their effective
`stage`, `state`, and rendered `status` are derived from descendants.

## Lifecycle Model

### Stage

Stage represents the high-level swimlane:

- `design`
- `develop`
- `test`
- `done`

Semantic meaning:

- `design`: the ticket is being appraised and refined
- `develop`: implementation is being performed
- `test`: the outcome is being verified/appraised
- `done`: the ticket is concluded as complete

### State

State represents progress within a stage:

- `idle`: ready but not currently in progress
- `active`: currently being worked on with a named assignee
- `complete`: work for the current stage is complete

Allowed states by stage:

- `design`: `idle | active | complete`
- `develop`: `idle | active | complete`
- `test`: `idle | active | complete`
- `done`: `complete`

### Rendered Status

`status` is not stored independently.

It is rendered as:

- `<stage>/<state>`

Examples:

- `design/idle`
- `develop/active`
- `test/complete`
- `done/complete`

## Invariants

These are hard lifecycle invariants:

- `state=active` requires `assignee != ""`
- `state=idle` may be unassigned
- `state=complete` may retain assignee for audit/history
- `stage=done` requires `state=complete`
- `stage!=done` allows `idle | active | complete`

## Direct Lifecycle Mutation Rules

Only leaf tickets may be explicitly moved by stage/state commands.

If a ticket has children, direct lifecycle mutation must fail with:

```text
ticket has children; stage/state is derived from descendants
```

## Leaf Ticket Lifecycle Commands

### Create

On ticket creation:

- `stage = design`
- `state = idle`

Representative command:

```bash
ticket create ...
```

Result:

- returns new ticket id

### Stage Commands

Stage commands mutate leaf tickets only.

```bash
ticket design <id>
ticket develop <id>
ticket test <id>
ticket done <id>
```

Behavior:

- `ticket design <id>` => `stage=design`, `state=idle`
- `ticket develop <id>` => `stage=develop`, `state=idle`
- `ticket test <id>` => `stage=test`, `state=idle`
- `ticket done <id>` => `stage=done`, `state=complete`

### State Commands

State commands mutate leaf tickets only.

```bash
ticket idle <id>
ticket active <id>
ticket complete <id>
```

Behavior:

- `ticket idle <id>` => keep current stage, set `state=idle`
- `ticket active <id>` => keep current stage, set `state=active`
- `ticket complete <id>` => keep current stage, set `state=complete`

Notes:

- `ticket active <id>` must fail when no assignee is set
- `ticket complete <id>` is valid for `design`, `develop`, and `test`
- `ticket done <id>` remains the canonical way to move a ticket into terminal
  `done/complete`

## Parent Ticket Derivation

If a ticket has children, its effective lifecycle is derived recursively from
all descendants.

### Effective Stage

Effective parent stage is the earliest stage of any descendant.

Stage ordering:

- `design < develop < test < done`

Examples:

- if any descendant is in `design`, parent effective stage is `design`
- if descendants are only in `test` and `done`, parent effective stage is `test`
- parent reaches `done` only if all descendants are in `done`

### Effective State

Effective parent state is derived as:

- `complete` if all descendants are complete
- `active` if any descendant is active
- `idle` otherwise

This means:

- when a leaf moves to `active`, all ancestors become effectively `active`
- when all descendants are complete, the parent becomes effectively `complete`
- otherwise, the parent is effectively `idle`

## Effective vs Stored Lifecycle

For the refactor, use the following rule:

- leaf tickets: stored `stage/state` are authoritative
- parent tickets with children: effective `stage/state` are authoritative

The system should expose only effective lifecycle values to normal CLI/API
consumers.

Implementation may still persist raw `stage/state` columns on every row for
schema uniformity, but parent values are recomputed from descendants whenever
relevant descendants change.

## CLI Contract

### Commands to Keep/Add

Required lifecycle commands:

- `ticket create`
- `ticket design <id>`
- `ticket develop <id>`
- `ticket test <id>`
- `ticket done <id>`
- `ticket idle <id>`
- `ticket active <id>`
- `ticket complete <id>`

### Commands to Remove or Rework

The following single-status commands are obsolete and must be removed from the
external command surface:

- `ticket open`
- `ticket ready`
- `ticket inprogress`
- `ticket fail`

`ticket update -status ...` remains valid only when `status` is expressed as a
rendered lifecycle value such as `develop/active`.

### Listing and Detail Views

`ticket list` and `ticket get` must show:

- `stage`
- `state`
- rendered `status` (`stage/state`)

Filtering/searching must support:

- `--stage`
- `--state`
- optionally `--status <stage/state>` as a convenience parser

`--status <legacy-status>` must not be supported after the migration is
complete.

## API Contract

### Ticket Shape

Ticket responses must expose:

- `stage`
- `state`
- `status`

where:

- `status` is rendered from `stage/state`
- values returned are effective values

### Mutation Endpoints

Ticket mutation APIs must support:

- create with lifecycle defaults
- set stage
- set state
- update assignee independently

The server must reject:

- illegal stage/state combinations
- `active` with empty assignee
- direct lifecycle mutation on parent tickets with children
- invalid parent/child type relationships

### Suggested API Direction

Preferred API shape:

- `POST /api/tickets`
- `PUT /api/tickets/{id}`
- explicit lifecycle actions:
  - `POST /api/tickets/{id}/design`
  - `POST /api/tickets/{id}/develop`
  - `POST /api/tickets/{id}/test`
  - `POST /api/tickets/{id}/done`
  - `POST /api/tickets/{id}/idle`
  - `POST /api/tickets/{id}/active`
  - `POST /api/tickets/{id}/complete`

If the API keeps a generic update endpoint, it must still enforce lifecycle
rules centrally in the service layer.

## Database Contract

### Schema Change

Replace the old single-status model with:

```sql
stage TEXT NOT NULL
state TEXT NOT NULL
```

Do not store composite `status`.

### Task Table

The `tasks` table must evolve from:

- `status TEXT`

to:

- `stage TEXT NOT NULL`
- `state TEXT NOT NULL`

### Constraints

The model must enforce:

- `stage IN ('design','develop','test','done')`
- `state IN ('idle','active','complete')`
- not allowed: `stage='done' AND state!='complete'`

Where DB-level enforcement is awkward, the same invariants must be enforced in
the service layer.

### Parent Recalculation

Parent lifecycle values may be persisted in the same columns after
recalculation.

The source of truth is still the derivation rules in this spec.

## Service/Backend Rules

Every mutation that may affect lifecycle must:

1. validate the requested change
2. load the current ticket and its parent chain
3. reject direct lifecycle changes for parent tickets with children
4. update the target leaf ticket
5. walk ancestors upward
6. recalculate each ancestor's effective `stage/state`
7. persist changed parent rows
8. emit history events for direct and derived changes

### Recalculation Algorithm

For a given parent:

1. load all descendants recursively
2. derive effective stage from earliest descendant stage
3. derive effective state:
   - `complete` if all descendants are complete
   - `active` if any descendant is active
   - `idle` otherwise
4. persist if changed

## History/Audit Requirements

History events must distinguish:

- direct lifecycle changes
- derived parent lifecycle changes

Recommended event types:

- `ticket_created`
- `ticket_stage_changed`
- `ticket_state_changed`
- `ticket_assignee_changed`
- `ticket_parent_changed`
- `ticket_lifecycle_derived`

Payloads should include old and new values where applicable.

## Migration Plan

### Phase 1: Introduce lifecycle vocabulary

- add lifecycle constants and validation helpers
- add spec docs
- add tests for lifecycle validation and stage ordering

### Phase 2: Schema expansion

- add `stage` and `state` columns
- backfill from existing `status`
- keep old `status` column temporarily as a rendered/cache field during migration

Suggested migration mapping from historical databases:

- `notready` -> `design/idle`
- `open` -> `design/idle`
- `inprogress` -> `develop/active`
- `complete` -> `done/complete`
- `fail` -> `test/complete`

This mapping is transitional only and must not remain the long-term model.

### Phase 3: Service/API cutover

- make service layer use `stage/state`
- expose effective lifecycle in API responses
- implement parent recalculation

### Phase 4: CLI cutover

- replace single-status commands and help text
- add stage/state commands and filters
- update output rendering

### Phase 5: Remove legacy status compatibility

- remove old single-status aliases and parsers from CLI/API edges
- remove old docs
- remove old tests
- optionally drop the `status` DB column in a final migration if it is no
  longer needed as a stored rendered field

## Testing Requirements

### Unit Tests

Must cover:

- valid stage values
- valid state values
- valid stage/state combinations
- invalid stage/state combinations
- stage ordering
- parent/child type validation

### Store/Service Tests

Must cover:

- create defaults to `design/idle`
- `active` requires assignee
- `done` forces `complete`
- stage commands reset state to `idle` except `done`
- parent lifecycle is derived, not direct
- parent stage derives from earliest descendant stage
- parent state derives from descendant activity/completion
- recursive derivation over deep trees
- parent lifecycle recalculates after child lifecycle change

### API Tests

Must cover:

- create responses include `stage`, `state`, `status`
- parent tickets reject direct lifecycle mutation
- invalid lifecycle transitions return validation errors
- lifecycle action endpoints update descendants/ancestors correctly

### CLI Tests

Must cover:

- `ticket design|develop|test|done`
- `ticket idle|active|complete`
- list/get render effective `stage/state/status`
- attempts to mutate parent lifecycle fail with the expected message
- filtering by `--stage` and `--state`

### Migration Tests

Must cover:

- old databases migrate safely
- historical single-status values backfill deterministically
- recalculation after migration produces valid parent lifecycle values

## Refactor Order

Recommended implementation order:

1. add lifecycle constants/helpers
2. land schema changes and migrations
3. update store/service model
4. update API layer
5. update CLI
6. update docs
7. update UI
8. remove legacy single-status compatibility code

## Non-Goals for First Pass

The following are explicitly out of scope for the first implementation pass:

- manual lifecycle overrides on parent tickets
- workflow automation between stages beyond explicit commands
- separate raw vs effective lifecycle fields in API responses
- preserving old status command aliases indefinitely
