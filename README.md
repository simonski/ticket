# ticket (`tk`)

An issue tracker for agentic software engineering.

One Go binary — CLI, terminal UI, HTTP server, REST API, and an agent skill — backed by SQLite.

```shell
tk new "Fix the login bug"   # create a ticket
tk ls                        # list open tickets
tk get TK-1                  # view one
```

## Install

```shell
brew install simonski/tap/ticket        # Homebrew
go install github.com/simonski/ticket/cmd/tk@latest   # Go
```

Or from source: `git clone https://github.com/simonski/ticket.git && cd ticket && make install`.

## Learn more

- [Quickstart](./docs/QUICKSTART.md) — setup and the daily workflow
- [Tutorial](./docs/TUTORIAL.md) — an executable end-to-end walkthrough
- [Design](./docs/DESIGN.md) — architecture and data model

Full docs index: [docs/INDEX.md](./docs/INDEX.md). Contributing or building agents? See the [Developer Guide](./docs/DEVELOPER_GUIDE.md).

## Community

[Contributing](./.github/CONTRIBUTING.md) ·
[Code of Conduct](./.github/CODE_OF_CONDUCT.md) ·
[Security](./.github/SECURITY.md) ·
[Support](./.github/SUPPORT.md)
