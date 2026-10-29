# internal/broker internals

How the meter counts, admits, settles, and stays honest about what it does
not know.

> Verified 2026-09-20 against commit `456e5217` (`main`).

## The one path every call takes

All metering lives in `core` (`internal/broker/broker.go:35-41`), which is
never handed out as an `LLMClient` on its own. `Wrap`
(`internal/broker/wrap.go:32-72`) composes `core` with exactly the optional
interfaces the underlying client implements, so capability probes above the
broker get the same answer they would get below it. `Complete`
(`internal/broker/broker.go:254-266`), `CompleteWithSystem`
(`internal/broker/broker.go:269-280`), and `CompleteWithTools`
(`internal/broker/broker.go:283-295`) all funnel through `compressing`; only
`CompleteWithStreaming` (`internal/broker/broker.go:303-322`) admits
directly, because a stream settles late (see below).

`admit` (`internal/broker/broker.go:127-160`) builds the receipt skeleton
first, so a refusal is still recorded, then counts and asks the ledger. A
count failure refuses closed (`internal/broker/broker.go:139-150`): a budget
that cannot be checked is not a budget. `settle`
(`internal/broker/broker.go:165-210`) is the only place spend reaches the
ledger (`internal/broker/broker.go:199`), which makes double counting a
structural impossibility rather than a convention. Refusals never touch the
ledger (`internal/broker/broker.go:328-331`): refusing a request costs no
tokens, and recording otherwise would inflate the number this package exists
to make trustworthy.

## Where the actual comes from

`callObserver` (`internal/broker/broker.go:98-115`) accumulates provider
usage reports (`Observed`, `internal/broker/broker.go:105-115`), installed
per call by `observed` (`internal/broker/broker.go:244-247`). The observer
is primary because it fires on every provider and every path, including
streaming where the return value carries no usage; a response-carried usage
block is the fallback (`internal/broker/broker.go:173-180`). Cached and
thinking counts are copied as metadata only — already inside the
input/output totals, never added again
(`internal/broker/broker.go:184-187`). `Spend.Total`
(`internal/broker/types.go:142`) likewise counts input plus output on the
grounds every provider here folds cache reads into input
(`internal/broker/types.go:140-141`).

Calibration closes the loop: `calibrate`
(`internal/broker/broker.go:215-235`) feeds the measured request size and
the provider actual back into the counter, adding cached tokens back first
so a cache hit does not masquerade as dense content
(`internal/broker/broker.go:225`). Reconciliation is kept separate from
calibration (`internal/broker/broker.go:205-207`) because a bias shared by
both sides would otherwise be quietly absorbed into the ratio instead of
detected.

## Counting with labelled confidence

`Counter` (`internal/broker/counter.go:11-13`) turns an assembled `Request`
(`internal/broker/types.go:86-96`) into a `Count`
(`internal/broker/types.go:68-76`). Counting happens here rather than
upstream because content is routinely appended after subsystems count —
persona concatenation, template expansion, last-minute tool serialization
(`internal/broker/types.go:80-85`). `Confidence`
(`internal/broker/types.go:14-33`) labels every count `exact`,
`calibrated`, or `seeded`, weakest first, with `Rank`/`AtLeast`
(`internal/broker/types.go:36-50`) for comparison. The contract on the
interface is explicit: returning `ConfidenceExact` for a guess is the one
unforgivable bug (`internal/broker/counter.go:7-10`).

The non-endpoint counter is `EstimatingCounter`
(`internal/broker/counter.go:31-33`), built by `NewEstimatingCounter`
(`internal/broker/counter.go:37-42`), counting by measured chars over a
learned ratio (`Count`, `internal/broker/counter.go:45-60`) and learning
through `Observe` (`internal/broker/counter.go:63`) via the optional
`CalibratingCounter` half (`internal/broker/counter.go:18-21`). `Segments`
(`internal/broker/types.go:57-65`) attributes the total proportionally when
the counter reports only a total — attribution for observability, not
separately measured quantities.

## Wrapper shapes and why there are eight

`Wrap` probes three gating capabilities — tool results, schema, thought
streaming — and returns one of eight shapes
(`internal/broker/wrap.go:54-71`), each embedding `baseClient`
(`internal/broker/wrap.go:143-145`), which embeds `*core` safely because
`core`'s gating methods are unexported
(`internal/broker/wrap.go:139-142`). The alternative — one wrapper
advertising everything — would route Gemini's synthesized-envelope path
into a native tool loop it cannot serve, failing far from this file
(`internal/broker/wrap.go:14-22`). Everything else forwards
unconditionally (`internal/broker/passthrough.go:9-26`): an "enrich if
available" probe that fails behaves identically to one that forwards to
nothing, so those stay simple. The three gating methods stay conditional
because they select control flow
(`internal/broker/passthrough.go:21-26`).

`Base` (`internal/broker/wrap.go:81-88`) reaches the innermost client for
call sites that switch on concrete type; `Walk`
(`internal/broker/wrap.go:103-114`) is the single traversal, bounded by
`maxUnwrapDepth = 16` (`internal/broker/wrap.go:119`) against a cycling
`Unwrap`. `IsBrokered` (`internal/broker/wrap.go:124-131`) reports whether
metering sits anywhere in the chain; the marker is unexported so nothing
outside can claim to be metered without being metered
(`internal/broker/wrap.go:135`). `Unwrap` on `core`
(`internal/broker/broker.go:49`) is what the walk follows.

## The methods that stay conditional

The three optional entries live on `core` so every shape forwards to one
implementation (`internal/broker/optional.go:9-14`): `completeWithToolResults`
(`internal/broker/optional.go:17-43`), `completeWithSchema`
(`internal/broker/optional.go:46-67`), and
`completeWithStreamingAndThoughts` (`internal/broker/optional.go:71-104`).
The schema is counted as transmitted content, not just the prompts
(`internal/broker/optional.go:54-62`); thought channels forward untouched
and settle with the content stream so thinking-heavy turns are not recorded
as cheap (`internal/broker/optional.go:100-103`).

Two forwarding details matter. `GetModel`
(`internal/broker/passthrough.go:33-40`) answers from the broker's own
state because the broker tracks mid-session `SetModel`
(`internal/broker/broker.go:54-63`, current model at
`internal/broker/broker.go:83-90`) and is authoritative after a switch the
underlying client ignored. `SupportsGrounding`
(`internal/broker/passthrough.go:117-120`) asks the client underneath, not
the wrapper's method set — otherwise every metered client would answer yes,
which is exactly what happened on 2026-09-19 before the fix
(`internal/broker/passthrough.go:108-116`).

## Streaming settles at the true end

`proxyStream` (`internal/broker/stream.go:39-46`) forwards content and
errors on buffered channels while recording the first error, then settles
after both forwarders return (`internal/broker/stream.go:123-129`). Settling
at return time would record every streamed turn — most of chat traffic — as
costing nothing (`internal/broker/stream.go:29-31`). Both forwarders select
on `ctx.Done` and then drain, because a consumer that abandons a stream
mid-turn would otherwise wedge the forwarder on a send forever, leaking
the observer, the receipt, and the provider goroutine
(`internal/broker/stream.go:33-38`); `drainStrings`
(`internal/broker/stream.go:136-139`) and `drainErrors`
(`internal/broker/stream.go:143-147`) are that drain, with `closedStringChan`
(`internal/broker/stream.go:13-17`) and `errChanWith`
(`internal/broker/stream.go:19-24`) constructing the refusal case. The
ordering has one stated consequence: the stream closes before the receipt
exists, so nothing may assume a receipt is present the instant a stream
closes (`internal/broker/stream.go:114-122`).

## Vocabulary the rest of the package shares

`Purpose` (`internal/broker/types.go:101-119`) travels on the context;
`PurposeUnattributed` is a real account, deliberately, so untagged spend
shows up as a number instead of vanishing
(`internal/broker/types.go:114-118`). `Decision`
(`internal/broker/types.go:163-170`) carries the admission outcome with
`DecisionCode` (`internal/broker/types.go:145-160`): `admitted`,
`window_exceeded` (a hard provider limit — sending anyway bills for the
attempt), `budget_exhausted`, `count_unavailable`. `Receipt`
(`internal/broker/types.go:176-207`) is produced by the same pass that
assembles, admits, and dispatches, so it cannot drift from what was sent;
`Scope` keeps concurrent sessions from splicing into one alternating run
and `Prefix` fingerprints the cacheable head
(`internal/broker/types.go:184-193`). `AdmissionError`
(`internal/broker/types.go:210-219`) formats the refusal; `IsAdmissionError`
(`internal/broker/types.go:228-234`) uses `errors.As` because refusals
arrive wrapped, and a bare assertion answers false on every path a refusal
actually travels.

Cross-process durability is `FileSink`
(`internal/broker/filesink.go:20-22`), opened by `NewFileSink`
(`internal/broker/filesink.go:25-31`) at `DefaultReceiptLogName`
(`internal/broker/filesink.go:17`), appending via the shared
`internal/jsonl` mechanics and readable oldest-first with `ReadReceiptLog`
(`internal/broker/filesink.go:70-72`). The in-process ring is not enough
because the spend and its readout live in different processes
(`internal/broker/filesink.go:7-14`).

## Invariants the tests pin

- Fail-closed counting: an uncountable request is refused, never admitted
  on hope (`internal/broker/broker.go:139-150`; error constructors in
  `internal/broker/errors.go`).
- Single settlement: `settle` is the only ledger write; refusals emit a
  receipt and record nothing (`internal/broker/broker.go:165-210`,
  `internal/broker/broker.go:328-331`).
- Capability honesty: every wrapper shape exposes exactly the probed
  interfaces, proven at compile time
  (`internal/broker/wrap.go:214-222`); `IsBrokered` plus the wiring test
  prove no construction path escapes metering
  (`internal/broker/wrap.go:124-131`).
- Confidence honesty: no `exact` without endpoint or provider evidence
  (`internal/broker/counter.go:7-10`, `internal/broker/types.go:16-33`).
- Streamed turns cost what they cost: late settlement plus drain-on-cancel
  (`internal/broker/stream.go:27-46`).
