README-WIGGUM.md

`wiggum` - a go binary which looks at beads and performs the work via a large language model, recording the output for the work carried out.

The instance of this wiggum will have a unique name "fred, pete, ralph, jane" which is what is used when assignment occurs.  Each instance is considered to be an agent.

1. take a piece of work from beads
    wiggum will have a name - use that as the assignee
    choose a bead you have been assigned to - or assign yourself to an appropriate bead
    an appropriate bead is
        (in development assigned to this wiggum, OR
        open, ready for development) AND
        appropriate priority AND
        not blocked by another bead AND
        not assigned to a different wiggum AND
        either unassigned OR 
        assigned to this wiggum 

2. take the work packet (The bead itself) and feed it into an LLM coding agent session
3. close teh bead
4. repeat

Usage

./wiggum loop -name fred -max 1
    performs the work in a loop -max times (0 = forever)

./wiggum loop -name fred -max 1 -dryrun
    simulates the work where it does not feed the packet to an LLM but prints to STDOUT only, sleeps for 1s then marks as complete in beads.

./wiggum 
    prints help

what sort of ralph wiggum loop woudl make beads keep working until it finished - can you write me a program that does that? go run wiggum.go

no, I want wiggum to look at beads directly using bd commands, find hte next best ticket and work on it

extend wiggum to have an agent commmand

wiggum agent 
    where it will spawn a new process and retain proxy STDIN/OUT to this agent process, optiosn are

wiggum agent "entire command"

Where the entire command is then executed in a process.
It is expected that the process spawned is a coding agent, so the STDIN/STDOUT should be rewired so that "wiggum agent" acts as a sort of wrapper aroudn the invocation.  

The user of "wiggum agent" shoudl then be able to interact with the process IN and OUT as if they had called it directly.  

This is so that we can demonsrate wiggum can talk to a coding agent.


codex --dangerously-bypass-approvals-and-sandbox "

-./wiggum agent "codex --approval-mode never"
-    runs a coding agent command with stdin/stdout/stderr passed straight through, so the session behaves like a direct invocation.
-
-./wiggum agent codex --approval-mode never
-    same as above, but without shell parsing.
-


-extend wiggum.go and parser.go so that they print a useful help usage if they are invoked with no command

- extend "wiggum" so that for each piece of work carried out during the loop, for each bead, maintain a logfile of all the STDIN/STDOUT to/from the AGENT.   At the end, ensure the exit code is recorded in the file.    The file should be plain text and should contain

modify wiggum so that it
    finds the next appropriate bead as before
    creates a new folder to hold the work
        logs/<bead-id>-bead-name/ 
    it writes the bead as "folder/input.md"
    it writes to the folder the response from the agent as $folder/output.md
    it writes start/stop/exit code/branch/bead id/title/instruction to $folder/status.json
    it invokes and instructs the agent using "codex exec - < path/to/prompt.md" where the prompt.md is the prompt

when run in dry-run the outcome should be the same except the instruction won't be "codex" it'll be the echo/sleep

>>>>>
once a job claims to be finished
it should create a clone of it that is only for independent testing
>>>>>

The filename should be logs/<wiggum-name>/<bead-id>-<branch-name>.log - replace any path like characters with a hyphen.