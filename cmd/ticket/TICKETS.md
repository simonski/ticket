# Ticket — Issue Tracking for Agents

This project uses `tk` for issue tracking. All work is managed through CLI commands using a workflow.

## Setup

```bash
# Check connection and auth status
tk status

# See available projects
tk project ls

# Set active project (used as default for subsequent commands)
tk project use <id>
```

## Viewing Work

```bash
# List tickets in the active project
tk ls

# Filter by type, status, or assignee
tk ls --type task
tk ls --type epic
tk ls --type bug
tk ls --status develop/idle
tk ls -user alice

# Search titles and descriptions
tk search "password reset"

# View ticket detail with history and comments
tk get -id <id>
tk get -id <id> -json

# List tickets with no parent
tk orphans

# Aggregate counts by type and status
tk count
```

## Creating Work

```bash
# Create tickets (title as positional arg or -title flag)
tk new "Implement password reset"
tk bug "Login fails on Safari"
tk epic "Authentication overhaul"

# With full options
tk new -t task -title "Fix signup" -d "Description here" -ac "Acceptance criteria" -p <project-id> -parent <epic-id>

# Create and specify priority/estimates
tk new -title "Urgent fix" -priority 1 -estimate_effort 3

# Shorthand typed creation
tk note "Meeting notes from standup"
tk question "Should we migrate to Postgres?"
```

## Updating Work

```bash
# Update fields
tk update -id <id> -title "New title" -d "New description" -ac "New criteria"
tk update -id <id> -priority 2 -estimate_effort 5

# Set parent/hierarchy
tk set-parent -id <child-id> <parent-id>
tk unset-parent -id <child-id>
```

## Status Lifecycle

Tickets have a two-part status: `stage/state` (e.g. `develop/active`, `done/success`).

### Workflow-Driven Stages

Stages are defined by the project's workflow (an ordered sequence of stages). The default workflow has: `design → develop → test → done`.

Stages advance automatically: when a ticket's state is set to `success`, it moves to the next workflow stage with state `idle`. On the final stage, `success` means the ticket is complete.

You cannot set a ticket's stage directly — use state commands to drive progression.

```bash
# View a project's workflow stages
tk workflow get -id <workflow-id>
```

### State Commands

States: `idle`, `active`, `success`, `fail`

```bash
tk idle -id <id>            # Pause work
tk complete -id <id>        # Mark success (auto-advances stage)
tk state -id <id> active    # Set state directly
tk state -id <id> success   # Completes current stage, advances to next
tk state -id <id> fail
```

## Assignment

```bash
# Self-assign / unassign
tk claim -id <id>
tk unclaim <id>

# Request next available ticket
tk request

# Admin-only: assign to others
tk assign <id> <username>
tk unassign <id> <username>
```

## Labels

```bash
# Manage project labels
tk label create -name "bug" -color "red"
tk label ls
tk label delete <label-id>

# Tag tickets
tk label add <ticket-id> <label-id>
tk label remove <ticket-id> <label-id>
tk label show <ticket-id>

# Filter list by label
tk list --label "bug"
```

## Roles

```bash
# List all roles
tk role list
tk role ls

# Create a role
tk role create -title "Security Lead" -motivation "Protect systems" -goals "Zero breaches"

# Update a role
tk role update -id <id> -title "New Title" -motivation "Updated" -goals "Updated"

# Delete a role
tk role delete -id <id>
```

## Time Tracking

```bash
# Log time against a ticket (minutes)
tk time log -id <ticket-id> -m 30 -note "Morning session"
tk time list <ticket-id>
tk time total <ticket-id>
tk time delete <entry-id>
```

## Board View

```bash
# Kanban-style view grouped by workflow stage
tk board
```

## Requirements and Decisions

```bash
# Curate a requirement from existing tickets
tk curate <id> [id...]

# Review requirements by status
tk review
tk review -status proposed
tk review -status accepted

# Accept or reject a requirement
tk accept requirement <id>
tk reject requirement <id>

# Mark a requirement as revised
tk revise requirement <id>

# Record and list decisions
tk decision add "Use Postgres for production"
tk decision list

# View ticket conversation (history + comments)
tk conversation show <id>
```

## Comments and History

```bash
tk comment add -id <id> "Blocked on API changes"
tk history <id>
```

## Other Operations

```bash
# Archive / restore
tk archive -id <id>
tk unarchive -id <id>

# Close / reopen
tk close -id <id>
tk open -id <id>

# Delete permanently
tk delete -id <id>

# Dependencies
tk dependency add -id <id> <depends-on-id>
tk dependency remove -id <id> <depends-on-id>

# Clone a ticket or epic
tk clone <id>
```

## Project Management

```bash
tk project ls                      # List projects (* = current)
tk project create -title "Name"    # Create project
tk project use <id>                # Switch active project
tk project get <id>                # View project detail
tk project init                    # Write .ticket.json in current dir
```

## Workflow Guidelines

1. **Pick up work**: `tk ls --status design/idle`, then `tk claim -id <id>` and `tk state -id <id> active`
2. **Advance to develop before coding**: Once design is done and you are about to write code, run `tk complete -id <id>` to advance the ticket from design → develop, then `tk state -id <id> active` to set it to develop/active. **Never start coding on a ticket that is still in design/active.**
3. **Mark ready when in development**: Once the ticket is in develop/active and coding has begun, mark it ready: `tk ready -id <id>`. This signals the ticket is in active development.
4. **Track progress**: `tk complete -id <id>` when a stage's work is done — auto-advances to the next stage
5. **File new issues**: create tickets for anything discovered during work
6. **Comment**: leave context on tickets for future sessions
7. **Complete work**: keep completing stages until the ticket reaches the final stage (`done`) with `success`

### Stage Lifecycle Reference

```
design/idle  →  design/active  →  [complete]  →  develop/idle  →  develop/active  →  [complete]  →  test/idle  → ...  →  done/success
```

- **design**: Understand the problem, design the solution
- **develop**: Write the code (ticket should be develop/active and `ready=yes` while coding)
- **test**: Verify the solution works, run tests
- **done**: Work is fully complete
