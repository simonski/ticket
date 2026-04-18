# MVP release plan

## Goal

Assess how close `ticket` is to being used to manage its own development, then
drive it through a phased beta plan until it can operate as its own system of
record.

This document resolves the current ambiguities so work can be logged and
executed without reopening the same scope questions every session.

## Resolved assumptions

1. **Phase 1 is CLI + API only.** The website and TUI are explicitly deferred to
   Phase 4.
2. **Phase 1 uses the broad admin surface.** “Entities and entity management are
   done” means: projects, tickets, stories, labels, SDLCs, stages, roles,
   teams, users, agents, comments, dependencies, and time tracking all have
   agreed CLI/API CRUD and lifecycle behavior.
3. **Upgrade compatibility is mandatory.** The existing `.ticket/ticket.db` must
   remain upgradeable as the project evolves; no throwaway reset-only approach
   is acceptable for release work.
4. **Self-hosting readiness matters more than UI polish.** Installability,
   repeatable tests, backup safety, and day-to-day workflow reliability are the
   blockers for MVP.
5. **Release work is tracked in the `release` project.** All tickets, epics,
   bugs, and follow-ups for this plan should live there.

## Release phases

### Phase 0 — evaluation

Establish the executable baseline and prove the plan is grounded in the current
product rather than assumptions.

Current status:

- `QUICKSTART_CLIENT.md` passes through `cmd/tk-test`
- `QUICKSTART_SERVER.md` passes through `cmd/tk-test`
- `scripts/testharness.sh` now covers both scripting/count assertions and an
  SDLC workflow scenario
- focused Go regression checks pass for `./internal/store` and `./cmd/tk`
- SDLC role-routing defects found during this pass were fixed and covered

Outcome of this phase:

1. The quickstarts are a credible smoke contract for local and remote use.
2. The shell harness is now part of the repeatable CLI/workflow contract.
3. Core CRUD confidence is good for projects, tickets, stories, labels,
   dependencies, time entries, users, teams, roles, SDLCs, and stages.
4. Weaker broad-scope areas still need a deliberate pass before `mvp-1`
   sign-off: comments, ideas, decisions, and some agent/admin flows.

### Phase 1 — `mvp-1`

Prove the core product is operational without depending on the website or TUI.

Exit criteria:

- CLI and HTTP API support the full entity/admin surface listed above.
- SDLC lifecycle behavior is implemented and considered stable.
- Local mode and remote client/server mode are both installable and usable.
- Test coverage includes unit, integration, contract, and executable scripting
  harnesses/examples.
- The Phase 0 evaluation findings are closed or explicitly deferred.
- Backup/restore and database upgrade expectations are documented and exercised.
- Website and TUI are allowed to lag as long as they do not block CLI/API use.

### Phase 2 — `beta-1`

Run a multi-project server used by humans for real CRUD-management work.

Exit criteria:

- A shared server can host multiple active projects reliably.
- Human users can manage day-to-day work through the supported surfaces.
- Operational basics are documented: startup, auth, backup, restore, upgrade.

### Phase 3 — `beta-2`

Add semi-autonomous agent usage on top of the Phase 2 human workflow.

Exit criteria:

- Agents can authenticate, request work, and act safely in real projects.
- Human/agent boundaries and operational controls are documented.
- Failures are observable and recoverable without database corruption.

### Phase 4 — `beta-3`

Bring the website and TUI back as first-class supported capabilities.

Exit criteria:

- Website and TUI cover the agreed workflows without regressing CLI/API use.
- Parity gaps are documented and deliberately prioritized rather than accidental.

## Immediate preparation sequence

1. Create a project named `release`.
2. Record all self-improvement work there as epics, tickets, bugs, and follow-up
   tasks.
3. Define and automate a backup routine for `.ticket/ticket.db` before risky
   schema or workflow changes land.
4. Baseline the current state against Phase 1 exit criteria and open the missing
   work as release tickets. **Partially complete**: the evaluation report now
   exists in `reports/08-mvp-evaluation.md`.
5. Treat schema upgrade safety as a standing requirement for all release work.

## Current MVP assessment

1. **Executable baseline**
   - local quickstart: verified
   - remote client/server quickstart: verified
   - scripted CLI harness: verified
2. **Workflow stability**
   - SDLC stage/role initialization and stage-change role persistence were
     defective and are now fixed
   - workflow progression/regression is now covered more directly
3. **Broad CRUD confidence**
   - strong for core entities and lifecycle-heavy flows
   - still uneven for lower-frequency admin surfaces
4. **Main remaining `mvp-1` risk**
   - proving the weaker broad-scope entities are either solid enough to keep in
     scope or intentionally deferred

## Next assessment and delivery pass

1. Do a targeted CRUD-depth pass on comments, ideas, decisions, and agent/admin
   flows.
2. Decide explicitly which of those weaker areas remain in `mvp-1` scope.
3. Keep growing `scripts/testharness.sh` with scenario-based operator workflows.
4. Exercise backup/restore and upgrade expectations as release gates rather than
   documentation-only promises.
5. Identify the smallest set of release tickets still required to reach
   `mvp-1`.
