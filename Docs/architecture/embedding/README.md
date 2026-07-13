# embedding — Architecture Corpus (`internal/embedding`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/embedding/`  
> Scale: **6** non-test Go files ≈ **1,498** lines; **7** test files ≈ **1,747** lines; **0** `.mg`

## Scope

This corpus documents the **vector embedding substrate**: provider-abstracted text→vector engines (Ollama local, Google GenAI cloud), GenAI task-type selection, cosine similarity (generic + optional SIMD), and the factory/config surface used across store, prompt JIT, perception, MCP, campaign, init, and CLI maintenance commands.

It is **not**:

- The sqlite-vec / local vector tables (`Docs/architecture/store/`)
- The JIT prompt compiler that *consumes* embeddings (`Docs/architecture/prompt/`)
- The CLI `nerd embedding` verb surface (covered here for wiring only; primary CLI corpus is `Docs/architecture/cli/`)

## Role in fact-flow

Embeddings sit **beside** the executive path, not on it:

```
user_intent → kernel next_action → VirtualStore → articulation
                      ▲
                      │ (semantic retrieval feeds perception/prompt/store)
         EmbeddingEngine.Embed / CosineSimilarity
                      ▲
         store | prompt | perception | mcp | campaign
```

The Mangle kernel does **not** call this package. Logic remains executive; embeddings power fuzzy retrieval that later becomes structured facts.

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and funcs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, store, prompt wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, dimensions |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories and timers |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

## Verify

```powershell
# Package tests (no live Ollama/GenAI required for unit coverage)
go test ./internal/embedding/...

# Optional race
go test -race ./internal/embedding/...

# Optional live GenAI batch bench (needs API key + network)
# go test -bench=BenchmarkEmbedBatchParallel -benchtime=1x ./internal/embedding/

# Reverse-dep inventory
rg "codenerd/internal/embedding" -g "*.go" --glob "!*_test.go"
```

## Related corpora

- `Docs/architecture/store/` — vector tables, reembed, brute-force search
- `Docs/architecture/prompt/` — AtomLoader, CompilerVectorSearcher, corpus sync
- `Docs/architecture/perception/` — semantic classifier
- `Docs/architecture/system/` — `initIntelligenceLayer` boot
- `Docs/architecture/cli/` — `embedding_cmd.go`, chat reembed/reflection
- `Docs/architecture/mcp/` — tool store / JIT tool compiler
- `Docs/architecture/campaign/` — document ingestor

## Quality bar

Modeled on `Docs/architecture/cli/`: real inventories, control-flow diagrams, wiring evidence, honest gaps — **not** auto-generated file tables.
