# Observability

## The receipt

One per call, produced by the pass that produced the request:

```
purpose  provider  model  method  duration
estimated: tokens + confidence + source + segment split
actual:    input / output / cached / thinking, from the provider
estimate_error_pct
decision:  code, reason, window, headroom
error
```

`EstimateErrorPct` is the number to watch. It is the meter grading itself: the
gap between what admission believed and what the provider charged. On Anthropic
it should sit at zero, because the count came from the provider. Elsewhere it
should shrink over a session as the calibrator learns; if it does not, either the
calibration guards are rejecting every sample or `measure()` has drifted from
what the client actually sends.

## Reading it

```go
for _, r := range broker.Default().Receipts() { ... }        // the recent ring
broker.Default().Ledger().Accounts()                          // spend per purpose
broker.Default().Ledger().Total()                             // session total
broker.Default().Calibrator().Snapshot()                      // ratio + n, per model
broker.Default().TextCounter("").Confidence()                 // is the budget measured or guessed
```

## Logging

`LogSink` writes one debug line per call under `logging.CategoryAPI`, and
escalates refusals to warn. A refusal is the one receipt an operator must not
have to go looking for: it means a turn did not happen, and the reason needs to
sit next to whatever the user saw instead.

Never logged: prompts, tool arguments, tool results, user content, API keys.

## What this makes answerable for the first time

- What did this session cost, broken down by subsystem?
- Which subsystem is the expensive one? (Historically assumed to be the session
  executor; the classification tier runs on every turn and nobody had counted it.)
- How wrong is the meter, per provider, right now?
- How many turns were refused, and for what?
- How long does a turn actually take, and where?

## The measurements the roadmap is gated on

Two histograms decide whether the expensive parts of the idea set are worth
building. Both are computable from receipts and neither exists yet as a report:

**Calls per epoch.** The rebuild controller in Phase 4 breaks even after a
computable number of calls against a warm prefix. If the median epoch in real
sessions is shorter than that, the rebuild never pays and the machinery is dead
weight. This is a day's work against the receipt ring and it gates months of
build.

**Atom co-use.** Which atoms are selected together in turns that succeeded. The
lane taxonomy in Phase 5 is currently a guess — architecture / implementation /
verification is how *humans* organize software teams, which is not evidence about
how *information* clusters. Measure before hard-coding an org chart.

## Deliberately absent

**Specialist utilization**, when lanes exist, is a trap metric. The correct
utilization for a well-designed verification lane might be 5% — activated rarely,
decisive when it fires. Driving it up means a router consulting specialists out of
habit. It is the same proxy inversion as cache-hit rate: measure it, never
optimize it. Recorded here because someone will put it on a dashboard, and
dashboards make people optimize what is on them.
