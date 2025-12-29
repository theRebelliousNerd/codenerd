# internal/context - Semantic Compression & Spreading Activation

This package implements the Context Compression system for achieving "Infinite Context" (§8.2) through semantic compression and logic-directed spreading activation.

**Related Packages:**
- [internal/core](../core/CLAUDE.md) - Kernel consuming compressed context
- [internal/prompt](../prompt/CLAUDE.md) - JIT compiler using context budget allocation
- [internal/store](../store/CLAUDE.md) - LocalStore for compression state

## Architecture

The context package achieves 100:1+ compression by:
- **Spreading Activation** (§8.1): Energy flows from user intent through fact graph
- **Semantic Compression** (§8.2): Surface text discarded, only logical atoms retained
- **Token Budget Management**: Prioritized allocation across tiers (core, atoms, history, working)
- **Corpus-Aware Ordering**: Serialization order from predicate_corpus.db

## File Index

| File | Description |
|------|-------------|
| `activation.go` | Spreading Activation Engine implementing §8.1 Logic-Directed Context with campaign/issue/session awareness. Exports `ActivationEngine`, `CampaignActivationContext`, `IssueActivationContext` for tiered file relevance and dependency-based spreading. |
| `compressor.go` | Context Compressor orchestrating §8.2 Infinite Context via semantic compression. Exports `Compressor` with kernel, store, LLM dependencies, `CompressedTurn`, and rolling summary state for >100:1 compression ratio. |
| `tokens.go` | Token counting utilities calibrated for Claude's tokenizer (~4 chars/token). Exports `TokenCounter` with `CountString()`, `CountFact()`, `CountTurn()` for budget management. |
| `serializer.go` | Fact serialization to Mangle notation for LLM context injection. Exports `FactSerializer` with predicate grouping, corpus-based ordering via `LoadSerializationOrderFromCorpus()`, and comment generation options. |
| `types.go` | Configuration types for compression including `CompressorConfig` with 128k token budget allocation. Exports `DefaultConfig()` with predicate priorities (deprecated in favor of corpus). |
| `activation_test.go` | Unit tests for spreading activation scoring and dependency propagation. Tests issue context tiering and campaign-aware boosts. |
| `serializer_test.go` | Unit tests for fact serialization with grouping and corpus ordering. Tests Mangle notation output formatting. |

## Key Types

### ActivationEngine
```go
type ActivationEngine struct {
    config              CompressorConfig
    factTimestamps      map[string]time.Time
    dependencies        map[string][]string
    symbolGraph         map[string][]string
    campaignContext     *CampaignActivationContext
    issueContext        *IssueActivationContext
    corpusPriorities    map[string]int
}
```

### CompressorConfig
```go
type CompressorConfig struct {
    TotalBudget          int     // 128k tokens default
    CoreReserve          int     // 5% for constitutional facts
    AtomReserve          int     // 30% for high-activation atoms
    HistoryReserve       int     // 15% for compressed history
    WorkingReserve       int     // 50% for current turn
    CompressionThreshold float64 // Trigger at 60% usage
}
```

## Budget Allocation

| Reserve | Percentage | Purpose |
|---------|------------|---------|
| Core | 5% | Constitutional facts, schemas |
| Atom | 30% | High-activation context atoms |
| History | 15% | Compressed history + recent turns |
| Working | 50% | Current turn processing |

## Dependencies

- `internal/core` - Kernel for fact queries
- `internal/perception` - LLMClient for compression
- `internal/store` - LocalStore for persistence

## Testing

```bash
go test ./internal/context/...
```

---

**Remember: Push to GitHub regularly!**
