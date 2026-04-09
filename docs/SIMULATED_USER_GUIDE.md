# tk User Guide (Simulated)

This is a simulated walkthrough of the `tk` CLI as it will work after the SDLC refactor. Every command and output shown here is the target behaviour. Use this to validate the design before writing code.

---

## 1. Initialise a Project

```bash
$ tk init
admin user: admin
admin password: secret
default project: 1
sdlc       : "default" (2 stages: develop, done)
roles      : 0 found
initialized database at /path/to/.ticket/ticket.db
```

A fresh project starts with a minimal SDLC called "default" containing two stages: `develop` and `done`. No roles are assigned yet.

```bash
$ tk project list
ID  PREFIX  TITLE
1   TK      ticket

$ tk status
  project  : TK (ticket)
  sdlc     : default (2 stages)
  draft    : false
```

---

## 2. Set Up an SDLC

### 2.1 Create an SDLC

```bash
$ tk sdlc create -name "Agile v1.0" -d "Standard agile lifecycle with design, develop, test and UAT"
created sdlc #2 "Agile v1.0"
```

### 2.2 Add Stages

Stages are added in order. The last stage should always be `done`.

```bash
$ tk sdlc stage-add -sdlc_id 2 -name design
added stage: design (id 1, order 1)

$ tk sdlc stage-add -sdlc_id 2 -name develop
added stage: develop (id 2, order 2)

$ tk sdlc stage-add -sdlc_id 2 -name test
added stage: test (id 3, order 3)

$ tk sdlc stage-add -sdlc_id 2 -name uat
added stage: uat (id 4, order 4)

$ tk sdlc stage-add -sdlc_id 2 -name done
added stage: done (id 5, order 5)
```

Update a stage with description and acceptance criteria:

```bash
$ tk sdlc stage-update -sdlc_id 2 -stage_id 1 -description "Architecture and requirements gathering" -ac "All requirements documented and architecture approved"
updated stage #1 (design)

$ tk sdlc stage-update -sdlc_id 2 -stage_id 2 -description "Implementation" -ac "Code written, unit tests passing"
updated stage #2 (develop)
```

### 2.3 Add Roles

```bash
$ tk sdlc role-add -sdlc_id 2 -title "Product Owner" -description "Owns the product vision and priorities" -ac "Requirements are clear and prioritised"
created role #1 "Product Owner"

$ tk sdlc role-add -sdlc_id 2 -title "Business Analyst" -description "Translates business needs to requirements" -ac "Requirements documented in acceptance criteria"
created role #2 "Business Analyst"

$ tk sdlc role-add -sdlc_id 2 -title "Architect" -description "Ensures structural integrity" -ac "Architecture reviewed and approved"
created role #3 "Architect"

$ tk sdlc role-add -sdlc_id 2 -title "Engineer" -description "Implements features and fixes" -ac "Code complete with tests"
created role #4 "Engineer"

$ tk sdlc role-add -sdlc_id 2 -title "QA" -description "Validates quality" -ac "All test cases pass"
created role #5 "QA"
```

### 2.4 Assign Roles to Stages

Design has three roles (in order). Develop and Test have one each. UAT reuses the Product Owner. Done has no roles.

```bash
$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 1 -role_id 1
assigned "Product Owner" to stage "design" (position 1)

$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 1 -role_id 2
assigned "Business Analyst" to stage "design" (position 2)

$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 1 -role_id 3
assigned "Architect" to stage "design" (position 3)

$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 2 -role_id 4
assigned "Engineer" to stage "develop" (position 1)

$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 3 -role_id 5
assigned "QA" to stage "test" (position 1)

$ tk sdlc stage-role-add -sdlc_id 2 -stage_id 4 -role_id 1
assigned "Product Owner" to stage "uat" (position 1)
```

### 2.5 Review the SDLC

```bash
$ tk sdlc get -sdlc_id 2
ID          : 2
Name        : Agile v1.0
Description : Standard agile lifecycle with design, develop, test and UAT

Stages:
  ORDER  ID  STAGE    ROLES                                    DESCRIPTION
  1      1   design   Product Owner, Business Analyst, Archi..  Architecture and requirements gathering
  2      2   develop  Engineer                                  Implementation
  3      3   test     QA                                        (none)
  4      4   uat      Product Owner                             (none)
  5      5   done     (none)                                    (none)

Roles:
  ID  TITLE             DESCRIPTION
  1   Product Owner     Owns the product vision and priorities
  2   Business Analyst  Translates business needs to requirements
  3   Architect         Ensures structural integrity
  4   Engineer          Implements features and fixes
  5   QA                Validates quality
```

### 2.6 Attach SDLC to Project

```bash
$ tk project set-sdlc -project_id 1 -sdlc_id 2
project "ticket" now uses sdlc "Agile v1.0"

$ tk status
  project  : TK (ticket)
  sdlc     : Agile v1.0 (5 stages, 5 roles)
  draft    : false
```

---

## 3. Working with Tickets

### 3.1 Create Tickets

```bash
$ tk add "User login feature"
TK-1 created (design/idle, role: Product Owner)

$ tk add -type epic "Authentication system"
TK-2 created (design/idle, role: Product Owner)

$ tk add "OAuth2 integration" --parent TK-2
TK-3 created (design/idle, role: Product Owner, parent: TK-2)

$ tk add "Password reset flow" --parent TK-2
TK-4 created (design/idle, role: Product Owner, parent: TK-2)
```

New tickets start at the first stage and the first role within that stage.

### 3.2 Draft Mode

If the project has `draft: true`, tickets start in draft mode and won't appear in active work queues:

```bash
$ tk project set-draft true
project draft default set to true

$ tk add "Needs refinement"
TK-5 created (design/idle, role: Product Owner, draft)

$ tk show TK-5
Key     : TK-5
Title   : Needs refinement
Stage   : design
Role    : Product Owner
State   : idle
Draft   : true
...

$ tk undraft TK-5
TK-5 updated (draft is now false)
```

### 3.3 View Tickets

```bash
$ tk list
KEY   TYPE  STAGE    ROLE             STATE  TITLE
TK-1  task  design   Product Owner    idle   User login feature
TK-2  epic  design   -                -      Authentication system
TK-3  task  design   Product Owner    idle   OAuth2 integration
TK-4  task  design   Product Owner    idle   Password reset flow

$ tk show TK-1
Key       : TK-1
Type      : task
Title     : User login feature
Stage     : design
Role      : Product Owner
State     : idle
Draft     : false
Complete  : false
Archived  : false
...
```

---

## 4. Moving a Ticket Through the Lifecycle

Let's walk TK-1 through the full "Agile v1.0" SDLC.

### 4.1 Design Stage - Product Owner (role 1 of 3)

```bash
$ tk active TK-1
TK-1 updated (state is now active)

$ tk show TK-1
...
Stage   : design
Role    : Product Owner
State   : active
...

# Product Owner finishes their work
$ tk success TK-1
TK-1 updated (state is now success)
```

### 4.2 Advance to Next Role

The ticket is now `design/success` at the Product Owner role. Use `tk next` to advance to the next role within the same stage:

```bash
$ tk next TK-1
TK-1 advanced: design/Product Owner -> design/Business Analyst (idle)

$ tk show TK-1
...
Stage   : design
Role    : Business Analyst
State   : idle
...
```

### 4.3 Continue Through Design

```bash
# Business Analyst works
$ tk active TK-1
TK-1 updated (state is now active)

$ tk success TK-1
TK-1 updated (state is now success)

$ tk next TK-1
TK-1 advanced: design/Business Analyst -> design/Architect (idle)

# Architect works
$ tk active TK-1
TK-1 updated (state is now active)

$ tk success TK-1
TK-1 updated (state is now success)

# Last role in design - next advances to next stage
$ tk next TK-1
TK-1 advanced: design/Architect -> develop/Engineer (idle)
```

### 4.4 Develop, Test, UAT

```bash
# Develop stage (one role: Engineer)
$ tk active TK-1
TK-1 updated (state is now active)

$ tk success TK-1
TK-1 updated (state is now success)

$ tk next TK-1
TK-1 advanced: develop/Engineer -> test/QA (idle)

# Test stage (one role: QA)
$ tk active TK-1
TK-1 updated (state is now active)

$ tk success TK-1
TK-1 updated (state is now success)

$ tk next TK-1
TK-1 advanced: test/QA -> uat/Product Owner (idle)

# UAT stage (one role: Product Owner)
$ tk active TK-1
TK-1 updated (state is now active)

$ tk success TK-1
TK-1 updated (state is now success)

# Last role in last stage before done - next completes
$ tk next TK-1
TK-1 advanced: uat/Product Owner -> done (complete)
```

The ticket is now complete. Stage is `done`, `complete=true`.

---

## 5. Handling Failure

### 5.1 Fail and Retry in Place

```bash
$ tk active TK-3
TK-3 updated (state is now active)

# QA finds a problem
$ tk fail TK-3
TK-3 updated (state is now fail)

# Retry: reset to idle, try again
$ tk idle TK-3
TK-3 updated (state is now idle)

$ tk active TK-3
TK-3 updated (state is now active)
```

### 5.2 Fail and Regress with `tk previous`

```bash
# Ticket is at test/QA and fails
$ tk fail TK-4
TK-4 updated (state is now fail)

# Send it back to the previous step
$ tk previous TK-4
TK-4 regressed: test/QA -> develop/Engineer (idle)
```

If the ticket was at the first role of a stage, `tk previous` goes to the last role of the previous stage:

```bash
# Ticket is at develop/Engineer and fails
$ tk fail TK-4
TK-4 updated (state is now fail)

$ tk previous TK-4
TK-4 regressed: develop/Engineer -> design/Architect (idle)
```

### 5.3 Preconditions for next/previous

```bash
# Can only advance if state is success
$ tk next TK-3
error: cannot advance TK-3 — state is "active", must be "success"

# Can only regress if state is fail
$ tk previous TK-3
error: cannot regress TK-3 — state is "active", must be "fail"
```

---

## 6. Completing and Reopening

### 6.1 Completing a Ticket

`tk complete` (or its alias `tk close`) marks a ticket as finished regardless of its current position:

```bash
$ tk complete TK-3
TK-3 completed (stage: done, complete: true)

$ tk show TK-3
...
Stage     : done
State     : idle
Complete  : true
...
```

### 6.2 Reopening a Completed Ticket

```bash
$ tk reopen TK-3
TK-3 reopened (restored to: test/QA, state: idle)
```

The ticket returns to the stage and role it was at before completion.

---

## 7. Archiving

```bash
$ tk archive TK-1
TK-1 archived

$ tk list
KEY   TYPE  STAGE    ROLE             STATE  TITLE
TK-2  epic  design   -                -      Authentication system
TK-3  task  test     QA               idle   OAuth2 integration
TK-4  task  design   Architect        idle   Password reset flow

# TK-1 is hidden. To see archived tickets:
$ tk list --archived
KEY   TYPE  STAGE  ROLE  STATE  TITLE
TK-1  task  done   -     idle   User login feature

$ tk unarchive TK-1
TK-1 unarchived
```

---

## 8. Direct Stage Movement

You can jump a ticket directly to a specific stage, bypassing the role-by-role progression:

```bash
$ tk stage TK-4 develop
TK-4 updated (stage: develop, role: Engineer, state: idle)

$ tk stage TK-4 test
TK-4 updated (stage: test, role: QA, state: idle)
```

This always sets the role to the first role in the target stage and resets state to `idle`.

---

## 9. Reordering Stages and Roles

### 9.1 Reorder Stages

```bash
$ tk sdlc stage-order -sdlc_id 2 -stages 1,3,2,4,5
reordered stages: design, test, develop, uat, done
```

### 9.2 Reorder Roles Within a Stage

```bash
# Move Architect before Business Analyst in the design stage
$ tk sdlc stage-role-order -sdlc_id 2 -stage_id 1 -roles 1,3,2
reordered roles in "design": Product Owner, Architect, Business Analyst
```

### 9.3 Remove a Role from a Stage

```bash
$ tk sdlc stage-role-rm -sdlc_id 2 -stage_id 1 -role_id 2
removed "Business Analyst" from stage "design"
```

---

## 10. Export and Import SDLCs

Share an SDLC definition between projects:

```bash
$ tk sdlc export -sdlc_id 2 -o agile_v1.json
exported "Agile v1.0" to agile_v1.json

$ tk sdlc import -f agile_v1.json
imported sdlc "Agile v1.0" (id 3, 5 stages, 5 roles)

# Use it in another project
$ tk project set-sdlc -project_id 2 -sdlc_id 3
project "other-project" now uses sdlc "Agile v1.0"
```

---

## 11. Epics and Parent Tickets

Epics cannot have their state or stage changed directly. Their lifecycle is derived from their children:

```bash
$ tk show TK-2
Key       : TK-2
Type      : epic
Title     : Authentication system
Children  : TK-3 (test/QA/idle), TK-4 (design/Architect/idle)
Stage     : design (least-advanced child)
State     : idle
Complete  : false
...

# You cannot change an epic's state directly:
$ tk active TK-2
error: TK-2 has children — state is derived from descendants

# Complete all children, and the epic completes:
$ tk complete TK-3
TK-3 completed (stage: done, complete: true)

$ tk complete TK-4
TK-4 completed (stage: done, complete: true)

$ tk show TK-2
...
Stage     : done
Complete  : true
...
```

---

## 12. Board View

The TUI board shows tickets organised by stage columns. Each column corresponds to a stage in the project's SDLC:

```bash
$ tk tui
```

```
 Home | Projects | Ideas | Tickets | Board | Workflows | Config
 ticket
 ─────────────────────────────────────────────────────────────
 DESIGN (2)     DEVELOP (1)    TEST (0)       UAT (0)        DONE (1)
 ─────────────────────────────────────────────────────────────
 TK-3           TK-4
 TK-5
 ─────────────────────────────────────────────────────────────
 TK-3  OAuth2 integration
 type: task  status: design/idle  role: Product Owner  assignee: alice
 parent: TK-2
```

Navigation: arrows/wasd move between columns and tickets. Enter opens detail. Tab cycles tickets then advances to the next column.

---

## 13. Minimal SDLC (Simplest Setup)

Not every project needs a full lifecycle. The minimum is two stages and one role:

```bash
$ tk sdlc create -name "Simple" -d "Just build it"
created sdlc #4 "Simple"

$ tk sdlc stage-add -sdlc_id 4 -name develop
added stage: develop (id 10, order 1)

$ tk sdlc stage-add -sdlc_id 4 -name done
added stage: done (id 11, order 2)

$ tk sdlc role-add -sdlc_id 4 -title "Developer" -description "Does all the work" -ac "Code complete"
created role #6 "Developer"

$ tk sdlc stage-role-add -sdlc_id 4 -stage_id 10 -role_id 6
assigned "Developer" to stage "develop" (position 1)

$ tk project set-sdlc -project_id 1 -sdlc_id 4
project "ticket" now uses sdlc "Simple"
```

Now tickets go: `develop/Developer` (idle -> active -> success) -> `done`. Two commands to complete a ticket:

```bash
$ tk add "Fix the bug"
TK-6 created (develop/idle, role: Developer)

$ tk active TK-6
TK-6 updated (state is now active)

$ tk success TK-6
TK-6 updated (state is now success)

$ tk next TK-6
TK-6 advanced: develop/Developer -> done (complete)
```
