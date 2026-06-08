# DESIGN_ORCHESTRATOR.md

> **Status: IMPLEMENTED (Phases 1–6).** The deterministic execution orchestrator
> AND the interactive backlog-preparation loop are built and tested on the
> `feature/orchestrator` branch. Sections below are the living design record;
> **✅ RESOLVED** marks agreed decisions.

## Implementation status

| Phase | Scope | Status |
|------|-------|--------|
| 1 | Orchestrator core — deterministic `Pass()` (assign / advance / recover / abandon / skip) | ✅ `internal/orchestrator` |
| 2 | Remove agent self-claim; push model | ✅ server + LocalService; `findClaimCandidate` no longer in the agent poll path |
| 3 | `tk server -orchestrator` background worker; config (interval, heartbeat timeout, per-project on/off) + API + UI | ✅ |
| 4 | `tk orchestrator [-id N \| N \| -project_id N] [-apply]` dry-run CLI | ✅ |
| 5 | Heartbeat during LLM; abandonment guard (stale-result rejection); audit history events | ✅ |
| 6 | Interactive backlog preparation loop (idea→refinement→breakdown) | ✅ refinement dialogue (comments), orchestrator-driven, human approval, idea→epic breakdown |

### Phase 6 — preparation loop (as built)

Decisions (Q9): the refinement **dialogue reuses the ticket comment thread** (the
refiner and human exchange comments — persisted, ticket-scoped, presented in the UI
as a dedicated refinement chat); the human ends it with **explicit approval**; on a
multi-story **breakdown the idea becomes an epic** and its proposed child stories
become ready; and the **orchestrator pushes refinement** to refiner agents (the
refiner no longer self-pulls).

Turn-based flow, all driven by the deterministic orchestrator:
1. A human submits an idea into the `refine` stage.
2. Each orchestrator pass: if a `refine`-stage ticket is idle and it is the
   **agent's turn** (the latest comment is the human's, or there are none yet), it
   assigns a refiner agent (`assignee` + active). `RefinementDialogueTurn` /
   `ListOrchestratorCandidates.refinement_agent_turn` computes whose turn it is.
3. The refiner agent reads the idea + thread, posts one reply, and either asks
   questions, proposes a single ready story (`recommended_ready`, refined
   description/AC), or proposes a breakdown (creates draft child stories). It then
   releases the ticket to idle (awaiting the human).
4. The human replies (continuing the dialogue → agent's turn again) or **approves**
   (`POST /api/tickets/{id}/refinement/approve`): single → `MarkTicketReady`;
   breakdown → re-type to epic + children ready.

Key code: `internal/store/refinement.go` (`RefinementDialogueTurn`,
`ApplyRefinementTurn`, `ApproveRefinement`, `AddRefinementProposalChild`),
agent action `POST /api/agents/tickets/{id}/refine`, `decideIdle` refine branch,
agent runtime `buildRefinementPrompt`/`parseRefinementOutput`. History events:
`refinement_approved_breakdown`. Note: the dialogue medium is the comment thread
(no new table / schema-version bump), which the refinement UI renders as a chat.

Key files: `internal/orchestrator/orchestrator.go`, `internal/store/orchestrator.go`,
`internal/server/server.go` (`runOrchestrator`), `internal/server/api_agents.go`
(push model + abandonment guard), `cmd/tk/cmd_orchestrator.go`. History event types:
`orchestrator_assigned`, `orchestrator_advanced`, `orchestrator_recovered`,
`orchestrator_abandoned`. "Sealed sprint" == a sprint in stage `active`.

Notable behaviour: refiner agents are now pushed refinement turns by the
orchestrator (no more self-pull); `tk agent request -id X` still performs an
explicit manual claim of a named ticket (a human directing work, not the agent
self-selecting).

### Decisions settled so far
- **A — Assignment is push-only.** Agents do **not** self-claim. The agent poll is
  simply *"what work is there for me?"*; the orchestrator has already decided, so the
  response is either *"here is the ticket you're assigned"* or *"no work for you"*.
- **B — The orchestrator is NOT LLM-assisted; it is purely deterministic.** The
  intelligence lives in the **agent** (which runs an LLM to do the work and to
  self-assess success/fail). The orchestrator only applies workflow rules.
- **C — `tk orchestrator N` is shorthand for `-id N`** (a single ticket); projects
  use `-project_id N`.
- **D — The orchestrator drives both loops** (backlog preparation *and* sprint
  execution). The preparation loop is the harder part because refinement is, by
  design, an interactive LLM+human dialogue.
- **Q1 — Trust comes from adversarial multi-role workflows**, not from the
  orchestrator re-checking. It takes `success`/`fail` at face value and relies on
  downstream roles (Reviewer, QA, …) to scrutinise upstream work.
- **Q2 — Assignment is load-balanced** to keep agents occupied; the choice among
  eligible agents is otherwise unimportant.
- **Q3 — Heartbeat + timeout + abandonment.** Agents heartbeat continuously,
  *including during long prompt/LLM phases*. If heartbeats stop, the orchestrator
  marks the job **abandoned / timed-out**; when that agent next wakes it sees the
  abandoned marker and drops the work it was doing.
- **Q4 — Exactly one orchestrator.** Never more than one instance.
- **Q5 — A "sealed" sprint is the green light for execution.** Sealing means every
  story in the sprint is `ready`; once sealed, agents/orchestrator may work them.
- **Q6 — One server-wide orchestrator with per-project on/off flags.**

**All Phase-1 gates are resolved.** Remaining open items (Q7 template engine, Q8
observability, Q9 interactive refinement) are later-phase and do not block Phase 1.

---

## 1. Purpose

Today, ticket movement through a workflow is **reactive and manual**: a ticket only
advances when someone (CLI, API, or a self-claiming agent) explicitly acts on it.
There is **no component that autonomously coordinates work**.

The **Orchestrator** is that component. It periodically wakes, inspects the state of
all tickets across all programmes/projects, and decides what should happen next —
who works which story, in which role, and when a story advances to the next stage.

The orchestrator is the **single source of assignment authority**. Everything else
(agents finishing work, marking success/fail) is *advice* the orchestrator observes
and acts upon.

---

## 2. Vocabulary (target model)

| Term | Meaning |
|------|---------|
| **Workflow** | The ordered sequence of steps a story goes through, start to end. Pluggable: assigned per-project. |
| **Stage** | One discrete step / area of work in a workflow (e.g. `design`, `develop`, `test`). Rendered as board columns. |
| **Role** | A job description that implements a stage (e.g. Engineer, Tester, Architect, Product Manager). A stage has one or more ordered roles. |
| **Story** | The unit of work. (Implemented as a ticket.) |
| **Agent** | The worker that executes a story. Marks it `active`, does the work, then self-assesses `success`/`fail` and **un-assigns itself**. |
| **Orchestrator** | The coordinator. The **only** thing that assigns work to agents. |
| **Sprint** | A grouping of stories to be worked together. Prepared in the backlog, then "sealed" and executed. |
| **Epic** | A container of multiple stories produced by breaking down an idea. |

### The composed prompt

A story's working **goal** is composed from:

```
Story + Role + Stage + Project + Organisation  ->  filled template  ->  prompt for the LLM/agent
```

This combined prompt describes the goal, acceptance criteria, expectations, how to
fulfil the work, and all technical information.

---

## 3. The two agentic loops

The model contains **two distinct agentic flows** that the orchestrator coordinates.
Keeping them separate is important.

### 3.1 Preparation loop (backlog)

Turns a raw requirement into sprint-ready, well-formed stories.

```
idea  ->  refinement  ->  breakdown
```

- **idea** — the user's first text description of a requirement.
- **refinement** — agent and user iterate (clarification dialogue) until both agree
  the requirement is unambiguous and complete.
- **breakdown** — refinement may reveal the idea is several stories. If the user
  accepts, the stories are **packaged into an epic** (a container ticket) with the
  specific stories as children.

Once stories are ready, they are allocated into a sprint; the sprint is **sealed**
and handed to the execution loop.

### 3.2 Execution loop (sprint)

Moves sealed stories through the project's workflow, stage by stage, role by role,
until done — coordinated entirely by the orchestrator.

```
for each (stage, role) in workflow:
    orchestrator assigns story to an agent for that role  ->  state = active
    agent works, self-assesses  ->  state = success | fail, un-assigns
    orchestrator wakes, reads result, decides next move:
        success -> advance to next role (same stage) or next stage (state = idle/ready)
        fail    -> move back / re-assign / escalate
```

---

## 4. Current state of the codebase (what exists today)

Grounded findings from the current implementation:

### 4.1 Workflow / Stage / Role — **EXISTS**
- Workflows, stages, roles, and the stage→role junction all exist with full CRUD.
- A ticket tracks `stage`, `role_id`, and lifecycle `state` (`idle|active|success|fail`).
- Pluggable per project: a project has a `workflow_id`; the board derives columns
  from the workflow's stages (with `is_backlog_stage` distinguishing backlog vs sprint).

### 4.2 Advancement logic — **EXISTS, but reactive only**
- `internal/store/ticket.go`:
  - `findNextStep()` / `findPrevStep()` — compute the next/prev `(stage, role)`:
    within-stage role advancement first, then stage-to-stage transition (via
    `workflow_stage_transitions`, falling back to linear order).
  - `NextTicket()` requires `state=success`; `PreviousTicket()` requires `state=fail`;
    both reset `state=idle` on entering a new stage.
  - `UpdateTicket()` has a conditional **auto-advance** when `state=success` is set
    *and* the workflow's approval policy is `all_roles` and progression mode is not
    `stage_only`.
- Sprint gating: a ticket at `ready` cannot advance unless assigned to a sprint
  (`ticket.go` ~L869).
- **There is no background process that triggers any of this.** It only happens on an
  explicit CLI/API call.

### 4.3 Background workers — **PATTERN EXISTS**
- `internal/server/server.go` already runs background goroutines:
  - `runAgentReaper()` — 1-minute ticker, marks stale agents idle.
  - `runRetentionPurge()` — 24-hour ticker, purges expired sessions/history.
- This is the pattern the orchestrator loop would follow. The server command is
  **`tk server`** (handled in `cmd/tk/cmd_setup.go`).

### 4.4 Agent work assignment — **EXISTS, but PULL-based** ⚠️
- Agents currently **pull** work: `cmd/tk/cmd_agent.go` polls `RequestAgentWork`,
  the server calls `store.RequestTicket()` → `findClaimCandidate()`, which finds an
  idle, unassigned, leaf ticket whose **current role title** matches one of the
  agent's roles (case-insensitive), and the agent **claims it itself**.
- The agent then runs an LLM, writes the result back, and (for refiner) posts a
  comment + sets `recommended_ready`.

> **✅ RESOLVED (Decision A) — Push-only assignment; the poll is a query.**
> The agent's poll is just *"what work is there for me?"*. The orchestrator has
> already decided. The server response is either the ticket already assigned to that
> agent, or *"no work for you"*. Therefore `findClaimCandidate()` self-assignment is
> **removed** from the work-request path: `RequestAgentWork` returns only the agent's
> own already-assigned active ticket (the existing `findAssignedTicketForUser` path),
> never claims one. The assignment *mechanism* (set `assignee` + `state=active`) is
> reused verbatim by the orchestrator, so the agent runtime is otherwise unchanged.

### 4.5 Preparation loop — **PARTIAL**
- A `refiner` agent role exists; `RequestRefineTicket()` claims draft backlog tickets.
- `SetRecommendedReady()` flags a ticket as recommended-ready; `MarkTicketReady()`
  promotes it (separate, manual step).
- **One-way only:** refiner posts a comment; there is **no interactive
  clarification dialogue** between agent and user.
- **No breakdown/decomposition:** epics and parent/child links exist, but there is
  no operation that splits an idea into multiple stories under an epic. Any ticket
  type can parent any other; no epic-specific logic.

### 4.6 Prompt composition — **PARTIAL**
- `internal/store/execution_packet.go` (`BuildExecutionPacket`) and
  `EnrichTicketContext()` assemble structured context (project, parents, workflow,
  role, layered DOR/DOD/AC guidance).
- `cmd/tk/cmd_agent.go` builds the actual prompt via hard-coded `strings.Builder`
  templates (`buildAgentPrompt`, `buildRefinerPrompt`).
- **No reusable, admin-editable template engine** that "fills in the blanks" from
  Story + Role + Stage + Project + Organisation.

### 4.7 Sprints — **PARTIAL**
- Sprints exist with stages `active` / `closed` (`internal/store/sprint.go`).
- A sprint cannot be activated if tickets are still in `idea`/`refine`.
- **No explicit "seal" concept** — "sealing" currently maps loosely to activation.

### 4.8 Config — **EXISTS (KV store)**
- `internal/store/settings.go` provides an app-settings key/value store
  (`ListAppSettings` / `SetAppSetting` / `DeleteAppSetting`) with typed wrappers
  (e.g. `RegistrationEnabled`, `ChatLimitsConfig`).
- A "Settings" admin panel exists in the web UI. A new orchestrator-frequency
  setting fits this pattern directly.

### 4.9 Orchestrator — **DOES NOT EXIST**
- No orchestrator code, no `tk orchestrator` command, no `-orchestrator` flag.

---

## 5. The Orchestrator — proposed design

### 5.1 Responsibilities
1. **Observe** all tickets across all programmes/projects on each wake.
2. **Decide**, per ticket, the next action based on the ticket's workflow + state.
3. **Assign** work to agents (the only component permitted to do so).
4. **Advance** tickets whose work is complete (success → next; fail → back/escalate).
5. **Gate** correctly: respect backlog/sprint rules, sealed sprints, leaf-only work,
   parent derivation.

### 5.2 The assignment handshake (Decision A — settled)

The orchestrator pushes; the agent's poll is a pure *"anything for me?"* query.

```
Orchestrator (deterministic, push)          Agent (LLM, execute only)
----------------------------------          -------------------------
picks idle, unassigned, in-sprint story
whose current role matches an available
agent for that role
   -> sets assignee = agent, state = active
                                            polls: "what work is there for me?"
                                            server answers with EITHER
                                              - the ticket assigned to it, OR
                                              - "no work for you"
                                            (server NEVER self-assigns)
                                            runs LLM, does the work, self-assesses
                                            HEARTBEATS THROUGHOUT (incl. LLM phase)
                                            -> state = success|fail, assignee = ""
wakes, sees success/fail, decides next
   -> advance (success) or recover (fail)

  ── abandonment branch (Q3) ──
if heartbeats stop past the timeout:
   -> mark job abandoned/timed-out,
      release ticket to idle (re-assignable)
                                            agent eventually wakes, sees its ticket
                                            was marked abandoned -> DROPS the work,
                                            does not report a stale result
```

The agent runtime and polling transport are largely unchanged; the server's
work-request path stops self-claiming (returns only the agent's already-assigned
active ticket), and the agent must (a) heartbeat during LLM execution and (b)
recognise the "abandoned" marker on a ticket it thought it held.

### 5.3 Decision making

For each candidate ticket the orchestrator evaluates, using the ticket's workflow:
- `state=success` → compute `findNextStep()`; advance role/stage; set `state=idle`
  (or `ready` when crossing into a sprint stage); un-assign. **No extra verification
  gate** — trust comes from the workflow's adversarial downstream roles (Q1).
- `state=fail` → `findPrevStep()` / re-assign same role / escalate (policy TBD). A
  downstream adversarial role reporting `fail` is the normal way work bounces back.
- `state=idle` + in active sprint + leaf + a role-matching agent is **free** → assign
  to the least-busy matching agent (load-balanced; order otherwise unimportant — Q2).
- `state=active` → in progress, leave alone (unless stale — reaper concern).
- backlog stages → **in scope (Decision D):** route the ticket into the preparation
  loop (refinement/breakdown). The orchestrator decides *when* a ticket enters
  refinement and reacts to the outcome; the refinement dialogue itself is performed
  interactively by the agent + human (see Open Question 9).

The "trust" model (Q1) is why a real workflow is built from **multiple roles that
check each other** (e.g. Engineer → Reviewer → QA). The orchestrator never judges
quality; it only routes work through those roles, and the adversarial sequence is
what makes the combined output trustworthy.

> **✅ RESOLVED (Decision B) — The orchestrator is purely deterministic; NOT
> LLM-assisted.** It applies workflow rules only (`findNextStep`, sprint gating,
> role matching, leaf/parent checks). All intelligence lives in the **agent**, which
> runs an LLM both to perform the work and to self-assess `success`/`fail`. The
> dry-run CLI (§5.6) is therefore also deterministic: it reports the mechanical
> action the orchestrator would take, with no LLM reasoning.

### 5.4 Running

- Started as part of the server: `tk server -orchestrator`.
  *(Note: the existing command is `tk server`, not `tk serve`. Confirm flag name.)*
- Runs as a background goroutine alongside the existing reaper/purge workers.
- One orchestrator pass = one transaction-safe sweep over eligible tickets.

### 5.5 Config

- Wake frequency is an **admin-only** config value, editable in the UI and CLI,
  stored via the existing app-settings KV store
  (e.g. key `orchestrator_interval_seconds`).
- **Heartbeat timeout** (Q3): how long without a heartbeat before a job is marked
  abandoned (e.g. `orchestrator_heartbeat_timeout_seconds`).
- **Per-project on/off** (Q6): a per-project flag to include/exclude a project from
  the single server-wide orchestrator.
- Possibly later: max concurrent assignments.

### 5.6 Dry-run / assessment CLI

Admin-only. Produces a summary of the action(s) the orchestrator *would* take,
without mutating state:

```
tk orchestrator -id N           # single ticket: the action it would take
tk orchestrator N               # shorthand for -id N
tk orchestrator -project_id N   # all tickets in a project, each with its action
```

Output: a per-ticket list of `(ticket, current stage/role/state) -> proposed action`.

> **✅ RESOLVED (Decision C) — Shorthand confirmed.** `tk orchestrator N` is exactly
> shorthand for `tk orchestrator -id N` (a single ticket). Projects always use the
> explicit `-project_id N`.

---

## 6. Decisions & open questions to resolve together

> **✅ RESOLVED (Decision A) — Orchestrator-only assignment; remove self-claim.**
> Agents only execute assigned work. `findClaimCandidate()` is removed from the
> work-request path. The assignment *mechanism* (set `assignee` + `state=active`) is
> reused unchanged by the orchestrator, so the agent runtime stays the same.

> **✅ RESOLVED (Decision D) — The orchestrator drives BOTH loops.** It coordinates
> backlog preparation *and* sprint execution. Execution is mostly mechanical and will
> be built first. Preparation is harder: refinement is, by design, an **interactive
> LLM+human dialogue**, so the orchestrator's role there is to *route* a ticket into
> refinement and react to its outcome — not to conduct the dialogue itself (the
> agent + human do that). The interactive-refinement design is called out as its own
> work item (see §7, and Open Question 9).

**Resolved:**

> **✅ RESOLVED (Q1) — Trust via adversarial multi-role workflows, not re-checking.**
> The orchestrator takes the agent's `success`/`fail` at face value — it *is* that
> agent's output, nothing more. Correctness is not guaranteed by any single role.
> Instead, **trust is a property of the workflow design**: a stage/workflow composes
> multiple roles that run separately and **adversarially** (e.g. Engineer →
> Reviewer → QA), so a later role independently scrutinises the earlier role's work.
> The combined, multi-role output is what addresses the trust issue. The orchestrator
> therefore does **not** add a verification gate of its own; it simply advances
> through the roles the workflow defines, and a downstream role's `fail` is the
> mechanism that sends work back. (Workflow authors get trustworthiness by adding
> adversarial roles, not by configuring the orchestrator.)

> **✅ RESOLVED (Q2) — Assignment is load-balanced; order is unimportant.** When
> several agents can perform a role, the choice between them does not matter for
> correctness. The orchestrator's only goal is to **keep agents occupied and balance
> load**: prefer agents that are currently free (no active assignment), and spread
> work so no agent sits idle while another is overloaded. No round-robin ordering or
> priority semantics are required beyond "balanced + busy".

> **✅ RESOLVED (Q3) — Heartbeat / timeout / abandonment protocol.**
> The agent must **heartbeat continuously**, *including while a prompt/LLM phase is
> executing* (long LLM calls must not look like a hang). If heartbeats stop for
> longer than a timeout, the orchestrator marks the in-flight job **abandoned /
> timed-out** — the ticket is released back to `idle` (re-assignable) and flagged so
> history records the timeout. The orchestrator owns this (it already sweeps
> everything; the existing 1-min agent reaper informs staleness). When the timed-out
> agent next makes contact, it sees the abandoned marker for the ticket it thought it
> held and **drops that work** rather than reporting a stale result.
> *Design implications:* (a) the agent runtime must emit heartbeats during LLM
> execution, not only between polls; (b) "abandoned" needs to be representable so the
> agent can detect "the ticket I was working is no longer mine".

> **✅ RESOLVED (Q4) — Exactly one orchestrator.** There is never more than one
> orchestrator instance. Single-instance is a hard invariant (a single in-process
> background worker). Assignment still uses a guarded conditional update
> (`SET assignee=?, state='active' WHERE assignee='' AND state='idle'`) as
> belt-and-braces, but no multi-instance coordination is designed for.

> **✅ RESOLVED (Q5) — "Sealed" sprint = the execution green light.** A sprint is
> sealed only when **every story in it is `ready`**; sealing declares the sprint's
> stories available for agents/orchestrator to work. Before sealing, the orchestrator
> ignores the sprint's stories for execution. (Exact storage — a new sprint state vs.
> a `sealed` flag alongside `active`/`closed` — and reversibility are an
> implementation detail for the sprint model; the *meaning* is settled.)

> **✅ RESOLVED (Q6) — One server-wide orchestrator, per-project on/off.** A single
> orchestrator loop spans the whole server (all programmes/projects), with a
> per-project enable/disable flag so admins can opt individual projects in or out.

**Later-phase open questions:**
7. **Prompt template engine.** Is the admin-editable Story+Role+Stage+Project+Org
   template part of this work, or a separate effort the orchestrator depends on?
8. **Observability.** History/audit events for every orchestrator decision; a way to
   see "why did this ticket move?".
9. **Interactive refinement (prep loop).** Refinement is an LLM+human dialogue. How
   does it work end-to-end? Where does the conversation live (comments? a dedicated
   chat?), how does the human signal "agreed", how does breakdown produce child
   stories under an epic, and what state/flags tell the orchestrator the ticket is
   refined and ready? This is the hardest sub-design (Decision D) and is sequenced
   after execution.

---

## 7. Proposed phased plan

All Phase-1 decisions are settled (A, B, C, D, Q1–Q6). Phase 1 is fully specified
and ready to build on request. Q7–Q9 are later-phase and do not block it.

1. **Phase 1 — Orchestrator core (execution loop, deterministic).**
   - `internal/orchestrator` package: one `Pass(ctx, db)` that sweeps eligible
     tickets and returns a list of decisions (advance / assign / recover / skip).
   - Reuse `findNextStep`, sprint gating, role-matching for candidate selection.
   - Assignment = set `assignee` + `state=active` (same mechanism self-claim used).
2. **Phase 2 — Remove agent self-claim (Decision A).**
   - `RequestAgentWork` stops calling `findClaimCandidate`; it returns only the
     agent's already-assigned active ticket, else "no work for you".
3. **Phase 3 — Wiring & config.**
   - `tk server -orchestrator` starts the loop as a background worker (alongside
     reaper/purge).
   - `orchestrator_interval_seconds` app setting; admin UI + CLI to edit it.
4. **Phase 4 — Dry-run CLI.** `tk orchestrator [-id N | N | -project_id N]`
   producing per-ticket proposed actions (deterministic, no LLM).
5. **Phase 5 — Hardening.** Stale-active recovery, contention strategy, idempotency,
   audit/history of every decision, tests (red/green per CLAUDE.md).
6. **Phase 6 — Preparation loop (Decision D).** Orchestrator routes backlog tickets
   into refinement; design the interactive LLM+human refinement dialogue, breakdown
   into epic+child stories, and the "refined/ready" signal (Open Question 9).
7. **Later — Sprint sealing semantics, admin-editable prompt-template engine.**

---

## 8. Notes / conventions
- Authoritative lifecycle spec: `docs/LIFECYCLE.md`.
- Follow repo rules in `CLAUDE.md` (red/green tests, `make test`/`make lint`,
  update `docs/DESIGN.md` + guides when code changes).
- This doc is the working spec; we iterate here until coherent, then execute.
