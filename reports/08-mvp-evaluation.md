# MVP evaluation report

## Objective

Establish an explicit evaluation phase before `mvp-1`:

1. verify that the commands shown in the quickstarts still work end-to-end
2. determine which entity CRUD/lifecycle flows are covered by
   `scripts/testharness.sh`, CLI tests, API tests, store tests, and contract tests
3. identify the missing work required for a self-hosting MVP
4. harden SDLC/workflow behavior so the lifecycle commands work reliably

## Verified execution baseline

- `QUICKSTART_CLIENT.md` passes through `cmd/tk-test`
- `QUICKSTART_SERVER.md` passes through `cmd/tk-test`
- `scripts/testharness.sh` passes
- broader Go checks pass for `./cmd/tk`, `./internal/server`, `./internal/store`,
  `./internal/client`, and `./libticket`

Verification commands used:

```bash
go build -o ./bin/tk ./cmd/tk
./scripts/testharness.sh
go run ./cmd/tk-test -ticket ./bin/tk QUICKSTART_CLIENT.md QUICKSTART_SERVER.md
```

## Current coverage summary

### Strong coverage already present

- **Projects, tickets, stories, labels, teams, users, roles, SDLCs, stages,
  dependencies, time entries** all have meaningful automated coverage across a
  mix of CLI tests, API tests, store tests, contract tests, or quickstart
  execution.
- **Quickstart workflows** already exercise the highest-value user-facing paths:
  project setup, ticket capture, lifecycle moves, labels, decisions, ideas,
  time logging, server mode, login/register, request/claim, and Claude skill
  setup.
- **Store-level SDLC behavior** is already strong for happy paths: stage
  ordering, stage-role wiring, parent lifecycle recalculation, import/export,
  and next/previous step resolution.

### Coverage matrix

| Area | Quickstarts | Harness / CLI / API / store | Assessment |
| --- | --- | --- | --- |
| Project lifecycle | yes | strong | good |
| Ticket/task/bug/epic lifecycle | yes | strong | good |
| Story CRUD | no | strong | good |
| Label / dependency / time flows | yes | strong | good |
| Team / user / role / SDLC admin flows | partial | strong | good |
| Comments / ideas / decisions / agents | partial | now stronger | improved in this pass, but still worth explicit MVP scope review |
| SDLC workflow progression / regression | partial | now strong | improved in this pass |

### Verified weak points

- The quickstarts do **not** cover every admin/entity command in the broad MVP
  scope, so the evaluation has to rely on a blend of quickstart execution and
  targeted tests instead of quickstarts alone.
- Some lower-frequency admin surfaces are now better covered at the CLI/harness
  layer, but still remain thinner than core ticket/project flows if they remain
  in `mvp-1`: comments full lifecycle, idea/decision lifecycle depth, and some
  agent/admin flows still need deliberate scope review.
- The explicit `done` stage contract is now tested, but it is more nuanced than
  the casual reading suggests: the verified flow is `... -> done/idle`,
  `success -> done/success`, then `next -> complete`.

## Work todo

### Must do for the evaluation phase

1. Expand the shell harness beyond ticket counts to cover SDLC workflow
   progression/regression. **Done**
2. Add direct regression coverage for SDLC stage-role workflow behavior at the
   CLI/store level. **Done**
3. Produce an entity-by-entity coverage picture showing where CRUD/lifecycle
   behavior is verified and where it is not. **Done at summary level in this
   report; remaining follow-up is to deepen the weaker entity groups**

### Likely follow-up after the first pass

1. Decide whether weaker entities are truly in `mvp-1` scope or should move to a
    later phase.
2. Add more scripted or CLI coverage for the remaining broad admin surface if we
   keep that broad scope.
3. Review lifecycle consistency questions such as terminal-state semantics after
   the direct CLI coverage lands.

## Progress updates

### Update 1

- Quickstart execution is green for both local and server flows.
- The evaluation established that quickstarts are a strong smoke contract but
  not a complete CRUD contract for the whole admin surface.

### Update 2

- Expanded `scripts/testharness.sh` from count assertions into a second scenario
  that exercises SDLC workflow setup plus regression/terminal-stage behavior.
- Added store and CLI regression coverage around workflow role assignment and
  stage/role progression.
- Fixed a real SDLC bug: tickets created under a project or explicit SDLC now
  pick up the first role of the first workflow stage, and workflow transitions
  now persist the correct role when the stage changes.
- Re-verified:
  - `go test ./internal/store ./cmd/tk`
  - `./scripts/testharness.sh`
  - `go run ./cmd/tk-test -ticket ./bin/tk QUICKSTART_CLIENT.md QUICKSTART_SERVER.md`

### Update 3

- Fixed two CLI/documentation mismatches in the weaker Phase 1 entity group:
  `tk idea revise` now works, and `tk decision` now accepts the documented
  `new` / `ls` aliases while inheriting the shared creation flags such as
  `-printid`.
- Expanded `scripts/testharness.sh` again so it now covers:
  - comment + `get` visibility
  - idea creation + revise alias
  - decision creation + list alias
  - snapshot export/import restore behavior
  - remote server login, multi-project switching, and `tk agent request`
- Expanded CLI integration coverage for the remote agent request path using a
  real httptest server.
- Re-verified:
  - `go test ./cmd/tk ./internal/server ./internal/store ./internal/client ./libticket`
  - `./scripts/testharness.sh`
  - `go run ./cmd/tk-test -ticket ./bin/tk QUICKSTART_CLIENT.md QUICKSTART_SERVER.md`

## Concrete findings from this pass

1. **The documented quickstarts are executable today.** That is a strong
   baseline for `mvp-1`.
2. **The SDLC role-routing path had a real defect.** Tickets were getting the
   correct stage but not the correct role assignment on creation and subsequent
   stage changes. This is now fixed and covered.
3. **The workflow contract is now clearer than before.** The tests and harness
   now verify:
   - project SDLC assignment
   - initial stage-role selection for new tickets
   - success-driven stage progression
   - fail + `previous` regression
   - explicit `done` stage terminal flow
4. **Broad CRUD confidence is improving but still uneven.** Core entities are in
   good shape, and comments / ideas / decisions / agent request flows now have
   meaningfully better CLI/harness coverage, but the lower-frequency admin
   surfaces still need deliberate review if they stay in the `mvp-1` promise.

## Recommendations

### Recommended for `mvp-1`

1. Treat the quickstarts as the **user-facing smoke contract**.
2. Treat `scripts/testharness.sh` as the **CLI scripting/workflow contract**.
3. Keep layering entity-specific CLI/API/store tests underneath them for the
   broad admin surface.
4. Explicitly decide whether comments, ideas, decisions, and agents are fully in
   `mvp-1` scope or should be deferred.

### Recommended next

1. Add a second CRUD-depth pass focused only on the weaker entities.
2. Decide whether the `done` stage semantics need UX/documentation cleanup, now
   that they are at least explicitly tested.
3. Continue growing the shell harness with one scenario per high-value operator
   workflow rather than trying to turn the quickstarts into exhaustive admin
   documentation.
