# SDLC Methodology

This document defines the standard for running a repeatable SDLC assessment of this project. It is intentionally stricter than "good". The benchmark is professional excellence backed by evidence, not a vague sense that the project is probably fine.

This methodology is for **writing the assessment**, **structuring the outputs**, and **scoring progress across repeated runs**. It does **not** perform the assessment itself.

## Relationship to the SDLC skill

This methodology defines the **stable reporting contract** for future SDLC runs.

- The existing SDLC skill may assess the repository through many specialist lenses.
- Those specialist findings MUST be consolidated into the nine domains in this document.
- If a future SDLC run uses more granular role reports, those role reports are supporting evidence, not a replacement for the domain reports required here.
- `docs/process/SDLC.md` is the governing methodology; the skill implementation must conform to it.

## 1. Objectives

The SDLC assessment must:

1. assess the project across the full delivery lifecycle, not only code style
2. score the project against explicit requirements
3. produce a stable set of reports under `./reports/sdlc/`
4. preserve prior summary scores between runs so progress can be tracked
5. produce recommendations that are specific, evidence-backed, and actionable

## 2. Standard of judgement

The standard is **distance from excellence**, not distance from failure.

- A project is not "passing" because it basically works.
- A high score requires both strong implementation and proof that it is strong.
- Missing evidence lowers confidence and therefore lowers score.
- A weak area cannot be offset by a polished area if it introduces material risk.

## 3. Required output structure

Each SDLC run must write or update the following files under `./reports/sdlc/`:

```text
reports/sdlc/
  00-SUMMARY.md
  01-standards.md
  02-testing.md
  03-cyber.md
  04-readability.md
  05-devops.md
  06-qa.md
  07-infosec.md
  08-architecture.md
  09-technical-writing.md
  10-RECOMMENDATIONS.md
  history.json
```

### Output requirements

- `00-SUMMARY.md` is the executive view.
- `01-09` are domain reports, one per assessment area.
- `10-RECOMMENDATIONS.md` is the consolidated action register.
- `history.json` is the machine-readable score history and **must persist between runs**.
- If a future SDLC run generates specialist or role-level reports, they SHOULD be written under `reports/sdlc/supporting/` so the stable top-level contract remains unchanged.

## 4. Summary requirements

`reports/sdlc/00-SUMMARY.md` MUST include:

- methodology version or reference to `docs/process/SDLC.md`
- assessment date
- project name
- scope reviewed
- overall score
- previous overall score
- score delta
- per-area score table
- score bands
- top systemic risks
- cross-domain contradiction log
- key delivery metrics
- notable improvements since last run
- notable regressions since last run
- prioritized recommendation list
- cumulative score history

## 5. Score persistence requirements

Score history must survive future runs of the SDLC agent skill.

### Persistence rules

- `reports/sdlc/history.json` MUST be created on the first run.
- Future runs MUST append a new entry rather than overwrite prior history.
- `00-SUMMARY.md` MUST show the current score and at least the previous score and delta.
- If prior history exists, the summary MUST include a cumulative history table.
- If no prior history exists, the summary MUST explicitly label the run as the baseline.

### Required `history.json` shape

```json
{
  "version": 1,
  "runs": [
    {
      "date": "2026-04-26",
      "overall_score": 72,
      "areas": {
        "standards": 70,
        "testing": 76,
        "cyber": 68,
        "readability": 74,
        "devops": 71,
        "qa": 75,
        "infosec": 69,
        "architecture": 73,
        "technical-writing": 72
      },
      "recommendation_counts": {
        "critical": 0,
        "high": 3,
        "medium": 8,
        "low": 5
      },
      "summary": "Baseline run."
    }
  ]
}
```

### History maintenance rules

- `version` MUST be present so the format can evolve safely.
- New fields MAY be added, but old runs MUST remain readable.
- A run entry SHOULD include recommendation counts by severity to make trend reporting easier.
- If a recommendation tracking model exists, summary reports SHOULD show closure rate since the previous run.

## 6. Scoring model

Every area is scored from `0-100`.

### Requirement scoring

Each requirement in an area must be scored as one of:

- `pass`
- `partial`
- `fail`
- `not-applicable`

### Weighting

- `MUST` requirements carry a weight of `5`
- `SHOULD` requirements carry a weight of `2`
- `not-applicable` items are excluded from the denominator

### Formula

For each requirement:

- `pass = 1.0`
- `partial = 0.5`
- `fail = 0.0`

Area score:

```text
score = 100 * weighted_points_earned / weighted_points_possible
```

Overall score:

```text
overall = average of the 9 area scores, rounded to whole numbers
```

### Score caps and gating rules

- A domain with unresolved `critical` findings MUST be capped at `69`.
- A domain with unresolved `high` findings SHOULD be capped at `79` unless the residual risk is explicitly justified.
- The overall score MUST be capped at `79` if any of `cyber`, `infosec`, `testing`, or `architecture` has unresolved `critical` findings.
- The overall score MUST NOT be presented without the count of `critical` and `high` recommendations.

### Score bands

| Band | Meaning |
|------|---------|
| 90-100 | excellent, low unmanaged risk |
| 80-89 | strong, but still with meaningful gaps |
| 70-79 | workable, below target standard |
| 60-69 | materially weak |
| 0-59 | high risk / poor control |

## 7. Evidence rules

Every scored claim MUST be backed by evidence.

Acceptable evidence includes:

- source code
- tests
- CI/CD workflows
- deployment manifests
- documentation
- generated artifacts
- runtime configuration
- security tooling output
- lint/build/test output

Evidence should cite concrete locations wherever possible, for example:

- `cmd/tk/main.go:101-145`
- `Makefile:90-127`
- `.github/workflows/ci.yaml:1-88`

### Evidence sufficiency rules

- A requirement cannot be marked `pass` based only on intention, TODOs, or aspirational documentation.
- A domain report SHOULD distinguish between verified strengths, inferred risks, and open questions.
- Where a requirement spans code and operations, the report SHOULD cite both implementation evidence and operational evidence if available.

## 8. Reporting template for each domain

Each domain report (`01-09`) MUST use this structure:

```markdown
# {Area}

**Score:** NN/100 **(was NN, +/-NN)**

## Standard
Short statement of what excellence looks like in this area.

## Assessment scope
What part of the system this domain reviewed in the current run.

## Inputs reviewed
- Files, commands, artifacts, configs, and tests examined

## Requirements assessed
| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|

## Findings

### Strengths
- Evidence-backed strengths

### Gaps
| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|

## Required handoffs
| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|

## Recommendations
| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|

## Changes since last run
- Improvements
- Regressions

## Open questions
- Unproven or ambiguous areas that affected confidence

## Verdict
2-4 sentence summary.
```

## 9. Assessment workflow

Each SDLC run SHOULD follow this order:

1. establish project intent from docs, specs, and user-facing flows
2. map current implementation and operational reality
3. assess each domain against its MUST and SHOULD requirements
4. score each domain
5. reconcile overlap, contradictions, and handoffs across domains
6. consolidate recommendations and de-duplicate overlap
7. compare against the previous run and calculate deltas
8. update `history.json`
9. update `00-SUMMARY.md`

## 10. Domain ownership and overlap rules

The domains are complementary and must not collapse into each other.

| Domain | Primary concern | What it must not be reduced to |
|--------|------------------|--------------------------------|
| standards | engineering consistency and maintainability discipline | only style or lint noise |
| testing | direct proof that behavior works | generic process quality |
| cyber | attacker-facing and exploit-oriented security posture | generic documentation or supply-chain review |
| readability | human understandability and clarity | only comments or naming nits |
| devops | build, release, deploy, and runtime operations | only CI syntax or Docker formatting |
| qa | quality-system design, proof boundaries, and gate discipline | raw test count |
| infosec | secrets, data handling, dependency trust, and operational control | only app-layer exploits |
| architecture | system boundaries, invariants, and structural fitness | code style or file organization alone |
| technical-writing | correctness and usefulness of docs for users and operators | README grammar alone |

Where a finding belongs to multiple domains, the report that owns the root cause SHOULD carry the primary recommendation and other domains SHOULD cross-reference it rather than duplicate it.

## 11. Assessment domains

The SDLC assessment consists of nine required domains.

---

## 11.1 Standards

**Standard:** The codebase follows explicit engineering standards that reduce avoidable defects and ownership cost.

### MUST

1. The project MUST use and document consistent formatting, linting, and build commands.
2. The project MUST avoid dead code, stale code paths, and unused dependencies unless explicitly justified.
3. The project MUST use consistent naming for packages, functions, variables, flags, files, and user-facing terms.
4. The project MUST keep complexity bounded in critical paths; high cyclomatic complexity must be justified or reduced.
5. The project MUST use parameterized database access and avoid raw string-built SQL where injection is possible.
6. The project MUST prefer shared helpers and patterns over repeated custom implementations of the same behavior.
7. The project MUST make errors explicit rather than silently swallowing failures.

### SHOULD

1. Functions and files SHOULD remain cohesive and reasonably small.
2. Non-obvious logic SHOULD be documented close to the code.
3. Constants and shared literals SHOULD be centralized where that improves consistency.
4. Tooling SHOULD make standard violations cheap to detect.

### Typical evidence

- lint config
- Makefile / scripts
- duplicate logic
- complexity hotspots
- SQL construction patterns
- unused packages or files

---

## 11.2 Testing

**Standard:** The project proves behavior with reliable automated tests rather than confidence by inspection.

### MUST

1. Critical user and system paths MUST be covered by automated tests.
2. Bug fixes MUST add or update regression tests where practical.
3. Tests MUST be deterministic and isolated.
4. CI MUST execute the relevant automated test suites.
5. Test failures MUST fail the build or clearly fail the quality gate.
6. Test fixtures and harnesses MUST reflect the supported product behavior, not stale assumptions.

### SHOULD

1. Coverage thresholds SHOULD exist for key packages or layers.
2. Tests SHOULD cover success, failure, and edge-case behavior.
3. Documentation examples SHOULD be executable or validated.
4. Test suites SHOULD be fast enough to run regularly in local development.

### Typical evidence

- unit/integration/e2e tests
- coverage gates
- doc-test harnesses
- flaky test patterns
- CI test stages

---

## 11.3 Cyber

**Standard:** The project is resilient against realistic attack paths and insecure operating conditions.

### MUST

1. Authentication and authorization boundaries MUST be explicit and tested.
2. Untrusted input MUST be validated, constrained, encoded, or rejected appropriately.
3. The project MUST assess common attack classes relevant to the stack, including injection, XSS, CSRF, SSRF, path traversal, and command execution.
4. Sensitive endpoints and privileged actions MUST fail safely.
5. Security-relevant configuration MUST default to the safer option.
6. Known critical vulnerabilities MUST block a high score.

### SHOULD

1. Threat modeling SHOULD exist for major trust boundaries.
2. Abuse cases SHOULD be covered in tests where feasible.
3. Security tooling SHOULD run in CI or a documented release process.
4. Residual risks SHOULD be recorded rather than left implicit.

### Typical evidence

- auth middleware
- handler validation
- templating and encoding
- threat-model notes
- gosec / vuln scanning
- privileged workflows

---

## 11.4 Readability

**Standard:** A capable engineer can understand intent, flow, and impact without reverse-engineering the entire system.

### MUST

1. Names MUST communicate intent clearly.
2. Control flow MUST be understandable without excessive mental branching.
3. Public behavior and surprising edge cases MUST be discoverable in code or docs.
4. Files MUST have coherent responsibility rather than being arbitrary dumping grounds.
5. Error messages and logs MUST be understandable to a maintainer or operator.

### SHOULD

1. Comments SHOULD explain why, not restate the code.
2. Large files SHOULD be split when structure or ownership is unclear.
3. User-visible wording SHOULD be consistent across CLI, API, UI, and docs.
4. Examples SHOULD use realistic names and commands.

### Typical evidence

- file size and cohesion
- naming consistency
- comments and docstrings
- user-facing output
- onboarding friction

---

## 11.5 DevOps

**Standard:** The project can be built, shipped, configured, observed, and recovered with disciplined operational practice.

### MUST

1. Builds MUST be reproducible from documented commands.
2. Deployment artifacts MUST be versioned and understandable.
3. CI/CD workflows MUST be visible, repeatable, and relevant to the shipped system.
4. The runtime MUST expose enough health and logging information for basic diagnosis.
5. Backup, restore, rollback, or recovery expectations MUST be documented where state exists.
6. Secrets MUST not be hardcoded into deployment artifacts.

### SHOULD

1. Container images and release artifacts SHOULD be signed or attestable.
2. Environments SHOULD be configurable without source changes.
3. Infrastructure assumptions SHOULD be documented.
4. Local developer workflows SHOULD resemble production behavior where practical.

### Typical evidence

- Dockerfiles and compose
- GitHub Actions
- release scripts
- health endpoints
- runbooks
- deployment docs

---

## 11.6 QA

**Standard:** Quality is managed as a system, not left to ad hoc manual checking.

### MUST

1. Acceptance criteria MUST be testable.
2. Regressions MUST be traceable to tests, checks, or clearly identified gaps.
3. The project MUST distinguish between unit, integration, and end-to-end confidence where relevant.
4. Quality gates MUST be clear enough that contributors know what proves a change.
5. Flaky or unreliable checks MUST be treated as defects.
6. The assessment MUST identify what is proven and what is only assumed.

### SHOULD

1. Test plans SHOULD be visible for high-risk changes.
2. Quality metrics SHOULD be trended over time.
3. Manual test steps SHOULD be documented when automation is not realistic.
4. Review checklists SHOULD exist for risky areas.

### Typical evidence

- TESTING.md
- harness scripts
- release gates
- flaky checks
- review patterns
- defect-to-regression linkage

---

## 11.7 Infosec

**Standard:** Information assets, secrets, dependencies, and operational trust are handled with least privilege and auditability.

### MUST

1. Secrets MUST be stored, transmitted, and referenced safely.
2. Sensitive data handling MUST be documented where the system stores or emits it.
3. Dependencies MUST be reviewable, and critical vulnerable components MUST be visible.
4. The project MUST avoid leaking credentials or sensitive tokens in docs, logs, tests, or examples.
5. Access to operationally sensitive actions MUST have clear controls and audit trails where applicable.
6. License and supply-chain risks MUST be understood well enough to avoid accidental exposure.

### SHOULD

1. SBOMs, provenance, or dependency inventories SHOULD exist.
2. Retention and deletion expectations SHOULD be explicit for stored data.
3. Privilege boundaries SHOULD be reviewed periodically.
4. Security ownership and escalation paths SHOULD be documented.

### Typical evidence

- credential storage patterns
- logs and examples
- dependency manifests
- SBOMs
- license docs
- admin flows

---

## 11.8 Architecture

**Standard:** The system structure supports change, correctness, and operations without uncontrolled coupling.

### MUST

1. Major components and boundaries MUST be identifiable.
2. The architecture MUST reflect actual runtime behavior, not an outdated diagram.
3. Core domain invariants MUST be enforced somewhere reliable.
4. Data flow and ownership MUST be clear across major subsystems.
5. High-risk dependencies and bottlenecks MUST be visible.
6. Architectural decisions that materially affect implementation MUST be documented.

### SHOULD

1. Interfaces SHOULD separate stable contracts from volatile details.
2. Change hotspots SHOULD be tracked and reduced over time.
3. Operational constraints SHOULD inform architectural recommendations.
4. The architecture SHOULD make testing and debugging easier, not harder.

### Typical evidence

- design docs
- package/module boundaries
- service interfaces
- lifecycle rules
- hotspots
- runtime topology

---

## 11.9 Technical writing

**Standard:** Documentation helps the next user, operator, and contributor succeed without tribal knowledge.

### MUST

1. Core setup and usage docs MUST match the current implementation.
2. Important workflows MUST have a clear entry point.
3. Commands, flags, examples, and filenames MUST be accurate.
4. Docs MUST state assumptions, prerequisites, and failure recovery where relevant.
5. Release-significant changes MUST be reflected in change-facing docs.
6. Operator-facing documentation MUST exist for production-relevant behavior.

### SHOULD

1. Quickstarts SHOULD be executable.
2. Reference docs SHOULD avoid duplication that drifts.
3. Docs SHOULD distinguish between beginner guidance and deep reference.
4. Examples SHOULD use realistic scenarios and current terminology.

### Typical evidence

- README
- QUICKSTART docs
- user guide
- runbooks
- changelog
- command help

## 12. Recommendation rules

Recommendations must be:

- specific
- evidence-backed
- ranked
- assigned to an owner area
- written so an engineer can act without reinterpretation

Each recommendation in `10-RECOMMENDATIONS.md` MUST include:

- ID
- priority
- domain
- finding
- recommendation
- owner
- dependency, if any
- status
- expected risk reduction
- suggested evidence of completion

`10-RECOMMENDATIONS.md` SHOULD order recommendations by:

1. severity
2. breadth of impact
3. dependency order
4. effort-to-risk-reduction ratio

Recommendations carried over from prior runs SHOULD preserve their IDs so closure can be tracked over time.

## 13. Severity model

Every gap should be tagged with one severity:

| Severity | Meaning |
|----------|---------|
| critical | likely to cause serious security, data, release, or reliability failure |
| high | material weakness with real delivery or operational risk |
| medium | meaningful gap that reduces confidence or maintainability |
| low | worthwhile improvement with limited immediate risk |

## 14. Non-negotiable assessment rules

1. Every domain MUST assess all listed MUST and SHOULD requirements.
2. A requirement cannot be marked `pass` without evidence.
3. Missing evidence should normally score `partial` or `fail`, not `pass`.
4. A domain with unresolved critical findings SHOULD NOT score above `69`.
5. A domain with unresolved high-severity findings SHOULD NOT score above `79` unless the residual risk is explicitly justified.
6. The overall score MUST NOT hide severe weaknesses in cyber, infosec, testing, or architecture.
7. Summary language MUST be plain and difficult to misread.
8. The summary MUST state whether the current trend is improving, flat, or regressing.
9. Recommendations SHOULD be stable enough across runs that progress can be compared meaningfully.

## 15. What this document does not do

This document does not run the assessment. It defines:

- what to assess
- how to score it
- where to write the outputs
- how to preserve score history between runs

The actual SDLC review should be executed separately using this document as the governing methodology.
