# Preserve semantic recall errors at the KNN boundary

## Decision and evidence boundary

Before merging the new sqlite-vec KNN path, define its supported result-limit contract and preserve row-iteration failures. The branch passes any positive requested limit to a backend that refuses `k > 4096`, then returns successful empty results because its reader never checks `rows.Err()`. A supported direct Mangle query or accepted predicate-selector configuration can therefore lose semantic candidates solely by increasing its requested limit.

**Branch only:** inspected `lane/worktree-agent-a70bb621a19639eb0` at `a53cfc308e8836c72370adaa46deba3f7b00180d`, five commits ahead of main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`. Main still uses a cosine full scan with SQL `LIMIT`; the new hard KNN boundary is not on main. The missing iteration-error check already exists on main, so this is a new deterministic trigger for an older error-propagation gap—not a claim that KNN introduced the reader omission. [Exact query change](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store.go#L930-L962), [main predecessor](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/vector_store.go#L930-L964).

This is source-derived behavior, not an observed incident. No tests, code execution, inference, training, implementation, were performed. Default hydration and predicate limits are below the boundary; no stock default workload exceeding it is established.

## Why this is distinct

Current store guidance requires explicit query limits, cancellation and missing-result behavior, as well as embedding-space isolation. The existing representation/migration audit addresses compatible spaces, incomplete migration and index drift; the recent legacy-repair audit addresses an attempt latch and recovery. Neither inspected audit identifies the subsequently added KNN limit interacting with deferred SQL errors. Preserve their identity and recovery safeguards rather than replacing them. [Store guidance](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/agents.md), [representation audit](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/embedding-representation-and-migration.md), [legacy-repair audit](https://github.com/theRebelliousNerd/codenerd/blob/003dc26512e375efef64ddd06e223e1aea5378a7/Docs/audits/legacy-vector-repair-attempt-lifecycle.md).

The branch's KNN parity test uses 240 rows, limit seven, valid vectors and several filters. That is useful normal-path coverage, but it does not exercise the limit boundary or deferred errors. Its existence is counterevidence to a blanket claim that KNN ranking is wrong; the test was read, not run. [Branch test](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_knn_test.go#L45-L140).

## Exact producer and consumer path

### The backend has a pinned execution-time limit

CodeNERD requires `sqlite-vec-go-bindings v0.1.6` and `go-sqlite3 v1.14.44`. The inspected tags resolve respectively to commits `529a0138d40e50087db7043e361601f91d711e81` and `20826e87d8f061d0a7266562f43950ee06e2e9c0`. These are the actual binding and driver sources, not an unqualified upstream mirror. LocalStore opens the `sqlite3` driver, and the tagged extension initializer registers the binding with that driver. [Dependency pins](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/go.mod#L5-L23), [driver selection](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/local_core.go#L99-L104), [extension registration](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/init_vec.go#L1-L13).

The binding's vendored C checks `k` inside `vec0Filter_knn`. It returns `SQLITE_ERROR` above 4096, after validating the query vector and before ranking. This cap is on the requested `k`, not the number of matching rows: one correctly indexed row does not make `k=4097` valid. [Backend check](https://github.com/asg017/sqlite-vec-go-bindings/blob/529a0138d40e50087db7043e361601f91d711e81/cgo/sqlite-vec.c#L6802-L6804), [limit refusal](https://github.com/asg017/sqlite-vec-go-bindings/blob/529a0138d40e50087db7043e361601f91d711e81/cgo/sqlite-vec.c#L6877-L6891).

### The caller accepts a larger request

`vectorRecallVec` defaults nonpositive limits to ten but has no positive upper bound. Its new query uses `embedding MATCH ? AND k = ?`, binding the caller's limit unchanged. [Reader and query](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store.go#L914-L962).

There are concrete consumers:

- **Direct Mangle recall.** `recall_similar/3` declares a numeric TopK. Its VirtualStore handler accepts any positive bound integer, forwards it through `RecallSimilar` to `VectorRecallSemantic`, and uses the requested TopK in returned facts. Direct kernel queries obtain live VirtualStore results, superseding cached external rows. This witness needs no rule-body external-evaluation assumption. [Declaration](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/defaults/schemas_memory.mg#L60-L62), [TopK handling](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/virtual_store_predicates.go#L884-L907), [store call](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/virtual_store_predicates.go#L291-L321), [live direct query](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/kernel_query.go#L306-L334).
- **Configured predicate selection.** `jit.predicate_vec_limit` defaults to 200; its validation rejects values below one but imposes no upper bound. MangleRepairShard applies the configured value and attaches its kernel's LocalStore. PredicateSelector passes this value unchanged to metadata-filtered semantic recall. A configured value of 4097 therefore reaches the new KNN boundary when this semantic path is active. [Configuration](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/config/jit.go#L62-L68), [default and validation](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/config/jit.go#L128-L175), [configuration application](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/shards/system/mangle_repair.go#L146-L172), [store attachment](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/shards/system/mangle_repair.go#L186-L199), [recall consumer](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/prompt/predicate_selector.go#L520-L535).

### The error arrives after Query succeeds

The pinned driver's statement query binds arguments and constructs a Rows object; it does not step the SQLite statement there. SQLite stepping occurs in `SQLiteRows.nextSyncLocked`, called by `Next`, and execution failure is returned through the row iterator. [Driver query](https://github.com/mattn/go-sqlite3/blob/20826e87d8f061d0a7266562f43950ee06e2e9c0/sqlite3.go#L2329-L2350), [driver iteration](https://github.com/mattn/go-sqlite3/blob/20826e87d8f061d0a7266562f43950ee06e2e9c0/sqlite3.go#L2514-L2561).

CodeNERD checks the initial `s.db.Query` error, then loops over `rows.Next()`, skips individual scan failures, and ultimately returns `results, nil`. It never inspects `rows.Err()`. Consequently the KNN refusal at the first step is indistinguishable at this API boundary from a healthy zero-match result. The semantic wrappers return this indexed result directly when `vecEnabled` is true; their brute-force branch is for disabled indexing, not a retry after an indexed failure. [Result loop](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store.go#L959-L1002), [ordinary wrapper](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store.go#L498-L513), [filtered wrapper](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store.go#L630-L645).

## Minimal conditional witness

Use the branch's sqlite-vec-enabled build with an attached, correctly named embedding engine and a valid nonzero query vector matching the index dimensions. Finish index backfill and ensure one compatible canonical row and its index row exist. Exclude legacy repair, foreign spaces, concurrent swaps, missing indexes and malformed metadata from this fixture.

Compare otherwise identical direct queries:

- `recall_similar("query", 4096, R)`
- `recall_similar("query", 4097, R)`

The first can return the stored content. In the second, the bound limit crosses the pinned backend cap; the initial query can prepare normally, the first step fails, and the CodeNERD reader returns an empty result with nil error. The direct VirtualStore path converts that into no external facts. A corresponding direct LocalStore call isolates the same failure without Mangle.

The expected outcome is derived from the sources above, not measured. No corpus larger than 4096 is needed to reach the error. Separately, a corpus larger than 4096 is needed to evaluate whether a proposed cap or fallback preserves the full requested top-k semantics.

For the predicate-selector route, use an accepted `predicate_vec_limit=4097`, a nonempty task description, an attached indexed store and at least one eligible predicate vector. The semantic candidates can disappear while ordinary domain/core/safety candidates remain. Do not describe this as an empty whole prompt or a demonstrated bad generated rule. The merger retains its base selection. [Base selection and merge](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/prompt/predicate_selector.go#L486-L517).

## Limits and counterevidence

- Stock session hydration requests five results; default predicate selection requests 200. Both remain below this specific cap. The demonstrated route is a supported high-limit query or configuration, not an established default incident. [Hydration default](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/virtual_store_predicates.go#L743-L749), [default](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/core/virtual_store_predicates.go#L799-L805).
- The main JIT flesh searcher is **not** this generic LocalStore path. `CompilerVectorSearcher` reads `prompt_atoms` and computes cosine scores in Go. Do not infer that its whole-corpus request size automatically hits the new KNN cap. [Actual flesh backend](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/prompt/vector_searcher.go#L56-L139).
- Prepare/bind errors are already returned; the finding specifically concerns execution/iteration failure. Valid small-k ranking and existing metadata/path eligibility are not disproven.
- Correctly stamped, dimension-matched rows suffice for the witness. It does not depend on index drift, stale foreign vectors, provider refusal or malformed embeddings.
- Without sqlite-vec enabled, the brute-force path does not have this KNN cap and already checks `rows.Err()`. [Brute-force iteration](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/vector_store_bruteforce.go#L16-L65).
- Merely adding `rows.Err()` will expose the error but will not preserve successful recall for previously supported large limits. Some higher consumers deliberately degrade after a reported error; that policy is separate from falsely reporting a healthy empty relation.

## Proposed contract

1. **Preserve errors first.** Check row-iteration completion and propagate failures rather than publishing an empty or partial success. Decide explicitly whether scan failures are individually skippable or make results incomplete. Keep a healthy empty result distinct from a failed search.
2. **Choose a public limit policy.** Either reject unsupported limits before embedding/search with a documented typed/configuration error, or retain the prior API semantics by routing oversized requests to an exact fallback. A silent clamp is not equivalent when more than 4096 eligible rows exist.
3. **Do not duplicate provider work.** If exact fallback is selected, reuse the already computed query vector and captured embedding-space identity. Preserve path and metadata predicates, current-space gating, ranking, cancellation and disclosure of partial results.
4. **Budget the fallback.** A large requested result count should not silently authorize unlimited CPU, memory or time. State whether the operation is bounded, explicitly rejected or returns a disclosed incomplete result. If introducing a separate work cap, make its relationship to the requested result limit clear.
5. **Keep the default fast path.** The normal small-k path can retain KNN's intended speedup. The backend cap belongs in capability validation, not as a Mangle semantic inference that no matching content exists.

Counting eligible rows first could permit `min(requested, eligible)` when that count is below the backend cap, but adds another query and a snapshot-consistency obligation. It still does not solve requests whose eligible top-k exceeds 4096. Do not present counting as a free universal fix.

## Costs and comparative evaluation

Error completion checking is small host work relative to a query. Validation adds a compatibility decision and documentation/migration cost for configurations that were previously accepted. Exact fallback costs a scan and cosine computation proportional to eligible rows times vector dimension, plus ranking and retained results; the existing fallback sorts the full candidate set. A heap or streaming alternative could reduce ranking memory, but needs an independently verified equivalence contract. Query-vector reuse avoids an unnecessary second embedding call.

No new dollar or latency estimate is claimed. The branch reports KNN performance measurements, but they were not reproduced in this review. Measure query embedding, database/filter work, ranking, allocations, fallback/error frequency, lock time and end-to-end consumer work separately. A falsely empty result must not count as a speed improvement.

Proposed evaluation, not run:

- Compare main's full-scan baseline, branch KNN, and the chosen bounded/fallback remedy with the same valid vectors, canonical rows, filters and query.
- Exercise limits 1, default 5/200, 4095, 4096 and 4097, with eligible populations of 0, 1, below the limit and above 4096. Include a legitimate no-match control.
- Pin both binding and driver versions and require the sqlite-vec-enabled route. Tagless brute-force tests alone cannot establish KNN behavior.
- Verify row identity, ordering and score tolerance for distinct distances; define tie behavior rather than accidentally asserting undocumented order.
- Inject a first-step failure and a failure after some rows. Require explicit failure/incompleteness, never a normal complete result. Distinguish initial Query failures from iterator failures.
- Run through direct LocalStore, direct `recall_similar` and configured predicate selection. Capture error status, returned rows, candidate merger output and eventual injected predicate signatures. Confirm base candidates remain when semantic recall fails.
- Preserve foreign-space exclusions, metadata/path filters, backfill controls and shutdown/cancellation behavior. Benchmark small-k unchanged as well as oversized fallback.
- Report retrieval coverage and downstream repair work alongside latency and resources. Reject a treatment that appears faster only because it dropped the semantic channel or silently truncated requested coverage.

The acceptance boundary is straightforward: an admitted query either returns its declared complete bounded result, reports an explicit resource/unsupported-limit condition, or discloses an intentionally partial result. A backend refusal must not masquerade as a healthy empty Mangle relation.

