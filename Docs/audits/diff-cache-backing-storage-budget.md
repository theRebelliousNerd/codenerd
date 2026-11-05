---
title: Diff-cache byte budgets must follow retained backing storage
subsystem: diff-session-repair-context
status: source-supported; proposed contract; evaluations unrun
source_commit: 5ce0623a1a385268b0564657a9a4c249343d1063
review_date: 2026-11-05
---

# Diff-cache byte budgets must follow retained backing storage

## Finding and decision

**A small cached diff can keep a much larger reconstructed source segment alive while being charged only for its visible hunk lines.** The diff engine reconstructs whole equal/change segments, splits each segment into substring views, keeps a few context lines, and copies the resulting line structs into its LRU. The copies isolate mutable slices but do not detach the strings' backing storage. The byte estimator then adds each visible string's length, not the allocation that the substring keeps reachable. [Engine pipeline][engine] [Line projection][operations] [Hunk selection][hunks] [Cache admission][admission] [Clone and charge][clone-charge]

Consequently, the default 32 MiB byte counter is not a reliable bound on the source storage retained by the cache. Its independent 512-entry limit still holds. This is a **retained-storage accounting defect**, not a claim of unbounded entry growth, incorrect diff content, a hash collision, or a measured out-of-memory incident. A short repair prompt and an accurate logical-payload counter do not establish a small live cache footprint.

**Proposed decision:** make cache-owned byte storage explicit. Detach the selected hunk text into bounded owned storage before publishing it, or conservatively account for every larger allocation it retains. Preserve the existing LRU, result isolation, cache identity and decision-consumer restrictions. Do not fix a host-memory contract by hiding necessary context or changing Mangle's work obligations.

This report is a static source deduction at public main [`5ce0623a1a385268b0564657a9a4c249343d1063`][pin]. The declared `github.com/sergi/go-diff v1.4.0` was inspected at [`57c41f4cb9849a2e83cdbd7644b31e6d7a7e2586`][library-pin]. The declared Go 1.27.3 standard-library implementation was inspected at [`2aae1000ea1d4b28923b96db28acf4e3b99d6aff`][go-pin]. These source pins do not establish the compiler or binary deployed by any particular user. [Declared versions][module]

No CodeNERD application, test, build, benchmark, runtime reproduction, failure injection, inference or training was executed. The examples, heap observations and comparisons below are proposed and unrun. No production frequency, exact RSS, observed GC delay or measured token saving is claimed.

## Intended behavior and present status

Root guidance requires bounded, recoverable context throughout long-running work. The diff architecture assigns this package two useful jobs: condense source changes for repair reasoning, and avoid repeated computation through a bounded cache. Those are different resource contracts. A bounded model-facing projection can coexist with excessive retained host storage if the projection borrows larger allocations. [Root guidance][root] [Architecture guidance][architecture-guidance] [Diff architecture][architecture]

The cache code explicitly describes its byte limit as protection against a large-file diff dwarfing many small ones. `Stats.Bytes` is labeled an approximate resident payload size, and `approxSize` says exactness is unnecessary but its estimate should track real memory use. The architecture also records a decision that no production lifecycle clear is necessary because the LRU is bounded. The witness below leaves the charged amount essentially constant while the retained equal segment grows. This is more than ordinary map-header or allocator-rounding error. [Limits and stats][limits] [Estimator][clone-charge] [Lifecycle decision][lifecycle]

| Status | Source-supported behavior and limit |
| --- | --- |
| Implemented | A mutex protects the LRU, with entry and byte counters, eviction, oversized-entry rejection and explicit clearing. |
| Implemented | Cached and returned hunk/line slices are distinct, preventing a caller's struct or slice edits from corrupting later results. |
| Implemented | Repair prompt output has separate per-file and per-turn budgets with omission notices. |
| Partial | Cache charging counts visible text lengths; retained strings can still refer to larger reconstructed segments. |
| Proposed | An explicit owned-storage representation or conservative backing-storage charge at cache admission, plus retained-heap and production-route tests. |
| Unverified | Actual retained-heap magnitudes, runtime/toolchain variation, workload prevalence, proposed implementation, throughput and end-to-end task outcomes. |

Sharing immutable string storage is not itself a mutation-safety bug. The existing `Clone` comment is correct about sharing no **mutable** state. That guarantee must not be confused with independent ownership of the bytes used for memory budgeting.

## Production route

### Repair context uses the process-level cached engine

The build repair specification's `promptFor` appends `turnDiffSection` to the repair prompt. Test repair does the same. The renderer walks the written paths, obtains the retained preimage, reads the current file, and calls `renderFileDiff`. That function calls the package-level `diff.ComputeDiff`, which dispatches to the package-global `DefaultEngine = NewEngine()`. Its zero options enable caching and do not enable exact-input retention. [Build repair][build-repair] [Test repair][test-repair] [Renderer][renderer] [Render call][render-call] [Default engine][engine] [Options][options]

The full diff is computed and inserted before `assembleTurnDiff` applies the presentation budgets. Even a file section later omitted from the prompt has already gone through `renderFileDiff`. Thus shortening the final prompt does not release the cache's references. The saved-attempt renderer also uses this engine, but no automatic restoration or recovery artifact is needed to reach the resource defect. [Renderer ordering][renderer] [Budget assembly][assembly]

The source establishes reachability through ordinary repair work, conditional on its normal admission and failure conditions. It does not assert that every written file triggers a repair, that every session contains large files, or that a model has actually observed the example.

### Decision previews are a different lifetime

The approval preview uses a cache-disabled engine. The importer contract also identifies the verification evidence consumer as requiring a decision-safe engine. The primary finding therefore does not claim that authorization preview caching is enabled or that a memory issue grants permission. The normal repair-context cache is sufficient. An uncached returned diff may still borrow source-size storage during the caller's lifetime, but that is not the persistent LRU witness developed here. [Approval engine][approval] [Importer contract][importers]

## Where the retained bytes escape the charge

### 1. Whole segments are reconstructed

`ComputeDiff` converts source lines to compact identifiers, computes and cleans the identifier diff, then calls the dependency's `DiffCharsToLines`. In the pinned dependency, that function gathers each segment's original lines and reconstructs the segment with `strings.Join(text, "")`. A long equal suffix therefore becomes a long equal `Diff.Text` string. [Engine reconstruction][engine] [Dependency reconstruction][rehydrate]

For a multi-line segment, the inspected Go `Join` calculates its combined length, grows a builder and writes every element. This is a source-size backing allocation, not just an array of references to the three context lines that will eventually be shown. [Go Join][join]

### 2. Hunk formation retains substring views

`diffsToOperations` calls `strings.Split(diff.Text, "\n")` and assigns each resulting line to `operation.content`. The inspected standard-library splitter forms those lines by slicing its input string. The hunk builder assigns the selected `op.content` strings to `Line.Content`; it does not clone the bytes. [Operations][operations] [Go split][split] [Hunk construction][hunks]

With default context width three, a change near the beginning of a file keeps three lines from a long equal suffix. Dropping the remaining line operations removes their references, but any one retained nonempty substring still refers to the same larger segment allocation under the inspected standard Go implementation. A shorter string length is not a deallocation of the rest of that object.

### 3. Cache cloning preserves those string references

`diffCache.put` calls `fd.Clone`. The clone allocates hunk and line slices and uses `copy` for the `Line` structs. It does not copy `Content` bytes. The cache therefore retains the selected substring headers, including their reference to the large equal segment. The same distinction applies to a cache hit's clone: structural isolation is preserved while immutable text storage remains shared. [Admission][admission] [Clone][clone-charge]

This is not relying on the optional `cacheEntry.oldContent` and `newContent` fields. With default `VerifyCacheContent:false`, those explicit fields are not populated. The larger object is retained indirectly through a hunk line.

### 4. Admission and eviction see only the small projection

`approxSize` adds path lengths, a fixed amount per hunk and line, and `len(l.Content)`. `put` adds full input lengths only when verification is enabled. Oversized-entry rejection and byte eviction both compare the resulting estimate with `maxBytes`. `Stats.Bytes` reports the same accounting value. No subsequent pass determines which larger text allocation the selected substrings keep reachable. [Estimator][clone-charge] [Admission and eviction][admission] [Stats][stats]

This creates a self-consistent counter with an incomplete ownership model: insertion, eviction and the reported number agree with one another, but not with the byte-storage domain the bound is intended to control.

## Concrete source-derived witness

Use ordinary UTF-8 text with no NUL bytes, no empty lines and no unusual paths. Let:

```text
S(N) = N repetitions of "padding padding padding\n"
old_i = "old" + a three-digit i + "\n" + S(N)
new_i = "new" + the same three-digit i + "\n" + S(N)
oldPath = newPath = "f"
```

The notation describes constructed strings, not a fixture that was run. Use `N >= 4`, default context width and a fresh default engine. The only edit replaces the first six-character line. The repeated nonempty suffix avoids the converter's separate empty-line edge cases.

The pinned source predicts:

1. Both inputs pass binary admission and produce a cache miss for a fresh pair.
2. The first line is removed/added, while the long common suffix is an equality. Its multi-line rehydration joins the whole suffix into a string containing `24 × N` bytes.
3. Hunk selection keeps the two changed lines and three suffix context lines. The three context strings each have visible length 23 and still refer to the suffix allocation.
4. Cache admission clones the structs, preserving those references.
5. The estimator charges `2` path bytes, `64` hunk bytes, `5 × 40` line bytes, `2 × 6` changed-text bytes and `3 × 23` context-text bytes: **347 accounting bytes**, independent of `N` once the context is full.
6. After the caller releases its inputs and returned diff, the engine remains reachable and its entry still references the suffix. That suffix's storage cannot be reclaimed merely because the caller finished rendering the prompt.

The numerical 347 is an arithmetic prediction of this source-level hunk and estimator, not measured allocation or a claim that Go object headers have those exact sizes. Additional retained allocations may exist; the equal segment alone establishes the mismatch.

For an illustrative family, `N = 8192` makes each suffix 192 KiB. With 512 distinct, noncolliding content keys, the same visible hunk shape would charge 177,664 bytes while the separately reconstructed equal suffixes alone account for 96 MiB of reachable text. That exceeds the default 32 MiB budget without exceeding the 512-entry limit. These are unrun source-model values, not a benchmark or an instruction to allocate that workload.

An even narrower future acceptance fixture can set a small cache budget and use a modest suffix larger than that budget. There is no need to stress the machine or provoke OOM to demonstrate the ownership error.

### Controls and limits

- A short suffix lowers the hidden retained amount without changing the mechanism.
- A fully rewritten file has large visible hunks, so the current byte estimator can reject it correctly. That does not cover sparse changes in large files.
- No cache hit, hash collision, mutation race, failed I/O, malicious input or model mistake is necessary.
- Cache-disabled execution does not retain the result in this LRU. It still pays computation and may retain large segments while its caller holds the result.
- Eviction and `ClearCache` remove the cache's references. A caller that still holds a returned clone may independently keep shared storage live. Collection and returning pages to the OS are separate events.
- The entry limit remains meaningful. At a fixed workload and finite maximum source size, retention is finite; the defect is that the configured byte bound does not express that retention.

## Existing coverage and overlap

The source includes a particularly relevant authored test. `TestCache_ShouldEvictBeyondMaxBytes` changes a short header before 60 identical padding lines, then checks that `Stats.Bytes` stays at or below 8 KiB and that eviction occurred. It is close to the witness's sparse-change shape, but it checks the estimator against the limit enforced using that same estimator. It does not examine backing storage after other owners are released. The oversized-entry test instead changes all 200 lines, making the visible payload large. Both are useful tests, but neither establishes the missing ownership guarantee. Neither was run here. [Byte-limit tests][byte-tests]

Mutation-isolation tests replace returned line fields and check independent slice addresses. Those support the existing mutable-ownership guarantee; they should continue to pass after any storage change. A correct remedy need not eliminate safe immutable sharing between a cache entry and its returned clones. [Isolation tests][isolation-tests]

The current 396-file audit inventory, audit queue, diff architecture, related reports and targeted repository searches were screened. Exact searches for `approxSize` in audits found no result, and `diffCache` located the human-approval report. This is a bounded overlap check, not a claim about every historical branch or unpublished review.

Relevant prior work is credited rather than rediscovered:

- **Native keyword context allocation and byte budgets** already explains that short substrings can keep larger storage live. Its primary defect materializes a whole matched line repeatedly into a keyword cache with an entry-count bound. Here the diff LRU already has a byte limit, but its estimate ignores storage retained by sparse hunk views. The new contribution is this concrete library-to-hunk-to-byte-admission chain and the existing test's circular measurement. [Keyword audit][prior-keyword]
- **Projection-cache allocation acceptance** concerns an unchecked decoded row count allocating before cache rejection. The present input is valid, the allocation is legitimate during computation, and the failure is retaining it after projection under an insufficient charge. [Projection audit][prior-projection]
- **Build recovery attempt artifact fidelity** concerns the inability to reconstruct an attempted afterimage from a display artifact. This report assumes the diff's visible text is suitable for its existing display role and does not reopen binary or final-newline representation findings. It concerns the lifetime of bytes outside that visible display. [Recovery audit][prior-recovery]
- **Request-bound human approval** establishes exact decision identity and uncached approval previews. No change to those guarantees is proposed, and disabling the decision cache does not repair the separate default repair-context cache. [Approval audit][prior-approval]

## Primary-source interpretation

The Go project's `strings.Clone` documentation explicitly identifies retaining a small substring of a much larger string as a reason to request a fresh allocation. It also warns that indiscriminate cloning can increase memory use. That is directly relevant to the ownership boundary here: copy the selected retained text when needed, not every temporary line operation. The pinned splitter and dependency reconstruction source establish the specific reference chain; the API documentation is not a substitute for tracing it. [Clone contract][go-clone]

The Go team's garbage-collector guide distinguishes live heap from allocation rate and explains their different CPU/memory tradeoffs. A live reference keeps an object relevant to the collector; improving a cache counter does not itself make an object unreachable. The guide also distinguishes its standard-toolchain implementation model from the language specification. Accordingly, the proposed evaluation must state the actual compiler/runtime and measure retained heap separately from transient allocations and RSS. It does not justify a universal latency or throughput claim. [Go GC guide][gc-guide]

No new storage algorithm or general memory-retention principle is claimed. The engineering finding is an incomplete implementation of this repository's existing cache-bound contract.

## Proposed storage contract

These are design requirements, not shipped behavior or authorization to modify the implementation.

### Cache publication owns what it charges

Before publishing an entry, ensure that every text reference it retains either:

1. refers to cache-owned compact storage whose allocation size is accounted for; or
2. carries an explicit conservative backing-storage charge and lifetime contract.

For the default path, a dedicated cache-freeze operation can clone only the selected `Line.Content` strings. A compact owned arena for all selected text is another option, potentially avoiding one allocation per line. The latter must charge the owned arena's allocation and keep offsets valid. Do not introspect arbitrary string capacities with unsafe production code to guess an ownership boundary the API does not provide.

Include retained paths and, when enabled, exact verification inputs in the same ownership policy. Turning on `VerifyCacheContent` is not a principled repair: it retains and charges extra input strings but does not by itself detach the reconstructed equality. Conversely, changing the general `Clone` function to copy every string on every hit may pay unnecessary work; structural cloning and cache freezing can have distinct responsibilities.

### State the bound's domain

Specify whether `MaxCacheBytes` covers owned text plus conservative metadata, logical payload only, or another measurable domain. Preserve a useful bound on cache-retained storage rather than relabeling the existing number to excuse uncontrolled backing retention. The bound need not pretend to equal process RSS or account for the allocator with byte-perfect precision. It must not permit arbitrarily large source allocations to remain hidden behind a fixed-size charge.

A cache entry and a caller may share immutable **compact** storage safely. Eviction releases the cache's ownership, not every caller's. Report that lifetime explicitly and avoid double-counting shared storage in whole-process measurements. Preserve clearing, replacement, hit recency, concurrent access and collision behavior.

### Preserve useful work and correctness

Keep the computed diff equivalent, retain the configured context and omission notices, and leave permission/verification policy unchanged. A zero-context workaround can make the main suffix witness smaller while changing the evidence given to the repair model; it is not an equal-capability repair.

Likewise, a global source-size cache exclusion is a legitimate conservative alternative only if its reduced hit rate and repeated compute cost are measured. It may be preferable for some workloads, but it should not be described as free. Prompt truncation and the diff algorithm's timeout do not supply a retained-memory bound after a successful computation.

## Evaluation plan, all unrun

### Establish the storage witness first

Use a small isolated engine, a budget much smaller than a modest repeated-line suffix, and the real `ComputeDiff` path. Retain the engine while releasing input strings and the returned diff. Compare the current implementation with candidate cache freezing after a controlled collection point. Keep heap profiling and ownership assertions out of production behavior.

A deterministic structural test can verify that cache freezing produces independently owned text or an explicitly charged arena; the exact assertion depends on the chosen representation. A supported-toolchain diagnostic test may inspect string addresses, but such a test must be labeled implementation-specific and cannot replace whole-path retained-heap measurement. A counter-only assertion repeats the present blind spot.

| Case | Required result or observation |
| --- | --- |
| Short changed header, long equal suffix | Selected hunk unchanged; long discarded suffix is not retained without its actual storage being admitted and charged. |
| Same visible hunk, increasing suffix lengths | Cache-owned retained storage does not grow with discarded suffix bytes under a compact-storage design. |
| Tiny configured budget | Admission/eviction uses owned retained storage, not just visible string length. |
| Many distinct revisions below the entry cap | Total retained storage follows the declared byte contract; duplicate calls still hit. |
| Large visible rewrite | Oversized cache admission remains rejected without losing the computed result. |
| Equal prefix, suffix and interior gaps; several hunks | Every retained view obeys the same ownership contract, not only trailing context. |
| Cache disabled, verification enabled, custom context widths | Behavior and accounting are separately qualified; defaults are not assumed to cover every option. |
| Caller keeps a clone after eviction | No mutation corruption; caller lifetime is attributed separately from cache residency. |
| Caller drops all results; eviction and clear | Cache references disappear; eventual collection is distinguishable from OS page release. |
| Concurrent misses/hits/clear/replacement | Existing race and isolation guarantees preserved; duplicate cold computation is reported rather than confused with persistent retention. |
| Binary and unchanged inputs | Existing short-circuit/no-change semantics remain intact. |

The production-path fixture should enter the real repair prompt renderer with known preimages and readable afterimages, confirm the same bounded prompt, then end that caller lifetime while the default engine remains alive. A model call is unnecessary to establish cache ownership. A later separately authorized model evaluation can hold models, prompts, tools, source states and budgets equal to detect any unintended capability change.

### Fair baselines and full costs

Compare:

- current LRU and estimator;
- cache-owned compact text with the same entry and storage budget;
- cache disabled;
- an explicit source-size cache-admission policy, if proposed.

Use sparse edits in large files, dense edits, small files, repeated identical pairs, many distinct revisions, and production-shaped mixtures. Separate cold misses from warm hits and normal contexts from unusually wide contexts. Do not compare a warmed candidate with a cold baseline or claim a speedup from omitting the required diff.

Let `B` be input/rehydrated source bytes, `L` the line count and `H` the selected hunk-text bytes. Existing source acquisition, hashing, line conversion, reconstruction and operation materialization still examine source-sized data. The diff computation itself is data-dependent; the timeout is not a proof that every surrounding stage has constant cost. Compact cache freezing adds work proportional to selected text and its metadata. It can reduce retained source-size storage while leaving much of the cold-path transient allocation unchanged.

Measure separately:

- diff/hunk equivalence, line/context preservation, omission notices and required repair evidence;
- cache hits, misses, computes, entries, evictions, logical bytes, owned backing bytes and accounting overhead;
- allocation bytes/count, peak live heap, post-caller retained heap, GC work, pause/CPU effects and RSS;
- hashing, diffing, freezing, cache-lock wait, rendering, cold/warm latency and full task critical path;
- recomputation caused by stricter admission or eviction, plus concurrent duplicate computation;
- model input/output/reasoning and cache-token classes only where model work is actually run, including retries and failed attempts;
- actual task success, regressions and operator recovery effort in any later system trial.

Cloning may increase allocation count and cold-miss latency while reducing long-lived bytes. A single compact arena may trade allocation count for implementation complexity and internal fragmentation. Cache disabling reduces retained state but can repeat expensive diff work. Budgeting every original allocation conservatively can evict useful small projections. None is universally best without the matched measurements.

A host-memory repair should normally preserve the model-facing prompt, so it has no inherent token-saving claim. Any indirect reduction in retries, crashes or user interruption is a hypothesis requiring evidence. Do not add concurrent durations, count shared storage twice, translate unmeasured remote cost into zero, or use RSS alone as proof that an object is still live.

## Acceptance boundary

Close this issue when cache admission and retention have a defensible relationship to the backing storage that entries keep reachable, the sparse-edit witness cannot hide source-size retention behind a constant small charge, and ordinary diff results, cache usefulness and caller isolation remain correct through the repair renderer.

The narrow source-supported conclusion is: **copying hunk structs establishes mutation isolation, but a cache byte bound also needs ownership of the text allocations those structs retain.**

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/5ce0623a1a385268b0564657a9a4c249343d1063
[root]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/agents.md#north-star
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/architecture/AGENTS.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/architecture/diff/README.md
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/architecture/diff/TODO.md#L18-L32
[module]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/go.mod#L1-L26
[engine]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/diff.go#L234-L308
[options]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/diff.go#L155-L227
[operations]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/diff.go#L339-L396
[hunks]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/diff.go#L399-L485
[limits]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache.go#L8-L58
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache.go#L92-L186
[clone-charge]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache.go#L226-L264
[stats]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache.go#L210-L223
[build-repair]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/session/build_verify.go#L290-L302
[test-repair]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/session/build_verify.go#L439-L452
[renderer]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/session/turn_diff.go#L75-L122
[assembly]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/session/turn_diff.go#L125-L167
[render-call]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/session/turn_diff.go#L222-L260
[approval]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/approval/preview.go#L57-L61
[importers]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/importers_test.go#L15-L41
[byte-tests]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache_test.go#L192-L223
[isolation-tests]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/internal/diff/cache_test.go#L12-L67
[prior-keyword]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/audits/native-keyword-context-allocation-and-byte-budgets.md
[prior-projection]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/audits/projection-cache-allocation-acceptance.md
[prior-recovery]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/audits/build-recovery-attempt-artifact-fidelity.md
[prior-approval]: https://github.com/theRebelliousNerd/codenerd/blob/5ce0623a1a385268b0564657a9a4c249343d1063/Docs/audits/request-bound-human-approval.md
[library-pin]: https://github.com/sergi/go-diff/commit/57c41f4cb9849a2e83cdbd7644b31e6d7a7e2586
[rehydrate]: https://github.com/sergi/go-diff/blob/57c41f4cb9849a2e83cdbd7644b31e6d7a7e2586/diffmatchpatch/diff.go#L403-L418
[go-pin]: https://github.com/golang/go/commit/2aae1000ea1d4b28923b96db28acf4e3b99d6aff
[join]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/strings/strings.go#L468-L498
[split]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/strings/strings.go#L272-L299
[go-clone]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/strings/clone.go#L11-L23
[gc-guide]: https://go.dev/doc/gc-guide
