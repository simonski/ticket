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
