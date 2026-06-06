# ticket

The ticket system `tk` is an issue tracking toolkit for agentic software engineering.  It is "batteries included" - providing a CLI, server, terminal UI, an agent SKILL and a REST API — backed by SQLite.

---

## Introduction

It operates as a client/server system:

- **Server** — the HTTP server owns the SQLite database and exposes API + web UI.
- **Client** — CLI/TUI talk to the configured server using environment
  credentials or a stored session token.

Architecture and design notes are in [docs/DESIGN.md](./docs/DESIGN.md).

## Start here

If you're new to the repo, read these first:

1. [QUICKSTART.md](./docs/QUICKSTART.md) - quick setup and daily workflow
2. [TUTORIAL.md](./docs/TUTORIAL.md) - executable end-to-end walkthrough
3. [DEVELOPER_GUIDE.md](./docs/DEVELOPER_GUIDE.md) - contributor and agent implementation context
4. [CLAUDE.md](./CLAUDE.md) - execution-focused build/test guidance

Historical and superseded documents are archived under [docs/archive/](./docs/archive/).

## Community

- [CONTRIBUTING.md](./.github/CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](./.github/CODE_OF_CONDUCT.md)
- [SECURITY.md](./.github/SECURITY.md)
- [SUPPORT.md](./.github/SUPPORT.md)
