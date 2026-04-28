# Systems Architect

**Score: 74/100** (was 74)

## Mission
Protect structural integrity, bounded complexity, and evolvability.

## Review objective
Verify package boundaries, runtime topology, and resource constraints.

## Inputs reviewed
- `docs/DESIGN.md`
- `cmd/tk`
- `internal/server`
- `internal/store`
- `libticket`
- `libtickethttp`

## Findings

### Passing checks
- Architecture explicitly describes one binary with server, CLI, TUI, and web UI (`docs/DESIGN.md:15-22`).
- Global versus repo-local configuration is documented as named remotes plus project binding (`docs/DESIGN.md:24-33`).
- SQLite access is deliberately single-connection with foreign keys and busy timeout (`internal/store/schema_version.go:40-56`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| CLI command surface remains concentrated in large files. | Medium | Review and extension cost remain high for every CLI behavior change. | `cmd/tk/cmd_ticket.go`, `cmd/tk/main_test.go` | Continue extracting namespace helpers and focused command modules. |
| Trust-boundary decisions are split across server/runtime code. | Medium | Proxy, cookie, and child-process decisions are harder to reason about together. | `internal/server/server.go:668-681`, `internal/server/chat_ws.go:232-237` | Document a runtime trust-boundary module or design note. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-lead | Refactor plan for CLI hotspots. | Module split proposal. |
| application-security | Trust boundary needs hardening. | Proxy/env boundary inventory. |

## Verdict
The overall architecture is coherent for a single-binary SQLite product. The main structural risks are concentrated CLI ownership and implicit trust-boundary design.

## Changes since last assessment
- Named remote model is now documented in design docs.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| CLI file concentration | Medium | Extract stable namespace helpers and test fixtures. | systems-architect |
