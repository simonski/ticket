# Ticket — Issue Tracking for Agents

This project uses `ticket` (aliased as `tk`) for issue tracking. All work is managed through CLI commands using a workflow.

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
tk list,ls

# Filter by type, status, or assignee
tk list --type task
tk list --type epic
tk list --type bug
tk list --status develop/idle
tk list -u alice

# Search titles and descriptions
tk search "password reset"

# View ticket detail with history and comments
tk get <id>
tk get -json <id>

# List tickets with no parent
tk orphans

# Aggregate counts by type and status
tk count
```

## Creating Work

```bash
# Create tickets (title as positional arg or -title flag)
tk add "Implement password reset"
tk bug "Login fails on Safari"
tk epic "Authentication overhaul"

# With full options
tk create -t task -title "Fix signup" -d "Description here" -ac "Acceptance criteria" -p <project-id> -parent <epic-id>

# Create and specify priority/estimates
tk add -title "Urgent fix" -priority 1 -effort 3
```

## Updating Work

```bash
# Update fields
tk update <id> -title "New title" -d "New description" -ac "New criteria"
tk update <id> -priority 2 -effort 5

# Set parent/hierarchy
tk set-parent <child-id> <parent-id>
tk unset-parent <child-id>
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
tk idle <id>            # Pause work
tk complete <id>        # Mark success (auto-advances stage)
tk state <id> active    # Set state directly
tk state <id> success   # Completes current stage, advances to next
tk state <id> fail
```

## Assignment

```bash
# Self-assign / unassign
tk claim <id>
tk unclaim <id>

# Request next available ticket
tk request

# Admin-only: assign to others
tk assign <id> <username>
tk unassign <id> <username>
```

## Comments and History

```bash
tk comment add <id> "Blocked on API changes"
tk history <id>
```

## Other Operations

```bash
# Archive / restore
tk archive <id>
tk unarchive <id>

# Close / reopen
tk close <id>
tk open <id>

# Delete permanently
tk delete <id>

# Dependencies
tk dependency add <id> <depends-on-id>
tk dependency remove <id> <depends-on-id>

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

1. **Pick up work**: `tk list --status design/idle`, then `tk claim <id>` and `tk state <id> active`
2. **Track progress**: `tk complete <id>` when stage work is done — auto-advances to next stage
3. **File new issues**: create tickets for anything discovered during work
4. **Comment**: leave context on tickets for future sessions
5. **Complete work**: keep completing stages until the ticket reaches the final stage with `success`
