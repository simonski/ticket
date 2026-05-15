- read .github/copilot-instructions.md
- read CONTRIBUTING.md
- read README.md
- read QUICKSTART.md 

## Testing workflow

- `make test` is the ultra-fast default (unit tests).
- `make test-fast` runs the recommended developer loop (unit + JS API + Go API smoke).
- `make test-api-js` validates the JavaScript API client library (`web/site2/api.test.js`).
- `make test-api-smoke` validates the fast Go API smoke packages (`internal/client`, `internal/server`).
- `make test-cli` runs the heavier CLI package suite (`cmd/tk`).
- `make test-contract` runs the heavier shared `libticket` contract suite.
- `make test-api-cli` validates CLI/API interactions (`cmd/tk`, `internal/client`, `internal/server`, `libticket`).
- `make test-api` runs both API suites (`test-api-js` + `test-api-cli`).
- `make test-browser` runs browser E2E Playwright tests.
- `make test-quickstart` validates executable docs in `QUICKSTART.md` and `TUTORIAL.md`.
- `make test-all` runs the full suite (unit + api + browser + quickstart + docs/harness).

### Policy

- On each change: run `make test` and `make lint`.
- On normal feature work: prefer `make test-fast` before jumping to the full API suites.
- If API surface/contract changes (`openapi.yaml`, server/client/CLI API code): run `make test-api`.
- If web UI changes: run `make test-browser`.
- Before completion/PR: run `make test-all` and `make lint`.
