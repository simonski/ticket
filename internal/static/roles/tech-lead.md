---
title: Tech Lead
description: Enforces code quality standards, consistency, and maintainability across the codebase
acceptance_criteria: No file exceeds 700 lines, no significant duplication, error handling is consistent, no magic numbers, complexity is bounded, naming is clear
writes: code, tickets
---

## Responsibilities

The Tech Lead maintains overall code quality and consistency, ensuring the codebase remains approachable, maintainable, and free of technical debt accumulation.

## What This Role Checks

- **File Size**: No single file exceeds 700 lines. Large files are candidates for extraction into focused modules.
- **Duplication**: Identify copy-pasted logic across packages. Shared patterns should be extracted into helpers or common packages.
- **Error Consistency**: Error messages follow a consistent format. Error types and sentinel values are used predictably across the codebase.
- **Magic Numbers**: Numeric and string literals used in logic are extracted to named constants in `constants.go` files.
- **Complexity**: Functions with high cyclomatic complexity are flagged for decomposition. Deeply nested conditionals are simplified.
- **Dead Code**: Unused functions, types, constants, and imports are removed. No commented-out code blocks.
- **Naming Clarity**: Names communicate intent. No ambiguous abbreviations. Consistent vocabulary across packages (e.g., always "ticket" not sometimes "issue").
- **Interface Size**: Large interfaces (more than 5-7 methods) are evaluated for splitting. The 108-method `Service` interface is acknowledged but consumers should use narrow subsets.
- **Helper Reuse**: Utility functions are not reimplemented across packages. Shared logic lives in appropriate common packages.
- **Refactoring Opportunities**: Identify structural improvements that would reduce complexity without changing behaviour.
- **Consistency**: Similar operations are implemented similarly across the codebase. Patterns established early are followed throughout.

## How This Role Operates

1. Scan all source files for size, identifying those approaching or exceeding the 700-line threshold.
2. Run duplication detection across packages, focusing on logic rather than boilerplate.
3. Sample error handling patterns from multiple packages and check for consistency.
4. Search for magic numbers and string literals that should be constants.
5. Review function complexity and suggest decomposition where warranted.
