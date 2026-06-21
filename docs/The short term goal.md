The short term goal

- HUMAN create a ticket
- HUMAN refine with an AGENT (refiner) until it is ready for development
- AGENT (refiner) recommend ready for development
- HUMAN mark it as ready for development
- AGENT (developer) agent picks it up and makes the work all the way through to a PR
- HUMAN receives T-MAIL indicating PR ready for review.

What's missing from our codebase to run a demo of that?

Each aggent is a separate process that is occasionally "phoning home" to see if there is any work for them, where work for them is described as work that meets certain criteria (stage/state/status).

----

board view agents

render a green/red/blue (Some colour scheme) on each story to represent if they are currently
acively being worked on by an active agent.  Perhaps a little robot or cpu icon that is glowing faintly green to simulate currently active, to the left of the ticket id in the view.

Also perhaps show the agents as little icons in the top to the right of the selected project.  The icon would be like a plittle cpu or robot image.  The image woudl then h ave a little icon on the top-right indicating what it was doing.

If they are active make them green, if they are idle make them the regular colour.   this way we can see the utilisation of agents at a glance.

If an agent is on the project but not currently available (Say it is not phoning home) then it is not healthy and there would be an (!) near the agent to understand.   

If it was avaialble but doing nothing perhapsa. (zzz) sleeping idle style

If an agent had a question perhaps a (?) next to it.   

Clicking on the agent icon woudl then bring up the appropriate popup.


This also requires either explicitly assigning agents to projects, or that a project darws from the "pool" of registered agents such that projects are "living" pieces of execution.

The demo and initial setup should create some agents in those roles - "refiner", "developer", "tester" etc. with passwords of "password" so that I can just run the agent locally, which I think is going to be 

```shell
export TICKET_URL=xxxxx
export AGENT_ID=xxxxx
export AGENT_PASSWORD=xxxx
tk agent 
```

where the AGENT_ID points to an id which is say the refiner, or the engineer, or the qa

----

clicking on a story shoudl show the lineage, dependencies, what it enables, what depends on it, the size, any commits attached to it etc.

----


perhaps "fix" the stages and sprints

idea
    A: refine
        requirement+
            A: breakdown
                story/task/bug+
                A: assess 
                    ready
                    notready
    ready:
        develop
            role1:
            role2:
            role3:

    For each iteration
        retain prompt
        retain meta (tokens, time, outcome, model)
    At end of story
        review all and ask for suggestions on improvements
            role augment
            role collapse
            role remove
            role add
            model change
            