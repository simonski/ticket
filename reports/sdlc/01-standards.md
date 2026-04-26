# Standards

**Score:** 77/100 **(was 75)**

## Standard
The codebase follows explicit engineering standards that reduce avoidable defects and ownership cost.

## Assessment scope
Build/test commands, source-of-truth drift, SQL/query hygiene, and maintainability hotspots.

## Inputs reviewed
- `Makefile`
- `TESTING.md`
- `.github/workflows/makefile.yaml`
- `cmd/tk/VERSION`
- `openapi.yaml`
- `SPEC.md`
- assessment run: `wc -l cmd/tk/cmd_ticket.go cmd/tk/main_test.go web/static/index.html`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Formatting/lint/build commands documented | MUST | pass | `Makefile:1-35`; `TESTING.md:1-18` | Core commands are centralized and documented. |
| Dead/stale code avoided | MUST | partial | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | The malformed-spec issue is fixed, but versioned artifacts still drift. |
| Naming consistency | MUST | pass | `internal/store/keys.go:13-24`; `README.md:20-35` | Reviewed package and domain names remain coherent. |
| Complexity bounded in critical paths | MUST | partial | assessment run `wc -l` on 2026-04-26 | A few large files still concentrate too much ownership. |
| Parameterized DB access | MUST | pass | `internal/server/api_system.go:26-27`; `internal/server/api_system.go:47-54` | Reviewed database access uses placeholders instead of string-built SQL. |
| Shared helpers preferred | MUST | pass | `Makefile:117-167`; `tests/quickstart_test.sh:1-8` | Shared harnesses and helpers are used for repeated workflows. |
| Errors explicit rather than swallowed | MUST | pass | `internal/server/api_system.go:26-29`; `internal/store/keys.go:23-24` | Reviewed paths return explicit errors. |
| Functions/files cohesive and reasonably small | SHOULD | partial | assessment run `wc -l` on 2026-04-26 | Large ownership hotspots remain. |
| Non-obvious logic documented near code | SHOULD | partial | `internal/server/server.go:668-681`; `docs/ONBOARDING.md:219-221` | Some risky logic is documented, but not uniformly. |
| Constants/shared literals centralized where useful | SHOULD | partial | mixed reviewed evidence | No systemic failure, but not a uniformly strong practice. |
| Tooling cheaply detects violations | SHOULD | pass | `.github/workflows/makefile.yaml:25-48`; `Makefile:105-158` | Validation, tests, and linting run in CI. |

## Findings

### Strengths
- Build, validation, and testing commands are centralized and automation-friendly (`Makefile:1-35`, `Makefile:105-158`).
- The malformed OpenAPI blocker from the baseline is gone; the spec now validates cleanly through the documented gate (`openapi.yaml:1-4`, `Makefile:105-109`).
- Reviewed database access continues to rely on parameterized queries and explicit error paths (`internal/server/api_system.go:26-29`, `internal/server/api_system.go:47-54`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Large hotspot files still concentrate ownership | medium | Future changes and reviews remain riskier than they should be | assessment run: `cmd/tk/cmd_ticket.go` = 2304 lines; `cmd/tk/main_test.go` = 7268 lines; `web/static/index.html` = 7764 lines | Break up the largest CLI, test, and UI files over time. |
| Versioned source artifacts still drift | medium | The repo still has more than one answer for “what version is current” | `SPEC.md:1-4`; `openapi.yaml:1-4`; `cmd/tk/VERSION:1` | Sync version-bearing artifacts from one source of truth. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| architecture | Version and hotspot drift both affect system trust and changeability | Carry forward version-source and ownership-hotspot risks |
| technical-writing | Version-bearing artifacts are also documentation contracts | Align spec/version update workflow |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R7 | medium | Sync `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION` from one source of truth | architecture | version/source-of-truth decision | Matching versions across spec, API contract, and binary |
| R10 | low | Break up the largest CLI/test/UI hotspots | standards | none | Smaller files with equivalent behavior/tests |

## Changes since last run
- The malformed OpenAPI artifact is fixed and no longer drags this domain down (`openapi.yaml:1-4`, `Makefile:105-109`).
- Standards now improve rather than regress on test/config hygiene because the enforced coverage gate is green again (`Makefile:131-153`).

## Open questions
- Should version-bearing docs/spec files be generated, synced by tooling, or kept manual with a stricter update rule?

## Verdict
The repo’s engineering standards are stronger than the baseline because the broken public contract file is fixed and the core automation path is green again. The remaining drag is not in day-to-day code style; it is in ownership hotspots and version-bearing artifacts that still drift apart.
