# Developer Guide

This is the single entrypoint for contributor and agent-facing implementation context.

## Core references

1. **Architecture / design**: `docs/DESIGN.md`, `docs/ENTITY_MODEL.md`
2. **Workflow + SDLC method**: `docs/process/SDLC.md`, `docs/LIFECYCLE.md`
3. **Contributor workflow**: `.github/CONTRIBUTING.md`
4. **Testing strategy**: `docs/TESTING.md`
5. **Agent execution rules**: `docs/Agents.md`, `.github/copilot-instructions.md`, `CLAUDE.md`

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

- User-facing docs: `README.md`, `docs/QUICKSTART.md`, `docs/TUTORIAL.md`, `docs/USER_GUIDE.md`, `docs/ONBOARDING.md`
- Developer-facing docs: this file + the architecture/process/instructions listed above

Superseded guidance is removed rather than archived; the authoritative current
specs are `docs/SPEC.md`, `docs/api/openapi.yaml`, and the design docs above.
