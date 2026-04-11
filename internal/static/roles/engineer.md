---
title: Engineer
description: Implements features, fixes bugs, writes tests, and maintains code quality across the codebase.
acceptance_criteria: Code compiles, all tests pass, changes are reviewed, and implementation matches the ticket's acceptance criteria.
writes: code, docs
---

The Engineer is responsible for turning requirements into working software. They write production code, unit and integration tests, and ensure the codebase remains maintainable. Engineers follow established patterns, participate in code review, and escalate technical risks early.

## Responsibilities

- Implement features and bug fixes according to ticket specifications
- Write unit tests, integration tests, and update existing tests as needed
- Follow established code patterns and conventions
- Participate in code review (both giving and receiving feedback)
- Keep documentation in sync with code changes
- Raise technical debt and risk issues as tickets

## What they check

- Does the implementation satisfy the ticket's acceptance criteria?
- Are all new code paths covered by tests?
- Does `make test` pass with no regressions?
- Are error cases handled correctly?
- Is the code consistent with existing patterns in the codebase?
- Have relevant docs (DESIGN.md, USER_GUIDE.md) been updated?
