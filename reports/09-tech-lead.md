# Tech Lead

**Score: 58/100** (was 56)

## Mission
Protect day-to-day code health by reducing concentrated complexity, duplication, and change risk in the parts of the codebase engineers touch most.

## Review objective
Identify the maintainability hotspots that most directly raise the cost of safe iteration.

## Inputs reviewed
- `cmd/tk/main.go`
- `libticket/service.go`
- `internal/client/client.go`
- `internal/tui/model_forms.go`
- `web/static/index.html`

## Findings

### Passing checks
- The TUI now uses store lifecycle constants for states and stages instead of ad-hoc state strings (`internal/tui/model_forms.go:266-268`).
- The service interface is at least decomposed into smaller named sub-interfaces before being recomposed into the main `Service` type (`libticket/service.go:12-169`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The embedded SPA is still a single massive file. | High | Small UI changes remain high-risk because HTML, CSS, and JS are interleaved in one place. | `web/static/index.html:1` | Split the frontend into smaller source units and keep the embed step as packaging, not authorship. |
| CLI command dispatch still relies on one large central switch. | Medium | Adding or modifying commands keeps increasing change blast radius in the main entrypoint. | `cmd/tk/main.go:52-118`, `cmd/tk/main.go:192-307` | Move to a command registry or grouped dispatch tables. |
| The local/remote client implementation remains a large mixed-responsibility file. | Medium | Store-backed and HTTP-backed logic are harder to review and refactor independently. | `internal/client/client.go:1-260` | Split local-mode and remote-mode operations into separate files or helpers. |
| The TUI ticket-type list still diverges from the backend’s canonical type set. | Medium | Every new domain change risks another UI/backend mismatch. | `internal/tui/model_forms.go:265-267`, `internal/store/ticket.go:1620-1627` | Derive allowed types from a shared source instead of duplicating them in the TUI. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| maintainer | These are long-term ownership hotspots as much as coding hotspots. | Refactor sequencing order. |
| frontend-engineer | The largest hotspot is in the SPA. | Frontend decomposition plan. |
| systems-architect | Main switch/service/client structure affects long-term boundaries. | Command/service restructuring options. |

## Verdict
The codebase is still workable, but the main change surfaces are too concentrated. The highest-value leadership move now is not adding more behavior into the biggest files; it is shrinking those files before they become the only place work can happen.

## Changes since last assessment
- The score improves slightly because some lifecycle constants are shared instead of ad hoc, but the largest concentration risks remain in the same files (`internal/tui/model_forms.go:266-268`, `cmd/tk/main.go:52-118`, `web/static/index.html:1`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| SPA monolith | High | Decompose the frontend into smaller maintainable units. | frontend-engineer |
| Central command switch | Medium | Replace with a registry/grouped dispatch model. | tech-lead |
| Client file size | Medium | Separate local and remote client paths. | maintainer |
