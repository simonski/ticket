# Maintainer

**Score: 64/100** (was 56)

## Mission
Protect long-term ownership cost so the next engineer can upgrade, debug, and extend the system without tribal knowledge.

## Review objective
Assess whether the repository can be safely carried forward by maintainers who were not present for the original design decisions.

## Inputs reviewed
- `README.md`
- `docs/ONBOARDING.md`
- `CONTRIBUTING.md`
- `libticket/service.go`
- `cmd/tk/main.go`
- `internal/client/client.go`
- `web/static/index.html`

## Findings

### Passing checks
- The repo does provide a real onboarding, contributing, and testing surface instead of forcing maintainers to infer process from code alone (`README.md:33-41`, `docs/ONBOARDING.md:23-47`, `CONTRIBUTING.md:27-95`).
- The service surface is at least named and grouped, which makes it easier to reason about than a flat unlabelled method list (`libticket/service.go:12-169`).

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|
| Several primary change surfaces are still concentrated in very large files. | High | Ownership and review cost remain much higher than they need to be for routine changes. | `web/static/index.html:1`, `cmd/tk/main.go:52-118`, `internal/client/client.go:1-260` | Continue splitting the SPA, command dispatch, and client logic into smaller units with narrower responsibilities. |
| Multiple docs now disagree on important facts. | Medium | Maintainers have to guess which document is current before they can change anything safely. | `CLAUDE.md:27`, `TESTING.md:124-126`, `docs/PRIVACY.md:4-5`, `README.md:64-71`, `docs/ONBOARDING.md:111-117` | Run doc-drift cleanup and make key facts single-sourced where possible. |
| The top-level `Service` interface is still broad enough to raise coupling pressure. | Medium | Any new implementation or refactor has to satisfy a wide cross-section of unrelated behavior. | `libticket/service.go:12-169` | Prefer passing sub-interfaces at call sites instead of the whole service. |

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|
| tech-lead | Most ownership cost is concentrated in code-organization decisions. | Refactor priority order. |
| tech-writer | Doc drift directly increases maintenance risk. | Single-source-of-truth cleanup list. |
| systems-architect | Service/interface breadth is also a boundary design issue. | Where should sub-interface boundaries harden? |

## Verdict
The repo is more maintainable than its worst hotspots suggest because the documentation and process scaffolding are real. The problem is concentration: a few oversized files and a few drifting docs still dominate the ownership cost curve.

## Changes since last assessment
- Ownership confidence improves because onboarding/process docs are stronger than the older reports suggested, but code/doc concentration risks remain stubborn (`README.md:33-41`, `CONTRIBUTING.md:27-95`, `web/static/index.html:1`).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
| Large-file concentration | High | Continue targeted decomposition of main change surfaces. | tech-lead |
| Doc drift | Medium | Make key operational facts single-sourced. | tech-writer |
| Broad service coupling | Medium | Prefer sub-interfaces at call sites. | systems-architect |
