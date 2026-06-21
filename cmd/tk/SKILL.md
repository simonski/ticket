---
name: tk
description: Use this skill for ticket/project workflow operations in repositories using the `tk` CLI.
metadata:
  version: 0.0.3
---

# tk Skill

Use `tk` as the source of truth for ticket status, lifecycle, and context. This is a binary that is on the `$PATH`. It is already authenticated to talk to the server, so you can just run `tk` commands in your terminal or execute them as part of coding sessions.

## Core rule: update the ticket *as you go*

The ticket is a live journal of the work, not a form you fill in at the end. **Every meaningful step in the work must be mirrored immediately by a `tk` command** — before you start, while you implement, while you test, and when you finish.

Do **not** batch lifecycle changes at the end (e.g. writing all the code and only then claiming, activating, and completing the ticket). That hides progress and defeats the purpose of the tracker. The correct pattern is to interleave: each change to the *work* is paired with a change to the *ticket*.

Most lifecycle commands accept `-m "comment"` — use it on every transition so the state change and the explanation land together.

## Trigger phrase behavior

When the user says **"refine ticket N"** / **"refine N"**: read the ticket with `tk get N -v` and refine it so it has a clear title, description, acceptance criteria, and any missing or unclear information tidied up, until you can say "this ticket is unambiguous and ready to work on". If you cannot reach that state, `tk comment N "..."` with what is missing or unclear and ask for human input.

When the user says **"work on ticket N"** / **"work on N"** / **"ticket N"**: follow the lifecycle sequence below.

## The lifecycle sequence (start → working → testing → complete)

This is the canonical template. Run the `tk` command at each phase *as you reach it*, not afterwards.

### 1. Start — before writing any code

```bash
tk get N -v                       # read current state; confirm work can begin
tk prompt N                       # load the full SDLC / project execution context
tk claim N                        # self-assign
tk ready N                        # publish if it is still a draft
tk active N -m "starting: <one-line plan>"   # mark active AND record your plan
```

If `tk get N` shows the ticket is already done, blocked on an unmet dependency, or otherwise not ready, stop: `tk comment N "..."` with what blocks it and ask for human input.

Use the ticket to decide which git branch to work in. If it does not say, follow the SDLC rules and include the ticket ID in the branch name, e.g. `feature/TK-42`.

### 2. Working — while you implement

Pair each meaningful change with a ticket update. After a notable commit, decision, or sub-task:

```bash
tk stage N develop -m "implementation underway: <what you are building>"
tk comment N "<what you just did, decided, or discovered>"
```

Comment again whenever you change direction, hit a surprise, or finish a sub-part. Several small comments during the work are expected and correct.

### 3. Testing — while you verify

```bash
tk stage N test -m "verifying: make test / make lint"
```

Then record the outcome:

```bash
tk success N -m "all green: <test + lint summary>"   # tests pass
tk fail N    -m "blocked: <reason>"                  # you cannot make it pass
```

### 4. Complete — when the work is truly done

```bash
tk complete N -m "done: <summary of change>, tests + lint green"   # stage=done, complete=true
tk close N                                                          # close when fully wrapped up
```

`tk complete` finishes the ticket (sets stage `done`, `complete=true`) in one step. Do not leave finished work sitting in `active`.

## Lifecycle command reference

```bash
tk active   N [-m "..."]    # mark active in the current stage
tk idle     N [-m "..."]    # pause work
tk stage    N <design|develop|test|done> [-m "..."]   # set the stage
tk state    N <idle|active|success|fail> [-m "..."]   # set the state
tk success  N [-m "..."]    # mark the current stage successful
tk fail     N [-m "..."]    # mark failed / blocked
tk next     N               # advance to the next role/stage
tk complete N [-m "..."]    # finish: stage=done, complete=true
tk reopen   N [-m "..."]    # reopen a completed ticket
tk comment  N "text"        # add a standalone comment
tk claim    N               # self-assign
tk ready    N               # publish a draft (undraft)
tk close    N               # close
```

All of these accept either a positional id (`tk active 42`) or `-id` (`tk active -id 42`).

## Daily orientation

```bash
tk status
tk summary
tk ls
```

## Ticket detail behavior

```bash
tk get N            # concise view (id/type, title, description, a/c)
tk get N -v         # full detail view (history, lifecycle fields, etc.)
tk prompt N         # agent prompt with execution context
```

## Create/update tickets

```bash
# single ticket
tk new "Title"
tk bug "Bug title"
tk epic "Epic title"

# file-driven create/update preview + commit flow
tk new -f <filename>               # preview only
tk new -f <filename> -commit       # write tickets
tk update -f <filename>            # preview only
tk update -f <filename> -commit    # apply updates
```

## Useful operations

```bash
tk ls
tk search "text"
tk history N
tk dep add -id N <depends-on-id>
tk dep remove -id N <depends-on-id>
tk label ls
tk label add -id N <label-id>
tk time log -id N -m <minutes> -note "note"
```

## Project and setup

```bash
tk project ls
export TICKET_PROJECT=<id-or-prefix>
tk initdb
```
