README-PARSER.md

`parser` - a go binary which that translates a requirements.md into beads commands (but do not call beads). It should just be a single go file runnable as "go run parser.go -f REQUIREMENTS.md" which writes to stdout all the beads commands with double- newlines between beads.   It should read yhe whole requirements, validate they are correct and have referntial integrity where they refer to other EPICS or STORIES, call out the error-line if there is one, exit 1 if there is a problem, or just print the commands and exit 0.

Each entry acceptance criteria should include a reference to look at DESIGN.md, USER_GUIDE.md, docs/BEADS-RULES.md as additional context.


The output of the parser.go has problems when running trying to create beads:
- The problem is beads creates entryies with beads ids.  What shoudl we do as the commands are invalid?
