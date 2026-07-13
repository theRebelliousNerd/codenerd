# mangle — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/mangle/` (complete internal coverage)
> **Implementation: `internal/mangle/` — 21 non-test .go, 39 tests, 1 .mg**


## 1. Purpose

Mangle engine bindings, differential evaluation, generation feedback

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/mangle/` | Primary implementation |
| `Docs/architecture/mangle/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (21 src / 39 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/mangle/engine.go` | 1100 | source |
| `internal/mangle/lsp.go` | 1055 | source |
| `internal/mangle/differential.go` | 866 | source |
| `internal/mangle/grammar.go` | 787 | source |
| `internal/mangle/proof_tree.go` | 482 | source |
| `internal/mangle/feedback/loop.go` | 476 | source |
| `internal/mangle/feedback/prompt_builder.go` | 446 | source |
| `internal/mangle/synth/compile.go` | 424 | source |
| `internal/mangle/schema_validator.go` | 412 | source |
| `internal/mangle/feedback/pre_validator.go` | 402 | source |
| `internal/mangle/transpiler/sanitizer.go` | 379 | source |
| `internal/mangle/synth/validate.go` | 330 | source |
| `internal/mangle/feedback/types.go` | 253 | source |
| `internal/mangle/feedback/error_classifier.go` | 252 | source |
| `internal/mangle/synth/schema.go` | 213 | source |
| `internal/mangle/synth/decoder.go` | 169 | source |
| `internal/mangle/synth/spec.go` | 122 | source |
| `internal/mangle/feedback/normalize.go` | 76 | source |
| `internal/mangle/simd_intersect_amd64.go` | 51 | source |
| `internal/mangle/parse_lock.go` | 44 | source |

### Types (sampled)

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

### Functions (sampled)

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

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Primary |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
