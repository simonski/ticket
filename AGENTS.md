# Agent Instructions

This project uses **ticket** for issue tracking. Run `ticket onboard` to get started.

- read docs/RULES.md

## Quick Reference

```bash
ticket create                       # create a ticket
ticket update <id> --status develop/active  # Mark work active
ticket list --status develop/idle   # Find available work
ticket get <id>           # View issue details
ticket done <id>          # Complete work
```

You MUST use remote mode - that is, the `ticket` client needs to talk to the ticket server - NOT the database directly.

Default to

```bash
TICKET_URL=http://localhost:8080
TICKET_USERNAME=admin
TICKET_PASSWORD=password
```

## Workflow

Always create a ticket using `ticket create`, then track that through using `ticket update` and close the ticket once complete.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
