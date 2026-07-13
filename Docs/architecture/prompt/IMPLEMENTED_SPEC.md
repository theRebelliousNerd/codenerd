# prompt — Implemented Spec

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded corpus**
> Mode: dark-factory autonomous generation via arch-propose/corpus-build port
> **Implementation: present under `internal/prompt/` (25 non-test .go, 32 tests, 0 .mg)**


## 1. Purpose

JIT prompt compiler, atoms, selector, budget

## 2. Source paths

| Path | Role |
|------|------|
| `internal/prompt/` | Primary implementation |
| `Docs/architecture/prompt/` | This corpus |

## 3. Implementation Status

> Status reflects **living code**, not pre-implementation zeroing.

| Component | Status | Completion |
|-----------|--------|------------|
| Source package tree | Implemented | **90%** |
| Exported types (sampled) | Implemented | **80%** |
| Tests | Implemented | **90%** |
| Mangle local sources | N/A or global | **n/a** |
| Docs corpus (this) | Implemented | **100%** |

**Overall (heuristic): 90% complete as living package**

## 4. Public surface (inventory-driven)

### Largest implementation files

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

### Sampled types

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

## 5. Integration points (codeNERD spine)

| Surface | Relevance |
|---------|-----------|
| Kernel / facts | Consumer/producer |
| VirtualStore | User if effectful |
| Shards | Related |
| Prompt JIT | Owner |
| CLI | Invokes via cmd/nerd |
| Config | Reads global config |

## 6. Non-goals for this corpus revision

- Full behavioral prose rewrite of every function
- Filling Docs/Spec 18-file product templates (use spec-doc-sprint)
- Implementing missing runtime features (use corpus-build implementation mode)
