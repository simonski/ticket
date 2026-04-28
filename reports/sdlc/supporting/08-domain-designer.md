# Domain Designer

**Score: 76/100** (was 74)

## Mission
Ensure the software model reflects the real work domain and protects lifecycle invariants.

## Review objective
Evaluate entity boundaries, lifecycle naming, state transitions, and project/ticket rules.

## Inputs reviewed
- `README.md`
- `docs/LIFECYCLE.md`
- `docs/ENTITY_MODEL.md`
- `internal/store/ticket.go`
- `cmd/tk`

## Findings

### Passing checks
- README expresses project, ticket type, stage, and state concepts clearly (`README.md:9-18`).
- Design docs identify roles and stage-role ordering as first-class domain concepts (`docs/DESIGN.md:109-123`).
- Ticket model carries explicit lifecycle fields (`internal/store/ticket.go:19-55`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| "story" exists as both ticket type and separate story entity. | Medium | Users and maintainers can confuse generic stories with the separate story namespace. | `README.md:15`, `internal/store/story.go:10-18`, `internal/store/ticket.go:19-25` | Document the distinction or converge the domain model. |
| Current/most-recent noun behavior changes domain expectations. | Low | Empty `get` semantics may surprise users without a declared rule. | `cmd/tk/namespace_helpers.go` | Capture the rule in `USER_GUIDE.md` and help text. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-writer | Story/entity distinction needs clear docs. | Domain glossary update. |
| qa-architect | New CLI invariants need tests. | Namespace behavior tests. |

## Verdict
The lifecycle domain is coherent and improving. The remaining model risk is the overlap between typed tickets and separate story records.

## Changes since last assessment
- Idea now behaves as a real ticket type in the CLI worktree.

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Story duality | Medium | Decide whether story is a ticket type, a separate entity, or both with clear boundaries. | domain-designer |
