# Ticket Lifecycle Model

This document describes the desired lifecycle model for tickets.

In this case I dont want to create the logic of orchesrtator (YET) - I want to ensure I have the whole CRUD management that works so that all entities can be created and managed.  the orchestrator of automatically moving
tickets around is for later.  If the datamodel, apis and CLI are all working, along with all docs, that is the first goal met.

---

## 1. Project

A project is the top-level container in which all tickets and the Workflow are stored.

- Has a **title**, **description**, **prefix** (1-5 uppercase ASCII letters for ticket IDs).
- Has exactly one **workflow** which describes how a ticket "progreses" through its lifecycle
- A workflow can be exported/imported as JSON to share across projects.

A project carries workflow defaults and guidance, and new tickets start in draft
mode until they are explicitly readied for work.

```bash
tk project new -title "My Project"
tk project ls
export TICKET_PROJECT=<prefix>
tk project workflow <workflow_id>
```

---

## 2. workflow

A workflow defines the ordered sequence of **stages** a ticket moves through, and the **roles** involved at each stage.

- A workflow has a **name** and **description**.
- A workflow has one or more **stages**, ordered.
- A workflow has one or more **roles**.
- Each stage can have one or more roles assigned to it.

```bash
tk admin workflow ls
tk admin workflow create -name "Agile v1.0" -d "Standard agile process"
tk admin workflow get -id <workflow_id>
tk admin workflow export -id <workflow_id> -o workflow_agile.json
tk admin workflow import -file workflow_agile.json
tk project workflow <workflow_id>
```

---

## 3. Stage

A stage represents a phase of work within an workflow.  Visually they would be rendered as swimlanes or columns in a kanban board.

- The **default** stage (the one which tickets are created in) is the first stage.
- Has a **name** (e.g. `idea`, `refine`, `ready`, `develop`, `complete`).
- Has an optional **description** 
- Has an optional **acceptance criteria** explaining the purpose and expected outcome.
- Has an **order** within its workflow.
- Can have one or more **roles** assigned to it, which are ordered within the stage

Example stages: `idea` -> `refine` -> `ready` -> `develop` -> `complete`.

### Backlog stages

The first three stages (`idea`, `refine`, `ready`) are **backlog stages**. A ticket in a backlog stage:
- Starts at `idea` when created.
- Progresses `idea` → `refine` → `ready` via `tk next` (requires `state=success` at each step).
- **Cannot** advance past `ready` until the ticket is assigned to an open sprint.
- Once in a sprint, `tk next` advances from `ready` to the first sprint workflow stage (e.g. `develop`).

```bash
tk workflow stage-list -id <workflow_id>
tk workflow stage-add -id <workflow_id> -name develop
tk workflow stage-get -stage-id <stage_id>
tk workflow stage-update -stage-id <stage_id> -name MyName
tk workflow stage-update -stage-id <stage_id> -d "My description"
tk workflow stage-update -stage-id <stage_id> -ac "My acceptance criteria"
tk workflow stage-rm -stage-id <stage_id>
tk workflow stage-order -id <workflow_id> <stage_id1,stage_id2,stage_id3>
```

The default bootstrap workflow has five stages: `idea`, `refine`, `ready`, `develop`, `complete`.

`develop`: represents the stage where sprint work is carried out.
`complete`: the terminal stage for accepted work — ticket is shipped and done.
`reject`: an optional terminal stage for rejected sprint tickets. Not in the default linear chain; tickets are moved here manually when a sprint ticket will not be completed.

The **backlog board** shows only the three backlog stages: `idea`, `refine`, `ready`.
The **sprint board** shows: `ready`, `develop`, `complete`, `reject` (in that order).

Sprint assignment rules:
- A ticket must be in `ready` stage before it can be added to a sprint (`SetTicketSprint`).
- A sprint cannot be activated if any of its tickets are in `idea` or `refine` stage.

---

## 4. Role

A role represents a job function within an Workflow (e.g. architect, engineer, tester, product owner).

- Has a **title**, **description**, and **acceptance criteria**.
- Roles are defined per workflow and assigned to stages.

```
tk workflow role-list -workflow_id <workflow_id>
tk workflow role-add -workflow_id <workflow_id> -title "Architect" -description "..." -ac "..."
tk workflow role-get -workflow_id <workflow_id> -role_id <role_id>
tk workflow role-update -workflow_id <workflow_id> -role_id <role_id> -title "Senior Architect"
tk workflow role-update -workflow_id <workflow_id> -role_id <role_id> -description "Performs reviews and ensures alignment"
tk workflow role-update -workflow_id <workflow_id> -role_id <role_id> -ac "Review the ticket and ensure it is aligned with the ..."
tk workflow role-rm -workflow_id <workflow_id> -role_id <role_id>
```

A role can be associated with a stage

```
tk workflow stage-role-add -workflow_id <workflow_id> -stage_id <stage_id> -role_id <role_id>
tk workflow stage-role-rm -workflow_id <workflow_id> -stage_id <stage_id> -role_id <role_id>
tk workflow stage-role-order -workflow_id <workflow_id> -stage_id <stage_id> -roles <role_id1,role_id2,role_id3>
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
| **stage** | stage_id | Defined by workflow (example: `design`, `develop`, `test`, `done`) | Where in the workflow the ticket sits |
| **role** | role_id | The current role attached to this ticket |
| **draft** | bool | (true/false): "not yet ready to be worked on, a human is still curating it." |
| **state** | text | `idle`, `active`, `success`, `fail` | Progress within the current stage |
| **archived** | bool | `true`, `false` | Soft-delete, hides from listings |
| **complete** | bool | (true/false): "totally finished with no more work" — when true, stage is always "done". |

Closing a ticket moves it's `complete` column to true and moves the stage to done.  It does not change the success/fail state.

Archiving a ticket just marks it as archived - which means it is no longer visible and will not accept changes unless it is un-archived.

---

## 6. State Transitions

Let's take an example Workflow, "Agile v1.0"

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

This means there would be a minimum of 6 different steps involved in completing the workflow described in "Agile v1.0".   If every step was a success, then it would flow through in sequence:

    1. Design/Product Owner
    2. Design/Business Analyst
    3. Design/Architect
    4. Develop/Engineer
    5. Test/QA
    6. UAT/Product Owner
    7. DONE

The workflow rules (the Workflow itself) then has an implied "next(), current(), last()" functions - which tell us the Stage and Role a ticket should be moved to, where next() is based on success, current() reports where it currently is and "last()" would look at the prior role, or the prior stage.

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
| `tk project set-workflow -project_id <project_id> -workflow_id <workflow_id>` | Attach Workflow to project |

### Workflow

| Command | Effect |
|---------|--------|
| `tk admin workflow list` | List all Workflows |
| `tk admin workflow create -name <n> -d <desc>` | Create a Workflow |
| `tk admin workflow get -workflow_id <workflow_id>` | Show Workflow with stages and roles |
| `tk admin workflow export -workflow_id <workflow_id> -o <file>` | Export Workflow to JSON |
| `tk admin workflow import -f <file>` | Import Workflow from JSON |

### Stages

| Command | Effect |
|---------|--------|
| `tk workflow stage-list -workflow_id <workflow_id>` | List stages |
| `tk workflow stage-add -workflow_id <workflow_id> -name <name>` | Add a stage |
| `tk workflow stage-get -workflow_id <workflow_id> -stage_id <stage_id>` | Show stage detail |
| `tk workflow stage-update -workflow_id <workflow_id> -stage_id <stage_id> ...` | Update stage (title, description, ac) |
| `tk workflow stage-rm -workflow_id <workflow_id> -stage_id <stage_id>` | Remove a stage |
| `tk workflow stage-order -workflow_id <workflow_id> -stages <stage_id1,stage_id2,...>` | Reorder stages |

### Roles

| Command | Effect |
|---------|--------|
| `tk workflow role-list -workflow_id <workflow_id>` | List roles |
| `tk workflow role-add -workflow_id <workflow_id> -title <t> -description <d> -ac <ac>` | Add a role |
| `tk workflow role-get -workflow_id <workflow_id> -role_id <role_id>` | Show role detail |
| `tk workflow role-update -workflow_id <workflow_id> -role_id <role_id> ...` | Update role (title, description, ac) |
| `tk workflow role-rm -workflow_id <workflow_id> -role_id <role_id>` | Remove a role |

### Stage-Role Assignment

| Command | Effect |
|---------|--------|
| `tk workflow stage-role-add -workflow_id <workflow_id> -stage_id <stage_id> -role_id <role_id>` | Assign role to stage |
| `tk workflow stage-role-rm -workflow_id <workflow_id> -stage_id <stage_id> -role_id <role_id>` | Remove role from stage |
| `tk workflow stage-role-order -workflow_id <workflow_id> -stage_id <stage_id> -roles <role_id1,role_id2,...>` | Reorder roles within stage |

---

## 9. Implementation Plan

The refactor from the current model to this target model:

1. **Rename `workflow` to `workflow`** throughout codebase (DB tables, Go types, CLI commands, API endpoints).
2. **Replace `open` field with `complete`** (inverted semantics). `tk close` sets complete=true + stage=done.
3. **Replace `ready` field with `draft`** (inverted semantics). `tk draft` / `tk undraft`.
4. **Add `role` field to tickets** — references the current active role within the stage.
5. **Add stage-role junction table** — supports multiple ordered roles per stage (replaces single `role_id` on `workflow_stages`).
6. **Drop `status` column** — compute `stage/state` at read time only.
7. **`tk state <ticket_id> <stage>`** sets the role to the first role in that stage's role order.
8. **Document `fail -> idle` as the retry path** in CLI help.
9. **Orchestration (future)** — `next()`, `current()`, `last()` functions for automated stage/role advancement.
