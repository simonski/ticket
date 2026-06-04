---
name: tk
description: Use this skill for ticket/project workflow operations in repositories using the `tk` CLI.
metadata:
  version: 0.0.2
---

# tk Skill

Use `tk` as the source of truth for ticket status, lifecycle, and context.  This is a binary that is on the $PATH.  It is already authenticated to talk to the server, so you can just run `tk` commands in your terminal or execute them as part of coding sessions.

## Core rule

Always read ticket state from `tk` before acting using `tk get N` and `tk prompt N`, and update ticket state/comments after meaningful progress using `tk update` commands.

## Trigger phrase behavior

When the user says **"refine ticket N"** or **"refine N", read the ticket using `tk get N` and perform a refinement operation where yuo are updating the story to have title, description, acceptance criteria and any other relevant missing or unclear information tidied up, such that you can say " this ticket is unambiguous and ready to work on".  If you cannot refine the ticket to that state, then comment on the ticket with what is missing or unclear and ask for human input."

When the user says **"work on ticket N"** or **"ticket N", "work on N" **, do this flow:

1. `tk get N`.   Verify the ticket is in a state where work can begin (e.g. not already done, not blocked on dependencies, etc).  If it is not ready, comment on the ticket with what is missing or blocking and ask for human input.

2. `tk prompt N`.   Retrieve the "entire" prompt meaning the project SDLC and associated information.  

3. Begin implementation work for that ticket.
  tk state N idle|active|success|fail|design|develop|test|done> [-m comment]
  tk stage N idle|active|success|fail|design|develop|test|done> [-m comment]
When you have completed the work and believe it is successful

Note: use the ticket to expain which branch to work in on git.  If there is no indication in the ticket, then use the SDLC rules, making use of the ticket ID as part of the branch, e.g. feature/TK-42

4. tk success N

If you believe you cannot complete teh work

5. tk fail N

This phrase should be interpreted as: get the ticket and the prompt for that ticket, then begin work on it.

## Daily orientation

```bash
tk status
tk summary
tk ls
```

## Ticket detail behavior

```bash
# concise view (default)
tk get -id <id>

# full detail view
tk get -id <id> -v

# agent prompt for execution context
tk prompt <ticket-id>
```

`tk get` default output is concise (`id/type`, `title`, `description`, `a/c`) and suggests using `-v` for full details.

## Start work

```bash
tk claim -id <id>          # optional/self-assign
tk active -id <id>         # mark active in current stage
```

If coding is starting and the ticket is still in an earlier stage, advance it with lifecycle commands first.

## Progress and completion

```bash
tk comment add -id <id> "implementation notes"
tk complete -id <id>       # marks success and advances stage
tk fail -id <id>           # marks failed state
tk close -id <id>          # close when truly done
```

Do not leave finished work in `active`.

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
tk history <id>
tk dep add -id <id> <depends-on-id>
tk dep remove -id <id> <depends-on-id>
tk label ls
tk label add -id <ticket-id> <label-id>
tk time log -id <ticket-id> -m <minutes> -note "note"
```

## Project and setup

```bash
tk project ls
export TICKET_PROJECT=<id-or-prefix>
tk initdb
```
