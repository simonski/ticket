---
name: qa
description: Use this skill when writing or improving tests. Applies when the user asks to write unit tests, integration tests, fuzz tests, or improve test coverage for any package.
---

# QA Test Writing Skill

Write the best possible tests for this Go codebase. Tests should be exhaustive, well-isolated, fast, and reflect how the code is actually used.

## Core Principle

**Tests are first-class code.** A test that is flaky, slow, or poorly isolated is worse than no test. Prefer fewer, well-crafted tests over many shallow ones.

## Phase 1: Orient Before Writing

Before writing a single test, gather context:

```bash
# Count what exists
grep -r 'func Test' --include='*_test.go' . | wc -l
grep -r 'func Fuzz' --include='*_test.go' . | wc -l

# Identify untested areas
go test ./... -v 2>&1 | grep -E 'FAIL|no test files'

# Check coverage per package
go test ./... -coverprofile=cover.out && go tool cover -func=cover.out | tail -20
```

Read `libtickettest/contract.go` — it is the canonical contract test suite. New `Service` methods **must** be tested there, not just in local_test.go.

Read the QA report at `reports/07-qa.md` to see known gaps before writing redundant tests.

## Phase 2: Coverage Thresholds (mandatory)

Thresholds are enforced by `make test-go-cover`:

| Package | Minimum |
|---------|---------|
| `cmd/tk` | 55% |
| `libticket` | 65% |
| `libtickethttp` | 75% |
| `internal/client` | 55% |
| `internal/store` | 70% |
| `internal/config` | 70% |

Work on packages below threshold first. Run `make test-go-cover` after adding tests to verify.

## Phase 3: Unit Tests

### Isolation rules (non-negotiable)

- **Always** use `t.TempDir()` for any file or SQLite database. Never use `/tmp/` with a hardcoded name.
- **Always** use `t.Setenv()` instead of `os.Setenv()` so env vars are cleaned up.
- **Never** use `time.Sleep`. Use `t.Cleanup` + channels, or poll with a deadline.
- Each test must be independent: no shared global state, no ordering dependencies.

### Store layer (`internal/store/`)

Pattern used throughout this codebase:

```go
func TestMyFeature(t *testing.T) {
    dbPath := filepath.Join(t.TempDir(), "ticket.db")
    store, err := store.New(dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()

    // arrange — create minimal prerequisites
    proj := store.CreateProject(store.CreateProjectParams{Prefix: "TST", Name: "Test"})

    // act
    result, err := store.MyMethod(params)

    // assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Field != expected {
        t.Errorf("want %v, got %v", expected, result.Field)
    }
}
```

Test **error paths** as much as success paths:
- Duplicate creation (unique constraint)
- Update/delete of non-existent ID
- Invalid foreign key references
- Empty/nil inputs
- Boundary values (empty string, max length string, 0, negative numbers)

### Config / password layer

These are pure functions. Use table-driven tests:

```go
func TestHashAndVerify(t *testing.T) {
    tests := []struct{
        name     string
        password string
        wantErr  bool
    }{
        {"normal", "correct-horse", false},
        {"empty", "", false},
        {"unicode", "pässwörd🔑", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hash, err := password.Hash(tt.password)
            // ...
        })
    }
}
```

## Phase 4: Integration Tests

### API tests (`internal/server/api_test.go`)

Use the project's helper pattern:

```go
func TestMyEndpointAPI(t *testing.T) {
    h := testHandler(t)                     // sets up real server + DB

    // POST with JSON body
    var resp MyResponseType
    doJSONRequest(t, h, http.MethodPost, "/api/my-endpoint",
        MyRequestBody{Field: "value"}, &resp)

    if resp.ID == 0 {
        t.Error("expected non-zero ID")
    }

    // Auth: verify unauthenticated requests are rejected
    doJSONRequestUnauth(t, h, http.MethodPost, "/api/my-endpoint",
        MyRequestBody{}, nil)
}
```

Every new API endpoint needs:
1. Happy-path test
2. Auth test (unauthenticated → 401)
3. Validation test (bad input → 400)
4. Not-found test (unknown ID → 404)
5. Permission test (wrong role/project → 403)

### Contract tests (`libtickettest/contract.go`)

Every new method on `libticket.Service` **must** be added to `contract.go`. Both the `LocalService` (local_test.go) and `libtickethttp.Service` (http_test.go) run the same suite.

Structure for a new contract section:

```go
func (s *Suite) TestMyFeatureCRUD(t *testing.T) {
    ctx := context.Background()
    svc := s.Factory(t)

    t.Run("create", func(t *testing.T) { ... })
    t.Run("get", func(t *testing.T) { ... })
    t.Run("update", func(t *testing.T) { ... })
    t.Run("delete", func(t *testing.T) { ... })
    t.Run("not-found", func(t *testing.T) { ... })
    t.Run("duplicate", func(t *testing.T) { ... })
}
```

### CLI tests (`cmd/tk/main_test.go`)

Use `setupLocalCLI` to run the binary in a temp directory:

```go
func TestMyCommand(t *testing.T) {
    tk := setupLocalCLI(t)

    out := tk.run("my-command", "--flag", "value")
    if !strings.Contains(out, "expected output") {
        t.Errorf("unexpected output: %s", out)
    }

    // Test error case
    tk.runFails("my-command", "--invalid-flag")
}
```

## Phase 5: Fuzz Tests

Add fuzz tests for any function that:
- Parses external input (JSON, URLs, config files, ticket keys)
- Validates user-supplied strings
- Performs string/byte manipulation

### Pattern

```go
func FuzzParseTicketKey(f *testing.F) {
    // seed corpus — known valid and boundary cases
    f.Add("CUS-T-1")
    f.Add("CUS-T-0")
    f.Add("")
    f.Add("a")
    f.Add("CUS-T-99999999")
    f.Add("'; DROP TABLE tickets; --")
    f.Add("<script>alert(1)</script>")
    f.Add(strings.Repeat("A", 1000))

    f.Fuzz(func(t *testing.T, input string) {
        // should never panic
        result, err := ParseTicketKey(input)
        if err != nil {
            return // errors are fine
        }
        // if it parses successfully, invariants must hold
        if result.Prefix == "" {
            t.Error("parsed key must have non-empty prefix")
        }
    })
}
```

### High-value fuzz targets in this codebase

| Function | File | Why |
|----------|------|-----|
| `ParseTicketKey` / key parsing | `internal/store/keys.go` | User input, string parsing |
| `ResolveLocation` | `internal/config/config.go` | Path/URL parsing |
| JSON request body unmarshalling | `internal/server/api_helpers.go` | Network input |
| `Hash` / `Verify` | `internal/password/hash.go` | User input, unicode edge cases |
| Label `color` validation | `internal/store/` | CSS injection surface |
| Project prefix validation | `internal/store/keys.go` | Regex validation |

Run fuzz tests locally:
```bash
go test ./internal/store/ -fuzz=FuzzParseTicketKey -fuzztime=30s
```

Committed fuzz corpus lives in `testdata/fuzz/` alongside the test file.

## Phase 6: What Makes a Test Great

### Do
- Test **behavior**, not implementation. Call the public API; don't reach into private fields.
- Give each `t.Run` a descriptive name: `t.Run("returns 404 when ticket not found", ...)`
- Assert **all** relevant fields, not just the one you care about.
- Use `t.Helper()` in any shared helper function so failures point to the caller.
- Use `cmp.Diff` (from `github.com/google/go-cmp`) for struct comparison to get readable diffs.
- Test **error messages** as well as error presence — `errors.Is`, not just `err != nil`.

### Don't
- Don't add `t.Parallel()` to tests that share SQLite files or `t.Setenv`.
- Don't use `assert` libraries that swallow context — use `t.Fatalf` / `t.Errorf` with clear messages.
- Don't test the database schema directly — test via the store methods.
- Don't mock when you can use a real SQLite in-memory or `t.TempDir` database.
- Don't leave commented-out tests or `t.Skip` without a linked issue.

## Phase 7: Running and Verifying

```bash
# All tests
make test-go

# Single package
go test ./internal/store/ -v -run TestMyFeature

# With race detector (always run before submitting)
go test -race ./...

# Coverage for a specific package
go test ./internal/store/ -coverprofile=c.out && go tool cover -html=c.out

# Enforce thresholds (will fail below minimums)
make test-go-cover

# Fuzz (30 seconds per target)
go test ./internal/store/ -fuzz=FuzzMyTarget -fuzztime=30s
```

After adding tests, verify:
1. `make test-go` passes (no regressions)
2. `make test-go-cover` passes (thresholds met)
3. `go test -race ./...` passes (no data races)
