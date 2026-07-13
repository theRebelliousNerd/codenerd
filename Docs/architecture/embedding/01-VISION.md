# 01 — Vision: embedding

> Last verified against codebase: 2026-07-13  
> Status: Target architecture vision (not a claim that all bullets are already true)

## 1. Purpose

Provide a **single, boring, reliable** interface for text embeddings so every codeNERD subsystem that needs semantic memory speaks the same dialect:

- Construct once per Cortex from `.nerd/config.json`.
- Embed documents and queries with **intent-appropriate** task types when the backend supports them.
- Compare vectors with a well-defined cosine contract.
- Degrade cleanly when the local daemon or cloud key is missing.

## 2. Product outcomes

| Outcome | Why it matters |
|---------|----------------|
| Semantic knowledge search | LocalStore / sqlite-vec retrieval for past work |
| JIT prompt atom selection | Right atoms without hand-curated string matching |
| Semantic intent assist | Perception classifier over embedded patterns |
| MCP tool discovery | Embed tool docs; retrieve by natural language |
| Campaign document grounding | Ingest large docs into vector-backed context |
| Offline corpus build | Tools precompute embeddings for shipping defaults |

## 3. Architectural vision

```
┌──────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ UserConfig   │────▶│ embedding.NewEngine │────▶│ EmbeddingEngine  │
│ .nerd/config │     │  ollama | genai     │     │  + optional ifaces│
└──────────────┘     └─────────────────────┘     └────────┬─────────┘
                                                          │
              ┌───────────────┬───────────────┬───────────┼───────────┐
              ▼               ▼               ▼           ▼           ▼
           store           prompt        perception      mcp      campaign
```

### 3.1 Provider strategy

- **Default local (Ollama)** for privacy, offline, zero API cost.
- **Optional cloud (GenAI)** for higher-quality / task-typed embeddings and parallel batch scale.
- Never hardcode model names outside config + package defaults (chat boot comment enforces this).

### 3.2 Task-type strategy

Treat GenAI task types as part of the **index schema**:

- Document side: `RETRIEVAL_DOCUMENT`, `FACT_VERIFICATION`, etc.
- Query side: `RETRIEVAL_QUERY`, `CODE_RETRIEVAL_QUERY`, `QUESTION_ANSWERING`.
- Persist task type with vectors (store responsibility) so reembed/reflection can detect drift.

### 3.3 Operational vision

| Situation | Desired behavior |
|-----------|------------------|
| First run, no model | Auto-resolve or pull known family; progress logs |
| Ollama down at boot | Clear warning; Cortex up; semantic features off or deferred |
| Provider switch | Operator runs reembed; stats show engine/dims |
| Huge reembed | GenAI parallel batch or async job; Ollama honest sequential cost |

## 4. Non-goals

- Building an ANN library inside `embedding`.
- Replacing sqlite-vec.
- Unifying chat LLM and embed providers into one mega-client.
- Making the kernel call Embed directly.
- Silent model remaps for unknown custom names.

## 5. Success criteria

1. One `EmbeddingEngine` instance per Cortex, shared.
2. All production embed call sites prefer task-aware APIs when available.
3. Switching provider is a documented two-step: config set + reembed.
4. Package tests stay green without network.
5. No imports from core/mangle into embedding (leaf preserved).

## 6. Horizon (dependency-ordered, no time estimates)

1. Dimension discovery or explicit config override aligned with store schema.
2. Unified boot health policy (factory ↔ chat).
3. Optional in-package GenAI batch-job poll helper for tools.
4. Throughput path for Ollama (worker pool) if local reembed remains a bottleneck.
5. Metrics counters (embed latency histogram, pull count) if observability platform expands.
