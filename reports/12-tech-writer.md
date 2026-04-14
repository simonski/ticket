# Tech Writer

**Score: 81/100** (was 81)

## What is being assessed
Accuracy and maintainability of the human-facing docs set: onboarding, lifecycle, user guides, runbooks, and published API documentation.

## Methodology
Reviewed `README.md`, `QUICKSTART.md`, `USER_GUIDE.md`, `docs/ONBOARDING.md`, `docs/LIFECYCLE.md`, `docs/PRIVACY.md`, and `openapi.yaml`, then cross-checked the latest CLI/doc changes against current code.

## Findings

### Passing checks
- **There is now a clear top-level reading path** — README and ONBOARDING both expose “Start here” entry points for contributors (`README.md:33`, `docs/ONBOARDING.md:23-47`).
- **`tk skill` is documented on the main user surfaces** — the command appears in README, QUICKSTART, and USER_GUIDE with concrete usage (`README.md:205`, `QUICKSTART.md:114`, `USER_GUIDE.md:37-43`).
- **Agent-run documentation is current on the main guide surfaces** — USER_GUIDE now documents `tk agent run -id ...`, `TICKET_URL`, and the allow-list environment variable (`USER_GUIDE.md:159-186`, `USER_GUIDE.md:765`, `USER_GUIDE.md:792-987`).
- **The lifecycle docs now state the real project-prefix rule** — prefixes are documented as 1-5 uppercase letters (`docs/LIFECYCLE.md:14-15`).

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The published OpenAPI document is currently malformed at the top of the file | High | `openapi.yaml:1-10` | Repair the `info.version` entry and validate the spec as part of the docs/spec refresh workflow. |
| The privacy document still carries a stale version/date and old `ticket` command examples | Medium | `docs/PRIVACY.md:4-5`, `docs/PRIVACY.md:113-114` | Update the header metadata and switch the examples to current `tk` commands. |

## Verdict
The highest-traffic docs improved in this refresh: `tk skill` and agent-run behavior are now easy to discover, and the lifecycle docs are closer to the backend truth. The remaining documentation debt is concentrated in specialist artifacts, especially the malformed OpenAPI header and stale privacy metadata.

## Changes since last assessment
- Credited the new `tk skill` documentation across README/QUICKSTART/USER_GUIDE (`README.md:205`, `QUICKSTART.md:114`, `USER_GUIDE.md:37-43`).
- Credited the refreshed `tk agent run` flag/environment guidance in USER_GUIDE (`USER_GUIDE.md:159-186`, `USER_GUIDE.md:792-987`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Invalid OpenAPI document header | High | Restore a valid `info.version` field and add automated spec validation. |
| Stale privacy metadata/examples | Medium | Refresh `docs/PRIVACY.md` so its version/date and command examples match the current product. |
