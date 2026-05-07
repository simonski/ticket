---
name: workflow
description: Use this skill when reviewing the overall codebase. Applies when the user asks for an "workflow review" - or types /workflow.  Version 0.0.2 adds a detailed role roster and report template, as well as a summary report structure and execution guidance.
metadata:
    version: 0.0.2
---

## Core Principle

Perform a full-spectrum, evidence-backed Workflow assessment from **26 distinct professional roles**. The standard is not "good enough"; it is the quality expected from a team that assumes defects will be expensive, public, and hard to reverse. Each role must think like a world-class operator whose job is to prevent avoidable failure, not merely spot nits.

This skill is intentionally adversarial, but not theatrical. The goal is to produce the most useful assessment possible: precise findings, clear ownership, strong handoffs, and actionable next steps that improve the system as a whole.

## Operating model

Treat the assessment as a coordinated review program, not a loose collection of opinions.

- Every role has a **mission**, **review objective**, **inputs**, **outputs**, and **handoffs**.
- Every claim must be backed by evidence from the codebase, configuration, documentation, tests, or generated artifacts.
- Every issue must identify the consequence if left unresolved.
- Every recommendation must be concrete enough that a capable engineer could execute it without reinterpretation.
- Roles should disagree when necessary, but they must reconcile through explicit handoffs, not duplicate work blindly.

## Output format

### Per-report template (mandatory structure)

Every role report MUST follow this structure:

```markdown
# {Role Name}

**Score: NN/100** (was NN)

## Mission
One paragraph describing what this role is protecting and what excellence looks like.

## Review objective
What this role must prove or disprove in the current system.

## Inputs reviewed
Bulleted list of the evidence sources used for this assessment.

## Findings

### Passing checks
Bulleted list of verified strengths with file:line references.

### Issues found
| Finding | Severity | Consequence | Location | Recommendation |
|---------|----------|-------------|----------|----------------|

## Required handoffs
| Handoff to | Why | Artifact / question |
|------------|-----|---------------------|

## Verdict
2-4 sentence summary.

## Changes since last assessment
Bulleted list (omit on first run).

## Remaining recommendations
| Finding | Severity | Recommendation | Owner |
|---------|----------|----------------|-------|
```

### Summary report (00-SUMMARY.md)

Must include:

- Date, project name, overall score, and assessment scope
- Score table with previous/delta columns
- Score distribution (90+, 80-89, 70-79, below 70)
- Top systemic risks across roles
- Cross-role contradiction log (where roles disagreed and how that was resolved)
- What changed since last assessment
- Cumulative improvement table (original -> current)
- Key delivery metrics table (test count, coverage, migrations, indexes, SLOs, vulnerabilities, docs gaps, etc.)
- Prioritised action register with owner role and dependency notes

## Review sequence

Use this high-discipline flow:

1. **Establish system intent** — product, user journeys, constraints, and declared architecture.
2. **Map system reality** — code structure, runtime topology, data flows, interfaces, dependencies.
3. **Assess build quality** — implementation, tests, security, performance, resilience, operations.
4. **Assess change quality** — release, incident readiness, onboarding, documentation, governance.
5. **Reconcile findings** — merge overlapping concerns, resolve contradictions, produce a ranked action plan.

## Role roster (26 roles)

### Strategy, product, and user outcome

1. **product-manager**
   - **Mission:** Ensure the software solves the right problem for the right users with the right scope.
   - **Review objective:** Verify that the delivered system matches stated goals, critical user journeys, and product boundaries.
   - **Inputs:** README, roadmap docs, tickets, CLI help, visible feature surface, user-facing errors.
   - **Outputs:** Gaps between stated intent and actual behaviour; missing journeys; unclear priorities.
   - **Handoffs:** Feeds UX, tech-writer, support-readiness, and release-manager with journey-critical gaps.

2. **user-researcher**
   - **Mission:** Protect task success, clarity, and user trust.
   - **Review objective:** Evaluate whether a real user can understand the workflow, recover from mistakes, and complete high-value tasks.
   - **Inputs:** UI flows, CLI prompts, validation messages, defaults, examples, screenshots if present.
   - **Outputs:** Friction points, ambiguity, failure-recovery weaknesses, terminology problems.
   - **Handoffs:** Sends usability blockers to ux-review, accessibility, and product-manager.

3. **ux-review**
   - **Mission:** Ensure interaction quality is coherent, legible, and efficient under normal and error conditions.
   - **Review objective:** Check consistency, feedback, loading states, validation, responsiveness, and interaction polish.
   - **Inputs:** Templates, CSS/Tailwind classes, client-side logic, form states, visual patterns.
   - **Outputs:** UI inconsistency list, feedback gaps, mobile issues, state-management concerns.
   - **Handoffs:** Escalates keyboard and semantics issues to accessibility; unclear flows to user-researcher.

4. **accessibility**
   - **Mission:** Ensure the system remains usable for people with different abilities and assistive technologies.
   - **Review objective:** Verify semantic structure, keyboard access, focus handling, labels, contrast indicators, and error announcements.
   - **Inputs:** HTML/templates, component markup, forms, interactive states, docs that describe usage.
   - **Outputs:** A11y barriers, severity by task impact, remediation guidance.
   - **Handoffs:** Sends markup and client-side fixes to frontend roles; user-impact framing to product-manager.

5. **support-readiness**
   - **Mission:** Ensure users can be helped quickly when things go wrong.
   - **Review objective:** Assess diagnosability of user-facing failures, supportability of workflows, and clarity of recovery paths.
   - **Inputs:** Error messages, logs, runbooks, FAQs, issue templates, documentation.
   - **Outputs:** Support blind spots, ambiguous failures, missing recovery instructions, triage blockers.
   - **Handoffs:** Pushes logging gaps to sre, docs gaps to tech-writer, workflow issues to product-manager.

### Architecture and design integrity

6. **systems-architect**
   - **Mission:** Protect structural integrity, bounded complexity, and evolvability.
   - **Review objective:** Verify that the codebase structure, dependency graph, runtime boundaries, and abstractions make long-term sense.
   - **Inputs:** Package/module layout, dependency DAG, interfaces, runtime composition, docs.
   - **Outputs:** Architectural risks, over-coupling, boundary leaks, missing abstractions, unbounded resources.
   - **Handoffs:** Routes implementation-level consequences to tech-lead, database, devops, and performance.

7. **api-architect**
   - **Mission:** Protect contract clarity and integration safety.
   - **Review objective:** Check API shape, naming, versioning, schema fidelity, examples, error contracts, and spec/implementation drift.
   - **Inputs:** OpenAPI/spec files, handlers, routers, generated code, examples, tests.
   - **Outputs:** Contract drift, inconsistent semantics, missing examples, weak compatibility guarantees.
   - **Handoffs:** Sends security-relevant endpoints to security-engineer and performance-critical collections to performance.

8. **domain-designer**
   - **Mission:** Ensure the software model reflects the real business domain rather than incidental implementation detail.
   - **Review objective:** Evaluate entity boundaries, invariants, state transitions, naming, and whether workflows match domain reality.
   - **Inputs:** Core types, services, business rules, validation logic, migrations, docs, tests.
   - **Outputs:** Domain mismatch findings, invariant violations, confusing naming, lifecycle holes.
   - **Handoffs:** Shares invariant expectations with backend-engineer, qa-architect, and database-engineer.

9. **tech-lead**
   - **Mission:** Protect day-to-day code health and maintainability.
   - **Review objective:** Review complexity, duplication, cohesion, naming, file size, helper reuse, and refactoring pressure.
   - **Inputs:** Source tree, diff hotspots, helper packages, errors, test structure.
   - **Outputs:** Maintainability risks and prioritised cleanup opportunities.
   - **Handoffs:** Feeds backend/frontend roles with refactors and release-manager with risky hotspots.

### Implementation quality

10. **backend-engineer**
    - **Mission:** Ensure server-side code is correct, idiomatic, explicit, and safe under load.
    - **Review objective:** Check error handling, context propagation, concurrency, state management, boundaries, and control flow.
    - **Inputs:** Go code, background jobs, handlers, services, tests, Makefile, tooling config.
    - **Outputs:** Correctness risks, goroutine/resource hazards, poor abstractions, non-idiomatic patterns.
    - **Handoffs:** Sends persistence concerns to database-engineer; runtime concerns to performance and sre.

11. **frontend-engineer**
    - **Mission:** Ensure client-side behaviour is safe, robust, and maintainable.
    - **Review objective:** Review progressive enhancement, DOM updates, fetch/HTMX behaviour, state transitions, and browser-side error handling.
    - **Inputs:** Templates, inline JS, assets, browser interactions, form handling.
    - **Outputs:** Unsafe DOM patterns, state bugs, missing error handling, brittle client logic.
    - **Handoffs:** Sends XSS and CSRF concerns to security-engineer; usability issues to ux-review.

12. **code-reviewer**
    - **Mission:** Act as the final skeptical peer who rejects weak reasoning and evidence-free changes.
    - **Review objective:** Identify places where the code appears to work but lacks proof, tests, safeguards, or clarity.
    - **Inputs:** Source code, tests, comments, PR-like change seams, generated artifacts.
    - **Outputs:** Review-level objections, unclear assumptions, insufficient evidence findings.
    - **Handoffs:** Pushes proof gaps to qa-architect, release-manager, and relevant implementation roles.

13. **maintainer**
    - **Mission:** Protect long-term ownership cost.
    - **Review objective:** Assess how hard the system will be to upgrade, debug, extend, and safely hand to the next engineer.
    - **Inputs:** Dependency layout, config sprawl, scripts, generated files, documentation, naming.
    - **Outputs:** Upgrade traps, ownership risks, brittle workflows, hidden tribal knowledge.
    - **Handoffs:** Shares onboarding pain with new-starter and docs debt with tech-writer.

### Data, security, and trust

14. **security-engineer**
    - **Mission:** Protect confidentiality, integrity, and access boundaries.
    - **Review objective:** Audit auth, authorisation, session/cookie handling, secrets, data protection, rate limiting, and secure defaults.
    - **Inputs:** Auth flows, middleware, config, secrets handling, deployment files, tests, dependency posture.
    - **Outputs:** Exploitable weaknesses, weak defaults, privilege boundary issues, missing controls.
    - **Handoffs:** Escalates exploit paths to application-security; deployment controls to devops; audit needs to compliance.

15. **application-security**
    - **Mission:** Think like an attacker and prove whether the system can be broken through realistic attack paths.
    - **Review objective:** Produce a threat model and attack-surface review covering injection, traversal, SSRF, XSS, CSRF, command execution, and abuse flows.
    - **Inputs:** Entry points, parsers, handlers, filesystem access, network calls, templates, deserialisation paths.
    - **Outputs:** Threat model table, exploit hypotheses, mitigations, residual-risk statements.
    - **Handoffs:** Hands validated control requirements back to security-engineer and performance tradeoffs to systems-architect.

16. **database-engineer**
    - **Mission:** Protect data correctness, durability, and query behaviour.
    - **Review objective:** Assess schema design, migrations, indexes, constraints, cascades, parameterisation, and operational safety of data access.
    - **Inputs:** Migrations, schema files, query code, repository layer, fixtures, backups if documented.
    - **Outputs:** Data-model defects, migration risk, missing indexes, integrity weaknesses, hot-query concerns.
    - **Handoffs:** Shares latency hotspots with performance and recovery concerns with sre.

17. **privacy-and-compliance**
    - **Mission:** Protect lawful, accountable handling of data and obligations.
    - **Review objective:** Review retention, deletion, minimisation, auditability, consent implications, licenses, and documented controls.
    - **Inputs:** Data model, logs, docs, policies, third-party dependencies, SBOM/license data.
    - **Outputs:** Compliance gaps, undocumented data flows, retention risks, audit trail weaknesses.
    - **Handoffs:** Sends operational control needs to sre/devops and documentation gaps to tech-writer.

18. **supply-chain**
    - **Mission:** Protect the project from dependency and build-system compromise.
    - **Review objective:** Assess dependency hygiene, pinning, provenance, generated code safety, vulnerability scanning, and release artifact trust.
    - **Inputs:** go.mod/go.sum, lockfiles, build scripts, CI workflows, generated code, release config.
    - **Outputs:** Dependency risk register, provenance gaps, weak scanning coverage, unsafe generation paths.
    - **Handoffs:** Routes build pipeline issues to devops and release integrity issues to release-manager.

### Verification and performance

19. **qa-architect**
    - **Mission:** Prove the system works for the important paths and fails safely for the dangerous ones.
    - **Review objective:** Evaluate test strategy, isolation, coverage shape, fixture quality, flakiness risk, and missing negative-path tests.
    - **Inputs:** Test files, helpers, mocks, CI test runs, coverage data, bug-prone code paths.
    - **Outputs:** Verification gaps, brittle test patterns, untested invariants, false-confidence risks.
    - **Handoffs:** Sends missing behavioural proof to backend/frontend roles and release risk to release-manager.

20. **performance-engineer**
    - **Mission:** Protect latency, throughput, and bounded resource usage.
    - **Review objective:** Check hot paths, N+1s, allocations, connection usage, pagination, concurrency, and scalability assumptions.
    - **Inputs:** Data access patterns, background work, HTTP handlers, caches, metrics, benchmarks if present.
    - **Outputs:** Bottlenecks, unbounded loops, poor query patterns, scale-break assumptions.
    - **Handoffs:** Shares query issues with database-engineer and capacity concerns with sre/devops.

21. **resilience-engineer**
    - **Mission:** Ensure the system degrades safely instead of collapsing messily.
    - **Review objective:** Evaluate timeouts, retries, backpressure, circuit-breaking behaviour, graceful shutdown, idempotency, and failure isolation.
    - **Inputs:** Network clients, job processors, queues, shutdown logic, retry logic, operational docs.
    - **Outputs:** Cascading-failure risks, retry storms, unsafe recovery paths, weak boundedness.
    - **Handoffs:** Feeds sre with incident scenarios and tech-lead with code hotspots that block resilience.

### Delivery and operations

22. **devops-engineer**
    - **Mission:** Protect repeatable builds, safe deployment, and correct runtime packaging.
    - **Review objective:** Review CI/CD, Dockerfiles, compose/manifests, env handling, secrets flow, and release automation.
    - **Inputs:** Makefile, Dockerfiles, compose/Kubernetes manifests, GitHub Actions, release scripts, config docs.
    - **Outputs:** Pipeline gaps, unsafe container/runtime defaults, environment drift, deployment fragility.
    - **Handoffs:** Sends runtime observability and rollback needs to sre; provenance issues to supply-chain.

23. **sre**
    - **Mission:** Protect operability in production.
    - **Review objective:** Assess observability, alertability, runbooks, backup/restore, capacity signals, SLO readiness, and incident response support.
    - **Inputs:** Metrics/logging/tracing setup, dashboards if present, health checks, runbooks, backup docs, deployment config.
    - **Outputs:** Operational blind spots, weak telemetry, missing playbooks, recovery concerns.
    - **Handoffs:** Pushes supportability issues to support-readiness and release gates to release-manager.

24. **release-manager**
    - **Mission:** Decide whether the current state should be allowed to ship.
    - **Review objective:** Reconcile all prior findings into a go/no-go view based on severity, dependencies, and rollback confidence.
    - **Inputs:** All role reports, CI/CD evidence, versioning/release process, migration risk, rollback plan, unresolved defects.
    - **Outputs:** Ship decision, release blockers, conditional approvals, post-release watchpoints.
    - **Handoffs:** Produces the final action register and informs summary scoring.

### Documentation and onboarding

25. **tech-writer**
    - **Mission:** Ensure the project can be understood, operated, and changed by someone who was not in the room when it was built.
    - **Review objective:** Assess README quality, architecture docs, setup steps, operational docs, examples, migration notes, and drift between docs and reality.
    - **Inputs:** README, CLAUDE.md, docs/, examples, inline comments, generated references, reports.
    - **Outputs:** Documentation inventory, drift findings, missing guides, ambiguous instructions.
    - **Handoffs:** Sends onboarding issues to new-starter, operational docs gaps to sre/devops, product messaging gaps to product-manager.

26. **new-starter**
    - **Mission:** Represent the engineer who joins tomorrow and has to be productive quickly without private context.
    - **Review objective:** Evaluate setup speed, reading order, local workflow clarity, ticket/process visibility, testing expectations, and common traps.
    - **Inputs:** README, contribution docs, Makefile, scripts, repo layout, docs, issue templates, CI conventions.
    - **Outputs:** Onboarding blockers, undocumented assumptions, missing reading path, day-one risks.
    - **Handoffs:** Feeds maintainer and tech-writer with fixes that reduce long-term ownership cost.

## Cross-role handoffs that must happen

These handoffs are mandatory because they turn isolated observations into a coherent assessment:

| Producer | Consumer | Required artifact |
|----------|----------|-------------------|
| product-manager | user-researcher, ux-review, tech-writer | Critical user journeys and declared product boundaries |
| systems-architect | backend-engineer, database-engineer, devops-engineer, performance-engineer | System map, key boundaries, bounded-resource inventory |
| api-architect | security-engineer, qa-architect, tech-writer | Contract inventory, drift list, example gaps |
| domain-designer | backend-engineer, qa-architect, database-engineer | Core invariants and state-transition expectations |
| security-engineer | application-security, devops-engineer, privacy-and-compliance | Control inventory and trust-boundary model |
| application-security | security-engineer, release-manager | Threat model, exploit hypotheses, residual-risk summary |
| database-engineer | performance-engineer, sre | Query/index risk list and recovery-sensitive migrations |
| qa-architect | release-manager | Evidence of what is and is not proven |
| performance-engineer | sre, devops-engineer | Capacity risks and scaling assumptions |
| resilience-engineer | sre, release-manager | Failure scenarios and degradation requirements |
| devops-engineer | sre, supply-chain, release-manager | Delivery topology, rollback path, artifact provenance notes |
| tech-writer | new-starter, support-readiness | Documentation inventory and drift map |
| all roles | release-manager | Open issues with severity, owner, and dependency notes |

## Scoring rules

- Score against demonstrated evidence, not inferred intent.
- A high score requires both **quality** and **proof**.
- A role cannot score above 85 if it found major unanswered questions in its own scope.
- A role cannot score above 90 if critical handoffs were missing or contradictory.
- A release recommendation cannot be "go" while unresolved critical findings remain in security, resilience, data integrity, or core user journeys.

## Execution guidance

- Launch parallel research agents for independent roles, but reconcile centrally.
- Every finding MUST include a file path and line number.
- Every recommendation MUST be actionable and assigned to an owner role.
- Every role report MUST distinguish verified facts from inferred risks.
- Resolve cross-role contradictions explicitly in `00-SUMMARY.md`.
- Reports go in `reports/` as `00-SUMMARY.md` and `01-26-{role}.md`.
- If prior reports exist, compare scores, findings, and closure rate rather than just listing deltas.
- The summary is the executive decision surface: concise, ranked, and impossible to misread.
