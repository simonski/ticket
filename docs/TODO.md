FOLLOW-UPS (2026-06-10)
    - Revive tests/playwright/site2.spec.js: the harness is fixed (make test-browser-site2
      serves web/default+web/shared via tests/serve-site.py and the app honours
      window.__site2MockFetch again), and the context/reorder tests pass, but ~28 older
      tests predate the site2→default UI rename and need their selectors/flows updated.
      Once green, wire test-browser-site2 into test-all.
    - Enforce the ticket parenting matrix (SPEC.md 5.4.2): validateTicketParenting
      currently only validates type names, not the feature→epic→story hierarchy.

THE GOAL
    PLANNING
    1. create a goal
    2. break it down and refine it in a planning session
    keep reviewing and see a visualisation of hte work and order
    eventually say yes
    EXECUTION
    it then drops into an engineering queue
    either it appears to succeed
    or it appears to fail
    or there are clarifying questions
    goto 2. 
    VALIDATION
    the user says yes no etc
    either it
    SUCCEED - integrate/merge
    FAIL - goto 1.

tk init should not exist - remove it completely -all code and references
- there should be no need for a "local" .ticket.json 
the git remote shouldbe enough to "find" the poject bein speciifed
OR the user will have specified a project using -project_id or TICKET_PROJECT

the error "error not a tk project" should not happen because the server should work out which porject to use
- via a TICKET_PROJECT, -project_id, .git remote discovery, or just use the default project assined to that user (normally their own private project)

A project can have multiple git repositories - ensire they have be CRUD-managed if the user is an admin of the project

**given**

# Terminal 1
tk initdb
tk server

**and**

# Terminal 2
export TICKET_URL=http://localhost:8000
export TICKET_USERNAME=admin
export TICKET_PASSWORD=password

**Then**

All/all calls `tk xxxx` to the server *should* pass the nearest git remote from the CWD (going "up" and stopping at $HOME).

Any call *may* pass a project_id explicitly

    1. The user has an environment variable TICKET_PROJECT=, in which case use that.
    2. The user explicitly states the project id as -project_id N, in which case use this, overriding the env var

The request *may* contain a project id, in that case, IF the user is a member of that project, honour this value.  If the user is NOT a member of this project, return unauthorised and log a possible breach attempt on project ID by this user.   If the project does not exist, return not found explaining the project does not exit.

The aliases "public" and "private" can be used when referring to TICKET_PROJECT_ID or -project_id

If the project_id is NOT specified explicitly, the server will use heuristics to determine the project
    if the client provides the git remote a project is looked up
    if the project is public, then the user "is" a member of the project by default without needing explicit membership and can interact according to the default membership role of the project

        if the user is a member of a team that has access, the project can be accessed
        if the user is not a member of a team that has access, the user will 
            receive an unauthorised response
            if the project accepts new members, they will be informed they can request access
    if it is foumd, use that project
    if it is not found, use the users private project

project roles/ACL
    a user has permissions within a project based on their membership role on that project
    admin: full CRUD permissions on all entities
    member: crud permissions on stories
    observer: visibility
    commenter: obsrever plus ability to add comments
a user cannot modify a ticket of a project they do not have write access to

- modify the server initdb so that the admin/password user is setup using the same flow but is an admin-level user.
ensure the public project is made and the public team is made and that the public project is assigned to the public team
ensure that new users by default as assigned to the public team
ensure that new users by default have a "private" project created that is aliased as private

An admin user should be able to control this auto-assigned to teams and projects and also control whether or not users automatically recieve their own private team too.  This should be in the "plan" th euser is assigned to so that each plan can describe the various "on registration" actions that should be taken.

----
ticket.exe.xyz
    
    We will run a ticket server there with some settings turned on: 
        auto-approve registration
        auto-assign to plan "free"

    plan "free"
        auto-create private project as role: owner
            project - no other members permitted

        auto-join "public" team as role: developer

        set quotas
            projects 2
            teams 3 (private/public/spare)
            tickets.total 100
            tickets.per-project 100
            api.calls.per.day 100; resets at 0000 UTC

        auto-assign to team project "public"
        set default project to be "public"

Normal install is

    brew install simonski/tap/tk
    cd code/project1
    tk <command>


    tk register -username N -email Y
    if auto-register is set, reply success with password

    


A project has a unique ID, a title and a description and 1 or more git repo origins

From the CLI, The ID or title can be used in the -project_id if necessary
From the website, the user will select from a dropdown their project so will send project_id

A user cannot access a project that they are not a member of.

Every user has their own "private" project.
Every user is a member of a "public" project.

Projects can be private, team or public

Private is visible only to one user - the owner.
Team is a project that is available to multiple.   Each user has a role in the project allowing them different permissions.
Public is a project that will be visible to *any* user.


Using the tk command will complain if the user is not logged in unless there is a TICKET_USERNAME and TICKET_PASSWORD or an access token available (via ~/.config/ticket/credentials.json).   The user will be prompted to login or set the env vars.  Logging in will store an access token in ~/.config/ticket/credentials.json which would hten be used until it expires server-side.

The exception is `tk register` which will require a -username and -email then the registration will - depending on the server configuration - either auto-accept returning a password, or return waitlist, or a check yuor email, or a server not acccepting registrations right now style response.


seeding

A new command `tk seed` should be created to seed example projects, requirements and tickets.

1. todolist
    A 3-tier todo list
        cli
        website
        backend
        using openapi
        red/green
        drag+drop
        user registration
        email integration
        admin user

    Option 1: in python
    Option 2: in go
    Option 3: in rust
    Option 3: in zig


A "template" set of requirements should be seeded in an examples project.
A template set of already broken-down tickets shoudl be seeded in an examples project.

DRAG-DROP a document in
    refinement and breakdown
        show a gui
            a project?
                an epic
                    a feature?
                        a set of stories?
