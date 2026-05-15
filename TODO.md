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
    tk init
        use .ticket.json if present
        use TICKET_URL, TICKET_USERNAME, TICKET_PASSWORD, TICKET_PROJECT if set
        wizard will request for user to login to a server if not set

        TODO: figure out terminal-end-credentials and/or auth token.

        select default project (look at current location/git remote to see)
            if none, request to create one

    tk register -username N -email Y
    if auto-register is set, reply success with password

    
