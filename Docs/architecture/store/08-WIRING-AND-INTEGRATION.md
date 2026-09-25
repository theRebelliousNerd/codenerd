# store — Wiring and Integration

> Last verified: **2026-07-13**  
> Evidence primarily from `internal/system/factory.go` and reverse importers.

## Boot sequence (Cortex factory)

Relevant path: `internal/system/factory.go`

1. **Open knowledge.db**  
   `localDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")`  
   `store.NewLocalStore(localDBPath)` → `bctx.localDB`

2. **Wire tracing LLM**  
   On success: `createTraceStoreAdapter(db)` + `perception.NewTracingLLMClient(base, traceStore)`  
   Same pattern for optional worker LLM.

3. **Taxonomy store**  
   If SharedTaxonomy present: `perception.NewTaxonomyStore(localDB)`

4. **LearningStore**  
   `store.NewLearningStore(learningStorePath)` under `.nerd/shards`

5. **Graph adapter**  
   `store.NewLocalStoreGraphAdapter(bctx.localDB)` registered into graph query surfaces

6. **Embedding attach (after engine init)**  
   ```
   localDB.SetEmbeddingEngine(engine)
   localDB.SetReflectionConfig(appCfg.GetReflectionConfig())
   learningStore.SetEmbeddingEngine(engine)
   learningStore.SetReflectionConfig(...)
   ```

## VirtualStore

`internal/core/virtual_store.go` holds:

- `localDB *store.LocalStore`
- `learningStore *store.LearningStore`

Effectful knowledge/learning operations route through VirtualStore methods that call into these fields (and tool-related helpers may use ToolStore paths elsewhere). Store does not register Mangle predicates itself.

## World model

`internal/world/persist.go` (+ deep/incremental scan) uses LocalStore world APIs so fingerprint-stable files skip reparse. Failures at store open cascade to full scan paths (consumer responsibility).

## Prompt JIT

`internal/prompt/compiler.go` / `compiler_db.go` / `predicate_selector.go` read/write prompt atoms and related vectors via store. Disk tables are the persistence side of JIT; compilation policy remains in prompt.

## Perception / taxonomy

- Taxonomy persistence uses LocalStore.
- Learning candidates table stages phrase→verb/target promotions.
- Semantic classifier may use corpus/store surfaces (`storepkg` alias in classifier).

## Campaign / init

- Campaigns pull specialist knowledge and ingest documents into store-backed KBs.
- Init registers shared knowledge, strategic documents, agent knowledge into LocalStore / KnowledgeStore paths.
- Validation may open stores to check health.

## CLI / tools

| Surface | Wiring |
|---------|--------|
| Interactive chat | Via Cortex boot → factory |
| `cmd/query-kb` | Direct store open for inspection |
| Corpus builders | `cmd/tools/*` write embeddings into DBs |
| Embedding re-embed commands | Call `ReembedAllDBsForce` / LocalStore reembed methods |

## Fact-flow placement

```
user_intent (perception/kernel)
    → next_action (Mangle)
    → VirtualStore action
         ├─ may read LocalStore (knowledge, graph, vectors, world, …)
         ├─ may write cold/session/traces/learnings
         └─ tool path may journal ToolStore
    → articulation
```

Store is **orthogonal** to OODA control flow: it is invoked when actions or boot need durable memory, not on every tick by itself.

## Registration hooks summary

| Hook | Location |
|------|----------|
| LocalStore construct | `system/factory.go` |
| LearningStore construct | `system/factory.go` |
| Embedding + reflection | `system/factory.go` after engine |
| Trace adapter | `system/factory_adapters.go` |
| Graph adapter | `system/factory.go` |
| VirtualStore inject | core factory path / VirtualStore setters |
| ToolStore | opened at every boot by `initFactoryToolStore` (`.nerd/tools.db`), auto-cleaned there and by maintenance |

## Maintenance wiring (2026-09-25)

| Hook | Location |
|------|----------|
| tools.db budget (`ToolStore.AutoCleanup`) | `autoCleanupToolStore`, at boot in `initFactoryToolStore` and in `Cortex.runMaintenance` (`internal/system/factory.go`) |
| ANN drift heal (`ReconcileVecIndex`) | `MaintenanceConfig.ReconcileVecIndex`, on in `Cortex.runMaintenance` |
| Store gauges (`vec_index_missing`, `reflection_trace_backlog`) | `LocalStore.GetStats`, printed by `nerd memory` |

## Wiring gaps re-checked (2026-09-25)

- `LearnedCorpusStore`: wired. `perception.NewLearnedCorpusStore` wraps it for
  the semantic classifier (`internal/perception/semantic_classifier.go`).
- Tool smart-cleanup LLM path: deliberately not wired. `CleanupIntelligent`
  would let a model choose what to delete; retention is the budgets'.
  `/cleanup-tools --smart` states the decision.
- Embedded corpus only when `defaults.IntentCorpusAvailable()`: unchanged,
  fails closed with an error in dev builds without the corpus.
- Build tags and `defaultRequireVec`: documented in the root `agents.md`
  (`-tags sqlite_vec`). With cgo, `init_vec.go` registers sqlite-vec either
  way; the tag only makes it required.
