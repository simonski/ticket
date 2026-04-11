---
name: YOLO
description: Single-player straight-to-dev workflow with minimal ceremony.
---

## Stages

### develop
order: 1

Implementation, testing, and review — all done by the same person.

Previous: none
Next: done

Roles:
1. @engineer

Entry requirements:
- Ticket has a title and type assigned
- Ticket is not in draft mode

Exit requirements:
- Code compiles and all tests pass (`make test`)
- Changes have test coverage
- Documentation updated if user-facing behaviour changed

Acceptance criteria:
- Implementation satisfies the ticket's acceptance criteria
- No regressions introduced

### done
order: 2

Work is complete. This is the terminal stage.

Previous: develop
Next: none

Roles: none

Entry requirements:
- Develop stage exit requirements are met

Exit requirements: n/a

Acceptance criteria:
- Ticket is closed and requires no further action
