# Readability

**Score:** 74/100 **(was 72)**

## Standard
A capable engineer can understand intent, flow, and impact without reverse-engineering the entire system.

## Assessment scope
Primary docs, main web UI affordances, secondary web drift, hotspot files, and user-visible control clarity.

## Inputs reviewed
- `README.md`
- `USER_GUIDE.md`
- `docs/LIFECYCLE.md`
- `web/static/index.html`
- `web/site2/index.html`
- `internal/store/keys.go`
- assessment run: `wc -l cmd/tk/cmd_ticket.go cmd/tk/main_test.go web/static/index.html`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Names communicate intent clearly | MUST | pass | `README.md:20-35`; `internal/store/keys.go:13-24` | Reviewed terminology remains understandable. |
| Control flow understandable | MUST | partial | assessment run `wc -l` on 2026-04-26 | Hotspot files still create mental overhead. |
| Public behavior discoverable in code/docs | MUST | pass | `USER_GUIDE.md:91-95`; `TESTING.md:138-153` | Core product behavior remains discoverable. |
| Files have coherent responsibility | MUST | partial | assessment run `wc -l`; `web/static/index.html:1279-1287` | Main flows improved, but ownership remains concentrated. |
| Errors/logs understandable | MUST | pass | `internal/server/server.go:520-548` | Request logging is still legible and actionable. |
| Comments explain why | SHOULD | partial | `internal/server/server.go:668-681` | Some risky logic is documented, but not uniformly. |
| Large files split when structure unclear | SHOULD | fail | assessment run `wc -l` on 2026-04-26 | Large ownership centers remain. |
| User-visible wording consistent across surfaces | SHOULD | partial | `web/static/index.html:1622-1623`; `web/site2/index.html:808-809`; `docs/LIFECYCLE.md:14-15` | The main UI is aligned, but `site2` still drifts. |
| Examples realistic/current | SHOULD | pass | `README.md:27-35`; `QUICKSTART_SERVER.md:13-35` | Examples are generally realistic, even if some bootstrap wording still needs hardening. |

## Findings

### Strengths
- The main web UI is clearer than the baseline: app status is a live region, project/profile controls use semantic menu buttons, and membership flows use named datalist selectors instead of raw numeric IDs (`web/static/index.html:1279-1287`, `web/static/index.html:1503-1528`, `web/static/index.html:5191-5224`).
- The backend/domain rule for project prefixes remains explicit and easy to discover (`internal/store/keys.go:13-24`, `docs/LIFECYCLE.md:14-15`, `USER_GUIDE.md:91-95`).
- Operational logs remain readable because requests are logged with request IDs, method/path, status, and duration (`internal/server/server.go:520-548`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Very large CLI/test/UI hotspots remain | medium | Future changes and reviews still concentrate risk in a few files | assessment run: `cmd/tk/cmd_ticket.go` = 2304 lines; `cmd/tk/main_test.go` = 7268 lines; `web/static/index.html` = 7764 lines | Split hotspots over time into smaller responsibility units. |
| `site2` still drifts from the documented/backend prefix rule | medium | Users can see different validation rules in different supported web surfaces | `web/site2/index.html:808-809`; `internal/store/keys.go:13-24`; `docs/LIFECYCLE.md:14-15` | Align `site2` prefix validation with the store/docs rule. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| architecture | Hotspot files and UI-rule drift affect changeability and consistency | Carry forward hotspot and `site2` drift concerns |
| technical-writing | Cross-surface rule drift is also a docs/contract accuracy issue | Align wording and validation together |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R7 | medium | Finish aligning cross-surface rule drift, especially `site2` project-prefix validation | readability | UI/spec follow-through | Matching rule enforcement across both web UIs, docs, and backend |
| R10 | low | Break up the largest CLI/test/UI hotspots | standards | none | Smaller files and equivalent behavior |

## Changes since last run
- The main web/admin surface is more discoverable and accessible than the baseline thanks to semantic menus, live regions, and named membership selectors (`web/static/index.html:1279-1287`, `web/static/index.html:1503-1528`).
- The old main-web prefix mismatch is closed; the remaining drift is now limited to `site2` (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`).

## Open questions
- Is `site2` intended to be held to the same product rule set as the main web UI, or is it intentionally experimental?

## Verdict
Readability improved because the main product surface is now more consistent and easier to operate than the baseline. The next ceiling is structural: very large hotspot files remain, and the secondary `site2` UI still breaks the otherwise clear 1-5 uppercase prefix rule.
