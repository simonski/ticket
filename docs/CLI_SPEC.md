# CLI Specification — Noun-Verb Reorganization

**Requirement:** TK-34
**Status:** Implemented

## Principles

1. **`tk <noun> <verb> [-id <id>] [flags]`** — every command follows this pattern
2. **12 nouns** — `ticket`, `req`, `dep`, `label`, `time`, `project`, `role`, `workflow`, `decision`, `team`, `agent`, `user`
3. **Top-level shortcuts** for the highest-frequency actions
4. **Hidden aliases** for all old command forms during migration

## Top-level shortcuts

```bash
tk                              # alias for: tk ticket list
tk add "title"                  # alias for: tk ticket add "title"
tk bug "title"                  # alias for: tk ticket add -type bug "title"
tk epic "title"                 # alias for: tk ticket add -type epic "title"
tk idea "title"                 # alias for: tk req add "title"
tk ideas                        # alias for: tk req list
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
tk ticket state -id <id> <state>        # set state directly

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
tk req break -id <id>                   # show breakdown (child tickets)
tk req break -id <id> --retry           # regenerate, keep pinned
tk req break -id <id> --reset           # discard all children, regenerate
tk req pin -id <id>                     # pin a breakdown item (not yet implemented)
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
tk project create -title "Name" -prefix "PRJ"
tk project get <id>
tk project use <id>
tk project init
tk project add-user -id <project-id> -user_id <user-id>
tk project remove-user -id <project-id> -user_id <user-id>
tk project add-team -id <project-id> -team_id <team-id>
tk project remove-team -id <project-id> -team_id <team-id>
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
tk workflow list                                    # list all workflows
tk workflow create -name <n> [-d desc]              # create a workflow
tk workflow get -id <id>                            # show workflow details
tk workflow delete -id <id>                         # delete a workflow
tk workflow add-stage -id <wf-id> -name <n>         # add a stage
tk workflow remove-stage -stage-id <id>             # remove a stage
tk workflow reorder-stages -id <wf-id> <ids>        # reorder stages
tk workflow export -id <id> [-o file]               # export to JSON
tk workflow import -file <file>                     # import from JSON
```

## decision

```bash
tk decision add "Use Postgres for production"
tk decision list
```

## story

```bash
tk story create -title "Story title" [-d desc]     # create a story
tk story list                                       # list stories in project
tk story get <id>                                   # show story detail
tk story update <id> -title "New title"             # update a story
tk story delete <id>                                # delete a story
```

## team

```bash
tk team list                                        # list all teams
tk team create -name "Platform"                     # create a team
tk team update -id <id> -name "New Name"            # update a team
tk team delete -id <id>                             # delete a team
tk team add-user -team_id <id> -user_id <id> -role <member|owner>
tk team remove-user -team_id <id> -user_id <id>
tk team users -team_id <id>                         # list team members
tk team add-agent -team_id <id> -agent_id <id>
tk team remove-agent -team_id <id> -agent_id <id>
tk team agents -team_id <id>                        # list team agents
```

## agent

```bash
tk agent list                                       # list all agents
tk agent create -name <n> -description <d>          # create an agent
tk agent update -id <id> [-name n] [-desc d]        # update an agent
tk agent delete -id <id>                            # delete an agent
tk agent enable -id <id>                            # enable an agent
tk agent disable -id <id>                           # disable an agent
tk agent run -name <n> -password <p> [-url u]       # run agent worker loop
tk agent request -name <n> -password <p>            # request work envelope
```

## user

```bash
tk user list                                        # list all users (admin)
tk user create --username <u> --password <p>        # create a user (admin)
tk user delete -id <id>                             # delete a user (admin)
tk user enable -id <id>                             # enable a user (admin)
tk user disable -id <id>                            # disable a user (admin)
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
tk init
tk export
tk import
tk upgrade
tk onboard
tk health
```

## Noun summary

| Noun       | What it covers                                              |
|------------|-------------------------------------------------------------|
| `ticket`   | All ticket CRUD, state, ownership, hierarchy, comments      |
| `req`      | Requirements lifecycle — capture, shape, break down, review |
| `dep`      | Dependencies between tickets                                |
| `label`    | Label management and tagging                                |
| `time`     | Time tracking                                               |
| `project`  | Project CRUD, switching, and membership                     |
| `role`     | Role definitions                                            |
| `workflow` | Workflow definitions, stages, import/export                 |
| `decision` | Decision records                                            |
| `story`    | Story CRUD within a project                                 |
| `team`     | Team hierarchy and membership (users + agents)              |
| `agent`    | Autonomous agent management and worker loops                |
| `user`     | Admin-only user management                                  |

## Migration

- All old command forms (`tk complete -id X`, `tk curate X`, etc.) remain as hidden aliases
- Hidden aliases are not shown in help output
- Old forms continue to work indefinitely — no breaking changes
- Help text and documentation point to the new noun-verb forms only
- Stage commands (`design`, `develop`, `test`, `done`) have been removed — use `tk ticket state` instead
