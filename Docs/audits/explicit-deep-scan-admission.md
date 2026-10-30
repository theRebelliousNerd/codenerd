---
doc-class: deep-dive
subsystem: world-chat-scan
implementation-status: partial
last-verified: 2026-10-30
verified-against: 4cb0b3650de09085351014d522beb440c7e97ae3
verification-method: static-source-inspection
---

# An unchanged source tree does not discharge a requested deep scan

## Decision

Separate **source-change detection** from **requested analysis coverage**. On the inspected main, the TUI accepts `/scan --deep`, but its unchanged-fast-scan return precedes the deep stage. An ordinary fast scan can therefore prevent a subsequent explicit deep request from reaching the already-implemented deep cache/parser, even when deep facts are missing.

The fix should not be “disable incremental scanning.” Preserve fast-scan reuse and the existing deep cache. Instead, evaluate each requested stage against its own completion evidence. An unchanged source tree is useful evidence for a cached analysis, but is not evidence that the analysis was ever performed or hydrated into this kernel.

This is a source-only audit of main [`4cb0b3650de09085351014d522beb440c7e97ae3`](https://github.com/theRebelliousNerd/codenerd/tree/4cb0b3650de09085351014d522beb440c7e97ae3). No tests, application execution, inference, training or benchmarks were performed. The finding establishes skipped work on a production command path; it does not establish a failed coding task or claim that CodeNERD has no other source of deep facts.

## Implemented path and the missing contract

| Stage | Source-backed behavior | Classification |
|---|---|---|
| Request | `handleCmdScan` recognizes `--deep` and `-d`, then dispatches `runScan(deep)` | Implemented |
| Fast analysis | `runScan` requests an incremental scan with `SkipWhenUnchanged:true` and applies its result | Implemented |
| Fast no-change result | `ScanWorkspaceIncremental` returns `Unchanged:true` when changed/new/deleted sets are empty | Implemented; not a deep-coverage result |
| Requested deep analysis | `runScan` calls `ensureDeepWorldFacts` only after the early unchanged return | Partially wired: reachable on changed/full results, skipped on unchanged results |
| Deep cache/parser | `EnsureDeepFactsInRoot` loads depth-specific cached rows, reuses matching content, and parses missing/changed content | Implemented |
| User receipt | `scanCompleteMsg` has counts, duration and error, but no requested/completed-depth field | Implemented generic receipt; depth fidelity missing |
| Stage-aware admission/outcomes proposed below | Independent depth coverage, explicit partial/failure result and bounded scheduling | Proposed, not measured |

The command parser is not losing the flag. The loss happens after the fast stage. [`handleCmdScan`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/commands_handlers_analysis.go#L124-L147), [`runScan`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/helpers_scan.go#L159-L232)

### 1. A normal fast scan can establish the short-circuit condition

Boot dispatches `runScan(false)`. The fast full-scan path prepares and persists a fast snapshot and saves its file cache. Its Go extraction uses the fast tree-sitter parser, not the deep Cartographer pass. This is normal separation of analysis levels, not a defect in the fast scanner. [`Boot dispatch`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/model_update.go#L895-L905), [`fast snapshot publication`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/incremental_scan.go#L219-L248), [`fast extraction`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/fs.go#L435-L465)

On the next scan, unchanged means no source additions, changes or deletions. The decision does not inspect deep facts, deep parse coverage, or whether the running kernel has received those facts. Orientation refresh and treatment information have their own handling; their existence does not make this a deep-stage receipt. [`No-change decision`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/incremental_scan.go#L251-L307), [`unchanged-result application`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/apply_incremental.go#L20-L37)

### 2. The caller expands that narrow result into whole-request completion

`runScan` returns at lines 179–187 for `res.Unchanged`. The `if deep` branch is later, at lines 209–214. Therefore the caller does not ask the deep cache whether work is needed; it skips the stage entirely. The no-change branch returns a nil-error `scanCompleteMsg` with zero generated facts. The UI subsequently displays its generic successful-scan table and says the kernel has fresh codebase facts, without showing that requested deep analysis was not reached. [`Early return and deep branch`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/helpers_scan.go#L170-L214), [`completion rendering`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/model_update.go#L1061-L1085)

### 3. The intended downstream machinery already exists

`ensureDeepWorldFacts` queries `file_topology`, selects Go files using `types.ExtractString` (accepting kernel atom readback), and calls `EnsureDeepFactsInRoot`. It retracts prior rows, loads returned rows and projects knowledge links. This is not a dead helper awaiting invention. [`Deep orchestration`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/helpers_scan.go#L335-L392)

The deep worker loads cache depth `deep`, compares content fingerprints, and returns cached facts when bytes match; otherwise it invokes the Cartographer. Cached analysis and kernel hydration are distinct: even with zero new parses, a call can return cached facts for the kernel. A correct fix can exploit this existing reuse rather than deliberately invalidating every parse. [`Deep cache and parse branches`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/deep_scan.go#L77-L146)

## Minimal source-supported witness

1. Use a valid Go workspace and a functioning scanner/kernel, with a source file that has no deep-cache rows yet. Do not rely on an existing open scope or other producer to populate that file's deep facts.
2. Complete the normal boot fast scan or an ordinary `/scan`. This establishes the file manifest and fast facts.
3. Without modifying the source, request `/scan --deep`.
4. Fast change detection returns `Unchanged:true`; `runScan` returns before `ensureDeepWorldFacts`.
5. The requested route adds no deep facts and does not consult its depth-specific cache. Repeating the same request without changing the fast scan's source set repeats the skipped stage.

This witness does not require a parser error, cache corruption, concurrency race, unsupported language or model mistake. It is a control-flow result conditional on the specified normal no-change state. It remains to be reproduced through the real command handler before calling it runtime-confirmed.

The consequence is an unmet analysis request, not necessarily an empty global call graph. Missing `code_calls` and `code_defines` can prevent corresponding impact/context derivations, but whether a particular request lacks them depends on other producers and prior state. The Mangle impact rules join those predicates to modified functions and derive prioritized context; the audit does not infer a wrong patch from missing inputs. [`Impact consumers`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/core/defaults/policy/impact.mg#L28-L81)

## Counterevidence and boundaries

- **Other production routes exist.** `HolographicCodeScope.Open` and `Refresh` invoke deep maintenance. The factory installs this bridge; its scoped route can hydrate matching cached deep rows. Incremental sibling-call repair can also supply selected deep rows. Normal factory boot's `LoadAllWorldFacts("fast")` is not a general deep-cache rehydration path. Fast result application deliberately preserves pre-existing deep facts, but preservation is not creation. Do not characterize all deep analysis, CodeDOM or holographic retrieval as disconnected. [`Bridge`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/system/holographic_code_scope.go#L39-L74), [`factory`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/system/factory.go), [`sibling repair`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/incremental_scan.go#L577-L622), [`fast replacement ownership`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/apply_incremental.go#L39-L51)
- **Changed/full scans reach the branch.** The finding is the early no-change return, not unconditional suppression of `--deep`.
- **The TUI helper is Go-filtered.** Its scope is the admitted Go `file_topology` set; this audit does not promise all-language deep coverage merely because the lower-level worker supports additional extensions.
- **Fast reuse is correct for fast work.** Removing all no-change optimization would waste work and would conflate separate obligations in the opposite direction.
- **Existing policy and source tools can compensate.** Current source views or independently populated graph facts may be sufficient for a later coding task. Compensation does not make the explicit skipped request fulfilled.
- **A zero fact count alone is not failure.** A valid empty analysis and a cache hit can both involve no new parses. The missing information is stage outcome and coverage, not a minimum fact quota.

## Proposed stage-aware contract

The request should describe an analysis obligation independently of source freshness:

```text
ScanRequest = (request_id, workspace_identity, requested_stages, admitted_scope)
StageReceipt = (stage, source_generation, analyzer_identity,
                scope_coverage, outcome, reused_files, parsed_files,
                failed_files, hydrated_fact_count)
outcome = complete | cached_complete | partial | failed | canceled
```

This is a proposed interface, not a schema already implemented in CodeNERD. Avoid emitting redundant high-cardinality facts merely to satisfy this sketch; a typed host receipt can provide the measured inputs, and Mangle can derive whether the requested obligations are satisfied.

Required semantics:

1. **Retain requested stages after a fast cache hit.** A successful unchanged fast result can skip fast reparsing and still continue into requested deep admission. Do not move expensive work into every ordinary scan.
2. **Distinguish reuse from omission.** Cached deep rows matching source/analyzer identity are reusable evidence; a fast-only manifest is not. Scope coverage must say which files were considered and which were unsupported or failed.
3. **Distinguish persisted analysis from kernel hydration.** A valid durable cache need not mean the current kernel already contains its projection. Conversely, repeated hydration should avoid unnecessary retract/reload churn where current generation is known.
4. **Keep an empty result meaningful.** A successfully parsed file with no emitted deep relations can be complete. A failed read or parse cannot be reported as successful emptiness. This matters before optimizing around `len(NewFacts)==0`.
5. **Settle the actual request.** The UI should report that fast files were unchanged and deep work completed from cache, parsed additional files, partially failed, or was canceled. It should not make the user infer stage execution from one aggregate fact count.
6. **Keep authority and membership unchanged.** Stage continuation must operate only on the already admitted scope; this is not permission to inspect excluded/private files, enlarge the request or invoke models.

The smallest change conceptually is to preserve the deep branch across fast no-change completion. The stronger contract adds stage receipts and avoids future copies of the same error when other enrichment levels are introduced. Evaluate these separately: a local control-flow repair should not be blocked on building a universal analysis scheduler.

## Adjacent outcome and work-budget caveats

Honoring the branch is necessary but does not by itself prove complete deep analysis. On the inspected source, `runScan` logs a returned deep error and still proceeds to generic completion. The deep worker skips some stat/hash/parse failures without returning a per-file failure receipt. `ensureDeepWorldFacts` also returns before applying retractions when `NewFacts` is empty. These are reasons to make completion evidence explicit, not reasons to reject all existing cache reuse. This audit does not claim a tested stale-fact incident for those additional cases. [`Caller error handling`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/helpers_scan.go#L209-L232), [`worker outcomes`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/deep_scan.go#L61-L158), [`empty-result guard`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/cmd/nerd/chat/helpers_scan.go#L369-L381)

The worker semaphore limits simultaneous worker bodies, but it is acquired inside a newly spawned goroutine for every eligible path. Therefore the configured worker count is not a bound on the number of waiting goroutines. Production calls shown here pass `context.Background`; after path resolution the deep worker has no further context checks. These observations motivate bounded admission and request-owned cancellation if deep requests become frequent. They are not measured memory or latency regressions, and this proposal must not cancel shared independent work when one consumer leaves. [`Worker admission`](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/world/deep_scan.go#L38-L75)

## Full cost model

Correctly honoring a previously skipped request performs real work. It may increase immediate CPU, memory, I/O and latency compared with the broken no-op; that is not an optimization regression unless equivalent coverage is held fixed.

Account for:

- fast census/stat/hash work already performed;
- deep file identity resolution, content fingerprint checks and any rereads;
- cached-row SQL reads, decoding and allocations, even when parsing is avoided;
- parser construction, source parsing and temporary syntax structures on misses;
- per-file cache writes, including updated stamps;
- kernel retraction/loading, invalidation and subsequent derivation;
- knowledge and graph projections performed by the caller;
- receipt/coverage metadata and retained result storage;
- queued workers and cancellation latency;
- downstream context quality, additional source retrieval and repeated user/model turns caused by missing analysis.

A later generation-aware hydration optimization can avoid reinstalling an unchanged projection, but must retain cold-kernel rehydration and invalidation after source changes. Do not count absent facts, skipped files or hidden errors as a speedup. No quantitative improvement is claimed here.

## Fair evaluation and acceptance

When execution is separately authorized, use the real slash-command parser, scanner and kernel; a unit test that calls the deep helper directly misses this defect.

| Scenario | Required evidence |
|---|---|
| Fast boot or `/scan`, then unchanged `/scan --deep` with cold deep cache | Deep stage reached; expected fixture relations present; no source edit required |
| Deep scan, then unchanged deep scan | Same admitted fact set; cache reuse observable; no unnecessary source reparsing |
| Durable deep cache but a fresh kernel | Matching cached facts hydrated; cache existence not mistaken for kernel presence |
| Partly populated deep cache | Missing files analyzed; valid cached files reused; complete versus partial coverage explicit |
| Changed/full deep request | Existing reachable behavior preserved; prior generation replaced appropriately |
| Ordinary unchanged fast request | No unsolicited deep analysis; fast reuse remains cheap |
| Valid empty deep output | Complete empty result distinguished from failure |
| Read/parse/store/kernel failure or cancellation | Actual stage outcome preserved; no generic whole-request success for an unfulfilled obligation |
| Other producer already supplied facts | Correct shared state preserved; duplicate work and kernel churn measured |
| Large admitted Go set | Worker admission, peak queued work, memory and cancellation responsiveness measured |

Compare current main, the minimal skip repair, and an optional stage-receipt/generation-aware version under identical source, cache state and requested depth. First compare delivered coverage and exact fixture relations. Only then compare wall time, CPU, allocations, files hashed/parsed, SQL rows read/written, kernel mutations/derivations and canceled-work tail. If a later model-level study is authorized, hold model/tools/budgets fixed and measure verified task success plus total turns and cost. A faster first prompt is not a substitute for completed requested analysis.

**Exit condition:** a fast cache hit cannot suppress explicitly requested deep work; each requested stage either supplies valid current/cached coverage or an honest incomplete outcome; ordinary fast scans and valid deep reuse retain their efficiency.

## Prior-work boundary

This pass inspected repository and world guidance, current main source, existing audit inventory and unchanged comparison-branch heads. [World holographic continuation](./world-holographic-continuation.md) concerns identity-preserving retrieval from impact candidates. [World cache rolling-format admission](./world-cache-rolling-format-admission.md) concerns cache codec compatibility. [Scanner graph source retirement](./scanner-graph-source-retirement.md) concerns durable projection retirement. [Structural coverage/refactor preconditions](./structure-coverage-and-refactor-preconditions.md) concerns incomplete observations and edit authority. None makes a fast source no-change result proof that a newly requested deep stage has completed.

This finding belongs at the orchestration boundary: **reuse evidence for the computation it actually proves, and preserve additional requested obligations across that reuse**.
