# Receipts and epochs

What `internal/broker` records per call, and what the accumulated
record can and cannot prove about prefix caching.

Verified 2026-09-20 against the working tree (commit hash unavailable:
no shell tool in this environment; re-pin with `git rev-parse --short HEAD`).

## Receipts: one per call, through non-blocking sinks

`ReceiptSink` (receipt.go:11) consumers must not block: they run on
the request completion path. Three in-memory shapes plus one on disk:

- `RingSink` (receipt.go:19): bounded, newest wins; non-positive
  size gets a 256 default rather than a buffer that silently drops
  everything (receipt.go:29-34).
- `MultiSink` (receipt.go:76): fans out, tolerates nils.
- `LogSink` (receipt.go:93): debug line per receipt, escalated to
  warn on refusal — a refusal means a turn did not happen and must
  sit next to whatever the user saw instead (receipt.go:87-103).
- `FileSink` (filesink.go:20): appends to a rotating JSONL log under
  `meter/receipts.jsonl` (filesink.go:17). Rotation mechanics live
  in `internal/jsonl`, shared with prompt-atom selection logs, so
  the two cannot drift (filesink.go:11-14). `Failures()` separates
  "nothing spent" from "sink broken since boot" (filesink.go:41-49),
  and `ReadReceiptLog` (filesink.go:70) is the read path back.

Settle fills the receipt's bill side from the accumulating observer
first — it fires on every provider and path, including streaming,
where the return value carries no usage — falling back to the
response-carried usage block; cached/thinking sub-counts are copied
as metadata already inside the totals, never double-counted
(broker.go:171-191). `EstimateErrorPct` is estimate-vs-input only
(broker.go:194-197).

## Reconciliation: a drift alarm, not a brake

`Reconciler` (reconcile.go:45) exists for the blind spot
calibration cannot see: on non-Anthropic providers nothing
independent checks `measure`, so a systematically wrong measure
would produce a plausible ratio and correlated wrong counts with
nothing contradicting them. Comparing accumulated predicted-vs-billed
totals catches the shared bias calibration absorbs
(reconcile.go:30-43).

It reports only: 20-sample floor, 10% mean-absolute-error
threshold, one warn per model ever (reconcile.go:11-28, 91-105).
`ModelDrift.Trustworthy` is true below the sample floor — untested,
not passing — and `Samples` is what distinguishes the two
(reconcile.go:146-149). Nothing feeds back into admission; a
drifting model keeps spending while the log complains.

## Epochs: the Q1 measurement for an unbuilt cache

An epoch is one run of calls sharing a cacheable prefix. The prefix
fingerprint covers system plus tools only — messages are excluded
because the JIT-compiled system prompt plus per-turn file context
changes turn to turn by construction, while a turn's native tool
loop reuses one system string across rounds (epoch.go:28-42).
`PrefixTokens` is therefore an order of magnitude, not a count
(epoch.go:168-172).

`Segment` (epoch.go:208) groups by scope/provider/model, orders by
start time, cuts on prefix change, and drops refusals (a refused
call never warmed or read any cache, epoch.go:205-207).
`Histogram` (epoch.go:358) reports the calls-per-epoch distribution
with geometric buckets, singleton and no-prefix rates, per-provider
break-even verdicts that also expire epochs past the provider TTL
(epoch.go:417-427), and a per-method split — because p50 = 1 means
opposite things for plain completions (nothing to loop over) versus
tool loops (the caching problem), and the aggregate hides which
(epoch.go:337-355). Percentiles use nearest-rank so results are
observed values, never interpolations (epoch.go:469-484).

`BreakEvenCalls` (epoch.go:136) is call-count, not token-count: the
prefix size cancels out of the amortization. Reads at full price
return +Inf — no call count ever repays the write (epoch.go:134-140).

Nothing on the admit/settle/emit path invokes `Segment` or
`Histogram` — analysis runs outside the request path. The comments
frame the histogram as the input to a build decision ("whether
Phase 4 is worth building", epoch.go:452; "a prefix that serves one
call can never repay a cache write", epoch.go:295-297). That rebuild
controller does not exist in this package: a grep over
`internal/broker` for rebuild/controller returns comments and test
comments only, no implementation.
