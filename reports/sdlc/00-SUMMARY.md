# SDLC Assessment Summary

**Methodology:** `SDLC.md`  
**Date:** 2026-04-26  
**Project:** ticket / tk  
**Assessment scope:** repository documentation, Go services, SQLite store, web UIs, Playwright/Go test setup, CI/CD, deployment config, and release paths  
**Overall Score:** 74/100 **(was 72, +2)**

## Score Table

| Domain | Previous | Current | Delta |
|--------|----------|---------|-------|
| standards | 75 | 77 | +2 |
| testing | 74 | 76 | +2 |
| cyber | 73 | 73 | 0 |
| readability | 72 | 74 | +2 |
| devops | 70 | 72 | +2 |
| qa | 72 | 73 | +1 |
| infosec | 69 | 71 | +2 |
| architecture | 74 | 74 | 0 |
| technical-writing | 70 | 73 | +3 |
| **Overall** | **72** | **74** | **+2** |

## Score Distribution

| Band | Domains |
|------|---------|
| 90+ | None |
| 80-89 | None |
| 70-79 | standards, testing, cyber, readability, devops, qa, infosec, architecture, technical-writing |
| Below 70 | None |

## Top Systemic Risks

1. **The deploy path is still insecure by default** - the shipped compose bundle still pins both services to mutable `latest` tags and commits `TICKET_ADMIN_PASSWORD: password`, while the entrypoint falls back to `password` on first boot (`deploy/compose.yaml:3-10`, `deploy/compose.yaml:28-33`, `deploy/entrypoint.sh:12-16`).
2. **Release provenance is still weak** - CI publishes release assets, checksums, an SBOM, and GHCR images, but there is still no signing or attestation path for binaries or images (`Makefile:169-243`, `.github/workflows/makefile.yaml:50-103`).
3. **Security trust boundaries remain too implicit** - secure-cookie/HSTS behavior still trusts `X-Forwarded-Proto` without trusted-proxy validation, and the chat bridge still forwards the full server environment into child processes (`internal/server/server.go:668-681`, `internal/server/chat_ws.go:232-237`).
4. **Cross-surface contract drift still exists even though the major blockers were fixed** - `SPEC.md` still says version `0.1.774`, `openapi.yaml` says `0.1.824`, and the binary version is `0.1.848`; the `site2` project editor still allows a 12-character prefix while the store and docs enforce 1-5 uppercase letters (`SPEC.md:1-4`, `openapi.yaml:1-4`, `cmd/tk/VERSION:1`, `web/site2/index.html:808-809`, `internal/store/keys.go:13-24`, `docs/LIFECYCLE.md:14-15`).
5. **Proof is stronger than the baseline, but still not deep enough in the server layer** - the enforced coverage gate now passes, but `internal/server` is still only 57.0%, request-level metrics remain shallow, and the largest CLI/UI/test files still concentrate ownership and review cost (`Makefile:131-153`, assessment run `TICKET_FAST_HASH=1 make test-go-cover` on 2026-04-26, `internal/server/api_system.go:33-85`, assessment run `wc -l` on 2026-04-26).

## Cross-domain Contradiction Log

| Contradiction | Resolution |
|---------------|------------|
| A sub-review initially treated project-prefix drift as still present in the main web UI. | Direct review showed the main web project modal is now aligned with the backend rule using `maxlength="5"` and `pattern="[A-Z]{1,5}"`; the remaining drift is confined to `site2` (`web/static/index.html:1622-1623`, `web/site2/index.html:808-809`, `internal/store/keys.go:13-24`). |
| The previous baseline treated fixed Playwright ports as an open QA/devops blocker. | That finding is now closed: both Playwright configs resolve a free localhost port, and the test docs explain the override knobs (`playwright.config.js:1-20`, `playwright.site2.config.js:1-19`, `TESTING.md:138-153`). |
| The previous baseline treated the OpenAPI artifact as malformed. | That finding is now closed: `openapi.yaml` contains a valid `openapi/info/version` header and `make validate-openapi` succeeds (`openapi.yaml:1-4`, `Makefile:105-109`). |

## What Changed Since Last Assessment

- `openapi.yaml` is now structurally valid, so the previous P1 contract break is closed (`openapi.yaml:1-4`, `Makefile:105-109`).
- The Go coverage gate now passes because `internal/config` rose from 58.8% to 74.9% (`Makefile:131-153`, assessment run `TICKET_FAST_HASH=1 make test-go-cover` on 2026-04-26).
- Browser tests no longer depend on fixed local ports, and both Playwright entry points passed in this rerun (`playwright.config.js:1-20`, `playwright.site2.config.js:1-19`, assessment run `npx playwright test -c ...` on 2026-04-26).
- The main web UI now uses semantic menus, live regions, and discoverable member/team selectors instead of raw numeric-only inputs (`web/static/index.html:1279-1287`, `web/static/index.html:1503-1528`, `web/static/index.html:5191-5224`).
- Privacy, testing, and onboarding docs are current again, but bootstrap-password guidance still drifts in the server quickstart and user guide (`docs/PRIVACY.md:3-5`, `TESTING.md:150-153`, `docs/ONBOARDING.md:219-221`, `QUICKSTART_SERVER.md:13-35`, `USER_GUIDE.md:85-92`, `USER_GUIDE.md:159-162`).

## Cumulative Improvement Since Baseline

| Item | Baseline | Current | Evidence |
|------|----------|---------|----------|
| Overall SDLC score | 72 | 74 | `reports/sdlc/history.json` |
| OpenAPI contract health | malformed header | validates cleanly | `openapi.yaml:1-4`, `Makefile:105-109` |
| Coverage gate status | failing on `internal/config` | passing across all enforced packages | `Makefile:131-153`, assessment run `TICKET_FAST_HASH=1 make test-go-cover` |
| Browser-test port strategy | fixed ports in both configs | dynamic/free-port selection with documented overrides | `playwright.config.js:1-20`, `playwright.site2.config.js:1-19`, `TESTING.md:150-153` |
| Privacy policy freshness | stale version/date | current to `0.1.848` / 2026-04-26 | `docs/PRIVACY.md:3-5` |
| Main web membership discoverability | raw numeric IDs and click-built menus | semantic menus, datalists, named user/team display | `web/static/index.html:1279-1287`, `web/static/index.html:1503-1528`, `web/static/index.html:5191-5224` |

## Key Delivery Metrics

| Metric | Current | Evidence |
|--------|---------|----------|
| Go test files | 41 | assessment run: `find . -name '*_test.go' | wc -l` on 2026-04-26 |
| Playwright spec files | 12 | assessment run: `find tests/playwright -name '*.spec.js' | wc -l` on 2026-04-26 |
| GitHub Actions workflows | 1 | assessment run: `find .github/workflows -name '*.yaml' | wc -l` on 2026-04-26 |
| Go coverage gate packages | 6 | `Makefile:131-153` |
| Go coverage gates | passing | assessment run: `TICKET_FAST_HASH=1 make test-go-cover` on 2026-04-26 |
| Lowest gated package | `internal/server` 57.0% / 55% required | assessment run: `TICKET_FAST_HASH=1 make test-go-cover` on 2026-04-26 |
| OpenAPI validation step in CI | present and passing | `.github/workflows/makefile.yaml:25-29`, `Makefile:105-109` |
| Browser tests in CI | yes | `.github/workflows/makefile.yaml:43-48` |
| Main Playwright suite | 118 passed | assessment run `npx playwright test -c playwright.config.js` on 2026-04-26 |
| `site2` Playwright suite | 9 passed | assessment run `npx playwright test -c playwright.site2.config.js` on 2026-04-26 |
| Browser-test port strategy | dynamic | `playwright.config.js:1-20`, `playwright.site2.config.js:1-19` |
| Metrics endpoint depth | health plus coarse gauges | `internal/server/api_system.go:19-85` |
| Release artifact/image signing | none | `Makefile:169-243`, `.github/workflows/makefile.yaml:50-103` |
| Deploy image mutability | 2 latest tags | `deploy/compose.yaml:3`, `deploy/compose.yaml:28` |

## Prioritized Action Register

| Priority | Finding | Owner role | Dependency notes |
|----------|---------|------------|------------------|
| P1 | Remove the committed default bootstrap password from the deploy bundle and from bootstrap-oriented docs | security-engineer | Depends on deciding whether first boot requires explicit env input, generated secret output, or a separate demo profile |
| P2 | Add signing/attestation to releases and stop treating mutable image refs as the default production path | release-manager | Depends on CI/release-process changes and pinned deploy references |
| P3 | Validate trusted proxies before honoring `X-Forwarded-Proto`, and whitelist env vars for chat/analyse child processes | application-security | Depends on deployment-topology/config decisions |
| P4 | Finish cross-surface contract alignment: sync `SPEC.md`, `openapi.yaml`, and `cmd/tk/VERSION`, and align `site2` prefix validation with the store rule | api-architect | Depends on deciding the single source of truth for versioning and secondary UI rules |
| P5 | Raise proof in the server layer, especially auth/websocket/agent paths, and decide whether race coverage belongs in CI | qa-architect | Depends on targeted test additions and CI cost tolerance |
| P6 | Reduce large ownership hotspots and document the practical SQLite concurrency ceiling and scaling trigger | systems-architect | Depends on refactor slicing and architecture/runbook follow-up |

## Overall Verdict

This rerun is materially stronger than the baseline. The public API contract now validates, the enforced Go coverage gate is green, Playwright is no longer brittle around fixed ports, and the main web/admin surface is more accessible and discoverable. The project still sits meaningfully short of "excellent" because the deploy bundle remains insecure by default, release provenance is unsigned, trust-boundary assumptions are too implicit, and a smaller set of contract/documentation drifts still weakens the single-source-of-truth story.
