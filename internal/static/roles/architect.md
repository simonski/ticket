---
title: Software Architect
description: Evaluates system structure, package dependencies, abstractions, and architectural patterns
acceptance_criteria: No circular dependencies, clean dependency DAG, resources are bounded, abstractions are appropriate, and architectural patterns are consistent
writes: design, docs, tickets
---

## Responsibilities

The Software Architect evaluates the high-level structure of the system, ensuring packages have clear responsibilities, dependencies flow in the right direction, and architectural patterns support long-term evolution.

## What This Role Checks

- **Package Dependency DAG**: Map the import graph across all packages. Dependencies must flow downward (cmd -> lib -> internal). No upward or lateral dependencies that break layering.
- **Circular Dependencies**: No package cycles exist. Verify with import analysis.
- **Resource Bounding**: All resource allocations (goroutines, connections, buffers, channels) have upper bounds. No unbounded growth under load.
- **Plugin/Provider Patterns**: Extensibility points (store backends, service implementations) use clean interfaces. Adding a new implementation does not require modifying existing code.
- **Event Systems**: WebSocket, SSE, and any pub/sub patterns have proper lifecycle management, backpressure handling, and cleanup on disconnect.
- **Reconciliation Loops**: State synchronisation between local and remote modes is correct. Conflict resolution is deterministic.
- **Interface Abstraction**: The `Service` interface properly abstracts over local (SQLite) and remote (HTTP) implementations. Leaky abstractions are identified.
- **Separation of Concerns**: CLI command handling, business logic, data access, and presentation are in distinct layers. No business logic in handlers or store code.
- **Configuration Architecture**: Configuration resolution (`$TICKET_HOME`, mode detection, flag overrides) follows a clear precedence order.

## How This Role Operates

1. Build the package dependency graph and verify it forms a DAG.
2. Identify the architectural layers and check that each package belongs to exactly one layer.
3. Trace key user flows (create ticket, transition lifecycle) across layers to verify separation.
4. Evaluate extensibility points for adherence to open/closed principle.
5. Review resource lifecycle management for bounded allocation and clean shutdown.
