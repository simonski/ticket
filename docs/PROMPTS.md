------------------------------------------------------------------
0. <freeform converted to DESIGN.md>

------------------------------------------------------------------
1. Write a USER_GUIDE.md at the top based on a hypothetical implementation of this using the docs/DESIGN.md.    

Do not include how to run it, only from the perspective of a user in the terminal using the software.

------------------------------------------------------------------
2. Refine the USER_GUIDE and docs/DESIGN so they are consistent and do not contradict each other.

------------------------------------------------------------------
3. Using the DESIGN and USER_GUIDE write an example breakdown of implementation requirements as REQUIREMENTS.md in the format:

EPIC: title
ID: E1, E2, E3 etc
DESCRIPTION: description
AC: list of acceptance criteria
PRIORITY: 1-N (1 highest, do this first)
DEPENDS-ON: E2, E4

<indent for stories "in" the epic (the story ID should increment and be EPIC-STORY)>
    STORY: title
    ID: E1-S1, E1-2, E1-S3 etc.
    DESCRIPTION: description
    AC: list of acceptance criteria
    PRIORITY: 1-N (1 highest, do this first)
    DEPENDS-ON: E1-S2

The intent is to take this output and model it in an issue tracker.  The scope is:
- ALL examples in the user guides
- ALL of the backend and frontend functionality as per the design

Note the DEPENDS-ON is a method of describing blocking features.

Ensure the acceptance critera contains
    - work in a branch that contains the EPIC and TASK name for example feature/<epic>-<task>

------------------------------------------------------------------
4. Write/rewrite a parser go program that translates a requirements.md into `ticket` commands (but do not call `ticket`). 

It should just be a single go file runnable as "parser -f REQUIREMENTS.md" which writes to stdout all the `ticket` commands with double- newlines between them.   It should read the whole requirements, validate they are correct and have referntial integrity where they refer to other EPICS or STORIES, call out the error-line if there is one, exit 1 if there is a problem, or just print the commands and exit 0.

Each entry acceptance criteria should include a reference to look at RULES.md, DESIGN.md, USER_GUIDE.md as additional context.

Put this in tools/parser.go and update e the Makefile to have a `make tools` which builds a parser binary in the root

------------------------------------------------------------------
5. Work on the REQUIREMENTS in order.

------------------------------------------------------------------

`-json` in client calls will pretty-print JSON as the response.

`ticket create|new|add I am a new task` should create a new task called "I am a new task"
Note: new,create,add are the same
Note: list,ls are the same
Note: rm,delete,del are the same

-title TITLE is the same as not specifying the title
-ac ACCEPTANCE_CRITERIA

If -t[ype] is unspecified, default to a task
If -p[riority] is unspecified, default to 1
If -a[ssignee] is unspecified, leave blank
If -d[escription] is unspecified, leave blank
If -parent is unspecifed, leave blank
If -project is unspecifed, use the current project

`ticket get <id>` should pretty print the entity by major headings.

An entity is deemed `orphaned` if it does not have a parent_id.  Orphans can be found with

`ticket orphans`

If a task is created, print to stdout the task id
If any client command fails, exit 1
If any client command succeeds, exit 0

`ticket count` should print the total number of everything by type
    users
    tasks 123 (50 completed, 75 in progress, 110)
    epics 10 (5 completed)
    projects (5)

`ticket count -project_id` should print the total number of everything by type for a given project
    users
    tasks 123 (50 completed, 75 in progress, 110)
    epics 10 (5 completed)

`ticket status` should print the effective configuration first, then perform the documented remote/local connectivity check.

`ticket assign <id> <name>` is an admin only command that assigns a task ID to a user
`ticket unassign <id> <name>` is an admin only command that un-assigns a task ID to a user

`ticket claim <id>` assigns the caller to the task.  If another user is assigned, fail.  A user cannot override an assignment.
`ticket unclaim <id>` un-assigns the caller to the task.  If the user is not assigned, fail.   A user cannot override an asssigment.

`ticket ls,list -u[ser] <name>` lists all tasks assigned to the user

`ticket server` : below the "rainbow" task in the USAGE print the VERSION
`ticket server` : below the VERSION print the taskdb location.


ticket list 
    should be much nicer - print in a table perhaps?
    should incldue the assignee
    -n should limit number of responses on the server side (default 0 meaning all)



A task is worked on by one worker (the assignee)
A task can be in 3 stages [design, develop, test]
    - design [idle,inprogress,review,complete]
    - develop [idle,inprogress,review,complete]
    - test [idle,inprogress,review,complete]
OR
A task can be in 3 states: idle,inprogress,complete
A task can have two outcomes: success, failure
A task can be closed/archived/deleted to remove it from visibility

If a task has children, it cannot be complete unlesss all children are complete.

ticket state change commands
    task open 1             - moves state to open
    task close 1            - moves status to closed
    task ready 1            - moves ready state to true
    task unready 1          - moves ready state to false
    task fail 1             - moves state to failed
    task success 1          - moves state to success
    task active 1           - moves status to active
    task idle 1              - moves state to idle
    task inprogress 1        - moves state to inprogress
    task complete 1          - moves state to complete

`ticket onboard` should append an `${CWD}/AGENTS.md` file which is embeddedin the go code under cmd/ticket/AGENTS.md

group the CLI usage by admin commands and client commands
order the CLI commands alphabetically in their section

Ensure the CLI usage is up to date.
Update the code, DESIGN and USER_GUIDE for the above.  



`ticket get N` should return in format
ID           :
ParentID     :
ProjectID    :
Type         : task
Description  :
Title        :
Assignee     :
Order        :
DependsOn    : [1,2,3]
Status       :
Priority     :    
Created      :
LastModified :
Closed       :
Acceptance Criteria :

`ticket history N` should print the history.


Create the `add-dependency` `remove-dependency` commands.

If a task 4 depends on 3 other tasks (1, 2, 3) completing 

ticket dependency add 4 1,2,3

Now 4 depends-on 1,2,3.

LEt's task 4 does not depend on task 2

ticket dependency remove 4 2

note, the comma-separated ability for the tasks.

Ensure the CLI usage is up to date.
Update the code, DESIGN and USER_GUIDE for the above.  



remove slug from projects everywhere, cli, model, database.   

to add acceptance criteria
ticket project N update -ac "the acceptance criteria"

to update title or description
ticket project N update -title "the new title"
ticket project N update -description "the new description"

to add acceptance criteria
ticket project N update -ac "the acceptance criteria"

also make it an option when creating projects

## project status
ticket project N enable
ticket project N disable




## New instruction ticket

`ticket ticket -f file1,file2,file3 -o requirements.md` should read all files mentioned in -f and write to the -o filename the results of the prompt to an agent.  The agent should be prompted via a process invocation that receives the entire prompt.  

The invocation should be wired to print the STDOUT as well as to the file.

The agent should default to codex however can be overridden using `-agent` in which case e.g. a call to copilot coudl occurr using `copilot -p PROMPT`

PROMPT:
-------

Write an example breakdown of implementation requirements as $OUTPUT_FILE in the format:

EPIC: title
ID: E1, E2, E3 etc
DESCRIPTION: description
AC: list of acceptance criteria
PRIORITY: 1-N (1 highest, do this first)
DEPENDS-ON: E2, E4

<indent for stories "in" the epic (the story ID should increment and be EPIC-STORY)>
    STORY: title
    ID: E1-S1, E1-2, E1-S3 etc.
    DESCRIPTION: description
    AC: list of acceptance criteria
    PRIORITY: 1-N (1 highest, do this first)
    DEPENDS-ON: E1-S2
----

------------------------------------------------------------------

test and implement as server side checks
- a ticket must be assigned to the user in order to modify the status or return 403.

- a closed ticket cannot be reopened

- a ticket can be cloned/copied using `ticket cp,clone`.  Update the clone ticket to have a clone_of key/value.   A clone should be set to status=notready and unassisnged.

- an epic can be cloned/copied using `ticket cp,clone`.  All sub-tickets are the cloned also.  

------------------------------------------------------------------

MODE: REMOTE or LOCAL

The ticket process can work in REMOTE (TICKET_MODE=remote) or LOCAL (TICKET_MODE=local).  This is set using 

```bash
# either
export TICKET_MODE=local
# or
export TICKET_MODE=remote
```

If unspecified TICKET_MODE will default to local.

REMOTE-mode

Uses TICKET_HOME for local files (~/.config/ticket/)

- Requires TICKET_SERVER to be set to the address of the remote server.  If it is not present, fail.
- Requires a valid session token for all comms (except login/register)
- `ticket login` will store the session token in $TICKET_HOME/credentials.json
- If the user supplied the username via the login prompt directly, the username will be stored in `$TICKET_HOME/config.json` to be used on next login as the default.

TICKET_USERNAME/TICKET_PASSWORD are only used in REMOTE mode when logging in; If present they are used to authenticate via login and then a session token is used after that.  If they are not present the user is prompted for their username/password.

If a user is not authenticated
    - fail
    - instruct user to run `ticket login`
    
`ticket status` in remote mode:
    - prints the current effective configuration first
    - prints:
         mode: remote
         server: <TICKET_SERVER>
         username: <configured username or blank>
         authenticated: true|false
    - attempts a remote connection by calling the remote status endpoint
    - prints:
         connection: success   (green)
         connection: failure   (red)
    - if `-nocolor` is set, print the same output without ANSI colors

LOCAL-mode

In Local mode TICKET_SERVER, TICKET_USERNAME, TICKET_PASSWORD are ignored.

It will then select a database file using the following logic

    1. if -f <task_db_file> is specified in any command, chooose this
    2. if TICKET_HOME is specified, choose this and assume `$TICKET_HOME/ticket.db`
    3. fallback to a `$CWD/ticket.db` file

TICKET_USERNAME and TICKET_PASSWORD are NOT used in local mode.  The username is $USERNAME of the computer.

`ticket status` in local mode:
    - prints the current effective configuration first
    - prints:
         mode: local
         db_path: <resolved database path>
         db_exists: true|false
    - if the database exists, opens it and verifies the schema is usable
    - a usable schema means the required application tables exist and can be queried
    - prints:
         connection: success   (green)
         connection: failure   (red)
    - if the database does not exist, print:
         hint: run ticket initdb
    - if `-nocolor` is set, print the same output without ANSI colors

------------------------------------------------------------------

REFACTOR: LOCAL AND REMOTE CLIENT LIBRARIES

Refactor the task code so that the CLI does not directly decide between store calls and HTTP calls throughout the command handlers.

Create two libraries with the same task-domain service contract:

`libticket`
    - defines the service interface used by the CLI
    - provides the LOCAL implementation backed by SQLite/store
    - owns local-mode behavior, including DB path resolution and local user resolution

`libtickethttp`
    - provides the REMOTE implementation of the same service interface
    - talks to the HTTP API described by the OpenAPI spec
    - should not expose raw HTTP details to the CLI

Dependency direction:

    cmd/ticket      -> chooses libticket or libtickethttp based on TICKET_MODE
    libtickethttp   -> calls HTTP endpoints only
    internal/server -> uses libticket service implementation internally
    libticket       -> uses store/database

Do not define the interface around raw tables or CRUD helpers.  Define it around task-domain operations the CLI actually needs, for example:

    Status
    Login / Logout / Register
    Count
    ListProjects / GetProject / CreateProject / UpdateProject / SetProjectEnabled
    ListTasks / GetTask / CreateTask / UpdateTask / CloneTask / RequestTask
    ListDependencies / AddDependency / RemoveDependency
    ListHistory / AddComment / ListComments
    ListUsers / CreateUser / DeleteUser / SetUserEnabled

Testing requirements:

    - Create a comprehensive contract test suite for the shared service interface.
    - Run the same red/green service tests against:
        1. libticket (local SQLite-backed implementation)
        2. libtickethttp (HTTP-backed implementation)
    - Keep transport-specific tests for HTTP request/response handling in libtickethttp.
    - Keep storage/schema edge-case tests in store/libticket.

Acceptance criteria:

    - CLI command handlers depend on the shared service interface, not on HTTP/store branching.
    - LOCAL mode uses libticket.
    - REMOTE mode uses libtickethttp.
    - Existing CLI behavior remains the same in both modes.
    - `go test ./...` passes with comprehensive coverage for both implementations.

------------------------------------------------------------------

CONFIGURATION

Configuration key/pairs can be set using a config file.  
    - local `.ticket-config.toml` file 
    - user-wide $TICKET_HOME/ticket-config.toml
    
Configuration can be set

ticket config set key value -scope local,global
ticket config rm key value -scope local,global
ticket config ls,list [-scope local,global]

local = $CWD/ticket-config.json
global = $TICKET_HOME/ticket-config.json

Configuration keys

# the default CLI output mode if not specified (default)
output.format=json,markdown (markdown)

# the default CLI output mode if not specified (default)
output.format=json,markdown (markdown)

# the default CLI output mode if not specified (default)
ticket.file=$TICKET_HOME/ticket.db

----

I want to think about remodelling how to use tickets in this system.
Once we get to a solid design, I then want to refactor it all - documentation, CLI, tests, server, model, backend, database, to reflect this.

Reason about the following and come back with your proposal.

Overall goal: a ticket management system for software engineering.

A ticket is a piece of work to be done.  It can be one of:
    epic, task, bug.

An epic can contain epics, tasks, bugs.  A task can have tasks and bugs.  

"have" means it can be a parent_id of another ticket.

A ticket is in a given "stage" to represent the high level "swimlane" of its progress.   
    
    design      - the ticket is being appraised and refined
    develop     - the ticket has been design and is now being worked on
    test        - the ticket outcome is verified and appraised
    done        - the ticket is concluded as complete

A ticket in a stage is then in a given "state"
    idle: ready but not currently in progress
    active: currently being worked on with a named assignee
    complete: work for the current stage is complete

design: idle, active, complete
develop: idle, active, complete
test: idle, active, complete
done: complete

When a ticket moves to an active state, all parent tickets are marked as active.  

The stage of an epic is set as the earliest stage of any descendant.

Status of a ticket is the composite of stage/state = design/idle
    
So a ticket is moved between stages by setting the stage

ticket create ...
    stage = design
    state = idle
    return N (ticket id)

ticket design N
    stage = design
    state = idle

ticket develop N
    state = idle
    stage = develop

ticket test N
    state = idle
    stage = test

ticket done N
    stage = done
    state = complete

ticket idle N
    state = idle

ticket active N
    state = active

status is not stored independently. It is rendered as stage/state, for example design/idle.

If a ticket has children, its effective stage/state is derived only.

state=active requires assignee != ""
state=idle should probably allow unassigned
state=complete may keep or clear assignee; I recommend keep it for audit/history
stage=done requires state=complete
stage!=done allows idle | active | complete

allow explicit stage/state changes only on leaf tickets
parent tickets recalculate from descendants

Derived Parent Stage

For any parent ticket:

effective stage = earliest stage of any descendant
Ordering:

design < develop < test < done
This is good and should apply to all parent tickets, not only epics.

parent is complete if all descendants are complete
parent is active if any descendant is active
otherwise parent is idle


Behavior:

stage commands mutate leaf tickets only
state commands mutate leaf tickets only
parent tickets reject direct stage/state edits if they have children
ticket get and ticket list show effective stage/state/status
Optional nicety:

if user tries to change a parent directly, return:
ticket has children; stage/state is derived from descendants
Database Proposal

Replace old single-status model with:

stage TEXT NOT NULL
state TEXT NOT NULL

------

Entities
    project
        prefix: 3 letter string, unique
        ID <uuid>
        title: string, unique
        description: string
        created: datetime
        status: open/closed
        ac: string
        notes: 

    ticket  
        project_id
        ID : project_prefix-shortuuid
        type
        title
        description
        ac
        stage
        state
        created
        user
        history

A project has a prefix.
IDs for tickets in a project are in the format {PREFIX}-{UUID}

The UUID is to be in the format <PREFIX>-<TYPE>-<ULID>
    e.g. cus-p-01J3FQ3H7S9G9K2M7NQ0D2Y7XG

where the type is the first letter of the ticket type d,e,t,b,s
    d[esign]
    e[epic]
    t[task]
    b[big]
    s[spike]
    c[chore]

What's the shortest UUID we can use to combine
    no collisions
    rememberable, easy on the eye (e.g 5 characters)

The commands to alter tickets should be

ticket <command> (-t[ype] epic,task,bug)

so, where N is 

ticket ls,list -t project,epic,task,bug
ticket get,show -id ID
ticket update -id ID
ticket rm,delete -id ID
ticket ls,list -t project,epic,task,bug

ticket create -t[type] project,epic,task,bug 
    # project only
    -prefix (project only, mandatory)

    # all tickets
    -parent_id
    -title
    -description
    -priority
    -type
    -ac

# set the parent of ID to be PARENT_ID
ticket attach -id ID -parent_id PARENT_ID

# remove the ID from the parent (orphan it in the project)
ticket detach -id ID -parent_id PARENT_ID

# assign a named user to work on it
ticket assign -id ID -username X

# assign a named user to work on it
ticket unassign -id ID -username X

# as a user, try to claim a specific ticket (server will assign if valid)
ticket claim -id ID

# as a user, try to claim any ticket (server will assign if valid)
# if a story is already assigned, it is returned
ticket claim

# as a user, simulate a claim but do not assign on the server
ticket claim -dryrun



all entities can have comments
    (id, user, date, comment)

I think thsese live in a central comment db that goes
    comment_id, entity_id, date, user, comment

And that is the method that comments can be referred to for given entities

# add a comand onto an entity
ticket comment add,create,new -id N -comment "I am the comment for the thing"

# list comments for an entity
ticket comment ls -id N 

# list comments by author
ticket comment ls -user N

# list comments by author for a project
ticket comment ls -user N -project_id N

# update comment for an entity
ticket comment update -id N -comment_id X -comment "blah blah"

# delete comment for an entity (cannot delete other users comments)
ticket comment update -id N -comment_id X

