# Ticket — System Specification

**Version:** 0.1.774
**Date:** 2026-04-12

This document is the authoritative specification for the `ticket` system. It is
designed so that an agent or team can rebuild the codebase, documentation, and
design from scratch using only this document and the OpenAPI specification in
[`openapi.yaml`](./openapi.yaml).

For the phase 1 entity-model pass, `docs/ENTITY_MODEL.md` is the authoritative
definition of PROJECT, SDLC, STAGE, ROLE, and TICKET where older sections of
this spec still differ.

---

## 1. Overview

`ticket` is a ticket and project management system for software engineering
work. It is delivered as a single Go binary that provides:

1. **CLI** — 60+ commands for all sdlcs
2. **HTTP API** — RESTful JSON API under `/api/`
3. **Web UI** — Embedded single-page application served from the binary
4. **Terminal UI (TUI)** — Interactive BubbleTea-based full-screen interface
5. **Agent Framework** — Autonomous worker agents with LLM integration
6. **Real-time** — WebSocket channels for live updates and chat

The system operates in two modes:

- **Local mode** — Direct SQLite access via the shared database at `$TICKET_HOME/ticket.db` (default `~/.ticket/ticket.db`), with repo-local `.ticket/config.json` used for project routing
- **Remote mode** — HTTP client connecting to the `http(s)://...` URL stored in repo-local `.ticket/config.json` or an explicit env/global override

---

## 2. Goals

1. Provide a lightweight, self-contained issue tracker that works locally or as a server.
2. Model projects, tickets, sdlcs, teams, and agents as first-class entities.
3. Give every ticket a stable, project-scoped human identifier (e.g. `CUS-T-42`).
4. Preserve the `stage + state` lifecycle model with sdlc-driven progression.
5. Make hierarchy, assignment, and claim rules explicit and deterministic.
6. Support autonomous agent workers that can claim and complete tickets.
7. Offer a CLI and API simple enough to use directly from the terminal.

## 3. Non-Goals

- Sprint planning or velocity tracking
- Story points beyond the existing priority/estimate fields
- Cross-project ticket hierarchy
- Arbitrary ticket-type plugins
- Multi-database backends (SQLite only)

---

## 4. Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.26 |
| Database | SQLite (modernc.org/sqlite, pure Go) |
| Password hashing | Argon2id (golang.org/x/crypto) |
| TUI framework | BubbleTea + Lipgloss + Bubbles |
| UUID generation | github.com/google/uuid |
| Web UI | Embedded static assets (Go embed) |
| Build | Makefile, multi-platform cross-compilation |
| Testing | Go testing, Playwright (E2E) |
| Distribution | Homebrew tap, GitHub releases, Docker |

### Dependencies (go.mod)

```
github.com/charmbracelet/bubbles v1.0.0
github.com/charmbracelet/bubbletea v1.3.10
github.com/charmbracelet/lipgloss v1.1.0
github.com/google/uuid v1.6.0
golang.org/x/crypto v0.49.0
golang.org/x/term v0.41.0
modernc.org/sqlite v1.48.0
```

---

## 5. Core Entities

### 5.1 User

Represents a human operator or service account.

| Field | Type | Constraints |
|-------|------|-------------|
| user_id | TEXT (UUID) | Primary key |
| username | TEXT | Unique, required |
| password_hash | TEXT | Argon2id hash, required |
| role | TEXT | `admin` \| `user` |
| display_name | TEXT | Required |
| enabled | INTEGER | Boolean, default 1 |
| user_type | TEXT | `user` \| `agent`, default `user` |
| email | TEXT | Default empty |
| status | TEXT | Default empty |
| description | TEXT | Default empty |
| last_seen | TEXT | Timestamp, default empty |
| uuid | TEXT | Default empty |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.2 Session

Authentication token storage.

| Field | Type | Constraints |
|-------|------|-------------|
| session_id | INTEGER | Primary key, autoincrement |
| user_id | TEXT | FK → users |
| token | TEXT | Unique |
| created_at | TEXT | Timestamp |
| expires_at | TEXT | Nullable |

### 5.3 Project

Top-level namespace and container for tickets.

| Field | Type | Constraints |
|-------|------|-------------|
| project_id | INTEGER | Primary key, autoincrement |
| prefix | TEXT | 1-5 uppercase ASCII letters, unique |
| title | TEXT | Required, unique |
| description | TEXT | Default empty |
| acceptance_criteria | TEXT | Default empty |
| git_repository | TEXT | Default empty |
| notes | TEXT | Default empty |
| status | TEXT | `open` \| `closed`, default `open` |
| visibility | TEXT | `public` \| `private`, default `public` |
| sdlc_id | INTEGER | Nullable FK → sdlcs |
| ticket_sequence | INTEGER | Auto-incrementing per project, default 0 |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

**Rules:**
- A closed project does not accept new tickets or lifecycle mutations unless reopened.
- Tickets cannot move between projects.

### 5.4 Ticket

The primary work artifact.

| Field | Type | Constraints |
|-------|------|-------------|
| ticket_id | TEXT | Primary key, human key format |
| project_id | INTEGER | FK → projects, required |
| parent_id | TEXT | Nullable FK → tickets |
| clone_of | TEXT | Nullable FK → tickets |
| type | TEXT | Required (see Ticket Types) |
| title | TEXT | Required |
| description | TEXT | Default empty |
| acceptance_criteria | TEXT | Default empty |
| git_repository | TEXT | Default empty |
| git_branch | TEXT | Default empty |
| sdlc_stage_id | INTEGER | Nullable FK → sdlc_stages |
| stage | TEXT | Default `design` |
| state | TEXT | Default `idle` |
| status | TEXT | Default `open` |
| priority | INTEGER | Default 3 |
| sort_order | INTEGER | Default 0 |
| estimate_effort | INTEGER | Default 0 |
| estimate_complete | TEXT | Default empty |
| health_score | INTEGER | Default 0 |
| assignee | TEXT | Default empty |
| draft | INTEGER | Boolean, default 0. When true, ticket is not ready for work. |
| complete | INTEGER | Boolean, default 0. When true, ticket is finished (stage=done). |
| archived | INTEGER | Boolean, default 0 |
| role_id | INTEGER | FK → roles. Current active role within the stage. |
| previous_sdlc_stage_id | INTEGER | Saved stage for reopen after completion. |
| previous_role_id | INTEGER | Saved role for reopen after completion. |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

#### 5.4.1 Ticket Key Format

```
{PROJECT_PREFIX}-{TYPE_CODE}-{SEQUENCE}
```

Examples: `CUS-E-12`, `CUS-T-143`, `OPS-B-9`

- Unique across the whole system
- Generated by the server
- Immutable after creation
- Sequence is monotonically increasing per project

#### 5.4.2 Ticket Types

| Type | Code | Can be parent? |
|------|------|----------------|
| epic | E | Yes: epic, task, bug, spike, chore |
| task | T | Yes: task, bug, spike, chore |
| bug | B | No |
| spike | S | No |
| chore | C | No |
| note | N | No |
| question | Q | No |
| requirement | R | No |
| decision | D | No |

Stories are first-class records in their own table (see section 5.18) and link
to tickets through `story_ticket_links`; they are not a valid ticket `type`.

#### 5.4.3 Ticket Hierarchy

- `parent_id` represents the parent–child relationship
- Parent and child must belong to the same project
- A ticket may have at most one parent
- Cycles are forbidden
- A parent ticket with children derives its lifecycle from descendants

### 5.5 SDLC

Defines a sequence of stages that tickets progress through.

| Field | Type | Constraints |
|-------|------|-------------|
| sdlc_id | INTEGER | Primary key, autoincrement |
| name | TEXT | Unique, required |
| description | TEXT | Default empty |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.6 SDLC Stage

An individual stage within a sdlc.

| Field | Type | Constraints |
|-------|------|-------------|
| sdlc_stage_id | INTEGER | Primary key, autoincrement |
| sdlc_id | INTEGER | FK → sdlcs |
| stage_name | TEXT | Required, unique per sdlc |
| description | TEXT | Default empty |
| role_id | INTEGER | Nullable FK → roles |
| sort_order | INTEGER | Default 0 |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.7 Team

User group with optional hierarchy.

| Field | Type | Constraints |
|-------|------|-------------|
| team_id | INTEGER | Primary key, autoincrement |
| name | TEXT | Unique, required |
| parent_team_id | INTEGER | Nullable FK → teams |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.8 Team Member

| Field | Type | Constraints |
|-------|------|-------------|
| team_id | INTEGER | FK → teams |
| user_id | TEXT | FK → users |
| role | TEXT | Required |
| job_title | TEXT | Default empty |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.9 Team Agent

| Field | Type | Constraints |
|-------|------|-------------|
| team_id | INTEGER | FK → teams |
| user_id | TEXT | FK → users (agent) |
| created_at | TEXT | Timestamp |

### 5.10 Project Member

| Field | Type | Constraints |
|-------|------|-------------|
| project_id | INTEGER | FK → projects |
| user_id | TEXT | FK → users |
| role | TEXT | `viewer` \| `editor` \| `owner` |
| created_at | TEXT | Timestamp |

### 5.11 Project Team

| Field | Type | Constraints |
|-------|------|-------------|
| project_id | INTEGER | FK → projects |
| team_id | INTEGER | FK → teams |
| role | TEXT | Required |
| created_at | TEXT | Timestamp |

### 5.12 Role

Custom role definition for sdlc stages.

| Field | Type | Constraints |
|-------|------|-------------|
| role_id | INTEGER | Primary key, autoincrement |
| sdlc_id | INTEGER | FK → sdlcs. Roles are scoped to an SDLC. |
| title | TEXT | Unique per sdlc_id, required |
| description | TEXT | Default empty |
| acceptance_criteria | TEXT | Default empty |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.13 Label

Project-scoped tag for tickets.

| Field | Type | Constraints |
|-------|------|-------------|
| label_id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| name | TEXT | Unique per project |
| color | TEXT | Default empty |
| created_at | TEXT | Timestamp |

### 5.14 Ticket Label

| Field | Type | Constraints |
|-------|------|-------------|
| ticket_id | TEXT | FK → tickets |
| label_id | INTEGER | FK → labels |
| created_at | TEXT | Timestamp |

### 5.15 Comment

Discussion thread entry on a ticket.

| Field | Type | Constraints |
|-------|------|-------------|
| id | INTEGER | Primary key, autoincrement |
| item_id | TEXT | FK → tickets |
| user_id | TEXT | FK → users |
| comment | TEXT | Required |
| created_at | TEXT | Timestamp |

### 5.16 Dependency

Ticket-to-ticket dependency link.

| Field | Type | Constraints |
|-------|------|-------------|
| id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| ticket_id | TEXT | FK → tickets |
| depends_on | TEXT | FK → tickets |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |

### 5.17 Time Entry

| Field | Type | Constraints |
|-------|------|-------------|
| time_entry_id | INTEGER | Primary key, autoincrement |
| ticket_id | TEXT | FK → tickets |
| user_id | TEXT | FK → users |
| minutes | INTEGER | Required |
| note | TEXT | Default empty |
| created_at | TEXT | Timestamp |

### 5.18 Story

User story template linked to tickets.

| Field | Type | Constraints |
|-------|------|-------------|
| story_id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| title | TEXT | Required |
| description | TEXT | Default empty |
| status | TEXT | Default `draft` |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.19 Story–Ticket Link

| Field | Type | Constraints |
|-------|------|-------------|
| story_id | INTEGER | FK → stories |
| ticket_id | TEXT | FK → tickets |
| created_at | TEXT | Timestamp |

### 5.20 History Event

Append-only audit log.

| Field | Type | Constraints |
|-------|------|-------------|
| id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| ticket_id | TEXT | FK → tickets |
| event_type | TEXT | Required |
| payload | TEXT | JSON, default `{}` |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |

Representative event types:
- `ticket_created`, `ticket_updated`, `ticket_deleted`
- `ticket_attached`, `ticket_detached`
- `ticket_assigned`, `ticket_unassigned`, `ticket_claimed`
- `ticket_stage_changed`, `ticket_state_changed`
- `ticket_parent_stage_changed`, `ticket_parent_state_changed`
- `comment_added`

### 5.21 App Settings

System-wide key/value configuration.

| Field | Type | Constraints |
|-------|------|-------------|
| key | TEXT | Primary key |
| value | TEXT | Required |

Known keys: `registration_enabled`, `chat_enabled`, `chat_max_connections`, `chat_max_duration_minutes`.

---

## 6. Lifecycle Model

### 6.1 Stages

Ordered progression:

```
design → develop → test → done
```

| Stage | Meaning |
|-------|---------|
| design | Work is being appraised, explored, or refined |
| develop | Implementation is being done |
| test | The outcome is being verified |
| done | Ticket is complete |

### 6.2 States

| State | Meaning |
|-------|---------|
| idle | Ready but not in progress |
| active | Currently being worked on (requires assignee) |
| success | Stage complete; auto-advances to next stage |
| fail | Stage did not succeed |

### 6.3 Valid Combinations

- `design`: idle, active, success, fail
- `develop`: idle, active, success, fail
- `test`: idle, active, success, fail
- `done`: success, fail only

### 6.4 Rendered Status

Status is always rendered as `{stage}/{state}` (e.g. `develop/active`, `done/success`).

### 6.5 Lifecycle Invariants

- `state=active` requires `assignee != ""`
- `state=idle` may be unassigned
- `state=success` may retain assignee
- `stage=done` requires `state=success` or `state=fail`

### 6.6 Lifecycle Mutation Rules

Only leaf tickets (no children) may be directly mutated.

Parent tickets with children return:
```
ticket has children; stage/state is derived from descendants
```

### 6.7 Ticket Creation Defaults

- `stage=design`
- `state=idle`

### 6.8 State Commands

- `idle` — keeps stage, sets `state=idle`
- `active` — keeps stage, sets `state=active`
- `complete` — sets `state=success`; auto-advances to next sdlc stage with `state=idle`
- On the final stage, `success` means the ticket is complete

### 6.9 Derived Parent Lifecycle

For tickets with children, lifecycle is derived recursively:

**Effective parent stage:** earliest stage among all descendants.

Stage ordering: `design < develop < test < done`

**Effective parent state:**
- `success` if all descendants are `success`
- `active` if any descendant is `active`
- `fail` if any descendant is `fail` (and none active)
- `idle` otherwise

---

## 7. Assignment and Claim Rules

### 7.1 Assignment

- Only tickets may be assigned
- Assignment is allowed only within the same project namespace
- `active` requires an assignee
- `unassign` is forbidden if the ticket would remain `active`

### 7.2 Claim

Claim is a server-mediated assignment using the authenticated user.

**Eligibility:**
- Only leaf tickets are claimable
- Only tickets in `develop/idle` are claimable
- Only tickets in open projects are claimable
- Archived tickets are not claimable

**Behavior:**
- `claim -id <KEY>` — if eligible, set `assignee=current_user` and `state=active`
- `claim` (no ID) — return current active ticket or assign next eligible
- `claim -dry-run` — return candidate without changing state

**Selection order** (when no ID specified):
1. Highest priority first
2. Oldest created ticket first
3. Key lexical order as tie-break

---

## 8. Authentication and Authorization

### 8.1 Authentication

- **Registration:** username + password → Argon2id hash → stored in DB
- **Login:** verify hash → create session → return token
- **Session validation:** Bearer token in `Authorization` header, or session cookie (`__Host-session` on secure requests; legacy `ticket_token` supported)
- **Agent auth:** HTTP Basic Auth with agent UUID + password

### 8.2 Authorization

**System roles:**
- `admin` — full access to all resources
- `user` — standard user, access governed by project membership

**Project roles:**
- `viewer` — read-only
- `editor` — read/write tickets
- `owner` — full project management

### 8.3 Rate Limiting

Auth endpoints (`/api/register`, `/api/login`) are rate-limited to 10 requests per minute per client IP.

---

## 9. Agent Framework

Agents are autonomous worker accounts that poll for and complete tickets.

### 9.1 Agent Lifecycle

1. Admin creates agent (`POST /api/agents`) — returns UUID + password
2. Agent registers (`POST /api/agents/register`) with Basic Auth
3. Agent polls for work (`POST /api/agents/request`)
4. Agent sends heartbeats (`POST /api/agents/heartbeat`)
5. Agent completes work (`POST /api/agents/{id}/tickets/{ticket_id}/update`)

### 9.2 Agent States

- `idle` — no work assigned
- `working` — actively processing a ticket
- `disabled` — administratively disabled

### 9.3 Heartbeat Reaper

A background thread marks agents as offline if no heartbeat received within 10 minutes.

### 9.4 LLM Integration

Agents can use an LLM backend for ticket analysis and completion:
- Default: Claude (Sonnet 4.5)
- Alternatives: Codex, or a custom binary path
- Configured via `TICKET_AGENT_LLM` or `-llm` flag

---

## 10. Real-time

### 10.1 Live Updates (`/api/ws`)

WebSocket endpoint that broadcasts ticket and project change events to connected clients.

### 10.2 Chat (`/api/chat/ws`)

WebSocket endpoint for streaming LLM chat sessions. Configurable via:
- `chat_enabled` — feature flag
- `chat_max_connections` — concurrent session limit (default 10)
- `chat_max_duration_minutes` — session timeout (default 30)

---

## 11. Configuration

### 11.1 Environment Variables

| Variable | Purpose |
|----------|---------|
| `TICKET_URL` | Effective location override: bare paths and `file:///...` are local, `http(s)://...` is remote |
| `TICKET_USERNAME` | Default username |
| `TICKET_PASSWORD` | Default password |
| `TICKET_TRUSTED_PROXY_CIDRS` | Comma-separated CIDRs trusted for forwarded proxy headers |
| `AGENT_ID` | Agent UUID for worker mode |
| `AGENT_PASSWORD` | Agent password |
| `TICKET_AGENT_LLM` | LLM command override |

### 11.2 Config Resolution

1. Resolve `$TICKET_HOME` from the environment or default it to `~/.ticket`
2. Walk up from the current directory looking for the nearest `.ticket/config.json`
3. Load repo-local routing from that file when present
4. Overlay global defaults from `$TICKET_HOME/config.json`
5. If `TICKET_URL` is set, use it instead of the stored `location`
6. If the effective location is `http://...` or `https://...` -> **remote mode**
7. If the effective location is `file://...` or a bare path -> **local mode** using that path
8. If the effective location is empty -> **local mode** at `$TICKET_HOME/ticket.db`

Bare local paths are resolved like this:
1. Absolute paths are used as-is
2. Relative paths are resolved under `$TICKET_HOME`

### 11.3 Config Files

- `.ticket/config.json` — repo-local routing (`location`, bound project, local project state)
- `$TICKET_HOME/config.json` — global defaults (`username`, fallback location, current project override, TUI state)
- `$TICKET_HOME/credentials.json` — remote auth tokens
- `$TICKET_HOME/ticket.db` — default SQLite database (local mode)

---

## 12. CLI Commands

The binary is named `ticket` with the alias `tk`.

### 12.1 System

| Command | Description |
|---------|-------------|
| `tk initdb` | Create or repair the shared local database and bootstrap the default admin/project |
| `tk init` | Bind the current repo or directory to a local or remote project |
| `tk server` | Start HTTP server and web UI on :8080 |
| `tk version` | Show version |
| `tk upgrade` | Check for newer version from GitHub |
| `tk status` | Show connection status (mode, database, config) |
| `tk summary` | Show project summary, active tickets, recent activity |
| `tk export -o file.json` | Export all data to JSON snapshot |
| `tk import -i file.json` | Restore from snapshot |
| `tk onboard` | Print agent onboarding instructions |
| `tk gui` / `-g` | Launch interactive TUI |
| `tk doctor` | Run diagnostic checks |

### 12.2 Authentication

| Command | Description |
|---------|-------------|
| `tk register -username NAME -password PASS` | Register new account |
| `tk login -username NAME -password PASS` | Authenticate |
| `tk logout` | Clear session |
| `tk whoami` | Show current user |
| `tk config` | Manage client config |

### 12.3 Projects

| Command | Description |
|---------|-------------|
| `tk project list` | List projects |
| `tk project create -prefix CUS -title "..."` | Create project |
| `tk project use <id>` | Set active project |
| `tk project get <id>` | View project details |
| `tk project update <id> -title "..."` | Update project |
| `tk project delete <id>` | Delete project |
| `tk project init` | Write `.ticket.json` in current directory |
| `tk project sdlc <sdlc-id>` | Assign an SDLC to the active project |
| `tk project set-draft <true\|false>` | Toggle draft mode on the active project |

### 12.4 Tickets

| Command | Description |
|---------|-------------|
| `tk list` / `tk ls` | List open tickets (filters: `-type`, `-status`, `-user`, `-label`) |
| `tk search "query"` | Full-text search |
| `tk get -id <id>` | View ticket details |
| `tk add "Title"` / `tk new "Title"` | Create task |
| `tk bug "Title"` | Create bug |
| `tk epic "Title"` | Create epic |
| `tk note "Text"` | Create note |
| `tk question "Text"` | Create question |
| `tk update -id <id> -title "..." -d "..."` | Update ticket |
| `tk delete -id <id>` | Delete ticket |
| `tk clone <id>` | Duplicate ticket and children |

### 12.5 Lifecycle

| Command | Description |
|---------|-------------|
| `tk state -id <id> <state>` | Set state directly |
| `tk idle -id <id>` | Set state to idle |
| `tk active -id <id>` | Set state to active |
| `tk complete -id <id>` | Set state to success (auto-advance) |
| `tk stage -id <id>` | View current stage |

### 12.6 Assignment

| Command | Description |
|---------|-------------|
| `tk claim -id <id>` | Self-assign and activate |
| `tk unclaim <id>` | Remove self from assignment |
| `tk assign <id> <username>` | Assign to user |
| `tk unassign <id> <username>` | Remove assignment |
| `tk request` | Request next available ticket |

### 12.7 Hierarchy

| Command | Description |
|---------|-------------|
| `tk set-parent -id <child> <parent>` | Create parent/child link |
| `tk unset-parent -id <child>` | Remove parent link |
| `tk orphans` | List tickets with no parent |

### 12.8 Comments and History

| Command | Description |
|---------|-------------|
| `tk comment add -id <id> "Text"` | Add comment |
| `tk history <id>` | View audit trail |
| `tk conversation show <id>` | Show comments + history |

### 12.9 Labels

| Command | Description |
|---------|-------------|
| `tk label create -name "bug" -color "red"` | Create label |
| `tk label ls` | List project labels |
| `tk label add <ticket-id> <label-id>` | Tag ticket |
| `tk label remove <ticket-id> <label-id>` | Remove tag |
| `tk label show <ticket-id>` | Show ticket labels |
| `tk label delete <label-id>` | Delete label |

### 12.10 Time Tracking

| Command | Description |
|---------|-------------|
| `tk time log -id <id> -m 30 -note "..."` | Log time |
| `tk time list <id>` | List entries |
| `tk time total <id>` | Sum time |
| `tk time delete <entry-id>` | Remove entry |

### 12.11 Dependencies

| Command | Description |
|---------|-------------|
| `tk dependency add -id <id> <depends-on>` | Add dependency |
| `tk dependency remove -id <id> <depends-on>` | Remove dependency |
| `tk dependency list <id>` | Show dependencies |

### 12.12 SDLCs

| Command | Description |
|---------|-------------|
| `tk sdlc list` | List sdlcs |
| `tk sdlc get -id <id>` | View sdlc stages |
| `tk sdlc create -name "..." -description "..."` | Create sdlc |
| `tk sdlc delete -id <id>` | Delete sdlc |
| `tk sdlc add-stage <sdlc-id> <stage-name>` | Add stage |

### 12.13 Requirements and Decisions

| Command | Description |
|---------|-------------|
| `tk idea new "Title" -d "..."` | Create requirement |
| `tk idea ls` | List requirements |
| `tk review` | Review requirements by status |
| `tk accept <id>` | Accept requirement |
| `tk reject <id>` | Reject requirement |
| `tk revise <id>` | Mark revised |
| `tk curate <id> [<id>...]` | Convert tickets to requirement |
| `tk decision add "..."` | Record decision |
| `tk decision list` | List decisions |

### 12.14 Roles

| Command | Description |
|---------|-------------|
| `tk role list` / `tk role ls` | List all roles |
| `tk role create -title "..." -description "..." -ac "..."` | Create role |
| `tk role update -id <id> -title "..."` | Update role |
| `tk role delete -id <id>` / `tk role rm -id <id>` | Delete role |
| `tk sdlc stage-role-add -sdlc_id <id> -stage_id <id> -role_id <id>` | Assign role to stage |
| `tk sdlc stage-role-rm -sdlc_id <id> -stage_id <id> -role_id <id>` | Remove role from stage |
| `tk sdlc stage-role-order -sdlc_id <id> -stage_id <id> -roles <ids>` | Reorder roles in stage |

### 12.15 Teams

| Command | Description |
|---------|-------------|
| `tk team list` | List teams |
| `tk team create -name "..."` | Create team |
| `tk team get -id <id>` | View team |
| `tk team update -id <id> -name "..."` | Update team |
| `tk team delete -id <id>` | Delete team |
| `tk team add-member <team-id> <user-id>` | Add member |
| `tk team remove-member <team-id> <user-id>` | Remove member |

### 12.16 Users (Admin)

| Command | Description |
|---------|-------------|
| `tk user list` | List users |
| `tk user create -username <name> -password <pass>` | Create user |
| `tk user enable <username>` | Enable user |
| `tk user disable <username>` | Disable user |
| `tk user reset-password <username>` | Change password |
| `tk user delete <username>` | Delete user |

### 12.17 Agents

| Command | Description |
|---------|-------------|
| `tk agent create` | Register new agent (returns UUID + password) |
| `tk agent list` | List agents |
| `tk agent enable <id>` | Enable agent |
| `tk agent disable <id>` | Disable agent |
| `tk agent delete <id>` | Delete agent |
| `tk agent run -id <uuid> -url <server-url>` | Start agent worker loop |

### 12.18 Board and Counts

| Command | Description |
|---------|-------------|
| `tk board` | Kanban view by stage |
| `tk count` | Aggregate counts by type/status |
| `tk health -id <id> <score>` | Set health score |

### 12.19 Other

| Command | Description |
|---------|-------------|
| `tk complete -id <id>` | Mark ticket as complete (stage=done) |
| `tk reopen -id <id>` | Undo completion |
| `tk close -id <id>` | Alias for `tk complete` |
| `tk archive -id <id>` | Archive ticket |
| `tk unarchive -id <id>` | Unarchive ticket |
| `tk draft -id <id>` | Mark as draft (not ready for work) |
| `tk undraft -id <id>` | Mark as not draft (ready for work) |
| `tk next -id <id>` | Advance to next role or stage |
| `tk previous -id <id>` | Regress to previous role or stage |
| `tk success -id <id>` | Shortcut: state → success |
| `tk help <command>` | Show command help |

---

## 13. HTTP API

All endpoints are under `/api/`. Request and response bodies are JSON.
Error responses use `{"error": "message"}` with appropriate HTTP status codes.

See [`openapi.yaml`](./openapi.yaml) for the full OpenAPI 3.1 specification.

### 13.1 Middleware

1. **Security headers** — `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`, `Strict-Transport-Security` (HTTPS)
2. **Logging** — request method, path, status, size, duration (verbose mode)
3. **Authentication** — Bearer token or session cookie (`__Host-session` on secure requests; legacy `ticket_token` supported)
4. **Rate limiting** — 10 req/min per IP on auth endpoints

### 13.2 Endpoint Summary

#### Health and Status
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/healthz` | None | Health check |
| GET | `/api/status` | None | Server status, auth info, feature flags |

#### Authentication
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/register` | None | Register user |
| POST | `/api/login` | None | Login, returns token |
| POST | `/api/logout` | Bearer | Logout, clear session |

#### Configuration (Admin)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/config/registration` | Admin | Enable/disable registration |
| POST | `/api/config/chat_enabled` | Admin | Enable/disable chat |
| POST | `/api/config/chat_limits` | Admin | Set chat connection/duration limits |

#### Users (Admin)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/users` | Admin | List users |
| POST | `/api/users` | Admin | Create user |
| POST | `/api/users/{username}/enable` | Admin | Enable user |
| POST | `/api/users/{username}/disable` | Admin | Disable user |
| DELETE | `/api/users/{username}` | Admin | Delete user |

#### Agents
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/agents` | Admin | List agents |
| POST | `/api/agents` | Admin | Create agent |
| GET | `/api/agents/statuses` | Admin | Get agent status enums |
| POST | `/api/agents/register` | Basic | Agent registration |
| POST | `/api/agents/heartbeat` | Basic | Agent heartbeat |
| POST | `/api/agents/request` | Basic | Request work assignment |
| POST | `/api/agents/{id}/tickets/{ticket_id}/update` | Basic | Complete ticket |
| PUT | `/api/agents/{id}` | Admin | Update agent |
| DELETE | `/api/agents/{id}` | Admin | Delete agent |
| POST | `/api/agents/{id}/enable` | Admin | Enable agent |
| POST | `/api/agents/{id}/disable` | Admin | Disable agent |

#### Roles (Admin)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/roles` | Admin | List roles |
| POST | `/api/roles` | Admin | Create role |
| PUT | `/api/roles/{id}` | Admin | Update role |
| DELETE | `/api/roles/{id}` | Admin | Delete role |

#### SDLCs
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/sdlcs` | User | List sdlcs |
| POST | `/api/sdlcs` | Admin | Create sdlc |
| POST | `/api/sdlcs/import` | Admin | Import sdlc |
| GET | `/api/sdlcs/{id}` | User | Get sdlc with stages |
| DELETE | `/api/sdlcs/{id}` | User | Delete sdlc |
| POST | `/api/sdlcs/{id}/stages` | User | Add stage |
| PUT | `/api/sdlcs/{id}/reorder` | User | Reorder stages |
| GET | `/api/sdlcs/{id}/export` | User | Export sdlc |
| DELETE | `/api/sdlcs/stages/{id}` | Admin | Delete stage |

#### Teams
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/teams` | User | List teams |
| POST | `/api/teams` | Admin | Create team |
| GET | `/api/teams/{id}` | User | Get team |
| PUT | `/api/teams/{id}` | Admin/Owner | Update team |
| DELETE | `/api/teams/{id}` | Admin/Owner | Delete team |
| GET | `/api/teams/{id}/users` | Member | List members |
| POST | `/api/teams/{id}/users` | Admin/Owner | Add member |
| DELETE | `/api/teams/{id}/users/{user_id}` | Admin/Owner | Remove member |
| GET | `/api/teams/{id}/agents` | Member | List agents |
| POST | `/api/teams/{id}/agents` | Admin/Owner | Add agent |
| DELETE | `/api/teams/{id}/agents/{agent_id}` | Admin/Owner | Remove agent |

#### Projects
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/projects` | User | List projects |
| POST | `/api/projects` | User | Create project |
| GET | `/api/projects/{id}` | Read | Get project |
| PUT | `/api/projects/{id}` | Editor | Update project |
| POST | `/api/projects/{id}/enable` | Owner | Enable project |
| POST | `/api/projects/{id}/disable` | Owner | Disable project |
| GET | `/api/projects/{id}/tickets` | Read | List tickets (with filters) |
| GET | `/api/projects/{id}/history` | Read | List history events |
| GET | `/api/projects/{id}/stories` | Read | List stories |
| GET | `/api/projects/{id}/users` | Owner | List project members |
| POST | `/api/projects/{id}/users` | Owner | Add member |
| DELETE | `/api/projects/{id}/users/{user_id}` | Owner | Remove member |
| GET | `/api/projects/{id}/teams` | Owner | List project teams |
| POST | `/api/projects/{id}/teams` | Owner | Add team |
| DELETE | `/api/projects/{id}/teams/{team_id}` | Owner | Remove team |
| GET | `/api/projects/{id}/labels` | User | List labels |
| POST | `/api/projects/{id}/labels` | User | Create label |
| DELETE | `/api/projects/{id}/labels/{label_id}` | User | Delete label |

#### Tickets
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/tickets` | Write | Create ticket |
| POST | `/api/tickets/claim` | User | Claim/request ticket |
| GET | `/api/tickets/{ref}` | Read | Get ticket |
| PUT | `/api/tickets/{ref}` | Write | Update ticket |
| DELETE | `/api/tickets/{ref}` | Write | Delete ticket |
| GET | `/api/tickets/{ref}/history` | Read | Get history |
| POST | `/api/tickets/{ref}/health` | Write | Set health score |
| GET | `/api/tickets/{ref}/labels` | Read | List labels |
| POST | `/api/tickets/{ref}/labels` | Write | Add label |
| DELETE | `/api/tickets/{ref}/labels/{label_id}` | Write | Remove label |
| GET | `/api/tickets/{ref}/time` | Read | List time entries |
| GET | `/api/tickets/{ref}/time/total` | Read | Get total time |
| POST | `/api/tickets/{ref}/time` | Write | Log time |
| GET | `/api/tickets/{ref}/comments` | Read | List comments |
| POST | `/api/tickets/{ref}/comments` | Write | Add comment |
| GET | `/api/tickets/{ref}/dependencies` | Read | List dependencies |
| POST | `/api/tickets/{ref}/clone` | Write | Clone ticket |
| POST | `/api/tickets/{ref}/close` | Write | Close ticket |
| POST | `/api/tickets/{ref}/open` | Write | Reopen ticket |
| POST | `/api/tickets/{ref}/archive` | Write | Archive ticket |
| POST | `/api/tickets/{ref}/unarchive` | Write | Unarchive ticket |
| POST | `/api/tickets/{ref}/ready` | Write | Mark ready |
| POST | `/api/tickets/{ref}/notready` | Write | Mark not ready |
| POST | `/api/tickets/{ref}/sdlc` | Write | Set sdlc |
| DELETE | `/api/tickets/{ref}/sdlc` | Write | Remove sdlc |
| POST | `/api/tickets/{ref}/analyse` | Write | Analyse epic (LLM) |

#### Stories
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/stories` | Write | Create story |
| GET | `/api/stories/{id}` | Write | Get story |
| PUT | `/api/stories/{id}` | Write | Update story |
| DELETE | `/api/stories/{id}` | Write | Delete story |
| POST | `/api/stories/{id}/analyse` | Write | Analyse story (LLM) |

#### Dependencies
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/dependencies` | Write | Create dependency |
| DELETE | `/api/dependencies` | Write | Delete dependency |

#### Utility
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/count` | User | Count summary |
| DELETE | `/api/labels/{id}` | User | Delete label by ID |
| DELETE | `/api/time/{id}` | User | Delete time entry |

#### WebSocket
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/ws` | Bearer/Cookie | Live update events |
| GET | `/api/chat/ws` | Bearer/Cookie | Chat/AI streaming |

---

## 14. Database

### 14.1 Engine

SQLite via `modernc.org/sqlite` (pure Go, no CGO).

### 14.2 Configuration

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
```

Connection pool: `MaxOpenConns=1`, `MaxIdleConns=1`.

### 14.3 Schema

Full schema is defined in Section 5 (Core Entities). The database uses
`CREATE TABLE IF NOT EXISTS` for idempotent setup and a migration layer for
column additions and table renames.

### 14.4 Indexes

Indexes on: `sessions(user_id, token)`, `tickets(project_id, parent_id, assignee, stage, state)`, `stories(project_id)`, `story_ticket_links(ticket_id)`, `history_events(project_id, ticket_id)`, `ticket_history(project_id, ticket_id)`, `comments(item_id, user_id)`, `dependencies(project_id, ticket_id, depends_on)`, `labels(project_id)`, `ticket_labels(label_id)`, `time_entries(ticket_id, user_id)`, `sdlc_stages(sdlc_id, role_id)`.

### 14.5 Initialization

`tk initdb` creates the database, schema, admin user, default sdlc (`design → develop → test → done`), and a default project with prefix `TK`. `tk init` writes `.ticket/config.json` to bind the current repo or directory to a project.

---

## 15. Web UI

The web UI is a single-page application with pre-compiled static assets embedded into the Go binary using `//go:embed`. It is served from the same port as the API (`:8080`). Unknown routes are rewritten to `index.html` (SPA fallback).

---

## 16. Terminal UI (TUI)

Built with BubbleTea (Elm-inspired architecture).

**Modes:**
- Summary — project overview, active tickets, recent activity
- List — filtered ticket list with cursor navigation
- Ideas — requirements/ideas view
- Projects — project switcher
- Settings — theme selection

**Features:**
- Full-screen interactive navigation
- Mouse and keyboard support (j/k, Enter, etc.)
- Multiple themes (The Grey, Dracula, Monochrome, etc.)
- State persistence (cursor, expanded items, theme)

---

## 17. Deployment

### 17.1 Docker

Multi-stage build:
1. **Builder:** `golang:1.26-alpine`, compile to `/out/tk`
2. **Runtime:** `alpine:3.21`, non-root user `ticket`, expose 8080

```dockerfile
CMD ["tk", "server"]
```

### 17.2 Installation

```bash
# Homebrew
brew install simonski/tap/ticket

# From source
go install github.com/simonski/ticket/cmd/tk@latest

# Docker
docker build -t ticket .
docker run -p 8080:8080 tk server
```

### 17.3 Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary, increment patch version |
| `make test` | Run all tests (unit + integration + Playwright) |
| `make test-unit` | Unit tests only |
| `make test-integration` | Integration tests only |
| `make test-playwright` | Playwright E2E tests |
| `make release` | Cross-compile for all platforms |
| `make release-publish` | Upload to GitHub releases |
| `make install` | Build and `go install` |

**Release platforms:** darwin/arm64, darwin/amd64, linux/amd64, linux/arm64

---

## 18. Project Structure

```
ticket/
├── cmd/tk/              # CLI entry point and command handlers
│   ├── main.go              # Root command router
│   ├── cmd_*.go             # Individual commands
│   ├── help.go              # Command help
│   ├── printer.go           # Output formatting
│   ├── prompt.go            # Interactive prompts
│   └── VERSION              # Semantic version
├── internal/
│   ├── client/              # HTTP client for remote mode
│   ├── config/              # Configuration resolution
│   ├── password/            # Argon2id password hashing
│   ├── server/              # HTTP server, API handlers, WebSocket
│   ├── store/               # SQLite database layer
│   └── tui/                 # Terminal UI (BubbleTea)
├── libticket/               # Public service interface and types
├── libtickethttp/           # HTTP adapter for libticket
├── libtickettest/           # Test helpers
├── web/                     # Embedded web UI static assets
├── tools/                   # Utility scripts
├── tests/                   # Playwright E2E tests
├── docs/                    # Architecture and design docs
├── Makefile                 # Build automation
├── Dockerfile               # Container build
├── go.mod / go.sum          # Go dependencies
└── openapi.yaml             # API specification
```

---

## 19. Test Requirements

Tests must cover:

- Project prefix validation and uniqueness
- Human ticket key generation and per-project sequence
- Hierarchy validation including cycle rejection
- Same-project parent/child enforcement
- Lifecycle mutation rules for leaf tickets
- Derived lifecycle recalculation for parent tickets
- Assignment and unassignment validation
- Claim eligibility and selection ordering
- History emission for direct and derived changes
- CLI parsing for project and ticket command separation
- API serialization of id, key, stage, state, and status
- Authentication and session management
- Rate limiting on auth endpoints
- WebSocket live update delivery
- Agent registration, heartbeat, and work assignment
- Export/import round-trip fidelity
