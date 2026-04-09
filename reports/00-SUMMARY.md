# SDLC Assessment Summary

**Project:** ticket  
**Version:** 0.1.737  
**Date:** 2026-04-08  
**Overall Score: 70/100** (was 69, was 62)

---

## Score Table

| # | Category | v0.1.730 | v0.1.737 | Current | Δ | Key Finding |
|---|----------|----------|----------|---------|---|-------------|
| 01 | OpenAPI Spec | 55 | 62 | **64** | +2 | `/metrics` endpoint not in spec; param descriptions improved to 27% |
| 02 | Security | 58 | 65 | **68** | +3 | All previous Criticals fixed; IdleTimeout missing; 6 remaining medium/low items |
| 03 | InfoSec / Cyber | 52 | 60 | **60** | 0 | SSRF mitigated; command injection #nosec justified; threat model complete |
| 04 | Idiomatic Go | 72 | 81 | **80** | -1 | `chat_ws.go` uses `context.Background()` in handler; Service interface still no ctx |
| 05 | Idiomatic JavaScript | 78 | 78 | **78** | 0 | Stable; 4 silent `.catch` swallows and 9 `var` declarations remain |
| 06 | DevOps | 72 | 81 | **81** | 0 | Stable; strong CI pipeline, govulncheck, cross-platform releases |
| 07 | QA / Testing | 74 | 76 | **76** | 0 | 473 Go tests; coverage thresholds enforced; WS/TUI still untested |
| 08 | Tech Lead | 55 | 63 | **63** | 0 | `client.go` 1894 lines / 98 mode branches; 11 files >700 lines |
| 09 | Architecture | 70 | 73 | **72** | -1 | `client.go` regression (80→98 branches); WS cross-project event leakage |
| 10 | Performance | 60 | 60 | **62** | +2 | 9 new indexes confirmed; `messages`/`goals` tables unindexed; N+1 unfixed |
| 11 | Database | 60 | 66 | **68** | +2 | `messages`/`goals` missing FK indexes confirmed; zombie `history_events` persists |
| 12 | Tech Writer | 68 | 74 | **74** | 0 | Strong docs; CONTRIBUTING.md service count stale; no GitHub issue templates |
| 13 | Product Owner | 76 | 78 | **78** | 0 | All goals met; delete confirm exists (token-based); messaging UX gaps remain |
| 14 | Compliance | 42 | 62 | **74** | +12 | `docs/PRIVACY.md` added (comprehensive 130 lines); SBOM stale; assignee not cleared |
| 15 | SRE | 28 | 44 | **42** | -2 | `/metrics` unauthenticated (new High); structured logging requires `-v`; no SIGTERM |
| 16 | UX Review | 64 | 68 | **68** | 0 | Delete confirmation confirmed in code (token-based); loading states still missing |
| 17 | New Starter | 62 | 83 | **83** | 0 | Excellent onboarding; CONTRIBUTING.md service count stale |
| **Overall** | | **62** | **69** | **70** | +1 | |

---

## Score Distribution

| Band | Categories |
|------|-----------|
| **80–100** (Good) | New Starter (83), DevOps (81), Idiomatic Go (80) |
| **70–79** (Acceptable) | Idiomatic JS (78), Product Owner (78), QA (76), Compliance (74), Tech Writer (74), Architecture (72) |
| **60–69** (Needs work) | Security (68), UX (68), Database (68), Tech Lead (63), OpenAPI (64), Performance (62), InfoSec (60) |
| **50–59** (Poor) | — |
| **0–49** (Critical) | SRE (42) |

---

## What Changed Since Last Assessment (v0.1.737 → current re-assessment)

| Category | Change | Impact |
|----------|--------|--------|
| OpenAPI (+2) | `/metrics` endpoint identified as spec drift; param descriptions 13.5%→27% | Improved coverage |
| Security (+3) | Verified all previous Criticals fixed; `IdleTimeout` missing; new Low finding | Maturity confirmed |
| Idiomatic Go (-1) | `chat_ws.go:168,177` uses `context.Background()` inside WS handler | New regression found |
| Architecture (-1) | `client.go` branches grew 80→98 (regression); cross-project WS leakage identified | Structural debt increasing |
| Performance (+2) | 9 new indexes confirmed; `messages`/`goals` unindexed; N+1 pattern still present | Incremental improvement |
| Database (+2) | `time_entries`/`workflow_stages` indexes confirmed; messages/goals FK gaps confirmed | Incremental improvement |
| Compliance (+12) | `docs/PRIVACY.md` confirmed comprehensive (130 lines, Arts. 15-21); SBOM stale at 0.1.733 | Large jump from Privacy.md |
| SRE (-2) | `/metrics` unauthenticated (new High); structured logging requires `-v` flag | New findings lower score |

---

## Cumulative Improvement

| Assessment | Version | Score | Change |
|------------|---------|-------|--------|
| Round 1 | v0.1.730 | 62/100 | — (baseline) |
| Round 2 | v0.1.737 | 69/100 | +7 (+11%) |
| Round 3 | v0.1.737 (re-assess) | 70/100 | +1 (+2%) |
| **Total** | | | **+8 (+13%)** |

---

## Key Metrics

| Metric | v0.1.730 | Current |
|--------|----------|---------|
| Go test functions | 390 | 473 |
| Playwright E2E tests | 111 | 111 |
| Coverage thresholds enforced | No | Yes (55–75% per package) |
| OpenAPI operations | 104 | 104 (but `/metrics` endpoint missing) |
| OpenAPI param descriptions | 13.5% | 27% |
| Go files >700 lines | 15 | 11 |
| Go files >1000 lines | 7 | 4 |
| Service interface methods | 104 | 108 (7 sub-interfaces) |
| DB tables | 20 | 25 |
| DB indexes defined | 26 | 35 |
| DB indexes missing (confirmed) | 9 | 3 (`messages` ×2, `goals` ×1) |
| `client.go` mode branches | ~80 | 98 (regression) |
| Direct dependencies | 7 | 7 (all MIT/Apache/BSD) |
| Known CVEs in dependencies | 0 | 0 |
| SBOM | No | Yes (stale: v0.1.733) |
| Privacy documentation | No | Yes (`docs/PRIVACY.md`, 130 lines) |
| Runbooks | No | Yes (`docs/RUNBOOKS.md`, 9 scenarios) |
| Graceful shutdown | No | **No** (still missing) |
| Authenticated `/metrics` | N/A | **No** (unauthenticated) |

---

## Prioritised Action Items

### Critical — Fix Before Production Exposure

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P0 | No graceful shutdown — SIGTERM kills all connections abruptly | 15-sre | 2 hours |
| P0 | `/metrics` unauthenticated — exposes org size to anonymous callers | 15-sre | 30 min |
| P1 | `tickets.assignee` not cleared on user deletion — GDPR Art. 17 partially broken | 14-compliance | 30 min |
| P1 | Credential logging in verbose mode — passwords appear in log files | 02-security | 1 hour |

### High — Fix This Cycle

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P2 | Add alerting rules (TicketDown, High5xxRate, SlowAPI) | 15-sre | 1 day |
| P2 | `Service` interface takes no context; 125x `context.Background()` | 04-idiomatic-go | 3 days |
| P2 | Add `GET /api/users/{id}/export` — GDPR Art. 20 portability | 14-compliance | 2 days |
| P2 | Complete `history_events` zombie table removal | 11-database | 1 day |
| P2 | Add `ON DELETE CASCADE` to all 25+ FK relationships | 11-database | 2 hours |
| P2 | Structured logging unconditional (gate body logging behind `-v`) | 15-sre | 2 hours |
| P2 | Add indexes: `idx_messages_from_user_id`, `idx_messages_to_user_id`, `idx_goals_project_id` | 11-database | 30 min |
| P2 | Refactor `client.go` — 98 mode branches, 1894 lines (regression) | 08-tech-lead | 1 week |

### Medium — Next Quarter

| Priority | Finding | Report | Effort |
|----------|---------|--------|--------|
| P3 | WebSocket cross-project event broadcast filtering (leakage risk) | 09-architect | 1 day |
| P3 | Add `max_live_connections` cap to WebSocket hub | 09-architect | 2 hours |
| P3 | Define and document SLOs (99.5% availability, p99 < 500ms) | 15-sre | 4 hours |
| P3 | Add per-username account lockout (rate limit login) | 02-security | 1 day |
| P3 | Regenerate SBOM on every release; enforce version match in CI | 14-compliance | 2 hours |
| P3 | Add `/api/metrics` to `openapi.yaml` | 01-openapi | 30 min |
| P3 | N+1 fix: replace `ListTickets` with `WHERE parent_id = ?` | 10-performance | 4 hours |
| P3 | Schema migration versioning (`schema_version` table) | 11-database | 1 day |
| P3 | `chat_ws.go` context propagation (use upgrade request context) | 04-idiomatic-go | 1 hour |
| P3 | Warn at startup if `TICKET_ENCRYPTION_KEY` is absent | 14-compliance | 30 min |

---

## Strengths

- Clean package DAG with no circular imports
- Strong crypto: Argon2id, 32-byte random tokens, AES-256-GCM, correct cookie flags
- Excellent contract test pattern: same suite runs against both implementations
- Well-designed dual-mode abstraction (local SQLite / remote HTTP)
- Real-time WebSocket event system with proper channel management
- All stated product goals met
- Consistent parameterised SQL queries — no user-data injection
- Zero known CVEs in dependency set
- Comprehensive `docs/RUNBOOKS.md` covering 9 operational scenarios
- Automated release pipeline: cross-platform builds, Homebrew formula, GitHub releases
- Coverage thresholds enforced per package in CI
- `docs/PRIVACY.md` comprehensive GDPR documentation now present
- `docs/ONBOARDING.md` enables new starters to be productive on day one
