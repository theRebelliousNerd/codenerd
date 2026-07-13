# 08 — Wiring and Integration: embedding

> Last verified against codebase: 2026-07-13  
> How the package is registered/called (boot, CLI, stores, prompt, MCP)

## 1. Config → engine mapping

Authoritative user config lives in `config.UserConfig` / `EmbeddingConfig` (not in this package). Every boot maps:

```go
embCfg := appCfg.GetEmbeddingConfig()
engineCfg := embedding.Config{
    Provider:       embCfg.Provider,
    OllamaEndpoint: embCfg.OllamaEndpoint,
    OllamaModel:    embCfg.OllamaModel,
    GenAIAPIKey:    embCfg.GenAIAPIKey,
    GenAIModel:     embCfg.GenAIModel,
    TaskType:       embCfg.TaskType,
}
engine, err := embedding.NewEngine(engineCfg)
```

**Do not** invent a second config path with hardcoded models.

## 2. Shared Cortex factory (`internal/system/factory.go`)

Function: `initIntelligenceLayer`.

```
GetEmbeddingConfig
  → if Provider empty: DefaultConfig()
  → if genai && no key && cortex apiKey: use apiKey
  → NewEngine
  → if HealthChecker:
        HealthCheck fail → WARN, do NOT attach engine
        HealthCheck ok  → attach
     else:
        attach (GenAI path)
  → AtomLoader(engine)
  → localDB.SetEmbeddingEngine
  → learningStore.SetEmbeddingEngine
  → MCPIntegrationBridge(..., engine, ...)
  → CompilerVectorSearcher(engine) for JIT
  → Cortex.EmbeddingEngine = engine
```

Implications:

- Unhealthy **Ollama at boot** → session without embeddings.
- **GenAI** attaches without preflight health (no HealthChecker).

Failed overall boots are not cached (`GetOrBootCortex` comment mentions transient embedding failure must not poison cache).

## 3. Interactive chat boot (`cmd/nerd/chat/session_boot.go`)

```
log "Initializing embedding engine from config.json..."
if embCfg.Provider != "":
  NewEngine(embConfig)
  localDB.SetEmbeddingEngine
  learningStore.SetEmbeddingEngine
  // NO mandatory HealthCheck gate
JIT CompilerVectorSearcher if engine != nil
SyncEmbeddedToSQLite / ReloadAllPrompts with engine
shardMgr.SetPromptLoader → LoadAgentPrompts(..., engine)
model.EmbeddingEngine field for later reembed/reflection
```

More lenient than factory: engine can be present even if Ollama is flaky; first Embed may still EnsureModel.

## 4. Workspace init (`internal/init/initializer.go`)

Creates engine from init-time embedding config for seeding vectors during `nerd init` style flows.

## 5. Store wiring

| Integration | Mechanism |
|-------------|-----------|
| Attach | `LocalStore.SetEmbeddingEngine`, `LearningStore.SetEmbeddingEngine` |
| Index | Embed / EmbedBatch + task types in `vector_store.go` |
| Search | Embed query task type; sqlite-vec or brute cosine |
| Reembed | `ReembedAllDBsForce(ctx, roots, engine, progress)` |
| Reflection | Background workers re-embed traces/learnings with expected task/model/dim |

Store owns schema, persistence, and when to call the engine. Embedding package never opens sqlite.
Provider implementations now validate the response before this boundary, so
store and prompt consumers receive either a finite, cardinality-correct vector
result or an error. Store still owns cross-run vector-space identity and
migration; package-local validation cannot detect an old index created by a
different provider/model/task tuple.

## 6. Prompt / JIT wiring

| Integration | File |
|-------------|------|
| `NewAtomLoader(engine)` | `prompt/loader.go`, factory |
| `SyncEmbeddedToSQLite(ctx, path, engine)` | boot when corpus missing |
| `LoadAgentPrompts` / `ReloadAllPrompts` | session_boot prompt sync |
| `NewCompilerVectorSearcher(engine)` | JIT atom retrieval |

Prompt atoms use `ContentTypePromptAtom` → `RETRIEVAL_DOCUMENT`; queries use `ContentTypeQuery` → `RETRIEVAL_QUERY`.

## 7. Perception wiring

`perception.InitPerceptionLayer` + semantic classifier:

- May construct or receive engine.
- Embeds query with query task type.
- Cosine against embedded intent patterns / learned corpus.

Historical note: InitPerceptionLayer was once unwired; chat boot now invokes it so embeddings actually affect classification when configured.

## 8. MCP wiring

```
NewMCPIntegrationBridge(workspace, kernel, embedder, llm, serverConfigs)
  → MCPToolStore(db, embedder)
  → JITToolCompiler(store, embedder, kernel)
  → ToolAnalyzer(llm, embedder)
```

Tool docs embedded as documentation; user queries as query task type.

## 9. Campaign wiring

`NewDocumentIngestor(dbPath, embedCfg)` → `NewEngine(embedCfg)` for document chunks.  
`decomposer_documents.go` may use `embedding.DefaultConfig()` when no override supplied.

## 10. CLI wiring (`cmd/nerd/embedding_cmd.go`)

| Subcommand | Wiring |
|------------|--------|
| `embedding set` | Mutates `.nerd/config.json` only; tells user to restart |
| `embedding stats` | NewEngine + LocalStore.GetVectorStats |
| `embedding reembed` | NewEngine + `store.ReembedAllDBsForce` on `.nerd` + `internal` |

Registered on root Cobra tree via `main` package init/AddCommand (see CLI corpus).

## 11. Chat slash / TUI wiring

| Surface | File |
|---------|------|
| Reembed flow | `cmd/nerd/chat/reembed.go` |
| Reflection probes | `cmd/nerd/chat/reflection.go` (`embedReflectionQuery` type-asserts task-aware) |
| Ingest | `cmd/nerd/chat/ingest.go` |
| Model field | `cmd/nerd/chat/model_types.go` |

## 12. Offline tools

| Tool | Provider bias |
|------|----------------|
| `cmd/tools/corpus_builder` | typically GenAI via API key |
| `cmd/tools/prompt_builder` | typically GenAI; uses TaskTypeAwareEngine |

These write precomputed embeddings into DBs shipped or materialised later.

## 13. Fact-flow position (explicit)

```
[Embedding] is NOT on the executive critical path.

Optional:
  perception.semantic_classifier  ──uses──▶ EmbeddingEngine
  prompt.JIT / AtomLoader         ──uses──▶ EmbeddingEngine
  store vector search             ──uses──▶ EmbeddingEngine

Always:
  user_intent → kernel → next_action → VirtualStore → articulation
```

If engine is nil, semantic features degrade; kernel and policy still run.

## 14. Wiring anti-patterns

| Anti-pattern | Prefer |
|--------------|--------|
| NewEngine per Embed call | Share Cortex engine |
| Hardcoded model in session_boot | config.GetEmbeddingConfig |
| Ignoring task-aware assert on GenAI | SelectTaskType + EmbedWithTask |
| Deleting engine field because “optional” | Audit all SetEmbeddingEngine / NewAtomLoader call sites |
| Mixing providers without reembed | config set + reembed + stats verify |
| Persisting an HTTP-success payload without shape checks | Trust the engine's validated result and still enforce store vector-space identity |

## 15. Registration checklist for new consumers

1. Accept `embedding.EmbeddingEngine` (interface), not concrete.
2. Type-assert task-aware when embedding typed content.
3. Pass engine from Cortex / boot, do not re-read API keys ad hoc unless a standalone tool.
4. Handle `engine == nil` (features off).
5. Log under appropriate category; use embedding category only inside this package or for engine lifecycle.
6. Treat `Name()` plus `Dimensions()` as partial identity only; do not infer
   compatibility across provider/model/task changes until the store contract is
   upgraded.
