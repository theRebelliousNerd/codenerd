# internal/transparency - Visibility Into Internal Operations

This package provides visibility into codeNERD's internal operations, making the "magic" visible to users on demand.

**Related Packages:**
- [internal/mangle](../mangle/CLAUDE.md) - DerivationTrace for proof trees
- [internal/shards/system](../shards/system/CLAUDE.md) - Shard execution being observed
- [internal/config](../config/CLAUDE.md) - TransparencyConfig

## Architecture

The transparency layer operates as a parallel concern:
- **ShardObserver**: Tracks shard execution phases in real-time
- **SafetyReporter**: Explains constitutional gate blocks
- **Explainer**: Builds human-readable explanations from derivation traces
- **ErrorClassifier**: Categorizes errors with remediation suggestions

## File Index

| File | Description |
|------|-------------|
| `doc.go` | Package documentation describing transparency layer principles: opt-in, non-intrusive, lazy, informative. Lists key capabilities: shard phases, safety explanations, JIT explain mode, proof trees, operation summaries, error categorization. |
| `transparency.go` | Central `TransparencyManager` coordinating all transparency features. Exports `TransparencyManager` with Enable/Disable/Toggle methods, configurable ShardObserver and SafetyReporter components. |
| `shard_observer.go` | Real-time shard execution phase tracking with status lines. Exports `ShardPhase` enum (Idle→Complete), `ShardExecution` with Duration/StatusLine, `PhaseUpdate` for notifications, and `ShardObserver` managing active executions. |
| `safety_reporter.go` | Safety gate block tracking with explanations and remediation. Exports `SafetyViolationType` enum (6 types), `SafetyViolation` with rule/target/explanation, and `SafetyReporter` maintaining violation history. |
| `explainer.go` | Human-readable explanations from Mangle derivation traces. Exports `Explainer` with configurable depth/detail, and `ExplainTrace()` converting DerivationTrace to markdown with query, results, and premises. |
| `error_classifier.go` | Error categorization with remediation suggestions. Exports `ErrorCategory` enum (9 categories: safety, config, api, kernel, shard, filesystem, network, timeout, unknown), `ClassifiedError` with remediation, and `ClassifyError()` pattern matching. |

## Key Types

### ShardPhase
```go
const (
    PhaseIdle ShardPhase = iota
    PhaseInitializing
    PhaseLoading
    PhaseAnalyzing
    PhaseGenerating
    PhaseExecuting
    PhaseComplete
    PhaseFailed
)
```

### SafetyViolationType
```go
const (
    ViolationDestructiveAction SafetyViolationType = iota
    ViolationProtectedPath
    ViolationSecretExposure
    ViolationResourceLimit
    ViolationPolicyRule
    ViolationUnauthorized
)
```

## Design Principles

1. **Opt-in**: All features toggled via TransparencyConfig
2. **Non-intrusive**: Does not modify core execution path
3. **Lazy**: Expensive computations (proof trees) only run when requested
4. **Informative**: Explains "why" not just "what"

## Dependencies

- `internal/mangle` - DerivationTrace types
- `internal/config` - TransparencyConfig

## Testing

```bash
go test ./internal/transparency/...
```
