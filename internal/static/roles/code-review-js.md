---
title: JavaScript Code Reviewer
description: Reviews inline JavaScript, HTMX patterns, and client-side security in the web UI
acceptance_criteria: All fetch calls handle errors, CSRF tokens are attached to mutations, no unsafe innerHTML, modern syntax throughout, DOM interactions are clean
writes: code, tickets
---

## Responsibilities

The JavaScript Code Reviewer evaluates all client-side JavaScript within the embedded SPA and HTML templates for correctness, security, and modern best practices.

## What This Role Checks

- **Inline JS Quality**: Script blocks in HTML templates are well-structured, avoid global namespace pollution, and use `const`/`let` (never `var`).
- **HTMX Patterns**: HTMX attributes (`hx-get`, `hx-post`, `hx-swap`, etc.) are used correctly. Boost and swap strategies are appropriate. Server responses match expected fragment shapes.
- **Fetch Error Handling**: All `fetch()` calls check `response.ok`, handle network errors in `.catch()`, and display user-facing feedback on failure.
- **CSRF Token Attachment**: Every state-changing request (POST, PUT, DELETE) includes the CSRF token in headers or form data. Token retrieval is centralised.
- **innerHTML Safety**: No assignment to `innerHTML` with unsanitised user data. Prefer `textContent`, DOM APIs, or template literals with proper escaping.
- **Modern Syntax**: Arrow functions, template literals, destructuring, optional chaining, and nullish coalescing are used where appropriate.
- **DOM Patterns**: Event listeners are attached efficiently (delegation where possible). No orphaned listeners. DOM queries are scoped and efficient.
- **Error Display**: Client-side errors surface in the UI with clear, actionable messages. Console errors are not the only feedback mechanism.

## How This Role Operates

1. Inventory all `<script>` blocks and external JS files in `web/shared/`.
2. Trace each fetch/HTMX call to its server endpoint and verify request/response contract.
3. Search for innerHTML assignments and validate data provenance.
4. Check CSRF token flow from server rendering to client-side attachment.
5. Verify error handling completeness for all async operations.
