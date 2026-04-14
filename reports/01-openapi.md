# OpenAPI

**Score: 74/100** (was 78)

## What is being assessed
The quality and trustworthiness of `openapi.yaml`: whether the documented paths match the running API, whether the document parses cleanly, and whether the spec remains useful for client generation, review, and drift detection.

## Methodology
Reviewed `openapi.yaml`, cross-checked recent endpoint additions against the current server routes, counted `operationId` entries, and validated the file with a YAML parser after the latest report baseline.

## Findings

### Passing checks
- **Recent route coverage is materially better than the last baseline** — `/metrics`, reset-password, agent-config, and project draft-default endpoints are all documented now (`openapi.yaml:811-849`, `openapi.yaml:1228-1267`, `openapi.yaml:1620-1718`, `openapi.yaml:3016-3059`).
- **The health endpoints reflect the real auth model** — `/api/healthz` is anonymous while `/metrics` requires Bearer or cookie auth, which matches the server handler wiring (`openapi.yaml:811-849`, `internal/server/api_system.go:19-40`).
- **The spec surface has grown rather than stagnated** — the current file contains 119 `operationId` entries, up from the previous 113-count baseline.

### Issues found
| Finding | Severity | Location | Recommendation |
|---------|----------|----------|----------------|
| The OpenAPI document is currently invalid YAML because `info.version` was replaced by a bare `.1.775` line | High | `openapi.yaml:1-10` | Restore a proper `version:` key/value pair and add a parser validation step to CI before future report/spec refreshes. |
| Newly added paths still have very thin example coverage | Low | `openapi.yaml:811-849`, `openapi.yaml:1228-1267`, `openapi.yaml:1620-1718`, `openapi.yaml:3016-3059` | Add request/response examples for the recently added health, agent-config, reset-password, and project draft-default operations. |

## Verdict
Endpoint drift improved, but the spec as a document regressed. The malformed header is now the dominant OpenAPI issue because it breaks standard tooling even though the path inventory is more complete than before.

## Changes since last assessment
- Added documentation for `/metrics`, `/api/users/{username}/reset-password`, `/api/agents/{agent_id}/config*`, and `/api/projects/{project_id}/set-draft` (`openapi.yaml:811-849`, `openapi.yaml:1228-1267`, `openapi.yaml:1620-1718`, `openapi.yaml:3016-3059`).
- Lost parseability at the top of the file because the version line is no longer valid YAML (`openapi.yaml:1-10`).

## Remaining recommendations
| Finding | Severity | Recommendation |
|---------|----------|----------------|
| Invalid top-level YAML | High | Restore `info.version`, validate `openapi.yaml` in CI, and fail the docs/spec workflow when the file cannot be parsed. |
| Thin examples on recent endpoints | Low | Add concrete request/response examples for the newest documented operations so the spec remains useful beyond route discovery. |
