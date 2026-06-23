# GOAL

> **Status: product vision / north-star, not a feature reference.** "Goal" here is
> the conceptual outcome a unit of work refines from. The earlier standalone *goal*
> entity/command was removed; that concept is now realized through the
> Feature → Epic → Story refinement model (see `docs/RELEASES.md`, `docs/FACTORY.md`,
> `docs/DESIGN_ORCHESTRATOR.md`). There is no `tk goal` command.

## Vision

`ticket` should be a general-purpose **software development** system where:

- Humans define goals/specifications.
- Agents refine goals, implement work, and verify outcomes.
- Humans approve readiness and final outcomes.

Core operating principle: **humans steer intent and acceptance; computers execute implementation**.

## Desired workflow

### 1. Planning / goal setting

- A user creates a goal.
- An agent refines the goal by clarifying requirements, identifying ambiguities, and defining acceptance criteria.
- User and agent jointly mark the goal as **ready for development** according to user-defined Definition of Ready rules.

### 2. Implementation

- One or more agents execute the work.
- Workflow-defined roles are executed via an orchestrator to assess work against requirements, rules, and guardrails.
- Assessment is triggered when the implementing agent reports work is complete according to the defined criteria.
- Human and agent jointly determine whether the implementation proceeds to verification or returns for refinement.

### 3. Verification

- An agent performs final verification against approved acceptance criteria.
- The user signs off completion according to user-defined Definition of Done rules.

## Product philosophy

- This is outcome-driven, not ceremony-driven.
- Planning artifacts (decomposition, sequencing, sub-goals) are useful, but heavyweight agile ritual is not required.
- Agent planning still exists internally and should be transparent to the human for approval.
- The human operates at a higher abstraction level: define goals, constraints, and acceptance criteria; review outcomes.

## Resolved decisions

- **Database**: SQLite.
- **Scope of "general purpose"**: software development.
- **Sign-off policy**: mandatory sign-off for each phase (planning, implementation, verification), and this policy is itself configurable.
- **Assessment model**: orchestrated, role-based assessment runs when the implementing agent reports the work is complete according to the defined criteria.
- **Failure/escalation policy**: when outcomes fail verification or assessment, create a mailbox entry for a human decision including recommendations: clarify goal, start again, or refine requirements.
- **Compatibility strategy**: this effort is a deliberate breaking-change-if-necessary evolution of `ticket`.
- **Definition of Ready / Definition of Done**: pluggable user-defined rules.
- **Roles**: pluggable user-defined roles with CRUD management and role-specific objectives.
- **Workflow composition**: each project workflow maps phases to roles; at runtime the agent is evaluated against goal + role + phase + project-level rules.
- **Rules source of truth**: user-authored markdown text across goals, roles, workflows, and projects.
- **Rules content**: these markdown rules can encode SDLC process, guardrails, compliance instructions, engineering ways of working, coding standards, and test instructions.
- **POC auth model**: keep security close to current model — CRUD-managed users and agents, session/bearer auth for users, and Basic Auth credentials for agent endpoints.
- **POC authorization model**: no policy-based access control engine yet; keep role-based authorization close to current behavior.
- **Security roadmap**: once the POC workflow is working end-to-end, refactor and harden authentication/authorization.

## Proposed platform shape (initial)

1. **CLI** for interaction and CRUD-style management of entities/configuration via API.
2. **Server + web UI** for human/agent collaboration and entity/configuration management via API.
3. **SQLite backend** owned by server (single writer boundary through server APIs).
4. **One or more agent loops** integrating through server APIs.

## Immediate next planning objective

Measure the current `ticket` codebase and capabilities against this goal, then produce a concrete change plan (architecture, data model, workflow logic, and rollout approach).

## Use Case

WEBSITE:
    GOAL REFINEMENT
        1. User enters goal
        2. User clicks "refine"
        3. Agent goes into a back-and-forth conversaton with user via website chat interface
        4. User an agent eventually agree the goal makes sense.
        5. User clicks "ready"
        6. Goal is moved to a "ready state"

Remember that part of the refinement of a goal - clarifying the outcome, the method etc.  Also does include a breakdown/decompose the work.  The agent itself should be looking at that such that the outcome of the refinement is

input: (dirty goal)
output(s): clean goal
    decomposition to high level objectives, sequence of work, equivalent to epics, stories etc.

the user will agree the goal is right, but may then move around the priority of the decomposition, ro discuss about changing the breakdown somehow.

### Website goal-refinement contract (explicit)

Input:
- A "dirty goal" from the user (title/description/notes).

Required refinement outputs before "ready":
1. Clean goal statement (single clear outcome).
2. Decomposition that includes:
   - high-level objectives,
   - sequence of work,
   - equivalent epics/stories/tasks.
3. Ordered decomposition that the user can reprioritize in the UI before sign-off.

"Ready" transition contract:
1. User clicks **Refine** to move the goal to `refining`.
2. User and agent iterate in goal chat until the user agrees.
3. User can edit and reorder decomposition priorities.
4. User clicks **Ready** only when clean goal + decomposition are present.
5. System moves goal to `ready` state and preserves the agreed refinement output.

Pause/resume contract:
- Users may walk away at any point.
- A project may contain many goals across `draft`, `refining`, and `ready`.
- System must preserve state without friction and let users resume/refine/reprioritize later.



A goal can have context attached to it along with notes. e.g.

    1. Text
    2. links to an external document.
    3. document uploaded (via a file upload, e.g. a PDF or MD file)
    4. A reference to an internal document.

It is possible to updload documents into the system whihc then contribute to othe knowledge graph

    All content should be available for query via some graph; a context graph for example.  What technologiy should be used for that? pg vector/postgres? neo4j? another graph db?


UX
    do not embed css or js in the html; make external links to it
    create an api.js that maps directly to the openapi specificationa nd test harness. do not embed UX logic in this
    use an app.js that us the UX logic which calls the api.js when necessary



----
docs

there are too many documents and they polluate the context - .md files and I want to tidy up so that the goal is

    - reduce unnecessaery documentation files
    - ensure we retain only what is necessary I think by these classification

        README

        USER_GUIDE
            QUICKSTART
            TUTORIAL

        DEVELOPER_GUIDE
            DESIGN/ARCHITECTURE/GOALS/TODO
            WAY_OF_WORKING/SDLC
            AGENTS.md/copilot instructions/CLAUDE.md

    I want you to consolidate all the documentation so that we end up with a clean repository of only the necessary docs.
    Once complete it shoudl mean all necessary documentaiton context is available via these.