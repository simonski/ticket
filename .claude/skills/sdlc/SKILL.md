---
name: sdlc
description: Use this skill when reviewing the overall codebase. Applies when the user asks for an "sdlc review".
---

## Core Principle

Perform a full-spectrum red team assessment of this project from **17 distinct professional perspectives**. Produce a report directory (`reports/`) with one numbered file per category and an overall summary.

## Output format

### Per-report template (mandatory structure)

Every report file MUST follow this structure:

```markdown
# {Category Name}

**Score: NN/100** (was NN)

## What is being assessed
One paragraph explaining the scope and what "good" looks like for this category.

## Methodology
How the assessment was conducted (files read, patterns searched, tools used).

## Findings

### Passing checks
Bulleted list of verified items with file:line references.

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|

## Verdict
2-3 sentence summary.

## Changes since last assessment
Bulleted list (omit on first run).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
```

### Summary report (00-SUMMARY.md)

Must include:
- Date, project name, overall score
- Score table with previous/delta columns
- Score distribution (grouped by 90+, 80-89, etc.)
- What changed since last assessment
- Cumulative improvement table (original -> current)
- Key metrics table (test count, migrations, indexes, etc.)
- Remaining action items (prioritised)

## Assessment categories (17 roles)

### Engineering quality

1. **openapi** — Spec/drift analysis. Count operationIds vs routes. Check multipart fields, response schemas, examples. Verify generated code is unmodified.

2. **security** — Auth (JWT, password hashing, account lockout), access control (team-based, ForwardAuth), data protection (encryption at rest, env masking), CSRF (all state-changing endpoints), cookie security (HttpOnly, Secure, SameSite, origin binding), container security (CapDrop, PidsLimit, volumes), rate limiting (login + API), vulnerability management (govulncheck).

3. **infosec/cyber** — Threat model with attack surface table. For each surface: risk, mitigation, status. Cover: SQL injection, XSS, CSRF, path traversal, command injection, SSRF, credential stuffing, session hijacking, container escape, privilege escalation. Adopt paranoid posture. Check for non-alphanumeric character handling in all inputs.

4. **idiomatic-go** — Error handling patterns, context propagation, concurrency (goroutine bounds, channel usage, mutexes), package organisation, interface design, naming conventions, code generation approach, deprecated patterns. SDLC: Makefile, CI/CD, linting config, test patterns.

5. **idiomatic-javascript** — Inline JS quality in templates. HTMX patterns. fetch() error handling (.catch on all calls). CSRF token inclusion. innerHTML safety. Modern syntax (const/let vs var). DOM manipulation patterns.

6. **devops** — Build pipeline (Makefile targets), Docker (multi-stage, Alpine version, non-root USER, health checks), compose (resource limits on ALL services, health checks, network segmentation), CI/CD (Go version alignment, linting, vulnerability scanning, coverage threshold), secrets management, version management, release pipeline.

7. **qa** — Test count (files and functions). Package coverage map. Test isolation (t.Cleanup, t.TempDir, unique IDs). Mock quality (thread-safe, configurable). Timing patterns (polling vs sleep). Coverage threshold. Integration vs unit split. Flakiness risk.

8. **tech-lead** — File sizes (no file >700 lines). Code duplication. Error message consistency. Magic numbers. Cyclomatic complexity. Dead code. Naming conventions. Interface sizes. Helper reuse (pathParam, listParams, etc.). Refactoring opportunities.

### Architecture & design

9. **architect** — Package dependency DAG. Circular dependency check. Resource bounding (enumerate ALL bounded resources with limits). Plugin/provider patterns. Event/notification system. Reconciliation loop design. Interface abstraction quality.

10. **performance** — N+1 query detection. Unbounded resource audit. Connection pooling. Goroutine leak check. SSE scalability. Build context memory usage. Pagination on all list endpoints. Keepalive/heartbeat patterns. Query timing metrics.

11. **database** — Schema evolution (all migrations). Index coverage (list every index, flag missing ones). Foreign key cascades (verify all). Connection pool tuning. HMAC/tamper evidence. Query parameterisation audit. N+1 detection. Pagination support.

### Documentation & onboarding

12. **tech-writer** — Documentation inventory (README, CLAUDE.md, SBOM.md, docs/, reports/). Completeness scoring per document. OpenAPI example coverage. Inline comment density. Stub/draft detection. Upgrade/migration guide existence.

### Business & compliance

13. **product-owner** — Feature completeness vs stated goals (CLAUDE.md "What is Pixel"). User journey assessment (deploy, expose, SSH, team management). Error UX (are error messages user-friendly?). Missing user-facing features. Accessibility basics.

14. **compliance** — GDPR (user data purge, data retention, right to erasure). Audit trail completeness and integrity (HMAC). Cookie consent implications. Data processing documentation. License compliance (SBOM review).

### Operations

15. **sre** — Observability: metrics (Prometheus), logging (structured slog), tracing. Alerting readiness. Runbook existence. Incident response documentation. Backup/restore procedures. Capacity planning. SLA/SLO definition. Graceful degradation patterns.

16. **ux-review** — UI consistency (Tailwind classes, dark mode support). Form validation feedback. Loading states. Error display patterns. Mobile responsiveness (viewport meta, grid breakpoints). Keyboard navigation. Confirm dialogs. Flash messages.

### Onboarding

17. **new-starter** — Onboarding effectiveness assessment. Evaluate from the perspective of an engineer joining the project on day one. Check:
    - **Reading order**: Is there a clear sequence of documents to read? (README -> CLAUDE.md -> QUICKSTART -> ARCHITECTURE -> ?)
    - **Way of working**: Are branching conventions, commit message style, PR process, and code review expectations documented?
    - **Development setup**: Can a new developer go from clone to running tests in under 10 minutes? Check `make setup`, `make test`, `make dev` flow.
    - **Ticket workflow**: Where do tickets live? How to pick up work? How to mark work complete? (check for `tk` tool, backlog/, .ticket/)
    - **Testing expectations**: Are test conventions documented? When to write tests? What patterns to follow?
    - **Collaboration**: How to communicate decisions? Where to document architecture changes? How to update SBOM?
    - **Common pitfalls**: What are the gotchas a new starter would hit? (e.g. test DB must be running, `make generate` after OpenAPI changes, `make css` after Tailwind changes)
    - **Recommendation**: Produce a `docs/ONBOARDING.md` if one doesn't exist, covering the above.

## Execution guidance

- Launch parallel research agents for independent categories.
- Every finding MUST include a file path and line number.
- Every recommendation MUST be actionable (not "consider" or "maybe").
- Scores should be justified by specific findings, not vibes.
- Delta tracking: always compare against previous assessment if reports/ exists.
- Reports go in `reports/` directory, numbered 00-17.
- Use the mandatory template structure above for consistency.
- The summary (00-SUMMARY.md) is the executive view — keep it scannable.
