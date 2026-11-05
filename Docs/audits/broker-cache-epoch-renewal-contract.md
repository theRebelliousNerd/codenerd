---
title: Cache-epoch verdicts must follow renewal events, not total span
subsystem: broker-receipt-analysis
status: source-supported; proposed contract; evaluations unrun
source_commit: 61e33771802f865193c9c07ab2bd39f6782a7b1f
review_date: 2026-11-05
---

# Cache-epoch verdicts must follow renewal events, not total span

## Finding and decision

**`nerd meter epochs` can report an actively reused prompt cache as expired and nonpaying because it compares the whole unchanged-prefix epoch with one TTL.** The implementation marks an epoch expired when the completion of its last request by start-time order is more than five minutes after its first start, then excludes it from `PayingEpochs`. Anthropic's cache lifetime refreshes on reuse. Three eligible requests starting at minutes 0, 4 and 8 can therefore have one cache write and two reads while this report says the entry was evicted between calls and was not profitable. The Anthropic adapter itself already documents refresh-on-read. [Verdict][verdict] [CLI explanation][cli-print] [Adapter cache plan][cache-plan] [Provider contract][anthropic]

The distinct defect is **a renewal-policy measurement mismatch**, not a failure of the provider cache, a reason to increase the TTL, or proof that prompt reuse always saves money. A long total span does not determine the intervals between cache uses. The source also includes the final response duration in the comparison, although a final response cannot retrospectively turn its own cache read into a miss.

**Proposed decision:** preserve the useful unchanged-prefix call histogram, but stop presenting its total span as evidence of eviction or unprofitability. Analyze renewal opportunities from request events under an explicit provider policy, and report observed usage separately from hypothetical cache economics. Unknown timing, applicability or usage must remain unknown rather than become a cache miss or a confirmed saving.

This is a static audit of public main [`61e33771802f865193c9c07ab2bd39f6782a7b1f`][pin]. No application, test, build, benchmark, provider call, inference, training or runtime reproduction was executed. All fixtures, expected results and comparisons below are proposed and unrun. No production prevalence, saved dollars, latency improvement or cache-hit rate was measured.

## Why the measurement matters

The broker corpus treats the epoch histogram as evidence for Gate A / Q1: the distribution of repeated-prefix calls should inform whether a later cache-rebuild or prompt-ordering design is worth its cost. `OPEN-QUESTIONS.md` explicitly interprets an epoch whose span exceeds TTL as one evicted between calls. The CLI repeats that interpretation to its operator. This is not an isolated comment in an unused helper. [Q1][q1] [CLI entry][cli-entry] [CLI explanation][cli-print]

The proposed rebuild controller remains unbuilt/gated in the inspected wiring documentation. This audit does **not** claim that the incorrect statistic currently changes Mangle policy, rebuilds a cache, selects a model, or incurs a particular bill. Its immediate consequence is an incorrect user-facing classification and an unreliable input to a documented engineering decision. The raw call counts and provider-reported usage need not be wrong for that classification to be wrong. [Controller status][wiring]

Root and broker guidance require evidence-backed decisions while preserving the intended system. Correcting the measurement must not turn the broker into an executive, relax task obligations, drop useful context to improve the metric, or pretend that fixing one statistic establishes the proposed controller's value. [Root guidance][root] [Broker guidance][broker-guidance] [Broker role][broker-role]

## Implemented, partial and proposed behavior

| Status | Behavior and qualification |
| --- | --- |
| Implemented | Brokered calls receive scope, provider/model, method, start, duration, prefix fingerprint and usage metadata. |
| Implemented | Receipt logs are written through the configured workspace sink and read by `nerd meter epochs`; text and JSON use the same histogram. |
| Implemented | `Segment` groups by scope/provider/model, sorts starts, cuts on fingerprint changes and excludes locally refused calls. |
| Implemented | Anthropic tool-call paths put ephemeral cache breakpoints on the system block; the tool-results path also marks a message tail. |
| Partial / incorrect | The amortization report equates total epoch span greater than TTL with eviction and excludes that entire epoch from paying counts. |
| Proposed | Separate prefix-shape statistics, provider-qualified renewal analysis and observed usage/cost evidence, with explicit uncertainty and versioned output semantics. |
| Not established | A production cache-rebuild controller, exact per-prefix billing attribution, real workload frequency, provider-side event timing, or measured effects of this repair. |

The present cache support should not be described as missing. `CompleteWithTools` selects a system-cache plan and `CompleteWithToolResults` selects the system-plus-tail plan. Some older roadmap prose describes main-client caching as absent, but the pinned implementation supplies the relevant request-level breakpoints. That stale prose is not the evidence for this finding. [Tool methods][tool-methods] [Request rendering][cache-plan]

## Production route and the lost distinction

### 1. Calls produce receipts before analysis

The perception installer wraps clients with the broker. The broker's admission path records `Started`, scope, method and the fingerprint, then counts and admits the request. Settlement records elapsed duration and provider usage, including cache-read and cache-write token counts, usage availability, partial status and attempt count. [Installer][installer] [Receipt production][admit-settle] [Usage settlement][settle-usage] [Receipt fields][receipt-fields]

These are useful observations, but `Started` is recorded before counting, not at the remote provider's cache lookup. `Duration` includes the subsequent local and provider work up to settlement. A future timing model must name this measurement boundary instead of treating these local timestamps as exact provider arrival events.

Boot configures a file sink under the workspace. The CLI reads the receipt log, calls `Histogram(Segment(receipts))`, and emits either its JSON representation or the table headed `PAYING` and `EXPIRED`. This path requires readable receipts, not a new model execution during analysis. [Sink wiring][sink-wiring] [Receipt storage][filesink] [CLI loading][cli-load] [CLI entry][cli-entry]

### 2. A prefix epoch is a content grouping, not a cache residence interval

The fingerprint hashes tool definitions and system text; messages are deliberately excluded. An unchanged fingerprint identifies a possible repeated head under that local representation. It does not encode the complete provider cache key, actual cache-control plan, breakpoint-level hit outcome or provider eviction event. [Fingerprint][fingerprint]

`Segment` groups calls by scope/provider/model, sorts each group by `Started`, and continues the epoch while `Prefix` is unchanged. It records the first start, repeatedly assigns `Ended = r.Started + r.Duration`, and sums input and cache-read counts. It does not retain the per-request gaps in `Epoch`. [Segmentation][segment]

This content grouping is useful and can remain unchanged. Conflating it with one nonrenewable cache lease is the error. One prefix epoch may contain many renewals, several expired-and-rewritten residences, or no actual caching at all.

### 3. The verdict treats the entire span as a single expiration clock

The provider table gives Anthropic a five-minute TTL, 1.25 write multiplier and 0.10 read multiplier. `Histogram` computes:

```text
expired = TTL > 0 and Epoch.Span() > TTL
paying  = not expired and Prefix is nonempty and Calls >= BreakEvenCalls
```

It neither examines individual gaps nor uses the accumulated `CachedTokens` when deciding whether the epoch expired or paid. `Epoch` does not carry the corresponding cache-write total or receipt-level usage qualification into this decision. [Economics][economics] [Epoch fields][epoch-fields] [Verdict][verdict]

The text renderer then goes beyond naming a span threshold: it tells the user that those epochs were evicted between calls and were not profitable. JSON exposes the same classification as `expired_epochs` and `paying_epochs`, so changing only the printed sentence would leave the structured-result contract misleading. [CLI explanation][cli-print] [Verdict type][verdict-type]

### 4. The provider and adapter use a renewal clock

Anthropic's official documentation says cache use refreshes the lifetime and identifies request start, rather than response completion, as the lifetime origin. It also notes that an initial cache entry is available only after the first response starts. The inspected adapter's `ephemeral` comment independently states refresh-on-read, and its request renderer actually supplies ephemeral breakpoints. [Provider contract][anthropic] [Adapter cache plan][cache-plan]

These facts justify a conditional counterexample, not a claim that every matching request will hit. Cache identity, minimum size, available entry, provider/platform policy, routing and cache-control configuration still matter. The main witness holds those conditions fixed and uses request gaps comfortably inside the configured lifetime.

## Concrete source-derived witnesses

### A. Repeated refresh across a long content epoch

Construct three successful, single-attempt, fully reported receipts with the same scope, provider `anthropic`, model, nonempty fingerprint and method. Use the same eligible system/tool prefix throughout. Assume the initial write is available before the second request, no other invalidation occurs, and all relevant requests are in the same provider cache domain.

| Request | Start relative to first | Completion | Cache event for the fixed prefix |
| --- | --- | --- | --- |
| R1 | 00:00 | 00:01 | Write; initial expiry at 05:00 under the stipulated policy |
| R2 | 04:00 | 04:01 | Read and refresh; next expiry at 09:00 |
| R3 | 08:00 | 08:01 | Read and refresh; next expiry at 13:00 |

The times are synthetic fixture inputs, not measurements or suggested model latency. Both reuse gaps are four minutes. The first response has already started before R2. No TTL boundary equality or concurrent-cache-warmup ambiguity is needed.

The pinned `Segment` returns one three-call epoch starting at 00:00 and ending at 08:01. Its `Span()` is greater than five minutes. `Histogram` consequently returns one expired epoch and zero paying epochs for this provider, even if R2 and R3's complete usage receipts explicitly report cache reads. The operator-facing explanation claims eviction between calls. [Segmentation][segment] [Verdict][verdict] [CLI explanation][cli-print]

For the **fixed-prefix component only**, let P be its token count and let one uncached input-token price be the accounting unit. Using the pinned report's own multiplier assumptions:

```text
uncached reference = 3P
one write, two reads = 1.25P + 0.10P + 0.10P = 1.45P
```

That component is cheaper than the identical uncached reference despite spanning more than one TTL. This arithmetic refutes the unconditional nonpaying classification; it is not a measured saving or a claim about total task cost. Suffix input, output/reasoning, retries, storage charges, platform modifiers, discounts and model-specific prices must be included where applicable before reporting actual money. [Pinned multipliers][economics]

The example is reachable through the implemented Anthropic cache-plan surface. It does not require the proposed cache-rebuild controller to exist. It also does not imply that the CLI can attribute a provider's aggregate cache-read tokens to this exact prefix in a real growing conversation.

### B. The last response cannot undo an earlier cache hit

Use two eligible requests starting at 00:00 and 04:00. Let the first complete at 00:01 and the second complete at 06:00. The second request can read the warm prefix at its start, yet the histogram sees a six-minute span and calls the epoch expired and nonpaying.

The long final response matters to the possibility of a **later** reuse. There is no later request in this witness. Its elapsed generation time does not establish that R2 missed. The corresponding fixed-prefix calculation is `1.25P + 0.10P`, compared with `2P` uncached. Again this is conditional arithmetic, not a runtime or invoice observation.

### C. Actual expiration and whole-epoch loss are different questions

Even a genuine expiration inside an epoch does not prove that the entire epoch lost money. Under the same stipulated rates, requests at 00:00, 04:00 and 10:00 have write/read/write events if no other reuse occurs. Their fixed-prefix cost is `2 × 1.25P + 0.10P = 2.60P`, less than the `3P` uncached reference. The last gap caused an expiration; the whole sequence can still be cheaper.

Conversely, repeated cold writes can lose against uncached input. The contract must retain both possibilities. Counting all long epochs as nonpaying is not a conservative estimate with a declared bound; it is a categorical verdict that erases the event sequence.

## Why a one-line threshold adjustment is insufficient

Raising the TTL or comparing average gap with TTL does not reconstruct renewal events. Two four-call schedules can have the same first start and last completion but different gaps:

- 00:00, 02:40, 05:20, 08:00: every gap is below five minutes.
- 00:00, 00:01, 00:02, 08:00: the final gap exceeds five minutes.

A span-and-count summary cannot distinguish these schedules. A maximum gap can answer a limited timing question under declared assumptions, but does not by itself price multiple residence intervals or prove actual hits. Preserve event information, or compute the relevant renewal summary before collapsing the receipts.

Similarly, simply splitting the raw histogram at every long gap changes its question from “how many consecutive calls shared this head?” to “how many calls could share one residence under this policy?” Both may be useful, but they must have separate names and denominators. Keep the original shape distribution for prompt-design analysis.

The boundary clocks also matter. The broker starts its timer before the counting step; a logical receipt may combine multiple provider attempts. Treating each receipt start as an exact cache event would replace one unjustified certainty with another. Unknown or multi-attempt timing needs a qualified model or additional attempt-level observations, not invented precision. [Receipt production][admit-settle] [Attempt and status fields][receipt-fields]

## Required measurement contract

### Preserve three separate outputs

1. **Content-shape statistics.** Call counts, prefix changes, per-method groups, no-prefix cases and sample coverage. These describe the local request sequence, without claiming a provider hit or eviction.
2. **Conditional renewal analysis.** State provider/platform/model, cache namespace, policy version, cache-plan assumptions, TTL mode, timing source and price schedule. Identify likely renewal opportunities or gaps under those assumptions, along with uncertainty. Do not label a modeled expiration as an observed eviction.
3. **Observed usage and qualified cost.** Preserve read/write/ordinary-input counters, usage-known bits, partial status, conflict and attempt information. Report missing categories as unavailable. Price only the categories for which the appropriate rates and completeness are known, and distinguish reported cost from a counterfactual without caching.

These are proposed output contracts, not shipped APIs. A minimal repair can remove the categorical `EXPIRED`/`PAYING` inference and expose an explicitly descriptive long-span count while retaining raw usage. A richer implementation can add event-level renewal analysis after its assumptions and evidence are specified. It need not manufacture the full analysis before stopping the incorrect claim.

### Renewal and accounting invariants

- Extending the final response duration alone cannot change a previously established hit into a miss. It can change wall latency and subsequent availability.
- For a declared reset-on-use policy, successful reuse extends the relevant lifetime from that use's policy-defined timestamp. Total epoch duration is not the expiry predicate.
- A gap that forces a new write starts a new residence in the model; it does not automatically erase earlier savings in the content epoch.
- Under a deliberately different insertion-only or hard-expiry policy, use that policy's rules rather than importing refresh behavior from Anthropic.
- Refused-before-dispatch calls cannot warm a cache. Admitted calls without evidence of a successful cache event are not automatically hits or renewals.
- A source fingerprint is a local comparison key, not proof of full provider-cache equivalence. Unknown cache plan, changed routing/domain, unrecognized model policy or unsupported rate schedule requires a qualified result.
- Zero reported reads with the usage-known bit absent is not an observed miss. Aggregate cache tokens may include message-prefix reuse beyond the system/tool head.
- Cache read/write counts are components of normalized input totals in this repository. Do not add them to input again when calculating tokens or money. [Spend semantics][spend]
- Define “break-even,” “strictly cheaper” and “no discount” separately. Equality is not a saving, and unknown prices are not zero cost.
- Raw receipt data remains unchanged. Version the meaning of derived JSON fields or add clearly named fields rather than silently changing a stored metric's interpretation.

A cache policy belongs to its provider-qualified configuration, not to one permanent provider-name constant. The present table can supply the witness's stipulated model, but is not evidence that every model, TTL mode or platform uses identical rates. Updating all providers' economic policies is separate qualification work, not accomplished by fixing the Anthropic renewal error.

### Architecture ownership

The broker remains a meter. It may transform events into labeled measurements, but Mangle continues to own orchestration, evidence retention and completion decisions. A later controller must consume qualified evidence under its own policy and acceptance gates; this audit does not authorize enabling one.

Before implementation, reconcile the broker's receipt/epoch contract, Q1 description, CLI output and tests. A proposed gap such as **GAP-BROKER-EPOCH-RENEWAL** would close only when event semantics and report labels agree through the actual log-to-CLI path. This identifier is local to the proposal, not an assertion that it is already registered. Preserve existing architecture ambitions rather than rewriting them to match the faulty metric.

## Existing evidence and missing acceptance

The current tests cover useful fingerprint distinctions, separate scopes/models, ordering by start, refusal exclusion, billed-total accumulation and method summaries. Keep those behaviors. They do not establish renewal-aware cache interpretation. [Segmentation tests][tests-segment] [Method tests][tests-method]

`TestHistogramTreatsTTLExpiryAsNotPaying` constructs a twenty-call epoch spanning an hour and a twenty-call epoch spanning a minute. It asserts that the former does not pay. The input contains only aggregate endpoints and a count, not the individual request times or cache outcomes. Its comment says every later call in the hour-long example is a miss, but that does not follow from the supplied information: twenty starts spread regularly across an hour can have sub-five-minute gaps. The test currently encodes the disputed assumption rather than independently verifying it. This is a source reading, not a failing-test result. [TTL test][tests-ttl]

No production receipt log was read in this audit. Existing documentation's historical verification statements were not rerun and do not supply workload measurements for this issue.

## Proposed, unrun evaluation

### Deterministic semantic fixtures

| Fixture | Required observation |
| --- | --- |
| Three calls at 0, 4 and 8 minutes, complete read/write evidence | One unchanged-head epoch; no inferred expiration solely from its total span; observed reuse preserved. |
| Two calls inside TTL; extend only final response duration | Same hit/miss evidence and fixed-prefix economics; only recorded duration/span changes in this finite fixture. |
| Same count/endpoints, different internal gaps | Renewal analysis distinguishes schedules or reports insufficient information; shape histogram remains equal. |
| Write/read/write after a real idle expiry | Expiration and aggregate profitability are reported independently. |
| Every request cold-writes | No invented saving from call count alone. |
| TTL boundary exactly, just below and just above | Declared comparison and timestamp uncertainty; no unsupported universal boundary claim. |
| Initial concurrent requests before first cache availability | No inferred hit just because starts are close. |
| Refused request between successful calls | Excluded from cache-use events and cannot refresh a lifetime. |
| Failed/admitted request, partial usage, absent cache-known bit, multiple attempts | Honest uncertainty and preserved lower bounds; no zero-cost or automatic-renewal inference. |
| Prefix/model/scope/cache-domain or cache-plan change | Separate eligible analysis domains without destroying the useful raw histogram. |
| No cacheable head, below provider minimum, unknown policy/rates | No asserted paying status from timing alone. |
| Growing message tail with stable system/tools | No unsupported attribution of all cached tokens to the fixed head. |
| Truncated/rotated receipt coverage or legacy missing fields | Coverage limits remain visible; unknown sample history is not reconstructed as complete. |
| Text and JSON command outputs | Same meanings, units, denominators and completeness labels. |

Use synthetic receipts and a deterministic provider-policy oracle first. They can establish the arithmetic and reporting contract without inference or waiting eight minutes. Do not write a fixture whose expected answer is computed by the same total-span shortcut under test.

Then exercise the real receipt serialization, log loading, `Segment`, analysis and both CLI renderers with those fixtures. Unit tests of an isolated renewal helper are insufficient if the log projection discards the required event fields or the text continues to assert eviction.

A separately authorized observational evaluation may compare predictions with complete provider usage on normal production-entry tasks. It must retain the ordinary model, tool, world/CodeDOM context and task obligations. If provider-side event times or exact prefix attribution are unavailable, report that boundary rather than treating agreement on aggregate token counts as a proof of cache residence.

### Fair baselines and full costs

Compare the current report, a minimally corrected descriptive report, and a provider-qualified event analysis on **the same receipt corpus**. First score semantic correctness and uncertainty calibration. A new report should not look better merely because it removes inconvenient records, changes the grouping denominator or labels every uncertain case favorable.

For N receipts, the current grouping/sorting already uses source-sized memory and sorting work across groups. Once events are ordered, a deterministic per-policy renewal pass can be linear in that ordered input, with small state per active comparison domain. This is an algorithmic design observation, not a measured speedup. More detailed attempt/cache-domain records increase serialized bytes, privacy-review scope, parsing cost and log-retention pressure. Live collection or reconciliation can also add instrumentation and synchronization cost.

Measure separately:

- correct hit/miss/unknown classification under the declared oracle, false eviction and false profitability claims;
- report coverage, dropped/partial/legacy records, unknown timing and rate-policy cases;
- unchanged raw histogram counts, method/scoping behavior, deterministic output and schema compatibility;
- receipt/log bytes, parsing/grouping/sorting CPU, renewal-pass work, allocations, peak memory and report latency;
- added instrumentation overhead on the provider path, if new observations are introduced;
- real input/output/reasoning, cache-read/cache-write and retry costs only when an authorized live study collects them;
- full task completion, regression rate, user recovery effort and critical-path latency if a later cache policy is actually changed.

The immediate reporting repair has no inherent inference-token saving. It may alter an engineering decision, but that is a hypothesis to evaluate, not a booked benefit. A renewal model also does not establish that a new prompt layout preserves evidence quality or opaque provider continuation state. Those remain separate gates.

## Literature basis and limits

- Berger, Gland, Singla and Ciucu's **Exact Analysis of TTL Cache Networks** distinguishes a TTL reset on every request from a TTL reset only on misses, and from policies combining two timers. That primary distinction explains why elapsed residence, inter-use gaps and insertion age are different quantities. Its network-performance results do not validate CodeNERD's provider assumptions or predict LLM-cache savings. [Primary paper, Section 2][ttl-paper]
- Anthropic's official prompt-caching documentation supplies the relevant provider behavior: refresh on use, lifetime tied to request start, initial availability after response start, and usage categories. It is the provider contract for this counterexample, not evidence of a particular production hit. [Official documentation, accessed 2026-11-05][anthropic]

No claim depends on importing a stochastic traffic model from the paper. The main witness is a finite event sequence and the pinned report's own arithmetic.

## Overlap and scope

The full current `Docs/audits` inventory was checked before selecting this issue. The nearest existing reports address different boundaries:

- **`detailed-usage-wiring-and-verified-efficiency.md`** concerns usage observability and fair full-task efficiency evaluation. This report uses those distinctions but isolates a downstream TTL classifier that can contradict complete, correct usage receipts. [Usage audit][prior-usage]
- **`usage-cross-process-retry-accounting.md`** concerns usage persistence retries and duplicate peer-process accounting. The principal fixture here has one attempt per call and complete observations. [Retry audit][prior-retry]
- **`jsonl-independent-writer-rotation.md`** concerns log preservation under multiple writers. This witness assumes an intact, fully readable log; repairing persistence does not change the span predicate. [Log audit][prior-jsonl]
- **`context-feedback-independent-observations.md`** concerns context-feedback observation identity and usefulness scoring, not cache lifetime semantics. This fixture needs no duplicate feedback event. [Feedback audit][prior-feedback]
- **`diff-cache-backing-storage-budget.md`** concerns host memory owned by a local diff LRU. This report concerns provider prompt-cache telemetry and does not reuse its backing-storage witness. [Diff audit][prior-diff]

Closure requires a truthful log-to-report interpretation under renewing and nonrenewing policies, preserved raw observations and explicit unknowns. It does not require enabling a new controller or claiming a measured cost reduction.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/61e33771802f865193c9c07ab2bd39f6782a7b1f
[root]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/agents.md
[broker-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/architecture/broker/AGENTS.md
[broker-role]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/doc.go#L48-L53
[q1]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/architecture/broker/OPEN-QUESTIONS.md#L6-L23
[wiring]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/architecture/broker/WIRING-AND-NOT-BUILT.md#L93-L124
[installer]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/perception/broker_install.go#L10-L46
[admit-settle]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/broker.go#L162-L220
[settle-usage]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/broker.go#L222-L270
[receipt-fields]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/types.go#L188-L238
[spend]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/types.go#L127-L154
[sink-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/system/broker_meter.go#L27-L99
[filesink]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/filesink.go#L16-L38
[cli-load]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/cmd/nerd/cmd_meter.go#L69-L91
[cli-entry]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/cmd/nerd/cmd_meter.go#L389-L415
[cli-print]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/cmd/nerd/cmd_meter.go#L448-L461
[fingerprint]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L26-L76
[economics]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L85-L146
[epoch-fields]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L153-L196
[segment]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L198-L251
[verdict-type]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L322-L336
[verdict]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch.go#L408-L427
[cache-plan]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/perception/client_anthropic.go#L173-L223
[tool-methods]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/perception/client_anthropic.go#L322-L356
[tests-segment]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch_test.go#L185-L310
[tests-method]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch_method_test.go
[tests-ttl]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/internal/broker/epoch_test.go#L452-L469
[anthropic]: https://platform.claude.com/docs/en/build-with-claude/prompt-caching
[ttl-paper]: https://www.cs.cmu.edu/~dberger1/pdf/2014CachePerformance.pdf
[prior-usage]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/audits/detailed-usage-wiring-and-verified-efficiency.md
[prior-retry]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/audits/usage-cross-process-retry-accounting.md
[prior-jsonl]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/audits/jsonl-independent-writer-rotation.md
[prior-feedback]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/audits/context-feedback-independent-observations.md
[prior-diff]: https://github.com/theRebelliousNerd/codenerd/blob/61e33771802f865193c9c07ab2bd39f6782a7b1f/Docs/audits/diff-cache-backing-storage-budget.md
