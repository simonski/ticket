# Infosec

**Score:** 71/100 **(was 69)**

## Standard
Information assets, secrets, dependencies, and operational trust are handled with least privilege and auditability.

## Assessment scope
Secrets in deploy artifacts, release trust, child-process env propagation, privacy-policy freshness, and bootstrap/password guidance.

## Inputs reviewed
- `deploy/compose.yaml`
- `deploy/entrypoint.sh`
- `Makefile`
- `.github/workflows/makefile.yaml`
- `internal/server/chat_ws.go`
- `docs/PRIVACY.md`
- `QUICKSTART_SERVER.md`
- `USER_GUIDE.md`

## Requirements assessed

| Requirement | Level | Status | Evidence | Notes |
|-------------|-------|--------|----------|-------|
| Secrets stored/transmitted/referenced safely | MUST | fail | `deploy/compose.yaml:6-10`; `deploy/entrypoint.sh:12-16` | The clearest current infosec weakness. |
| Sensitive data handling documented | MUST | pass | `docs/PRIVACY.md:3-40` | Privacy guidance is now current again. |
| Dependencies reviewable / critical vulnerable components visible | MUST | pass | `Makefile:169-243`; `.github/workflows/makefile.yaml:37-40` | SBOM generation and vuln scans remain in place. |
| No credential/token leakage in docs/logs/examples | MUST | partial | `QUICKSTART_SERVER.md:13-35`; `USER_GUIDE.md:85-92`; `USER_GUIDE.md:159-162` | Bootstrap docs still normalize `admin/password`. |
| Sensitive actions have clear controls/auditability | MUST | partial | `internal/server/server.go:520-548`; `internal/server/chat_ws.go:232-237` | Request auditability is good; child-process least privilege is not. |
| License/supply-chain risks understood | MUST | partial | `Makefile:169-243`; `.github/workflows/makefile.yaml:50-103` | Reviewability exists, provenance trust does not. |
| SBOM/provenance/dependency inventories | SHOULD | partial | SBOM yes, provenance/attestation no | Better than nothing, not excellent. |
| Retention/deletion expectations explicit | SHOULD | pass | `docs/PRIVACY.md:29-40` | Better than the baseline. |
| Privilege boundaries reviewed periodically | SHOULD | fail | no periodic review artifact reviewed | Still implicit. |
| Security ownership/escalation documented | SHOULD | fail | no explicit security-ownership artifact reviewed | Governance remains light. |

## Findings

### Strengths
- The repo still generates an SBOM and runs vulnerability/security scans in CI (`Makefile:169-243`, `.github/workflows/makefile.yaml:37-40`).
- Request correlation IDs and structured request logs remain useful audit material (`internal/server/server.go:520-548`).
- The privacy policy is now current to version `0.1.848` dated 2026-04-26 (`docs/PRIVACY.md:3-5`).

### Gaps

| Finding | Severity | Consequence | Evidence | Recommendation |
|---------|----------|-------------|----------|----------------|
| Committed default admin password in deploy bundle | high | Known default credentials are incompatible with least-privilege deployment practice | `deploy/compose.yaml:6-10`; `deploy/entrypoint.sh:12-16` | Remove the committed default and require explicit bootstrap secret input. |
| No artifact signing/attestation | high | Consumers still cannot verify release provenance beyond repository trust | `Makefile:169-243`; `.github/workflows/makefile.yaml:50-103` | Add release signing/attestation. |
| Full server environment passed to chat subprocess | medium | Secrets/config can leak into subordinate command execution | `internal/server/chat_ws.go:232-237` | Whitelist child env variables. |
| Bootstrap docs still normalize `admin/password` | medium | Operator-facing docs undercut the least-privilege stance the codebase should project | `QUICKSTART_SERVER.md:13-35`; `USER_GUIDE.md:85-92`; `USER_GUIDE.md:159-162` | Remove default-password guidance from quickstarts and user-facing deploy docs. |

## Required handoffs

| Consumer | Reason | Artifact or question |
|----------|--------|----------------------|
| devops | Secret handling and release trust require deployment/release changes | Carry forward deploy password and signing gaps |
| technical-writing | Bootstrap-password wording is also a docs trust issue | Harden quickstart/user-guide guidance |

## Recommendations

| ID | Priority | Recommendation | Owner area | Dependency | Evidence of completion |
|----|----------|----------------|------------|------------|------------------------|
| R3 | high | Add signing/attestation to the release path and stop relying on unsigned artifacts/images | devops | release process | Signed artifacts/images |
| R4 | high | Remove the committed deploy default password and require explicit bootstrap secret input | cyber | deploy UX | No known default credential in repo deploy bundle or docs |
| R6 | medium | Whitelist child-process env for chat/analyse subprocesses | infosec | subprocess config | Child env no longer inherits everything |

## Changes since last run
- The stale privacy-doc finding is closed; the policy document is current again (`docs/PRIVACY.md:3-5`).
- Release-provenance and bootstrap-secret risks remain the dominant infosec blockers.

## Open questions
- Should the repo explicitly separate demo bootstrap guidance from production guidance, or harden the single public path?

## Verdict
Infosec is better than the baseline because the policy/documentation layer is no longer obviously stale. The score still stays low because deploy-time credentials, unsigned releases, and broad child-process environment inheritance remain out of line with a high-trust posture.
