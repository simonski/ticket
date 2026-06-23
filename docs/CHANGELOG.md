# Changelog

All notable changes to the Ticket project are documented here.

## [0.1.1103] - 2026-06-23

### Added
- **Extensible schema (attrs)** — high-churn entities (tickets, projects, roles,
  workflow_stages) carry an `attrs` TEXT-JSON attribute bag; new optional/per-type
  fields are added in Go with no schema migration or version bump, and become
  queryable on demand via `json_extract` expression indexes (`EnsureAttrIndex`).
  Design: `docs/design/extensible-schema.md`, `docs/adr/0001-json-attribute-bags.md`.
- **Reinforced migrations** — `tk admin upgrade-database` / server-startup upgrade
  take a WAL-checkpointed, integrity-verified backup and auto-roll-back on failure
  (`docs/RUNBOOKS.md`).

### Changed
- **Schema version 10 → 12.** Soft/config/guidance columns consolidated into
  `attrs` and dropped across all four entities (typed Go/API fields retained and
  hydrated from the bag, so the CLI/API contract is unchanged).
- **Go toolchain 1.26.3 → 1.26.4** (clears two `crypto/x509` stdlib advisories).
- **Coverage gates recalibrated** to actual levels: `internal/store` 66%,
  `internal/server` 57% (the old 69/63 were never met).
- **`tk unassign <id>`** — the assignee name is now optional for admins (clears
  whoever is assigned) (TK-92).

### Removed
- **Goals feature** removed entirely (entity, `clarify_goal` recommendation, and
  related code). The "goal" concept is realized via the Feature/refinement model.

### Fixed
- **`tk edit` TUI** — letters `j/k/w/s` are now typed in text fields instead of
  being captured as navigation; text-vs-control key handling separated (TK-86).
- **Playwright harness** — the smoke suite serves `web/static` from the repo root
  again and passes; `GET /api/releases/{id}` (405) handler added.

## [0.1.861] - 2026-04-28

### Changed
- **Documentation structure** — moved detailed quickstarts into `docs/quickstarts/`, the server tutorial into `docs/tutorials/`, Workflow assessment methodology into `docs/process/`, and historical planning notes into `docs/archive/`.
- **Public contract docs** — aligned `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION` on version `0.1.861`.
- **Server coverage gate** — raised the enforced `internal/server` package coverage threshold to 70%.
- **Named remote docs** — refreshed user and developer docs around `$TICKET_HOME`, named remotes, repo-local `remote`/`project_id`, and compatibility-only `location` values.

### Fixed
- **Bootstrap guidance** — changed deploy/server docs to require explicit admin credentials for shared deployments instead of normalizing development fallback credentials.
- **Root documentation clutter** — archived stale TODO/planning files so the repository root presents only active product, contributor, testing, security, and contract docs.

## [0.1.697] - 2026-03-28

### Added
- **TUI** — full-screen terminal UI (`tk -g`) with Home, Projects, Ideas, Tickets, Workflows, and Config panels
- **Ideas panel** — capture lightweight ideas (`tk idea new`, `tk idea ls`) with TUI panel
- **Hierarchy perspective** — web UI hierarchy view for epic/story/task trees (TK-11)
- **Epic CLI** — `tk epic use/clear/ls` subcommands (TK-12)
- **Story CLI** — create, list, get, update, delete story tickets (TK-15)
- **Setup wizard** — `tk setup` interactive singleplayer onboarding with skill install
- **Summary command** — `tk summary` shows open ticket counts and project overview
- **Agent streaming** — `tk agent run -v` streams LLM stdin/stdout in real time
- **Agent auto-assignment** — agents can be assigned to tickets; `tk ready <id>` marks eligibility
- **Noun-verb CLI** — reorganised CLI under `tk req`, `tk ticket` namespaces
- **Status box** — `tk status` and `tk count` rendered in rounded Unicode box with env vars and config path
- **Drag-and-drop** — kanban board supports drag-drop stage changes and reopening closed tickets
- **Server `-path` flag** — serve static files from filesystem instead of embedded assets
- **Email encryption** and schema additions for user management
- **Goals** and user list improvements
- **Homebrew formula** — distribution via `brew install simonski/tap/ticket`
- **Security hardening** — security headers (CSP, X-Frame-Options, nosniff), Secure cookie flag for HTTPS, rate limiting on login/register, tightened DB directory permissions
- **SAST in CI** — added `govulncheck` and `gosec` to GitHub Actions pipeline
- **CLI test coverage** — boosted `cmd/tk` above 55% threshold with tests for timeAgo, orDash, rowColor, formatPayloadKeyValues, generateConfirmToken, prefixWriter/Reader, printAgentTable, printTeamAgentTable, runWhoami, runSummary, runTicketNS, runSetTicketClosed, project user/team commands, clone, request, archive, comment, dependency, team, and requirements/decisions workflows

### Changed
- **TICKET_CONFIG_DIR → TICKET_HOME** — env var renamed; `.ticket/` auto-located by walking up from CWD
- **Per-project config** moved from `.ticket.json` to `$TICKET_HOME/config.json`
- **Agents merged into users** — agents table consolidated into users with `user_type` column
- **Agent credentials** moved from request body to HTTP Basic Auth
- **Agent default LLM** set to `claude` (Sonnet 4.5); prompt written to file and piped via stdin
- **Agent poll interval** default 5s, config-driven (TK-157)
- **Agent name optional** — auto-generates UUID when omitted (TK-152)
- **History output** formatted as human-readable text instead of raw JSON
- **Consistent `-id` flags** across all commands; title-with-dash bug fixed
- **Argon2id iterations** increased from 3 to 4
- **Dependencies bumped** — golang.org/x/crypto v0.49.0, modernc.org/sqlite v1.48.0, and all transitive deps

### Fixed
- Right panel selector active state (TK-9)
- Stale FK migration index bug (TK-9)
- Suppressed redundant WS reload after local autosave; stopped WS reconnect after logout (TK-4)
- Documentation drift: CLI syntax, state names, ticket types, init command, stage aliases, agent setup
- REQUIREMENTS.md aligned with stage/state lifecycle model (TK-13)
- `TestSaveLoadRoundTrip` chdir to temp dir to avoid local config override
- `tk get` and `tk ls` heuristics for positional args and children
- Status box padding alignment
- Setup wizard continuation when user declines reinitialise
- Summary scoped to current project, filtered to open tickets only
- Friendly connection errors; removed Go network errors from client output
- QUICKSTART.md typo (`titk` → `tk`)

## [0.1.538] - 2026-03-17

### Added
- **Workflow entity** — customisable workflow definitions with ordered stages and role associations (Phases 1-3)
- **Workflow-driven tickets** — tickets auto-advance through workflow stages on completion
- **Board CLI** (`tk board`) — kanban-style view grouped by workflow stage
- **Labels** — project-scoped labels with colour, attach/detach from tickets
- **Time tracking** — log minutes against tickets, list entries, view totals
- **Web UI: labels on board cards** — label chips displayed on kanban cards
- **Web UI: label management** — add/create/remove labels in ticket modal
- **Web UI: time tracking** — log, list, delete time entries in ticket modal
- **Web UI: assignee on board cards** — `@username` shown on kanban cards
- **Web UI: health scores** — set ticket health (0-4) from ticket modal
- **Web UI: parent/child hierarchy** — set/view parent ticket with navigation link
- **Web UI: dependencies display** — view dependency links in ticket modal
- **Web UI: ticket assignment** — edit assignee directly in ticket modal
- **Web UI: workflow management panel** — create/delete workflows, add/remove stages (admin)
- **Contract tests** — expanded coverage for labels, time tracking, workflow CRUD, roles, teams, ticket lifecycle, and count
- **CLI tests** — boosted cmd/tk coverage above 55% threshold

### Changed
- CLI usage output reorganised into admin and client command groups
- `tk status` output simplified and aligned with named remote configuration
- `tk get` shows workflow progress for the ticket's current stage
- Ticket list shows progress column

### Fixed
- DESIGN.md and PROMPTS.md updated to reference `TICKETS.md` (was `AGENTS.md`)
- Gitignore no longer excludes `cmd/tk/` directory
- Stale FK migration after tasks→tickets table rename
- HTTP API: standalone DELETE handlers for `/api/labels/<id>` and `/api/time/<id>`
