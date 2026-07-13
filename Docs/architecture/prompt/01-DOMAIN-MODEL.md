# prompt — Domain Model

> Last verified against codebase: 2026-07-13
> Status: Living Reference Document — **code-grounded full corpus**
> Mode: 1:1 with `internal/prompt/` (complete internal coverage)
> **Implementation: `internal/prompt/` — 25 non-test .go, 32 tests, 0 .mg**


## Package

`internal/prompt/`

## Exported types (sampled, up to 40)

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

## Exported functions/methods (sampled, up to 30)

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

## Mangle surface

| Artifact | Count |
|----------|------:|
| Package-local `.mg` | 0 |

| Path | Lines |
|------|------:|
| — | 0 |

Global kernel schemas/policy (when this package participates):

- `internal/core/defaults/schemas.mg`
- `internal/core/defaults/policy/`

## Fact-flow placement

```
user input → perception → user_intent → kernel(core/mangle) → next_action
  → VirtualStore / shards / tools → articulation
```

This package: **JIT prompt compiler, atoms, selector, budget, resolver**
