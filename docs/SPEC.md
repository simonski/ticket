# Ticket — System Specification

**Version:** 0.1.1087
**Date:** 2026-06-10

This document is the authoritative specification for the `ticket` system. It is
designed so that an agent or team can rebuild the codebase, documentation, and
design from scratch using only this document and the OpenAPI specification in
[`openapi.yaml`](./api/openapi.yaml).

For the phase 1 entity-model pass, `docs/ENTITY_MODEL.md` is the authoritative
definition of PROJECT, Workflow, STAGE, ROLE, and TICKET where older sections of
this spec still differ.

---

## 1. Overview

`ticket` is a ticket and project management system for software engineering
work. It is delivered as a single Go binary that provides:

1. **CLI** — 60+ commands for all workflows
2. **HTTP API** — RESTful JSON API under `/api/`
3. **Web UI** — Embedded single-page application served from the binary
4. **Terminal UI (TUI)** — Interactive BubbleTea-based full-screen interface
5. **Agent Framework** — Autonomous worker agents with LLM integration
6. **Real-time** — WebSocket channels for live updates and chat

The system operates as a client/server architecture:

- **Server** — owns the SQLite database and exposes HTTP API + web UI
- **Client** — CLI/TUI connect to the URL from `TICKET_URL`; repo-local `.ticket/config.json` stores the selected `project_id`, and `$TICKET_HOME/credentials.json` stores reusable session credentials

---

## 2. Goals

1. Provide a lightweight, self-contained issue tracker that runs as a server with first-class CLI and web clients.
2. Model projects, tickets, workflows, teams, and agents as first-class entities.
3. Give every ticket a stable, project-scoped human identifier (e.g. `CUS-42`).
4. Preserve the `stage + state` lifecycle model with workflow-driven progression.
5. Make hierarchy, assignment, and claim rules explicit and deterministic.
6. Support autonomous agent workers that can claim and complete tickets.
7. Offer a CLI and API simple enough to use directly from the terminal.

## 3. Non-Goals

- Velocity tracking or burndown charts
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
| workflow_id | INTEGER | Nullable FK → workflows |
| ticket_sequence | INTEGER | Auto-incrementing per project, default 0 |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

**Rules:**
- A closed project does not accept new tickets or lifecycle mutations unless reopened.
- Tickets cannot move between projects.

### 5.3a Release

Top-level delivery container for a project. Holds the **features** (and their
epic/story subtrees) designed, sealed, then executed together. See
[`RELEASES.md`](./RELEASES.md) for the full model.

| Field | Type | Constraints |
|-------|------|-------------|
| id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| title | TEXT | Required |
| purpose | TEXT | Default empty |
| target_date | TEXT | Aspirational target delivery date |
| status | TEXT | `in_design` \| `in_progress` \| `complete`, default `in_design` |
| designed_at | TEXT | Timestamp, nullable |
| started_at | TEXT | Timestamp, nullable |
| completed_at | TEXT | Timestamp, nullable |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |
| feature_count | INTEGER | Derived count of features |
| story_count | INTEGER | Derived count of stories across features |

**Rules:**
- A release moves `in_design` → `in_progress` → `complete`.
- Features may be added to / removed from a release only while it is `in_design`.
- `in_progress` ("sealed") freezes the feature set; the orchestrator then executes the release's ready stories.
- Tickets carry `release_id`; adding a feature propagates `release_id` across its whole epic/story subtree.

### 5.4 Ticket

The primary work artifact.

| Field | Type | Constraints |
|-------|------|-------------|
| ticket_id | TEXT | Primary key, human key format |
| project_id | INTEGER | FK → projects, required |
| parent_id | TEXT | Nullable FK → tickets |
| clone_of | TEXT | Nullable FK → tickets |
| release_id | INTEGER | Nullable FK → releases. Propagated across a feature's subtree. |
| type | TEXT | Required (see Ticket Types) |
| title | TEXT | Required |
| description | TEXT | Default empty |
| acceptance_criteria | TEXT | Default empty |
| git_repository | TEXT | Default empty |
| git_branch | TEXT | Default empty |
| workflow_stage_id | INTEGER | Nullable FK → workflow_stages |
| stage | TEXT | Default `design` |
| state | TEXT | Default `idle` |
| status | TEXT | Default `open` |
| priority | INTEGER | Default 3 |
| sort_order | INTEGER | Default 0 |
| estimate_effort | INTEGER | Default 0 |
| estimate_complete | TEXT | Default empty |
| health_score | INTEGER | Default 0 |
| assignee | TEXT | Default empty |
| draft | INTEGER | Boolean, default 1. New tickets start as draft until explicitly readied for work. |
| complete | INTEGER | Boolean, default 0. When true, ticket is finished (stage=done). |
| archived | INTEGER | Boolean, default 0 |
| deleted | INTEGER | Boolean, default 0. Soft-delete flag. |
| recommended_ready | INTEGER | Boolean, default 0. Set when the refiner proposes the ticket is ready. |
| dor_map / dod_map / ac_map | TEXT | JSON guidance maps (Definition of Ready / Done / Acceptance Criteria) keyed by stage with a `default` fallback. |
| workflow_id | INTEGER | Nullable FK → workflows. Explicit per-ticket workflow override. |
| pr_url | TEXT | Pull-request URL recorded by agents. |
| role_id | INTEGER | FK → roles. Current active role within the stage. |
| previous_workflow_stage_id | INTEGER | Saved stage for reopen after completion. |
| previous_role_id | INTEGER | Saved role for reopen after completion. |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

#### 5.4.1 Ticket Key Format

```
{PROJECT_PREFIX}-{SEQUENCE}
```

Examples: `CUS-12`, `CUS-143`, `OPS-9`

- Unique across the whole system
- Generated by the server
- Immutable after creation
- Sequence is monotonically increasing per project
- The ticket type is validated at creation but is not embedded in the key

#### 5.4.2 Ticket Types

| Type | Code | Can be parent? |
|------|------|----------------|
| feature | F | Yes: epic. The "grand plan"/requirement, refined with a human + agent then broken down. |
| epic | E | Yes: story, bug, task, spike, chore |
| story | Y | No |
| task | T | Yes: task, bug, spike, chore |
| bug | B | No |
| spike | S | No |
| chore | C | No |
| note | N | No |
| question | Q | No |
| requirement | R | No |
| decision | D | No |
| idea | I | No |
| action | A | No |

> **Implementation note:** the parent-type rules in this table are the design
> intent. The current implementation validates that parent and child are valid
> ticket types and belong to the same project, but does not yet enforce the
> per-type parenting matrix.

The delivery hierarchy is **Release → Feature → Epic → Story/Bug**: a feature
contains epics, an epic contains stories/bugs, linked through `parent_id`
(story.parent = epic, epic.parent = feature). A feature is added to a release
(see section 5.3a and [`RELEASES.md`](./RELEASES.md)).

#### 5.4.3 Ticket Hierarchy

- `parent_id` represents the parent–child relationship
- Parent and child must belong to the same project
- A ticket may have at most one parent
- Cycles are forbidden
- A parent ticket with children derives its lifecycle from descendants
- A feature may be deep-cloned (its whole subtree) to extend functionality

### 5.5 Workflow

Defines a sequence of stages that tickets progress through.

| Field | Type | Constraints |
|-------|------|-------------|
| workflow_id | INTEGER | Primary key, autoincrement |
| name | TEXT | Unique, required |
| description | TEXT | Default empty |
| created_at | TEXT | Timestamp |
| updated_at | TEXT | Timestamp |

### 5.6 Workflow Stage

An individual stage within a workflow.

| Field | Type | Constraints |
|-------|------|-------------|
| workflow_stage_id | INTEGER | Primary key, autoincrement |
| workflow_id | INTEGER | FK → workflows |
| stage_name | TEXT | Required, unique per workflow |
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

Custom role definition for workflow stages.

| Field | Type | Constraints |
|-------|------|-------------|
| role_id | INTEGER | Primary key, autoincrement |
| workflow_id | INTEGER | FK → workflows. Roles are scoped to an Workflow. |
| title | TEXT | Unique per workflow_id, required |
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

Known keys: `registration_enabled`, `chat_enabled`, `chat_max_connections`,
`chat_max_duration_minutes`, `orchestrator_interval_seconds`,
`orchestrator_heartbeat_timeout_seconds`, `refinement_idle_minutes`,
`orchestrator_enabled_project_<id>`, plus agent-model and automation-policy
configuration keys.

### 5.22 Document

Project-scoped knowledge artifact. Documents (and their uploaded files)
contribute their content to the project context graph (5.24).

| Field | Type | Constraints |
|-------|------|-------------|
| document_id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects |
| title | TEXT | Required |
| description | TEXT | Default empty |
| notes | TEXT | Default empty |
| content | TEXT | Default empty (markdown/plain text) |
| created_at / updated_at | TEXT | Timestamps |

### 5.23 Document File

Uploaded binary attachment on a document (e.g. PDF, MD).

| Field | Type | Constraints |
|-------|------|-------------|
| file_id | INTEGER | Primary key, autoincrement |
| document_id | INTEGER | FK → documents, cascade delete |
| file_name | TEXT | Required |
| content_type | TEXT | Default empty |
| size_bytes | INTEGER | Derived from content |
| content | BLOB | Required |
| created_at | TEXT | Timestamp |

Documents also support project labels via a `document_labels` junction table.

### 5.24 Context Edge

The project **context graph** links tickets, documents, and external URLs.
Nodes are the entities themselves; edges are typed links. Every document in a
project is always a graph node; tickets and URLs join the graph when an edge
references them.

| Field | Type | Constraints |
|-------|------|-------------|
| edge_id | INTEGER | Primary key, autoincrement |
| project_id | INTEGER | FK → projects, cascade delete |
| source_type | TEXT | `ticket` \| `document` \| `url` |
| source_id | TEXT | Ticket key, numeric document id, or http(s) URL |
| target_type | TEXT | `ticket` \| `document` \| `url` |
| target_id | TEXT | As source_id |
| relation | TEXT | Default `references` |
| title | TEXT | Display title (used for url nodes) |
| created_by | TEXT | FK → users |
| created_at | TEXT | Timestamp |

**Rules:**
- Both endpoints must resolve inside the project (URLs must be http/https).
- Duplicate edges (same endpoints + relation) are rejected.
- The graph is queryable as a whole (`GET /api/projects/{ref}/context`) and by
  text search across document title/description/notes/content and ticket
  key/title/description (`GET /api/projects/{ref}/context/search?q=`).

### 5.25 Plan

Subscription/registration plan controlling quotas and on-registration actions
(default plan slug: `free`). Fields include max projects, private projects,
tickets, tickets per project, team memberships, API calls per day, and
registration actions (auto-assign public team, auto-create private project).

### 5.26 Other entities

The implementation also models: user notifications, project access requests,
inbox entries (failure-escalation mailbox), interventions, work items,
execution packets, ticket phase sign-offs, passkey credentials/flows, org
(singleton), programmes, and per-project git repositories
(`project_git_repositories`, CRUD-managed). See `internal/store/` and
[`openapi.yaml`](./api/openapi.yaml) for the full field lists.

---

## 6. Lifecycle Model

### 6.1 Stages

Stages are split into a **backlog** phase (preparation) and a **delivery**
phase (workflow execution). The full set understood by the system:

```
idea → refine → ready   (backlog)
design → develop → test → done   (delivery; workflow-defined)
```

| Stage | Phase | Meaning |
|-------|-------|---------|
| idea | backlog | Raw requirement awaiting refinement |
| refine | backlog | In the human/agent refinement dialogue |
| ready | backlog | Refined and approved; awaiting a release |
| design | delivery | Work is being appraised, explored, or refined |
| develop | delivery | Implementation is being done |
| test | delivery | The outcome is being verified |
| done | delivery | Ticket is complete |
| reject | delivery | Optional terminal stage for rejected work |
| complete | delivery | Terminal alias used by completion flows |

A ticket in `ready` cannot advance into delivery stages until it belongs to a
release that is `in_progress` (see 5.3a). Delivery stage names beyond the
defaults are workflow-defined (see 5.5/5.6).

The authoritative lifecycle specification is [`LIFECYCLE.md`](./LIFECYCLE.md).

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
- `complete` — sets `state=success`; auto-advances to next workflow stage with `state=idle`
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
- `observer` — read-only visibility
- `commenter` — observer plus the ability to add comments
- `member` — read/write tickets and stories
- `admin` — full project management

Legacy aliases are accepted and normalized: `viewer` → `observer`,
`editor` → `member`, `owner` → `admin`.

**Passkeys:** in addition to passwords, users can enroll WebAuthn passkeys for
website and CLI sign-in (`/api/auth/passkey/*`, `/api/users/me/passkeys`).

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
| `TICKET_HOME` | Global Ticket home directory (default `~/.ticket`) |
| `TICKET_TIMEOUT` | Remote HTTP timeout in seconds |
| `TICKET_TRUSTED_PROXY_CIDRS` | Comma-separated CIDRs trusted for forwarded proxy headers |
| `AGENT_ID` | Agent UUID for worker mode |
| `AGENT_PASSWORD` | Agent password |
| `TICKET_AGENT_LLM` | LLM command override |

### 11.2 Config Resolution

1. Resolve `$TICKET_HOME` from the environment or default it to `~/.ticket`
2. Read `TICKET_URL` to determine the target server
3. Walk up from the current directory looking for the nearest `.ticket/config.json`
4. Overlay repo-local `project_id` routing from that file when present
5. Reuse any matching session from `$TICKET_HOME/credentials.json`
6. Fall back to `TICKET_USERNAME` / `TICKET_PASSWORD` or `TICKET_TOKEN` when no stored session exists
7. When no explicit project is supplied, send the nearest git remote URL so the server can resolve project context

### 11.3 Config Files

- `.ticket/config.json` — repo-local routing (`project_id`)
- `$TICKET_HOME/preferences.json` — TUI state only
- `$TICKET_HOME/credentials.json` — remote auth tokens keyed by canonical remote URL
- `$TICKET_HOME/ticket.db` — default SQLite database used by `tk server`

---

## 12. CLI Commands

The binary is named `ticket` with the alias `tk`.

### 12.1 System

| Command | Description |
|---------|-------------|
| `tk initdb` | Create or repair the shared local database and bootstrap the default admin/project |
| `tk server` | Start HTTP server and web UI on :8080 |
| `tk version` | Show version |
| `tk upgrade` | Check for newer version from GitHub |
| `tk status` | Show connection status (mode, database, config) |
| `tk summary` | Show project summary, active tickets, recent activity |
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
| `tk admin config` | Manage server registration settings |
| `tk export -o file.json` | Export all data to JSON snapshot |
| `tk import -i file.json` | Restore from snapshot |
| `tk upgrade-database -o file.db` | Port an older database into a fresh file |

### 12.3 Projects

| Command | Description |
|---------|-------------|
| `tk project list` | List projects |
| `tk project create -prefix CUS -title "..."` | Create project |
| `TICKET_PROJECT=<id\|prefix>` / `-project_id <id\|prefix>` | Set active project |
| `tk project get <id>` | View project details |
| `tk project update <id> -title "..."` | Update project |
| `tk project delete <id>` | Delete project |
| `tk project repo add <id\|prefix> <git-url>` | Associate a git repository with a project |
| `tk project workflow <workflow-id>` | Assign an Workflow to the active project |
| `tk project set-draft <true\|false>` | Toggle draft mode on the active project |

### 12.3a Releases

| Command | Description |
|---------|-------------|
| `tk release list` | List releases for the active project |
| `tk release create -title "..." [-purpose "..."] [-target-date ...]` | Create a release (`in_design`) |
| `tk release update <id> -title "..."` | Update release title/purpose/target date |
| `tk release status <id> <in_design\|in_progress\|complete>` | Transition release status (seal / complete) |
| `tk release add-feature <release-id> <feature-id>` | Add a feature (and its subtree) to a release |
| `tk release remove <release-id> <feature-id>` | Remove a feature from a release |
| `tk release delete <id>` | Delete a release |
| `tk feature clone <feature-id>` | Deep-clone a feature and its subtree |

See [`RELEASES.md`](./RELEASES.md).

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
| `tk merge <target-id> <source-id>...` | Merge draft tickets into the first ticket and archive the rest |

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

### 12.12 Workflows

| Command | Description |
|---------|-------------|
| `tk admin workflow list` | List workflows |
| `tk admin workflow get -id <id>` | View workflow stages |
| `tk admin workflow create -name "..." -description "..."` | Create workflow |
| `tk admin workflow delete -id <id>` | Delete workflow |
| `tk admin workflow add-stage <workflow-id> <stage-name>` | Add stage |

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
| `tk admin role list` / `tk admin role ls` | List all roles |
| `tk admin role create -title "..." -description "..." -ac "..."` | Create role |
| `tk admin role update -id <id> -title "..."` | Update role |
| `tk admin role delete -id <id>` / `tk admin role rm -id <id>` | Delete role |
| `tk admin workflow stage-role-add -workflow_id <id> -stage_id <id> -role_id <id>` | Assign role to stage |
| `tk admin workflow stage-role-rm -workflow_id <id> -stage_id <id> -role_id <id>` | Remove role from stage |
| `tk admin workflow stage-role-order -workflow_id <id> -stage_id <id> -roles <ids>` | Reorder roles in stage |

### 12.15 Teams

| Command | Description |
|---------|-------------|
| `tk admin team list` | List teams |
| `tk admin team create -name "..."` | Create team |
| `tk admin team get -id <id>` | View team |
| `tk admin team update -id <id> -name "..."` | Update team |
| `tk admin team delete -id <id>` | Delete team |
| `tk admin team add-member <team-id> <user-id>` | Add member |
| `tk admin team remove-member <team-id> <user-id>` | Remove member |

### 12.16 Users (Admin)

| Command | Description |
|---------|-------------|
| `tk admin user list` | List users |
| `tk admin user create -username <name> -password <pass>` | Create user |
| `tk admin user enable <username>` | Enable user |
| `tk admin user disable <username>` | Disable user |
| `tk admin user reset-password <username>` | Change password |
| `tk admin user delete <username>` | Delete user |

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

### 12.20 Additional Commands

| Command | Description |
|---------|-------------|
| `tk init` | Create a project from the nearest git remote (slated for removal per TODO.md) |
| `tk demo` | Seed demo/example data (see also `tk initdb -populate`; a dedicated `tk seed` is planned in TODO.md but not implemented) |
| `tk ready -id <id>` / `tk notready -id <id>` | Mark ticket ready / not ready (aliases of undraft/draft) |
| `tk open -id <id>` | Reopen a closed ticket |
| `tk edit -id <id>` | Edit a ticket interactively |
| `tk req` | Shortcut for requirements (`tk idea`) |
| `tk request-dryrun` | Dry-run version of `tk request` |
| `tk add-dependency` / `tk remove-dependency` | Dependency shortcuts |
| `tk orchestrator [-id <id> \| -project_id <id>] [-apply]` | Run an orchestrator pass (dry-run by default) |
| `tk intervene` | Record a human intervention on a ticket |
| `tk document` | Document management namespace |
| `tk skill` | Agent skill/capability management |
| `tk prompt` | Agent prompt inspection namespace |
| `tk story` | Story namespace |
| `tk ticket` / `tk work-item` | Nested ticket / work-item namespaces |
| `tk docker-compose` | Docker compose helper |

---

## 13. HTTP API

All endpoints are under `/api/`. Request and response bodies are JSON.
Error responses use `{"error": "message"}` with appropriate HTTP status codes.

See [`openapi.yaml`](./api/openapi.yaml) for the full OpenAPI 3.1 specification.

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

#### Workflows
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/workflows` | User | List workflows |
| POST | `/api/workflows` | Admin | Create workflow |
| POST | `/api/workflows/import` | Admin | Import workflow |
| GET | `/api/workflows/{id}` | User | Get workflow with stages |
| DELETE | `/api/workflows/{id}` | User | Delete workflow |
| POST | `/api/workflows/{id}/stages` | User | Add stage |
| PUT | `/api/workflows/{id}/reorder` | User | Reorder stages |
| GET | `/api/workflows/{id}/export` | User | Export workflow |
| DELETE | `/api/workflows/stages/{id}` | Admin | Delete stage |

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
| GET | `/api/projects/{ref}/releases` | Read | List releases |
| POST | `/api/projects/{ref}/releases` | Write | Create release |

#### Releases
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PUT | `/api/releases/{id}` | Write | Update release (title, purpose, target_date) |
| POST | `/api/releases/{id}/status` | Write | Set status (`in_design`/`in_progress`/`complete`) |
| DELETE | `/api/releases/{id}` | Write | Delete release |

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
| POST | `/api/tickets/{ref}/clone` | Write | Clone a feature and its subtree |
| PUT | `/api/tickets/{ref}/release` | Write | Add feature to / remove from a release |
| POST | `/api/tickets/{ref}/close` | Write | Close ticket |
| POST | `/api/tickets/{ref}/open` | Write | Reopen ticket |
| POST | `/api/tickets/{ref}/archive` | Write | Archive ticket |
| POST | `/api/tickets/{ref}/unarchive` | Write | Unarchive ticket |
| POST | `/api/tickets/{ref}/ready` | Write | Mark ready |
| POST | `/api/tickets/{ref}/notready` | Write | Mark not ready |
| POST | `/api/tickets/{ref}/workflow` | Write | Set workflow |
| DELETE | `/api/tickets/{ref}/workflow` | Write | Remove workflow |
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
| GET | `/api/refinement/ws?ticket=ID` | Bearer | Streaming refiner dialogue (token-by-token) |

#### Passkeys
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/passkey/login/start` | None | Begin passkey login |
| POST | `/api/auth/passkey/register/start` | Bearer | Begin passkey enrollment |
| GET | `/api/auth/passkey/challenge` | None | Fetch challenge |
| POST | `/api/auth/passkey/finish` | None | Complete a passkey flow |
| GET | `/api/auth/passkey/poll` | None | Poll cross-device flow |
| GET/DELETE | `/api/users/me/passkeys[/{credential_id}]` | Bearer | Manage own passkeys |

#### Documents
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET/POST | `/api/projects/{ref}/documents` | Read/Write | List/create project documents |
| GET/PUT/DELETE | `/api/documents/{id}` | Read/Write | Get/update/delete document |
| GET/POST | `/api/documents/{id}/files` | Read/Write | List/upload document files |
| GET/DELETE | `/api/documents/{id}/files/{file_id}` | Read/Write | Download/delete file |
| GET/POST/DELETE | `/api/documents/{id}/labels` | Read/Write | Manage document labels |

#### Context Graph
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/projects/{ref}/context` | Read | Project context graph (nodes + edges) |
| GET | `/api/projects/{ref}/context/search?q=` | Read | Search context nodes by text |
| GET | `/api/tickets/{ref}/context` | Read | List context links on a ticket |
| POST | `/api/tickets/{ref}/context` | Write | Attach a document, URL, or ticket as context |
| DELETE | `/api/tickets/{ref}/context/{edge_id}` | Write | Remove a context link |

#### Refinement and Decomposition
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/tickets/{ref}/refinement/approve` | Write | Approve refinement (single story → ready; breakdown → epic + ready stories) |
| POST | `/api/tickets/{ref}/children/reorder` | Write | Reorder a ticket's children (reprioritize the proposed decomposition before sign-off) |

#### Lifecycle Actions (Tickets)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/tickets/{ref}/complete` \| `/reopen` | Write | Complete / reopen |
| POST | `/api/tickets/{ref}/draft` \| `/undraft` | Write | Toggle draft |
| POST | `/api/tickets/{ref}/next` \| `/previous` | Write | Advance / regress role or stage |
| POST | `/api/tickets/{ref}/intervene` | Write | Record a human intervention |
| GET/POST | `/api/tickets/{ref}/intervention-state` | Read/Write | Intervention state |
| GET | `/api/tickets/{ref}/execution-packet` | Read | Assembled agent execution packet |
| GET/POST | `/api/tickets/{ref}/inbox[...]` | Member | Failure-escalation mailbox entries |
| GET/POST | `/api/tickets/{ref}/work-items[...]` | Member/Write | Work items and actions (reassign/cancel/retry/feedback) |
| GET/POST | `/api/tickets/{ref}/phase-signoffs[/{phase}]` | Read/Write | Phase sign-off records |

#### Plans, Users, and Registration
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET/POST | `/api/plans` | User/Admin | List/create plans |
| GET/POST | `/api/plans/default` | User/Admin | Get/set default plan |
| GET/PUT/DELETE | `/api/plans/{ref}` | Admin | Manage a plan |
| GET/PUT | `/api/users/{username}/plan` | Admin | Get/set a user's plan |
| POST | `/api/users/{username}/reset-password` | Admin | Reset password |
| GET | `/api/users/me/notifications` | Bearer | List own notifications |
| POST | `/api/users/me/notifications/{id}/read` | Bearer | Mark notification read |
| GET/PUT | `/api/users/me/default-project` | Bearer | Get/set default project |
| GET | `/api/users/me/access-requests` | Bearer | Own access requests |

#### Project Management Extensions
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/projects/by-repository?git_repository=` | User | Resolve project from git remote |
| GET/POST/DELETE | `/api/projects/{ref}/repositories[/{repository}]` | Admin | CRUD project git repositories |
| POST | `/api/projects/{id}/set-draft` | Write | Toggle project draft mode |
| GET/POST | `/api/projects/{ref}/access-requests` | User/Owner | Request / list access |
| POST | `/api/projects/{ref}/access-requests/{id}/{approve\|reject}` | Owner | Decide access request |
| GET | `/api/projects/{id}/interventions[/report\|/trends]` | Member | Intervention analytics |
| GET | `/api/projects/{id}/forecast[/calibration]` | Member | Delivery forecasting |
| GET | `/api/projects/{id}/work-items/queue` | Member | Work-item queue |
| GET/POST | `/api/projects/{id}/orchestrator` | Admin | Per-project orchestrator enable/disable |

#### Configuration and System
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET/POST | `/api/config/settings[/{key}]` | Admin | Generic settings CRUD |
| GET/POST | `/api/config/agent-model` | Admin | Agent LLM model config |
| GET/POST | `/api/config/orchestrator` | Admin | Orchestrator settings |
| GET/POST | `/api/config/automation_policy` | Admin | Automation policy |
| GET/POST | `/api/agents/{id}/config[/{key}]` | Admin | Per-agent configuration |
| GET | `/metrics` | None | Prometheus-style metrics |
| GET/PUT | `/api/org` | User/Admin | Org singleton |
| GET/POST/PUT/DELETE | `/api/programmes[/{id}]` | User/Admin | Programme CRUD |

#### Workflow Extensions
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/workflows/{id}/validate` | User | Validate workflow graph |
| GET/POST | `/api/workflows/stages/{id}/transitions` | User/Admin | Stage transition edges |
| GET/POST/DELETE | `/api/workflows/stages/roles/{workflow_id}/{stage_id}[/{role_id}]` | Admin | Stage-role assignment and ordering |

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

Indexes on: `sessions(user_id, token)`, `tickets(project_id, parent_id, assignee, stage, state)`, `stories(project_id)`, `story_ticket_links(ticket_id)`, `history_events(project_id, ticket_id)`, `ticket_history(project_id, ticket_id)`, `comments(item_id, user_id)`, `dependencies(project_id, ticket_id, depends_on)`, `labels(project_id)`, `ticket_labels(label_id)`, `time_entries(ticket_id, user_id)`, `workflow_stages(workflow_id, role_id)`.

### 14.5 Initialization

`tk initdb` creates the database, schema, admin user, default workflow (`design → develop → test → done`), a default project with prefix `TK`, and the appropriate local-remote wiring (`~/.ticket/ticket.db` for `tk initdb`, `./.ticket/ticket.db` for `tk initdb .`). Remote project resolution then uses explicit project selection, nearest-git-remote discovery, or the user's default project without a separate repo bootstrap step.

---

## 15. Web UI

The web UI is a single-page application with pre-compiled static assets embedded into the Go binary using `//go:embed`. It is served from the same port as the API (`:8080`). Unknown routes are rewritten to `index.html` (SPA fallback).

Assets live in `web/default/` (site-specific, e.g. `index.html`, `site.css`)
overlaid on `web/shared/` (shared logic). JavaScript is split into:

- `api.js` — a thin client that maps 1:1 onto the HTTP API (no UX logic)
- `app.js` — all UX logic, calling `api.js`/the API as needed

CSS and JS are external files; no styling or logic is embedded in the HTML
(except a small theme-bootstrap snippet that must run before first paint).

Notable UI features: kanban board, list/plan views, ticket modal with
Details/Refinement/Properties/Activity tabs, refinement chat with streaming
refiner replies, proposed-breakdown reordering before approval, ticket context
attachments (documents/URLs/tickets), document management with file upload,
plan/quota administration, and an AI-provider (agent-model) settings panel.

The **Context view** (Process nav → Context) renders the project context graph
(5.24) as an interactive SVG node-link map: tickets, documents, and URLs are
color-coded nodes, clicking a node opens it, and a search box highlights
matches via `/context/search`. "View in graph" in a ticket's Context section
jumps to the graph focused on that story with its direct context highlighted
and everything else dimmed.

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
| `make homebrew` | Push the generated formula to the Homebrew tap |
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
│   ├── client/              # HTTP client for server mode
│   ├── config/              # Configuration resolution
│   ├── password/            # Argon2id password hashing
│   ├── server/              # HTTP server, API handlers, WebSocket
│   ├── store/               # SQLite database layer
│   └── tui/                 # Terminal UI (BubbleTea)
├── libticket/               # Public service interface, implementations, and contract tests
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
- Context graph: edge CRUD, node validation, graph assembly, and text search
- Decomposition reordering: permutation validation and persisted child order
