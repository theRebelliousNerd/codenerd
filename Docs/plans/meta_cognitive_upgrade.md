# Meta-Cognitive Architecture Upgrade: Vectorizing Traces and Learnings (System 2 Memory)

## 1. Executive Summary and Philosophy
The codeNERD architecture currently exhibits a "meta-cognitive gap". While it separates Logic (Mangle/Skeleton) from Intuition (Vectors/Flesh), the self-improvement systems (TraceStore and LearningStore) live entirely in the logic domain.

- Problem: The agent suffers from semantic amnesia. If it fails a task labeled "Fix auth", it will not recall that experience when facing a task labeled "Repair login", because SQL LIKE strings do not match.
- Goal: Enable System 2 reflection by allowing the agent to ask: "Have I faced a problem like this before?" or "What did I learn last time I touched a similar pattern?" regardless of phrasing.
- Solution: Implement semantic recall by (1) synthesizing descriptors, (2) asynchronously embedding them, (3) injecting recall hits into the kernel during Observe, and (4) applying policy logic to guide behavior.

### Scope and Non-Goals
- Scope: Vectorized TraceStore and LearningStore recall, OODA integration, policy guidance, JIT prompt injection, and observability.
- Non-goals: Replacing prompt-atom embeddings, full RAG over raw traces, or changing the existing intent classifier.

---

## 2. Objectives and Success Criteria
- Behavioral: Recall similar prior tasks and surfaced learnings in >70% of relevant turns.
- Latency: Add <150ms to Observe on cache hits; <400ms on cold embeds.
- Precision: >80% of surfaced recalls rated relevant (manual spot checks).
- Stability: Reflection must degrade gracefully if embeddings or sqlite-vec are unavailable.
- Operations: Embedder backlog remains under a configured watermark for 95% of sessions.

---

## 3. Data Model and Vector Indexes

### A. TraceStore Evolution (reasoning_traces)
Traces are currently high-volume logs. We will transform them into lessons.

**Schema Upgrade (v5):**
```sql
ALTER TABLE reasoning_traces ADD COLUMN summary_descriptor TEXT;
ALTER TABLE reasoning_traces ADD COLUMN descriptor_version INTEGER;
ALTER TABLE reasoning_traces ADD COLUMN descriptor_hash TEXT;
ALTER TABLE reasoning_traces ADD COLUMN embedding BLOB;
ALTER TABLE reasoning_traces ADD COLUMN embedding_model_id TEXT;
ALTER TABLE reasoning_traces ADD COLUMN embedding_dim INTEGER;
ALTER TABLE reasoning_traces ADD COLUMN embedding_task TEXT;
```

**Descriptor Template (deterministic):**
`Intent: [verb|target] | Shard: [type] | Files: [key files] | Outcome: [success|failure] | Key Issue: [distilled note] | Tags: [error tags]`

### B. LearningStore Evolution (learnings)
Learnings are currently rigid Mangle facts. We will give them semantic handles.

**Schema Upgrade:**
```sql
ALTER TABLE learnings ADD COLUMN semantic_handle TEXT;
ALTER TABLE learnings ADD COLUMN handle_version INTEGER;
ALTER TABLE learnings ADD COLUMN handle_hash TEXT;
ALTER TABLE learnings ADD COLUMN embedding BLOB;
ALTER TABLE learnings ADD COLUMN embedding_model_id TEXT;
ALTER TABLE learnings ADD COLUMN embedding_dim INTEGER;
ALTER TABLE learnings ADD COLUMN embedding_task TEXT;
```

**Semantic Bridge Example:**
When Autopoiesis creates a new preference/3 fact, it also generates a semantic_handle:
"When working with Cobra commands, use RunE instead of Run to ensure errors propagate."

### C. Vector Index and Metadata
- sqlite-vec virtual tables:
  - `reasoning_traces_vec(rowid, embedding)`
  - `learnings_vec(rowid, embedding)`
- rowid maps to the base table primary key for join and retrieval.
- Persist model metadata (embedding_model_id, embedding_dim, distance_metric) in a DB metadata table or a local store header. On mismatch, disable semantic recall and emit a warning.
- Record embedding_task per row (RETRIEVAL_DOCUMENT vs RETRIEVAL_QUERY) to detect task mismatch.

### D. Task-Type Contract (Query vs Document)
- Stored descriptors/handles are **documents** and MUST use `RETRIEVAL_DOCUMENT`.
- Observe-time intent descriptors are **queries** and MUST use `RETRIEVAL_QUERY`.
- Use `embedding.SelectTaskType(...)` or `embedding.GetOptimalTaskType(...)` consistently and persist `embedding_task` per row.
- Any vector write path must set `content_type` metadata to drive task selection (docs, knowledge atoms, prompt atoms, predicates).

---

## 4. Descriptor Synthesis and Dedupe
- Deterministic synthesis from structured fields (intent, shard, file targets, outcome, error tags).
- SanitizeDescriptor() removes secrets and PII before persistence and embedding.
- Dedupe by descriptor_hash to avoid re-embedding identical descriptors.
- Re-summarize only on descriptor_version changes or when missing/empty.

---

## 5. Operational Infrastructure

### A. Write-Behind Embedder
Embedding is slow (~300 to 500ms). Use an asynchronous background worker to prevent latency in the TUI/CLI loop.

1. Capture: StoreReasoningTrace saves to SQLite with embedding = NULL.
2. Describe: Worker fills missing summary_descriptor and semantic_handle if needed.
3. Embed: Worker batches up to 32 items with EmbeddingEngine.EmbedBatch().
   - Use task-aware embedding when available:
     - Documents: RETRIEVAL_DOCUMENT
     - Queries: RETRIEVAL_QUERY
   - Prefer EmbedBatchWithTask/EmbedWithTask when the engine supports it.
   - Task types selected via embedding/task_selector.go and `content_type` metadata.
   - If task-aware APIs are unavailable, embed anyway and mark embedding_task empty for re-embed later.
4. Index: Sync embeddings into sqlite-vec tables.

### B. Retention and Backpressure
- Retention options: TTL (days), size cap (rows), or stratified sampling by shard/outcome.
- Backpressure: If the embedder backlog exceeds the watermark, temporarily reduce batch size or skip success traces.
- Degrade mode: If backlog is sustained, fall back to lexical recall only (LIKE search) for the session.

### C. Integration with Existing Tooling
- `nerd embedding reembed`: update ReembedAllDBsForce to include reasoning_traces and learnings.
- `/reflection`: new TUI command to inspect the reflection buffer and current recall hits.
- `nerd stats`: add "semantic memory health" metrics (backlog, hit rate, drift).

### D. Feature Flags and Config
Add config toggles under `.nerd/config.json`:
- `reflection.enabled`
- `reflection.top_k`
- `reflection.min_score`
- `reflection.recency_half_life_days`
- `reflection.backlog_watermark`

---

## 6. Reflection Loop (OODA Integration)
Reflection must be synchronous in Observe so results are available before Decide.

1. Build intent descriptor (cached for the turn).
2. Embed or reuse cached embedding (RETRIEVAL_QUERY).
3. Vector search TraceStore and LearningStore (top_k).
4. Apply gating: score >= min_score, recency weighting, and model_id match.
5. Assert ephemeral facts:
   - `trace_recall_result(ID, Score, Outcome, Summary).`
   - `learning_recall_result(ID, Score, Predicate, Description).`
6. Retract ephemeral facts after the turn.

Fallback: If embeddings or sqlite-vec are disabled, skip vector search and optionally use lexical fallback against descriptors.

---

## 7. JIT Prompt Integration (No Wiring Gaps)
- Add a reflection prompt atom under `internal/prompt/atoms/` (e.g., `internal/prompt/atoms/system/reflection.yaml`).
- Add selector logic so reflection atoms are included when recall results exist.
- Include reflection hits in Piggyback control packets for transparency and debugging.

---

## 8. Policy Logic (Executive Guidance)
```mangle
# Past failure guard
past_failure_warning(TraceID, Summary) :-
    user_intent(_, _, Task, _),
    trace_recall_result(TraceID, Score, /failure, Summary),
    Score > 85.

# Strategy alignment
aligned_preference(Pred, Args) :-
    user_intent(_, _, Task, _),
    learning_recall_result(ID, Score, Pred, _),
    Score > 80,
    learning_data(ID, Args).
```

---

## 9. Risk Analysis and Mitigation
- Model drift: On embedding_model_id mismatch, disable semantic search and prompt reembed.
- Task mismatch: If embedding_task is not RETRIEVAL_DOCUMENT for stored rows, mark as stale and reembed.
- Privacy leaks: SanitizeDescriptor() before persistence and embedding.
- Hallucinated reflection: Hard gating on score and recency; cap to top_k.
- Performance: Cache embeddings per turn, speculative embedding, and fail open with lexical fallback.
- Missing sqlite-vec: Feature flag auto-disables reflection; log a warning.

---

## 10. Observability and Evaluation
- Metrics: reflection_latency_ms, recall_hit_rate, recall_precision_samples, embedder_backlog, drift_events, avg_hit_age_days.
- Logging: Use embedding and system_shards categories for recall events and gating decisions.
- Tests:
  - Unit: SanitizeDescriptor(), descriptor_hash stability, drift detection.
  - Integration: vector search returns joinable rowids.
  - E2E: reflection hits appear in prompt atoms and policy warnings.

---

## 11. Implementation Phases

### Phase 1: Foundations (v5 Migration)
- [ ] Update ReasoningTrace and Learning structs.
- [ ] Implement migrations v5.
- [ ] Add vector table definitions and metadata storage.
- [ ] Update `nerd embedding reembed` to include new tables.

### Phase 2: Descriptor Pipeline
- [ ] Implement deterministic descriptor synthesis and hashing.
- [ ] Add SanitizeDescriptor() and versioning.
- [ ] Backfill missing descriptors in reembed flow.

### Phase 3: Async Embedder
- [ ] Create `internal/store/background_worker.go`.
- [ ] Implement EmbedBatch orchestration with retry and backoff.
- [ ] Add backlog watermark handling and degrade mode.

### Phase 4: Reflection Hook
- [ ] Implement `Kernel.PerformReflection(intent)` with caching and gating.
- [ ] Update `internal/core/defaults/schemas_memory.mg` for new predicates.
- [ ] Add policy rules to `internal/core/defaults/policy/trace_logic.mg`.

### Phase 5: JIT + UI
- [ ] Add reflection prompt atom under `internal/prompt/atoms/`.
- [ ] Update prompt selector/compiler to inject reflection context.
- [ ] Implement `/reflection` command and `nerd stats` metrics.

### Phase 6: Validation and Tuning
- [ ] Evaluate recall precision/latency, tune thresholds.
- [ ] Add automated checks for model drift and stale embeddings.

---

## 12. Wiring Map and Checklist

**Store and Embedding**
- [ ] `internal/store/trace_store.go`: descriptor + embedding fields, save flow.
- [ ] `internal/store/learning.go`: semantic_handle + embedding fields.
- [ ] `internal/store/migrations.go`: v5 migration and vector table creation.
- [ ] `internal/store/vector_store.go` and `internal/store/local_vector.go`: sqlite-vec search + joins.
- [ ] `internal/store/init_vec.go`: vector schema bootstrapping.
- [ ] `internal/store/reembed_all.go`: include reasoning_traces and learnings.
- [ ] `internal/store/background_worker.go`: write-behind embedder.
- [ ] `internal/store/reflection_search.go`: trace + learning recall (vec + brute-force fallback).

**Kernel and Policy**
- [ ] `internal/core/kernel.go` or `internal/core/kernel_facts.go`: PerformReflection hook and ephemeral fact lifecycle.
- [ ] `internal/core/defaults/schemas_memory.mg`: declare recall predicates.
- [ ] `internal/core/defaults/policy/trace_logic.mg`: add guidance rules.

**Prompt and JIT**
- [ ] `internal/prompt/atoms/`: reflection atom content (system/reflection).
- [ ] `internal/prompt/compiler.go`: reflection atoms gated by world_state=reflection_hits.
- [ ] `internal/articulation/emitter.go`: piggyback reflection diagnostics.

**CLI/TUI and Observability**
- [ ] `cmd/nerd/`: `/reflection` command and `nerd stats` additions.
- [ ] `internal/logging/logger.go`: log events in embedding and system_shards.
- [ ] `.nerd/config.json`: reflection + embedding feature flags.


> *[Archived & Reviewed by The Librarian on 2026-01-26]*
