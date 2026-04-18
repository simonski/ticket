---
name: tk
description: Use this skill when working on tasks within a codebase that uses `tk` for ticket tracking. Applies when starting work on a feature or bug, completing work, needing to understand current project state, capturing new requirements or decisions, logging time, or when the user references a ticket ID. Also applies when the user says things like "what are we working on", "mark that done", "log this as a bug", or "record that decision".
metadata: 
    version: 0.0.1
---

# tk Ticket Management Skill

`tk` is the project's ticket and sdlc management CLI. You must use it to read and update task state throughout your work. Do not rely on memory or conversation history for ticket state — always query `tk` directly.

## Core Principle

**Before starting any significant piece of work, check the active project and relevant tickets. After completing work, update ticket state.**

## Error Recovery: Missing config.json

If any `tk` command fails with `no active project; use 'ticket project create' or 'ticket project use <id>' first`, this usually means `.ticket/config.json` is missing or has no `project_id` while `.ticket/ticket.db` exists. Do NOT ask the user to fix this manually. Instead, recover automatically:

1. Run `tk project list` to see what projects exist in the database.
2. If exactly one project exists, activate it with `tk project use <prefix>`.
3. If multiple projects exist, pick the first one and activate it with `tk project use <prefix>`.
4. Verify the fix worked by re-running the original command.
5. Tell the user what you did: which project you activated and that config.json was repaired.

If `tk project list` returns no projects at all, then run `tk project init` to create one from the current directory name, then retry.

## Project Context
```bash
# Check current project and connection
tk status

# Check a summary of this project
tk summary

# List open tickets
tk list
```

## SDLC Stages

The default sdlc has four stages: `design`, `develop`, `test`, `done`. Each ticket also has a state (e.g. `idle`, `active`, `success`, `failed`). Together these form the ticket's status as `stage/state` (e.g. `develop/active`).

Always check the project's available sdlc stages before transitioning — don't assume stage names:
```bash
tk sdlc list
tk sdlc get -id <sdlc-id>
```

## Starting Work

When the user asks you to work on a ticket or feature:
```bash
# Find the ticket
tk list

# View a specific ticket
tk show <id>

# Move ticket to develop stage 
tk state <id> develop

# And set it to active
tk state <id> active
```
**Important:** 
  - Always move tickets to `develop/active` when you begin implementation. 
  - Do not leave tickets in `design/idle` while actively working on them.

## Completing Work - Success

When a task is done, **always** set it to either `success` or `failed` state and close it:

```bash
# Mark ticket as successfully completed
tk state <id> success

# Add a completion comment summarising what was done
tk comment <id> "What was done and any relevant notes"

# Log time if the user mentioned duration
tk time log <id> <duration> "description"

# Close the ticket if it was a success
tk close <id>
```

**Important:** Do not leave tickets in `active` state after work is complete. Always transition to `success` before closing. For epics, close all child tasks first — the epic's state derives from its children.

## Completing Work - Failure

When a task fails and cannot be completed:

```bash
# Mark ticket as failed
tk state <id> failed

# Add a comment explaining what happened
tk comment <id> "What was attempted and why it failed"

# Close the ticket
tk close <id>
```


## Creating Tickets
```bash
# General ticket
tk add "Title of work"

# Bug
tk bug "Description of the bug"

# Epic (with child tasks)
tk epic "Name of epic"
tk add "Child task" --parent <epic-id>

# Capture a requirement or idea
tk idea "The requirement"
```


## Recording Decisions

When you make an architectural or design decision during your work, record it:
```bash
tk decision add "Decision title" --rationale "Why this approach was chosen"
tk decision list   # review existing decisions before making new ones
```

## Requirements
```bash
# List requirements
tk ideas

# Shape / refine a requirement
tk req shape <id>

# Accept or reject
tk req accept <id>
tk req reject <id> --reason "reason"
```

## Dependencies

When work depends on another ticket:
```bash
tk dep add <from-id> <to-id>
tk dep list <id>
```

## Labels
```bash
tk label list
tk label <id> <label>
```

## Key Habits

1. **On session start** — run `tk list` to see open tickets and orient yourself
2. **Before implementing** — run `tk show <id>` to read the full ticket, then `tk state <id> develop` and `tk state <id> active`
3. **On completion** — `tk state <id> success`, add a comment, then `tk close <id>`
4. **On decisions** — always `tk decision add` rather than leaving decisions implicit in code
5. **On new bugs found during work** — `tk bug "..."` immediately so nothing is lost
6. Only work on 1 ticket at a time — only have 1 active ticket in `develop/active` state at a time


# Ticket — Issue Tracking for Agents

This project uses `tk` for issue tracking. All work is managed through CLI commands using a sdlc.

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

### SDLC-Driven Stages

Stages are defined by the project's sdlc (an ordered sequence of stages). The default sdlc has: `design → develop → test → done`.

Stages advance automatically: when a ticket's state is set to `success`, it moves to the next sdlc stage with state `idle`. On the final stage, `success` means the ticket is complete.

You cannot set a ticket's stage directly — use state commands to drive progression.

```bash
# View a project's sdlc stages
tk sdlc get -id <sdlc-id>
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
# Kanban-style view grouped by sdlc stage
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

## SDLC Guidelines

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

