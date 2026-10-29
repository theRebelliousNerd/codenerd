# Index only recovery and compiled prompt freshness

## Scope and conclusion

This CodeNERD addendum identifies a residual conformance gap: **index-only recovery can change project semantic retrieval without changing the source token validating a compiled-prompt cache entry**. A retained entry can omit knowledge now available to fresh lookup, provided the actual cache key remains identical. This is a conditional source-level witness, not a measured incident or model-quality regression.

The baseline is main commit `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, unchanged when checked at 15:08 UTC on 29 October 2026. The active Mangle/store commit is `a53cfc308e8836c72370adaa46deba3f7b00180d`. Relevant generation, collection and publication code is unchanged there; its KNN query change does not add index readiness. All CodeNERD source links below use the main pin.

This supplements the JIT generation-aware optimization and embedding-representation-and-migration audits. The former **already proposes `embedding_model_and_index_generation` in the complete key** and documents narrower implemented sources. The new contribution is the concrete producer-to-consumer witness and missing implemented revision/health signal, not a generic cache-key proposal. The embedding audit leaves whole-store activation versus qualified partial serving open; partial serving alone is not established as a bug. [Existing contract][contract]; [as-built qualification][asbuilt]; [activation boundary][embeddingboundary]; [open serving choice][embeddingchoice].

## Retrieval changes before the tracked source changes

`SetEmbeddingEngine` initializes the vector index, snapshots the embedding space, and starts asynchronous backfill without a completion barrier. Initialization drops and recreates `vec_index`; backfill subsequently reads eligible, stamped canonical vectors and inserts only index rows in 100-row transactions. It does not rewrite canonical `vectors`. [Engine attachment][engine]; [index initialization and backfill][backfill].

Once vector lookup is enabled, `VectorRecallSemanticFiltered` returns the index query result without waiting for backfill. At the main pin, this is a scalar cosine-distance ordered scan, **not approximate nearest-neighbor search**, despite internal terminology. An empty result can carry no error. [Recall routing][recall]; [scalar query][query].

The prompt collector requests five semantic project results. Nonempty results return directly; empty results fall back to five lexical matches. Only errors mark this path degraded, so an incomplete index can look healthy to caching. Selected-expert lexical lookup is separate; combined knowledge is bounded retrieved context. [Knowledge collection][collection]. Mangle context/prohibition rules, rendering and token fitting still apply; this is not an authority bypass. [Mangle candidate rule][mangle]; [compile pipeline][pipeline].

## Why ordinary index recovery does not invalidate the entry

The source token combines `schema_version` with trigger-maintained generations. `KnowledgeSourceGeneration` tracks `knowledge_atoms` and `vectors`, not `vec_index`, with a comment expecting the latter to remain synchronized. Initial index DDL can invalidate the token, so the witness begins **after index creation and all tracking DDL**. Thereafter, successful index-row insertion or repair need not change either tracked canonical table. [Source generation][generation].

The pinned sqlite-vec dependency is v0.1.6. Its binding creates shadow tables during `xCreate`; ordinary chunk creation inserts rows/blobs into existing tables. SQLite documents schema changes, including `VACUUM`, as advancing `schema_version`. Ordinary index DML does not supply that invalidation. The witness excludes further DDL and `VACUUM`. [Dependency][dependency]; [shadow-table creation][shadow]; [chunk writes][chunks]; [chunk invocation][chunkcall]; [SQLite schema version][schema].

Unattributed-vector healing also does not rescue this case: with all relevant canonical rows already correctly stamped in the same embedding space, the healer returns without canonical changes. Existing foreign-space and dimensional eligibility checks remain useful and should remain intact. [Healing gate][healing].

The compiler captures the source vector before LRU lookup and publishes only when sources remain fully tracked, unchanged and nondegraded. These guards reject errors, cancellation and untracked sources, but cannot observe an omitted dependency. The 1,000-entry cache has no TTL; its key includes context/kernel, configuration revision and source vector. [Lookup and publication][pipeline]; [Cache implementation][cache].

## A bounded single process witness

1. Create canonical same-space knowledge `K`, initialize the index and tracking machinery, then pause backfill before `K` is inserted, possibly after earlier batches. Use a healthy query engine and valid SQLite query. Choose the corpus/query so the completed semantic lookup selects `K`, while lexical fallback misses it; alternatively, partial nonempty semantic results contain other atoms and bypass fallback.
2. Compile successfully while other optional collectors are healthy. Hold kernel, context, configuration and tracked generations stable, and ensure the eventual `K` is admissible and fits the prompt budget. The initial prompt lacks `K` and is eligible for caching.
3. Complete backfill or a successful index-only repair without canonical writes, DDL or `VACUUM`. Direct semantic lookup and an immediate fresh compilation can now include `K`.
4. Repeat compilation with the **same actual cache key and retained LRU entry**. The cache can return the earlier prompt without rerunning knowledge lookup. Recovery finishing after collection but before publication has the same missing-dependency problem.

No multi-process race is required. A changed current-time/kernel/context value, canonical source write, configuration change, new registration, eviction or compiler restart breaks the equality/retention assumptions. Errors, degradation and cancellation block the healthy-publication witness. Direct queries recover independently. An already-ready index, lexical match for `K`, canonical brute-force path without sqlite-vec, and nil-engine lexical path are controls, not evidence of failure.

## Production reach and limits

The factory attaches the resolved engine to the local store and supplies that handle to JIT. Boot has no explicit project-backfill wait, although sequential startup may finish backfill before compilation; incidence and frequency are unknown. Maintenance enables reconciliation without `VACUUM` or an explicit JIT clear. Reconciliation waits for backfill, then repairs index rows only. This is another route after prior canonical/index drift, not a claim that normal writes always drift. [Factory attachment][factoryengine]; [JIT attachment][factoryjit]; [Boot steps][boot]; [Maintenance][maintenance]; [Reconciliation][reconcile].

Successful compilation feeds `Compile.Prompt` into `systemPrompt`, the tool loop and provider context: a real consumer, not guaranteed model harm. [Executor][executor]; [Provider context][provider].

## Repair direction and tradeoffs

Prefer a store-owned semantic-view revision covering readiness, engine/index incarnation and index mutation/repair generation, integrated with the existing source-vector publication fence. Partial or unready retrieval should be reported accurately so the current request can fall back without being cached as healthy. A completion-only epoch leaves partial-build cache hits possible before completion; combine revision and health.

Alternatives are bounded canonical brute-force retrieval while unready or caller-context-aware readiness waiting. Boot-only blocking misses later drift repair. Avoid unbounded database scans on every cache hit, preserve goroutine supersession/`Close` safety, and retain embedding-space gates. Reuse correct canonical embedding bytes rather than unnecessarily calling an embedding provider. Mangle admission remains unchanged; no extra LLM call is required.

Costs include API/lifecycle synchronization, transactional publication consistency, repair-batch invalidation churn, retained-generation memory and cold-miss work. Brute force adds roughly O(ND) distance work plus decoding/memory; waiting adds latency and cancellation concerns. Better retrieval can increase prompt, prefill and cache-write tokens. No cost savings are established.

## Evaluation required before implementation claims

All proposed evaluations are **unrun**. Compare baseline, revision-plus-health, bounded brute force and readiness waiting using a matched corpus, deterministic scripted query engine and equal prompt budget. Place barriers before first insertion and after partial batches. Record source tokens, actual keys, cache hits, lookup calls and whether `K` reaches the delivered prompt, with an immediate fresh-compilation reference.

Cover repair during compilation, completion before first compilation, genuinely empty knowledge, lexical recovery, nil engine, brute force, foreign-space rows, query errors/timeouts, canonical writes, DDL, `VACUUM`, eviction, same versus changed keys, shutdown and supersession. Count cold requests, hits and coalesced joiners separately; measure SQL rows, CPU, memory, latency, embedding-provider calls, model tokens and full-task verification, including miss/failure denominators.

Inspected tests cover backfill supersession/race/close safety, settled reconciliation and canonical-generation/degraded-recovery behavior, not the composed index-only recovery/cache transition. No tests, runtime experiments or implementation were performed; no exhaustive novelty claim is made. [Backfill tests][backfilltests]; [Reconciliation tests][reconciletests]; [Generation tests][generationtests].

[contract]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md#L83-L97
[asbuilt]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md#L199-L217
[embeddingboundary]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/embedding-representation-and-migration.md#L62-L69
[embeddingchoice]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/embedding-representation-and-migration.md#L364-L372
[engine]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L28-L87
[backfill]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L1005-L1152
[recall]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L583-L645
[query]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L951-L1002
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_db.go#L329-L478
[mangle]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/jit_compiler.mg#L304-L312
[pipeline]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_pipeline.go#L110-L374
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/source_generation.go#L111-L182
[dependency]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/go.mod#L5-L10
[shadow]: https://github.com/asg017/sqlite-vec-go-bindings/blob/529a0138d40e50087db7043e361601f91d711e81/cgo/sqlite-vec.c#L4963-L5172
[chunks]: https://github.com/asg017/sqlite-vec-go-bindings/blob/529a0138d40e50087db7043e361601f91d711e81/cgo/sqlite-vec.c#L4407-L4545
[chunkcall]: https://github.com/asg017/sqlite-vec-go-bindings/blob/529a0138d40e50087db7043e361601f91d711e81/cgo/sqlite-vec.c#L7715-L7718
[schema]: https://www.sqlite.org/pragma.html#pragma_schema_version
[healing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_identity.go#L193-L224
[cache]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_cache.go#L29-L89
[factoryengine]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1737-L1745
[factoryjit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L1856-L1858
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L2847-L2903
[maintenance]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L831-L861
[reconcile]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L695-L777
[executor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1388-L1448
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/working_context.go#L1005-L1025
[backfilltests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_backfill_race_test.go#L18-L128
[reconciletests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vec_reconcile_test.go#L11-L96
[generationtests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_generation_test.go#L70-L133
