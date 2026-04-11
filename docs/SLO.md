# Service Level Objectives

## Availability

| Service | Target | Measurement |
|---------|--------|-------------|
| HTTP API | 99.5% uptime | `ticket_up` metric via `/api/healthz` |
| WebSocket | 99.0% uptime | Connection success rate |

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

Recommended Prometheus alerting rules (add to `alerts/ticket.yml`):

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

      - alert: HighGoroutineCount
        expr: go_goroutines > 500
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Goroutine count exceeds 500"

      - alert: HighMemoryUsage
        expr: go_memstats_alloc_bytes > 512 * 1024 * 1024
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Heap allocation exceeds 512MB"

      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "HTTP 5xx error rate exceeds 1%"
```

## Capacity Guidelines

| Resource | Single Server Guideline |
|----------|------------------------|
| CPU | 1 core handles ~100 concurrent users |
| Memory | 256MB base + ~1MB per concurrent WebSocket |
| Disk | ~1KB per ticket, ~10KB per ticket with full history |
| SQLite DB | Recommended max ~500K tickets before considering PostgreSQL |
| Concurrent connections | Limited by `SetMaxOpenConns` (currently 1 for SQLite write serialization) |
