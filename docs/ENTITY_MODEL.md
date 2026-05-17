# Entity Model

This document is the **phase 1 authoritative design document** for the core
domain entities in `ticket`.

Its purpose is to define the target entity model clearly enough that:

1. humans can reason about the system,
2. later implementation phases can align the schema, API, CLI, TUI, and web UI,
3. older or inconsistent descriptions elsewhere in the codebase can be resolved
   against one definitive source.

Where this document conflicts with older entity descriptions in `SPEC.md`,
`docs/DESIGN.md`, or `docs/LIFECYCLE.md`, **this document takes precedence for
the entity model**.

---

## 1. Scope

This document defines these entities and their relationships:

- **PROJECT**
- **Workflow**
- **STAGE**
- **ROLE**
- **TICKET**

It also defines:

- Workflow inheritance for tickets
- the meaning of DOR, DOD, and AC
- ordering rules for stages and roles
- the intended distinction between business entities and implementation detail

This is a **design target**, not a claim that the entire codebase already
matches it.

---

## 2. Canonical Vocabulary

### 2.1 Ticket Types

In this design, **everything actionable is a ticket**.

The canonical ticket types are:

- `epic`
- `story`
- `task`
- `bug`
- `feature`
- `idea`
- `spike`
- `chore`
- `note`
- `question`
- `requirement`
- `decision`

These are **ticket types**, not separate top-level entities.

The word may still appear in existing code, APIs, or docs as a legacy concept,
but the target model is:

- **Ticket** is the single work-item entity
- ticket types are classifications of that entity

### 2.2 DOR / DOD / AC

- **DOR** — Definition of Ready
- **DOD** — Definition of Done
- **AC** — Acceptance Criteria

These are different kinds of guidance:

- **DOR** says what must be true before work should begin
- **DOD** says what must be true before work is considered complete
- **AC** says what outcome or behaviour is expected

---

## 3. Core Relationship Model

The core model is:

```text
PROJECT 1 --- * TICKET
PROJECT 1 --- 1 default Workflow

Workflow 1 --- * STAGE           (ordered)
Workflow 1 --- * ROLE
STAGE * --- * ROLE           via ordered stage-role assignments within an Workflow

TICKET 0..1 --- 1 explicit Workflow
TICKET 0..1 --- 1 parent TICKET
```

### 3.1 Effective Workflow Resolution

Every ticket has an **effective Workflow**.

The effective Workflow is resolved in this order:

1. If the ticket has its own `workflow_id`, use it
2. Otherwise, walk up the parent ticket chain and use the first ancestor with an
   explicit `workflow_id`
3. Otherwise, use the ticket's project's `default_workflow`

This means:

- a ticket may inherit its Workflow,
- parent tickets can define Workflow boundaries for their descendants,
- the project always provides a final fallback,
- the project-level default Workflow is therefore **mandatory**.

### 3.2 Same-Project Rule

Parent/child ticket relationships must remain **within the same project**.

Cross-project ticket hierarchy is not allowed.

---

## 4. PROJECT

## 4.1 Purpose

A project is the top-level container for work.

It defines:

- the ticket namespace (`TK-123`, `CUS-44`, etc.),
- the repository or codebase context,
- the boundary of what the project is trying to do,
- the boundary of what the project is explicitly not trying to do,
- the canonical reference links for humans and tools,
- the default lifecycle model,
- the draft behaviour for new tickets (which now start in draft mode by default).

## 4.2 Core Fields

| Field | Meaning |
|---|---|
| `title` | Human-readable project name |
| `description` | Free-text summary of the project |
| `prefix` | Ticket key prefix used in ticket identifiers |
| `in_scope_goals` | What the project does, is responsible for, or intends to deliver |
| `out_of_scope_goals` | What the project explicitly does not do or own |
| `reference_links` | Strongly typed map of reference-link keys to URLs |
| `dor_map` | Project-level DOR map keyed by stage name plus `default` |
| `dod_map` | Project-level DOD map keyed by stage name plus `default` |
| `ac_map` | Project-level AC map keyed by stage name plus `default` |
| `default_draft` | Project-level draft preference metadata; new tickets currently start in draft mode by default |
| `default_workflow` | Required link to the Workflow used when no ticket/ancestor override exists |

## 4.3 Rules

1. A project has exactly one **default Workflow**
2. `default_workflow` is **non-null**
3. `default_draft` is retained as project-level metadata; current ticket creation still starts new tickets in draft mode by default
4. A project's `prefix` defines the human-facing ticket key namespace
5. `in_scope_goals` and `out_of_scope_goals` define the product boundary for the project
6. `reference_links` is the home for canonical supporting URLs and document links
7. project guidance maps are keyed by stage name plus reserved `default`

## 4.4 Reference Links

The project should support a set of named links rather than only one Git URL.

The agreed shape is a **strongly typed map** keyed by documented constants.

Examples:

- `git`
- `wiki`
- `requirements`
- `spec`
- `design`

The design intent is that a project can answer both:

- “what does this project do?”
- “where do I go to read the source material for it?”

These keys should be defined in code as constants and documented as part of the
entity model so they remain stable across the store, API, CLI, TUI, web, and
prompt assembly logic.

The `git` URL should live inside `reference_links`; it does not need a separate
top-level project field if the map is present.

## 4.5 Prefix Decision

- the prefix is a **short human-readable alphabetic identifier**
- it is used to build ticket keys such as `TK-123` or `CUS-42`
- “digit” in the earlier notes is interpreted as “character”

The key point for the design is that the prefix is a **project-scoped namespace
token**, not just an arbitrary integer.

---

## 5. Workflow

## 5.1 Purpose

An Workflow defines **how work moves**.

It does not hold tickets directly. Instead, it defines the ordered stages and
the role sequence available to tickets whose effective Workflow resolves to it.

## 5.2 Core Fields

| Field | Meaning |
|---|---|
| `title` | Name of the Workflow |
| `description` | Free-text explanation of the Workflow |
| `stages` | Ordered list of stages |
| `stage_roles` | Ordered role assignments for each stage |

## 5.3 Rules

1. An Workflow contains one or more stages
2. Stages are ordered
3. Each stage can contain zero or more roles
4. Roles within a stage are ordered
5. The same role may appear in multiple stages
6. A stage may appear in one Workflow and not another

The Workflow is the reusable workflow template. The ticket is the runtime work item
moving through it.

---

## 6. STAGE

## 6.1 Purpose

A stage is a named phase within an Workflow.

Examples:

- `design`
- `develop`
- `test`
- `uat`
- `done`

## 6.2 Core Fields

| Field | Meaning |
|---|---|
| `title` | Stage name |
| `description` | Free-text description of the phase |
| `dor` | Stage-specific definition of ready |
| `dod` | Stage-specific definition of done |
| `ac` | Stage-specific acceptance criteria |
| `order` | Position within the Workflow |

## 6.3 Rules

1. A stage belongs to exactly one Workflow
2. Stage ordering is explicit and meaningful
3. The first stage is the default entry stage for tickets in that Workflow
4. A stage may have zero or more roles assigned to it

---

## 7. ROLE

## 7.1 Purpose

A role represents a job function or persona that can perform work in one or more
stages of an Workflow.

Examples:

- Product Owner
- Architect
- Engineer
- QA

## 7.2 Core Fields

| Field | Meaning |
|---|---|
| `title` | Role name |
| `description` | What the role is responsible for |
| `dor_map` | Role-level DOR guidance keyed by stage name plus `default` |
| `dod_map` | Role-level DOD guidance keyed by stage name plus `default` |
| `ac_map` | Role-level AC guidance keyed by stage name plus `default` |

The maps are keyed by stage name and may also contain a special `default` key.

Example:

```json
{
  "default": "Review work for quality and completeness",
  "test": "Verify reproducibility and edge-case coverage"
}
```

## 7.3 Rules

1. A role may be assigned to any stage in any Workflow
2. Role ordering is **per stage assignment**, not global
3. Role guidance is resolved like this:
   1. use the entry for the current stage name if present
   2. otherwise use `default` if present
   3. otherwise treat the role as having no value for that key

## 7.4 Consequence

This design allows a role to be:

- **general-purpose** via `default`, or
- **specialised** for a specific stage

without requiring multiple nearly-identical roles.

---

## 8. TICKET

## 8.1 Purpose

A ticket is the single business entity representing a unit of work.

Everything that humans casually call a task, bug, story, epic, spike, note,
question, requirement, or decision is a ticket.

### 8.1.1 Type vs Lineage

The newest requirement makes an important distinction:

- **type** is advisory classification
- **lineage** is structural meaning

In other words:

- a ticket with no parent may be standalone work
- a ticket with children is structurally complex
- a ticket with many children is often what humans would call an epic

So the system should not depend too heavily on the ticket type label alone to
infer complexity. The parent/child graph is the stronger signal.

## 8.2 Core Fields

| Field | Meaning |
|---|---|
| `id` | Human-facing key such as `TK-123` |
| `type` | Advisory classification of the ticket |
| `title` | Short summary |
| `description` | Main narrative description |
| `dor_map` | Ticket-specific DOR map keyed by stage name plus `default` |
| `dod_map` | Ticket-specific DOD map keyed by stage name plus `default` |
| `ac_map` | Ticket-specific AC map keyed by stage name plus `default` |
| `stage` | Current stage in the effective Workflow |
| `role` | Current role within the current stage |
| `state` | Current work state |
| `deleted` | Soft-delete flag |
| `draft` | Whether the ticket is still being curated |
| `archived` | Soft-hidden ticket |
| `complete` | Whether the ticket is finished |
| `workflow` | Optional explicit Workflow override |
| `parent_ticket_id` | Optional parent ticket |
| `project` | Required owning project |

## 8.3 Lifecycle Fields

### `stage`

The ticket's current stage within its effective Workflow.

### `role`

The ticket's current role within the stage.

This field is necessary whenever a stage has multiple ordered roles.

### `state`

Allowed values:

- `idle`
- `active`
- `success`
- `fail`

`state` describes the ticket's runtime condition at the current stage/role step.

### `draft`

If true, the ticket is not yet ready to enter normal work execution.

### `complete`

If true, the ticket is fully complete.

### `archived`

If true, the ticket is soft-hidden from normal working views.

## 8.4 Delete vs Archive

The design should distinguish these clearly:

- **archive** is a business state (`archived=true`)
- **delete** is a soft-delete state (`deleted=true`)
- **purge** is the physical removal operation
- **undelete** is the recovery operation from the deleted state

That addresses the field list in the requirement like this:

- `archived` is a field
- `complete` is a field
- `deleted` is a field

The intended semantics are:

- `archived` hides work from normal working views but keeps it live
- `deleted` marks work as soft-deleted
- `purge` permanently removes a deleted ticket

## 8.5 Workflow Inheritance Rules

Each ticket may have an explicit `workflow_id`, but it is optional.

That field means:

- `NULL` = inherit
- set value = use this Workflow for this ticket and its descendants unless a child
  overrides it

This is the key rule that enables an epic or story to branch into a different
workflow while still remaining in the same project.

## 8.6 Parent/Child Rules

1. A ticket may have a parent ticket
2. A parent ticket may have many children
3. Parent/child links must stay within one project
4. Children inherit the effective Workflow unless they override it explicitly

---

## 9. Ticket Types

Everything below is a **ticket type**, not a separate entity:

- `epic` — a large unit of work that is expected to be broken down into child tickets
- `story` — a user- or outcome-oriented piece of work
- `task` — a general unit of actionable work
- `bug` — a defect or unintended behaviour that needs correction
- `feature` — a deliverable capability or enhancement
- `idea` — an early concept that may later be refined into planned work
- `spike` — time-boxed investigation or research
- `chore` — supporting or maintenance work with limited user-facing impact
- `note` — a record of supporting information that still needs to be tracked
- `question` — a request for clarification or a decision-driving uncertainty
- `requirement` — a statement of capability, behaviour, or constraint that must be satisfied
- `decision` — a work item used to capture and track a concrete decision

The important structural rule remains:

- **type** is classification
- **lineage** is structure

---

## 10. Guidance Resolution

Different layers can provide DOR/DOD/AC or related guidance.

The design intent is:

- **project** provides broad defaults and operating context
- **stage** provides workflow-step expectations
- **role** provides stage-sensitive role expectations
- **ticket** provides ticket-specific requirements

For later implementation, prompt assembly and UI rendering should treat
ticket-level values as the most specific information and project-level values as
the broadest context.

This document does **not** require one merged text field. It requires the
system to preserve the source of each guidance layer.

The agreed storage direction is to use **strongly typed maps** for flexible
guidance rather than keep adding new columns whenever the guidance model grows.

That means:

- project guidance is map-based
- role guidance is map-based
- ticket guidance is map-based

The guidance-map keys are not a separate metadata enum. They are:

- the **stage names** from the effective Workflow, plus
- a reserved `default` key

That means a derived value is always resolved as:

1. `*_map[current_stage_name]`
2. otherwise `*_map["default"]`
3. otherwise no value at that layer

## 10.1 Map Key Contracts

The target model uses a small number of explicit key families.

| Map | Key contract |
|---|---|
| `project.reference_links` | `ReferenceLinkKey` constants such as `git`, `wiki`, `requirements`, `spec`, `design` |
| `project.dor_map`, `project.dod_map`, `project.ac_map` | stage names plus reserved `default` |
| `ticket.dor_map`, `ticket.dod_map`, `ticket.ac_map` | stage names plus reserved `default` |
| `role.dor_map`, `role.dod_map`, `role.ac_map` | stage names plus reserved `default` |

Guidance maps therefore vary by workflow stage, not by an extra nested
guidance-key vocabulary.

## 10.2 Guidance Composition Rules

Guidance from different layers must be **preserved by source**.

The layers are:

1. project
2. stage
3. role
4. ticket

The composition rules for V1 are:

1. **Do not destructively merge layers into one anonymous blob**
2. **Do not let a more specific layer erase a broader layer**
3. **Render guidance grouped by source layer**
4. For project, role, and ticket guidance, first resolve:
   1. current stage name entry if present
   2. otherwise `default`
5. For prompt assembly and detailed views, present guidance from broadest to most
   specific so the reader sees:
   1. project context
   2. stage expectations
   3. role expectations
   4. ticket-specific requirements
6. If multiple layers provide values, show each layer's resolved value
   separately; **do not concatenate them into a single stored value**

This gives prompts and UIs a stable rule:

- preserve provenance
- preserve specificity
- avoid hidden overwrite behaviour

## 10.3 DOR Resolution Use Cases

The easiest way to understand the map model is to simulate the derived value for
one stage.

The same rule applies to `dor_map`, `dod_map`, and `ac_map`. The examples below
use `dor_map`.

### Use case 1: stage-specific value exists

Given:

```json
{
  "default": "work is described clearly enough to begin",
  "develop": "design is approved and dependencies are understood"
}
```

If the current stage is `develop`, the derived DOR is:

```text
design is approved and dependencies are understood
```

because `dor_map["develop"]` exists and wins over `dor_map["default"]`.

### Use case 2: stage-specific value is missing, so `default` is used

Given:

```json
{
  "default": "work is described clearly enough to begin"
}
```

If the current stage is `test`, the derived DOR is:

```text
work is described clearly enough to begin
```

because `dor_map["test"]` is absent and the system falls back to
`dor_map["default"]`.

### Use case 3: neither stage-specific nor `default` exists

Given:

```json
{
  "design": "problem statement is understood"
}
```

If the current stage is `develop`, the derived DOR is:

```text
<no value>
```

because neither `dor_map["develop"]` nor `dor_map["default"]` exists.

### Use case 4: project and role both resolve for the same stage

Given the current stage `develop`:

Project:

```json
{
  "default": "work must map to a valid project objective"
}
```

Role:

```json
{
  "develop": "requirements and boundaries are clear enough to implement"
}
```

The derived values are:

- Project DOR: `work must map to a valid project objective`
- Role DOR: `requirements and boundaries are clear enough to implement`

Both should be shown because guidance is preserved by source layer. The role
value does **not** erase the project value.

### Testing expectation for implementation

When phase 2 begins, red/green implementation should create **unit tests**
covering these exact examples.

At minimum, tests should verify:

1. stage-specific lookup wins over `default`
2. `default` is used when the stage-specific value is absent
3. missing stage-specific and missing `default` produces no derived value
4. multiple layers can each resolve independently for the same stage without
   overwriting one another

---

## 11. Required Invariants

These rules are part of the target design:

1. Every ticket belongs to exactly one project
2. Every project has exactly one default Workflow
3. A ticket's explicit Workflow is optional
4. The effective Workflow resolution order is:
   `ticket -> nearest ancestor with explicit Workflow -> project default`
5. Stage ordering is defined by the Workflow
6. Role ordering is defined per stage assignment inside the Workflow
7. A story is a ticket type, not a separate entity
8. Archive is soft-hide; deleted is soft-delete; purge is physical removal
9. Parent/child relationships cannot cross projects

---

## 12. Current Codebase vs Target Design

This section captures the most important mismatches discovered during the phase 1
review.

## 12.1 Already aligned or partially aligned

1. **Project-level default draft exists**
   - `projects.default_draft` exists
2. **Project-level Workflow exists**
   - `projects.workflow_id` exists
3. **Ticket-level optional Workflow exists**
   - `tickets.workflow_id` exists
4. **Effective Workflow inheritance is already partly implemented**
   - ticket explicit Workflow
   - then parent-chain lookup
   - then project Workflow fallback
5. **Stages already support DOR/DOD/AC**
   - `workflow_stages` has acceptance criteria, definition of ready, definition of done

## 12.2 Not aligned with this target design

1. **Story still exists as a separate table**
   - current code has `stories` and `story_ticket_links`
   - target design says story is a ticket type only
2. **Project links are too narrow**
   - current code mainly models Git repository metadata
   - target design requires a strongly typed reference-link map
3. **Project guidance model is too shallow**
   - current code uses flatter guidance fields such as `acceptance_criteria`
     and `notes`
   - target design requires strongly typed project `dor_map`, `dod_map`, and
     `ac_map`
4. **Role guidance model is too shallow**
   - current roles have one `acceptance_criteria` field
   - target design requires stage-ID-keyed `dor`, `dod`, and `ac` maps with a
     reserved `default` fallback
5. **Project default Workflow is still nullable in the schema**
   - target design requires it to be non-null
6. **Ticket guidance model is too shallow**
   - current ticket schema is flatter than the target model
   - target design requires strongly typed ticket `dor_map`, `dod_map`, and
     `ac_map`
7. **Delete semantics are not yet aligned**
   - target design now requires a persistent `deleted` soft-delete field
   - later admin commands such as `purge` and `undelete` should act on it
8. **Current docs still over-emphasize type-specific entities in places**
   - some documents and code paths still model stories separately
   - the new requirement says a ticket is any actionable work

## 12.3 Phase 2 implications

Phase 2 should align the implementation directly to the target model.

For this V1 effort, **migration compatibility is not required**:

- do not spend effort on database migrations from older local schemas
- do not keep legacy compatibility layers just to preserve old shapes
- prefer a clean breaking refactor when that produces a simpler correct model
- treat the target model in this document as the thing to implement

1. collapse `stories` into tickets of type `story`
2. make project default Workflow mandatory
3. introduce project `reference_links` as a strongly typed map
4. replace flatter project guidance with strongly typed `dor_map` / `dod_map` /
   `ac_map`
5. introduce stage-ID-keyed role guidance maps
6. replace flatter ticket guidance with strongly typed `dor_map` / `dod_map` /
   `ac_map`
7. introduce the `deleted` soft-delete field and later admin recovery/purge
   flows
8. align API, CLI, TUI, and web terminology around one ticket-centric model

---

## 13. Recommended Interpretation for Later Phases

For the next implementation phases, the intended model should be:

- **PROJECT** owns the namespace and the default workflow
- **Workflow** defines ordered stages
- **STAGE** defines the phase-specific requirements
- **ROLE** defines who works in a stage and what that role expects there
- **TICKET** is the single work item entity and may override or inherit workflow

The persistence bias for flexible guidance and links is:

- use strongly typed **maps** for guidance and reference material
- use stable scalar fields for identity, lineage, workflow, and state

That gives one coherent rule:

> Tickets inherit workflow from the nearest explicit source, and work progresses
> through an Workflow-defined sequence of stage/role steps until complete.

---

## 14. Delivery Plan

This is the concrete plan implied by the target design.

### Phase 1 — Agree the entities and relationships

Goal:

- settle the target entity model
- identify where the codebase and docs currently disagree
- establish one source-of-truth design document

Deliverables:

- this document
- clear list of inconsistencies
- explicit inheritance and lineage rules

### Phase 2 — Refactor / implement the data model and codebase

Goal:

- align the schema and store layer to the target model
- do so directly, without spending effort on backwards-compatibility or migration machinery for pre-V1 shapes

Main work:

1. make project default Workflow mandatory
2. add project `reference_links` as a strongly typed map
3. enforce the canonical ticket-type set defined in this document
4. collapse legacy story storage into the ticket model
5. introduce strongly typed project guidance maps
6. introduce stage-ID-keyed role guidance maps
7. introduce strongly typed ticket guidance maps
8. introduce the `deleted` soft-delete field
9. align inheritance logic and persistence rules everywhere

### Phase 3 — Refactor / implement the CLI

Goal:

- make the CLI speak the same domain language as the entity model

Main work:

1. remove legacy story-as-separate-entity behaviour
2. expose project default Workflow and default draft cleanly
3. expose ticket Workflow override and inheritance clearly
4. expose lineage-first ticket management clearly
5. add/align `tk prompt <id>` so it assembles one coherent work prompt from the entity graph
6. align help text, usage, and JSON output with the new model

### Phase 4 — Refactor / implement the TUI

Goal:

- make the interactive terminal model reflect the same entities and relationships

Main work:

1. align forms and lists with the ticket-centric model
2. display effective Workflow, explicit Workflow, and inherited Workflow clearly
3. show lineage and child structure as first-class information
4. show project/stage/role/ticket guidance consistently

### Phase 5 — Refactor / implement the website

Goal:

- align the web application with the same domain model and terminology

Main work:

1. remove any remaining story/ticket ambiguity
2. display lineage, inheritance, and workflow clearly
3. show project defaults and ticket overrides explicitly
4. ensure forms, boards, and details pages follow the same entity semantics as
   the CLI and TUI

---

## 15. Putting It All Together

The entity model becomes most useful when the system can turn a ticket and its
relationships into a single work brief.

That is the conceptual purpose of:

```bash
tk prompt <ticket-id>
```

## 15.1 Purpose of `tk prompt <id>`

`tk prompt <id>` should gather the ticket's full working context and render one
plain-language prompt that can be handed to:

- a human,
- an agent,
- or an LLM-backed worker.

The command should assemble context from:

1. the **ticket itself**
2. its **parent chain**
3. its **project**
4. its **effective Workflow**
5. its **current stage**
6. its **current role**
7. the available **DOR / DOD / AC** guidance from each layer

## 15.2 Prompt Assembly Inputs

Conceptually, `tk prompt <id>` should resolve and include:

### Project context

- project title and description
- in-scope goals
- out-of-scope goals
- project-level links
- project-level resolved DOR / DOD / AC for the current stage

### Inheritance context

- explicit ticket Workflow, if present
- otherwise inherited Workflow source (nearest ancestor or project default)
- parent ticket chain, from nearest parent upward

### Workflow context

- effective Workflow title and description
- current stage title, description, DOR, DOD, AC
- current role title, description
- role guidance for the current stage, with `default` fallback when stage-specific values are absent

### Ticket context

- ticket id
- ticket type
- ticket title
- ticket description
- ticket-level resolved DOR / DOD / AC for the current stage
- current stage / role / state
- draft / archived / complete status

## 15.3 Prompt Assembly Goal

The output should answer these questions in one place:

1. What is this piece of work?
2. Why does it exist?
3. What project and workflow does it belong to?
4. What constraints and expectations apply right now?
5. What should the worker do next?
6. What does success look like?

The prompt is therefore not a raw dump of database rows. It is a **composed work
brief** built from the entity relationships.

## 15.4 Small Worked Example

### Example project

**Project:** Customer Portal

- Prefix: `CUS`
- Description: Web portal for customer account access and billing support
- In scope:
  - account sign-in
  - profile management
  - invoice viewing
- Out of scope:
  - payment processing
  - CRM administration
- Links:
  - Git: `https://git.example.com/customer-portal`
  - Wiki: `https://wiki.example.com/customer-portal`
  - Requirements: `https://reqs.example.com/customer-portal`
  - Spec: `https://specs.example.com/customer-portal-v2`
- Project `dor_map`:
  - `default`: work must map to a customer-facing problem or platform need
- Project `dod_map`:
  - `default`: behaviour is implemented, reviewed, and documented
- Project `ac_map`:
  - `develop`: change stays within the agreed product boundary
- Default draft: `true`
- Default Workflow: `Customer Delivery`

### Example Workflow

**Workflow:** Customer Delivery

Stages in order:

1. `design`
2. `develop`
3. `test`
4. `done`

### Example roles by stage

- `design`
  - Product Owner
  - Architect
- `develop`
  - Engineer
- `test`
  - QA

### Example stage guidance

- `design`
  - DOR: problem statement and desired outcome are known
  - DOD: approach is agreed and implementation boundaries are clear
  - AC: design explains what will change and what will not
- `develop`
  - DOR: design is approved
  - DOD: implementation is complete and reviewed
  - AC: code satisfies the agreed design intent
- `test`
  - DOR: implementation is available in a testable form
  - DOD: expected behaviour is verified and regressions are addressed
  - AC: tests demonstrate the required outcome

### Example role guidance

**Engineer**

- `default.dod`: leave the system in a shippable, understandable state
- `develop.dor`: requirements and boundaries are clear enough to implement
- `develop.ac`: implement the behaviour with minimal unrelated change

**QA**

- `default.ac`: verify expected behaviour and obvious edge cases

### Example tickets

**CUS-100** — type `epic`

- Title: Improve account sign-in reliability
- Description: Break down and deliver reliability improvements across login flows
- Explicit Workflow: none
- Project: Customer Portal

**CUS-114** — type `feature`

- Parent: `CUS-100`
- Title: Add lockout messaging for repeated failed sign-ins
- Description: Show a clear lockout message after repeated failed login attempts
- Explicit Workflow: none
- Current stage: `develop`
- Current role: `Engineer`
- Current state: `idle`
- Ticket `ac_map`:
  - `develop`: locked-out users see a clear, actionable explanation

In this example:

- `CUS-114` has no explicit Workflow
- its parent `CUS-100` has no explicit Workflow
- therefore its effective Workflow comes from the project's default Workflow: `Customer Delivery`

## 15.5 Simulated `tk prompt CUS-114`

```text
You are working on ticket CUS-114.

PROJECT
  Title: Customer Portal
  Description: Web portal for customer account access and billing support
  In scope:
    - account sign-in
    - profile management
    - invoice viewing
  Out of scope:
    - payment processing
    - CRM administration
  Links:
    - Git: https://git.example.com/customer-portal
    - Wiki: https://wiki.example.com/customer-portal
    - Requirements: https://reqs.example.com/customer-portal
    - Spec: https://specs.example.com/customer-portal-v2
  Project DOR (default): work must map to a customer-facing problem or platform need
  Project DOD (default): behaviour is implemented, reviewed, and documented
  Project AC (develop): change stays within the agreed product boundary

LINEAGE
  Parent: CUS-100 Improve account sign-in reliability
  Parent summary: deliver reliability improvements across login flows

WORKFLOW
  Effective Workflow: Customer Delivery
  Workflow source: project default
  Current stage: develop
  Current role: Engineer
  Current state: idle

STAGE GUIDANCE
  Stage DOR: design is approved
  Stage DOD: implementation is complete and reviewed
  Stage AC: code satisfies the agreed design intent

ROLE GUIDANCE
  Role: Engineer
  Role DOR (develop): requirements and boundaries are clear enough to implement
  Role AC (develop): implement the behaviour with minimal unrelated change
  Role DOD (default): leave the system in a shippable, understandable state

TICKET
  ID: CUS-114
  Type: feature
  Title: Add lockout messaging for repeated failed sign-ins
  Description: Show a clear lockout message after repeated failed login attempts
  Ticket AC (develop): locked-out users see a clear, actionable explanation
  Draft: false
  Archived: false
  Complete: false

TASK
  Implement the feature described by CUS-114.
  Stay within the project scope.
  Do not introduce payment-processing behaviour.
  Use the parent epic as context, the project goals as boundary, and the current
  stage/role guidance as the standard for execution.

SUCCESS LOOKS LIKE
  - the change satisfies the ticket acceptance criteria
  - the work meets the develop-stage expectations
  - the output is ready for the next workflow step
```

This example is intentionally small, but it shows the principle:

> `tk prompt <id>` is where the entity model becomes operational by turning
> project + lineage + workflow + guidance into one coherent work brief.
