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

### Phase 1 — `mvp-1`

Prove the core product is operational without depending on the website or TUI.

Exit criteria:

- CLI and HTTP API support the full entity/admin surface listed above.
- SDLC lifecycle behavior is implemented and considered stable.
- Local mode and remote client/server mode are both installable and usable.
- Test coverage includes unit, integration, contract, and executable scripting
  harnesses/examples.
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
   work as release tickets.
5. Treat schema upgrade safety as a standing requirement for all release work.

## First assessment pass to perform next

1. Inventory Phase 1 entity/API/CLI coverage and mark each area as:
   - done
   - partial
   - missing
2. Inventory the test surface for Phase 1:
   - unit
   - integration
   - contract
   - scripted CLI harnesses
   - executable documentation
3. Inventory install/run paths:
   - local CLI
   - remote CLI against server
   - server bootstrap and persistence
4. Identify the smallest set of tickets required to reach `mvp-1`.
