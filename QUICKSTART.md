QUICKSTARt.md

1. Write a DESIGN.md document

2. Either
Prompt your agent to create REQUIREMENTS.md
OR
parser -f REQUIREMENTS.md > commands.sh

3. Load them in to inspect them
./commands.sh

5. Set a working style
task project config-add workstyle = pr-on-success
task project config-add workstyle = merge-on-success

4. Approve them
task approve -all

5. Start working on them

#terminal 1 
wiggum loop -name ralph -role worker

#terminal 2 
wiggum loop -name ralph -role pr

#terminal 3
wiggum loop -name ralph -role tester
