---
doc-class: deep-dive
subsystem: embedding-batch-admission
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 24cca7bd6de49e79872861f4631c2dc9bd6d3b7f
verification-method: static-source-inspection
main-rechecked: 2026-11-04T17:48:00Z
supersedes: []
---

# GenAI chunk validation can admit a mixed dimension corpus as ready

## Finding and decision

**GenAI rejects mixed vector dimensions inside one provider chunk but can return them successfully across chunk boundaries.** `embedBatchWithTask` divides more than 100 inputs into separate requests. Each response is validated against its own first vector. The results are then concatenated without a whole-batch shape check. A response containing 100 finite 3072-dimensional vectors followed by a separate response containing one finite 768-dimensional vector passes both local validators and returns 101 vectors with no error. [Chunk orchestration][chunking] [Chunk validation][chunk-validation] [Shared validator][validator]

The missing check has a concrete consumer consequence. `StoreVectorBatchWithEmbedding` can persist all 101 rows with the same model stamp and return a successful count. Predicate indexing treats the subsequent metadata count as a completed index. A later 3072-dimensional query on the brute-force path rejects the short row during cosine comparison, yet the row continues to count toward readiness. The result is a stored, same-model representation that is unavailable to this semantic query while readiness suppresses another indexing attempt. [Publication][store-publication] [Count contract][count] [Readiness][ready] [Recall rejection][recall-skip]

The narrow repair is to enforce the existing uniform-dimension batch contract **after composition and before successful return or publication**. Keep per-chunk validation for early rejection and retain the current concurrency and ordering guarantees. Separately make readiness distinguish stored attribution from usable representation, so already-persisted malformed rows cannot certify a complete index.

**Evidence boundary:** this is a conditional source-derived finding at [main `24cca7bd6de49e79872861f4631c2dc9bd6d3b7f`][pin]. The witness deliberately supplies a malformed, provider-inconsistent response even though CodeNERD requests 3072 dimensions. No evidence establishes that Google's live service has returned it. No repository program, test, build, gate, model call, inference, training, benchmark or fault-injection fixture was executed. No implementation, credential or setting was changed. The proposed evaluations are unrun.

## Architectural contract and current status

Embedding is a semantic leaf: it supplies representations to retrieval and prompt selection without owning Mangle permission or dispatch decisions. Its scoped guidance specifically requires rejecting mixed batch dimensions before vectors reach stores, while warning against tightening every response to `Dimensions()` without a coordinated alternate-model and migration contract. The proposed global consistency check follows the first requirement without silently changing the second. [Embedding guidance][embedding-guidance]

Store guidance says row counts alone do not establish runtime recall. Root guidance assigns executive control to the deterministic system and requires evidence-backed context. A completed provider request, a stored row and a usable predicate representation are distinct observations that should remain distinct in the implementation. [Store guidance][store-guidance] [Root guidance][root]

The embedding architecture documents describe the engine abstraction, parallel chunking and shared validators. Some surrounding constructor, default-model and retry prose is historical. Current behavior here is established by pinned source, not by those stale passages. The architecture's claim that the chunk helper validates a batch is accurate for its argument but insufficient for the larger caller-visible result. [Embedding internals][embedding-internals] [Architecture guidance][architecture-guidance]

| Capability | Status at the pin | Evidence boundary |
|---|---|---|
| Reject blank input items, bad task types, wrong response count, empty/non-finite vectors and within-chunk dimensional disagreement | Implemented | Existing request and response checks. [Admission][batch-admission] [Validator][validator] |
| Process large GenAI batches in ordered chunks with bounded active concurrency | Implemented | Chunks of at most 100; concurrency limit six; fixed result slots and join. [Chunking][chunking] |
| Enforce uniform dimensions across the complete returned batch | Missing on the multi-chunk branch | Concatenation proceeds directly to logging and success. [Composition][composition] |
| Preserve model attribution on generic vector storage | Implemented | A model stamp is stored; known foreign/unstamped spaces are filtered. This is useful but does not inspect shape. [Identity][identity] |
| Treat malformed same-model vectors as missing from corpus readiness | Missing for the demonstrated count path | Readiness counts non-null, same-stamp rows. [Count][count] [Readiness][ready] |
| Correct whole-batch admission and representation-aware readiness | Proposed | No implementation or passing regression is claimed by this report. |
| Live-provider incidence and end-to-end quality or cost improvement | Unverified | No runtime or model measurements were made. |

## The producer to consumer chain

### Local shape checks reset at every chunk

The GenAI adapter validates task type and all input texts before dispatch. At 100 inputs or fewer it returns `embedBatchChunk` directly. Above 100 it creates an ordered set of chunks, executes each under the shared errgroup context, waits for all results, and flattens their vectors in input-chunk order. Worker completion order does not change output order. [Admission][batch-admission] [Chunking][chunking]

Every chunk uses the same engine model and task type and requests `OutputDimensionality: 3072`. `embedBatchChunk` rejects nil embedding entries and calls `validateEmbeddingBatchResponse(len(texts), embeddings)`. That validator checks response cardinality, nonempty/finite components, and equality to the length of the **first vector passed to that invocation**. It does not receive the surrounding batch's established dimension. [Request and response][chunk-validation] [Validator][validator]

Consequently each chunk can be internally uniform while two chunks disagree. After `eg.Wait`, the caller does not invoke the validator on the concatenated slice or compare chunk dimensions. It logs the first returned vector's length as `dimensions` and returns success. This is an omitted aggregate invariant, not a data race, ordering bug or failure to join the workers. [Composition][composition]

The source defines an injected provider-call field for tests. A future deterministic test can exercise the real orchestration and chunk validators while replacing only the external response. It need not invoke a live model or change production credentials. [Provider seam][provider-seam]

### Real callers can exceed the chunk size

Predicate indexing loads the missing corpus representations and appends them all into one `contents` slice. It imposes no 100-item cap before the batch store call. Each row has `content_type=knowledge_atom`, which the task selector recognizes and maps to `RETRIEVAL_DOCUMENT`. The store's uniform-task branch therefore calls `EmbedBatchWithTask` when the attached engine is GenAI. This path does not silently become 101 single-item calls. [Index construction][index-build] [Task selection][task-selection] [Store dispatch][store-dispatch]

The selector is connected to the real legislator through `SetParentKernel`, which supplies the kernel corpus and VirtualStore's LocalStore. Its feedback loop uses the selector for context when available. The claim is bounded to that configured path; a nil store, an absent corpus, a different provider or an already-complete compatible index changes what runs. [Legislator wiring][legislator] [Feedback consumer][feedback]

There is also a normal chat ingestion route. It chunks each admitted document into 2000-rune pieces and batches all of that file's pieces for vector storage. A sufficiently large supported file can therefore cross the provider's 100-item boundary. Its separately stored prompt atoms are a different representation surface and are not proved lost by this vector-table finding. [Ingest chunking][ingest-chunking] [Ingest storage][ingest-storage]

These source paths establish reachability for a qualifying input. They do not establish a measured installed corpus size, a particular user's provider configuration or a live inconsistent response.

### Storage success does not add the missing invariant

After embedding, `StoreVectorBatchWithEmbedding` checks for an error and verifies that output count equals input count. It obtains the engine's space identifier, enters a transaction, and checks each vector for nonempty content before JSON serialization and insertion. Finite vectors of different lengths can all satisfy those steps. [Store dispatch][store-dispatch] [Publication][store-publication]

The decisive witness uses the store's supported brute-force recall branch, so it does not depend on sqlite-vec accepting a wrong-length insert or on the transaction semantics of an index error. With the vector extension inactive in that fixture, the canonical rows are inserted and counted. Assuming ordinary SQL/metadata success, the store returns the full stored count and nil error. [Publication][store-publication] [Recall branch][recall-branch]

The sqlite-vec branch is a separate qualification case. Source shows canonical insertion preceding optional index insertion and logging of index failures, but this report does not need to infer a particular extension version's failure or commit behavior to establish the primary finding.

### Count based readiness retains the inconsistency

`vectorEligibleClause` tests only that an embedding is non-null and its model stamp matches. `CountVectorsByMetadata` applies that clause to predicate rows when an engine is attached. Neither condition checks whether a vector's length matches its peers or a query. [Identity][identity] [Count][count]

After the store returns success, predicate indexing checks whether this count reaches the corpus total. If so, it returns true; `ensurePredicateVectorIndex` sets `vectorIndexReady`, and later calls return before attempting indexing. A new selector over the same table can take the count-based early-success branch as well. [Readiness][ready] [Index completion][index-completion]

This is persistent only under the stated unchanged-row/model conditions. Explicit reembedding, replacing or removing the malformed row, changing the applicable corpus inventory, or other repair can alter the result. The finding is not that recovery is impossible.

### Retrieval protects the distance calculation but loses the row

Filtered semantic recall embeds the query and enters the brute-force path when the vector extension is inactive. The row is selected by its matching model stamp. `CosineSimilarity` then detects a dimension mismatch and returns an error; the row loop skips it and continues. The remaining compatible rows may still produce a successful nonempty result. [Recall branch][recall-branch] [Cosine guard][cosine] [Row loop][recall-skip]

The cosine guard is correct and should remain. Comparing unequal-length vectors by padding, truncating, or ignoring the error would replace a detectable representation failure with an unjustified similarity score. The defect is accepting and counting the representation upstream, not refusing the invalid comparison downstream.

Predicate selection also retains its direct corpus/domain selection and merges semantic additions into that baseline. A short vector does not prove that its predicate can never be presented through another route, that every prompt becomes incomplete, or that an invalid generated rule is accepted. The supported consequence is narrower: that row cannot contribute to the demonstrated semantic query, despite a readiness state that claims the corpus is indexed. [Selector merge][selector-merge]

## Conditional witness and partition control

Use a disposable fixture with a fresh LocalStore, an attached fixed-name GenAI engine, and 101 distinct valid predicate representations. The corpus inventory is unchanged, metadata encodes the same supported content type on every row, and there are no stale or duplicate historical representations. Arrange the store fixture to exercise the existing brute-force branch; this audit did not change any runtime setting.

All inputs are nonblank. The provider test seam returns ordinary response objects with correct positional correspondence and cardinality:

| Provider chunk | Input positions | Returned shape | Local validation |
|---|---|---|---|
| A | 0 through 99 | 100 finite nonzero vectors of length 3072 | Passes |
| B | 100 | One finite nonzero vector of length 768 | Passes |
| Subsequent query | One query | One finite nonzero vector of length 3072 | Passes single-vector admission |

Both chunk requests still specify 3072. Chunk B is intentionally inconsistent with that request. The fixture models an invalid external result; it does not assert that 768 is an allowed response to this request or that Google intermittently changes dimensions. Google's documentation describes controlling the requested output size, which is precisely why merely observing a finite vector does not establish request conformance. [Google embedding dimensions][google-size]

Source-derived outcome under the preconditions:

1. The two internally uniform chunks return no validation error.
2. Flattening returns 101 vectors with mixed lengths and logs 3072 from the first item.
3. The canonical store serializes and stamps all rows, returning `stored=101, err=nil`.
4. Metadata count is 101, so the selector declares the 101-item corpus ready.
5. A 3072-dimensional semantic query skips the 768-dimensional row on the cosine mismatch.
6. Another ensure call sees the ready latch. A fresh selector over unchanged storage instead sees the stamp-only count reach 101 and also skips reindexing.

The first four steps already establish contradictory admission/readiness. The fifth proves a usable-representation consequence without requiring a stochastic model, a generated bad rule, a top-k quality claim or a live provider run.

**Partition-sensitive control:** hold each input's assigned test response constant, but permute the short-vector item from position 100 into the first 100 items. The first chunk now contains both lengths and the existing validator rejects it. Restoring item identities after the permutation leaves the logical input/output population unchanged, yet whether the invalid population is accepted depends on a batching boundary. That is the distinguishing observation for this defect.

A two-item mixed-length input is likewise rejected. An all-3072 input should succeed on both sides of the boundary. An all-768 input is a separate request-conformance question: a global uniformity check alone still accepts it under the present validator. Do not claim that the narrow repair resolves every configured-dimension or provider-capability issue.

## Existing evidence and overlap

The reviewed `TestValidateEmbeddingBatchResponseEnforcesCardinalityAndShape` contains a two-vector mixed-dimension case. It verifies the helper's local contract but does not compose two separately accepted responses. The GenAI parallel benchmark uses 250 texts and a live key when authorized, but checks the returned error rather than injecting disagreeing chunk shapes. Those sources were read only; this report claims neither that those tests passed nor that an exhaustive repository-wide test inventory contains no other relevant fixture. [Validator fixture][validator-test] [Parallel benchmark][benchmark]

The novelty screen included the current main audit inventory and queue, embedding/store/prompt guidance and architecture, relevant implementation and the following reports:

- **Embedding representation and migration** already requires wrong-dimension rejection, discusses richer representation identity and notes the hard-coded GenAI dimension contract. Its evaluation case 9 is the general requirement this report makes concrete. It does not trace chunk-local validation through unchecked concatenation, ordinary successful storage, count-based readiness and later mismatch rejection. The general safety principle is credited to that report. [Representation audit][prior-representation]
- **Predicate inventory semantic fidelity** already identifies that counts do not prove membership or current content and that stale arity metadata can affect prompt signatures. The present witness uses one fresh, unchanged inventory and correct names/arities/content. Its defect enters through malformed vector shape before initial readiness, not an old corpus generation. An inventory-set equality fix alone would not reject the short vector. [Inventory audit][prior-inventory]
- **Embedding recursive pooling weight contract** concerns normalization and weighting when splitting one long input. This witness uses no long-input splitting or pooling and retains all input items. [Pooling audit][prior-pooling]
- **Embedding readiness waiter cancellation** concerns an Ollama mutex held through readiness network operations. The present issue uses GenAI response composition and needs no canceled waiter. [Readiness audit][prior-readiness]
- **Embedding stats observation boundary** and **vector migration null stamp continuation** concern observation APIs and legacy migration. The witness uses a non-null, current stamp written by an ordinary new batch, with no migration. [Stats audit][prior-stats] [Migration audit][prior-migration]

No dedicated report of this concrete composition path was found in that bounded screen. The general concepts of validation, representation identity and honest corpus readiness are not new.

## Required contract and bounded implementation choices

The following requirements are proposed, not shipped by this report.

### Validate the public batch result

For a successful nonempty `EmbedBatch` or `EmbedBatchWithTask` result:

- Return one vector per input in the promised input order.
- Every vector must be nonempty and finite.
- There must be one common dimension across the entire returned result, independent of request partitioning or completion order.
- A chunk or aggregate inconsistency must return an error before the caller can treat the batch as successfully admitted.
- Cancellation and provider errors must retain their causes; no partial result may be relabeled complete.

The minimal option is to run the existing validator over the final result before logging success and returning it. Another option is to retain each accepted chunk's dimension and compare it to a batch-wide dimension before exposing the composed result. A deterministic post-join comparison avoids introducing a shared mutable dimension race. Preserve the current ordering slots, errgroup join and early per-chunk checks.

Neither option should silently start rejecting all otherwise-uniform alternative-dimensional vectors merely by consulting the hard-coded `Dimensions()` method. Request-conformance and storage-schema compatibility deserve their existing coordinated design decision. The uniformity omission can be repaired without waiting for that broader migration.

### Separate storage receipt from usable coverage

The engine boundary should prevent this fresh malformed batch. A store boundary check provides defense for alternate engines and other mixed-response call paths. If the public store API promises a batch of usable same-space representations, reject or explicitly classify a mixed-shape batch before any canonical row is inserted. Partial publication must have an explicit per-item result and cannot be hidden behind a full-success count.

A row's model stamp establishes attribution, not every condition of serveability. Define the representation shape or validity status against which readiness is calculated. Counts and existing-content skipping must use that same qualification, while keeping model identity, corpus membership and source freshness as separate conditions. Do not replace the existing model filter with a dimension-only filter.

A historical malformed row requires a bounded repair path or an explicit incomplete state. Preserve original content and provenance. Do not fabricate a valid vector, overwrite its model stamp by guesswork, or make a vector usable by padding/truncation. A later successful reembedding may qualify the row; a failed repair should leave the deficiency visible rather than let the same stamp certify readiness again.

The existing predicate-inventory audit remains responsible for current membership/content coverage. A shape correction must compose with that work, not claim that an equal count of valid vectors proves the exact corpus is represented.

### Keep numerical validation outside executive policy

The host should reject an invalid numeric representation and expose a typed outcome. Mangle remains responsible for whether incomplete semantic coverage warrants a fallback, a narrower claim, more evidence or deferred work. No model should be asked to approve unequal-dimensional comparison, and a similarity result must never widen permissions.

Saltzer, Reed and Clark's *End-to-End Arguments in System Design* explains why local reliability mechanisms do not remove a higher-level correctness check when the higher level owns the complete requirement. Applied here as an engineering analogy, chunk validators remain useful for early failure, while the batch API must establish its own aggregate invariant. The paper does not prove this implementation correct or establish the frequency of malformed embeddings. [Primary literature][end-to-end]

## Full costs and failure modes

Let `N` be inputs, `K=ceil(N/100)` provider chunks, and `D` the normal vector dimension.

- The existing implementation already scans every vector component during per-chunk validation, so successful numeric validation is `O(ND)`. Reusing that validator after flattening adds another `O(ND)` pass. A shape-only cross-chunk comparison can be `O(K)` after each chunk's uniformity is established, or `O(N)` over vector lengths. The simpler full check provides stronger defense if future chunk paths change. These are algorithmic bounds, not timings.
- The current result is already materialized: retained numeric storage is approximately `O(ND)`, alongside input text, SDK objects and `K` chunk slots. A post-join check needs no second copy of the vector values. Bounded active provider calls do not imply a bounded total result size.
- The narrow check requires no additional provider call on a successful batch. If an inconsistent result is discovered only at the final barrier, the provider work for completed chunks has already been spent. A future fail-fast design may reduce wasted in-flight work but adds synchronization and cancellation/join obligations. Do not count canceled requests as free.
- Rejection prevents canonical inserts, index writes, repeated failed cosine comparisons and potentially misleading readiness. It can also force a retry or lexical/domain fallback, increasing latency or reducing semantic coverage. Count those costs and outcomes rather than reporting only validator CPU time.
- Storage qualification can add JSON parsing, dimension metadata, validity checks, SQL predicates and migration work. A full scan of existing rows is proportional to retained vector bytes. Prefer measured admission metadata for new rows and bounded repair for old rows; do not parse every historical vector during every prompt without measuring that cost.
- A batch-wide rejection discards otherwise valid vectors for this API call. That matches an all-or-error batch contract but can cost another embedding attempt. A partial-results API is an alternative design with extra provenance, ordering and readiness complexity, not a reason to report the present batch as wholly successful.
- Retrying a successful-but-malformed response indefinitely can amplify cost and conceal a provider contract violation. Use an explicit, bounded retry/fallback policy and preserve the original admission failure. No live-provider retry policy is changed by this report.
- Silent automatic coercion, selecting the first chunk's dimension as authoritative for all future engine calls, invalidating a legitimate alternate-model corpus, counting malformed rows as covered, or removing foreign-space filters would each create a new defect.
- Predicate domain selection and other representations may mitigate user-visible loss. Actual task quality, additional model turns and recovery expense remain unmeasured. A numerical count of stored rows is not a substitute for those outcomes.

## Unrun evaluation and closure criteria

Compare current code, a whole-batch validation repair, and that repair plus representation-aware storage/readiness on identical deterministic response schedules. Hold model identity, input order, task type, query vectors, corpus inventory, metadata and store mode fixed. Do not use a healthy live-provider run as the only test: it cannot exercise an intentionally invalid response reliably.

| Case | Required observation |
|---|---|
| Two mixed-length items within one chunk | Existing rejection remains |
| 100 normal items plus one short item in the next chunk | Entire batch rejected before successful store publication |
| Same invalid population permuted across a chunk boundary | Admission remains rejection after restoring item identity |
| 99, 100, 101, 200 and 201 compatible inputs | Cardinality and order retained; normal batching remains usable |
| 100/100/1 chunks with disagreement only in the last chunk | No false success or ready state |
| Reverse chunk completion order | Same verdict and input/output association |
| Wrong count, nil entry, empty or non-finite vector in any chunk | Existing rejection and informative cause retained |
| Error or cancellation in one chunk while others finish | Workers settle under existing ownership; no complete-success publication |
| Uniform alternate dimension | Behavior follows the explicitly retained compatibility contract; no accidental tightening |
| Fresh predicate corpus over 100 missing rows | Real metadata/task routing reaches the multi-chunk branch |
| Already-persisted same-stamp malformed row | Cannot certify usable corpus coverage; bounded repair or explicit incomplete state |
| Valid corpus after close and reopen | Readiness and actual recall agree |

First exercise the real GenAI batching code through its provider seam. Assert the returned shape/error and the number, identities and request dimensions of chunk calls. The minimal regression should fail if the final composition check is removed, rather than only calling the validator directly with an already-assembled mixed slice.

Then exercise `StoreVectorBatchWithEmbedding` with the production adapter through a suitable test-only provider boundary. For a rejected fresh batch, assert zero newly published canonical rows, no successful stored count, and no readiness certificate. For the current conditional witness, record the contradictory success, row count and later mismatch omission as expected pre-repair behavior, not as a desired acceptance criterion.

Finally instantiate a real `PredicateSelector` and fixture corpus containing more than 100 distinct predicates. Confirm the uniform `RETRIEVAL_DOCUMENT` route, ready latch, metadata count and subsequent semantic lookup. Keep direct domain selection enabled and report it separately, so a baseline fallback neither hides a broken semantic branch nor becomes an exaggerated claim of total task failure. Repeat the historical-row case with a new selector and after store reopen.

Run indexed and brute-force store variants separately. Indexed qualification must inspect canonical rows, actual index eligibility, error propagation and repair with the repository's supported sqlite-vec version; do not infer those outcomes from the brute-force fixture. A later explicitly authorized provider qualification can check the configured GenAI model/SDK against its documented dimensions and cardinality. It measures compatibility, not the prevalence of this hypothetical malformed response.

Report false-success batches, qualified versus stored rows, missing expected semantic candidates, readiness agreement, errors/fallbacks, retry counts, provider requests, provider usage when supplied, CPU, wall time, allocations, retained bytes, transaction/index I/O and repeated-query logging. Charge validation, failed work and recovery to the candidate strategy. Keep stochastic task studies separate and compare equal total budgets and assurance requirements if they are later warranted.

**Closure:** a batching optimization cannot change whether a mixed-shape logical result is accepted; successful storage and corpus readiness cannot conceal the demonstrated unusable row; valid ordered batches and existing model-identity safeguards continue to work. This closes the specific composition defect without claiming universal embedding compatibility, complete corpus freshness or improved coding-agent performance.

## Sources

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f
[chunking]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai.go#L271-L312
[chunk-validation]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai.go#L331-L369
[validator]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/engine.go#L198-L231
[store-publication]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_store.go#L291-L396
[count]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_store.go#L1188-L1216
[ready]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/prompt/predicate_selector.go#L565-L605
[recall-skip]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_store_bruteforce.go#L16-L75
[embedding-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/agents.md#L7-L20
[store-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/agents.md#L1-L25
[root]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/agents.md#L1-L34
[embedding-internals]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/architecture/embedding/INTERNALS.md#L1
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/architecture/AGENTS.md#L1-L76
[batch-admission]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai.go#L229-L268
[composition]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai.go#L310-L328
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_identity.go#L39-L90
[provider-seam]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai.go#L30-L70
[index-build]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/prompt/predicate_selector.go#L608-L651
[task-selection]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/task_selector.go#L107-L138
[store-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_store.go#L225-L296
[legislator]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/shards/system/legislator.go#L148-L169
[feedback]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/mangle/feedback/loop.go#L453-L471
[ingest-chunking]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/cmd/nerd/chat/ingest.go#L111-L124
[ingest-storage]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/cmd/nerd/chat/ingest.go#L134-L174
[recall-branch]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/store/vector_store.go#L619-L647
[index-completion]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/prompt/predicate_selector.go#L626-L662
[cosine]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/math.go#L10-L28
[selector-merge]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/prompt/predicate_selector.go#L449-L465
[validator-test]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/engine_coverage_test.go#L279-L323
[benchmark]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/internal/embedding/genai_bench_test.go#L10-L44
[prior-representation]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/embedding-representation-and-migration.md
[prior-inventory]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/predicate-inventory-semantic-fidelity.md
[prior-pooling]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/embedding-recursive-pooling-weight-contract.md
[prior-readiness]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/embedding-readiness-waiter-cancellation.md
[prior-stats]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/embedding-stats-observation-boundary.md
[prior-migration]: https://github.com/theRebelliousNerd/codenerd/blob/24cca7bd6de49e79872861f4631c2dc9bd6d3b7f/Docs/audits/vector-migration-null-stamp-continuation.md
[google-size]: https://aistudio.google.com/docs/embeddings#control-embedding-size
[end-to-end]: https://web.mit.edu/Saltzer/www/publications/endtoend/endtoend.pdf
