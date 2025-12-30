# internal/mangle/feedback - Validation & Retry Loop

This package provides a validation and retry loop for LLM-generated Mangle code. It catches common AI errors before expensive Mangle compilation and provides structured feedback for retry attempts.

**Related Packages:**
- [internal/mangle](../CLAUDE.md) - Engine consuming validated rules
- [internal/shards/system](../../shards/system/CLAUDE.md) - LegislatorShard using FeedbackLoop
- [internal/mangle/transpiler](../transpiler/CLAUDE.md) - Sanitizer for code cleanup

## Architecture

The feedback package provides:
- **PreValidator**: Fast regex-based checks before compilation
- **ErrorClassifier**: Parses compiler errors into structured feedback
- **PromptBuilder**: Progressive retry prompts with increasing constraints
- **FeedbackLoop**: Orchestrates validate-retry cycle with budget

## File Index

| File | Description |
|------|-------------|
| `loop.go` | `FeedbackLoop` orchestrating validate-retry cycle with LLM. Exports `FeedbackLoop`, `LLMClient` interface, `RuleValidator` interface, `PredicateSelectorInterface` for JIT-style selection, and `GenerateResult` with attempts/fixes tracking. |
| `types.go` | Core type definitions including `ErrorCategory` enum (10 categories) and `ValidationError`. Exports category constants (CategoryParse, CategoryAtomString, CategoryAggregation, etc.) and `ValidationBudget` for session-wide limits. |
| `error_classifier.go` | Parses Mangle compiler errors into structured `ValidationError` entries. Exports `ErrorClassifier` with regex patterns extracting line numbers, error types, and suggestions. |
| `pre_validator.go` | Fast regex-based validation catching common AI errors before compilation. Exports `PreValidator` with per-line and global checks for atom/string confusion, missing periods, aggregation syntax, etc. |
| `prompt_builder.go` | Constructs progressive feedback prompts for LLM retry attempts. Exports `PromptBuilder` with `MangleSyntaxReminder`, `BuildFeedbackPrompt()` with increasing constraints per attempt number. |
| `feedback_test.go` | Unit tests for feedback loop validation and retry logic. Tests error classification and prompt building. |

## Key Types

### ErrorCategory
```go
const (
    CategoryParse ErrorCategory = iota
    CategoryAtomString          // "string" should be /atom
    CategoryAggregation         // Missing |> do fn:
    CategoryMissingPeriod       // No terminating period
    CategoryUnboundNegation     // Variable only in negation
    CategoryUndeclaredPredicate // Unknown predicate
    CategoryStratification      // Cyclic negation
    CategoryTypeMismatch        // Float vs int
    CategoryPrologNegation      // \+ instead of !
    CategorySyntax              // General syntax
)
```

### ValidationError
```go
type ValidationError struct {
    Category   ErrorCategory
    Line       int
    Column     int
    Message    string
    Correct    string // Correct syntax example
    Suggestion string // How to fix
}
```

## Dependencies

- `internal/mangle/transpiler` - Sanitizer integration
- `internal/logging` - Structured logging

## Testing

```bash
go test ./internal/mangle/feedback/...
```

---

**Remember: Push to GitHub regularly!**
