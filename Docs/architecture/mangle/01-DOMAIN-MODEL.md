# mangle — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/mangle/` (21 non-test .go, 39 tests, 1 .mg)**


## Source package

`internal/mangle/`

## Exported / primary types (sampled)

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

## Planned vs actual Mangle surface

| Artifact | Count | Notes |
|----------|-------|-------|
| `.mg` files under package | 1 | See inventory |
| Core schemas (global) | shared | `internal/core/defaults/schemas.mg` when kernel-touching |
| Policy modules (global) | shared | `internal/core/defaults/policy/` |

### Package-local Mangle inventory (top)

| Path | Lines |
|------|-------|
| `internal/mangle/intent_routing.mg` | 414 |

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package's position: **Mangle engine bindings, differential evaluation, feedback loops**

## Data & control concepts

- Primary language surface: Go under `internal/mangle/`
- Logic surface: Mangle where listed above or via kernel defaults
- External effects: VirtualStore / tools / CLI depending on package
