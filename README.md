# ticket

`ticket` is an issue tracking toolkit for agentic software engineering.  It is "batteries included" - providing a CLI, server, terminal UI, an agent SKILL and a REST API — backed by SQLite.

---

## Introduction

`tk` tracks engineering work through a lightweight lifecycle:

| Concept | Example |
|---------|---------|
| Project | `CUS` — Customer Portal |
| Ticket key | `CUS-T-42` |
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

1. [QUICKSTART.md](./QUICKSTART.md) - choose local or server mode
2. [QUICKSTART_TODO_EXAMPLE.md](./QUICKSTART_TODO_EXAMPLE.md) - reproducible seeded todo scenario
3. [docs/ONBOARDING.md](./docs/ONBOARDING.md) - setup, reading order, and common pitfalls
4. [CLAUDE.md](./CLAUDE.md) - build/test commands, architecture, and package map
5. [CONTRIBUTING.md](./CONTRIBUTING.md) - branch naming, commit style, and PR expectations

