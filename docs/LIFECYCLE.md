# Ticket Lifecycle Model

This document describes the desired lifecycle model for tickets.

In this case I dont want to create the logic of orchesrtator (YET) - I want to ensure I have the whole CRUD management that works so that all entities can be created and managed.  the orchestrator of automatically moving
tickets around is for later.  If the datamodel, apis and CLI are all working, along with all docs, that is the first goal met.

---

## 1. Project

A project is the top-level container in which all tickets and the SDLC are stored.

- Has a **title**, **description**, **prefix** (2-5 uppercase letters for ticket IDs).
- Has exactly one **sdlc** which describes how a ticket "progreses" through its lifecycle
- A sdlc can be exported/imported as JSON to share across projects.

A project has defaults for new commands
    draft: (bool) - if true, new tickets start in draft mode.

```
tk project create "My Project"
tk project list
tk project use <prefix>
tk project set-sdlc -project_id X -sdlc_id Y
tk project set-draft true|false
```

---

## 2. sdlc

A sdlc defines the ordered sequence of **stages** a ticket moves through, and the **roles** involved at each stage.

- A sdlc has a **name** and **description**.
- A sdlc has one or more **stages**, ordered.
- A sdlc has one or more **roles**.
- Each stage can have one or more roles assigned to it.

```
tk sdlc list
tk sdlc create -name "Agile v1.0" -d "Standard agile process"
tk sdlc get -sdlc_id <sdlc_id>
tk sdlc export -sdlc_id <sdlc_id> -o sdlc_agile.json
tk sdlc import -f sdlc_agile.json
tk project set-sdlc -project_id X -sdlc_id Y
```

---

## 3. Stage

A stage represents a phase of work within an sdlc.  Visually they would be rendered as swimlanes or columns in a kanban board.

- The **default** stage (the one which tickets are created in) is the first stage.
- Has a **name** (e.g. `design`, `develop`, `test`, `done`).
- Has an optional **description** 
- Has an optional **acceptance criteria** explaining the purpose and expected outcome.
- Has an **order** within its sdlc.
- Can have one or more **roles** assigned to it, which are ordered within the stage

Example stages: `design` -> `develop` -> `test` -> `done`.

```
tk sdlc stage-list -sdlc_id <sdlc_id>
tk sdlc stage-add -sdlc_id <sdlc_id> -name develop
tk sdlc stage-get -sdlc_id <sdlc_id> -stage_id <stage_id>
tk sdlc stage-update -sdlc_id <sdlc_id> -stage_id <stage_id> -name MyName
tk sdlc stage-update -sdlc_id <sdlc_id> -stage_id <stage_id> -description "My description"
tk sdlc stage-update -sdlc_id <sdlc_id> -stage_id <stage_id> -ac "My acceptance criteria"
tk sdlc stage-rm -sdlc_id <sdlc_id> -stage_id <stage_id>
tk sdlc stage-order -sdlc_id <sdlc_id> -stages <stage_id1,stage_id2,stage_id3>
```

As a minimum in any SDLC there are TWO stages - "develop" and "done".   

Develop: represents the stage where work is carried out.
Done: represents the final stage of any work where it is completely finished.
    - Done is the final place a story goes when it is closed.
    - Done does not have any roles as no work is carried out there.
    - Done cannot have any roles assigned to it - it is the final place a ticket goes when it is closed.

---

## 4. Role

A role represents a job function within an SDLC (e.g. architect, engineer, tester, product owner).

- Has a **title**, **description**, and **acceptance criteria**.
- Roles are defined per sdlc and assigned to stages.

```
tk sdlc role-list -sdlc_id <sdlc_id>
tk sdlc role-add -sdlc_id <sdlc_id> -title "Architect" -description "..." -ac "..."
tk sdlc role-get -sdlc_id <sdlc_id> -role_id <role_id>
tk sdlc role-update -sdlc_id <sdlc_id> -role_id <role_id> -title "Senior Architect"
tk sdlc role-update -sdlc_id <sdlc_id> -role_id <role_id> -description "Performs reviews and ensures alignment"
tk sdlc role-update -sdlc_id <sdlc_id> -role_id <role_id> -ac "Review the ticket and ensure it is aligned with the ..."
tk sdlc role-rm -sdlc_id <sdlc_id> -role_id <role_id>
```

A role can be associated with a stage

```
tk sdlc stage-role-add -sdlc_id <sdlc_id> -stage_id <stage_id> -role_id <role_id>
tk sdlc stage-role-rm -sdlc_id <sdlc_id> -stage_id <stage_id> -role_id <role_id>
tk sdlc stage-role-order -sdlc_id <sdlc_id> -stage_id <stage_id> -roles <role_id1,role_id2,role_id3>
```

---

## 5. Ticket

A ticket is a unit of work within a project.

### 5.1 Core Fields

| Field | Type | Purpose |
|-------|------|---------|
| **type** | text | `epic`, `task`, `bug`, `spike`, `chore`, `story`, `note`, `question`, `requirement`, `decision` |
| **title** | text | Short description |
| **description** | text | Full details |
| **acceptance_criteria** | text | What "done" looks like |
| **parent_id** | ref | Parent ticket (epics contain children) |
| **assignee** | text | Who is working on it |
| **priority** | int | Ordering priority |

EPIC cannot be a child ticket.

### 5.2 Lifecycle Fields

| Field | Type | Values | Purpose |
|-------|------|--------|---------|
| **stage** | stage_id | Defined by sdlc (example: `design`, `develop`, `test`, `done`) | Where in the sdlc the ticket sits |
| **role** | role_id | The current role attached to this ticket |
| **draft** | bool | (true/false): "not yet ready to be worked on, a human is still curating it." |
| **state** | text | `idle`, `active`, `success`, `fail` | Progress within the current stage |
| **archived** | bool | `true`, `false` | Soft-delete, hides from listings |
| **complete** | bool | (true/false): "totally finished with no more work" — when true, stage is always "done". |

Closing a ticket moves it's `complete` column to true and moves the stage to done.  It does not change the success/fail state.

Archiving a ticket just marks it as archived - which means it is no longer visible and will not accept changes unless it is un-archived.

---

## 6. State Transitions

Let's take an example SDLC, "Agile v1.0"

5 Stages

Stage 1. Design
    role1: Product Owner
    role2: Business Analyst
    role3: Architect
Stage 2. Develop
    role4: Engineer
Stage 3. Test
    role5: QA
Stage 4. UAT
    role1: Product Owner
Stage 5. Done

This means there would be a minimum of 6 different steps involved in completing the sdlc described in "Agile v1.0".   If every step was a success, then it would flow through in sequence:

    1. Design/Product Owner
    2. Design/Business Analyst
    3. Design/Architect
    4. Develop/Engineer
    5. Test/QA
    6. UAT/Product Owner
    7. DONE

The sdlc rules (the SDLC itself) then has an implied "next(), current(), last()" functions - which tell us the Stage and Role a ticket should be moved to, where next() is based on success, current() reports where it currently is and "last()" would look at the prior role, or the prior stage.

#### Performing work / Orchestration

When we combine the ticket, the stage, the role, the parent tickets/epics acceptance criteria and descripitons, this shoudl give us a goal that is then provided to an LLM.   The outcome of that work is then assessed by the same ticket and it is decided to eithe rpass or fail.  If it passes, it moves onto the next step, if it fails, it moves back to the last.

### 6.1 Within a Stage

```
idle ──> active ──> success
OR
idle ──> active ──> fail
```

- `? -> idle`: does not require an assignee
- `idle -> active`: requires an assignee.
- `active -> success`: work completed for this stage.
- `active -> fail`: work failed for this stage.
- `fail -> idle` or `fail -> active`: retry (currently allowed but undocumented).

## 7. Commands Summary

### Ticket State & Stage

| Command | Effect |
|---------|--------|
| `tk state <ticket_id> <state_name>` | Change state within current stage/role |
| `tk stage <ticket_id> <stage_name>` | Move to stage (first role), reset state to idle |
| `tk idle <ticket_id>` | Shortcut: state -> idle |
| `tk active <ticket_id>` | Shortcut: state -> active |
| `tk success <ticket_id>` | Shortcut: state -> success |
| `tk fail <ticket_id>` | Shortcut: state -> fail |

### Ticket Lifecycle

| Command | Effect |
|---------|--------|
| `tk complete <ticket_id>` | Mark complete (complete=true, stage=done) |
| `tk reopen <ticket_id>` | Undo complete (complete=false, restores previous stage) |
| `tk close <ticket_id>` | Alias for `tk complete` |
| `tk draft <ticket_id>` | Mark as draft (not ready for work) |
| `tk undraft <ticket_id>` | Mark as not draft (ready for work) |
| `tk archive <ticket_id>` | Soft-delete, hide from listings |
| `tk unarchive <ticket_id>` | Restore from archive |
| `tk next <ticket_id>` | Moves the ticket to the next role or stage (if currently success), or returns failure |
| `tk previous <ticket_id>` | Moves the ticket to the last role or stage (if currently fail), or returns failure |

### Project

| Command | Effect |
|---------|--------|
| `tk project create "Title"` | Create a project |
| `tk project list` | List all projects |
| `tk project use <prefix>` | Switch active project |
| `tk project set-sdlc -project_id <project_id> -sdlc_id <sdlc_id>` | Attach SDLC to project |

### SDLC

| Command | Effect |
|---------|--------|
| `tk sdlc list` | List all SDLCs |
| `tk sdlc create -name <n> -d <desc>` | Create an SDLC |
| `tk sdlc get -sdlc_id <sdlc_id>` | Show SDLC with stages and roles |
| `tk sdlc export -sdlc_id <sdlc_id> -o <file>` | Export SDLC to JSON |
| `tk sdlc import -f <file>` | Import SDLC from JSON |

### Stages

| Command | Effect |
|---------|--------|
| `tk sdlc stage-list -sdlc_id <sdlc_id>` | List stages |
| `tk sdlc stage-add -sdlc_id <sdlc_id> -name <name>` | Add a stage |
| `tk sdlc stage-get -sdlc_id <sdlc_id> -stage_id <stage_id>` | Show stage detail |
| `tk sdlc stage-update -sdlc_id <sdlc_id> -stage_id <stage_id> ...` | Update stage (title, description, ac) |
| `tk sdlc stage-rm -sdlc_id <sdlc_id> -stage_id <stage_id>` | Remove a stage |
| `tk sdlc stage-order -sdlc_id <sdlc_id> -stages <stage_id1,stage_id2,...>` | Reorder stages |

### Roles

| Command | Effect |
|---------|--------|
| `tk sdlc role-list -sdlc_id <sdlc_id>` | List roles |
| `tk sdlc role-add -sdlc_id <sdlc_id> -title <t> -description <d> -ac <ac>` | Add a role |
| `tk sdlc role-get -sdlc_id <sdlc_id> -role_id <role_id>` | Show role detail |
| `tk sdlc role-update -sdlc_id <sdlc_id> -role_id <role_id> ...` | Update role (title, description, ac) |
| `tk sdlc role-rm -sdlc_id <sdlc_id> -role_id <role_id>` | Remove a role |

### Stage-Role Assignment

| Command | Effect |
|---------|--------|
| `tk sdlc stage-role-add -sdlc_id <sdlc_id> -stage_id <stage_id> -role_id <role_id>` | Assign role to stage |
| `tk sdlc stage-role-rm -sdlc_id <sdlc_id> -stage_id <stage_id> -role_id <role_id>` | Remove role from stage |
| `tk sdlc stage-role-order -sdlc_id <sdlc_id> -stage_id <stage_id> -roles <role_id1,role_id2,...>` | Reorder roles within stage |

---

## 9. Implementation Plan

The refactor from the current model to this target model:

1. **Rename `sdlc` to `sdlc`** throughout codebase (DB tables, Go types, CLI commands, API endpoints).
2. **Replace `open` field with `complete`** (inverted semantics). `tk close` sets complete=true + stage=done.
3. **Replace `ready` field with `draft`** (inverted semantics). `tk draft` / `tk undraft`.
4. **Add `role` field to tickets** — references the current active role within the stage.
5. **Add stage-role junction table** — supports multiple ordered roles per stage (replaces single `role_id` on `sdlc_stages`).
6. **Drop `status` column** — compute `stage/state` at read time only.
7. **`tk state <ticket_id> <stage>`** sets the role to the first role in that stage's role order.
8. **Document `fail -> idle` as the retry path** in CLI help.
9. **Orchestration (future)** — `next()`, `current()`, `last()` functions for automated stage/role advancement.
