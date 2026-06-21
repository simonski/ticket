Agents
------

The agent is really just a worker with very little identity. It is a wrapper around an agentic harness invocation where the only
things it can really do are

- phone home for work, indicating health and existence
- retrieve work via a prompt
- obtain the code and credentials to perofrm the ork
- do the work
- push teh work back to a code repository
- update the work ticket
- go back to phone home

so the agent is not started with anything other than credentials and a home address.   It will have some basic capabiltiies

Each time it has work to do it will be supplied with all necessary data to

- obtain or locate code
- credentials for an agentic harness
- the prompt to run

In this way an agent is no more an "Architect" than it is a "Software Engineer".   The prompt defines the agent.

The admin of ticket defines the agents - but really they are just identities that wrap agentic harnesses.  The admin user can then manage the "fleet" of agents by 

- assigning an agent to a dedicated project project(s)
- make the agent a "Free roaming" agent who will pickup work on proejcts that are so-called "unassigned"

In this way the Orchestrator will act as a sort of "fair scheduler" making sure stories across all projects are worked on. 


Story Lifecycle
---------------

A workflow describes the sequence of steps a ticet will go through fro start to end.
each discrete step is called a Stage.  

A stage covers an area of work like "design", "refinement", "implementation", "testing"

Each stage is then implemented using a Role

A Role is like a job description - e.g Engineer, Tester, Architect, Product Manager

A story is then the unit of work to conduct.

Composing all the units together

Story + Role + Stage + Project + Organisation

Creates a large "prompt" that will "fill in the blanks" of a template describing the goal, the acceptance criteria,
the expectation, how to fulfill the work, along wiht all technical infrmoation.

The work is actually carried out by an Agent, which is assigned the story.  The agent will then mark the stroy as "active" 
and work on it.  Once it finishes, it will either decide it was a success, or it was a failure.   If it was a success, it will
mark teh story as success and then un-assign itself.    

The orchestrator in ticket itself will then wake up, look at this ticket and decide to move the ticket to the next step in the workflow - this might be assigning to a new role for an agent to work on, or it might be moving the ticket onto a new stage and marking it as ready.

---

As the workflow is "pluggable" - that is, a project has a workflow assigned to it.  It means that depending on the workflow assigned, the project will have different "stages" to render in the board.

Sprints
-------

Work is generally prepared in the backlog.  Once it is deemed "ready for development" it can be moved into a sprint.  A sprint is just
a method of grouping a set of stories together.

Preparing a piece of work is its own agentic workflow - idea -> refinement -> breakdown

idea: this is the first text description of whatevever teh user has as a requirement
refinement: the agent and the user will go through a clarification step until the agent and the user agree
breakdown: perhaps the idea breakdown will result in multiple stories; if the user accepts them then they will be "packaged" into 
an epic which is a container of all the specific stories.

Once all the stories have been allocated into a sprint, the sprint can be "sealed" and then worked on by agents.

Agents and Orchestrators
-------------------------

The ORCHESTRATOR is responsible for coordinating movement of tickets.  It periodically wakes, looks at the current state
of all tickets across all programmes/projects and decides which ticket to actively move to which agent and udner what role.

So the orchestrator is the only thing that assigns work to agents.   All other work is "advice" that hte orchestrator will take
and act on.  For example an agent will complete a story, so the story will be marked as success.  At this point the next time the orchestrator wakes, it will see htis story and decide on the action to take based on the workflow the project the story is in - move it into a new stage, set it's status to idle.

## Running
 
Part of the server using -orchestrator as part of the `tk serve -orchestrator` call.

## Config

The frequency of wakeup is controlled via a config variable available in the UX and CLI to the admin.  The orchestrator can be run (by the admin only) in dry-run mode for a specific ticket or project

`tk orchestrator -id N`

it will product a summary of the action it would take

or `tk orchstrator N`

or project

`tk orchestrator -project_id N`

it will product a list of tickets by project with the action it would take


## Decision making

The decison making is based on workflow a given ticket is in.  

