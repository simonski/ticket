# Systems Architect

**Score: 78/100** (was 79)

## Mission
Protect structural integrity, bounded complexity, and long-term evolvability across the CLI, server, store, and web surfaces.

## Review objective
Verify that package boundaries, runtime composition, and resource bounds still make coherent long-term sense.

## Inputs reviewed
- `README.md`
- `CLAUDE.md`
- `libticket/service.go`
- `internal/server/server.go`
- `internal/server/api_tickets.go`
- `internal/store/store.go`
- `internal/server/chat_ws.go`

## Findings

### Passing checks
- The project still has a coherent multi-interface model around one binary and one shared data model (`README.md:26-31`, `CLAUDE.md:29-45`).
- Core runtime bounds are explicit: SQLite is opened with single-connection limits and WAL mode, and the HTTP server sets finite read and idle timeouts (`internal/store/store.go:21-53`, `internal/server/server.go:34-53`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The server API layer still calls `store.*` directly instead of consistently going through `libticket.Service`. | Medium | Business rules and validation can diverge between HTTP and other callers over time. | `internal/server/api_tickets.go:20-77`, `libticket/service.go:158-169` | Either route handlers through a local service implementation or explicitly document the dual-path design as intentional. |
| Chat runtime state is still a package-level singleton. | Low | Multiple server instances in tests or future embedding scenarios remain more coupled than necessary. | `internal/server/chat_ws.go:55-67` | Inject chat runtime through server/router construction. |
| The web UI remains a single 7k+ line static file. | Low | Architectural changes to the frontend are expensive and encourage cross-cutting edits in one place. | `web/static/index.html:1` | Split the SPA into smaller JS/CSS/template units while keeping the embedded asset workflow. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-lead | The main structural risks are now maintainability hotspots. | Refactor sequencing for API/server/web boundaries. |
| backend-engineer | Dual validation paths show up as implementation risk, not only architecture risk. | Service-vs-store path recommendation. |
| frontend-engineer | The SPA structure blocks safe iterative change. | Frontend decomposition proposal. |

## Verdict
The big-picture architecture still works: shared model, bounded SQLite use, and layered runtime composition remain understandable. The main warning sign is not a broken boundary but an inconsistent one: server handlers and service abstractions still coexist without a single authoritative path.

## Changes since last assessment
- The structural picture is largely stable; the unresolved OpenAPI and frontend-monolith issues are now more consequential because the product surface is broader than it was in earlier passes (`openapi.yaml:1-10`, `web/static/index.html:1`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Dual service/store path | Medium | Consolidate server business logic behind one authoritative path. | systems-architect |
| Global chat runtime | Low | Inject runtime state instead of using a package singleton. | backend-engineer |
| Monolithic SPA file | Low | Decompose the embedded frontend into maintainable units. | frontend-engineer |
