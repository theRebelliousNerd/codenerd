# 00 — Alignment & Vision Review: embedding (`internal/embedding`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/embedding/` (6 non-test Go ≈ 1.5k lines; 7 tests ≈ 1.7k lines)

## 1. North-star statement

codeNERD’s north star: **LLM = creative center; Mangle kernel = executive**.  
The embedding package is neither creative nor executive. It is **semantic substrate**: a deterministic-enough transduction of text into vectors so that perception, prompt JIT, stores, and tools can retrieve structure that the kernel can later treat as facts.

Alignment success for this package means:

1. Stay a **leaf** (no kernel imports, no policy ownership).
2. Prefer **local-first** defaults (Ollama) without blocking cloud GenAI.
3. Expose **task-type awareness** so retrieval quality is intentional, not accidental.
4. Fail in ways that **degrade semantic features** without killing Cortex boot or bypassing `permitted(...)`.
5. Keep auto-ops (model pull, retries) bounded and observable.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | No Mangle, no action dispatch; pure Embed/Cosine APIs (`engine.go`) |
| Fact-flow fidelity | **4** | Feeds perception/prompt/store; does not invent `user_intent` or `next_action` |
| Provider portability | **4** | Ollama + GenAI factory; dim/task asymmetry is the residual risk |
| Local-first DX | **5** | DefaultConfig ollama; EnsureModel auto-resolve/pull; HealthCheck |
| Task-type discipline | **4** | Rich Select/Detect; GenAI implements WithTask; Ollama ignores task (API limitation) |
| Observability | **4** | CategoryEmbedding timers + debug; no metrics |
| Safety / fail-soft | **4** | Boot warns and continues; factory may drop unhealthy Ollama; auto-pull disk cost |
| Test grounding | **4** | Large mock coverage; live GenAI/SIMD optional |
| Wiring honesty | **5** | Heavy reverse deps in store/prompt/perception/mcp/system/cli — not dormant |
| JIT atom synergy | **4** | AtomLoader + CompilerVectorSearcher depend on engine; package itself not atom-aware |

**Overall alignment: 4.3 / 5** — mature substrate; residual risk is dimension/provider consistency and boot-path asymmetry.

## 3. What “good” looks like (embedding-specific)

| Good | Bad |
|------|-----|
| Same engine instance shared across store + prompt + MCP after boot | Multiple NewEngine per keystroke with different models |
| Query task `RETRIEVAL_QUERY` vs document `RETRIEVAL_DOCUMENT` on GenAI | Always default SEMANTIC_SIMILARITY for both directions |
| Reembed after provider switch | Mixed 768/3072 vectors in one table without migration |
| HealthCheck before multi-minute batch | Blind sequential Ollama batch while daemon is down |
| Known-family-only model remaps | Silently swapping arbitrary user model names |

## 4. Score rationale notes

### 4.1 Executive split (5)

Deleting `internal/embedding` would break semantic search and JIT retrieval quality, but **would not** remove constitutional enforcement. That is correct layering.

### 4.2 Boot fail-soft (4, not 5)

`system.factory` HealthCheck failure **discards** the engine (semantic off for session). Chat boot is more permissive. Operators can see “embedding unavailable” warnings without understanding feature loss. Gap is operational clarity, not policy violation.

### 4.3 Task types (4)

Design is strong; effectiveness depends on GenAI provider. Ollama path cannot honor task types — documented asymmetry, not a bug, but retrieval quality differs by provider choice.

## 5. Related corpora

- `Docs/architecture/store/` — vector persistence & reembed  
- `Docs/architecture/prompt/` — JIT atom embedding  
- `Docs/architecture/perception/` — semantic classification  
- `Docs/architecture/system/` — Cortex intelligence layer  
- `Docs/architecture/cli/` — operator maintenance surface  
