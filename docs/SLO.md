# Service Level Objectives

## Availability

| Service | Target | Measurement |
|---------|--------|-------------|
| HTTP API | 99.5% uptime | `ticket_up` from authenticated `/metrics` scrapes plus `/api/healthz` probes |
| WebSocket | 99.0% uptime | Connection success rate from structured logs |

## Latency

| Endpoint | p50 | p95 | p99 |
|----------|-----|-----|-----|
| GET /api/tickets | < 50ms | < 200ms | < 500ms |
| POST /api/tickets | < 100ms | < 300ms | < 1s |
| GET /api/healthz | < 10ms | < 50ms | < 100ms |

## Error Rate

| Metric | Target |
|--------|--------|
| HTTP 5xx rate | < 0.1% of requests |
| Panic recovery events | 0 per day |

## Alerting Rules

The server exposes a small authenticated Prometheus surface at `/metrics`. Use it
for liveness/capacity signals, and derive request-rate / latency / 5xx alerts
from structured logs until request counters are added.

Recommended alert ownership:

| Alert | Owner | Signal Source |
|-------|-------|---------------|
| `TicketDown` | Primary on-call | `/metrics` + `/api/healthz` |
| `OpenTicketBacklogSpike` | Product / delivery owner | `/metrics` |
| `HighMemoryUsage` | Primary on-call | `/metrics` |
| `High5xxRate` | Primary on-call | Structured logs |

Example Prometheus alerting rules for the metrics that exist today:

```yaml
groups:
  - name: ticket
    rules:
      - alert: TicketDown
        expr: ticket_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Ticket server is down"

      - alert: OpenTicketBacklogSpike
        expr: ticket_open_tickets_total > 5000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Open ticket backlog exceeds expected steady-state range"

      - alert: HighMemoryUsage
        expr: go_memstats_alloc_bytes > 512 * 1024 * 1024
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Heap allocation exceeds 512MB"

```

For request latency and 5xx-rate alerting, prefer log-based monitors keyed on
the server's structured request logs (`path`, `status`, `duration_ms`,
`request_id`) until dedicated request counters and histograms are added.

## Tracing

The product currently has **no distributed tracing pipeline**. The deliberate
operational model today is:

- request-scoped correlation via `X-Request-ID`
- structured API logs with `path`, `status`, and `duration_ms`
- `/api/healthz` and authenticated `/metrics` for liveness/capacity checks

If deeper cross-service tracing is needed later, add it explicitly rather than
assuming it exists today.

## Capacity Guidelines

| Resource | Single Server Guideline |
|----------|------------------------|
| CPU | 1 core handles ~100 concurrent users |
| Memory | 256MB base + ~1MB per concurrent WebSocket |
| Disk | ~1KB per ticket, ~10KB per ticket with full history |
| SQLite DB | Recommended max ~500K tickets before considering PostgreSQL |
| Concurrent connections | Limited by `SetMaxOpenConns` (currently 1); treat this as an explicit SQLite scaling ceiling, not an implementation detail |
