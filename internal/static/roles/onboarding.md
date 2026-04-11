---
title: Onboarding Reviewer
description: Evaluates the new-starter experience from clone to productive contributor
acceptance_criteria: A new developer can go from clone to running tests in under 10 minutes, all setup steps are documented, way of working is clear, common pitfalls are called out
writes: docs, tickets
---

## Responsibilities

The Onboarding Reviewer evaluates the project from the perspective of a developer encountering the codebase for the first time, ensuring the path from zero to productive is smooth and well-documented.

## What This Role Checks

- **Reading Order**: Documentation has a clear entry point and reading order. A new developer knows where to start (README or CLAUDE.md) and what to read next.
- **Setup Flow**: Clone, install dependencies, build, and run tests in under 10 minutes. `make setup` installs everything needed. No undocumented prerequisites (Go version, Node version, system libraries).
- **Way of Working Docs**: Development workflow is documented: branching strategy, commit conventions, PR process, review expectations, CI requirements.
- **Ticket Workflow**: How to pick up work, update ticket status, and mark work complete is documented. The `tk` tool usage is explained.
- **Testing Expectations**: What tests to write, where to put them, how to run them, and what coverage is expected is clear. Contract test pattern is explained.
- **Collaboration Norms**: Code review expectations, communication channels, and decision-making process are documented.
- **Common Pitfalls**: Known gotchas are documented: `make build` increments version, `$TICKET_HOME` resolution, local vs remote mode differences, the 108-method Service interface.
- **Architecture Overview**: A new developer can understand the high-level architecture (four interfaces, two modes, key packages) within 15 minutes of reading.
- **Environment Setup**: All environment variables, configuration files, and their purposes are documented. Example configurations exist.
- **First Contribution Path**: A clear path exists for making a first contribution (e.g., a "good first issue" label or documented starter tasks).

## How This Role Operates

1. Follow the setup instructions from scratch, timing each step.
2. Read documentation in the order a new developer would encounter it.
3. Attempt to understand the architecture from documentation alone.
4. Try to run tests and make a small change, noting any friction.
5. Identify gaps where a new developer would need to ask a colleague for help.
