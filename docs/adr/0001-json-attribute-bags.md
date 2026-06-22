# ADR 0001: JSON attribute bags for extensible schema

- Status: Accepted (epic TK-105 / story TK-106)
- Date: 2026-06-22
- Deciders: Simon Gauld

## Context

`tk` stores all data in a single SQLite database with a monolithic, additive
migration system (`internal/store/store.go`, `internal/store/schema_version.go`).
Adding a field to a core entity (tickets/projects/roles/workflow_stages) is
costly out of proportion to its value: every addition needs a schema-version bump
and ripples positionally through ~10 hand-written `SELECT`s plus a 41-arg
`scanTicket`. We want to lower both the likelihood and the blast radius of schema
change without losing queryability or migration safety.

## Decision

Adopt a **governed three-tier model**:

1. **Tier 1 — first-class columns** for anything needing FK / NOT NULL / hot
   WHERE-ORDER BY / aggregation. Unchanged.
2. **Tier 2 — a per-entity `attrs` JSONB bag** as the default home for optional,
   sparse, display-only, and per-type fields. Adding a Tier-2 field is a pure-Go
   change to a typed accessor struct — no SQL, no version bump.
3. **Tier 3 — promotion** of a bag field to query-grade via an idempotent
   expression index (`json_extract`) or, rarely, a generated column.

Store the bag as **JSONB (BLOB)** using SQLite 3.45+ binary JSON, available via
`modernc.org/sqlite v1.48.0`. Centralize the ticket column list + scan to remove
the fan-out. Harden migrations with checkpointed, integrity-verified backups and
automatic rollback.

## Alternatives considered

### A. Status quo — keep adding typed columns
Rejected. This is exactly the churn the epic exists to remove. ALTER TABLE is
cheap, but the code fan-out and version coupling are not.

### B. EAV side table (`ticket_attributes(ticket_id, key, value, type)`)
Rejected for core fields. Pros: never ALTER, fully dynamic. Cons: every read
becomes a join or pivot, the atomic row is lost, reporting/sorting is painful,
and type-safety evaporates. Classic anti-pattern for first-class entity data.

### C. Plain TEXT JSON column
Partially accepted but superseded. The existing `dor_map`/`dod_map`/`ac_map`
columns already use TEXT JSON and work. JSONB is more compact and faster to parse
while remaining queryable by the same `json_*` functions, so new storage uses
JSONB; TEXT JSON is retained only transitionally until S6 folds those columns in.

### D. Generated columns for everything
Rejected as the default. Generated columns are useful for promotion (Tier 3) but
defining one per field reintroduces per-field DDL. Reserved for the rare case
that needs a real column surface over a bag field.

## Consequences

- Adding an optional field becomes a no-migration, no-version-bump Go change.
- Querying a bag field requires an explicit promotion step (expression index),
  making "is this field queryable?" an intentional decision rather than a default.
- The `attrs` blob is opaque in raw SQLite shells (mitigated: read via
  `json(attrs)`); slightly higher write cost to encode JSONB.
- Migration safety is materially improved for ALL migration paths, not just the
  consolidation in this epic.
