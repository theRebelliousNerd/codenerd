# mangle — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mangle/` (complete internal coverage)
> **Implementation: `internal/mangle/` — 21 non-test .go, 39 tests, 1 .mg**


## Package

`internal/mangle/`

## Exported types (sampled, up to 40)

| Type | Location |
|------|----------|
| `KnowledgeGraph` | `internal/mangle/differential.go:79` |
| `DifferentialEngine` | `internal/mangle/differential.go:94` |
| `ChainedFactStore` | `internal/mangle/differential.go:146` |
| `FactStoreProxy` | `internal/mangle/differential.go:800` |
| `Config` | `internal/mangle/engine.go:33` |
| `Engine` | `internal/mangle/engine.go:61` |
| `Fact` | `internal/mangle/engine.go:82` |
| `QueryResult` | `internal/mangle/engine.go:120` |
| `Stats` | `internal/mangle/engine.go:126` |
| `Persistence` | `internal/mangle/engine.go:133` |
| `ErrorClassifier` | `internal/mangle/feedback/error_classifier.go:20` |
| `LLMClient` | `internal/mangle/feedback/loop.go:19` |
| `TracingLLMClient` | `internal/mangle/feedback/loop.go:27` |
| `RuleValidator` | `internal/mangle/feedback/loop.go:34` |
| `PredicateSelectorInterface` | `internal/mangle/feedback/loop.go:46` |
| `PredicateCatalogProvider` | `internal/mangle/feedback/loop.go:51` |
| `FeedbackLoop` | `internal/mangle/feedback/loop.go:56` |
| `GenerateResult` | `internal/mangle/feedback/loop.go:100` |
| `PreValidator` | `internal/mangle/feedback/pre_validator.go:43` |
| `PromptBuilder` | `internal/mangle/feedback/prompt_builder.go:15` |
| `ErrorCategory` | `internal/mangle/feedback/types.go:13` |
| `ValidationError` | `internal/mangle/feedback/types.go:79` |
| `ValidationResult` | `internal/mangle/feedback/types.go:91` |
| `RetryConfig` | `internal/mangle/feedback/types.go:106` |
| `ValidationBudget` | `internal/mangle/feedback/types.go:144` |
| `ErrorPattern` | `internal/mangle/feedback/types.go:218` |
| `FeedbackContext` | `internal/mangle/feedback/types.go:229` |
| `OutputProtocol` | `internal/mangle/feedback/types.go:240` |
| `SynthMode` | `internal/mangle/feedback/types.go:247` |
| `AtomValidator` | `internal/mangle/grammar.go:20` |
| `PredicateSpec` | `internal/mangle/grammar.go:29` |
| `ArgSpec` | `internal/mangle/grammar.go:36` |
| `ArgType` | `internal/mangle/grammar.go:43` |
| `ValidationResult` | `internal/mangle/grammar.go:55` |
| `ValidationError` | `internal/mangle/grammar.go:63` |
| `ErrorSeverity` | `internal/mangle/grammar.go:70` |
| `RepairLoop` | `internal/mangle/grammar.go:719` |
| `LSPServer` | `internal/mangle/lsp.go:23` |
| `Document` | `internal/mangle/lsp.go:34` |
| `Definition` | `internal/mangle/lsp.go:42` |

## Exported functions/methods (sampled, up to 30)

| Symbol | Location |
|--------|----------|
| `NewKnowledgeGraph` | `internal/mangle/differential.go:86` |
| `Add` | `internal/mangle/differential.go:151` |
| `ListPredicates` | `internal/mangle/differential.go:155` |
| `EstimateFactCount` | `internal/mangle/differential.go:178` |
| `GetFacts` | `internal/mangle/differential.go:186` |
| `Contains` | `internal/mangle/differential.go:210` |
| `Merge` | `internal/mangle/differential.go:222` |
| `Snapshot` | `internal/mangle/differential.go:230` |
| `NewDifferentialEngine` | `internal/mangle/differential.go:261` |
| `AddFactIncremental` | `internal/mangle/differential.go:361` |
| `EnableUnifiedFastPath` | `internal/mangle/differential.go:378` |
| `UnifiedFastPathEnabled` | `internal/mangle/differential.go:400` |
| `ApplyAtomDelta` | `internal/mangle/differential.go:417` |
| `CopyAllFactsTo` | `internal/mangle/differential.go:502` |
| `ApplyDelta` | `internal/mangle/differential.go:541` |
| `Query` | `internal/mangle/differential.go:676` |
| `NewFactStoreProxy` | `internal/mangle/differential.go:805` |
| `RegisterLoader` | `internal/mangle/differential.go:812` |
| `GetFacts` | `internal/mangle/differential.go:817` |
| `RegisterVirtualPredicate` | `internal/mangle/differential.go:829` |
| `DefaultConfig` | `internal/mangle/engine.go:43` |
| `String` | `internal/mangle/engine.go:90` |
| `NewEngine` | `internal/mangle/engine.go:140` |
| `GetPersistence` | `internal/mangle/engine.go:154` |
| `ToggleAutoEval` | `internal/mangle/engine.go:160` |
| `RecomputeRules` | `internal/mangle/engine.go:168` |
| `ShouldPushdown` | `internal/mangle/engine.go:209` |
| `ShouldQuery` | `internal/mangle/engine.go:210` |
| `ExecuteQuery` | `internal/mangle/engine.go:214` |
| `GetDerivedFactCount` | `internal/mangle/engine.go:274` |

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 1 |

| Path | Lines |
|------|------:|
| `internal/mangle/intent_routing.mg` | 414 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **Mangle engine bindings, differential evaluation, generation feedback**
