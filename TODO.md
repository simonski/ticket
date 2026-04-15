We need a definitive specification of the entities as a design document and then we need to apply it against the codebase to ensure it is all correct.

We will work in phases.  For now it is phase1.

1. agree the entities and their relationships
2. refactor/implement the datamodel and codebase
3. refactor/implement the CLI commands
4. refactor/implement the TUI commands
5. refactor/implement the website

-----

1. agree the entities and their relationships

In this phase I want a back-and-forth conversation with you having read the docs and codebase agains this new requirement.  The final output should new definition document that describes the entities below, their purpose and usage.  It should be useable for humans as well as act as a design document.  

PROJECT, SDLC, STAGE, ROLE, TICKET

PROJECT has a default SDLC but any epic or story (synonym for ticket) can use a different SDLC 

This means a ticket of any type has an sdlc foreign key ewhich may-or-may not be populated. If it is, use that
as teh sdlc for that story.  If it is not, check any parent ticket until you find an sdlc, otherwise use project sdlc, which is non-null.   

dor: definition of ready
dod: definition of done

PROJECT has
    title: description: free text
    prefix: 1-3 digit prefix for ticket IDs
    git repo: git url
    dor: text describing definition of ready
    dod: text describing definition of done
    default_draft:boolean, indicates if tickets start in draft
    default_sdlc:link to sdlc

SDLC has
    title
    description
    list of stanges (ordered)
    each stage in the SDLC has a list of roles

STAGE has
    title
    description
    dor
    dod
    ac

ROLE has
    title
    description
    dor: map (keyed by stage)
    dod  map (keyed by stage)
    ac:  map (keyed by stage)

    Note1: the role dod, dor, ac values are keyed by stage however if the value for the current stage is missing, then the 'default' key is used.   in this way a role can 'specialise' for a stage or be general purpose across any stage it is called on to do work.

    Note2: a role can be put in any stage in any order - this is specific to the SDLC.   A Stage can be included or excluded in any SDLC also.  

TICKET has
    id (prefix+integer) "TK-123"
    title
    type:
    description
    dod
    dor
    ac
    stage:
    state:
    deleted:
    archived:
    compelte:
    draft: boolean
    sdlc: optional
    parent ticket id: optional
    project

A TICKET is *any* piece of actionable work.  It has a type (epic, task, bug, chore, idea, requirement, feature) however the type is more of a piece of advice to classify it.  The more important part is the LINEAGE - if it has a parent or not.  So by implication a ticket that has multiple children is "complex" - it has many parts "inside it".  You'd probably call that ticket type an epic.  

==========================================================================================
2. refactor/implement the datamodel and codebase






it should show the sequence of stages
    it shoudl show the sequence of roles in those stages
        the combination of any of them should yield

project + stage + role + ticket
    P: high level prompts that apply to 100%
    P-WOW
    P-DOR
    P-DOD
    P-AC
    +
    S: prompts that apply to the stage: design, do not write code, write diagrams and docs
    What is the WOW for the Stage Design
    S-WOW
    S-DOR
    S-DOD
    S-AC
    +
    R-WOW
    R-DOR
    R-DOR
    R-AC
    +
    Role-Stage: WOW: What is the WOW for the Stage Design for the Role "QA".  What does a QA want to do in Design?
    Role-Stage: DOR
    Role-Stage: DOR
    Role-Stage: AC
    +
    T-WOW
    T-DOR
    T-DOD
    T-AC
    T-TITLE
    T-DESCRIPTION
    
tk sdlc ls
tk sdlc get -id N / -name foo


renders



------

Ticket can ALSO be a client to a different backend

    1. ticket
    2. gitlab (issues, labels discriminate epic/story/task/bug)
    3. github (issues, labels)
    4. jira


A project is the container of work.
A project has a title, description.

A project has one SDLC (formerly called "Workflow") attached to it.

An SDLC can be exported and given to "other" projects via an `sdlc_<name>.json` import/export file

A project contains tickets.
A ticket can have child tickets.
A ticket has a type (epic, bug, story, etc)

- A ticket does not care what state/stage it is in - it does not have any logic or preference.

A ticket is either (active: true/false)
    idle (no-one is working on it)
    active (someone is working on it)

A ticket is either (complete: true/false)
    complete.  - it is totaly finished with no more work
    incomplete - it is not yet finished

A ticket is either (draft: true/false)
    draft      - it is NOT yet ready to be worked on (a human is still curating it)
    ready      - it is now ready to be worked on

A ticket is in a stage (stage: STRING)
    the value of which is defined in an SDLC process, which is part of the project.
    if a ticket is marked as complete=true, then the stage is always "done"

A stage
    has a name "design", "develop", "test", "release", "done"

A stage has a status
    "idle"
    "in progress"
    "success"
    "failed"

A project has SDLC "yolo"
    Which has one role: "Solo developer does it all in one."







------
TODO.md

test
    verify the test harness calls every single call in the openapi spec
    verift the test harness uses every single method in the library
    verify the http library calls every single method in the api


SKILLS.md/TICKETS.md
    merge the two into a SKILL.md for Claude
    QUICKSTART_CLIENT
    QUICKSTART_SERVER
----------------------------------------------------------------------------------------------------

QUICKSTART
    review the quickstart and see if it is correct
    write a program which reads the quickstart and actually runs it to verify that it is all correct as a test case
- QUICKSTART for CLIENT

Default to ticket.

- QUICKSTART for SERVER

Run it in !!!ticket.exe.dev!!!

initial setup woudl be quick and runnung on exe.dev serverless mode
the project would have a max number of tickets

the user is an admin of "their" projects
the admin is an admin of users
teams are users who get access to projects (git access is up to them)
agents are then identities run on "their" equipment (but could be mine eventually)


----------------------------------------------------------------------------------------------------

READ: docs/LIFECYCLE.md, docs/TICKET_LIFECYCLE_SPEC.md

**Note** thisis NOT for AGENTS - IGNORE THIS FILE.

claude skill/codex skill 
a ticket shoudl NTO just stop, or leave a ticket, or not use git, or not subtrees, or not touch various things.
or maybe it shoudl, and an epic should do that.  but the point is it should verify and perform `tk based` tasks or not.

a ticket should
    work in a branch
    use specific git instructions based on the ticket name
    the prompt should
        be stored
    
    completion should build a report

    a REPROT is
        the prompt andall associated
        the change
        the outcome
        # metrics
            tests passed/failed
            tests created
            code added/removes



conflict is
    git conflict


--------

get to bootstrapping via exe.dev

--------


---------

docker-ify the whole thing; watchtower the images on the exedev

---------

postgres the backend

---------
