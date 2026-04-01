
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
