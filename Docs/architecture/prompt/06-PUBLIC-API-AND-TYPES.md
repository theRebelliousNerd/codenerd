# prompt — Public API and Types

> Last verified: **2026-07-13**  
> Package import path: `codenerd/internal/prompt`

## Construction

| Func | Returns | Notes |
|------|---------|-------|
| `NewJITPromptCompiler(opts ...CompilerOption)` | `*JITPromptCompiler, error` | Primary entry |
| `DefaultCompilerConfig()` | `CompilerConfig` | Defaults |
| `NewCompilationContext()` | `*CompilationContext` | Budget 200k |
| `NewCompilationContextWithBudget(int)` | `*CompilationContext` | Preferred when config known |
| `NewAtomSelector()` | `*AtomSelector` | Usually internal |
| `NewDependencyResolver()` | `*DependencyResolver` | |
| `NewTokenBudgetManager()` | `*TokenBudgetManager` | |
| `NewFinalAssembler()` | `*FinalAssembler` | |
| `NewPromptAtom(id, cat, content)` | `*PromptAtom` | Computes hash/tokens |
| `NewAtomLoader(embedding.EmbeddingEngine)` | `*AtomLoader` | engine optional |
| `NewConfigFactory(ConfigAtomProvider)` | `*ConfigFactory` | |
| `NewDefaultConfigFactory()` | `*ConfigFactory` | |
| `NewDefaultConfigAtomProvider()` | `*DefaultConfigAtomProvider` | |
| `NewSimpleRegistry()` | `*SimpleRegistry` | |
| `RegisterDefaultConfigAtoms(*SimpleRegistry)` | — | Side-effect register |
| `LoadEmbeddedCorpus()` | `*EmbeddedCorpus, error` | go:embed walk |
| `NewEmbeddedCorpus([]*PromptAtom)` | `*EmbeddedCorpus` | |
| `NewCompilerVectorSearcher(engine)` | `*CompilerVectorSearcher` | |
| `NewEvolvedAtomManager(nerdDir)` | `*EvolvedAtomManager` | |
| `NewPredicateSelector(*core.PredicateCorpus)` | `*PredicateSelector` | |
| `NewAgentSynchronizer(root, *AtomLoader)` | `*sync.AgentSynchronizer` | subpkg |
| `AssembleEmbeddedBaselinePrompt(*CompilationContext)` | `string, error` | Non-JIT |
| `MaterializeDefaultPromptCorpus(...)` | see `default_corpus.go` | Seed DB |

## Compiler options

| Option | Effect |
|--------|--------|
| `WithEmbeddedCorpus` | Sets baked atoms |
| `WithProjectDB` | Project corpus connection |
| `WithKernel` | Kernel + selector kernel |
| `WithVectorSearcher` | Compiler + selector |
| `WithConfig` | Full `CompilerConfig` |
| `WithDefaultTokenBudget` | Override default budget |
| `WithConfigFactory` | Attach agent config gen |

## Core methods on JITPromptCompiler

| Method | Purpose |
|--------|---------|
| `Compile(ctx, *CompilationContext)` | Full pipeline |
| DB register/unregister helpers | `compiler_db.go` — project/agent DBs |
| Cache clear | On corpus change |
| Snapshot DBs for vector search | Used by `CompilerVectorSearcher` |
| Knowledge/learning collect | Internal to Compile |

(Exact export set for registrars: `CreateJITDBRegistrar` / `CreateJITDBUnregistrar` used by shard manager — see `compiler_db.go` / campaign boot.)

## CompilationContext builder methods

Fluent setters: `WithOperationalMode`, `WithCampaign`, `WithShard`, `WithLanguage`, `WithIntent`, `WithTokenBudget`, `WithSemanticQuery`, plus `Clone`, `Validate`, `Hash`, `String`, `WorldStates`, `AvailableTokens`, `ToContextFacts`, `GenerateFacts`.

## Selection / budget / resolve

| Type.Method | Role |
|-------------|------|
| `AtomSelector.SelectAtoms` | Skeleton+flesh |
| `AtomSelector.SelectAtomsWithTiming` | + vector ms |
| `AtomSelector.SetKernel/Vector*` | Wiring |
| `DependencyResolver.Resolve` | Order |
| `DependencyResolver.ValidateDependencies` | Preflight |
| `DependencyResolver.DetectCycles` | Graph check |
| `TokenBudgetManager.Fit` | Budget |
| `TokenBudgetManager.SetCategoryBudget/Strategy/Headroom` | Config |
| `FinalAssembler.Assemble` | String |
| `FinalAssembler.SetCategoryOrder/Headers/Separators` | Config |

## PromptAtom methods

| Method | Role |
|--------|------|
| `NormalizeSelectors` | Strip leading `/` |
| `MatchesContext` | Dimension AND match |
| `ToFact` / `ToSelectorFacts` / dependency/conflict facts | Mangle projection |
| `EstimateTokens` / `HashContent` | Package helpers |

## ConfigFactory

| Method | Role |
|--------|------|
| `Generate(ctx, *CompilationResult, intents...)` | Merge ConfigAtoms → runtime config |
| `GenerateFallback(ctx, intent, identity)` | Minimal path |

## Loader

| Method | Role |
|--------|------|
| `ParseYAML` / `LoadFromYAML` / `LoadFromDirectory` | Ingest |
| `EnsureSchema` | DDL |
| `StoreAtom` | Upsert |
| Embedding sync helpers | `loader_embedding.go`, `SyncEmbeddedToSQLite` |

## Result / observability types

| Type | Fields of interest |
|------|--------------------|
| `CompilationResult` | Prompt, IncludedAtoms, Manifest, Stats, EffectiveAgentRuntimeConfig |
| `CompilationStats` | Phase ms, atom/token counts, modes, cache |
| `PromptManifest` | Selected / Dropped entries |
| `ScoredAtom` | Logic/Vector/Combined, Source |
| `OrderedAtom` | Order, RenderMode |
| `BudgetReport` / `CategoryUsage` | Budget diagnostics |

## Output mode helpers

| Func | Role |
|------|------|
| `IsStructuredOutputOnly(shardType)` | legislator, mangle_repair |
| (internal filters) | Strip piggyback/reasoning atoms |

## Categories API

`AllCategories()`, category constants `CategoryIdentity`, … `CategorySystem`.

## Types intentionally *not* for casual external use

Internal fact builders, mangle string quoting helpers, max-atom caps — prefer stable constructors above.
