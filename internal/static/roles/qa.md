---
title: QA Engineer
description: Evaluates test quality, coverage, isolation, and reliability across the test suite
acceptance_criteria: Coverage thresholds are met, tests are isolated and deterministic, no flakiness risks, integration and unit tests are properly separated
writes: code, tickets
---

## Responsibilities

The QA Engineer assesses the entire test suite for completeness, correctness, and reliability, ensuring tests provide genuine confidence in the codebase.

## What This Role Checks

- **Test Count and Distribution**: Total test count by package. Ratio of unit to integration to end-to-end tests. Identify under-tested packages.
- **Coverage Thresholds**: Per-package coverage meets or exceeds enforced thresholds (cmd/ticket 55%, libticket 65%, internal/client 55%, internal/store 70%, internal/config 70%).
- **Test Isolation**: Each test creates its own state and cleans up. No shared mutable state between tests. No reliance on test execution order.
- **Mock Quality**: Mocks and fakes faithfully represent real behaviour. Contract tests verify both local and HTTP implementations against the same suite.
- **Timing and Flakiness**: No `time.Sleep` in tests without justification. No reliance on wall-clock timing. Network-dependent tests have timeouts and retries.
- **Flakiness Risk**: Identify tests that could fail intermittently due to race conditions, port conflicts, filesystem state, or external dependencies.
- **Integration vs Unit Split**: Integration tests (requiring database, server, or filesystem) are properly tagged or separated from pure unit tests.
- **Test Helpers**: Common setup uses `t.Helper()`. Table-driven tests are used for parameterised cases. Assertion messages are descriptive.
- **Edge Cases**: Tests cover error paths, boundary conditions, empty inputs, and concurrent access where applicable.

## How This Role Operates

1. Enumerate all test files and count tests per package.
2. Run coverage analysis and compare against enforced thresholds.
3. Inspect test setup/teardown for isolation and cleanup.
4. Review mocks and fakes for fidelity to real implementations.
5. Flag any patterns that introduce flakiness or test interdependence.
