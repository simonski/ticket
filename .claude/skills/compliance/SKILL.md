---
name: compliance
description: Use this skill when performing a full-codebase compliance review. Applies when the user wants to check whether the entire repository meets its documented Workflow, contribution, security, documentation, and release expectations.
metadata:
    version: 0.0.1
---

# Compliance Review Skill

Run a disciplined, evidence-backed compliance review against this repository's
own documented rules. This skill is meant to be run by a user who wants to
check whether the **entire codebase** complies with the standards explained in
the documentation. The goal is not legal theater or generic policy prose. The
goal is to determine whether the codebase, tests, CI, docs, templates,
artifacts, and workflows actually comply with the expectations set by
`docs/process/Workflow.md`, `CONTRIBUTING.md`, and the repository-facing support documents.

## Core Principle

**Compliance means proved alignment between stated policy and repo reality.**

- A policy that exists only in prose does not count as implemented.
- A workflow that exists in code but is undocumented is still a compliance gap.
- Missing evidence lowers confidence and must be called out explicitly.
- Findings must identify both the broken rule and the practical consequence.

## Primary source documents

Read these first:

1. `docs/process/Workflow.md`
2. `CONTRIBUTING.md`
3. `SECURITY.md`
4. `CODE_OF_CONDUCT.md`
5. `SUPPORT.md`
6. `TESTING.md`
7. `README.md`
8. `.github/PULL_REQUEST_TEMPLATE.md`
9. `.github/ISSUE_TEMPLATE/*`

When the review touches delivery or contract controls, also read:

- `CLAUDE.md`
- `docs/ONBOARDING.md`
- `SPEC.md`
- `openapi.yaml`
- `.github/workflows/*`
- `Makefile`

## What this skill is checking

This skill reviews whether the repository complies with its own stated
expectations across five areas:

1. **Repository governance** — community files, issue/PR templates, contribution flow
2. **Security process** — private vulnerability reporting, safe defaults, security documentation
3. **Engineering controls** — testing, linting, coverage, build/release guidance
4. **Documentation integrity** — README/quickstarts/guides/specs agree with implementation
5. **Workflow evidence** — the repo can support the kind of evidence-backed assessment required by `docs/process/Workflow.md`

Do not treat this as a docs-only review. Inspect the whole repository.

## Review sequence

Follow this order:

1. **Read the policy surface** — understand the stated rules before judging compliance.
2. **Map the entire implementation surface** — source code, tests, workflows, Make targets, templates, scripts, generated artifacts, and docs.
3. **Check for rule-to-reality alignment** — every important requirement should map to real codebase evidence.
4. **Check for missing controls** — policies with no implementation, or implementation with no documented rule.
5. **Produce a prioritized register** — sort by consequence, not by aesthetics.

## Mandatory compliance checks

### 1. Community and governance

Verify that the repository has and uses:

- `README.md`
- `LICENSE`
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `SECURITY.md`
- `SUPPORT.md`
- issue templates
- a pull request template

Assess whether these are:

- present
- coherent
- mutually consistent
- linked from the README or contribution flow where appropriate

### 2. Contribution process compliance

Check whether the repository supports the process described in
`CONTRIBUTING.md`:

- testing expectations are runnable
- documented commands actually exist
- branch / PR / ticket guidance is internally consistent
- high-risk change checklist aligns with real files and workflows
- docs-update requirements point to real documents

### 3. Security and disclosure compliance

Check whether `SECURITY.md` matches repository reality:

- private reporting path exists and is described clearly
- public issue templates do not encourage public disclosure of vulnerabilities
- security-sensitive workflows point to the right channels
- security-related docs do not contradict each other

### 4. Workflow methodology compliance

Use `docs/process/Workflow.md` as the governing standard for evidence and reporting discipline.
Assess whether the repo supports that methodology by checking:

- required reports and report locations exist where expected
- score history persistence rules can actually be followed
- the repo contains enough evidence sources to support scored claims
- documented controls can be traced to code, tests, CI, docs, or templates

### 5. Engineering gate compliance

Confirm that documented gates and commands are real and aligned:

- `make build`, `make build-dev`, `make test`, `make test-go-cover`, `make lint`
- documented coverage thresholds
- testing docs align with actual package layout
- scripts and helpers do not reference removed packages or obsolete workflows
- docs do not instruct users to use deprecated commands

## Output format

Use this structure for a compliance review:

```markdown
# Compliance Review

**Score:** NN/100

## Scope
- Confirm that the whole repository was reviewed, not just the documentation
- Summarise the code, tests, CI/CD, scripts, templates, and docs that were examined

## Policy sources
- Files used as the governing standard

## Compliance matrix
| Area | Requirement | Status | Evidence | Consequence | Recommendation |
|------|-------------|--------|----------|-------------|----------------|

## Passing controls
- Evidence-backed controls that are present and working

## Gaps
| Finding | Severity | Broken rule | Evidence | Consequence | Recommendation |
|---------|----------|-------------|----------|-------------|----------------|

## Drift log
- policy says X, repo does Y
- repo implements Z, but docs do not say so

## Required follow-ups
| Owner area | Action | Evidence of completion |
|------------|--------|------------------------|

## Verdict
2-4 sentences on current compliance posture.
```

## Severity model

Use this scale:

- **Critical** — the repo claims a control exists, but the gap creates material security, release, or governance risk
- **High** — important compliance expectation is missing, stale, or contradicted
- **Medium** — control exists but is incomplete, weakly evidenced, or inconsistently applied
- **Low** — wording, discoverability, or polish issue with limited operational risk

## Repo-specific guidance

When reviewing this repository, treat these as especially important:

- `make build-dev` is the development build path; docs should not direct people to direct ad-hoc build commands
- `make build` is allowed, but it bumps `cmd/tk/VERSION`
- the canonical contract test suite is `libticket/contract_test.go`
- remote-mode HTTP client references belong to `internal/client`, not removed packages
- public contract changes should stay aligned across `SPEC.md`, `USER_GUIDE.md`, and `openapi.yaml`
- user-visible workflow changes should be reflected in quickstarts and repo-facing docs

## What not to do

- Do not invent external compliance frameworks unless the repo explicitly claims them.
- Do not mark a requirement as passing without concrete file or command evidence.
- Do not confuse style preferences with compliance failures.
- Do not bury governance or security gaps under minor wording nits.

## Completion standard

A strong compliance review:

- names the governing rule
- cites the real evidence
- explains the risk of non-compliance
- recommends a concrete fix
- distinguishes implemented controls from aspirational documentation
