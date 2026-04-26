# Architecture

**Score:** 74/100 **(was 74)**

## Standard
The system structure supports change, correctness, and operations without uncontrolled coupling.

## Assessment scope
Interface boundaries, runtime composition, schema/store setup, contract artifacts, and cross-interface rule consistency.

## Inputs reviewed
- `README.md`
- `CLAUDE.md`
- `docs/LIFECYCLE.md`
- `internal/store/keys.go`
- `internal/store/schema_version.go`
- `internal/server/api_system.go`
- `openapi.yaml`
- `SPEC.md`
- `web/static/index.html`
- `web/site2/index.html`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Major components and boundaries identifiable | MUST | pass | `README.md:20-35`; `CLAUDE.md` architecture section | The four-surface model remains clear. |
| Architecture reflects actual runtime behavior | MUST | partial | `openapi.yaml:1-4`; `SPEC.md:1-4`; `cmd/tk/VERSION:1` | The malformed-spec break is fixed, but version/source drift remains. |
| Core domain invariants enforced reliably | MUST | partial | `internal/store/keys.go:13-24`; `web/static/index.html:1622-1623`; `web/site2/index.html:808-809` | The main UI is aligned; `site2` is not. |
| Data flow and ownership clear | MUST | pass | `README.md:20-35`; `internal/server/api_system.go:19-85` | Ownership remains understandable. |
| High-risk dependencies and bottlenecks visible | MUST | partial | `internal/store/schema_version.go:40-47` | SQLite boundedness is visible but still not actionably framed. |
| Material architectural decisions documented | MUST | pass | `docs/LIFECYCLE.md:10-18`; `README.md:20-35` | Decisions remain documented. |
| Interfaces separate stable contracts from volatile details | SHOULD | pass | service/store/server split remains explicit | Strong repo trait. |
| Change hotspots tracked/reduced | SHOULD | partial | assessment run `wc -l` on 2026-04-26 | Hotspot awareness exists, reduction incomplete. |
| Operational constraints inform recommendations | SHOULD | pass | `docs/RUNBOOKS.md:40-50`; `internal/store/schema_version.go:40-47` | Constraints are visible. |
| Architecture makes testing/debugging easier | SHOULD | pass | `Makefile:131-158`; `tests/quickstart_test.sh:1-8` | Shared harnesses and executable docs help. |

## Findings

### Strengths
- The repo still has a coherent multi-surface architecture centered on the same underlying service/data model.
- The OpenAPI boundary is materially healthier than the baseline because the file is now structurally valid (`openapi.yaml:1-4`).
- Core prefix validation remains explicit in the store and in the main web UI (`internal/store/keys.go:13-24`, `web/static/index.html:1622-1623`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Cross-surface invariant drift remains in `site2` | medium | The “same model across surfaces” promise is still weakened by one shipped UI allowing a different project-prefix rule | `web/site2/index.html:808-809`; `internal/store/keys.go:13-24`; `docs/LIFECYCLE.md:14-15` | Align `site2` validation with the backend/docs rule. |
| Version-bearing contract artifacts still drift | medium | The project still lacks one clear answer for current contract/spec versioning | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Sync spec and version-bearing artifacts from one source. |
| SQLite concurrency posture is explicit but not actionably framed | medium | Capacity limits remain visible but not operationally actionable for future scaling decisions | `internal/store/schema_version.go:40-47` | Document the practical ceiling and the trigger for a different storage posture. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| technical-writing | Version drift and cross-surface rules are also documentation-contract issues | Align source-of-truth workflow and surface wording |
| devops | SQLite ceiling and scaling trigger affect deployment guidance too | Clarify what “outgrowing SQLite” should mean operationally |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R7 | medium | Sync version-bearing artifacts and align `site2` project-prefix validation with the backend/docs rule | architecture | version/source-of-truth decision | Matching rules across spec, binary, and both web UIs |
| R14 | medium | Document the practical SQLite concurrency ceiling and the trigger for a different storage posture | architecture | none | Explicit capacity note in architecture/runbooks |

## Changes since last run
- The malformed OpenAPI artifact no longer weakens the architecture boundary.
- The architecture score does not rise further because the remaining issues are structural rather than accidental: secondary-surface drift, version-source ambiguity, and bounded SQLite posture.

## Open questions
- Is `site2` a supported first-class surface, or should it be treated as experimental and documented that way?

## Verdict
The architecture is still one of the better parts of the project: the boundaries are visible, the runtime model is coherent, and the docs explain the system well. The score stays flat because the remaining issues are structural - especially secondary-surface drift and unresolved version-source ambiguity - rather than day-to-day implementation mistakes.
