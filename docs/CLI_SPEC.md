# CLI Specification — Noun-Verb Reorganization

**Requirement:** TK-34
**Status:** Design

## Principles

1. **`tk <noun> <verb> [-id <id>] [flags]`** — every command follows this pattern
2. **9 nouns** — `ticket`, `req`, `dep`, `label`, `time`, `project`, `role`, `workflow`, `decision`
3. **Top-level shortcuts** for the highest-frequency actions
4. **Hidden aliases** for all old command forms during migration

## Top-level shortcuts

```bash
tk                              # alias for: tk ticket list
tk add "title"                  # alias for: tk ticket add "title"
tk bug "title"                  # alias for: tk ticket add -type bug "title"
tk epic "title"                 # alias for: tk ticket add -type epic "title"
tk idea "title"                 # alias for: tk req add "title"
```

## ticket

```bash
# List & search
tk ticket list                          # list tickets
tk ticket list --type bug               # filtered by type
tk ticket list --status develop/active  # filtered by status
tk ticket list -u alice                 # filtered by assignee
tk ticket search "query"                # full-text search
tk ticket board                         # kanban view
tk ticket count                         # aggregate counts
tk ticket orphans                       # tickets with no parent

# Create
tk ticket add "title"                   # create a task
tk ticket add -type bug "title"         # create with type
tk ticket add -type epic "title" -d "description" -ac "criteria"

# View
tk ticket get -id <id>                  # show detail
tk ticket get -id <id> -json            # JSON output
tk ticket tree -id <id>                 # show hierarchy

# Update
tk ticket update -id <id> -title "new title"
tk ticket update -id <id> -d "description"
tk ticket update -id <id> -ac "acceptance criteria"
tk ticket update -id <id> -priority 1

# State
tk ticket active -id <id>               # start work
tk ticket idle -id <id>                 # pause
tk ticket complete -id <id>             # finish stage, advance
tk ticket fail -id <id>                 # mark failed

# Ownership
tk ticket claim -id <id>                # assign to self
tk ticket unclaim -id <id>              # unassign self
tk ticket assign -id <id> <user>        # assign to someone
tk ticket unassign -id <id> <user>      # unassign someone
tk ticket request                       # next available ticket

# Hierarchy
tk ticket attach -id <id> <parent-id>   # set parent
tk ticket detach -id <id>               # remove parent

# Comments & history
tk ticket comment -id <id> "text"       # add comment
tk ticket history -id <id>              # activity log
tk ticket conversation -id <id>         # full thread

# Lifecycle
tk ticket close -id <id>
tk ticket open -id <id>
tk ticket archive -id <id>
tk ticket unarchive -id <id>
tk ticket clone -id <id>
tk ticket delete -id <id>
```

## req

```bash
tk req add "offline mode"               # capture an idea
tk req add "dark mode" -d "details"     # with description
tk req list                             # all requirements
tk req list -status raw                 # by review status
tk req get -id <id>                     # view detail
tk req shape -id <id> -d "more detail"  # refine
tk req break -id <id>                   # generate epics/stories
tk req break -id <id> --retry           # regenerate, keep pinned
tk req break -id <id> --reset           # discard all, regenerate
tk req pin -id <id>                     # pin a breakdown item
tk req accept -id <id>                  # approve
tk req reject -id <id>                  # reject
tk req revise -id <id>                  # send back for rethinking

# Shorthand
tk idea "offline mode"                  # alias for tk req add
tk ideas                                # alias for tk req list
```

## dep

```bash
tk dep add -id <id> <depends-on-id>     # add dependency
tk dep remove -id <id> <depends-on-id>  # remove dependency
```

## label

```bash
# Project-wide
tk label list
tk label create -name "bug" -color "red"
tk label delete -id <id>

# Per-ticket
tk label add -id <ticket-id> <label-id>
tk label remove -id <ticket-id> <label-id>
tk label show -id <ticket-id>
```

## time

```bash
tk time log -id <ticket-id> -m 30 -note "morning session"
tk time list -id <ticket-id>
tk time total -id <ticket-id>
tk time delete -id <entry-id>
```

## project

```bash
tk project list
tk project create -title "Name"
tk project use <id>
tk project get -id <id>
tk project init
```

## role

```bash
tk role list
tk role create -title "Security Lead" -motivation "..." -goals "..."
tk role update -id <id> -title "New Title"
tk role delete -id <id>
```

## workflow

```bash
tk workflow get -id <id>
```

## decision

```bash
tk decision add "Use Postgres for production"
tk decision list
```

## System commands

```bash
tk status
tk version
tk server
tk login
tk logout
tk register
tk config
```

## Noun summary

| Noun       | What it covers                                              |
|------------|-------------------------------------------------------------|
| `ticket`   | All ticket CRUD, state, ownership, hierarchy, comments      |
| `req`      | Requirements lifecycle — capture, shape, break down, review |
| `dep`      | Dependencies between tickets                                |
| `label`    | Label management and tagging                                |
| `time`     | Time tracking                                               |
| `project`  | Project CRUD and switching                                  |
| `role`     | Role definitions                                            |
| `workflow` | Workflow inspection                                         |
| `decision` | Decision records                                            |

## Migration

- All old command forms (`tk complete -id X`, `tk curate X`, etc.) remain as hidden aliases
- Hidden aliases are not shown in help output
- Old forms continue to work indefinitely — no breaking changes
- Help text and documentation point to the new noun-verb forms only
