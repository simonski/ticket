# Product Manager

**Score: 83/100** (was 85)

## Mission
Protect product fit, journey completeness, and scope discipline so the tool solves the real day-to-day tracking problem without making users stitch together hidden rules.

## Review objective
Verify that the shipped CLI, server, and web workflows still match the declared product shape and the highest-value user journeys.

## Inputs reviewed
- `README.md`
- `USER_GUIDE.md`
- `docs/ONBOARDING.md`
- `docs/LIFECYCLE.md`
- `cmd/tk/cmd_ticket_lifecycle.go`
- `cmd/tk/cmd_ticket_health.go`
- `web/static/index.html`

## Findings

### Passing checks
- The product surface is clearly framed as one binary with local and server modes plus CLI, web, and TUI interfaces (`README.md:26-31`, `README.md:83-158`).
- The repo still supports meaningful workflow improvements in the CLI, including multi-ID draft/undraft and richer health context (`cmd/tk/cmd_ticket_lifecycle.go:262-323`, `cmd/tk/cmd_ticket_health.go:218-246`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| The web project form accepts 8-character prefixes while the documented and user-guide rule is 1-5 uppercase letters. | Medium | A user can start a key workflow in the UI and fail later against backend/domain expectations. | `web/static/index.html:1610-1612`, `docs/LIFECYCLE.md:14-15`, `USER_GUIDE.md:64-67` | Clamp the UI to the same 1-5 uppercase rule and show the same validation copy everywhere. |
| The ticket modal still presents a generic manual health control while the CLI now computes a 10-check score. | Medium | Users get two competing product stories for the same concept and may not trust either. | `web/static/index.html:1897-1905`, `cmd/tk/cmd_ticket_health.go:218-246` | Decide whether health is computed, editable, or both, then align the web copy and control shape with that decision. |
| The main README has no clear “get help/report an issue” path. | Low | When a critical journey fails, users are left without a visible escalation path from the main entry point. | `README.md:1-237` | Add a short support section that points to issue filing and operator docs. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| ux-review | UI rules diverge from product rules. | Should health remain editable in the UI? |
| tech-writer | Main entry docs need a support path and validation rule consistency. | README/help section update plus prefix rule wording. |
| frontend-engineer | Product rules must land in the SPA. | Prefix validation + health control alignment. |

## Verdict
The core product shape is still coherent and useful, and the CLI continues to improve. The biggest remaining product issue is inconsistency: users encounter different rules and explanations depending on whether they use the CLI or the web UI.

## Changes since last assessment
- CLI lifecycle ergonomics improved, but the web surface did not catch up to the newer health semantics (`cmd/tk/cmd_ticket_lifecycle.go:262-323`, `cmd/tk/cmd_ticket_health.go:218-246`, `web/static/index.html:1897-1905`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Prefix validation drift | Medium | Align UI and backend validation to the same 1-5 uppercase rule. | frontend-engineer |
| Health model ambiguity | Medium | Publish one authoritative product rule for ticket health and implement it consistently. | product-manager |
| Missing help path | Low | Add a visible support/reporting path to README. | tech-writer |
