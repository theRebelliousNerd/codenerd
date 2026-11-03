# Context-feedback caches must not republish a pre-write score

## Finding and decision

**A successfully stored feedback event can invalidate the score cache, then an older reader can put a pre-event score back into that new cache.** Subsequent calls made after the write has completed return the older cached score without consulting the now-current database.

The store deliberately has no TTL: its source comment says every write invalidates the entries. The missing condition is publication ownership. Both score getters compute outside the cache lock and publish unconditionally; replacing the map does not revoke an already-running computation. This is a logical freshness race, not an unsynchronized Go map access or lost SQLite feedback row. [Cache contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L25-L37), [write invalidation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L196-L208), [getters](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L211-L250).

**Decision:** bind a cached score to the feedback generation from which it was computed. A computation overlapping a write may return its older snapshot to its own caller if that is the chosen read contract, but must not become the cache answer for later post-write readers. Preserve asynchronous feedback ingestion, the sample floor, bounded SQL scoring, normal Mangle selection and the existing fallback limits.

Source pin: main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), inspected October 30, 2026; main revalidated unchanged at approximately 15:14 UTC. Review included guidance, active branches and research additions through [`research-main-20261030-1450`](https://github.com/theRebelliousNerd/codenerd/commit/ac3013965dd2ce27d66f04ab833638ac8f4db788). An independent source reviewer corroborated the publication race and shared-store asynchronous producer. This is a conditional static-source finding; no tests, runtime reproduction, model inference, benchmarking or repository modification was performed.

## Existing controls and prior ownership

The [independent-observation audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/context-feedback-independent-observations.md) already owns repeated rating strings being mistaken for independent samples. This witness uses **ten distinct admitted events and one later distinct event**, with one rating for the predicate in each. It does not depend on duplicate array entries, contradictory ratings, malformed identifiers or changing the default sample floor.

The [JIT generation-aware work](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/jit-generation-aware-optimization.md) already establishes that computed results need a source generation at publication. That general principle is not new. Here the affected object is the separate SQLite-backed `ContextFeedbackStore` score cache, not the JIT compiled-prompt cache, source vector, consultation memo or semantic-file corpus.

Several defenses are already correct:

- Feedback insertion is transactional and writer-serialized. The cache is cleared only after a successful commit.
- Cache accesses use their own read/write mutex; ordinary map access is safe.
- Scores use at most the latest 100 rating rows, a ten-sample default floor, and time-decay weights. Query/scan failures do not become a partially computed score.
- Successful kernel-derived selection takes precedence for rendered active facts. Budget recalculation and hot-state snapshots separately invoke Go activation; they are not fallback-only. Learned feedback is not a permission decision.

These controls limit consequence but do not order an old cache computation against a committed invalidation. [Writer and transaction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L146-L208), [defaults](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L61-L82), [bounded scorer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L253-L340), [selection precedence](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L716-L749).

## The ordinary runtime supplies both sides

### One shared store, not a hypothetical second owner

When learning initialization succeeds, factory initialization opens `.nerd/context_feedback.db`, retains the store in the Cortex and gives the session executor a recorder pointing to it. The TUI obtains `cortex.ContextFeedback` and installs that exact store in its compressor; the source explicitly avoids opening a second handle there. This schedule needs neither cross-process invalidation nor two independently constructed stores. [Factory ownership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L432-L457), [Cortex publication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L3008-L3020), [TUI borrowing](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L151-L168).

This route requires a JIT compiler, usable judge/client, successful evolver setup and successful feedback-store initialization; it is not guaranteed on every degraded boot. `initContextFeedback` is called only after the earlier learning setup has passed. [Initialization preconditions](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L92-L120), [feedback setup call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L166-L175).

The executor forwards an admitted turn's optional context-feedback packet through its installed recorder. The production adapter launches the SQL write in a goroutine instead of waiting for it on the turn's goroutine. A later context build can therefore overlap that pending write, including after the delegated turn has returned; a simultaneous second foreground execution is not required. The witness conditions on a valid feedback packet and successful storage, not on every model supplying feedback. [Turn recording](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L2461-L2470), [packet forwarding](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_learning.go#L394-L424), [asynchronous recorder](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_learning.go#L460-L487).

### A real context consumer reads the cache

In compression-active ordinary articulation, chat calls `GetContextString` and carries the result into its conversation context. `BuildContext` first attempts `should_include_context`; only an error, empty selection, or unresolved derived selection invokes Go activation. [Articulation construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L975-L999), [fallback admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor.go#L718-L749).

Go activation reads intent-specific feedback first, falls back to the general score only when that result is zero, and scales usefulness by 20. Other relevance/recency/dependency components, threshold filtering and token-budget selection still apply. For rendered active-fact selection, this is a potentially stale **Go-fallback score component**, not proven omission of a useful fact or an observed bad answer. [Feedback component](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/activation_scoring.go#L469-L492), [threshold and budget path](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/activation.go#L601-L627).

Two other callers prevent a blanket “fallback only” conclusion: `recalcBudget` independently uses Go-selected facts to compute token usage, and `buildStateLocked` independently computes `HotFacts` for a state snapshot. Their scores can also read this cache, whether or not the rendered context used the Mangle selection. This review does not establish a downstream compression decision or persisted-state consequence from those additional readers. [Budget calculation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_turns.go#L279-L324), [hot-state snapshot](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_metrics.go#L168-L195).

## The exact publication schedule

Both getters have the same shape:

1. Read the map under `cacheMu.RLock`.
2. On a miss, release that lock and call `computePredicateScore`.
3. Acquire `cacheMu.Lock` and assign the computed value to the current map.

They take no feedback generation into step 2 and check none at step 3. The writer's separate `mu` does not enclose these readers. [Global and intent getters](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store.go#L213-L248).

Use predicate P and intent `/fix`, with recent, finite timestamps and the normal default floor:

| Step | Reader R | Writer W | Store/cache state |
| --- | --- | --- | --- |
| 1 | — | Ten distinct successful feedback writes have each rated P helpful for `/fix` | Database has ten positive observations; the last write left the cache empty |
| 2 | Misses `P:/fix`, reads those ten rows, computes +1, releases SQL rows; pauses before cache publication | The eleventh asynchronous write is still pending | R holds an old computed value, not a map lock |
| 3 | Paused | Commits one noise rating for P under `/fix`, replaces the cache with an empty map, returns successfully | Database now has eleven observations; new map contains no score |
| 4 | Acquires cache lock and inserts its old +1 into that new map | Complete | Cache is now older than the committed database |
| 5 | A new lookup started after W completed hits +1 | No further write | No SQL recomputation occurs |

The scorer uses each predicate's helpful/noise rating and event timestamp, not `overall_usefulness` or `task_succeeded`; recording an outcome does not independently reweight this score. The ten positive weights make the old score exactly +1. With a recent negative observation of positive weight included, the corresponding uncached eleven-event score is strictly less than +1. No particular equal-timestamp numerical approximation is necessary.

Choose the **nonzero intent-specific score** deliberately: activation will use it directly, so its general-score fallback does not rescue this witness. The same publication defect exists in the general getter, but the primary schedule does not depend on that secondary path.

This schedule remains legal even if SQLite serialized database operations: pause R after its score query has returned and closed its rows but before its map assignment. All operations can respect their locks. A race detector alone need not identify this semantic ordering defect.

The defect is not that an overlapping read returned old data once. The supported failure is that later, non-overlapping reads continue receiving the resurrected value. Any subsequent successful `StoreFeedback` clears the whole map again, and reopening builds a fresh cache. Thus persistence lasts **until another successful feedback write or store recreation**, not permanently regardless of later activity. The raw eleven-event evidence remains intact throughout.

## Consequence and alternatives

A stale positive or negative score can change fallback ranking or eligibility only where its difference matters relative to other score components, the threshold and the token reserve. Demonstrating a changed numeric component does not establish a changed selected set, and a changed selected set does not by itself establish worse task quality. Successful kernel selection bypasses this score for the rendered active-fact choice, but does not establish that other compressor accounting or snapshot readers bypass it. An inactive compressor defeats the specified ordinary rendered-context route; an absent feedback store, missing packet or failed write defeats the particular production witness.

The asynchronous producer is intentional: the turn should not wait for feedback persistence merely to avoid a local cache race. The fix should not make learning mandatory for execution, remove fallback disclosure, or allow model ratings to override executive policy.

Existing score tests exercise SQLite-backed helpful/noise ranking, separate general/intent keys and aggregate counts. Helper tests protect against recursive locking. Those are valuable controls, but they do not force an old computation to publish after a successful invalidation. They were inspected, not run. [Scoring fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store_scoring_test.go#L8-L56), [query-helper locking fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/feedback_store_test.go#L31-L72).

Generic concurrent compressor QA also covers budget refresh and state reload against live turns. Credit that synchronization coverage without treating it as a forced feedback-cache publication schedule. [Concurrent budget and state QA](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/compressor_race_test.go#L9-L103).

All five inspected active code/QA heads have `internal/context/feedback_store.go` byte-identical to main: `51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and `767a3291d6287b238417f2466592a9002810d05a`. This is a file-level comparison, not a claim of whole-branch equivalence. The recent 1450 audits address campaign pause ownership, zero-backlog learning-index upgrades, Northstar projection recovery and review-fix citations, not this score-cache publication boundary.

## Narrow repair and full costs

One compatible design is a store-local feedback generation guarded with cache state:

- Capture the generation with a cache miss before starting its SQL work.
- After a successful write commits, advance the generation and clear affected cache state atomically under the cache lock, before reporting write success.
- Publish a computed score only if its captured generation still matches.
- On mismatch, either return the already-read snapshot only to its overlapping caller without caching it, or perform a deliberately bounded fresh retry if stronger read semantics are required. Do not create an unbounded retry loop under continuous feedback.

The design names proposed state, not an existing field. Preserve lock ordering and avoid holding the cache mutex across a database query or provider work. A single coarse generation is sufficient for the current whole-map invalidation policy; per-key generations are an optional optimization with more bookkeeping. Singleflight can reduce duplicate misses, but it is not by itself a publication fence. A TTL merely limits the stale window and does not fulfill the post-write cache contract.

This repair addresses writes through the shared store instance. It does not magically detect independent-process database writes or configuration changes; a broader freshness contract would require separate ownership/version evidence. No new model or embedding call is necessary for local fencing.

Costs include an epoch check and small metadata, discarded overlapping computations, and possibly an extra bounded SQL read. The SQL scan remains capped at 100 rows per score, while database query planning and lock contention still need measurement. Coarse invalidation already causes cache misses across predicates; do not promise net latency savings or silently broaden this into a cache redesign. Correctly updated scores may select more, fewer or different context atoms and affect later prompt tokens, tool calls and repair turns. End-to-end benefit is an evaluation question, not a source-derived number.

## Unrun evaluation

1. With the default ten-sample floor, use ten distinct positive events and a barrier between read completion and cache publication. Commit the eleventh negative event, complete invalidation, then release the old reader. A subsequent post-write lookup must match an uncached read of the same eleven-event snapshot, not the old cached +1.
2. Cover both general and intent-specific getters; invalidation before query, during query and after publication; a newer reader publishing before the older reader; unrelated-predicate writes; failed writes; no-write cache hits; and repeated invalidation. Use deterministic barriers rather than timing sleeps. Verify durable event counts and no duplicated ingestion.
3. Preserve the overlapping-reader contract explicitly. Test for deadlock and data races separately from semantic freshness; neither absence of races nor eventual recovery after another write proves the required ordering.
4. Exercise factory recorder → shared TUI store → ordinary compressor with fixed feedback packets and controlled fact sets. Test successful Mangle selection as a rendered-active-fact no-impact control, and fallback with/without a ranking-cutoff change; separately observe budget and hot-state readers rather than assuming that control bypasses all Go scoring. Keep ordinary policy, corpus, CodeDOM/world context and request limits enabled.
5. Measure SQL calls/rows, cache hits and discarded fills, write acknowledgement latency, allocation and contention, selected fact identities, prompt tokens and verified outcomes. Compare paired normal-entry tasks at equal model, authority and budget, including no-overlap and no-feedback controls. Report downstream repairs and total cost, not merely fewer cache misses.

**Acceptance:** after a feedback write has completed its invalidation, an older in-flight computation cannot become the reusable score for later readers, while the existing bounded and policy-subordinate feedback mechanism remains intact.

