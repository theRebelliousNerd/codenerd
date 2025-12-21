# internal/verification - Quality Enforcement Loop

This package implements the quality-enforcing verification loop ensuring tasks are completed properly with no shortcuts, mock code, or corner-cutting.

**Related Packages:**
- [internal/autopoiesis](../autopoiesis/CLAUDE.md) - Orchestrator for corrective tool generation
- [internal/shards/researcher](../shards/researcher/CLAUDE.md) - Research corrective actions
- [internal/store](../store/CLAUDE.md) - LocalStore for verification state

## Architecture

After shard execution, results are verified and automatically retried with corrective action until success or max retries. Detects quality violations like:
- Mock/placeholder code
- Hallucinated APIs
- Incomplete implementations
- Empty functions
- Fake tests

## File Index

| File | Description |
|------|-------------|
| `verifier.go` | Core `TaskVerifier` implementing quality enforcement with corrective action loop. Exports `QualityViolation` enum (MockCode/PlaceholderCode/HallucinatedAPI/IncompleteImpl/EmptyFunction/FakeTests), `CorrectiveAction` (Research/Docs/Tool/Decompose), `VerificationResult`, and `Verify()` with auto-retry. |
| `verifier_test.go` | Unit tests for quality violation detection. Tests mock code detection, placeholder identification, and corrective action selection. |

## Key Types

### QualityViolation
```go
const (
    MockCode        QualityViolation = "mock_code"
    PlaceholderCode QualityViolation = "placeholder"
    HallucinatedAPI QualityViolation = "hallucinated_api"
    IncompleteImpl  QualityViolation = "incomplete"
    HardcodedValues QualityViolation = "hardcoded"
    EmptyFunction   QualityViolation = "empty_function"
    MissingErrors   QualityViolation = "missing_errors"
    FakeTests       QualityViolation = "fake_tests"
)
```

### CorrectiveAction
```go
type CorrectiveAction struct {
    Type      CorrectiveType // research, docs, tool, decompose
    Query     string
    Reason    string
    ShardHint string
}
```

## Verification Flow

1. Execute shard task
2. Verify result for quality violations
3. If violations detected:
   - Determine corrective action type
   - Execute corrective action (research, fetch docs, generate tool)
   - Retry with enriched context
4. Repeat until success or max retries

## Dependencies

- `internal/autopoiesis` - Tool generation for CorrectiveTool
- `internal/perception` - LLMClient for verification
- `internal/shards/researcher` - Research corrective actions
- `internal/store` - LocalStore for state

## Testing

```bash
go test ./internal/verification/...
```
