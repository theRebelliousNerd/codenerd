# Unit Tests

## Purpose
Validate pure logic and small components with fast, deterministic tests.

## Scope
- Single package, minimal dependencies, no network.
- Prefer in-memory objects and small fixtures.

## Patterns
- Table-driven tests with t.Run.
- Use subtests for scenarios; name cases with stable, descriptive strings.
- For public APIs, use package foo_test; for internal packages, use package foo when needed.

## Assertions
- Always check err; assert type via errors.Is or errors.As.
- Validate outputs fully; compare structs with reflect.DeepEqual or custom comparisons.
- Verify side effects (state changes) explicitly.

## Edge cases
- Empty, nil, zero, max values.
- Invalid inputs and malformed data.
- Unicode and special characters.
- Concurrency safety for shared state when applicable.

## Fixtures and helpers
- Use t.Helper on helpers.
- Use t.TempDir for temp files.
- Avoid global mutable state; reset via t.Cleanup.

## AegisDB focus areas
- Mangle rule parsing and evaluation
- Graph traversal invariants
- Vector distance calculations and thresholds
- Wormhole score calculations and cache key derivation

## Skeleton
```go
func TestXxx(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "empty", input: "", want: "", wantErr: true},
    }

    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            got, err := Xxx(tc.input)
            if tc.wantErr {
                if err == nil {
                    t.Fatalf("expected error")
                }
                return
            }
            if err != nil {
                t.Fatalf("Xxx() error: %v", err)
            }
            if got != tc.want {
                t.Fatalf("Xxx() = %q, want %q", got, tc.want)
            }
        })
    }
}
```

## Pitfalls
- Do not hide errors.
- Do not soften assertions to pass.
- Do not rely on time.Now or randomness without control.
