# internal/context/

Semantic Compression & Spreading Activation for "Infinite Context".

## Overview

The context package implements the Context Compression system for achieving "Infinite Context" through semantic compression and logic-directed spreading activation. It targets a 100:1 compression ratio (`TargetCompressionRatio`).

Selection of what enters the window is a **hybrid**: the Mangle kernel decides via `should_include_context`, and the Go activation engine is the fallback when the kernel has no opinion or names entities the fact store cannot resolve. `GetSelectionStats()` reports the split so the drift between the two paths is measurable rather than anecdotal.

## Architecture

```
User Intent → should_include_context (kernel)  ─┐
              Spreading Activation (Go fallback)─┴→ High-Activation Facts
                                            ↓
Verbose History → Semantic Compression → Mangle Atoms
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
├── activation.go           # Spreading activation engine (score / filter / budget select)
├── activation_scoring.go   # The 9 score components + caps and clamps
├── compressor.go           # Compressor, BuildContext, activation-context refresh
├── compressor_turns.go     # ProcessTurn, compress loop, observation-masked summary
├── compressor_metrics.go   # Metrics, state, kernel-derived context, C3 mask queries
├── tokens.go               # TokenCounter + TokenBudget
├── serializer.go           # Fact serialization to Mangle + control-packet extraction
├── feedback_store.go       # ContextFeedbackStore (SQLite, third learning loop)
├── types.go                # Configuration and context types
└── *_test.go               # unit / race / kernel-rule / audit tests
```

Package-owned Mangle: none. The rules live in `internal/core/defaults/schemas_context.mg`
(declarations) and `internal/core/defaults/policy/context_compilation.mg` (C1 relevance,
C3 observation masking, C4 dependency reachability).

## Key Concepts

### Spreading Activation

Energy flows from user intent through the fact graph:

- **Recency**: Recent facts get higher activation
- **Relevance**: Facts matching intent verbs/targets
- **Dependencies**: Activation propagates through symbol graph (cap 40)
- **Campaign Context**: Campaign-related facts get boosts (cap 60)
- **Issue Context**: Issue/benchmark boosts (cap 100, keyword weights clamped to [0,1])
- **Back-reference**: Follow-up questions re-heat old turns (cap 70)
- **Feedback**: Learned predicate usefulness (−20..+20)

The caps and the keyword clamp are a safety property — without them one crafted
fact can monopolise the atom reserve. They are enforced by `activation_caps_test.go`.

### Semantic Compression

Surface text is discarded, only logical atoms retained:

```
Before: "I fixed the null pointer bug in auth.go by adding a nil check on line 42"
After:  fix_applied(/auth.go, /null_pointer, 42).
```

### Observation masking (C3)

On compression the compressor asserts `turn_age_category(TurnID, /recent|/mid|/old|/ancient)`
and then reads back the kernel's decision:

- `should_mask_observation(TurnID)` → that turn's **observation** atoms are dropped from the summary.
- `should_preserve_reasoning(TurnID)` → intent / focus / action atoms are always kept.

Go does not decide what to mask; it obeys, and refuses to mask any turn the kernel
did not also promise to preserve reasoning for.

## Budget Allocation

Default total budget is **200,000 tokens** (`DefaultConfig`); callers should
override it from `config.ContextWindow.MaxTokens`.

| Reserve | Percentage | Default | Purpose |
|---------|------------|--------:|---------|
| Core | 5% | 10,000 | Constitutional facts, schemas |
| Atom | 30% | 60,000 | High-activation context atoms |
| History | 15% | 30,000 | Compressed history + recent turns |
| Working | 50% | 100,000 | Current turn processing |

Compression triggers on utilization ≥ `CompressionThreshold` (0.60) — never on turn count.
Activation threshold is 105.0: base (50) + recency (50) alone must not carry a fact
into the window, so pure recency cannot flood it.

## Key Types

| Type | Role |
|------|------|
| `Compressor` | Orchestrator: turns, build, compress, persist |
| `ActivationEngine` | Score / filter / budget select / contexts |
| `CompressorConfig` | Budgets, thresholds, fallback priorities |
| `CompressedContext` / `CompressedTurn` | Output block and surface-free turn record |
| `ScoredFact` | Fact + 9-way score breakdown |
| `SelectionStats` | Kernel-vs-Go selection counters (dual-path drift) |
| `TokenBudget` / `TokenCounter` | Window accounting (estimates, not a provider tokenizer) |
| `FactSerializer` / `ContextBlockBuilder` | Serialization |
| `ContextFeedbackStore` / `FeedbackStats` | Learned predicate usefulness |
| `CompressedState` | Persistence snapshot |

## Serialization

Facts are serialized to Mangle notation with corpus-based ordering:

```go
serializer := NewFactSerializer()
serializer.LoadSerializationOrderFromCorpus(kernel.GetPredicateCorpus())
output := serializer.SerializeFacts(facts)
```

## Persistence

`LoadState` restores a `CompressedState` **and recalculates the token budget**, so a
rehydrated session cannot report an empty window and dump raw history it had already
compressed away. `RefreshBudget()` remains exported and idempotent for callers that
mutate state by other means.

## Observability

- `GetMetrics()` — compression ratio, masked turns, kernel/Go selection split.
- `GetSelectionStats()` — `KernelSelections`, `GoFallbacks`, `KernelInclusionRate()`, last reason.
- `GetFeedbackStats(topN)` — helpful vs noise predicates from the feedback store.
- `GetBudgetUsage()` / `GetBudgetUtilization()` — window accounting for the status bar.

Feedback DB lives at `<workspace>/.nerd/context_feedback.db`.

## Testing

```bash
go test ./internal/context/...
go test -race ./internal/context/...
```

`chat_history_parity_test.go` statically audits `cmd/nerd/chat` so no new path can
inject raw history alongside the compressed block without an `IsCompressionActive` gate.
