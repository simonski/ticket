# SDLC Assessment Summary

**Project:** ticket  
**Version:** 0.1.730  
**Date:** 2025-07-18  
**Overall Score: 62/100**

---

## Score Table

| # | Category | Score | Grade | Key Finding |
|---|----------|-------|-------|-------------|
| 01 | OpenAPI Spec | 55/100 | D+ | 79% of operations missing error responses; 0% example coverage |
| 02 | Security | 58/100 | D+ | No CSRF protection; WebSocket no Origin validation; session expiry not enforced |
| 03 | InfoSec / Cyber | 52/100 | D | Command injection in agent shell execution; SSRF in analyse feature |
| 04 | Idiomatic Go | 72/100 | C+ | No context propagation in store layer; no golangci-lint config |
| 05 | Idiomatic JavaScript | 78/100 | C+ | Clean XSS hygiene; 9 `var` declarations; 4 silent error swallows |
| 06 | DevOps | 72/100 | C+ | Good pipeline; missing Dockerfile HEALTHCHECK, compose resource limits |
| 07 | QA / Testing | 74/100 | C+ | 390 Go tests + 111 Playwright; coverage not enforced in CI; Playwright timing flakiness |
| 08 | Tech Lead | 55/100 | D+ | 7 files >1000 lines; 104-method God interface; 55× path-parsing duplication |
| 09 | Architecture | 70/100 | C | Clean DAG; SQLite concurrency ceiling; unbounded data growth |
| 10 | Performance | 60/100 | D | N+1 comments query; unbounded list queries; 9 missing critical indexes |
| 11 | Database | 60/100 | D | 9 missing indexes; duplicate audit tables; no FK cascades |
| 12 | Tech Writer | 68/100 | D+ | Strong architecture docs; ~2% OpenAPI examples; 0 godoc comments |
| 13 | Product Owner | 76/100 | C+ | All stated goals met; missing delete confirm; undocumented board/TUI |
| 14 | Compliance | 42/100 | F | Incomplete user deletion; no retention policy; no SBOM; no privacy docs |
| 15 | SRE | 28/100 | F | No graceful shutdown; no metrics; no structured logging; no runbooks |
| 16 | UX Review | 64/100 | D | No ticket delete confirm; no loading states; missing form labels |
| 17 | New Starter | 62/100 | D | `make build` version-bump undocumented; no CONTRIBUTING.md; no onboarding guide |

---

## Score Distribution

| Band | Categories |
|------|-----------|
| **80–100** (Good) | — |
| **70–79** (Acceptable) | Idiomatic Go (72), Idiomatic JS (78), DevOps (72), QA (74), Architecture (70) |
| **60–69** (Needs work) | Performance (60), Database (60), Tech Writer (68), Product Owner (76→), UX (64), New Starter (62) |
| **50–59** (Poor) | OpenAPI (55), Security (58), InfoSec (52), Tech Lead (55) |
| **0–49** (Critical) | Compliance (42), SRE (28) |

---

## What Changed Since Last Assessment
First assessment — no delta available.

---

## Key Metrics

| Metric | Value |
|--------|-------|
| Go test functions | 390 |
| Playwright E2E tests | 111 |
| Coverage thresholds enforced | 55–75% per package |
| OpenAPI operations | 104 |
| OpenAPI operations with error responses | 22/104 (21%) |
| OpenAPI request body examples | 0/37 (0%) |
| Go files >700 lines | 15 |
| Go files >1000 lines | 7 |
| Service interface methods | 104 |
| DB tables | 20 |
| DB indexes defined | 26 |
| DB indexes missing (critical) | 9 |
| Exported functions without godoc | 103 (libticket alone) |
| Direct dependencies | 7 (all MIT/Apache/BSD) |
| Known CVEs in dependencies | 0 |

---

## Prioritised Action Items

### 🔴 Critical — Fix Before Production Exposure

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P0 | Command injection: `exec.Command("sh", "-c", llmCmd)` in agent | 03-infosec | 1 hour |
| P0 | No graceful shutdown — SIGTERM kills all connections abruptly | 15-sre | 2 hours |
| P0 | No CSRF protection on any state-changing endpoint | 02-security | 1 day |
| P1 | `DeleteUser()` does not erase associated data — GDPR Art. 17 | 14-compliance | 1 day |
| P1 | 9 missing critical DB indexes — full table scans on core queries | 11-database | 2 hours |
| P1 | Session `expires_at` never enforced — sessions permanent | 02-security | 2 hours |
| P1 | WebSocket Origin header not validated | 02-security | 2 hours |
| P1 | No SBOM generated | 14-compliance | 2 hours |

### 🟠 High — Fix This Cycle

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P2 | No structured logging (`slog`) | 15-sre | 2 days |
| P2 | No Prometheus `/metrics` endpoint | 15-sre | 2 days |
| P2 | N+1 query in `hydrateTicket()` | 10-performance | 1 day |
| P2 | Unbounded list queries (ListProjects, ListUsers, ListAgents, etc.) | 10-performance | 1 day |
| P2 | No context propagation in store layer | 04-idiomatic-go | 3 days |
| P2 | `api.go` 3009 lines — extract resource handlers | 08-tech-lead | 1 week |
| P2 | 104-method God interface — split into sub-interfaces | 09-architect | 1 week |
| P2 | No `CONTRIBUTING.md` or `docs/ONBOARDING.md` | 17-new-starter | 1 day |
| P2 | Ticket delete has no confirmation (CLI and web UI) | 16-ux | 2 hours |
| P2 | Duplicate `history_events` + `ticket_history` tables | 11-database | 1 day |

### 🟡 Medium — Next Quarter

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P3 | Add OpenAPI error responses to all 82 missing operations | 01-openapi | 1 week |
| P3 | Add request/response examples to OpenAPI spec | 01-openapi | 1 week |
| P3 | Add golangci-lint + `.golangci.yml` | 04-idiomatic-go | 2 hours |
| P3 | Enforce coverage thresholds in CI | 07-qa | 30 mins |
| P3 | Add Dockerfile HEALTHCHECK | 06-devops | 30 mins |
| P3 | Add resource limits to `compose.yaml` | 06-devops | 30 mins |
| P3 | Add data retention policy | 14-compliance | 2 days |
| P3 | Create `docs/PRIVACY.md` | 14-compliance | 1 day |
| P3 | Add godoc to all exported functions | 12-tech-writer | 2 days |
| P3 | Create `docs/RUNBOOKS.md` | 15-sre | 1 day |
| P3 | Document TUI and `tk board` | 13-product-owner | 2 hours |

---

## Strengths

- ✅ Clean package DAG with no circular imports
- ✅ Strong crypto: Argon2id, 32-byte random tokens, correct cookie flags
- ✅ Excellent contract test pattern: same suite runs against both implementations
- ✅ Well-designed dual-mode abstraction (local SQLite / remote HTTP)
- ✅ Real-time WebSocket event system with proper channel management
- ✅ All 7 stated product goals met at v0.1.730
- ✅ Consistent parameterised SQL queries — no user-data injection
- ✅ Good keyboard navigation in web UI; extensive ARIA foundations
- ✅ Zero known CVEs in dependency set
- ✅ Automated release pipeline: cross-platform builds, Homebrew formula, GitHub releases
