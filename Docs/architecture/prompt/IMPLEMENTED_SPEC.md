# prompt — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/prompt/` (complete internal coverage)
> **Implementation: `internal/prompt/` — 25 non-test .go, 32 tests, 0 .mg**


## 1. Purpose

JIT prompt compiler, atoms, selector, budget, resolver

## 2. Source paths (1:1)

| Path | Role |
|------|------|
| `internal/prompt/` | Primary implementation |
| `Docs/architecture/prompt/` | This full corpus |

## 3. Implementation Status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global-only | **n/a** |
| Full architecture corpus | Implemented | **100%** |

**Overall (heuristic): 90%** as living package (25 src / 32 tests)

## 4. Public surface inventory

### Largest files

| Path | Lines |
|------|------:|
| `internal/prompt/compiler.go` | 1185 | source |
| `internal/prompt/selector.go` | 1175 | source |
| `internal/prompt/loader.go` | 907 | source |
| `internal/prompt/budget.go` | 897 | source |
| `internal/prompt/predicate_selector.go` | 724 | source |
| `internal/prompt/context.go` | 701 | source |
| `internal/prompt/atoms.go` | 684 | source |
| `internal/prompt/assembler.go` | 634 | source |
| `internal/prompt/compiler_db.go` | 419 | source |
| `internal/prompt/resolver.go` | 410 | source |
| `internal/prompt/loader_embedding.go` | 355 | source |
| `internal/prompt/config_factory.go` | 352 | source |
| `internal/prompt/embedded.go` | 227 | source |
| `internal/prompt/evolved_atoms.go` | 209 | source |
| `internal/prompt/sync/synchronizer.go` | 193 | source |
| `internal/prompt/compiler_specialists.go` | 179 | source |
| `internal/prompt/vector_searcher.go` | 178 | source |
| `internal/prompt/query_expansion.go` | 154 | source |
| `internal/prompt/default_corpus.go` | 146 | source |
| `internal/prompt/baseline.go` | 114 | source |

### Types (sampled)

| Type | Location |
|------|----------|
| `FinalAssembler` | `internal/prompt/assembler.go:21` |
| `TemplateEngine` | `internal/prompt/assembler.go:298` |
| `TemplateFunc` | `internal/prompt/assembler.go:305` |
| `AssemblyOptions` | `internal/prompt/assembler.go:457` |
| `PromptStats` | `internal/prompt/assembler.go:592` |
| `AtomCategory` | `internal/prompt/atoms.go:27` |
| `PromptAtom` | `internal/prompt/atoms.go:135` |
| `EmbeddedCorpus` | `internal/prompt/atoms.go:603` |
| `AtomStore` | `internal/prompt/atoms.go:663` |
| `BudgetPriority` | `internal/prompt/budget.go:18` |
| `CategoryBudget` | `internal/prompt/budget.go:56` |
| `TokenBudgetManager` | `internal/prompt/budget.go:82` |
| `AllocationStrategy` | `internal/prompt/budget.go:95` |
| `BudgetReport` | `internal/prompt/budget.go:812` |
| `CategoryUsage` | `internal/prompt/budget.go:823` |
| `Fact` | `internal/prompt/compiler.go:24` |
| `KernelQuerier` | `internal/prompt/compiler.go:31` |
| `KernelRetracter` | `internal/prompt/compiler.go:41` |
| `VectorSearcher` | `internal/prompt/compiler.go:46` |
| `SearchResult` | `internal/prompt/compiler.go:55` |
| `CompilationStats` | `internal/prompt/compiler.go:62` |
| `CompilationResult` | `internal/prompt/compiler.go:207` |
| `JITPromptCompiler` | `internal/prompt/compiler.go:249` |
| `CompilerConfig` | `internal/prompt/compiler.go:306` |
| `CompilerStats` | `internal/prompt/compiler.go:1117` |
| `CompilerOption` | `internal/prompt/compiler_options.go:8` |
| `ConfigAtom` | `internal/prompt/config_factory.go:13` |
| `ConfigAtomProvider` | `internal/prompt/config_factory.go:49` |
| `ConfigFactory` | `internal/prompt/config_factory.go:54` |
| `DefaultConfigAtomProvider` | `internal/prompt/config_factory.go:152` |
| `SimpleRegistry` | `internal/prompt/config_registry.go:6` |
| `CompilationContext` | `internal/prompt/context.go:19` |
| `ContextDimension` | `internal/prompt/context.go:408` |
| `FactStyle` | `internal/prompt/context.go:555` |
| `EvolvedAtomManager` | `internal/prompt/evolved_atoms.go:17` |
| `AtomLoader` | `internal/prompt/loader.go:30` |
| `PromptManifest` | `internal/prompt/manifest.go:9` |
| `AtomManifestEntry` | `internal/prompt/manifest.go:22` |
| `DroppedAtomEntry` | `internal/prompt/manifest.go:33` |
| `PredicateSelector` | `internal/prompt/predicate_selector.go:23` |

### Functions (sampled)

| Symbol | Location |
|--------|----------|
| `NewFinalAssembler` | `internal/prompt/assembler.go:41` |
| `SetCategoryOrder` | `internal/prompt/assembler.go:91` |
| `SetSectionHeaders` | `internal/prompt/assembler.go:98` |
| `SetSeparators` | `internal/prompt/assembler.go:105` |
| `Assemble` | `internal/prompt/assembler.go:113` |
| `NewTemplateEngine` | `internal/prompt/assembler.go:308` |
| `RegisterFunction` | `internal/prompt/assembler.go:425` |
| `Process` | `internal/prompt/assembler.go:432` |
| `DefaultAssemblyOptions` | `internal/prompt/assembler.go:472` |
| `AssembleWithOptions` | `internal/prompt/assembler.go:482` |
| `AnalyzePrompt` | `internal/prompt/assembler.go:605` |
| `AllCategories` | `internal/prompt/atoms.go:103` |
| `EstimateTokens` | `internal/prompt/atoms.go:242` |
| `HashContent` | `internal/prompt/atoms.go:252` |
| `NewPromptAtom` | `internal/prompt/atoms.go:261` |
| `NormalizeSelectors` | `internal/prompt/atoms.go:277` |
| `MatchesContext` | `internal/prompt/atoms.go:301` |
| `ToFact` | `internal/prompt/atoms.go:433` |
| `ToSelectorFacts` | `internal/prompt/atoms.go:460` |
| `ToDependencyFacts` | `internal/prompt/atoms.go:489` |
| `ToConflictFacts` | `internal/prompt/atoms.go:504` |
| `ToExclusionFact` | `internal/prompt/atoms.go:519` |
| `Validate` | `internal/prompt/atoms.go:531` |
| `Clone` | `internal/prompt/atoms.go:564` |
| `NewEmbeddedCorpus` | `internal/prompt/atoms.go:610` |
| `Get` | `internal/prompt/atoms.go:625` |
| `GetByCategory` | `internal/prompt/atoms.go:632` |
| `AppendAll` | `internal/prompt/atoms.go:642` |
| `All` | `internal/prompt/atoms.go:648` |
| `Count` | `internal/prompt/atoms.go:658` |

## 5. Integration relevance

| Surface | Relevance |
|---------|-----------|
| Kernel | Related |
| VirtualStore | Consumer if effectful |
| Shards | Related |
| Prompt JIT | Owner |
| CLI | Related via `cmd/nerd` |
| Config | Reader |

## 6. Non-goals of this corpus revision

- Full prose rewrite of every function body
- Docs/Spec 18-file product templates (`spec-doc-sprint`)
- Implementing missing features (corpus-build implementation mode)
