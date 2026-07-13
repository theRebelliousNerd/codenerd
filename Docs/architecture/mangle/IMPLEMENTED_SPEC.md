# mangle — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/mangle/` (21 non-test .go, 39 tests, 1 .mg)**


## 1. Purpose

Mangle engine bindings, differential evaluation, feedback loops

## 2. Source paths

| Path | Role |
|------|------|
| `internal/mangle/` | Primary implementation |
| `Docs/architecture/mangle/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | Implemented | **85%** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 90% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Primary |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Optional |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
