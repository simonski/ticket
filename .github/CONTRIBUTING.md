# Contributing to ticket

Thank you for contributing! This guide covers everything you need to go from a
fresh clone to a merged pull request.

---

## Contents

1. [Code of Conduct](#code-of-conduct)
2. [Development setup](#development-setup)
3. [Branching conventions](#branching-conventions)
4. [Commit style](#commit-style)
5. [Pull request process](#pull-request-process)
6. [Testing expectations](#testing-expectations)
7. [Coding conventions](#coding-conventions)
8. [Architecture decisions](#architecture-decisions)

---

## Code of Conduct

Be kind, be constructive, assume good faith. See
[CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) for the full policy.

---

## Development setup

```bash
git clone https://github.com/simonski/ticket.git
cd ticket
make setup       # Go modules + Node + Playwright + dev tools
make test-all    # All tests must pass before you start
```

> `make build` auto-increments the patch version in `cmd/tk/VERSION` on
> every invocation. Use `make build-dev` when you want a development build
> without changing the version.

See [docs/ONBOARDING.md](docs/ONBOARDING.md) for a full onboarding walkthrough.

---

## Branching conventions

| Branch type | Pattern | Example |
|-------------|---------|---------|
| Feature | `feature/TK-XXX-short-description` | `feature/TK-42-add-labels` |
| Bug fix | `bug/TK-XXX-short-description` | `bug/TK-99-session-expiry` |
| Documentation | `docs/short-description` | `docs/update-onboarding` |
| Chore / refactor | `chore/short-description` | `chore/split-api-handlers` |

- Branch from `main`.
- Delete the branch after it merges.
- Keep branches short-lived (days, not weeks).

---

## Commit style

Format: `TK-XXX: imperative verb + brief description`

```
TK-190: enforce session expires_at in GetUserByToken
TK-193: add 9 missing database indexes for ticket list queries
docs: update ONBOARDING.md build guidance
```

Rules:
- Use imperative mood ("add", "fix", "remove" — not "added", "fixes")
- Reference the ticket ID where applicable
- Keep the subject line under 72 characters
- Separate subject from body with a blank line if adding detail

---

## Pull request process

1. **Link the ticket** — include `TK-XXX` in the PR title and/or description.
2. **Keep PRs small** — one concern per PR makes review faster.
3. **Pass quality gates** — `make test-go-cover` and `make lint` must pass locally before opening.
   For a GitHub-like local run, prefer `make ci-bootstrap && make ci`.
4. **Update docs** — if you change user-visible behaviour, update `USER_GUIDE.md`; if you change architecture, update `docs/DESIGN.md`.
5. **Request review** — tag a maintainer; expect a response within 2 business days.
6. **Squash or rebase** — keep a clean linear history before merging.

### PR checklist (copy into your PR description)

```
- [ ] Tests pass (`make test-all`)
- [ ] Lint passes (`make lint`)
- [ ] Ticket ID in PR title
- [ ] Documentation updated if behaviour changed
- [ ] Coverage thresholds still met
```

---

## Testing expectations

All code changes require tests. See [TESTING.md](TESTING.md) for the full test
strategy. Short version:

| Type | Location | Run with |
|------|----------|----------|
| Unit (Go) | `*_test.go` alongside source | `make test-unit` |
| Integration (Go) | `libticket/`, `internal/client/`, `internal/server/` | `make test-integration` |
| Contract | `libticket/contract_test.go` | included in integration |
| E2E (Playwright) | `tests/playwright/` | `make test-playwright` |

**Golden rule**: the contract tests in `libticket/contract_test.go` run the same
suite against both `LocalService` and `HTTPService`. If you add a `Service`
method, add a contract test for it.

Coverage thresholds (enforced in CI):

| Package | Threshold |
|---------|-----------|
| `cmd/tk` | 55% |
| `libticket` | 65% |
| `internal/client` | 55% |
| `internal/store` | 69% |
| `internal/server` | 63% |
| `internal/config` | 70% |

---

## Coding conventions

- **Error handling**: wrap with `%w`; use sentinel errors (`errors.Is`) for
  known failure modes (`ErrUnauthorized`, `ErrNotFound`, etc.)
- **Context**: pass `context.Context` as the first parameter to any function
  that does I/O.
- **Naming**: follow Go idioms — `ID` not `Id`, `URL` not `Url`; receivers
  are short (one or two letters).
- **No magic numbers**: define named constants in `constants.go`.
- **Lint**: run `make lint` before every commit. Fix all warnings.
- **Interface design**: prefer small, focused interfaces over the large
  `Service` monolith. New feature sets should define their own sub-interface.
- **SQL**: always use parameterised queries (`?` placeholders). Never
  interpolate user input into SQL strings.
