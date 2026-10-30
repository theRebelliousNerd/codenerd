---
doc-class: deep-dive
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Coalesce equal semantic heads before embedding, not only after storage

## Decision and evidence boundary

**The Tier 4 file-vector store shares equal content, but a cold corpus pass still submits that equal content once per file path.** Acquisition and in-flight ownership use `(path, digest, space)`; persistence uses `(digest, space)`. A single successful warm-up can therefore embed the same head repeatedly, then overwrite one durable row with each equivalent result. No concurrency, cache invalidation failure or model error is required.

A related accounting mismatch hides the multiplicity: `WarmReport.Embedded` is documented as files reaching the engine, but increments by the number of distinct digest keys in each successful chunk. In the bounded example below, 64 file heads reach the engine while the warm report says 2 embedded.

This is a **specific cold-miss work-coalescing and accounting gap**, not a claim that semantic caching is absent, every turn re-embeds the repository, or retrieval chooses the wrong answer. The proposed change preserves every file candidate and the existing represented head; it does not reduce corpus coverage.

Public main was rechecked at [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391). This review used static source, architecture and test-definition inspection only. No application, tests, model inference, benchmarks, provider requests or repository modifications were performed. Counts below are source-derived conditional predictions, not measured incidents, bills or speedups.

## What is already working

The current architecture specifies a whole eligible corpus, one shared searcher, persisted vectors, 32-item acquisition chunks and background warm-up. The earlier sampled corpus and per-seed ownership problems are not being reopened. Its store explicitly treats equal heads in the same embedding space as one reusable representation. [Current implemented specification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/retrieval/IMPLEMENTED_SPEC.md#L388-L398).

Important existing controls:

- **Reuse across passes/processes:** a matching path/digest/space returns from memory, then persisted content is consulted before requesting embeddings. A later healthy process can load one digest row for many files.
- **Same-path concurrent-miss control:** a caller claims each path/head/space; warm-up waits for an existing claim, while a turn counts it as pending rather than waiting.
- **Source and space identity:** head changes invalidate that path's memory hit; the store is partitioned by engine name. This report assumes a stable, nonempty engine identity and does not solve mutable model aliases or the broader transformation-identity issue.
- **Coverage and fallback:** source membership, source extensions, Go-test exclusion, nonblank/nonbinary heads, dimension checks, partial outcomes and definition fallback remain relevant. A working corpus acquisition is not equivalent to every file entering the final prompt.
- **Progress and lifecycle:** chunks publish as they succeed; shutdown cancels and joins the warm-up before closing its store. A later process reuses persisted progress.

These controls are implemented in [head/cache/in-flight acquisition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L338-L426), [partial ranking](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L220-L266), [corpus membership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L285-L319), [read-head/source filters](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L486-L520), and [warm-up lifecycle](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L24-L83).

## Exact production chain

### 1. A normal boot can acquire this corpus

The factory constructs a configured GenAI or Ollama engine and retains it when initialization/health checks permit. `initSemanticSearcher` passes that engine directly to a workspace-bound searcher, opens `.nerd/file_embeddings.db` when possible, and starts `Warm` in the background. Warm-up has shutdown cancellation but no fixed wall-clock budget; it is not gated on a model choosing a retrieval tool. Task retrieval subsequently shares the searcher. [Engine attachment](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L1728-L1761), [engine construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/embedding/engine.go#L124-L168), [searcher and warm-up](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L12-L60), [task wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2697-L2710).

No available engine means no semantic warm-up and therefore no instance of this cost. Store-open failure leaves a memory-only searcher; it does not remove the cold-pass multiplicity.

### 2. Head equality is known before the provider call

The searcher represents each file by at most its first 4 KiB. `vectorsFor` reads the heads and computes SHA-256 digests before taking the mutex. However, the memory map is keyed by path and `vectorKey` includes path. On an empty cache, two files with the same digest create two distinct claims and both enter `owned`. [Head size and keys](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L74-L113), [digest collection and ownership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L344-L382).

The persistent lookup is performed once for the entire `owned` slice, before embedding any chunk. A cold digest remains a miss for each path. The pass does not reconsult the store after an earlier chunk publishes the same digest. [Store lookup and misses](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L402-L426).

### 3. Coalescing occurs only after work has been submitted

The embedding loop appends every owned head's text to a chunk and calls `EmbedBatch`. It assigns returned vectors to their file paths, then constructs `fresh` keyed only by digest. That map collapses equal heads for persistence, **after** all their inputs have reached the engine. `cv.embedded += len(fresh)` also counts this collapsed map. [Chunk submission, fan-out and accounting](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L428-L462).

The store's primary key is `(digest, space)`, and Save uses replacement by that key. Equal heads therefore share one final durable row, although the cold pass may have submitted many identical texts. [Store identity](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store.go#L19-L27), [schema](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store.go#L46-L53), [save](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store.go#L115-L146).

### 4. The configured engine does not locally remove those duplicates

For GenAI, a retrieval chunk is below the adapter's 100-item threshold. `embedBatchChunk` constructs one Content per text and sends the resulting list to the SDK-facing request. For Ollama, the native request serializes `Input: texts`; fallback invokes `Embed` for each item. The inspected constructors return these concrete engines, not a content-cache wrapper. [GenAI batch dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/embedding/genai.go#L237-L268), [GenAI request construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/embedding/genai.go#L331-L369), [Ollama request](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/embedding/ollama.go#L278-L326), [Ollama native/fallback loop](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/embedding/ollama.go#L335-L398).

This establishes duplicate submitted inputs, not the provider's internal compute or billing. A provider may cache internally. Retries, readiness checks, fallback and input splitting can alter transport-request counts; they must be accounted for separately.

## Bounded, unexecuted witness

Consider one normal warm-up with:

- 64 distinct eligible, stable, ordinary source files; none is excluded, blank, binary or a Go test;
- identical first 4,096 bytes, with each head read returning that full prefix and any differing declarations after the boundary;
- an empty in-memory vector cache and no stored vector for that digest in the current space;
- no competing acquisition, corpus mutation, cancellation or shutdown;
- successful one-vector-per-input responses containing valid nonempty vectors, a stable engine identity and successful store operations.

For example, ordinary `.go` files can begin with the same sufficiently long comment prefix and then have valid package/declaration text. This is a representation-level fixture: it does not require the whole files to be duplicates, generated-file admission, or a hash collision. The equal represented bytes already imply the same digest.

| Quantity | Current source prediction |
|---|---:|
| Eligible path heads | 64 |
| Distinct represented heads in this space | 1 |
| Retrieval `EmbedBatch` calls | 2 |
| Corpus input items across those calls | 64 |
| Corpus head bytes supplied across those calls | 262,144 |
| Final persisted rows for this head/space | 1 |
| `WarmReport.Covered` | 64 |
| `WarmReport.Stored` | 0 |
| `WarmReport.Embedded` | 2 |

Both 32-item chunks contain the same digest, so each `fresh` map has size one. Warm copies `cv.embedded` into its report; the production log prints that as embedded files. The count is neither submitted file inputs nor global distinct representations: with the same digest spread over two chunks it counts twice. [Warm report contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic.go#L143-L185), [production reporting](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L49-L60).

The smallest witness needs only two equal heads within one chunk. The 64-file version additionally shows that saving the first chunk does not prevent redundant later-chunk acquisition. Query embeddings, boot health checks and other subsystem embeddings are deliberately excluded from these corpus-only counts.

**Warm controls:** a new searcher with the healthy persisted row can load it for all 64 paths, report 64 stored files and zero embedded, and submit no corpus heads. Repeating Warm on the same unchanged searcher uses memory, yielding zero Embedded and zero Stored while still covering all 64 files. This is not recurring cold cost on every task.

## Why existing tests and audits do not already settle it

The persistence test verifies that a fresh searcher reuses four unchanged heads and embeds one edited head, but its five file prefixes are distinct. The concurrent-miss test uses 40 files with distinct head text and verifies that four callers together submit each path once. Both controls should remain; neither of these two fixtures exercises cross-path equal-content acquisition. [Persistence fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store_test.go#L22-L67), [same-path concurrency fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/outcome_test.go#L195-L215).

Other existing tests **do contain repeated heads**. The partial-progress fixture uses repeated filler files and injects a failure into its second logical batch; the nonwaiting warm-up fixture creates four equal new heads. They exercise progress preservation and pending coverage, not minimum unique-input work. Coalescing changes the first fixture's batch layout: it must use enough distinct representations to reach its intended failure boundary, rather than deleting its partial-progress obligation or blindly retaining the old batch count. [Repeated-filler progress test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/wiring_test.go#L241-L280), [nonwaiting equal-head fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/semantic_store_test.go#L141-L194).

The [retrieval freshness audit's R2 and as-built status](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L87-L99) already call for reuse and concurrent-miss coordination, and [credit the path/content implementation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L189-L199). This is a concrete residual at **cross-path equal-content acquisition**, not a new discovery of caching or a request to restore missing wiring.

The [embedding representation audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/embedding-representation-and-migration.md#L17-L68) owns space compatibility, truncation, migration and unresolved transformation descriptors. The [stats audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/embedding-stats-observation-boundary.md) concerns an observational command activating index maintenance. Neither is this cold Tier 4 work-item multiplicity or its per-chunk report count.

Review included root guidance, main audit inventory, relevant current/as-built documents, recent research inventories through [1147](https://github.com/theRebelliousNerd/codenerd/tree/921628ea7b1af635cb9c99d4055f6feed2a5efeb/Docs/audits), and active lane comparisons. None of the three reviewed lanes changes `semantic.go`, `semantic_store.go` or the two embedding adapters. The detached legacy Jules branch could not be compared because GitHub reported no common ancestor; no exhaustive historical novelty claim is made.

## Narrow repair, risks and alternatives

**Prefer coalescing representation work before submission while retaining path fan-out.** Group cold owned heads by (head digest, captured embedding space), under the existing compatible representation contract; load each unique store key once, request one vector for each missing representation, then assign that vector to every associated path. Preserve corpus membership and file identity: do not return only one file per digest or change ranking to a digest census.

A first change can coalesce one pass's owned work and fix the demonstrated witness. Sharing in-flight work across *different paths in different callers* is a related extension, not something accomplished merely by deleting `path` from `vectorKey`. Waiters need a representation result and correct fan-out; cancellation/error must release every affected claim without waiting on a claim the same caller owns. Preserve nonblocking turn behavior, no lock across provider IO, and exact head/space checks when binding results.

Separate accounting units: paths covered, submitted input items/bytes, successful unique representations, store-loaded paths and provider attempts/retries. Do not relabel today's `Embedded` value as a global unique count: the same key can occur in several chunks.

**Representation safety is a prerequisite.** Current persistence already assumes that identical head bytes in a stable space may share a vector. Nevertheless, provider output can depend on batching, task role, model revision or other context; coalescing changes batch composition and, for a single remaining Ollama item, can change the adapter route to single-input embedding. Do not promise bit-identical cold outputs or infer global provider equivalence from a fake. Qualify the current representation contract with paired provider/ranking checks. Broader descriptor work remains the existing audit's domain.

Alternatives include leaving acquisition per path intentionally but fixing counters, or coalescing in a general embedding adapter. The latter affects every caller and may erase caller-specific task/representation distinctions, so the already-content-addressed retrieval boundary is the narrower starting point.

## Complete cost and fair evaluation

Let N be cold path heads and U their distinct compatible represented contents. Current corpus submission scales with N; proposed local grouping submits U. This is an input-work count, **not an N/U latency or monetary speedup**.

Reads, head hashing, membership and path-level ranking still cover N files. Grouping adds maps and fan-out bookkeeping; vector sharing can change memory ownership and requires immutable shared slices or defensive copies. Provider payload/response items, repeated serialization, repeated same-key saves and some chunk calls can fall, but provider caching, batch efficiency, request minimums, retry behavior and warm-cache prevalence determine real savings. Restored head reuse must not increase stale evidence or lose paths. Final brief/model tokens may remain exactly unchanged.

Proposed, unrun acceptance:

1. Add deterministic 2-, 32-, 33- and 64-path equal-head cases, including equal heads with distinct tails. Count actual requested items, bytes and calls separately from report fields. Require every eligible path to remain covered.
2. Mix repeated and unique heads across chunk boundaries. Check fresh store, warm store, memory-only, damaged-row miss, changed head, new path sharing an existing digest, and different-space controls.
3. Preserve current same-path concurrency and turn-not-waiting tests. For any cross-path shared-claim extension, test owner failure, waiter cancellation, shutdown, all claims released and no self-wait.
4. Exercise partial provider failures, wrong cardinality, empty vectors, persistence failure and cancellation. Rebuild chunk-boundary failure fixtures around more than 32 distinct representations when necessary; do not lose the existing progress-retention assertion as a side effect of coalescing. Distinguish attempted items, accepted vectors and durable rows; successful progress must remain reusable.
5. With deterministic fake embeddings, compare all path scores, tie ordering, top-k results and tier/fact/brief output before and after. With actual configured providers, use an authorized paired study for single/batch representation and retrieval quality; permit declared float tolerance rather than inventing bit equality.
6. Measure cold/warm distributions, N/U, IO/hash time, grouping CPU, allocations/peak memory, store operations, actual provider attempts/input usage, latency and downstream task cost. Include U=N to expose overhead and warm-heavy workloads to prevent overstating cold-only benefit. Keep world/CodeDOM, tools, model, corpus and task budget fixed.

**Acceptance is less redundant representation acquisition with truthful accounting and unchanged eligible path evidence—not fewer corpus files or a lower token count at any quality cost.**

