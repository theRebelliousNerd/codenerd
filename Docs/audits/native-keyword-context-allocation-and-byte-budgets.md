---
doc-class: research-proposal
subsystem: retrieval-context
implementation-status: proposed
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
feature-verified-against: c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
verification-method: static-source-inspection
---

# Native keyword context allocation and byte budgets

## Decision

Bound the bytes retained for keyword evidence, not only the number of hits. The native scanner converts an entire matched line to a string inside every retained-hit append. Several hundred matches on one long, admitted line therefore repeat the same full-line conversion. Those strings travel into the shared keyword cache before file ranking applies its result limit. The current one-pass scan removes repeated file reads across keywords, but it does not remove repeated line materialization across occurrences. [Scanner][scan], [search and publication][search], [ranking][ranking].

The proposed direction is to separate compact ranking evidence from source excerpts, bound any excerpt materialization before allocation, and account for bytes across scan-local work and the shared cache. Preserve existing membership, word-boundary behavior, occurrence counts, domain epochs, partial-result provenance and Mangle authority over context selection. A live definition-search consumer requires line-prefix semantics, so simply deleting every `Context` field is not a safe implementation. [Definition consumer][definitions], [root contract][guidance].

**Evidence limit:** this is a source-derived allocation risk and design proposal. No CodeNERD implementation code was changed or executed. No tests, compiler escape analysis, benchmarks, heap profiles or model inference were run. All sizes below are calculated payload models, not measured allocations, peak heap, RSS, production frequency or observed failures. Proposed evaluations remain unexecuted.

## Source freeze and novelty

Read-only connector inspection on 2026-10-28 resolved `main` to [`356f65e0e6bb639adaa90ab36212daa714114871`][main] and `feat/conversation-brief` to [`c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`][feature]. The research branch resolved to [`c592ff223cd207fca4c29ae793d5eba57aa4781d`][research]. Implementation claims below are pinned to main unless qualified otherwise.

The main and feature trees have identical blobs for `scan_native.go` (`f4e6ab020327909ab60111054b9647b21a1b14b5`), `search.go` (`bfafa8513e144657a4c8bc5fc474a70e90f303b8`), `keyword_cache.go` (`14dda632836724bae19dd6f13e5c23f57685971e`) and `scanner_generic.go` (`9a231b75cf8b6c727e6b4a25773fa185bb731b73`). The source-level conversion and retention mechanism therefore also exists on that feature pin. Its `sparse.go` differs: the inspected feature extractor additionally accepts identifiers in prose and domain terms. Its keyword collection still has no numeric keyword-count ceiling in `AllKeywords`. This is not a claim that the two branches produce identical queries or have identical entire retrieval pipelines. [Feature extraction][feature-extraction].

The existing [retrieval saturation audit][prior-saturation] establishes an all-keywords-full producer lifecycle defect and a ripgrep truncation/outcome mismatch. It calls for measuring peak memory and explicitly notes external subprocess buffering. It does not trace native per-occurrence line conversion, its cache lifetime, or the distinction between metadata-only ranking and a definition-prefix consumer. The present finding can arise in a one-file workspace, well below the 20,000-hit global cap, without stranding a producer. Repairing saturation alone leaves it intact.

The [earlier retrieval freshness audit][prior-freshness] already credits implemented domain/epoch invalidation and outcome propagation in its as-built addendum. Those protections are present and should be retained. This proposal is not another request to wire a missing retriever, cache failures differently, or replace Mangle selection with Go heuristics.

## Production path and ownership

The normal system factory creates `NewSparseRetriever(DefaultSparseRetrieverConfig(...))`, then shares its task retriever with the session executor and spawner. The default backend is native; ripgrep requires separate selection. The executor invokes `retrieveForTurn`; its bridge calls the attached retriever, and spawned executors inherit that attachment. `TaskRetriever.Retrieve` first asks whether the kernel wants retrieval and passes the shared retriever into `SeedIssueFacts`. This is a conditional live path for ordinary and delegated tasks, not an isolated benchmark helper. [Factory][factory], [configuration][config], [executor call][executor-call], [executor bridge][executor-bridge], [spawn inheritance][spawn-inheritance], [task retrieval][task].

Seeding uses a five-second default child deadline. The builder runs mentioned files, keyword search, import expansion and semantic expansion in sequence. In the keyword stage, `SearchKeywords` first collects cached hits, then scans all missing native keywords in one workspace walk. On a successful scan it copies each keyword's result into the cache before returning the aggregate hits. On a failed scan it preserves positive hits but skips publishing failed keywords. The failure protection limits cache residency; it does not undo allocations already performed. [Default seed deadline][seed], [seed application][seed-application], [stage order][stages], [search][search].

`RankFiles` groups the already acquired hits by path, computes weighted keyword relevance, retains each candidate's `Hits`, sorts candidates and only then slices to the requested file limit. The default builder permits 20 Tier 2 files and asks ranking for twice that many candidates. These limits do not constrain scan-local line conversion or cache admission. `TieredContextFacts` uses path, score and per-keyword occurrence totals; its fact arguments do not include `KeywordHit.Context`. Those context bytes are not directly injected into the task brief by this particular path. [Builder limits][builder-config], [keyword stage][keyword-stage], [ranking][ranking], [fact projection][facts].

There is an important second consumer. The definition fallback calls `searchSingleKeyword` for literal patterns such as `class Foo`, then checks `isLineLeading(hit.Context, pattern)`, including permitted modifiers. It therefore depends on line-prefix content. A metadata-only fast path must preserve that behavior through an explicit scan mode or typed prefix evidence rather than silently emptying or arbitrarily truncating all contexts. [Definition consumer][definitions].

## Source mechanism

`scanFile` checks `os.Stat` against a 4 MiB cutoff, reads the file and makes a lower-cased copy. For every requested keyword still open at the worker's last check, it calls `ScanBuffer`. After any match it constructs line slices and line offsets. For every word-bounded occurrence retained in that keyword slot, it appends a `KeywordHit` with this expression:

```go
Context: strings.TrimSpace(strings.TrimRight(string(lines[lineIdx]), "\r\n"))
```

The expression is inside the occurrence loop, not once per distinct line. There is no excerpt-length limit, per-line memo, shared line identifier or context-byte admission before this conversion. The local cap is 500 hits per keyword slot per file; the merge into the shared `found` map caps each keyword key at 20,000. The global cap is applied after `scanFile` has constructed the local results. Other workers can fill that global allowance while this worker continues constructing a file's now-unneeded local hits. [Limits][limits], [file scan][scan], [worker merge][workers].

The generic scanner collects all non-overlapping substring offsets before the caller applies word boundaries and the 500 retained-hit cap. Thus a hit-count cap is not an offset-vector cap either. The current `scanner_generic.go` is unconditional and is the only retrieval `ScanBuffer` implementation in the inspected tree; its source explains removal of the old amd64/SIMD variant. The primary finding is not limited to a SIMD tag, amd64, Windows or Unix. Architecture and compiler still affect actual memory layouts and attainable allocations. [Scanner implementation][generic].

### What Go semantics support

The [Go language specification](https://go.dev/ref/spec#Conversions_to_and_from_a_string_type) defines the bytes produced by a byte-slice-to-string conversion, but does not promise a particular allocation count. The [standard runtime conversion implementation](https://go.dev/src/runtime/string.go#L134) ordinarily allocates and copies long string data; compiler optimizations can alter which helper or allocation is emitted. This review did not inspect generated code for the repository's declared toolchain, so the defensible statement is repeated full-line conversion and a conventional-copy amplification risk, not a measured proof of one distinct heap object per occurrence.

The [standard string trimming implementation](https://go.dev/src/strings/strings.go#L1086) returns substrings. Trimming whitespace after converting a whole line is not a byte-budget guarantee; a shorter resulting string can retain a larger backing allocation. In the witness below no trimming occurs, avoiding that additional subtlety.

## Calculated payload models

Let `L` be the byte length of one stable, admitted line, and let `h` be the number of retained occurrences of one keyword input slot on it. The code creates `h` context string values of length `L` when no trimming applies. Their total logical lengths are `h × L`. If each long conversion materializes independent storage, context payload is approximately that amount, before headers, allocator rounding, file buffers and other structures.

| Conditional case | Calculation | Logical context bytes | Binary units |
|---|---:|---:|---:|
| One 256 KiB line and 500 retained occurrences | `500 × 262,144` | 131,072,000 | 125 MiB |
| One 4 MiB line and 500 retained occurrences | `500 × 4,194,304` | 2,097,152,000 | 2,000 MiB or 1.953125 GiB |
| One keyword key at 20,000 retained hits, each on a 4 MiB line | `20,000 × 4,194,304` | 83,886,080,000 | 80,000 MiB or 78.125 GiB |

The final row can be constructed logically from 40 eligible single-line files with 500 retained hits each. It is a conditional scale calculation, not a claim that a machine, deadline or allocator will let the scan reach that state. A process may time out or fail much earlier. The first row already shows why the issue does not require global saturation.

For distinct keywords, each keyword's retained context payload can contribute independently. `AllKeywords` concatenates category lists without a fixed cardinality ceiling; deduplication in extraction is per category, not across the complete list. The 500 bound therefore applies to an input slot rather than necessarily to a unique spelling appearing in several categories. The one-keyword witnesses deliberately avoid duplicate slots. No finite universal process-memory bound follows merely by multiplying the configured worker count by two file buffers. [Extraction and keyword collection][keywords], [worker budget][worker-budget].

### Why these are not process bounds

- The 4 MiB admission check precedes an unrestricted `os.ReadFile`; there is no post-read length enforcement. The numeric model assumes a stable ordinary file, not one that grows or changes between stat and read. [File admission][scan].
- A worker retains local results while the global `found` collection already holds earlier files. Several workers may temporarily construct results later discarded at the global cap. [Worker merge][workers].
- Raw offsets, line slice headers, line offsets, original/lower-cased buffers, hit arrays, maps, queued paths and ranking arrays add cost. Some costs depend on newline density, keyword length and unsuccessful boundary matches, not retained-hit count. [File scan][scan], [generic scanner][generic], [ranking][ranking].
- Default parallelism is `min(NumCPU, 32)`, but explicit positive configuration is not capped there. `workerBudget` divides by a snapshot of active searches; older pools are not resized, so 32 is not a shared hard process-wide worker limit. [Configuration][config], [worker budget][worker-budget].
- `scanFile` accepts no context and checks no deadline inside file matching or context creation. Cancellation is checked between files and during producer sends. A five-second seed deadline therefore does not impose a five-second hard stop on a file already being processed. [Scan and workers][scan], [producer and join][producer].
- Incremental invalidation reuses `scanFile` for every changed member and cached keyword, accumulates additions, and only then applies cache-result caps. That route has no context parameter at this boundary and can construct more transient data than the final 20,000-hit result cap suggests. [Incremental path][delta], [delta admission][cache-delta].

These limitations strengthen the need for acquisition-side accounting, but are not separate measured incidents. Use an ASCII witness to isolate allocation behavior from Unicode matching and coordinate concerns.

## Lifetime and cache behavior

The relevant ownership chain is file-local hit slices, the shared `found` map, aggregate search hits, cache entries and ranked candidates. Copying a `KeywordHit` copies its string value, not each string's bytes again. Accordingly, cache publication, cache reads and ranking prolong ownership and add metadata arrays, but should not be counted as another full deep copy of every context. [Hit type][hit-type], [cache][cache], [ranking][ranking].

The default cache holds up to 1,000 keyword entries, not a fixed number of bytes. `Set` copies hit structs and evicts by entry count. `Get` returns another hit-struct copy and removes an entry if that particular lookup observes it expired. The five-minute TTL is lazy; it is not a timer that releases every string five minutes after insertion. Entries can remain reachable until an expired lookup, eviction, replacement, domain reset, appropriate invalidation or retriever disposal. Concurrent callers may keep their own string references after cache eviction. This is retained ownership, not an established permanent leak. [Configuration][config], [cache lookup and admission][cache].

The cache's domain epochs and delta invalidation are valuable correctness mechanisms, but the delta publication path has a separately documented [generation-fencing gap](https://github.com/theRebelliousNerd/codenerd/blob/80cb12ed74abf8a33951e2f16df9ab6e507a1e37/Docs/audits/retrieval-delta-publication-fencing.md). A new byte budget must not permit stale results to republish after an epoch change, or label a byte-truncated acquisition complete. Likewise, eviction should not alter the validity of a result already returned to a caller. [Cache epochs][cache-epochs], [search outcomes][search].

## Unexecuted witness and controls

Construct one stable, eligible text file with no newline and no NUL. Put 500 occurrences of the lowercase identifier `quasar` near its start, each followed by a non-word delimiter, and fill the remaining line with a non-space ASCII character that does not introduce another occurrence. End with a non-space byte. Use a filename not excluded by the purpose filters, and satisfy actual workspace membership.

The task text `repair the "quasar" bug` can supply a single tertiary keyword on main. A 256 KiB line yields 500 returned context values, each 256 KiB, if acquisition completes; the calculated repeated payload is 125 MiB under independent long-string materialization. At exactly 4 MiB the file passes the source's strict greater-than size rejection, producing the larger model above. This is a reasoning witness, not a recommendation to run a high-memory test without isolation. [Extractor][keywords], [admission and conversion][scan].

Controls must distinguish mechanisms:

1. The same number of occurrences spread across short lines should reduce repeated context bytes even though the hit count remains 500.
2. A long file with no substring match should not enter context construction; a long line with many non-word-bounded substrings stresses raw offsets but produces no retained contexts.
3. One match on the long line establishes baseline whole-line retention without occurrence amplification.
4. A 500-hit file remains below the global cap, separating this finding from all-reader saturation.
5. Oversized, binary, excluded and nonmember files should preserve their existing admission behavior.
6. Duplicate keywords, several keywords on one line and simultaneous scans must be covered separately rather than inferred from the single-slot witness.

## Proposed contract and alternatives

### Preferred direction

Represent ranking evidence compactly: path and revision identity, keyword, occurrence count and bounded positional samples. Materialize source text only for a consumer that needs it, under an explicit excerpt budget. For any deferred read, bind positions to the observed revision and revalidate before presenting the excerpt; an old offset is not an address into a newly edited file.

For definition fallback, either compute the required line-leading/modifier predicate while the original bytes are present or carry bounded, sufficient prefix evidence with an explicit insufficiency state. Preserve the current case and modifier semantics; arbitrary prefix truncation must not silently turn a definition into a nonmatch.

Add per-result and shared-cache byte accounting, with accounting defined over actual owned payload rather than summing aliased string lengths repeatedly. Admit before materializing new context bytes. A rejected cache publication may safely leave an uncached complete result; dropping acquisition evidence requires a truthful partial/budget outcome. Preserve Go's role as bounded acquisition and accounting while Mangle continues to derive what enters active context.

### Alternatives and tradeoffs

| Approach | Benefit | Remaining cost or risk |
|---|---|---|
| Convert each distinct matched line once per file and share it across hits | Narrow compatibility-preserving reduction in repeated conversions | Long lines still retained; many distinct lines/files and keywords still need a byte budget |
| Clamp excerpts before byte-to-string conversion and share by line or excerpt key | Explicit payload ceiling; avoids copying a whole line first | Must define byte/rune boundaries, hit-relative offsets, truncated evidence and definition-prefix handling |
| Metadata-only rank scan with separate revision-aware excerpt acquisition | Avoids carrying unused source text through ranking/cache | More consumer contracts; source may change between scan and later read |
| Aggregate counts plus bounded positional samples | Reduces hit-record and context costs | Changing `Hits` length changes present keyword-count facts unless aggregation is explicitly migrated |
| Byte-weighted cache admission and eviction | Controls long-lived cache ownership | Does not prevent pre-admission scan allocations; shared payload needs consistent ownership accounting |
| Reduce hit/file/worker limits only | Small tactical change | Changes recall or concurrency; does not remove the occurrence-times-line-length mechanism |
| Rely on GC tuning or a process memory limit | Operational containment | Reachable data cannot simply be reclaimed; runtime soft limits are not a correctness budget |

A narrow first repair can share each line conversion without changing public hit semantics, followed by independent byte-admission work. Do not describe that first step as a complete process-memory bound. Conversely, a cache-only limit is too late for the one-file witness.

The official [Go slice ownership discussion](https://go.dev/blog/slices-intro) explains why smaller views can keep larger backing storage alive. It supports explicitly copying only an admitted excerpt rather than converting a full file and keeping a tiny substring. The [Go GC guide](https://go.dev/doc/gc-guide) separates live heap, allocation rate and collection cost and describes the runtime memory limit as soft. These primary sources motivate measurement and ownership discipline; they do not prove a CodeNERD allocation profile or a measured speedup.

## Costs and risks to measure

Track distinct quantities rather than one synthetic cost score: bytes read, lower-case work, raw offsets, unique lines materialized, context bytes copied, hit metadata bytes, cache-owned bytes, shared/aliased bytes, peak live heap, total allocated bytes, GC CPU, RSS, queue time, per-stage latency, cancellation delay and complete task latency. Existing metrics expose hits, files, cache events, errors and accumulated search time, but not these context-byte ownership quantities. Add counters only where their definition and synchronization cost are clear. [Current metrics][metrics].

The expected benefit is reduced copying and retained heap, potentially preserving more retrieval budget for imports, semantic search and useful task execution. None is measured here. Report candidate coverage, correct definition discovery, verified task success, model turns, model input/output tokens and recovery effort alongside memory and time. Raw native context bytes are not automatically model tokens; the task fact path projects counts and paths. Monetary savings require actual usage and provider pricing and are not estimated.

Sharing or aggregation adds indexing and lifecycle complexity. Byte admission adds accounting and possibly synchronization. Deferred excerpts add reads and revision checks. Lower budgets can reduce recall or discard the informative end of a long line. Hot-cache workloads may trade lower retained heap for more cold rescans. A changed representation can break external callers or test fixtures even when ranking looks unchanged. Validate those tradeoffs rather than asserting that a smaller heap is inherently a better assistant.

The pre-existing saturation problem must remain separately tracked: an allocation repair should not claim to join the producer, repair ripgrep outcomes, or improve cancellation unless those behaviors are also implemented and tested. [Prior saturation contract][prior-saturation].

## Evaluation and completion criteria

All following work is proposed, not executed. Start with small safe fixtures and isolated memory ceilings, then scale only when instrumentation confirms the expected shape. Record toolchain, OS, architecture, CPU count, configuration and backend. Inspect compiler escape/optimization output for the exact deployed build before claiming one allocation per source conversion.

1. **Allocation shape:** vary line length and occurrences independently. Compare baseline, per-line sharing, bounded excerpts and metadata-only ranking. Measure allocation bytes/objects and retained bytes with results alive, after uncached result release, with cache alive and after cache/caller release. Distinguish logical string lengths from unique backing allocations.
2. **Semantics:** verify paths, keyword counts, line/column positions, ranking order, partialness and default task facts. Test multiple occurrences per line and multiple keywords per line. Preserve current file filters and domain epochs.
3. **Actual consumers:** exercise `TaskRetriever` through the normal factory path with CodeDOM and world context enabled, plus definition fallback with indentation, modifiers, a definition beyond a candidate excerpt boundary and a nondefinition occurrence. A microbenchmark alone does not establish normal-path correctness.
4. **Cache lifecycle:** cover cold/warm reads, exact byte-admission boundary, too-large entry rejection, entry-count and byte eviction, lazy expiry, replacement, invalidation during a scan and a caller retaining hits after eviction. Do not mistake lower cache size for lower total live memory if callers still own data.
5. **Acquisition and cancellation:** vary workers, concurrent requests, already-full keywords and a deadline expiring inside a long file. Incremental maintenance needs its own changed-file workload because its accumulation occurs before final cap admission.
6. **Safe resource controls:** include many short lines, raw substring matches that fail word boundaries, zero hits, long but excluded files and a stat/read size-change fixture if enforcing a true read cap. Report unsupported cases explicitly.
7. **End-to-end comparison:** use identical repositories, tasks, model/tool settings and total budgets. Count failed and timed-out tasks. Require no unexplained loss of useful candidate/definition coverage and report any actual reduction in latency, memory, turns or human recovery separately.

Existing source-defined tests already cover skipping oversized/binary files and limiting per-file hit count. The per-file cap fixture uses many short lines, so it does not demonstrate bounded bytes for many hits on one long line. The multi-keyword equivalence fixture compares contexts and positions on small files and is a useful compatibility control, not an allocation measurement. None was executed in this review. [Bounds tests][bound-tests], [Multi-keyword tests][multi-tests].

Before implementation, follow the repository's spec-first workflow for the affected retrieval contracts. Completion requires an explicit ownership/admission specification, preserved consumer semantics, executed resource and normal-path validation, and source-linked updates to the architecture/current-state records. This research closes none of those implementation obligations.

## Source references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871
[feature]: https://github.com/theRebelliousNerd/codenerd/commit/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c
[research]: https://github.com/theRebelliousNerd/codenerd/commit/c592ff223cd207fca4c29ae793d5eba57aa4781d
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md
[prior-saturation]: https://github.com/theRebelliousNerd/codenerd/blob/c592ff223cd207fca4c29ae793d5eba57aa4781d/Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md
[prior-freshness]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/retrieval-freshness-coverage-and-context-value.md
[feature-extraction]: https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/retrieval/sparse.go#L266-L308
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L2602-L2616
[executor-call]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1218-L1223
[executor-bridge]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/issue_retrieval.go#L40-L57
[spawn-inheritance]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/spawner.go#L438-L447
[config]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L56-L121
[task]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/decisions.go#L294-L370
[seed]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/facts.go#L206-L244
[seed-application]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/facts.go#L202-L245
[stages]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L178-L228
[builder-config]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L62-L129
[keyword-stage]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L367-L408
[search]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/search.go#L47-L160
[limits]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L21-L49
[scan]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L139-L198
[workers]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L213-L257
[worker-budget]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L51-L67
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scan_native.go#L260-L322
[generic]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/scanner_generic.go#L7-L36
[keywords]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L249-L277
[hit-type]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L283-L301
[ranking]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/sparse.go#L354-L430
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/facts.go#L434-L505
[definitions]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/tiered_context.go#L633-L686
[cache]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L76-L126
[cache-epochs]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L61-L115
[delta]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/invalidation.go#L97-L162
[cache-delta]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/keyword_cache.go#L152-L210
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/metrics.go#L21-L55
[bound-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/bounds_test.go#L36-L131
[multi-tests]: https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/retrieval/multi_keyword_pass_test.go#L14-L79
