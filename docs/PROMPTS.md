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

`ticket onboard` should print the embedded `cmd/ticket/TICKETS.md` template to stdout

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

The mode is inferred from the `TICKET_URL` scheme:

```bash
# local mode (file scheme or default)
export TICKET_URL=file:///path/to/ticket.db
# remote mode (http/https scheme)
export TICKET_URL=http://localhost:8080
export TICKET_URL=https://your-server
```

If unspecified, `TICKET_URL` defaults to local mode with `~/.config/ticket/ticket.db`.

REMOTE-mode

Uses TICKET_CONFIG_DIR for local files (~/.config/ticket/)

- Requires `TICKET_URL` to be set to an `http(s)://` address.  If it is not present, fail.
- Requires a valid session token for all comms (except login/register)
- `ticket login` will store the session token in $TICKET_CONFIG_DIR/credentials.json
- If the user supplied the username via the login prompt directly, the username will be stored in `$TICKET_CONFIG_DIR/config.json` to be used on next login as the default.

TICKET_USERNAME/TICKET_PASSWORD are only used in REMOTE mode when logging in; If present they are used to authenticate via login and then a session token is used after that.  If they are not present the user is prompted for their username/password.

If a user is not authenticated
    - fail
    - instruct user to run `ticket login`
    
`ticket status` in remote mode:
    - prints the current effective configuration first
    - prints:
         mode: remote
         server: <TICKET_URL>
         username: <configured username or blank>
         authenticated: true|false
    - attempts a remote connection by calling the remote status endpoint
    - prints:
         connection: success   (green)
         connection: failure   (red)
    - if `-nocolor` is set, print the same output without ANSI colors

LOCAL-mode

In Local mode TICKET_USERNAME, TICKET_PASSWORD are ignored.

It will then select a database file using the following logic

    1. if -f <task_db_file> is specified in any command, choose this
    2. if TICKET_URL is set with a `file:///` scheme, use that path
    3. fallback to `~/.config/ticket/ticket.db`

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
         hint: run ticket init
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

    cmd/ticket      -> chooses libticket or libtickethttp based on TICKET_URL scheme
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
    - user-wide $TICKET_CONFIG_DIR/ticket-config.toml
    
Configuration can be set

ticket config set key value -scope local,global
ticket config rm key value -scope local,global
ticket config ls,list [-scope local,global]

local = $CWD/ticket-config.json
global = $TICKET_CONFIG_DIR/ticket-config.json

Configuration keys

# the default CLI output mode if not specified (default)
output.format=json,markdown (markdown)

# the default CLI output mode if not specified (default)
output.format=json,markdown (markdown)

# the default CLI output mode if not specified (default)
ticket.file=$TICKET_CONFIG_DIR/ticket.db

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


## COMMENT SYSTEM

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

## 

rename the `ticket initdb` command to `ticket init`

## Start using ticket as the method of work

Store the ticket DB in the repo
Start execing using the ticket db.
The workflow can be external for now bu tthe souce of truth shoudl be the ticketdb.


##

add a close/open commmand

tk close -id tk-1

A closed ticket is effectively frozen in its state/stage.
A closed ticket is visible but cannot be modified except for deletion or re-opening.

tk open -id tk-1

An open ticket can be modified, have its stage/state modified.

Opening or closing a ticket goes into the ticket history.

Update the tk get and tk ls calls to reflect if aticket is open/closed


##

new branch feature/ticket_board

ticket server (-p 9999)

should run a simple website with user administration/login, a drop-down to select projects and a trello-like board to view tickets.

minimal js, css html, attempt a simple single webpage application with zero 3rd party libraries.  embed the files via go:embed and serve them direct.  Use a websocket to communicate back to tehserver so that updates to tickets dispaly in realtime across all users.

the board swimlanes should be the stage, the tickets shoudl be displayed as paper tickets with a horizontal coloured line indicating their state

a user should be able to click ona. ticket to inspect the details
they should be able to drag/drop tivkets between swimlanes

they should be able to CRUD tickets and Projects

localStorage should "remember" the selected project


## 
commands should permit/deny registration ability via the website

## enable registrsation via/users
ticket config registration-enable

## enable registrsation via/users
ticket config registration-disable

If the server config has registration disabled, the UX shoudl not show the register feature.  If it has it enable, it shoudl show the register feature.

## add user to a project as a specific role
# roles viewer - can read tickets and see project but not make any changes
#       owner  - can CRUD tickets and users
#       editor - can CRUD tickets, cannot CRUD users
ticket project add-user -user_id X -project_id X -role [viewer,editor,owner]

## add user to a project as editor
ticket project remove-user -user_id X -project_id X 

## UX

landing page should be a login page unless the cookie indicates the user already has a session.   landing page should show a starfield and a login banner with a register link (if registration is permitted)

once logged in, whole screen should be the stages as swimlanes, top-right should be a proejct selector.  bottom let should have a + which shows a popup to create a ticket

pressing N should act as a popup "new" to create a ticket

clicking on an existing ticket shoudl show the same popup populated as an edit mode.

pressing ESC or clicking outside he popup shuld dismiss

no save button - changes should happen as yuo type them

the login screen should focus on the username
the login fails: Server unavailable: project not found
the login fails on an empty new database: "Server unavailable: Cannot read properties "of null (reading 'filter')
## conway

conway:

make a new directory, /conway which is a fullscreen threejs javascript application which renders an 8-bit pixel conways game of life in fast-forward.  Make it accelerate over 10 seconds until the pixels slowly start to accellerate into a ring and start orbiting clockwise.   once the pixels are roating, make their radius vary over time using some perlin noise but keep within the bounds so it retains the shape.  make some pixesl have differing velocities, again using perlin noise for difference.

make it render in the entire page, not a square.  start with a random arrangement of pixels aroud the screen and remove hte conways simulation but retian the pixels.  

gradually start one, then two, then four, then eight, then more over time to start meandering in a clockwise until they start to organise to a circle as previously described.   Make them behave as if they want to avoid the mouse pointer.   Occasionally send them off in a boids simulation then come back to circling again.

if the user presses the space bar, have the pixels gradually move to form the word hello, where some pixels go to the h, some to the e etc.  Where they "circle around" the shape of the letter in a clockwise manner.  once all the pixels are in their route for a given letter, stay i that animation for a half a second then start to peel off the pixels again one, two, eight, until all are back in a circle patttern.   

the word should vary each time using multiligual hello



----

the dialogs need work for 
    new type
    stage/state needs work for moving
    ability to create porjects eneeds work
    the time between typing and saving needs some work

    config register-enable/disable

    epic and then storie inside epics
    + and - for details

top banner contents

the top-right should have a profile circular icon for the logged in user
    click it to reveal dropdown containig
    settings
    profile
    logout
- remove the logotu button from the top banner

## logo

the left hand side should have an 8-bit 8x8 per character "ticket/tkt/tket" variations that slowly changes the hue/limunescence of the pixels.  use a threej3 rectangle and have it paint slowly over time

make the logo characters 50% larger and do not "switch" visually between words - transition the pixels until they are activated fully to their target colour.  in this way the word will morph appealingly from one to the other.  use perlin noise for the time and difference in colour

## status animator

to the right and for hte majority of the top banner (between the logo and the project selctor, make a threejs full-width full-height that is the current colour but have 8-bit pixels move from left to rigth or right to left to indicate activity in the system.  

the websocket will provide the activity
a pixel will be created and move from left ot right wiht a given perlin variable velocity and colour, where the colour is taken from a pool based on the classification of the event
    tickets being edited for their contents
    new tickets created
    tickets changing status
    tickets moving to done
    bug activity (more red)

activity in the status animator should tend to colourful for human interaction and grayscale for agentic interaction.  This is determined by the user "type" (human or agent).


the T character in the pixels shoudl not have pixels lit up in the bottom left-right but it does.

the transition from word1 to word2 should be that - a gradual transition e.g TICKET -> TKT means the I should morph into the K, the C into the T, the KET should fade back to teh background.  

make the logo banner rendering appear in the login and registration page in place of the "ticket" word.   on the login/register page dont use a websocket.

login page: remove the "logged out" and "please log in" messages


## project dropdown

the dropdown should not be a native browser dropdown - it should be html/css/js

project dropdown should have a new project selector in the following

[Curent Project]
[New Project]
------- <divider ------->
List of projects, most recently opened at top not including [Current]
Then sorted alphabetically

the new project dialog should not be native - it should be thematically similar to teh new/edit ticket dialog


## server logging

what is a common go server logging style for web services that include
    rotation
    http/websocket logging

to enable analyics/insight/operational telemetry?

server logging
    -q means stdout is quiet
    -l means write to file
    -v means be verbose to STDOUT as well as file (if file specified)

all errors write to 

tk server -l <file> 
    -l specifies the logfile to write to
    it should write
    date/time response-code duration-ms user-id url


v. quickly get to the ticket capture now via the browser and have a simulator workflow


## UX Review

page1: landing page (login, register)

page2: main page

top banner: always visible, contains logo, animator, project selector, user icon selector

main page: this can change depending on the "view" a user selects
pressing V should bring up a popup similar to the double-shift which allows the user to select a different "perspective"
    "swimlanes"
    "tv : ticketvision"

switching perspectives shob001udl fade out then fade in the other perspective.

swimlanes: the existing swim lanes view of stories for current project

tv: Let's create a new perspective

threejs alternate view which is like a two forced layout graph representation of a given project that renders by defalt left to right

project -> epics -> story
                  -> story
                -> Story



------

New branch, "feature/agent"

Create a new entity in the database, "agent" which represents a process which in turn will invoke an LLM to perform a task.

Create crud tools over API with CLI calls comparable to user registration but for agents.

# Example commands
```bash
ticket agent create -name X -description Y (-password PASSWORD)
# (password set to random on server-side if not supplied
>return ID, password 

ticket agent ls,list
> return ID, name, description, status

ticket agent udpate -id ID (-name <name> -desc[ription] <description> -password <password>)

ticket agent rm,delete -id ID

ticket agent enable -id ID
ticket agent disable -id ID
```

Create a new panel in the GUI to manage agents similar in theme to tickets.  Name, description, enable/disable.


------

Agent lifecyle

An agent is run with the command

```bash
ticket agent run -name <name> -password PASSWORD -url TICKET_URL
```

If the options are provided, they are used, else

AGENT_NAME=
AGENT_PASSWORD=
TICKET_URL=

If any are missing, the process will fail exit 1 explaining what is missing.

If all are present, the agnet will attempt to REGISTER - meaning declare that it is alive.  A success response from the server will move the agent into solitication mode where it asks for work to be assigned via a REQUEST call.

The REQUEST call ask the server to return and/or assign and return a ticket to be worked on.  It is up to the server to decide to assign or refuse to assign.  

If a ticket is assigned to the agent, the server will return the ticket details.  Agent will then delegate the ticket to an LLM via processing.   At the same tiem AGENT will then move the ticket to an active state.

Once the LLM completes, AGENT will call UPDATE on the ticket and pass back the results of the LLM.

------

UX enhancements

in the swimlane view, pressing P should bring up the edit project dialog
add git repository and branch to the project 

in the edit ticket view add git repository and branch to the edit dialog, store them on the ticket

add acceptance criteria to the edit dialogs for projects and tickets

-----

fetching tickets for work as an agent

The ticket an agent recieves will contain more than just the ticket details in the format:

```bash
ticket agent request (-name XXX -password YYY -url XXXX)
{
    status: "NEW,NONE,CURRENT",
    # return the prject details
    project: {}
    # the actual ticekt details
    ticket: {}
    # and parents, in order until it gets to the root
    parents: []
}

The response "status" NEW,NONE,CURRENT indicates
    NEW: a new ticket has been assigned based on this request
    NONE: there is no work for this agent
    CURRENT: the following work is the currently assigned ticket

normally when a ticket is returned via the `request` call, it will be assigned to the agent.  If the `-dryrun` option is specified, a ticket will be returned (randomly or -id) that simulates the response without assigning.  This is NOT to be worked on, only to demonstrate the JSON.

-----

roles

Create a new entity, "Role" which is a persona that an agent will be given when working on a ticket.
A role for example is one of the classical software engineering roles
    Product Owner
    Architect
    DevOps
    QA/Tester
    BA
    Lead Engineer
    Staff Engineer

They have a title, motivation and goals.

Create 
    - the API to CRUD-manage these entities
    - the UX in the website to manage them (via the R keypress for roles)
    - populate some classic roles by default such as the above


----

create a panel that can slide in/out on the left hand side to allow selection of
    board (swimlanes)
    agents
    roles
    settings


----

add a new section in the panel "chat" which is a chatbot experience 
    the user can engage in a conversation with an LLM which will on the backend invoke copilot via 
    a process exec running codex via an external process, mapping STDIN/STDOUT back via the websocket
    to the frontend.

    the chant experience requires a text area at the bottom and then the chat animates upwards as the
    conversaton contonues

embellish the role descriptioisn and motivation with classical descriptions of each role.  I want to see a number of paragraphs that are reasonable description, goals and motivations of each role.

----

Story

A new entity.  Many stories associated with one project.   A story can have multiple epics.

Stories should get their own panel entry and be associated with a single project.

'S' shoudl bring up a dialog to create a new Story type.  The Story is a high level requirement that is not a ticket itself rather it is the entitiy that represents an overall Requirement.  Once ready, the story will then be broken down into epics and tasks -which will all link to the story_id.

The breakdown of a story to epics and tasks needs to happen using an LLM using a role - these epics and tickets are then stored in the ticket database itself.  At that point the story will be marked as ready for review.  

Ensure the stories paenl is on the V popup.

When a story popup is visible, show an "analytse" button which will breakdown the story using the StoryReview role (make a StoryReview role) into epics and tasks.

When an epic dialog is present, add an "analyse" button which willbreakdown the epic into tickets using the EpicReview role (make an EpicReview role).

----

Settings Panel
An "admin" role user should have visibility in the UX to a settings panel.  
The settings shoudl contain all teh switchable settings and config that affect all users.

Teams
-----
A team is a set of users which can be treated like a user itself.  So a team can be a member
of a project.

A team can have child teams so that you can compose a hierarchy.

A user has a role in a team
    member 
    owner
    A user in a team can then have a job title
    An agent can be assigned to a team.

Create the backend, API, CLI and website UX to support this.


User Roles
-----------------
A user has a role
    admin - can perform all tasks
    user  - user cannot administer agents or other users

A user may have a role in a project
    owner of a project - can perform all activities on a project
        can manage membership and roles in projects
    editor of aproject - can crud contents a project
        can manage tickets in a project
    viewer of a project - can view aspects of te project

A project can be
    private - a membership list is required to view/edit
    public  - all users in the system can view it

If a user does not have a role in a project and the project is private then the project is not visible
to the user.

An admin can see and manage all entities - but should not be logging in all the time -as it is risky.  When an admin logs ino the system, a soft glowing rounbd-rect border should disaply on the website.  The colour scheme should rotate slowly throught he rainbow ina neon like/chroma effect to indicate "danger mode"

If a 401 occurs, go back to the login page.

Website optimisation: The website is making lots of calls to the selected card.  Why? the websocket should transmit that something changed and that shoudl force the lookup to the card details.  But nothing has changed, yet there is traffic continually.  This can be optimised, so optimise it.
------------

read agents; review for drift, design, documentation, testing and implementation.


`ticket init`
    Add an optional `--populate` which 
        - creates three example projects each with Storis and associated epics, tasks, bugs, chores.
        - creates example users across 3 teams


websocket
---------

the purpose of the websocket is to send indicators that "something changed" - that means it shoudl contain the entiy type, id, and change type.  

Agent Roles
------------

projects and git

A project has a git repository associated
An epic has a git repository associated and a branch
A ticket has a git repository associated and a branch

A project has branching rules.

A ticket fetched by an agent should contain project details and all parents.


REVIEW
------
read agents; review for drift, design, documentation, testing and implementation.



The analyse button on a story.

When clicked, the server shoudl take the text description of the story and pass to codex.
The prompt should include the instrutions on using `ticket` as a binary to create tickets representing the breakdown to epics and tasks associated with the project.

the process should be spawned using environment variables for the TICKET_URL, TICKET_USERNAME, TICKET_PASSWORD etc.

tk project should print the project usage

tk project ls
    should print all projects wiht a * indicating current



-----

## Config file

How to determine the configuration of the tk client:

1. Look for $CWD/.ticket.json, walking up to $HOME/.ticket.json
2. Look in $TICKET_CONFIG_DIR/ticket.json

A project in the database and config for the user file can be initialised with:

```bash
tk project init
prefix      : (3-letters from the $CWD)
title       : ($CWD dirname)
description : ($CWD dirname)
```

If the project already exists AND the config.json does not exist, the user would be prompted to associate this folder with the project.
    
This should allow a user to run ticket across multiple folders:

```bash
cd $CODE/project-1/
tk create "A new ticket"

cd $CODE/project-2/
tk create "A new ticket"
```

The above would then create two tickets in different projects.

When using tk from the terminal as a client<->server,  The user location in the filesystem should assist in determining the project.   tk should walk "up" the current directoy until $HOME, looking for a ticket.json which 

---

Let's discuss refactoring the concept of TICKET_CONFIG_DIR

TICKET_URL=file:///path/to/ticket.db
TICKET_URL=https://hostname
TICKET_URL=http://hostname

All provide implied location and style of access.  

A file:/// does not require an explicit user - the user is implicitly the admin.  This would be local mode.

An https:// or http:// is remote mode and demands a username/password and/or jwt/session token.

----

ticket onboard 
-----

the entire agent mode needs to be tested
agent mode

Review the command, `tk agent`

export AGENT_NAME=fred
export TICKET_URL=....
tk agent (-name $AGENT_NAME) -max_tickets N -workflow workflow.md

Should periodically query for tickets, requesting for work to be assigned.


---------------------------------------------
TICKET/PROJECT WORKFLOW REFACTOR

I want to refactor major parts of the ticket system to simply and introduce a workflow but I am not certain what I want to do.  

Workflow
    A workflow is a method of describing the journey of tickets through an engineering lifecycle.   For example

    Workflow: "default"
        A WORKFLOW has STAGE(s): Design -> Develop -> Test -> Complete

        A STAGE has a title "Develop", a description, and a ROLE attached to it

        A ROLE is an entity that describes the motivation of the agent, e.g. "You are a software engineer, you will write software according to the SDLC."

    A Project has a WORKFLOW associated with it.

    In this way a ticket can have a STAGE and a STATUS
        STAGE  = any of the workflow stages
        STATUS = idle,inprogress,complete
        

    The CLI should not permit the user to exlicitly set a STAGE; this is decided by the ORCHESTRATOR in the server which looks at the current stage of a ticket and progresses it to the next stage in the workflow the project has.

    In this way when a human or agent requests a ticket, they are given a ticket in a specific stage, with the role associated with the ticket.   This allows the agent all the context necessary to perform the work of the ticket.


I want to divide the work into phases where we will stop and test with me in the loop:

1. Create Workflow Entity, Backend, CLI tools.  Include an import/export function within the workflow.

2. Extend the project commands to associate workflows 
    
3. Update the ticket entities to use these new areas, removing old columns as necessary.

I don't want to create migration scripts for databases, we will just overwrite any existing databases and accept in-dev data loss 
in those .db files.

Please review above and make recommendations/ask clarifying questions until we have a coherent plan, at which point I will ask that you implement it.


Ticket



    Is in a STAGE eg "design"
    this means it either needs some "design"
    or it is currently doing "design"
    or it has completed "design"

    STATUS="idle/inprogress/complete"

    Once it completes, the WORKFLOW decides what 
    the next ACTION is

    A WORKFLOW is currently a file that a project adheres to which is the actions to follow.

    An ACTION is performed by a ROLE "tester", "Designer"

    so an AGENT only calls "NEXT" and the WORKFLOW decides what the "next" is

    e.g.

    WORKFLOW "A"
        CAN THEN BE ATTACHED TO A PROJECT
        API CALL IS "NEXT"

    THIS IS A WORKFLOW WHICH CAN BE ENTERED AT RUNTIME
    OR PRESUPPLIED AS A YAML/JSON
    AND CAN BE UPDATED/UPLOADED ETC.

    DESIGN, ROLE=A, NEXT=DESIGN_REVIEW
    DESIGN_REVIEW, ROLE=B, NEXT=DEVELOP
        DEVELOP, ROLE=C, NEXT=TEST
        DEVELOP, ROLE=E, NEXT=DRIFT_VERIFY
    TEST, ROLE=D, NEXT=MERGE
    
1. Create the workflow, action, role entities, CRUD management calls, CLI commands.  

Once this exists we will then refactor the appliation itself to use the workflow.

Create a workflow file

    DESIGN, role=DESIGNER 
    DEVELOP, role=DEVELOPER
    REVIEW, role=REVIEWER