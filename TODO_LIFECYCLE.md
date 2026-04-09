PROJECT
-------
A project is the container of work.
A project has a title, description.

SDLC
-------
A project has one SDLC attached to it.

An SDLC can be exported and given to "other" projects via an `sdlc_<name>.json` import/export file

TICKET
-------

A project contains tickets.

A ticket can have child tickets.

A ticket has a type (epic, bug, story, etc)

A ticket is either (complete: true/false)
    complete.  - it is totaly finished with no more work
    incomplete - it is not yet finished

A ticket is either (draft: true/false)
    draft      - it is NOT yet ready to be worked on (a human is still curating it)
    ready      - it is now ready to be worked on

A ticket is in a stage (stage: STRING)
    the value of which is defined in an SDLC process, which is part of the project.
    if a ticket is marked as complete=true, then the stage is always "done"

A ticket has a status, which indicates what is currently happening to this tikcet (based on the stage it is currently in)
    "idle": no-one is working on it (an assignee may be populated)
    "in progress":     if it is active then the ticket assignee is populated
    "success": this stage has completed successfully and no-one is working on it anymore
    "failed": this stage failed for some reason and no-one is working on it anymore

Changes to tickets are recorded in the ticket_history

STAGE
-------

The SDLC will describe the workflow - which is the sequence stages of work are in.  For example an SDLC "Agile v1.0" may have three stages described in order:
    Design
    Develop
    Test

A stage
    has a name: "design", "develop", "test", "release", "done"
    has its own acceptance criteria wihch explains the purpose of this stage and what the outcome of a ticket going through it shoudl be.  


ROLE
-----

An SDLC has multiple ROLEs representing individual job functions, like architect, engineer, tester, product owner, cyber specialist, dba.

A role has a name, description and acceptance criteria.

The SDLC indicates which ROLES are assigned to which stages, so for example an SDLC "Agile v1.0" might 
    have a stage called Design
    have 10 roles in total
    assign 3 roles to Design: "Product Owner", "Business Analyst", "Architect"

    In this case this implies there will be three separate "rounds" of execution for a ticket that enters the Design Stage.








