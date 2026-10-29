---
doc-class: research-proposal
subsystem: retrieval-context
implementation-status: partial
last-verified: 2026-10-27
verified-against: e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a
verification-method: static-source-inspection
---

# Retrieval freshness, coverage, and context value per token

**Status:** documentation-only source audit and proposed design, not an implementation or measured result.  
**Inspected revision:** `e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a` (2026-10-27).  
**Scope:** task retrieval → kernel facts → model-facing brief → revision-aware working context.  
**Suggested destination:** `Docs/audits/retrieval-freshness-coverage-and-context-value.md`.

## 1. Decision

Prioritize **correct negative-result invalidation and honest retrieval coverage** before adding a larger index or more context. Then make semantic reuse revision-safe before extending its lifetime. A cache hit or a short prompt is not an efficiency win if it hides the newly relevant file and forces another model turn.

This is a refinement of existing CodeNERD mechanisms, not a new retrieval architecture. Keep Mangle as the executive over retrieval and context; Go reports bounded observations and performs IO. Keep source facts, relevance estimates, and inferred relevance distinct.

No code was executed, no tests were run, no inference was performed, and no runtime outcome is claimed here. Findings below are static source deductions with counterexample designs for subsequent authorized verification.

## 2. Ground truth: what is already wired

The root [repository contract](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/agents.md) requires world-model/CodeDOM-first context, recoverable eviction, current evidence after edits, and bounded requests rather than arbitrary turn ceilings.

The current production chain is:

1. [factory.go:2593–2601](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/system/factory.go#L2593-L2601) constructs a shared sparse retriever and attaches a task retriever to both session executor and spawner.
2. [executor.go:1200–1201](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/executor.go#L1200-L1201) calls and releases per-turn retrieval.
3. [`internal/retrieval/decisions.go:289–336`, TaskRetriever.Retrieve](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/decisions.go#L289-L336) asks `issue_retrieval_wanted`, seeds issue-scoped facts, asks `retrieval_brief_file`, and renders only derived files. Concurrent issue facts are scoped; unscoped candidate facts are intentionally excluded by [`scopedFacts`, decisions.go:115](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/decisions.go#L115).
4. [SeedIssueFacts](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/facts.go#L171-L295) invalidates the shared keyword cache from kernel events and runs a bounded four-tier builder.
5. [`internal/session/issue_retrieval.go:71–76`, withRetrievalBrief](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/issue_retrieval.go#L71-L76) appends the brief to task evidence, not the system instructions.
6. `internal/session/working_context.go` installs a working store in [`beginWorkingLoop:198`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L198), records observations at the tool-effect boundary and preserves recall identities in [`recordWorkingResult:326`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L326), applies the ledger in [`updateLedger:744`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L744), and refuses an oversized request if all removable results are archived and the remaining essential request still cannot fit in [`prepareWorkingRequest:577–642`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L577-L642).

**Do not propose wiring retrieval into the executor as though it were absent.** The [July retrieval gap matrix](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/retrieval/03-GAP-ANALYSIS.md) explicitly carries a later correction. The [September context gap matrix](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/Docs/architecture/context/03-GAP-ANALYSIS.md) also cannot be used as a live caller index: the inspected working-set implementation has a ledger and revision-aware recall, not the older `Select` path described there.

Already implemented and worth preserving:

- Scope-local short recall handles and paginated archive search/read in `internal/context/working_store.go`: [`WorkingStore.Search:230`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/context/working_store.go#L230), [`Handle:298`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/context/working_store.go#L298), and [`Read:367`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/context/working_store.go#L367).
- Whole-file and element-level revision identities; unresolved `absent`/`unavailable` sentinels do not prove freshness, in [working_set.go:173–235](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/context/working_set.go#L173-L235).
- Stable request ledger in `internal/session/working_context.go`, `prepareWorkingRequest:577–642` (linked above), and explicit [`staleObservationNotice:840–842`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L840-L842).
- Tool-catalog and system/message costs included by `prompt.EstimateRequestTokens` at [`internal/session/working_context.go:863–868`, workingWindowRemaining](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L863-L868). These are estimates, not a provider-tokenizer guarantee.
- Structural tools offered before raw search when a structural replacement exists, in [`internal/session/working_context.go:106–126`, structuralFirstDefinitions/offersStructuralSearch](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/session/working_context.go#L106-L126); this is not a blanket removal of fallback search.

## 3. Finding R1: invalidating hits does not invalidate a query's absence claim

**Classification:** implemented invalidation, incomplete dependency coverage; high-priority correctness refinement.

[SearchKeywords:314–395](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/sparse.go#L314-L395) caches every successful per-keyword search, including an empty result. [KeywordHitCache.Get/Set:913–970](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/sparse.go#L913-L970) returns that entry until its TTL expires; the [default configuration at sparse.go:91](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/sparse.go#L91) and [constructor fallback at sparse.go:113–115](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/sparse.go#L113-L115) specify five minutes.

[InvalidateFiles](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/invalidation.go#L39-L65) delegates to [InvalidateWhere:194–221](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/invalidation.go#L194-L221), which only removes an entry if one of its **existing hits** matches a changed file.

Static counterexample:

- Search keyword `quasar`; no file contains it. Cache the empty list.
- An eligible file changes to contain `quasar`; a valid `file_written` event reaches the kernel.
- The invalidator receives the file, but the cached list contains no hit to match.
- The next identical query reads the empty entry before TTL expiry.

The same issue exists for a nonempty result: if `a.go` matched and `b.go` did not, inserting the keyword into `b.go` need not invalidate the cached `a.go`-only set. Merely disabling empty-result caching does not fix that case. A source view's revision guard cannot recover a candidate that retrieval never proposes.

### Minimal safe contract

A complete cached query result depends on the eligible **search domain**, not only its positive members:

```
cache key = (workspace identity, membership/config revision, backend semantics, query)
cache value = (hits, search-domain epoch, complete|partial, coverage)
reuse complete value only if its domain epoch is still current
```

A conservative first change can clear keyword results on any eligible write or membership-changing event. This trades work for correctness and is a valid baseline, not the intended final optimization.

A refined delta update must satisfy:

`H_next(q) = (H_prev(q) minus hits in changed/deleted files) union Search(q, changed/new files)`.

Recompute ranking after the delta; preserve the same bounds and matching semantics as a cold scan. Renames, ignore-rule changes, branch changes, unknown event gaps, and backend changes require a wider invalidation or full refresh. A truncated result may hide candidates that should replace a removed hit: unless overflow state is retained, conservatively refresh rather than claim exact incremental maintenance.

**Race requirement:** stamp a scan with an epoch before scanning and compare again before publishing. An invalidation that runs while a scan is in flight must not be undone by a late `cache.Set` of old results. This is a required future invariant, not a claim that the current locking has been dynamically shown to violate it.

**Authority boundary:** change-event facts remain the kernel's authority. A filesystem manifest or watcher-gap indicator supplies measurements; it must not silently substitute a different decision path. If change coverage is unknown, mark freshness unknown and refuse a “current complete result” claim.

## 4. Finding R2: semantic cache lifetime and coverage are separate problems

**Classification:** semantic path implemented; cross-pass reuse absent at this construction site; broad semantic coverage unverified.

[SeedIssueFacts:223–238](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/facts.go#L223-L238) creates a new `EmbeddingSemanticSearcher` for each seed. Its [constructor and vectorsFor](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/semantic.go#L75-L231) own an instance-local map keyed by path. The package-local vector cache therefore does not survive separate seed passes. An underlying embedding engine may cache requests; this inspection does **not** establish repeated paid embedding calls or quantify their cost.

Simply sharing this object would be unsafe: path-only cache hits do not check content revision. Reuse must be keyed by:

`(canonical workspace, path/element identity, content digest, representation version, embedding model/version, preprocessing config)`.

Persist successful vectors only after validating cardinality, dimensionality, and the exact bytes/digest they represent. Coordinate concurrent misses to avoid duplicate work; do not hold an IO-wide lock while embedding. Bound memory by bytes, not just entry count. Expire unreachable revisions independently of query-result freshness.

Coverage is equally important. [semantic.go:49–57](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/semantic.go#L49-L57) defines a 256-file corpus limit and 4 KiB head size; the [constructor:85–86](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/semantic.go#L85-L86) installs both. The [corpus function:144–178](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/semantic.go#L144-L178) sorts paths and takes the first 256, while [isSemanticSourceFile:262–267](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/semantic.go#L262-L267) excludes Go test files. This is a bounded lexicographic path-prefix sample, not whole-repository semantic search. A new alphabetically early file can displace a previously included file.

Proposed next step: expose that coverage before increasing it. Then compare task-conditioned selection from existing CodeDOM/world candidates against deterministic stratified sampling across modules. Preserve an exploration allocation for files outside the task's initial lexical neighborhood; otherwise lexical selection followed by semantic ranking merely repeats the same blind spot. For test-writing tasks, evaluate an explicitly selected test corpus rather than silently interpreting missing tests as irrelevant.

No new index should be approved solely on the assumption that all 256 heads are useful or that more files imply more accuracy.

## 5. Finding R3: partial/degraded retrieval needs first-class provenance

**Classification:** existing logging/reporting, incomplete machine-readable outcome contract.

[SearchKeywords](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/sparse.go#L314-L395) logs individual keyword errors after collecting successful hits but returns `allHits, nil`. A caller therefore cannot infer completeness from its error return.

Similarly, [semanticExpansion:531–541](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/tiered_context.go#L531-L541) falls back after failed search or no usable new embedding matches, including when raw matches are filtered out as duplicates or nonexistent paths. `SeedReport.SemanticTier` is assigned `embeddings` when an engine is supplied, before the actual outcome is known. It describes configuration more reliably than the backend that furnished the final files.

Proposed report fields:

| Field | Contract |
|---|---|
| query/workspace/epoch | Identifies the search domain and request |
| status | complete, partial, unavailable; empty is a result count, not a status |
| tier attempted / tier used | Distinguishes configuration from actual execution |
| reason | timeout, backend error, no semantic hits, bounded corpus, overflow, membership uncertainty |
| scope | eligible/visited/indexed counts and corpus-selection rule |
| omitted coverage | What was not searched, with a bounded next operation |
| cache state | cold, fresh hit, delta refreshed, invalidated; no hit for partial entries pretending to be complete |

Feed compact status facts to Mangle. Let policy decide whether to broaden, use a fallback, or continue with known partial evidence. Render a short model-facing qualifier only when decision-relevant; do not paste a large diagnostic record into every prompt. Full trace data belongs in the diagnostic ledger.

The current brief already says its files are places to start reading and structural queries can answer misses. Preserve that useful hedge, but it is not a replacement for knowing whether a pass failed halfway.

## 6. Context-value objective and full costs

Optimize verified task outcome under a budget, not compression ratio or cache hit rate in isolation:

`min E[C_total] subject to correctness >= preregistered target, stale-current admissions = 0, and input <= configured window`.

Use a cost vector before collapsing to money:

`C_total = scan IO + hashing/parsing + embedding input/compute + kernel work + persistence + model input/cache-write/cache-read/output + tool calls + retries + verification + human correction`.

Report bytes scanned, files visited, eligible/covered files, vector misses, duplicated in-flight embeds, request tokens estimated versus provider-reported, cache categories, latency distribution, peak memory, and storage growth. Retrieval file-size estimates in [facts.go:539–557](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/retrieval/facts.go#L539-L557) are not billed model input: the brief initially contains paths and ranks, not all file bodies.

For selected evidence item i, estimate net value as expected reduction in unresolved-task loss minus acquisition, prompt, and likely rehydration costs. These are decision features, not calibrated facts until measured. A small omission handle can be cheaper initially and more expensive over repeated recalls. Record total bytes/tokens reread across the task, not just the largest request.

Stable ordering matters for repeatability and prefix reuse. Do not reshuffle an unchanged brief every round. Freshness takes priority over cache preservation: byte stability is not a reason to retain a false current-state claim.

## 7. Evaluation contracts: proposed, not executed

### Deterministic correctness fixtures

1. Empty cached query → insert matching symbol → same query before TTL: newly matching file must appear.
2. Existing hit in A → add match in B: both appear; merely disabling negative caching must fail this fixture.
3. Delete/rename the only matching file, change ignore membership, and switch backend semantics: no stale-domain answer.
4. Pause a scan, mutate a file, resume scan publication: old-epoch results cannot overwrite a fresh result.
5. Fail one keyword worker while another succeeds: status is partial and preserves valid hits.
6. Embedding engine configured but returns error/empty: report attempted embeddings and actual fallback separately.
7. Reuse unchanged semantic content across passes, then edit one element/model version: only valid identities reuse; no numeric speedup threshold.
8. More than 256 files, relevant file after lexical cutoff, relevant declaration after byte 4096, test-only answer: coverage metadata exposes exclusions and authorized broadening finds them.
9. Evict and recall an observation after its file changes: the historical body remains recoverable but is not current evidence.
10. Parallel issues: release one pass without retracting the other's facts or contaminating its brief.

### End-to-end task suite

Use normal production entry paths with world/CodeDOM enabled. Include bug localization, API migration across importers, test addition, late-file discovery, multi-turn edits, rename/delete, and concurrent changes. Include repos ranging from small packages to large multi-language trees; stratify by cold/warm cache and edit rate.

Compare: current pinned harness; conservative full-cache invalidation; coverage-aware delta reuse; revision-safe semantic reuse; and coverage-aware semantic selection. Hold model/version, tools, verification budget, issue text, environment and sampling policy fixed. Also compare complete coding systems at equal total budget; do not compare CodeNERD's retrieval alone against an entire competitor's bill.

Primary outcomes: independently verified patch/task correctness, unresolved obligations, false absence claims, stale-evidence use, and total task cost. Secondary outcomes: candidate recall at fixed prompt budget, precision, latency, retries, replay stability, archive recalls and coverage. Include repeated seeds and confidence intervals for stochastic task outcomes. Count failures and timeouts in the denominator.

Ablate each refinement separately. Reject an optimization that lowers initial tokens while increasing wrong edits, hidden partial results, or full-task retries. Report Pareto frontiers, not an unsupported “most efficient harness” label.

## 8. Primary research grounding and limits

- [RepoCoder, Zhang et al., EMNLP 2023](https://arxiv.org/abs/2303.12570) studies iterative repository retrieval and generation for code completion. It motivates measuring retrieval's contribution across iterations rather than one static top-k list. Its completion results do not establish CodeNERD patch-task performance.
- [DBToaster, Ahmad et al., VLDB 2012](https://vldb.org/pvldb/vol5/p968_yanifahmad_vldb2012.pdf) develops incremental maintenance of materialized query results using deltas. The relevant principle here is that changed inputs can add as well as remove result members. The proposal above is a bounded retrieval-specific delta contract, not an implementation of DBToaster or a claimed novel algorithm.
- [Lost in the Middle, Liu et al., TACL](https://arxiv.org/abs/2307.03172) shows position-sensitive performance in the studied long-context QA and key-value tasks. This motivates order/position ablations; it does not prove that compression eliminates the issue for coding or for current models.

## 9. Ordered work queue and stop conditions

| Order | Proposed work | Checkable exit |
|---|---|---|
| 1 | Domain-correct keyword invalidation | Negative, new-positive, deletion, membership and in-flight fixtures pass |
| 2 | Partial/degraded outcome contract | Kernel/report/model-facing qualifier agree on actual backend and coverage |
| 3 | Revision-safe semantic ownership | Reuse survives passes without stale bytes or cross-workspace/model leakage |
| 4 | Coverage-aware corpus selection | Late-path/body/test counterexamples are exposed and recoverable under a bounded policy |
| 5 | Task-level optimization study | Correctness non-regression and complete cost vector reported on normal entry paths |

Before implementation, update the owning architecture specs and their gap IDs using the repository's spec-first convention; this audit does not close historical rows. Stop broadening a retrieval pass when its explicit budget is reached and preserve the unresolved coverage obligation. Do not turn a budget stop into evidence that the missing symbol or implementation does not exist.


## As built (2026-10-27)

Built on 6ef257cb (one native walk for every uncached keyword, reader pool of `min(NumCPU,32)`): the walk's per-file body is now `scan_native.go:SparseRetriever.scanFile`, shared by the walk and the delta below.

**Implemented**

* R1 — query-level absence invalidation (queue item 1). `keyword_cache.go:KeywordHitCache` is keyed by a search domain (`Begin`): `search.go:SparseRetriever.searchDomain` = backend + purpose globs + new `workspace.Membership.Revision` (root/patterns identity and git HEAD, its ref, index, `.gitignore`, `info/exclude` stamps); another domain empties the cache. Every change moves the cache epoch and a scan publishes only into the epoch it started in (`Set(keyword, result, epoch)` refuses a late pre-change result). `invalidation.go:ApplyFileChanges` maintains every cached result by `H_next = (H_prev − hits in changed files) ∪ Search(q, changed files)`, reading each changed member once for all cached keywords; deleted/excluded files only lose hits. Ignore files, `.git/` paths, the external backend, or a membership failure clear the cache instead; overflowed results (cap reached) are dropped, as is any result a delta pushes to the cap. `InvalidateFromKernel` now also reads `file_deleted/1` (deletes never invalidated before), and treats timestampless events as new when absent at the previous pass.
* R3 — first-class provenance (queue item 2). `outcome.go:TierOutcome` (status complete/partial/unavailable, reason, attempted vs used backend, eligible/covered, queries/cache hits/incomplete keywords). `SearchKeywords` returns `([]KeywordHit, TierOutcome)` instead of a nil error after logged failures; failed keywords are partial and never cached. `tiered_context.go:semanticExpansion` reports the attempted and used backend separately (embedding error → partial via `definition_scan`; no usable matches → `no_semantic_hits`; bounded or incomplete corpus → partial). `SeedReport.Outcomes` replaced `SemanticTier`. `OutcomeFacts` asserts `retrieval_outcome/4`, `retrieval_backend/4`, `retrieval_coverage/4` (schemas_knowledge.mg 52.6, issue-keyed, retracted with the issue). Policy `retrieval_brief_caveat/4` (policy/retrieval.mg) decides what the model is told; `decisions.go:RenderBrief(files, caveats)` renders it beside the brief, even when no file was found. Consumers: the task brief (`TaskRetriever.Retrieve` → working-loop anchor), the chat activation priorities (`retrieval_brief_caveat`: 85), `nerd retrieve` output and `--facts`.
* R2 — semantic cache lifetime vs coverage (queue item 3). `semantic.go:EmbeddingSemanticSearcher` keys each vector by path + sha256 of the exact head bytes (the searcher is bound to one engine, workspace and head size); concurrent misses wait on one in-flight embed; empty vectors are not cached; vectors whose dimension differs from the query's are not ranked and count as uncovered; vectors of files that left the corpus are dropped each pass. `SimilarFiles` returns `SemanticResult{Matches, Eligible, Covered, Incomplete}`.
* Found and fixed: a typed-nil `*EmbeddingSemanticSearcher` in `TieredContextConfig.Semantic` read as configured; `FindRelevantFiles` had no production caller and was removed.
* Tests (fixtures of §7 that are deterministic): `invalidation_test.go` (1 empty→new match maintained from cache, 2 second match joins, 3 delete/edit/ignore change, 4 in-flight publish refused, overflow, cursor, path spellings, 5 one failed keyword partial, canceled walk unavailable, overflow partial), `outcome_test.go` (policy caveats, retraction, render, end-to-end through `TaskRetriever`, 7 content-keyed reuse, forget, concurrent misses, dimension mismatch, 8 bounded corpus), `wiring_test.go` (6 attempted vs used), `workspace/membership_revision_test.go`.

**Ruled differently**

* The cache key's "membership/config revision" is `Membership.Revision`; the index stamp is included, so an index rewrite (stage, reset, checkout) clears the cache — correctness over hit rate.
* The bounded semantic corpus is asserted as fact but policy does not caveat it: it holds on every pass over a large repository.
* The cached delta is applied only to native-scan results; with the ripgrep backend any change clears the cache (path forms are the backend's).

**Deferred**

* Queue item 4 (coverage-aware corpus selection: stratified vs task-conditioned, exploration allocation, explicit test corpus): coverage is now exposed, but choosing the rule needs the paired task study; the corpus is still the lexicographic first 256 heads.
* Out-of-band edits with no kernel event (an editor outside codeNERD) are still bounded only by the 5-minute TTL; no watcher-gap measurement exists to mark freshness unknown.
* §6/§7 task-suite and cost-vector evaluation, fixture 9/10 (covered by existing working-set and issue-scoping tests) need live models.

> **Closing note (2026-10-28).** Earlier work (6ef257cb, c9432be6, 7b3844ec) built R1-R3; re-verified on main, none reopened. New this pass: the semantic corpus is now spread round-robin over directories instead of the first 256 sorted paths (`semantic_corpus.go`, `semantic_corpus_test.go`; queue item 4 in part).
> Ruled out: keeping the lexicographic prefix (task-independent blind spot). Left: task-conditioned selection and an explicit test corpus (need the paired task study); out-of-band edits are still bounded only by the cache TTL (no watcher-gap signal exists); the §6/§7 live evaluation.
