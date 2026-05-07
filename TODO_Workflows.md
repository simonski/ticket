# Workflow Refactor Specification (Draft)

## 1. Purpose

Replace the current **Workflow** concept with **Workflow**, then evolve that model so work can be executed primarily by agents with a human as accountable owner.

The Workflow system should produce software outcomes that are:

1. well-defined and understood
2. compliant with standards, best practice, and legal/industry obligations
3. sustainable, extendable, and reproducible

## 2. Core Idea

A Workflow is an ordered sequence (or DAG) of **Phases** and **Roles** that defines:

1. what objective must be achieved
2. who/what evaluates it
3. how work advances or returns for rework

Tickets move through this structure. Agents/humans execute assigned work items and report outcomes.

## 3. Product Goal in `tk`

Enable a user to:

1. start with an idea (text/docs/codebase)
2. refine that idea into a specification through Q+A
3. implement against the specification
4. formally verify and approve the outcome

## 4. Entity Model

### Workflow
- id
- title
- description
- motivation
- phases[] (ordered)

### Phase
- id
- workflow_id
- title
- description
- objective
- order_index
- roles[] (ordered or DAG edges)

### Role
- id
- phase_id
- title
- description
- motivation
- responsibility
- pass_criteria

### Project
- id
- title
- description
- workflow_id
- git_repo
- tickets[]

### Ticket
- id
- project_id
- title
- description
- current_work_item_id
- history[]

### WorkItem (renamed from TicketHistory operation)
- id (UUID)
- ticket_id
- phase_id
- role_id
- objective_snapshot
- prompt_snapshot
- status (`idle | active | success | fail`)
- assignee_type (`agent | human`)
- assignee_id
- feedback
- commit_ref
- started_at
- completed_at

### TicketHistory
- id
- ticket_id
- work_item_id
- datetime
- event
- payload

## 5. Lifecycle Semantics

1. A Workflow has one or more Phases (for example: `design -> implement -> review`).
2. A Phase has one or more Roles.
3. All Roles in a Phase must approve before work can move forward.
4. A Role approves or rejects the artifact for that phase objective.
5. Work advances only when required Role checks pass.
6. On failure, progression stops and requires accountable human intervention.
7. The human decision is recorded as TicketHistory and may:
   - send work back to a specific Role
   - send work back to a Phase
   - create a refinement/new WorkItem

## 6. Prompt Contract

Every executable unit should have a deterministic prompt generated from:

- phase
- role
- objective
- ticket context
- acceptance criteria

Template:

`During <PHASE>, perform the role <ROLE>. Objective: <OBJECTIVE>. Task: <WORK ITEM>. Acceptance criteria: <CRITERIA>.`

## 7. Example Workflow

### Phase: Plan
Objective: turn idea into an approved specification.
- Roles: Designer, Standards Reviewer

### Phase: Implement
Objective: deliver code/docs/tests matching the specification.
- Roles: Engineer, Tester

### Phase: Review
Objective: validate quality/compliance and approve release readiness.
- Roles: Reviewer, Compliance Checker

## 8. Execution Model

The board should show:

1. currently due work items
2. in-progress work items
3. blocked/failed work items
4. predicted future work from the workflow graph

Agents can repeatedly ask for the next available work item, execute it, then report result.

## 9. CLI Concept (Draft)

```bash
# get next due work item and assign to caller
tk request

# mark in progress
tk active <id>

# mark success/failure with reason
tk success <id> -m "reason"
tk fail <id> -m "reason"

# get generated prompt/context for execution
tk prompt <id>

# submit execution feedback/artifacts
tk feedback <id> -m "outcome"
```

## 10. Invariants

1. Every WorkItem belongs to exactly one Ticket + Phase + Role.
2. Every state transition is recorded in TicketHistory.
3. A WorkItem cannot be marked `success`/`fail` unless it has been `active`.
4. Workflow version used for execution is snapshotted per WorkItem.
5. Prompt text used for execution is snapshotted for auditability.

## 11. Migration Direction

1. Terminology migration: **Workflow -> Workflow** across API, CLI, DB, docs.
2. Preserve behavior first (compatibility layer), then evolve model.
3. Introduce WorkItem as explicit first-class execution unit.
4. Keep phase/role progression linear in v1.
5. Add DAG-capable progression rules in a later version.

## 12. Decisions (Locked for v1)

1. Must all roles in a phase approve, or can policy define quorum?
   All roles must approve.
   When a role does not approve, human intervention is required. Site2 should provide a mailbox-style UX that surfaces the failed item and supports conversation with the accountable human.

2. Is role order strictly linear or true DAG in v1?
   Linear in v1.

3. What is the canonical return path after failure?
   Failure stops progression and requests accountable human intervention.
   The human decision outcome is recorded as TicketHistory and may route back to a Phase/Role or create refinement work.

4. Should `request` auto-claim or remain separate from `claim`?
   Remove `claim` in v1.
   `request` means: "assign me the next available work item; I do not care which one."

5. How are agent identities and permissions modeled?
   Agents authenticate like users, but identity type must be explicit (for example `a-UUID` for agents, `u-UUID` for users).
   Fine-grained permission modeling is deferred for a later phase.
