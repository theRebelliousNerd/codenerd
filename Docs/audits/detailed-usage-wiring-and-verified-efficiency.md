---
doc-class: cross-cutting
subsystem: broker
implementation-status: planned
last-verified: 2026-10-27
verified-against: e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a
supersedes: []
---

# Accurate token economics: close the detailed-usage seam before optimizing

## Decision and scope

Improve verified coding outcomes per total resource spend, not tokens per request. The immediate recommendation is a narrowly scoped refinement of [T-01 and T-05 in the existing token-economy corpus](../journeys/11-idea-corpus-2026-09-26/tokens.md): preserve detailed provider usage through the observer-to-broker boundary on streaming and text-only calls. This is not a new replacement roadmap, nor a claim that codeNERD lacks metering.

**Evidence status:** source inspection only at the pinned revision above. No program, test, model, training, benchmark, or computer task was executed. The concrete seam below is demonstrated by source data flow, not a newly observed production incident. Proposed contracts and experiments are not shipped capabilities. No accuracy, speed, or cost win has been measured by this review.

This proposal preserves the repository's architecture: the LLM solves and synthesizes; Mangle derives executive decisions; Go carries provider observations and typed effects. It does not move orchestration into a Go optimizer, relax verification gates, or impose an arbitrary call ceiling.

## 1. What exists, and where information stops

| Boundary | Inspected evidence | Consequence |
|---|---|---|
| Anthropic streaming producer | [`internal/perception/anthropic_stream.go:17,54`, `CompleteWithStreaming`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/perception/anthropic_stream.go#L17) defers `trackUsageDetailed` with input, output, cache-read and cache-creation counts. | The adapter has the cache breakdown; absence downstream is not necessarily a provider limitation. |
| Detailed usage ingress | [`internal/usage/usage_tracker.go:1034`, `TrackDetailedFromContext`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/usage_tracker.go#L1034) sends all four counts to the tracker, then invokes the observer with only input and output. | Aggregate accounting and broker receipts receive different information from the same report. |
| Observer contract | [`internal/usage/observer.go:18,21`, `Observer` and `Observed`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/observer.go#L18) accepts model, provider, input, output and operation, but no cache fields. | The current observer interface cannot transport the detailed counts. |
| Broker capture | [`internal/broker/broker.go:114,121`, `callObserver` and `Observed`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/broker.go#L114) accumulates input/output. | It cannot recover cache detail that was not delivered. |
| Stream settlement | [`internal/broker/stream.go:128`, `proxyStream`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/stream.go#L128) passes nil response metadata to `settle`. | Streaming has no second metadata route to repair the loss. |
| Receipt enrichment | [`internal/broker/broker.go:183`, `settle`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/broker.go#L183) fills cache-read, cache-write and thinking fields only when response metadata is nonnil. | A streamed cache breakdown becomes zero-valued in the receipt even when the tracker received nonzero cache counts. |
| Existing durable receipt facility | [`internal/broker/filesink.go:34,70`, `FileSink.Record` and `ReadReceiptLog`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/filesink.go#L34) appends rotating JSONL and exposes write failures. | Reuse the receipt infrastructure; do not invent a parallel logger. |

The narrow conclusion is **cache-detail loss on this inspected stream path**, not lost total input/output or universal incorrect bills. The tracker can retain cache detail while the broker receipt cannot. Other providers and API methods need an explicit method-by-method matrix; do not extrapolate a fleet-wide incidence rate.

A second interpretation issue is explicit in [`internal/broker/types.go:182`, `Receipt.Actual`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/types.go#L182): zero actual usage can mean that the provider reported nothing. Therefore zero cache counts alone cannot establish a cache miss; they can reflect unavailable metadata. A refused request is separately identifiable through its admission decision.

### Stream closure is not receipt finalization

In [`internal/broker/stream.go:114–128`, `proxyStream`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/broker/stream.go#L114), both output channels close before a separate goroutine finishes waiting and calls `settle`. Thus observing channel closure is not evidence that the receipt has been emitted. This is an inspected ordering property, not a measured loss incident or a claim that every caller currently exits too soon.

**Proposed lifecycle requirement:** a task-cost exporter and graceful shutdown need an explicit settlement/drain barrier for all owned in-flight broker calls, followed by sink completion/failure accounting. Keep model-visible streaming responsive; do not delay every content chunk on persistence. An exporter must distinguish “stream ended, accounting pending” from “accounting complete.” If cancellation prevents final accounting, retain observed totals and mark the export incomplete rather than inventing zero spend. This barrier is a proposed contract; this review has not established whether an equivalent application-level shutdown mechanism already exists.

### Do not reopen already implemented work

The [usage gap analysis](../architecture/usage/03-GAP-ANALYSIS.md), dated July 13, lists events, cost, shard-name aggregation and atomic writes as absent. Current [`internal/usage/usage_types.go:17,54`, `UsageEvent` and `TokenCounts`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/usage_types.go#L17) and [`internal/usage/usage_tracker.go:113,199,528,617`, `Shared`, `WithEventLog`, `saveLocked` and its `atomicfile.Replace` call](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/usage_tracker.go#L528) contain these facilities. The actual shard-name aggregation is at [`usage_tracker.go:671`, inside `TrackDetailed`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/usage_tracker.go#L671); retained event writes and their bounded ring are at [`usage_tracker.go:685–712`, `TrackDetailed` and `appendEventLocked`](https://github.com/theRebelliousNerd/codenerd/blob/e16f5a3ac94e9b5887039bb5ce96832dfcf5b74a/internal/usage/usage_tracker.go#L685). Its old defect list must not be used as present implementation evidence. Updating those status claims should preserve the intended capabilities and their rationale rather than replacing the corpus with a thin summary.

## 2. Proposed contract: one normalized usage report, two consumers

Proposed, not existing API:

A detailed report carries provider/model, request-attempt identity when available, input, output, cache-read, cache-write, optional reasoning subcount, field-presence flags, report semantics, and completion status. Report semantics distinguish **incremental deltas** from **cumulative snapshots**; never infer that every stream chunk is additive.

1. Provider adapter normalizes its native semantics once.
2. The same normalized report reaches aggregate tracking and the per-call observer.
3. The broker records totals and available subcounts from that channel.
4. Response-carried metadata is fallback or consistency evidence, not a second charge. If both routes report the same request, disagreement is recorded rather than added.
5. A retry is a distinct physical attempt. Aggregate all billed attempts into task cost, but do not train a one-request token estimator against a multi-attempt total.
6. Cancellation and malformed terminal streams preserve observed usage and mark completeness unknown/partial; they must not manufacture a complete bill.
7. Unknown fields remain unknown. A compatibility adapter for the old observer marks cache fields unavailable, not observed zero.

For normalized input I, cache-read R and cache-write W, under the existing disjoint-input-category convention:

```
0 <= R, W; R + W <= I
uncached_input = I - R - W
total_tokens = I + O
estimated_USD = uncached_input*p_in + R*p_read + W*p_write + O*p_out
```

Reasoning tokens already included in output are not added again. If an API has different category semantics, normalize them explicitly before this equation. Unknown prices mean unpriced spend, not free spend. Record the pricing version; never assume one cache multiplier fits every model or TTL.

[Anthropic's primary documentation](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) distinguishes cache reads and writes and explains cache-prefix/TTL behavior. This supports retaining the categories, not an assumed win for codeNERD. A repeated prefix is an opportunity; actual cache usage is the outcome.

**Go/Mangle boundary:** Go validates and transports these measurements. Any future policy that chooses effort, caching or context retention reads typed availability and cost facts; no policy may interpret missing fields as evidence of a cheap call. A bad telemetry report must not grant permissions or change the task's correctness verdict.

## 3. Checkable closure conditions

| Local gap label | Required result | Proposed witness |
|---|---|---|
| Detail transport | Streaming receipt and tracker retain the same known cache categories. | Synthetic Anthropic-style stream through the real adapter and broker wrapper; known input/cache fields reach both sinks unchanged. |
| Dual-source consistency | Observer plus response metadata records one bill. | Same report through both paths, followed by deliberately conflicting reports; one charge and visible disagreement. |
| Presence semantics | Missing, partial and observed-zero are distinguishable. | No-usage CLI-style response, true zero cache usage, and canceled partial stream produce distinct statuses. |
| Retry identity | Retry cost is included without corrupting request calibration. | Two attempt reports: total spend sums; request estimator sees each corresponding request only, or explicitly skips unidentifiable aggregates. |
| Persistence completeness | Benchmark export detects missing records. | Sink failure/rotation/truncated-tail cases fail completeness checks instead of claiming a cheap task. |
| Settlement lifecycle | Export waits for accounting completion, not merely stream closure. | Delay settlement after closing output channels; exporter waits for its owned calls, preserves the last receipt, and reports sink/drain failures or incomplete cancellation. |

These are proposed future checks, not executed tests or acceptance receipts. Reuse existing broker streaming, cache-count, observer and persistence test seams after tracing current ownership. Before an implementation brief, reproduce the narrow seam with a falsifying fixture; the repository's elite-harness ladder correctly distinguishes plausible diagnosis from verified symptom.

## 4. Accuracy and token efficiency: the evaluation contract

A cheaper prompt that causes another repair cycle can be more expensive end to end. For task j, define A_j = 1 only when independent acceptance succeeds on the final artifact and the harness's completion claim matches the evidence. Use all attempted tasks, including failures:

```
verified_rate = sum(A_j) / number_of_tasks
tokens_per_verified_task = sum(all_attempt_input + all_attempt_output) / sum(A_j)
cost_per_verified_task = sum(all_attempt_cost + external_service_cost) / sum(A_j)
```

If no task passes, the per-success ratios are undefined, not zero. Report raw token totals separately from cache-discounted dollars. Record latency, tool CPU time, peak memory, index/embedding preparation, critic and repair calls, failed retries, and human rescue. Use a completeness indicator for telemetry; incomplete costs are lower bounds, not comparable point estimates.

Compare the same model and configuration with alternative harnesses to isolate harness effects; separately compare complete systems using each system's intended stack. Keep tasks, repository revisions, acceptance tests, tool authority, setup costs, time/resource limits and retry policy explicit. Count offline tuning and demonstrations separately from serving cost and amortize only under a stated reuse volume.

Use the normal production entry paths with world model/CodeDOM enabled. Cover small fixes, cross-file API changes, dependency migrations, ambiguous intents, stale evidence, concurrent edits, long repair chains and foreign repositories. External benchmark suites are one slice, not universal accuracy. The [SWE-bench harness](https://www.swebench.com/SWE-bench/reference/harness/) supplies a reproducible patch-and-test evaluation pattern; independent behavioral review and withheld checks remain necessary for claims beyond the test suite.

For any proposed context/cache change: predeclare paired tasks and repetitions; compare verified rate first, then latency/cost distributions and uncertainty. Preserve mandatory safety, protocol and acceptance information. Randomize cache cold/warm conditions or report them separately. Never declare an atom useless because the model did not cite it.

## 5. Next optimization, only after the measurement seam is closed

The existing token corpus already proposes cache-aware context, stable catalog and concise prompt work. Do not duplicate it. Select one existing proposal, revalidate its producer-consumer path at the implementation revision, and perform a paired capability-preservation experiment when execution is authorized.

Prioritize by **avoidable total task cost at unchanged correctness**, not estimated bytes trimmed. Make unknown reachability, absent telemetry and actual defects separate queue states. The immediate output of this document is a falsifiable wiring brief and a measurement contract—not a best-in-world claim.

