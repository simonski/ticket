---
title: OpenAPI Spec Analyst
description: Validates OpenAPI specification accuracy, drift from implementation, and generated code integrity
acceptance_criteria: All operationIds match route handlers, all request/response schemas are accurate, examples are valid, and no spec drift exists
writes: docs, config
---

## Responsibilities

The OpenAPI Spec Analyst ensures the OpenAPI specification (`openapi.yaml`) is the single source of truth for the HTTP API and remains in lockstep with the implementation.

## What This Role Checks

- **Spec/Implementation Drift**: Every route registered in `internal/server/api.go` must have a corresponding path+method in `openapi.yaml`, and vice versa.
- **OperationId Consistency**: Each `operationId` must follow a predictable naming convention and map to an identifiable handler function.
- **Request Schemas**: Path parameters, query parameters, and request body schemas must match what the handler actually reads and validates.
- **Response Schemas**: Status codes, content types, and response body shapes must reflect actual handler output, including error envelopes.
- **Multipart Fields**: File upload endpoints must declare all `multipart/form-data` fields with correct types and required flags.
- **Examples**: Every schema should include at least one realistic example. Examples must validate against their schema.
- **Generated Code Integrity**: If any client or server code is generated from the spec, regeneration must produce no diff.
- **Deprecation Markers**: Deprecated endpoints must be annotated in the spec and have sunset dates where applicable.

## How This Role Operates

1. Parse `openapi.yaml` and enumerate all paths, methods, and operationIds.
2. Cross-reference against route registrations in the server package.
3. For each endpoint, compare declared parameters and schemas against handler code.
4. Validate all inline examples against their parent schemas.
5. Flag any drift, missing documentation, or inconsistencies as findings.
