# store — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Package: `internal/store/`  
> Method: score each north-star dimension against **code evidence**, not aspirational prose.

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully realized in package behavior |
| 4 | Strong; minor gaps or external dependency |
| 3 | Partial / uneven |
| 2 | Weak / mostly aspirational |
| 1 | Contradicted or absent |

---

## Dimensions

### 1. LLM = creative center; logic = executive

**Score: 5**

Store never “decides” next_action. It persists and retrieves structured data for kernel, perception, and VirtualStore. No LLM calls inside store except **optional** embedding engines (vector math, not executive control) and optional LLM-smart tool cleanup recommendations that still leave policy outside.

Evidence: `LocalStore` APIs are CRUD/recall; `HydrateKnowledgeGraph` takes an inject `assertFunc` so assertion policy stays with the caller.

### 2. Constitutional safety / default deny

**Score: 2** (correctly out of package)

Store will write any fact/args callers pass. There is no `permitted(...)` check here. Safety is enforced upstream in Mangle policy / VirtualStore. This is an **alignment of placement**, not a store bug — but store is not a safety gate.

Evidence: `StoreFact`, `StoreLink`, `StoreReasoningTrace` have no permission predicates.

### 3. JIT prompt atoms over ad-hoc shard prose

**Score: 4**

Disk tier for prompt atoms (`prompt_atoms` table, polymorphism columns, selector JSON, embeddings) is production-ready. JIT selection/compilation lives in `internal/prompt`; store is the durable substrate. Gap: corpus docs historically under-described this dual ownership — fixed in this rebuild.

Evidence: `local_prompt.go`, migrations for `content_concise` / `content_min` / `source_file`.

### 4. Durable long-horizon memory without prompt drift

**Score: 5**

Cold facts with access tracking + archival, session turns, compressed states, world fingerprint cache, learnings with confidence decay, and reasoning traces all exist to reduce reliance on stuffing everything into the live prompt.

Evidence: `local_cold.go`, `local_session.go`, `local_world.go`, `learning.go`, `trace_store.go`.

### 5. Wiring before deletion / partial integrations

**Score: 4**

Package is heavily wired (`system/factory.go`, `core` VirtualStore, world, prompt, campaign, init, perception). Some satellite types (`LearnedCorpusStore`, tool smart-cleanup) need consumer greps before labeling unused. Build-tag vec requirement is a real integration footgun.

### 6. Deterministic executive substrate

**Score: 4**

SQLite + explicit codecs (`fact_codec`) give deterministic round-trips for structured args. Semantic search is inherently approximate (embedding + ANN) — correct for associative tier, not for constitutional truth. Cold facts remain the deterministic logic tier.

### 7. Observability / glass-box readiness

**Score: 4**

`CategoryStore` timers and debug logs are pervasive. Stats APIs exist but incomplete vs full table set. No dedicated metrics export beyond logging.

### 8. Testability

**Score: 5**

Large suite: unit, integration, benchmarks, e2e vector paths, cold lifecycle, migrations.

---

## Aggregate

| Area | Score |
|------|------:|
| Role separation | 5 |
| Safety placement honesty | 2–5* |
| JIT substrate | 4 |
| Long-horizon memory | 5 |
| Wiring health | 4 |
| Determinism of structured tiers | 4 |
| Observability | 4 |
| Tests | 5 |

\*Safety: store is correctly **not** the policy engine; low local score for “enforces constitution” is expected.

**Verdict:** Store is **strongly aligned** with the north star as the durable memory half of Cortex. Primary product risk is operational (ANN drift, multi-DB ops), not conceptual misplacement.
