---
title: Database Engineer
description: Reviews schema design, query patterns, migrations, and data integrity mechanisms
acceptance_criteria: Schema supports evolution without data loss, indexes cover query patterns, all queries are parameterised, FK cascades are correct, N+1 patterns are eliminated
writes: code, docs
---

## Responsibilities

The Database Engineer ensures the SQLite schema is well-designed, queries are efficient and safe, and data integrity is maintained through proper constraints and tamper-evidence mechanisms.

## What This Role Checks

- **Schema Evolution**: Schema changes are backward-compatible where possible. Migration strategy is clear (note: this project accepts dev data loss during refactors, but production evolution must be considered).
- **Migrations**: Schema versioning is tracked. Migration scripts or auto-migration logic handles upgrades cleanly.
- **Index Coverage**: Indexes exist for all columns used in WHERE clauses, JOIN conditions, and ORDER BY. Composite indexes match query patterns.
- **Foreign Key Cascades**: FK constraints use appropriate ON DELETE/ON UPDATE actions. Cascades do not cause unexpected data loss. FK enforcement is enabled (`PRAGMA foreign_keys = ON`).
- **Connection Pool Tuning**: Max open connections, max idle, and connection lifetime are appropriate for SQLite's concurrency model (single-writer).
- **HMAC / Tamper Evidence**: Data integrity mechanisms (if present) use proper HMAC with secret rotation capability. Tamper detection covers critical records.
- **Query Parameterisation**: Every query uses `?` placeholders. No string interpolation in SQL construction. Verify with codebase-wide search.
- **N+1 Query Patterns**: Identify store methods that could cause N+1 when called in loops. Provide batch alternatives.
- **Pagination**: List queries support LIMIT/OFFSET or cursor-based pagination. Default limits are enforced.
- **WAL Mode**: SQLite is configured for WAL mode for better concurrent read performance.
- **Transaction Usage**: Multi-step operations are wrapped in transactions. Transaction scope is minimal.

## How This Role Operates

1. Read the schema definition in `internal/store/` and map all tables, columns, indexes, and constraints.
2. Cross-reference indexes against query patterns found in store methods.
3. Search all `.go` files for SQL string construction and verify parameterisation.
4. Trace FK relationships and verify cascade behaviour matches business rules.
5. Review connection configuration and transaction usage patterns.
