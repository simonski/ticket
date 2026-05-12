# Client modes: local/remote mode

The mode is detected 'automatically' by the presence/absence of the env vars AND the existence of an accessible `ticket.db` database.

## local mode

is where there are no credentials provided or required and there is a ~/.ticket/ticket.db file present which is used.

The 'default' project is used in local mode UNLESS it is specified as an option -project <project_id>

Calls to tk should operate directly on the db AND advise "Warning: you are in local-mode under ~/.ticket/ticket.db"

## remote mode 

is when there are TICKET_URL, TICKET_USERNAME, TICKET_PASSWORD vars set and the database is "remote" - accessed over http/s openapi.
Am optional TICKET_PROJECT can be set to a project ID, otherwise a -project_id must be specified.

If a `.ticket.json` file is found then it is used for the TICKET_URL, TICKET_USERNAME and TICKET_PROJECT - BUT it NEVER contains the TICKET_PASSWORD and will advise and fail if it is present.

## tk initdb

`tk initdb` will create (`--force` will recreate) a `~/.ticket/ticket.db` database.  Supplying `-f <filename.db>` will override the location of the file to create.   An admin user of `admin/password` is created by default.   The default project, workflow, roles are populated on `initdb`.

The database file can then be accessed directly in local mode OR remote via the server running as `tk server`

## tk init

`tk init` is a prompt by the user to setup their client.   The user should be prompted with the current settings detected by the client in this moment by
    - checking if there are environment variables
    - checking if there is a `.ticket.json` file (and walking 'up' looking for one from CWD until it reaches a folder that contains a `.git` path; then stopping and not walking up further).

    if those exist it will present them as the vlaues ready for review/amendment

    if none exist it will go into a terminal-wizard to ask if the user wants to be local or remote and if they want to write a `.ticket.json` file OR just use env vars.

--------

Implement: 

tk new -f <filename> 

A method of creating multiple tickets by entries in a single file.

-commit is used to write the tickets

If -commit is missing it should print out the "intent" of the outcome - print the title and a couple of lines of each ticket in the similar style to `tk ls`.  No server comms is necessary.

If -commit is present then it will create teh tickets and Write "back" to the source file title the id:XXXX once the ticket has been made.
If the id: is present then the whole operation for that ticket shoudl act like a `tk update -f` command

tk update -f <filename>

A method of updating existing tickets by overriding their settings from a single file.

-commit is used to write the tifckets.

The id:XXXX is required so that the ticket can be found and compared to the title, description, labels

Update USAGE to reflect and docs.

When the user does not include -commit, the result should be an `tk ls` equivalent output - and advice to "Tip: `use -commit` to write back to tk".

Example

```
# I am the epic

## I am the ticket attached to the first epic

# I am a second epic

## I am a ticket attached to the second epic

### I am a ticket attached to the second ticket

### I too am a ticket attached to the second ticket

# I am a new epic

## I am a ticket attached to the third epic

Remainder is description.

Exception is a label:a, b,c - will assign the words a b and c as labels on the ticket
Exception is a type:bug - will set the "type" to be the type.

The whole operation fails if hte file cannot be parsed properly.
```

# SKATE TO WHERE THE PUCK IS

type:idea

By Humans
- no coding
- no merging
- no PRs
- no MRs
A human *could* do any of that, but it would be like digging a field.  Professional farmers use tractors AND know land.  A programmer remains
in charge but takes advice and support from their tools.

By Agents
- receives goals
- breakdown to requirements
- assess the plan
- implement the plan
- iterate against goals/breakdown
- call out to human where possible


# Many small tools.
type: idea

Was thinking about determinism in agentic engineering.   A given loop is

invoke agent with 
    GOAL, 
    RULES 
    CONTEXT
    SKILLS

Focus on the outcomes.  There is
    code in git
    tickets moved around a board
    GOAL updated

So the CONTRACT is that
    1. agent updates code
    2. agent updates tickets
    3. agent updates goal

We need a tool for each
We need to verify the tool usage
We need to capture the invocation

If we are in a shell, we can setup linux to capture all work and use the history

Invoke an agent to breakdown using the /breakdown skill

use tk in breakdown mode

tool + skill + goal + role + rules = outcome

So in that case


An agent loookig at a given folder
the folder is prepared
the agent is then invoked

>> AT ALL STAGES AN AGENT WILL ESCALATE for CLARITY and CONFIRMATION

# PLAN
goal -> refine -> iterate -> agree.

    # REFINE/CLARIFY
        The goal is review and refined to a clear statement.
        Depending on the "size" of the goal there will be a diffferent PATH (sequence of steps or gates)

    # BREAKDOWN
        - an agent is invoked to breakdown the goal into a set of tasks, and criteria for success
        - the agent and human then agree on the breakdown, or the refine it

# READY

Default url
-------
default to https://ticket.localhost

create a caddyfile thatruns locally via a make caddy and expects https://ticket.localhost, routing to http://localhost:8080

API.js
-------
create an api.js from the site2 javascript - it is ONLY to contain the networking api calls that match the openapi spec.  

This is to be tested separately from an addisitonal file, app.js which is the ux application logic, which will use the api.js.   this api.js is the interface betweeen teh backend and the ux
have the html refer to external .css and .js, not inline.

I think that the api.js  should not be tightly coupled to the site2, rather it is a library wit its own test harness and is then applied to the site.  In this way I believ we can creae site3, site4, siteN.


Agentic refintement
-------
 the agentic refinement section - there needs to be a config to specify which model is being used, if an api key or url is used etc.  
 The initial setup of the database should populate the major providers and default to one.   This should then be a systme wide default which can be overridden on a per-project  or per-goal setting.  
 
 Meaning the project itsef shoudl have this as a setting as wella s the goal.  if it is not set it shoudl default to the parent 
 (goal->project->system).


Documents, Mess, Drift
-------

 there are too many documents and they pollute the context - .md files and I want to tidy up so that the goal is

     - reduce unnecessaery documentation files
     - ensure we retain only what is necessary I think by these classification

         README

         USER_GUIDE
             QUICKSTART
             TUTORIAL

         DEVELOPER_GUIDE
             DESIGN/ARCHITECTURE/GOALS/TODO
             WAY_OF_WORKING/SDLC
             AGENTS.md/copilot instructions/CLAUDE.md

     I want you to consolidate all the documentation so that we end up with a clean repository of only the necessary docs.
     Once complete it shoudl mean all necessary documentaiton context is available via these.

DuckDB
------
What would happen if we migated to duckdb?

Testing
------
The test pass takes a long time.  

Perhaps we need to look at the rules to work out what set of test is required.  What do you think of:
`make test-unit` - unit (fast, go)
`make test-api-js` - js interface (javascript -> openapi -> server) fast - checks all js calls to the server (medium)
`make test-api-cli` - cli interface (cli/http -> openapi -> server) fast - checks all cli calls to the server (medium)
`make test-api` - (both `test-api-js` and `test-api-cli`) (fast)
`make test-browser` - browser (e2e playwright browser -> javascript -> opeapi -> server) slow
`make test-all` - all the above (slow)

Where `make test` defaults to `make test-unit` ?

If APIs are changed `make test-api` is run?
And then at the *end* of a given feature `make test-all` is run?

What changes to AGENTS.md/copilot instructions would be necessary?

Testing Quickstart
--------

`make test-quickstart` should test the quickstart.md document itself by running through the sequence of commands ensuring it does actually work as specified.

