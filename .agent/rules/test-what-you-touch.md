---
trigger: always_on
description: Every code change needs test coverage
---

# Test What You Touch

> **Every code change deserves a test change.**

## Required Tests

| Change Type | Required Test |
|-------------|---------------|
| New function | Unit test |
| New endpoint | Integration test |
| Bug fix | Regression test |
| Refactor | Verify existing + add missing |
| New error path | Error path test |

## Coverage Requirements

```go
func TestMyFunction_Success(t *testing.T) { ... }      // Happy path
func TestMyFunction_InvalidInput(t *testing.T) { ... } // Error path
func TestMyFunction_EdgeCases(t *testing.T) { ... }    // Boundaries
```

## Test Quality

- **Deterministic** – No flaky tests
- **Fast** – Slow tests don't get run
- **Isolated** – No shared state
- **Named clearly** – `TestX_WhenY_ShouldZ`
- **Assert behavior** – Not just "no panic"

## For Every Modified File

1. Is there a `_test.go` file? → Create one if not
2. Does it cover my changes? → Add coverage if not
3. Did I run the tests? → Run them now

> If you wrote it, test it.
