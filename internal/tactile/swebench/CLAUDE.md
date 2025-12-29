# internal/tactile/swebench - SWE-bench Evaluation Harness

This package provides SWE-bench specific evaluation infrastructure, wrapping the general-purpose python.Environment for benchmark-specific functionality.

**Related Packages:**
- [internal/tactile](../CLAUDE.md) - Base execution infrastructure
- [internal/tactile/python](../python/CLAUDE.md) - General Python environment (delegated to)

## Architecture

SWE-bench is a benchmark for evaluating AI coding agents on real-world GitHub issues. This package is a **thin wrapper** over python.Environment:
- Instance-specific metadata (FAIL_TO_PASS, PASS_TO_PASS tests)
- Benchmark-specific evaluation metrics
- Reporting in SWE-bench format

The heavy lifting (containers, git, pytest) is done by python.Environment.

## File Index

| File | Description |
|------|-------------|
| `instance.go` | SWE-bench instance types mirroring HuggingFace dataset schema (princeton-nlp/SWE-bench_Lite). Exports `Instance` (InstanceID/Repo/BaseCommit/ProblemStatement/Patch/TestPatch/FailToPass/PassToPass), `Prediction`, `EvaluationResult` with resolution status and test result maps. |
| `harness.go` | SWE-bench harness wrapping python.Environment for benchmark evaluation. Exports `Harness`, `NewHarness()` converting Instance to ProjectInfo, and delegation methods (Initialize/Setup/Teardown/Reset) plus SWE-bench specific `Evaluate()` running FAIL_TO_PASS and PASS_TO_PASS tests. |
| `environment.go` | Legacy Environment type for backward compatibility (deprecated). Exports `EnvironmentState` enum and `EnvironmentConfig` struct aliasing python package types; prefer `Harness` and `python.Environment` for new code. |

## Key Types

### Instance (HuggingFace Schema)
```go
type Instance struct {
    InstanceID       string   // e.g., "django__django-11001"
    Repo             string   // e.g., "django/django"
    BaseCommit       string   // Git commit to start from
    ProblemStatement string   // Issue description (NL)
    Patch            string   // Gold fix patch (validation only)
    FailToPass       []string // Tests that should flip fail→pass
    PassToPass       []string // Tests that should remain passing
}
```

### EvaluationResult
```go
type EvaluationResult struct {
    InstanceID        string
    Resolved          bool // All tests pass criteria met
    FailToPassResults map[string]TestResult
    PassToPassResults map[string]TestResult
    Duration          time.Duration
}
```

## Evaluation Flow

1. Load Instance from HuggingFace JSON
2. Create Harness wrapping python.Environment
3. Initialize → Setup → ApplyPatch
4. Run FAIL_TO_PASS tests (must flip from fail to pass)
5. Run PASS_TO_PASS tests (must remain passing)
6. Resolved = all criteria met

## Dependencies

- `internal/tactile/python` - General Python environment
- `internal/tactile` - PersistentDockerExecutor

## Testing

```bash
go test ./internal/tactile/swebench/...
```

---

**Remember: Push to GitHub regularly!**
