# Domain Designer

**Score: 75/100** (was 79)

## Mission
Protect the integrity of the ticketing domain so lifecycle, hierarchy, and naming rules remain consistent across implementations.

## Review objective
Verify that the documented model and the implemented model still represent the same domain.

## Inputs reviewed
- `docs/LIFECYCLE.md`
- `USER_GUIDE.md`
- `internal/store/ticket.go`
- `internal/tui/model_forms.go`

## Findings

### Passing checks
- The core lifecycle concepts remain explicit in the docs and store: stage, state, draft, archived, and complete are all separate domain concepts (`docs/LIFECYCLE.md:130-144`, `internal/store/ticket.go:153-188`).
- Same-project parenting and valid-ticket-type checks are enforced in the store, not left as convention only (`internal/store/ticket.go:165-176`, `internal/store/ticket.go:1620-1627`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The web project form violates the documented project-prefix rule. | Medium | The domain rule exists, but one first-class surface invites invalid domain input. | `docs/LIFECYCLE.md:14-15`, `USER_GUIDE.md:64-67`, `web/static/index.html:1610-1612` | Make every surface enforce the same prefix invariant. |
| The TUI ticket type picker omits valid domain types supported by the backend. | Medium | Different interfaces expose different domain vocabularies, which erodes trust in the model itself. | `internal/tui/model_forms.go:265-267`, `internal/store/ticket.go:1620-1627` | Expand the TUI picker to the full canonical set or explicitly scope it as a subset. |
| The lifecycle document still contains older CRUD-first framing and rough wording that no longer reads like a polished authoritative model. | Low | Contributors have to decide which parts are current rules and which are legacy context. | `docs/LIFECYCLE.md:5-6`, `docs/LIFECYCLE.md:177-194` | Tighten the document so it reads as a normative model rather than an in-progress note set. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| frontend-engineer | Domain invariants must hold in the SPA. | Prefix-rule alignment change. |
| tech-lead | Interface/domain drift is partly a maintainability problem. | TUI ticket-type drift fix. |
| tech-writer | The lifecycle document needs cleanup to stay authoritative. | Normative rewrite targets. |

## Verdict
The domain model is still recognizable and enforced in important places. The weaker spots are interface drift and documentation tone: when one UI omits valid types and another allows invalid prefixes, the domain starts to look negotiable.

## Changes since last assessment
- Domain enforcement in the store remains solid, but interface drift around prefix and ticket types is more visible than in earlier role-based reports (`internal/store/ticket.go:165-188`, `internal/tui/model_forms.go:265-267`, `web/static/index.html:1610-1612`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Prefix invariant drift | Medium | Enforce the documented rule everywhere. | frontend-engineer |
| TUI ticket-type drift | Medium | Bring the TUI picker up to the canonical domain type set. | tech-lead |
| Lifecycle doc polish | Low | Rewrite rough/legacy sections as normative domain guidance. | tech-writer |
