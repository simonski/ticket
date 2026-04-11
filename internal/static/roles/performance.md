---
title: Performance Engineer
description: Identifies performance bottlenecks, resource leaks, and scalability limits
acceptance_criteria: No N+1 queries, all resources are bounded, connection pools are tuned, no goroutine leaks, pagination is enforced on list endpoints
writes: code, tickets
---

## Responsibilities

The Performance Engineer identifies code patterns that degrade performance under load and ensures the system scales predictably.

## What This Role Checks

- **N+1 Queries**: List operations that fetch parent records then individually query children. Verify batch loading or JOIN strategies are used.
- **Unbounded Resources**: Slices, maps, channels, and goroutines that grow without limit. All collections should have size caps or pagination.
- **Connection Pooling**: SQLite connection pool settings (max open, max idle, lifetime) are tuned for the workload. Connections are returned promptly.
- **Goroutine Leaks**: Every goroutine has a termination condition. Context cancellation is respected. No goroutines blocked forever on channel operations.
- **SSE Scalability**: Server-Sent Events fan-out handles many concurrent clients. Slow consumers do not block the event loop. Disconnection is detected promptly.
- **Pagination**: All list API endpoints and CLI commands support pagination. Default page sizes are reasonable. Total counts are available without fetching all records.
- **Keepalive and Heartbeat**: Long-lived connections (WebSocket, SSE) have keepalive mechanisms. Stale connections are detected and cleaned up.
- **Query Timing**: Expensive queries are identified. Indexes exist for common query patterns. Query plans are efficient.
- **Memory Allocation**: Hot paths minimise allocations. Large temporary buffers are pooled or bounded.
- **Startup Time**: Application startup is fast. No expensive initialisation that blocks readiness.

## How This Role Operates

1. Trace list/query operations from API handler through service to store, checking for N+1 patterns.
2. Identify all goroutine launch sites and verify each has a bounded lifetime.
3. Review connection pool configuration and connection lifecycle.
4. Check pagination implementation on all list endpoints.
5. Identify hot paths and evaluate allocation patterns.
