# ticket

The ticket system `tk` is an issue tracking toolkit for agentic software engineering.  It is "batteries included" - providing a CLI, server, terminal UI, an agent SKILL and a REST API — backed by SQLite.

It operates as a client/server system:

- **Server** — the HTTP server owns the SQLite database and exposes API + web UI.
- **Client** — CLI/TUI talk to the configured server either by a human or an agent.

Architecture and design notes are in [docs/DESIGN.md](./docs/DESIGN.md).

The mission — what this program is meant to be — is captured in
[docs/FACTORY.md](./docs/FACTORY.md): a single, technology-neutral
specification of the software factory (vision, requirements, SDLC, and
worked examples), complete enough to rebuild the system from scratch.



---

## Install

```shell
# brew
brew install simonski/tap/ticket

# go
go install github.com/simonski/ticket/cmd/tk

# source
cd $CODE
git clone https://github.com/simonski/ticket.git
cd ticket
make install
```

## Start here

If you're new to the repo, read these first:

1. [QUICKSTART.md](./docs/QUICKSTART.md) - quick setup and daily workflow
2. [TUTORIAL.md](./docs/TUTORIAL.md) - executable end-to-end walkthrough
3. [DEVELOPER_GUIDE.md](./docs/DEVELOPER_GUIDE.md) - contributor and agent implementation context
4. [CLAUDE.md](./CLAUDE.md) - execution-focused build/test guidance

## Community

- [CONTRIBUTING.md](./.github/CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](./.github/CODE_OF_CONDUCT.md)
- [SECURITY.md](./.github/SECURITY.md)
- [SUPPORT.md](./.github/SUPPORT.md)
