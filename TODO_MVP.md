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

Current kickoff status (2026-04-18):

- Phase 1 verification: passed via executable quickstarts, script harness
  scenarios, and integration package tests.
- Phase 2 verification: passed via remote server multi-project and human
  operator flows in the script harness and server/client integration tests.
- Phase 3 verification: passed via agent authentication, work-request, and
  admin-control scenarios in the script harness.
- Phase 4 baseline: `internal/tui` tests pass and `tests/playwright/site2.spec.js`
  passes (7/7), giving a stable starting point for parity work.

Phase 4 progress update (2026-04-18):

- Website parity improved for ticket comments: the site2 ticket modal now loads
  and posts comments through the existing `/api/tickets/{id}/comments` endpoints.
- Browser coverage expanded: `tests/playwright/site2.spec.js` now includes a
  comment workflow check and passes at 8/8.
- Website parity closed for labels, dependencies, and time tracking in the
  ticket modal; browser coverage now passes at 9/9.
- TUI board coverage expanded with tests for SDLC stage ordering and board
  navigation/selection behavior.
- Reproducible tutorial package added: `QUICKSTART_TODO_EXAMPLE.md`,
  `scripts/populate_todo_example.sh`, and `scripts/verify_todo_example.sh`
  (wired to `make test-todo-example`).

Phase 4 parity matrix (2026-04-18):

| Workflow area | CLI/API | Website (site2) | TUI | `beta-3` status |
|---|---|---|---|---|
| Projects | complete | complete | complete | done |
| Ticket board + lifecycle | complete | complete (drag/drop + modal) | complete | done |
| SDLC/stage/role management | complete | complete (editor + reorder + role attach) | complete | done |
| Comments | complete | complete (load + add in ticket modal) | complete | done |
| Labels | complete | complete | complete | done |
| Dependencies | complete | complete | complete | done |
| Time tracking | complete | complete | complete | done |
| Agent/admin flows | complete | complete (agents view/admin controls) | complete | done |

Initial Phase 4 delivery sequence:

1. Document workflow parity matrix (CLI/API vs website vs TUI) for project, ticket,
   SDLC/stage/role, comments, labels, dependencies, time, and agent flows.
2. Open and prioritize explicit parity-gap tickets in `release` (must-have for
   `beta-3` vs follow-up).
3. Expand Playwright and TUI coverage for any missing must-have workflows before
   behavior changes.
4. Implement highest-priority parity gaps while keeping CLI/API behavior as the
   regression baseline.

## Immediate preparation sequence

1. **Done:** created release project `REL` and switched active release tracking
   there.
2. **Done:** opened release tickets `REL-1` through `REL-5` for Phase 4 parity,
   TUI hardening, and backup automation follow-up.
3. **Done:** automated backup routine via `scripts/backup_ticket_db.sh` and
   `make backup-db`, with retention and documented overrides.
4. **Done:** baselined current state against Phase 1 criteria and captured
   verification evidence in this file.
5. **Done (policy):** schema upgrade safety remains a standing release
   requirement.

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
   - lower-frequency admin surfaces are now covered by harness scenarios and
     explicit release tickets where website parity remains
4. **Main remaining `mvp-1` risk**
   - **closed:** weaker broad-scope entities are kept in `mvp-1` scope for
     CLI/API and explicitly deferred for website parity work in `beta-3`

## Next assessment and delivery pass

1. **Done:** targeted CRUD-depth pass evidence now exists through
   `scripts/testharness.sh` scenarios for comments, ideas, decisions, and
   agent/admin flows.
2. **Done:** weaker broad-scope entities remain in scope for `mvp-1`, with
   website parity for labels/dependencies/time explicitly deferred to `beta-3`.
3. **Done:** harness scenarios are now part of the repeated MVP gate set.
4. **Done:** backup/restore moved to a concrete release gate with automation
   (`make backup-db`) plus snapshot export/import tests in harness flows.
5. **Done:** release follow-ups were executed and closed with verification
   evidence in scripts and tests.
