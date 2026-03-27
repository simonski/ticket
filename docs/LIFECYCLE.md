# Ticket Lifecycle Model

This document describes the current ticket lifecycle as implemented. Its purpose is to inform a simplification refactor.

## Fields

A ticket has five lifecycle-related fields:

| Field | Type | Values | Stored? | Purpose |
|-------|------|--------|---------|---------|
| **stage** | text | `design`, `develop`, `test`, `done` | Yes | Where in the workflow the ticket sits |
| **state** | text | `idle`, `active`, `success`, `fail` | Yes | Progress within the current stage |
| **status** | text | `<stage>/<state>` e.g. `develop/active` | Yes (derived) | Rendered composite for display and filtering |
| **open** | bool | `true`, `false` | Yes | Whether the ticket accepts changes |
| **archived** | bool | `true`, `false` | Yes | Soft-delete — hides from all listings |
| **ready** | bool | `true`, `false` | Yes | "Definition of Ready" satisfied — eligible for work |

## Stage

Represents the workflow phase. Default progression: `design` -> `develop` -> `test` -> `done`.

- Set to `design` on creation (or first workflow stage if a custom workflow is attached).
- Changed via `tk stage <id> <stage>`.
- When stage changes, state resets to `idle` (unless moving to `done`, which forces `success`).
- Custom workflows can define different stage names and ordering.

## State

Represents progress within a stage. Constrained by the current stage:

- **design/develop/test**: `idle | active | success | fail`
- **done**: `success | fail` only

Transitions:
- `idle` -> `active` (requires an assignee)
- `active` -> `success` or `fail`
- `success` at a non-final stage **auto-advances** to the next stage with `idle`
- `success` at the final stage is terminal — ticket cannot be reopened via state change

Legacy alias: `complete` is normalized to `success`.

## Status

Not an independent field. Computed as `stage + "/" + state` on every read. Stored in the DB for query convenience but always overwritten on retrieval.

Examples: `design/idle`, `develop/active`, `test/fail`, `done/success`.

## Open / Closed

Binary gate controlling whether a ticket accepts any mutations:

- Tickets are `open=true` on creation.
- `tk close <id>` sets `open=false`. No further updates allowed (state, stage, comments, assignments).
- `tk open <id>` re-opens.
- **Exception**: an explicit stage override on a closed ticket will reopen it (e.g. dragging on a board).
- Closing does NOT change stage/state — a ticket can be closed at `develop/active`.

## Archived

Orthogonal soft-delete:

- `tk archive <id>` hides the ticket from all list queries.
- `tk unarchive <id>` restores it.
- Archived tickets cannot be updated or commented on (hard block).
- Independent of open/closed and stage/state.

## Ready

A boolean flag indicating the ticket meets its "Definition of Ready":

- `tk ready <id>` / `tk notready <id>` toggles.
- Defaults to `false` on creation.
- Used in project health calculations (count of not-ready tickets = blocked work).
- Workflow stages can have a `definition_of_ready` text field describing what "ready" means for that stage.

## Current Overlap and Complexity

The model has six fields tracking lifecycle but several overlap in purpose:

1. **status is redundant** — it is always `stage/state` and never set independently.
2. **open vs stage/state** — closing a ticket is conceptually "this work is finished" but doesn't move the ticket to `done/success`. A ticket can be `design/idle` and closed, which is semantically confusing.
3. **archived vs closed** — both prevent mutations. The distinction is visibility (archived hides, closed just locks). In practice users close then archive, making closed a halfway state with unclear value.
4. **ready is stage-agnostic** — it doesn't reset when stage changes, so a ticket marked ready in `design` stays ready in `develop` even though the definition of ready may differ per stage.
5. **state auto-advance** — `success` at non-final stages silently changes the stage, which can surprise users who expected to stay in the current stage.
6. **fail has no recovery path** — there's no documented way to move from `fail` back to `active` or `idle` within the same stage without an explicit state change.

## Commands Summary

| Command | Effect |
|---------|--------|
| `tk stage <id> <stage>` | Move to stage, reset state to idle |
| `tk state <id> <state>` | Change state within current stage |
| `tk idle <id>` | Shortcut: state -> idle |
| `tk active <id>` | Shortcut: state -> active (needs assignee) |
| `tk complete <id>` | Shortcut: state -> success (may auto-advance stage) |
| `tk fail <id>` | Shortcut: state -> fail |
| `tk open <id>` | Re-open a closed ticket |
| `tk close <id>` | Close — prevent all mutations |
| `tk archive <id>` | Hide from listings, prevent all mutations |
| `tk unarchive <id>` | Restore to listings |
| `tk ready <id>` | Mark as meeting definition of ready |
| `tk notready <id>` | Mark as blocked / not ready |
