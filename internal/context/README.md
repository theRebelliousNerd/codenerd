# internal/context/

Semantic Compression & Spreading Activation for "Infinite Context".

**Architecture Version:** 2.0.0 (December 2024 - JIT-Driven)

## Overview

The context package implements the Context Compression system for achieving "Infinite Context" through semantic compression and logic-directed spreading activation. It achieves 100:1+ compression ratios.

## Architecture

```
User Intent → Spreading Activation → High-Activation Facts
                                            ↓
Verbose History → Semantic Compression → Mangle Atoms (100:1 ratio)
                                            ↓
                                   Token Budget Manager
                                            ↓
                           ┌────────┬───────┬────────┬─────────┐
                           Core    Atoms   History  Working
                           (5%)    (30%)   (15%)    (50%)
```

## Structure

```
context/
├── activation.go      # Spreading activation engine
├── compressor.go      # Semantic compression orchestrator
├── tokens.go          # Token counting utilities
├── serializer.go      # Fact serialization to Mangle
├── types.go           # Configuration types
├── activation_test.go
└── serializer_test.go
```

## Key Concepts

### Spreading Activation

Energy flows from user intent through the fact graph:

- **Recency**: Recent facts get higher activation
- **Relevance**: Facts matching intent verbs/targets
- **Dependencies**: Activation propagates through symbol graph
- **Campaign Context**: Campaign-related facts get boosts

### Semantic Compression

Surface text is discarded, only logical atoms retained:

```
Before: "I fixed the null pointer bug in auth.go by adding a nil check on line 42"
After:  fix_applied(/auth.go, /null_pointer, 42).
```

## Budget Allocation

| Reserve | Percentage | Purpose |
|---------|------------|---------|
| Core | 5% | Constitutional facts, schemas |
| Atom | 30% | High-activation context atoms |
| History | 15% | Compressed history + recent turns |
| Working | 50% | Current turn processing |

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

## Serialization

Facts are serialized to Mangle notation with corpus-based ordering:

```go
serializer := NewFactSerializer()
serializer.LoadSerializationOrderFromCorpus("predicate_corpus.db")
output := serializer.Serialize(facts)
```

## Testing

```bash
go test ./internal/context/...
```

---

**Last Updated:** December 2024
