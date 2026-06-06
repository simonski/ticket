# Developer Guide

This is the single entrypoint for contributor and agent-facing implementation context.

## Core references

1. **Architecture / design**: `docs/DESIGN.md`
2. **Workflow + SDLC method**: `docs/process/SDLC.md`
3. **Contributor workflow**: `CONTRIBUTING.md`
4. **Testing strategy**: `TESTING.md`
5. **Agent execution rules**: `AGENTS.md`, `.github/copilot-instructions.md`, `CLAUDE.md`

## Build and validation

```bash
make setup
make build-dev
make lint
make test
make test-api
make test-browser
make test-quickstart
make test-all
```

## Documentation boundaries

The repository keeps user-facing and developer-facing docs intentionally compact:

- User-facing docs: `README.md`, `docs/QUICKSTART.md`, `docs/TUTORIAL.md`, `USER_GUIDE.md`
- Developer-facing docs: this file + architecture/process/instructions listed above

Historical plans, old quickstarts, and superseded guidance are archived under `docs/archive/`.
