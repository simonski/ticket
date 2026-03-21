---
name: tk
description: Use this skill when working on tasks within a codebase that uses `tk` for ticket tracking. Applies when starting work on a feature or bug, completing work, needing to understand current project state, capturing new requirements or decisions, logging time, or when the user references a ticket ID. Also applies when the user says things like "what are we working on", "mark that done", "log this as a bug", or "record that decision".
---

# tk Ticket Management Skill

`tk` is the project's ticket and workflow management CLI. You must use it to read and update task state throughout your work. Do not rely on memory or conversation history for ticket state — always query `tk` directly.

## Core Principle

**Before starting any significant piece of work, check the active project and relevant tickets. After completing work, update ticket state.**

## Project Context
```bash
# Check current project and connection
tk status

# check a summary of this project
tk summary

# List open tickets (shorthand)
tk list
```

## Starting Work

When the user asks you to work on a ticket or feature:
```bash
# Find the ticket
tk list

# View a specific ticket
tk show <id>

# Move to in-progress (check available workflow stages first)
tk workflow list
tk state <id> <stage>
```

## Creating Tickets
```bash
# General ticket
tk add "Title of work"

# Bug
tk bug "Description of the bug"

# Epic
tk epic "Name of epic"

# Capture a requirement or idea
tk idea "The requirement"
```

## Failing Work

When a task fails and cannot be completed, **always** close the ticket by setting it to `failed` state and adding a comment:

```bash
# Mark ticket as successfully completed
tk state <id> failed

# Add a completion comment summarising what was done
tk comment <id> "What was done and any relevant notes"

# Log time if the user mentioned duration
tk time log <id> <duration> "description"

tk close <id> 
```


## Completing Work

When a task is done, **always** close the ticket by setting it to `success` state and adding a comment:

```bash
# Mark ticket as successfully completed
tk state <id> success

# Add a completion comment summarising what was done
tk comment <id> "What was done and any relevant notes"

# Log time if the user mentioned duration
tk time log <id> <duration> "description"

tk close <id> 

```

**Important:** Do not leave tickets in `active` state after work is complete. Always transition to `success` (or `fail` if the work was abandoned/unsuccessful) before ending the session.

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

## Workflow

Always check the project's workflow stages before transitioning state — don't assume stage names:
```bash
tk workflow list
```

## Labels
```bash
tk label list
tk label <id> <label>
```

## Key Habits

1. **On session start** — run `tk` to see the current ticket list and orient yourself
2. **Before implementing** — run `tk ticket show <id>` to read the full ticket
3. **On completion** — update ticket state and add a comment summarising what changed
4. **On decisions** — always `tk decision add` rather than leaving decisions implicit in code
5. **On new bugs found during work** — `tk bug "..."` immediately so nothing is lost
6. Only work on 1 ticket at a time — don't switch between tasks without updating ticket state to reflect what you're doing.  Only have 1 active ticket in "in-progress" state at a time.
