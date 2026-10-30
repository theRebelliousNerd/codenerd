---
doc-class: research-proposal
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Batch the native definition fallback without weakening retrieval

## Decision

Extend the existing native multi-keyword scan to the Tier 4 definition fallback. The normal keyword tier already reads and lowercases each eligible file once for all missing keywords. The fallback instead invokes a separate uncached workspace scan for each of **nine** definition prefixes, for every mentioned symbol it processes. A repeated request can therefore have a warm keyword cache and still repeat all this file acquisition.

This is a bounded, source-established work-amplification gap, not a demonstrated latency incident or an unsafe acceptance defect. For a stable eligible corpus, absent definition patterns, sufficient remaining time and `S` extracted symbols, the fallback completes `9 × S` workspace walks. If its inherited deadline expires, only a prefix of that work may complete; the remaining attempts can be partial or immediately canceled. No ninefold wall-time, token or monetary saving is claimed.

The narrow repair is to acquire the nine patterns for one symbol in a single native pass, retaining the current filtering and selection semantics. Broader indexing, permanent caching, language pruning and removal of the fallback are not prerequisites.

## Source freeze and prior work

Public `main` resolved to [388205138ba6d47c1cff0c5e97c896dfa642a391][pin] during this review on 2026-10-30. Recent main changes include the prompt-evolution screening-cohort repair, strategy-context ownership, knowledge-synthesis answer ownership and verifier deletion-evidence repair. This finding is based on the implementation at the pinned main, not on old audit text or an unmerged branch.

Relevant earlier work was reviewed and is credited:

- The [retrieval freshness audit and its as-built account][freshness] already describe the native batched keyword pass, domain-aware caching, partial outcomes and semantic reuse. This proposal does not reopen those original missing-wiring or stale-negative-cache claims.
- The [saturation audit's as-built account][saturation] records the completed native producer-lifecycle and external cap-report repairs. The current scanner has the saturation channel, joins readers, and returns cap-aware results. The witness here has **no matches**, never reaches a hit cap and does not require a stranded producer.
- The [native keyword allocation audit][allocation] already identifies the definition fallback as a consumer of line-prefix evidence. Its specific mechanism is repeated full-line string materialization per retained hit and cache-owned bytes. The present mechanism is repeated file acquisition across different definition patterns; it exists even when there are no hits and no line excerpts are retained.
- The [delta-publication audit][delta] concerns a cache writer's missing generation fence. The present fallback bypasses the keyword result cache. Batching it need not introduce any persistent cache or depend on that separate repair.

Targeted audit searches for `findSymbolDefinitions`, `searchSingleKeyword` and definition scanning found no existing audit specifically proposing this acquisition batching. That is a bounded overlap review, not proof that nobody has previously considered the optimization.

## The live production path

The [system factory][factory] creates `SparseRetriever` with its default configuration and shares a task retriever with the session executor and spawner. No external backend is selected by that default configuration. The [semantic initializer][semantic-init] leaves the searcher absent when no embedding engine exists, explicitly preserving the definition fallback.

Retrieval is conditional. [Mangle policy][policy] requests the sparse pass for supported issue-oriented intents including `/fix`, `/debug`, `/review` and `/security`. [TaskRetriever.Retrieve][task] asks that policy before calling `SeedIssueFacts`; later it asks which files and caveats to render. The [executor bridge][bridge] attaches the resulting brief to task evidence rather than instructions. This is not a claim that every conversation turn runs the fallback.

The [seed implementation][seed] reuses the supplied sparse retriever, but creates a tiered builder for the pass. It gives the tiered build a child context with a five-second default deadline. The builder runs the mentioned-file, keyword, import and semantic stages in order. That single deadline is shared across the stages; Tier 4 receives whatever time remains. The deadline is a cancellation mechanism, not a demonstrated strict five-second bound on the enclosing function's return time.

Within Tier 4, [semanticExpansion][fallback] returns early if the embedding search supplies usable new files. A missing backend, an embedding error, or a result with no usable new matches leads to the definition fallback. Merely configuring embeddings therefore does not prove this path is unreachable; successful usable semantic results are an important negative control.

## Exact work amplification

The [extractor][extraction] recognizes non-common capitalized identifiers as mentioned symbols and deduplicates the list. Consider a valid fix task referring to `QuasarWidget` and `ZenithAdapter`. These spellings supply two distinct symbols without relying on a model-generated tool call.

The fallback loops over those symbols. For each one, [findSymbolDefinitions][definitions] constructs and searches these nine literal prefix forms:

`class`, `def`, `func`, `function`, `fn`, `type`, `struct`, `interface`, `impl`.

For each pattern it calls `searchSingleKeyword`, filters the returned lines using `isLineLeading`, and deduplicates the resulting file paths. The call does not stop after the first matching prefix. The outer tier stops after enough **new selected files**, not after a fixed amount of acquisition. At the default total of 50 files and 10% Tier 4 allocation, that output cap is five files. Symbols with no definitions do not advance it. Definitions whose files are already present in an earlier tier also do not fill the new-file allowance. [Defaults and allocation][budgets], [outer loop][fallback].

The [single-keyword adapter][single] creates a child timeout and, on the default native backend, directly invokes `searchKeywordsNative` with a one-element keyword slice. It neither reads nor writes the keyword result cache. Each of the nine serial calls receives a fresh per-search timeout, still bounded by its inherited seed context.

Each native invocation creates a worker pool and runs [workspace traversal][walk]. For an admitted, readable, nonbinary file within the size bound, [scanFile][read] performs a fresh stat, `ReadFile` and lowercase transformation before matching. Thus the nine calls repeat acquisition of the same eligible source bytes, even though those patterns could be matched against one acquired buffer.

Contrast the existing [Tier 2 search][batched]: it serves cached keywords first and sends all missing native keywords to one `searchKeywordsNative` call. The mechanism needed to amortize file acquisition already exists and is exercised by other production callers.

### Bounded witness

Use a stable workspace with ordinary eligible text files and no occurrence of either example symbol. Run a retrieval-admitted fix task with no embedding backend, the default nonzero Tier 4 budget, and enough remaining deadline for the small corpus to be fully searched.

1. Tier 2 acquires all missing keywords in one batched native pass and can cache their empty results.
2. Tier 4 processes both extracted symbols.
3. Each symbol produces nine separate full native scans. No definition hit fills the tier's output allowance.
4. Repeating the request with the same shared retriever before cache expiry can make Tier 2 entirely cache-backed. Tier 4 still performs the same eighteen full scans.

If each full scan successfully reads `N` eligible text files, the fallback performs `18N` such file reads in this two-symbol case. This is a conditional source-level operation count, not a benchmark, a prediction of physical disk traffic, or a statement that all production requests finish eighteen scans.

For larger corpora, slower filesystems, additional symbols or expensive earlier stages, the shared deadline can instead interrupt a scan. The code continues to attempt later patterns and symbols, but canceled attempts must not be counted as completed workspace traversals. The defensible consequence is avoidable work within the available retrieval budget, with possible partial fallback coverage. Whether it causes a user-visible delay, misses a useful file or changes the eventual task outcome requires execution-based evidence.

## Controls and alternative interpretations

**This is not an unbounded wait claim.** There is a seed deadline, per-search timeout, worker sharing, file-size and binary admission, per-file and per-keyword hit limits, native saturation handling, and an output-file cap. Preserve all of them. Their existence does not make repeated successful acquisition free, but it limits the failure mode.

**The fallback has a legitimate purpose.** It remains useful without embeddings, on an embedding failure and when semantic results provide no new files. Disabling it solely to reduce scan counts would trade away evidence coverage. Similarly, selecting only the apparent language of an explicitly named file could miss a cross-language definition. Such changes need separate coverage evidence.

**The strings are different queries.** Nine literal patterns require matching work; the claim is not that eight searches are semantically redundant. The avoidable part is independently reopening and lowercasing the same file for each pattern. A one-pass implementation still needs to match all requested patterns.

**Warm operating-system caches matter.** Repeated `ReadFile` calls may be served without physical disk reads. They still repeat file opening/reading into application buffers and case-folding work. The relative latency benefit depends on filesystem, corpus, CPU, cache state and contention.

**The brief is not a repository census.** It describes starting points, and structural tools remain available. A partial fallback does not establish an incorrect final answer or a missing test. The [outcome path][fallback] retains the first scan error and reports a partial tier where appropriate; Mangle controls which caveats reach the model.

**Current cap reporting has a separate limitation.** `searchSingleKeyword` returns `BackendResult.CapReached`, but `findSymbolDefinitions` reads only `Hits` and `error`. A successful cap-reached definition scan therefore does not by itself mark Tier 4 partial; only its error/embedding branches do that. The repaired scanner-level cap contract must not be described as fully propagated through this consumer. The no-hit witness above is below every cap and does not depend on this omission. If the acquisition API is refactored, carry the cap outcome through it rather than perpetuating or hiding the loss. [Single-keyword result][single], [definition consumer][definitions], [tier outcome][fallback].

**One batched timeout is not universally equivalent to nine serial timeouts.** At the defaults, the five-second seed context expires before the thirty-second per-search allowance, so that shared context is the relevant ceiling. With a supported custom seed budget longer than `searchTimeout`, the existing nine calls each start a fresh per-search timeout. Replacing them with one call receiving only one such allowance changes the available acquisition time. Conversely, removing that allowance can lengthen one scan. Specify the batched timeout policy and test nondefault combinations; do not assert universal behavioral equivalence from the default-budget witness. [Seed timeout][seed], [search configuration][search-config], [adapter][single].

**The number of symbols is not every word in the task.** Extraction filters and deduplicates its symbol list; lowercase-only tasks can yield no mentioned symbols. Early usable embedding results and a filled Tier 4 output budget also avoid some or all of this path.

## Narrow implementation contract

Prefer batching the nine literal patterns **per symbol** on the native backend first. This retains the outer symbol-by-symbol work boundary and does not pre-acquire every later symbol after an earlier symbol fills the output tier.

A safe implementation should:

1. Acquire all nine prefix patterns with the existing native multi-keyword mechanism, under the inherited seed deadline.
2. Consume per-pattern results in the existing prefix priority, preserve the outer symbol order, and retain file deduplication and output-tier admission. Do not accidentally replace priority with map iteration.
3. Preserve literal matching, case folding, word boundaries and the existing line-leading/modifier checks. The allocation audit is correct that arbitrarily removing or truncating `KeywordHit.Context` can break this consumer.
4. Retain membership, purpose filters, file-size/binary limits, per-pattern hit caps, worker sharing, cancellation and reader joining. Preserve and expose acquisition outcomes, including the cap flag this consumer currently drops. Define the timeout policy explicitly rather than silently treating one batched allowance as equivalent to nine fresh allowances.
5. Keep the external backend's contract explicit. An initial native-only batching repair can leave the optional external path unchanged; batching subprocess arguments is a different adapter change.
6. Avoid using a persistent definition-result cache as a shortcut. If later introduced, its identity, mutation invalidation, partialness and publication fencing must be specified independently.

Batching all symbols in one pass may save more traversal work, but can acquire patterns for symbols the present early-output stop would never visit and retain a larger hit set. Evaluate that option separately rather than assuming it dominates the smaller change.

The existing multi-keyword primitive preserves matching behavior in a small below-cap fixture. The strongest equivalence claim here is for the **per-pattern hit sets** of complete, below-scanner-cap searches over a stable corpus, not automatically for the final selected file identities. Native readers append results in completion order. If a prefix has more than five eligible new definition files, the default Tier 4 output cap can select a different five after batching changes that completion order, even when every hit is acquired and neither scanner hit cap is reached. Preserving symbol and prefix priority alone does not eliminate this difference. Concurrent mutations, scanner-capped selection and custom timeout combinations also require separate contracts and tests. Do not promise identical incidental result order, selected files or deadline-limited coverage without defining and implementing those contracts.

## Costs and fair evaluation

A per-symbol batch reduces repeated traversal, membership checks, file acquisition and lowercase transformations. It does **not** eliminate per-pattern matching complexity. It may retain several patterns' hits concurrently where the current method finishes one pattern at a time, increasing peak memory. The prior line-allocation concern is still relevant and may interact with this change. Track that tradeoff rather than reporting only reduced scan counts.

Existing [retriever metrics][metrics] expose walked/scanned/skipped files and aggregate time. `searchKeywordsNative` increments `searches` by the number of keywords, so that field is not an independent count of physical workspace walks. A batch of nine patterns can still increment it by nine. Use an explicit test counter or acquisition observer for walk starts and file reads; report `FilesWalked` and `FilesScanned` separately.

Current [definition tests][definition-tests] cover real definitions, absent symbols, indentation, modifiers and call-site rejection. The [multi-keyword test][multi-test] compares four terms across a small fixture. Those are useful existing controls, but neither establishes this production fallback's acquisition count or the cost of repeated warm passes. The opt-in timing test in the same file was not run.

Proposed acceptance and evaluation:

- A production-path fixture with two absent extracted symbols and sufficient deadline verifies the current eighteen fallback walks, then the proposed two native walks, without changing empty results or selection facts.
- Repeat with a shared warm retriever. Confirm Tier 2 cache hits while measuring Tier 4 independently; do not attribute the keyword-cache improvement to the new repair.
- Preserve mixed-language definitions, modifier/indentation behavior, call-site rejection, already-selected files, symbol and prefix priority, and the default output cap. Include more than five eligible definitions for one prefix while staying below scanner hit caps; compare acquired hit sets separately from selected file identities. Either define a compatible stable selection order or explicitly assess the selection change rather than calling hit-set equality full behavioral equivalence.
- Include no-symbol tasks, usable semantic results, semantic failure and no-new-match fallback as negative and positive controls.
- Exercise timeout and caller cancellation mid-acquisition, including custom seed budgets longer than the per-search timeout. Test one saturated pattern with another still open and verify cap status reaches the tier rather than reproducing the current consumer omission. Preserve membership exclusions, binary/oversized-file admission and concurrent retrieval behavior; report partial acquisition honestly.
- Compare per-symbol batching with the unchanged implementation on fixed small, medium and large corpora, cold and warm filesystem caches, and identical deadlines and worker settings. Report traversal count, bytes read, lowercasing work, peak live memory, allocation volume, per-tier latency and timeouts separately.
- Only after deterministic behavior is established, compare matched end-to-end tasks at equal total model/tool/verification budgets. Use paired quality checks for cases whose output-capped selected files change despite equal complete hit sets; include ambiguous multi-definition requests and useful files near the selection boundary. Include failed and timed-out tasks; measure useful candidate coverage, verified completion, model turns/tokens and human recovery. Do not equate scanner bytes with billed model tokens or claim cost savings from a microbenchmark alone.

No repository implementation was changed, no test or benchmark was run, and no model inference or runtime reproduction was performed for this report. This document proposes a narrowly testable refinement to an existing retrieval path.

## References

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2697-L2710
[semantic-init]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory_semantic.go#L12-L34
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/retrieval.mg#L12-L23
[task]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/decisions.go#L326-L381
[bridge]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/issue_retrieval.go#L39-L76
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/facts.go#L184-L245
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L578-L642
[extraction]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/sparse.go#L169-L219
[definitions]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L645-L731
[budgets]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/tiered_context.go#L91-L151
[single]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/scan_native.go#L96-L119
[search-config]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/sparse.go#L73-L87
[read]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/scan_native.go#L140-L198
[walk]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/scan_native.go#L207-L358
[batched]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/search.go#L75-L157
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/metrics.go#L34-L98
[definition-tests]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/bounds_test.go#L168-L241
[multi-test]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/retrieval/multi_keyword_pass_test.go#L55-L100
[freshness]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-freshness-coverage-and-context-value.md#L189-L213
[saturation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md#L233-L258
[allocation]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/native-keyword-context-allocation-and-byte-budgets.md#L13-L39
[delta]: https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/retrieval-delta-publication-fencing.md#L21-L31

