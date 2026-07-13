# 07 — Dependency Map: embedding

> Last verified against codebase: 2026-07-13  
> Upstream/downstream packages with evidence

## 1. Package position

```
                    external:
                 genai SDK, errgroup,
                 simd/archsimd (opt)
                          │
                          ▼
                 ┌─────────────────┐
                 │ internal/logging│◀── only internal dep
                 └────────▲────────┘
                          │
                 ┌────────┴────────┐
                 │internal/embedding│  ◀── leaf substrate
                 └────────┬────────┘
                          │ imported by
        ┌─────────────────┼──────────────────────────┐
        ▼                 ▼                          ▼
   internal/store   internal/prompt            internal/perception
   internal/mcp     internal/system            internal/campaign
   internal/init    cmd/nerd (+chat)           cmd/tools/*
```

## 2. Upstream (what embedding imports)

| Import | Kind | Usage |
|--------|------|-------|
| `codenerd/internal/logging` | internal | CategoryEmbedding, timers, Embedding/Debug helpers |
| `context` | std | cancellation |
| `fmt`, `time`, `strings`, `sync`, `math` | std | utilities |
| `net/http`, `encoding/json`, `bytes`, `io` | std | Ollama |
| `golang.org/x/sync/errgroup` | module | GenAI parallel batches |
| `google.golang.org/genai` | module | GenAI client/types |
| `simd/archsimd` | Go experiment package | SIMD cosine (`GOEXPERIMENT=simd`, amd64+simd tag only) |

**Does not import:** `core`, `mangle`, `store`, `prompt`, `config`, `perception`.

## 3. Downstream (who imports embedding)

Verified via `rg "codenerd/internal/embedding" -g "*.go"`.

### 3.1 Core product runtime

| Package / file | How used |
|----------------|----------|
| `internal/system/factory.go` | NewEngine, HealthChecker, Cortex.EmbeddingEngine, AtomLoader, MCP, CompilerVectorSearcher, store SetEmbeddingEngine |
| `internal/store/vector_store.go` | Embed + task types + queries |
| `internal/store/vector_store_bruteforce.go` | CosineSimilarity |
| `internal/store/vector_store_reembed.go` | GetOptimalTaskType + task-aware batch |
| `internal/store/reembed_all.go` | ReembedAllDBsForce(engine) |
| `internal/store/reflection_worker.go` | task types for traces/learnings |
| `internal/store/reflection_reembed.go` | expected task types |
| `internal/store/prompt_reembed.go` | prompt atom task type |
| `internal/store/learning.go` / `learned_store.go` | SetEmbeddingEngine, embed knowledge |
| `internal/store/local_core.go` | holds engine field |
| `internal/prompt/loader.go` / `loader_embedding.go` | AtomLoader, SyncEmbeddedToSQLite |
| `internal/prompt/vector_searcher.go` | query embed + cosine |
| `internal/perception/semantic_classifier.go` | NewEngine, SelectTaskType, CosineSimilarity, corpus load |
| `internal/mcp/store.go`, `compiler.go`, `analyzer.go`, `integration.go` | tool embedding + retrieval |
| `internal/campaign/document_ingestor.go` | NewEngine for doc vectors |
| `internal/campaign/decomposer_documents.go` | DefaultConfig → ingestor |
| `internal/init/initializer.go` | workspace init embeds |

### 3.2 CLI / TUI

| File | How used |
|------|----------|
| `cmd/nerd/embedding_cmd.go` | set / stats / reembed |
| `cmd/nerd/chat/session_boot.go` | boot NewEngine + wire |
| `cmd/nerd/chat/reembed.go` | force reembed |
| `cmd/nerd/chat/reflection.go` | task-typed query embeds |
| `cmd/nerd/chat/ingest.go` | NewEngine for ingest |
| `cmd/nerd/chat/model_types.go` | holds EmbeddingEngine fields |

### 3.3 Offline tools

| File | How used |
|------|----------|
| `cmd/tools/corpus_builder/main.go` | GenAI engine + SelectTaskType knowledge_atom |
| `cmd/tools/prompt_builder/main.go` | GenAI engine + prompt_atom task types |

### 3.4 Tests outside package

`internal/store/*_test.go`, `internal/prompt/loader_test.go` define mocks implementing `EmbeddingEngine` / task-aware variants.

## 4. Dependency rules

| Rule | Status |
|------|--------|
| embedding → core | **Forbidden** (currently clean) |
| store → embedding | **Allowed** |
| prompt → embedding | **Allowed** |
| embedding → store | **Forbidden** (would cycle) |
| config → embedding | **Not needed** (config is separate; mapping at boot) |

## 5. External service dependencies

| Service | When |
|---------|------|
| Ollama HTTP at endpoint | provider=ollama |
| Google Gemini Developer API | provider=genai |
| Network + API key | GenAI only |
| Disk space | Ollama model pulls |
| Go experimental SIMD API | optional accelerated test/release builds only |

## 6. Audit command

```powershell
rg "codenerd/internal/embedding" -g "*.go" --glob "!*_test.go"
rg "embedding\.(NewEngine|CosineSimilarity|SelectTaskType|GetOptimalTaskType)" -g "*.go"
```

## 7. Coupling heat map

| Consumer | Coupling strength | Notes |
|----------|-------------------|-------|
| store | **High** | runtime path for every vector write/search |
| prompt | **High** | JIT quality depends on vectors |
| perception | **Medium-High** | semantic classify optional if engine nil |
| system factory | **High** | single attach point for Cortex |
| mcp | **Medium** | tools degrade without embedder |
| campaign | **Medium** | doc ingest optional path |
| CLI embedding cmds | **Low** | maintenance only |

## 8. Safe deletion analysis

Removing any exported symbol requires coordinated multi-package change. Especially:

- `CosineSimilarity` — store brute force + perception + prompt searcher
- `SelectTaskType` / `GetOptimalTaskType` — dozens of call sites
- `NewEngine` — every boot path

**Do not** delete “unused” optional interfaces; they are load-bearing via type assert.
