# The Software Factory

**A complete, self-contained specification.** This document describes a
software system in full: its mission, vision, operating model, requirements,
and the development process by which it should be built. It is deliberately
technology-neutral — a competent team (human or agent) should be able to
implement it from this document alone, in any language, on any stack.

---

## 1. Mission Statement

Build a **software factory**: a system in which a human directs the creation
of working software by collaborating with **adversarial, role-based AI
agents** through four distinct phases — **refinement**, **implementation**,
**verification**, and **release**.

The human supplies intent, constraints, and acceptance. The machine supplies
execution. Between them sits a deterministic production line that turns rough
ideas into shipped software with an auditable trail at every step.

> **Core operating principle: humans steer intent and acceptance; computers
> execute implementation.**

## 2. Vision

Today, software delivery tools either track work (issue trackers) or do work
(coding agents), but nothing closes the loop between them. The factory closes
that loop:

- A human writes down a **goal** — rough, ambiguous, incomplete is fine.
- An agent **refines** it in dialogue with the human: clarifying the outcome,
  surfacing ambiguity, writing acceptance criteria, and proposing a
  decomposition into ordered, implementable stories.
- The human **reprioritizes and approves** the decomposition. Nothing
  proceeds without explicit human sign-off at each phase gate.
- An **orchestrator** — deterministic, rule-driven, with no intelligence of
  its own — pushes ready work to worker agents, advances successes, recovers
  failures, and abandons stalled work.
- Work flows through **adversarial roles**: the agent that implements is
  never the agent that verifies. An engineer's output is challenged by a
  reviewer; a reviewer's approval is challenged by a tester. Trust emerges
  from the structure of the workflow, not from any single agent's claims.
- Failures that the machine cannot resolve **escalate to a human mailbox**
  with concrete recommendations: clarify the goal, refine the requirements,
  or start over.
- Everything the team knows — documents, links, decisions, prior work — lives
  in a queryable **context graph** that gives every agent the same shared
  memory a good colleague would have.

### North Star

**A single human can responsibly operate a team of agents that designs,
builds, verifies, and ships production software — while remaining accountable
for every outcome.**

Progress toward the north star is measured by:

1. **Human leverage** — the ratio of shipped, verified stories to human
   interventions required.
2. **Trustworthy automation** — the fraction of agent-completed work that
   passes adversarial verification on the first attempt.
3. **Auditability** — every state change in the system can answer "who did
   this, when, and why?"
4. **Time-to-clarity** — how quickly a dirty goal becomes an approved,
   ordered decomposition.

### Product philosophy

- **Outcome-driven, not ceremony-driven.** Planning artifacts (decomposition,
  sequencing, sub-goals) exist because they are useful, not because ritual
  demands them. No velocity tracking, no burndown charts, no story-point
  theater.
- **The human operates at a higher abstraction level**: define goals,
  constraints, and acceptance criteria; review outcomes. Agent planning is
  internal but always transparent and subject to human approval.
- **Determinism where possible, intelligence where necessary.** The
  orchestrator applies rules; only agents think. This keeps the production
  line predictable, debuggable, and safe.
- **Everything is an API.** Every capability available to the human is
  available to an agent, and vice versa.

---

## 3. The Operating Model — Four Phases

Every unit of work moves through four phases. Each phase ends in a **gate**
that requires sign-off (human by default; the sign-off policy per phase is
configurable per project).

### Phase 1 — Refinement / Clarification

**Input:** a "dirty goal" — a title, a description, perhaps some attached
context (documents, links).

**Process:** a refiner agent opens a turn-based dialogue with the human on
the goal itself. The conversation is persisted as the goal's discussion
thread. On each turn the agent may:

- ask a **clarifying question** (the dialogue continues),
- propose the goal is **ready** as a single story, supplying a refined
  description and acceptance criteria, or
- propose a **breakdown** — a set of child stories that decompose the goal,
  each with its own title, description, and acceptance criteria.

**Required outputs before the gate:**

1. A clean goal statement — one clear outcome.
2. A decomposition: high-level objectives, a sequence of work, and
   epic/story-equivalent units.
3. An **ordered** decomposition the human can reprioritize in place before
   sign-off.

**Gate:** the human approves. A single refined story is marked ready; a
breakdown converts the goal into an epic whose ordered children are marked
ready. Pause/resume is free: a project may hold many goals across draft,
refining, and ready states indefinitely, and a human may walk away and return
at any point without losing state.

### Phase 2 — Implementation

**Input:** ready stories, sealed into a release (see §5.6).

**Process:** the orchestrator pushes each ready story to the best available
agent whose role matches the story's current workflow role. The agent
produces the work product (code, document, configuration — whatever the story
demands), reports success or failure, and records evidence (e.g. a change
reference). Agents heartbeat while working; stalled work is reclaimed and
reassigned.

**Gate:** the implementing agent reports the work complete *according to the
story's acceptance criteria and the role's Definition of Done*. This
triggers Phase 3 — it does not constitute completion.

### Phase 3 — Verification

**Process:** verification is **adversarial by construction**. The workflow
routes the story to one or more verifying roles (reviewer, QA) which are
never the implementing agent. Each verifying role independently assesses the
work against: the story's acceptance criteria, the role's own objectives, the
workflow's rules, and project-level guardrails. A verifier's job is to
*refute* the claim that the work is done.

- Verification **passes** → the story advances toward done.
- Verification **fails** → the story regresses one step with the verifier's
  findings attached; the implementing role tries again.
- Repeated failure (or failure the machine cannot classify) → a **mailbox
  entry** is created for human decision, carrying recommendations: clarify
  the goal, refine the requirements, or start again.

**Gate:** the human signs off completion according to the user-defined
Definition of Done.

### Phase 4 — Release

**Process:** stories belong to features; features belong to **releases**. A
release is designed (features added/removed freely), then **sealed** (the
feature set freezes; the orchestrator executes its ready stories), then
**completed** when every story is verified and integrated.

**Gate:** human acceptance of the release outcome: succeed → integrate and
ship; fail → return to Phase 1 with everything learned attached as context.

---

## 4. Core Concepts

| Concept | Definition |
|---|---|
| **Project** | Top-level namespace: a prefix, a team, a workflow, a context graph, and a body of work. Visibility: private, team, or public. |
| **Ticket** | The universal work artifact. Types: feature, epic, story, task, bug, spike, chore, note, question, requirement, decision, idea. Each has a stable human key (`PREFIX-N`), unique per system, generated by the server, immutable. |
| **Hierarchy** | Release → Feature → Epic → Story/Bug, linked by parent references. A feature parents epics; an epic parents stories, bugs, tasks, spikes, chores; a task parents tasks, bugs, spikes, chores; nothing else may be a parent. Parent and child always share a project. Cycles are forbidden. These rules are enforced, not advisory. |
| **Stage + State** | A ticket's position is `stage/state`. Backlog stages (idea → refine → ready) precede delivery stages (workflow-defined; canonically design → develop → test → done, optionally reject). States: idle, active (requires an assignee), success, fail. |
| **Workflow** | A named, ordered set of stages, each with an ordered set of **roles**. Defines the production line a project's tickets travel. Workflows are exportable/importable as data and validatable as a graph. |
| **Role** | A named participant in a workflow stage (e.g. Product Owner, Engineer, Reviewer, QA) with its own description, objectives, and guidance. Agents are matched to work by role name. |
| **Guidance maps** | Definition of Ready, Definition of Done, and Acceptance Criteria exist at project, role, and ticket level as user-authored text (markdown), keyed by stage with a `default` fallback. Guidance composes across layers and is always rendered grouped by its source. These texts are the system's rule language: SDLC process, guardrails, compliance, coding standards, test instructions. |
| **Agent** | An autonomous worker account, authenticated independently of humans, backed by a pluggable LLM or tool. Lifecycle: created by an admin → registers → receives pushed work → heartbeats while working → reports success/failure with evidence. States: idle, working, disabled. |
| **Orchestrator** | A deterministic, periodic rule engine with exactly five decisions per ticket: **skip** (nothing to do), **assign** (push to the least-busy role-matching agent), **advance** (success → next role or stage), **recover** (fail → previous step), **abandon** (heartbeat expired → release the work). It holds no intelligence and adds no verification gate of its own — trust comes from adversarial workflow structure. It can be enabled/disabled per project and explains every decision it takes as an audit event. A dry-run mode reports what it *would* do. |
| **Release** | A delivery container: in_design (features may be added/removed; membership propagates to the feature's whole subtree) → in_progress / sealed (feature set frozen; ready stories become executable) → complete. Stories not in a sealed release cannot enter delivery stages. |
| **Context graph** | The project's shared memory: documents (with uploaded files), external links, and tickets form nodes; typed edges connect them. All content is queryable — by node, by whole graph, and by text search. Goals carry attached context into refinement; agents receive relevant context with their work. The graph is visual: a human can see any story and what surrounds it. |
| **Mailbox** | The human escalation queue. Failed verification, repeated failure, and orchestration dead-ends create entries with recommendations; humans decide and the decision is recorded. |
| **History** | An append-only audit log. Every mutation — human, agent, or orchestrator — emits an event with actor, payload, and timestamp. |

---

## 5. Functional Requirements

Requirements are numbered for traceability. SHALL = mandatory.

### 5.1 Work management

- **FR-1** The system SHALL support projects with unique prefixes, titles,
  descriptions, visibility (private/team/public), one default workflow, and
  one or more associated source repositories (CRUD-managed by project
  admins).
- **FR-2** The system SHALL generate stable, immutable, per-project
  sequential ticket keys (`PREFIX-N`).
- **FR-3** The system SHALL support the ticket types and parenting matrix in
  §4 and SHALL reject invalid parent/child combinations and cycles.
- **FR-4** Only leaf tickets SHALL be directly mutable in lifecycle; a parent
  ticket's stage/state SHALL be derived from its descendants: earliest stage
  among them; state success iff all succeed, else active if any is active,
  else fail if any failed, else idle. Derivation SHALL update ancestors
  transitively and emit history events.
- **FR-5** Tickets SHALL carry: draft flag (new tickets start draft),
  complete flag, archive flag, soft-delete flag, priority, ordering,
  estimates, health score, assignee, labels, comments, dependencies, time
  entries, and guidance maps.
- **FR-6** The system SHALL support full-text search over tickets and a
  kanban/board projection grouped by stage.

### 5.2 Lifecycle and workflow

- **FR-7** Lifecycle SHALL be modeled as stage + state with the valid
  combinations of §4; `active` SHALL require an assignee; rendered status
  SHALL always be `stage/state`.
- **FR-8** Workflows SHALL define ordered stages; stages SHALL hold ordered
  role assignments; both SHALL be CRUD-managed, reorderable, exportable,
  importable, and structurally validatable.
- **FR-9** Advancement SHALL be: within-stage role progression first, then
  stage-to-stage transition; success advances, fail regresses, each as a
  single atomic step. Completion of the final stage marks the ticket
  complete; completion SHALL be reversible (reopen restores the prior
  stage/role).
- **FR-10** Backlog stages (idea/refine/ready) SHALL gate delivery: a ticket
  at `ready` SHALL NOT advance until it belongs to a sealed release.
- **FR-11** Guidance (DoR/DoD/AC) SHALL resolve per stage with `default`
  fallback at project, role, and ticket level, compose across layers, and be
  delivered to agents with their work.

### 5.3 Refinement

- **FR-12** A draft ticket in a backlog stage SHALL be considered "in
  refinement"; refinement SHALL happen in place via the ticket's persisted
  comment thread, turn-based between refiner agent and human.
- **FR-13** A refiner turn SHALL produce exactly one of: question, ready
  proposal (refined description + acceptance criteria), or breakdown
  proposal (ordered draft child stories).
- **FR-14** The human SHALL be able to edit and **reorder the proposed
  decomposition before approval**, and the order SHALL persist.
- **FR-15** Approval SHALL: (single story) mark it ready; (breakdown) convert
  the parent to an epic and mark its live children ready — atomically, with
  history.
- **FR-16** Refinement SHALL be available both orchestrator-driven
  (asynchronous turns) and live (streaming dialogue), interchangeably, and
  SHALL survive interruption at any point.

### 5.4 Orchestration and agents

- **FR-17** The orchestrator SHALL implement exactly the five decisions of §4
  deterministically, on a configurable interval, with a one-shot dry-run
  mode, per-project enablement, and a history event for every decision
  ("why did this ticket move?" must be answerable from the audit trail).
- **FR-18** Assignment SHALL be push-only: agents request work and receive
  either an assigned ticket or nothing; agents SHALL NOT self-select work.
  Selection SHALL match the ticket's current role by name and prefer the
  least-busy agent.
- **FR-19** Agents SHALL authenticate separately from humans, register,
  heartbeat (including during long tool runs), and report results only for
  work still assigned to them; abandoned work SHALL be rejected as stale.
- **FR-20** The agent's reasoning backend SHALL be pluggable (configurable
  command/model per system, per project, and per agent), with prompts
  assembled from: the ticket, its guidance, its role, and its context.
- **FR-21** A failure-escalation mailbox SHALL exist per FR-26 and §3 Phase 3.

### 5.5 Verification (adversarial)

- **FR-22** Workflows SHALL be able to express adversarial sequences (e.g.
  Engineer → Reviewer → QA) where verifying roles differ from implementing
  roles; the system SHALL ensure the verifying agent for a step is never the
  agent that produced the work under review.
- **FR-23** A verification failure SHALL regress the work one step with the
  findings attached to the ticket's thread.
- **FR-24** Phase sign-offs (planning, implementation, verification) SHALL be
  recorded per ticket — who, when, note — and the sign-off policy (which
  phases require a human) SHALL be configurable per project.
- **FR-25** Evidence (e.g. change references, test results) SHALL be
  attachable to completion reports and visible to verifiers.
- **FR-26** Unresolvable failures SHALL create mailbox entries carrying
  machine recommendations (clarify goal / refine requirements / start over);
  human decisions SHALL be recorded and actionable from the entry.

### 5.6 Releases

- **FR-27** Releases SHALL implement the in_design → in_progress (sealed) →
  complete state machine; feature membership SHALL be mutable only while
  in_design and SHALL propagate across the feature's entire subtree.
- **FR-28** Features SHALL be deep-clonable (entire subtree, provenance
  recorded) to fork or extend functionality without disturbing the original.

### 5.7 Context graph and knowledge

- **FR-29** Projects SHALL hold documents (title, description, notes, body)
  with uploaded binary files and labels.
- **FR-30** A context graph SHALL connect tickets, documents, and external
  URLs with typed, validated, deduplicated edges; every document is always a
  node; tickets and URLs join when referenced.
- **FR-31** The graph SHALL be queryable: per node (what context does this
  story have?), whole-graph, and by text search across node content.
- **FR-32** The graph SHALL be visually explorable: a map of nodes and edges,
  clickable to open any node, searchable, and focusable on a single story
  with its direct context highlighted.
- **FR-33** Attached context SHALL flow to agents: a story's context (and its
  ancestors') is part of the work packet an agent receives.

### 5.8 People, access, and plans

- **FR-34** Users SHALL register/login with credential hashing fit for
  modern standards; sessions SHALL be revocable; phishing-resistant
  passwordless login (e.g. passkeys) SHOULD be supported for web and CLI.
- **FR-35** Authorization SHALL be two-level: system roles (admin/user) and
  per-project roles — observer (read), commenter (read + discuss), member
  (read/write work), admin (full project control). A user SHALL NOT touch a
  project they have no role in; public projects grant a default role.
- **FR-36** Teams SHALL group users and agents and be attachable to projects
  with a role.
- **FR-37** Project resolution SHALL be frictionless: explicit selection, or
  source-repository lookup from the caller's environment, or the user's
  default project — with "private" and "public" aliases. Unauthorized access
  attempts SHALL be logged and answered with an access-request path when the
  project accepts members.
- **FR-38** Plans SHALL define registration-time actions (auto-create
  private project, auto-join public team) and quotas (projects, tickets,
  team memberships, API calls/day); an admin SHALL manage plans and the
  default plan.
- **FR-39** Rate limiting SHALL protect authentication endpoints;
  registration SHALL be configurable: auto-approve, waitlist, or closed.

### 5.9 Interfaces

- **FR-40** Every capability SHALL be exposed through a complete,
  documented API; the API specification SHALL be machine-readable and kept
  verifiably in sync with the implementation.
- **FR-41** A command-line interface SHALL cover the full workflow for both
  humans and agents, scriptable and CI-friendly.
- **FR-42** A web interface SHALL provide: board and list views, the
  refinement dialogue (with streaming agent replies and breakdown
  reordering), release planning, the context graph view, documents, mailbox,
  workflow/role editors, and administration. UI logic SHALL be cleanly
  separated from a thin API client layer that mirrors the API spec.
- **FR-43** Live updates SHALL stream to connected clients (work changes,
  refinement turns) so the factory floor is observable in real time.
- **FR-44** The full dataset SHALL export to and import from a portable
  snapshot with round-trip fidelity.

### 5.10 Audit

- **FR-45** Every mutation SHALL emit an append-only history event with
  actor, type, payload, and timestamp; history SHALL be queryable per ticket
  and per project; orchestrator and agent actions SHALL be first-class
  actors in this trail.

---

## 6. Non-Functional Requirements

- **NFR-1 Simplicity of operation.** The whole system SHALL deploy as a
  single self-contained service with an embedded or single-dependency
  durable store. One process, one port, one volume is the ideal.
- **NFR-2 Determinism.** Given the same state, the orchestrator SHALL make
  the same decisions. No hidden randomness in the production line.
- **NFR-3 Recoverability.** Every loop (refinement, implementation,
  verification) SHALL tolerate crash, restart, and walk-away: state lives in
  the store, never only in memory.
- **NFR-4 Security.** Modern credential hashing, secure session handling,
  distinct agent credentials, security headers, and least-privilege project
  access. The POC trust model is role-based authorization; a policy engine
  is a later hardening step, and the design SHALL NOT preclude it.
- **NFR-5 Observability.** Health endpoint, metrics endpoint, structured
  request logging, and the audit trail of FR-45.
- **NFR-6 Testability.** The service boundary SHALL be defined as an
  interface with a contract test suite that any implementation (in-process
  or remote) must pass identically.
- **NFR-7 Portability of the spec.** Nothing in §§1–8 requires a specific
  language, framework, database, or transport. Concrete choices are
  confined to §9.

---

## 7. SDLC — How This System Shall Be Built

The factory must be built the way the factory itself works. The
implementation SDLC is part of the specification:

1. **Red/green test-driven development.** Every behavior lands as a failing
   test first, then the implementation, then refactor. The default test
   suite must always pass.
2. **Contract-first service boundary.** Define the service interface early;
   write one contract test suite; run it against every implementation of the
   boundary (local and remote). The API spec document is the source of truth
   and is validated against the implementation in CI.
3. **Staged testing.** Fast unit tests in the inner loop; interface tests
   (API + CLI) when the contract surface changes; full end-to-end browser
   tests when the UI changes; the complete suite plus lint before any
   merge. Coverage thresholds are enforced per package and ratcheted, never
   lowered.
4. **Phased delivery mirroring §3.** Build in vertical slices: each slice is
   refined (this document is the input), implemented, adversarially
   reviewed (a verifier other than the author — human or agent), and
   released behind the gates. The system should manage its own backlog as
   early as possible (dogfooding).
5. **Documentation as code.** The spec, API document, user guide, and
   lifecycle rules live in the repository and are updated in the same change
   as the behavior; drift between docs and implementation is a defect and is
   checked mechanically where possible.
6. **Quality gates.** Linting with zero tolerance, vulnerability scanning,
   reproducible builds, semantic versioning, and a one-command CI pipeline
   (verify + browser + publish) that any contributor can run locally.
7. **Suggested phase order:** (1) entities + lifecycle + contract tests;
   (2) API + CLI; (3) workflows/roles/guidance; (4) refinement loop;
   (5) orchestrator + agents; (6) releases; (7) verification/sign-offs +
   mailbox; (8) context graph; (9) web UI; (10) plans/quotas + hardening.

**Definition of Done for the implementation:** all functional requirements
demonstrable through the API and at least one human interface; contract,
unit, interface, and end-to-end suites green; the worked examples in §8
executable as written; the system managing its own remaining backlog.

---

## 8. Worked Examples

### Example A — From dirty goal to shipped feature

1. Priya creates a goal in project `SHOP`: *"Customers should be able to
   save their cart and come back later."* She attaches the checkout design
   document and a link to the analytics dashboard. Both become context-graph
   edges on the goal.
2. She clicks **Refine**. The refiner agent reads the goal and its context
   and asks: *"Should saved carts expire? Are they per-device or
   per-account?"* Priya answers in the thread: per-account, expire after 30
   days.
3. The agent proposes a breakdown: `SHOP-41 Persist cart per account`,
   `SHOP-42 Restore cart on login`, `SHOP-43 Expire saved carts after 30
   days`, `SHOP-44 Saved-cart analytics event`. Each arrives with a
   description and acceptance criteria, as ordered draft children.
4. Priya drags `SHOP-44` last, decides expiry can wait, and deletes
   `SHOP-43`'s draft. She clicks **Approve** — the goal becomes an epic, the
   three children become `ready` stories.
5. She adds the parent feature to release **"June Drop"** and seals it. The
   subtree joins the release; the orchestrator may now act.
6. The orchestrator assigns `SHOP-41` to `eng-agent` (role: Engineer, least
   busy). The agent receives a work packet: story, acceptance criteria,
   composed DoD (project + role + ticket), and the attached context. It
   implements, records a change reference, reports **success**.
7. Advance: the story moves to the Reviewer role. `review-agent` — a
   different agent, prompted to refute — finds the expiry field unhandled
   in the restore path and reports **fail**. The story regresses to
   Engineer with the findings attached.
8. `eng-agent` fixes it; Reviewer passes it; QA passes it; the story reaches
   `done/success`. The epic's derived state updates as each child lands.
9. When all stories in "June Drop" complete, Priya reviews the release,
   signs off, and marks it **complete**. Every step — every assignment,
   advance, regression, and sign-off — is in the history.

### Example B — Failure escalates to the mailbox

`SHOP-42` fails QA three times: the acceptance criteria say "restore within
2 seconds" but no environment in the workflow can measure it. The
orchestrator's recovery loop detects the repeat failure and files a mailbox
entry: *"Verification cannot satisfy AC as written. Recommend: refine the
requirement (make the latency budget testable) or clarify the goal."* Priya
picks **refine requirement**, edits the AC to reference the existing
performance harness, and the story re-enters implementation. The decision
and its reasoning are part of the record.

### Example C — The factory builds itself

The implementation team feeds *this document* into the factory as the
founding goal of project `FACTORY`. Refinement decomposes §5 into epics per
requirement group; the SDLC of §7 becomes project-level guidance maps;
adversarial review is configured as Engineer → Reviewer → QA from day one.
From that point forward, every change to the factory is produced by the
factory's own four phases. A release is sealed for each delivery phase in
§7.7.

### Example workflow definition (data, not code)

```
workflow: "Agile Delivery"
stages:
  - design   (backlog: idea, refine, ready precede this)   roles: [Product Owner, Business Analyst]
  - develop                                                 roles: [Engineer]
  - test                                                    roles: [Reviewer, QA Engineer]
  - done
rules:
  - state=active requires assignee
  - success advances role-then-stage; fail regresses one step
  - ready tickets require a sealed release to enter develop
```

---

## 9. Implementation Directive

Implement this in **Go** and **containers**, using best practices.
