# Changelog

All notable changes to the Ticket project are documented here.

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
- **CLI tests** — boosted cmd/ticket coverage above 55% threshold

### Changed
- CLI usage output reorganised into admin and client command groups
- `tk status` output simplified: removed mode line, uses TICKET_URL as key
- `tk get` shows workflow progress for the ticket's current stage
- Ticket list shows progress column

### Fixed
- DESIGN.md and PROMPTS.md updated to reference `TICKETS.md` (was `AGENTS.md`)
- Gitignore no longer excludes `cmd/ticket/` directory
- Stale FK migration after tasks→tickets table rename
- HTTP API: standalone DELETE handlers for `/api/labels/<id>` and `/api/time/<id>`
