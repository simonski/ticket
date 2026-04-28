# ticket

`ticket` is an issue tracking toolkit for agentic software engineering.  It is "batteries included" - providing a CLI, server, terminal UI, an agent SKILL and a REST API — backed by SQLite.

---

## Introduction

`tk` tracks engineering work through a lightweight lifecycle:

| Concept | Example |
|---------|---------|
| Project | `CUS` — Customer Portal |
| Ticket key | `CUS-42` |
| Ticket types | `epic`, `task`, `bug`, `story`, `spike`, `chore`, `note`, `question`, `requirement`, `decision` |
| Lifecycle | `stage/state` — e.g. `develop/active` |
| Stages | `design → develop → test → done` |
| States | `idle`, `active`, `success`, `fail` |

It works in two modes:

- **Local** — CLI and TUI operate directly on a SQLite file. No server required.
- **Server** — HTTP server adds multi-user auth, a web Kanban board, WebSocket live updates, and AI agent support.

The authoritative system contract is in [SPEC.md](./SPEC.md). Full user-facing documentation is in [USER_GUIDE.md](./USER_GUIDE.md). Architecture and design notes are in [docs/DESIGN.md](./docs/DESIGN.md).

## Start here

If you're new to the repo, read these first:

1. [QUICKSTART.md](./QUICKSTART.md) - choose local, server, or deployed mode
2. [docs/quickstarts/todo-example.md](./docs/quickstarts/todo-example.md) - reproducible seeded todo scenario
3. [USER_GUIDE.md](./USER_GUIDE.md) - practical CLI, server, web, and TUI reference
4. [SPEC.md](./SPEC.md) and [openapi.yaml](./openapi.yaml) - public contract
5. [docs/ONBOARDING.md](./docs/ONBOARDING.md) - contributor setup, reading order, and common pitfalls
6. [CLAUDE.md](./CLAUDE.md) - build/test commands, architecture, and package map
7. [CONTRIBUTING.md](./CONTRIBUTING.md) - branch naming, commit style, and PR expectations

Longer mode-specific guides live under [docs/quickstarts/](./docs/quickstarts/).
Operational runbooks live in [docs/RUNBOOKS.md](./docs/RUNBOOKS.md), and
historical planning notes are archived under [docs/archive/](./docs/archive/).

## Community

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- [SECURITY.md](./SECURITY.md)
- [SUPPORT.md](./SUPPORT.md)
