# prompt — Observability

> Last verified: **2026-07-13**

## Log categories

| Category | Typical use |
|----------|-------------|
| `logging.CategoryJIT` | Compile start/hit/miss, selection phases, vector timing, config gen, stats summary |
| `logging.CategoryContext` | Selector/budget/resolver/assembler timers and debug detail |
| `logging.CategoryStore` | YAML load, embed load, synchronizer, schema |

Helpers: `logging.StartTimer(...)`, `logging.JITDebug` (evolved atoms).

## CompilationStats (flight metrics)

Produced per MISS compile; fields include:

- **Timing:** Duration, CollectAtomsMs, SelectAtomsMs, ResolveDepsMs, FitBudgetMs, AssembleMs, VectorQueryMs  
- **Counts:** AtomsSelected, SkeletonAtoms, FleshAtoms, AtomsCandidates, AtomsDropped  
- **Tokens:** TokensUsed, TokenBudget, BudgetUtilization, SkeletonTokens, FleshTokens  
- **Sources:** EmbeddedAtoms, ProjectAtoms, ShardAtoms, EvolvedAtoms  
- **Modes:** StandardModeCount, ConciseModeCount, MinModeCount  
- **Cache:** CacheHit, CacheKey, FallbackUsed  
- **Context:** ShardID, OperationalMode, IntentVerb  

APIs: `String()`, `ToLogFields()` for structured logs.

## PromptManifest

| Section | Content |
|---------|---------|
| Metadata | Timestamp, ContextHash, TokenUsage, BudgetLimit |
| Selected | ID, Category, Source, Priority, Logic/Vector scores, RenderMode |
| Dropped | ID, Reason (Mangle prohibited, budget, …) |

DebugMode on `CompilerConfig` triggers richer manifest logging via `logCompilationManifest`.

This is useful live evidence, but it is not yet a durable, queryable decision
receipt. Cache hits also do not reconstruct the full selection explanation.

## Runtime handles

| Handle | Purpose |
|--------|---------|
| `lastResult atomic.Pointer` | Last compilation for UI/introspection |
| Cache hit/miss counters | `cacheHits` / `cacheMiss` atomics |
| singleflight shared log | "joined via singleflight" |

## UI surface

`cmd/nerd/ui/jit_page.go` — operator-facing atom list / content from `CompilationResult`.

## What to log when debugging selection

1. Context string (`cc.String()`) and hash prefix.  
2. Source breakdown (embedded/project/shard/evolved).  
3. Skeleton count vs flesh count; any skeleton category warn.  
4. Vector ms and whether SemanticQuery empty.  
5. Mandatory skips from budget.  
6. Manifest dropped reasons.  
7. Whether `compile_context` assertion warned and whether the active adapter
   implements retraction; production `KernelAdapter` currently does not.

## Metrics not yet first-class

- Prometheus-style exporters (logs are primary).  
- Per-atom latency inside Fit.  
- Embedding engine error rate dashboards.
- Compile-scoped fact counts and ownership, including leaked or cross-request facts.
- Durable decision receipts joining context hash, selected/dropped atoms, tools,
  safety state, cache outcome, and artifact/version provenance.
- Runtime-loader versus strict-validator atom ID/count parity.

## Related glass-box / transparency

CLI transparency features may surface JIT stats; package itself emits structured fields suitable for aggregation.
