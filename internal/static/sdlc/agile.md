---
name: Agile
description: General purpose agile methodology with design, development, testing, and completion stages.
default: true
---

## Stages

### design
order: 1

Requirement analysis, architecture decisions, and acceptance criteria refinement.

Previous: none
Next: develop

Roles:
1. @product-owner
2. @business-analyst

Entry requirements:
- Ticket has a title and type assigned
- Ticket is in draft mode

---
Exit requirements:
- Description is complete and unambiguous
- Acceptance criteria are defined and reviewable
- Dependencies on other tickets are identified and linked
- Ticket type and priority are confirmed
- Ticket is no longer in draft mode

Acceptance criteria:
- A developer could pick up this ticket and begin work without further clarification

### develop
order: 2

Implementation, unit testing, and code review.

Previous: design
Next: test

Roles:
1. @engineer

Entry requirements:
- Description and acceptance criteria are complete (design stage exit requirements met)
- An assignee is set

Exit requirements:
- Code compiles and all existing tests pass (`make test`)
- New code has unit and integration test coverage
- Code has been reviewed or is ready for review
- Documentation updated if user-facing behaviour changed

Acceptance criteria:
- Implementation satisfies the ticket's acceptance criteria
- No regressions introduced

### test
order: 3

Integration testing, QA verification, and acceptance testing against the acceptance criteria.

Previous: develop
Next: done

Roles:
1. @qa

Entry requirements:
- Code is complete and tests pass (develop stage exit requirements met)
- Changes are deployed to a testable environment or branch

Exit requirements:
- All acceptance criteria verified and passing
- Edge cases and error paths tested
- No critical or high-severity bugs remaining
- Regression suite passes

Acceptance criteria:
- QA signs off that the ticket meets its stated acceptance criteria

### done
order: 4

Work is complete and accepted. This is the terminal stage.

Previous: test
Next: none

Roles: none

Entry requirements:
- All prior stage exit requirements are met
- Ticket is accepted by the product owner or assignee

Exit requirements: n/a

Acceptance criteria:
- Ticket is closed and requires no further action
